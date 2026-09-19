// sqlcheck §59.251 审核工具：原生 SQL 列引用静态验证器。
// 提取 internal/ 全部 gorm 链（表名+Where/Select/Group/Order/Having/Joins/Updates/Update/
// AssignmentColumns/Pluck/Exec/Raw 片段），在 AutoMigrate 后的空库上合成 SQL 逐条 EXPLAIN
// ——no such column 即运行时地雷（编译器双盲区：map 键与原生 SQL 字符串）。
// 已知假阳性（默认白名单跳过）：
//   1. internal/db/migration*.go 与 internal/model/migrate.go 的 Exec——条件迁移 SQL
//      （先 pragma 检查旧列存在才执行，空库必报=设计如此）
//   2. download_handler.go:585 / reseed_handler.go:469——子查询参数与变量持有 builder 的
//      片段缝合（真实 SQL 合法，人工已核）
// 已知假阴性：变量持有 query builder（q:=db.Model(); q=q.Where()）不跟踪；
//   Clauses+Create（无 Model 调用）的 clause.Column 不验证（表名静态不可得）。
// 用法：go run ./cmd/sqlcheck [目录=internal]  ——suspects 非 0 即存在真实列引用地雷。
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

type frag struct {
	file string
	line int
	kind string // where/select/group/order/having/joins/exec/raw
	sql  string
}

type chain struct {
	file  string
	line  int
	table string
	frags []frag
}

func main() {
	dir := os.Args[1]
	if dir == "" {
		dir = "internal"
	}
	chains, standalone := scan(dir)
	fmt.Printf("tables=%d fragments=%d chains=%d standalone=%d\n",
		len(mustTables(openDB())), countFrags(chains, standalone), len(chains), len(standalone))

	db := openDB()
	bad := 0
	seen := map[string]bool{}
	report := func(key, format string, args ...interface{}) {
		if seen[key] {
			return
		}
		seen[key] = true
		bad++
		fmt.Printf(format, args...)
	}
	// 已知假阳性指纹（内容键不随行号漂移）：
	// download_handler: 子查询实参（torrent_snapshots AS s2）片段缝合进外层——真实 SQL 合法
	// reseed_handler: 变量持有 builder（query := Model(ReseedMatch) 后续 query.Where）+ 子查询
	//   Model(TorrentMetadata) 被 analyzeStmt 直捕——source_info_hash 属外层 reseed_matches
	knownFP := func(c chain) bool {
		if c.file == "internal/api/download_handler.go" && c.table == "torrent_snapshots AS s" {
			return true
		}
		if c.file == "internal/api/reseed_handler.go" && strings.Contains(strings.Join(whereFrags(c), " "), "source_info_hash IN") {
			return true
		}
		return false
	}
	for _, c := range chains {
		if knownFP(c) {
			continue
		}
		full := synth(c, false)
		if full == "" {
			continue
		}
		if err := tryExplain(db, full); err != nil {
			if err2 := tryExplain(db, synth(c, true)); err2 != nil {
				report(c.file+fmt.Sprint(c.line)+full, "---- %s:%d [%s]\n  SQL: %s\n  ERR: %v\n", c.file, c.line, c.table, full, err)
			}
		}
	}
	for _, f := range standalone {
		if isConditionalMigration(f.file) {
			continue
		}
		if err := tryExplain(db, f.sql); err != nil {
			if err2 := tryExplain(db, stripAliasPrefixes(f.sql)); err2 != nil {
				report(f.file+fmt.Sprint(f.line)+f.sql, "---- %s:%d [%s]\n  SQL: %s\n  ERR: %v\n", f.file, f.line, f.kind, f.sql, err)
			}
		}
	}
	fmt.Printf("\nsuspects=%d\n", bad)
}

func openDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		panic(err)
	}
	if err := model.AutoMigrate(db); err != nil {
		panic(err)
	}
	return db
}

func mustTables(db *gorm.DB) []string {
	ts, err := db.Migrator().GetTables()
	if err != nil {
		panic(err)
	}
	return ts
}

// isConditionalMigration 迁移包 Exec 是条件 SQL（pragma 检查旧列后才执行）——跳过
func isConditionalMigration(file string) bool {
	return strings.Contains(file, "internal/db/migration") || strings.Contains(file, "internal/model/migrate.go")
}

func whereFrags(c chain) []string {
	var out []string
	for _, f := range c.frags {
		if f.kind == "where" {
			out = append(out, f.sql)
		}
	}
	return out
}

func tryExplain(db *gorm.DB, q string) error {
	var rows []map[string]interface{}
	return db.Raw("EXPLAIN " + q).Scan(&rows).Error
}

var aliasRe = regexp.MustCompile(`\b([a-z][a-z0-9_]*)\.([a-z_][a-z0-9_]*)`)

func stripAliasPrefixes(q string) string {
	return aliasRe.ReplaceAllString(q, "$2")
}

func countFrags(chains []chain, sa []frag) int {
	n := len(sa)
	for _, c := range chains {
		n += len(c.frags)
	}
	return n
}

func synth(c chain, stripAlias bool) string {
	var sel, wheres, groups, orders, havings, joins, setkeys []string
	for _, f := range c.frags {
		s := f.sql
		if stripAlias {
			s = stripAliasPrefixes(s)
		}
		switch f.kind {
		case "setkeys":
			setkeys = append(setkeys, strings.Split(s, ", ")...)
		case "select", "pluck":
			sel = append(sel, s)
		case "where":
			wheres = append(wheres, s)
		case "group":
			groups = append(groups, s)
		case "order":
			orders = append(orders, s)
		case "having":
			havings = append(havings, s)
		case "joins":
			joins = append(joins, s)
		}
	}
	if c.table == "" {
		return ""
	}
	if len(setkeys) > 0 {
		// UPDATE 合成（setkeys 链）：UPDATE t SET k1=1,k2=1 WHERE 1 —— 列名验证
		sets := make([]string, 0, len(setkeys))
		for _, k := range setkeys {
			k = strings.TrimSpace(k)
			if k == "" || strings.ContainsAny(k, "(),= ") {
				continue // 表达式键（SUM(...) 等）跳过
			}
			sets = append(sets, k+" = 1")
		}
		q := "UPDATE " + c.table + " SET " + strings.Join(sets, ", ") + " WHERE 1"
		return normalize(q)
	}
	selClause := "1"
	if len(sel) > 0 {
		selClause = sel[len(sel)-1]
	}
	q := "SELECT " + selClause + " FROM " + c.table
	joinKw := regexp.MustCompile(`(?i)^(INNER|LEFT|RIGHT|FULL|CROSS)?\s*(OUTER\s+)?JOIN\b`)
	for _, j := range joins {
		if joinKw.MatchString(strings.TrimSpace(j)) {
			q += " " + j
		} else {
			q += " JOIN " + j
		}
	}
	if len(wheres) > 0 {
		q += " WHERE " + strings.Join(wheres, " AND ")
	}
	if len(groups) > 0 {
		q += " GROUP BY " + strings.Join(groups, ", ")
	}
	if len(havings) > 0 {
		q += " HAVING " + strings.Join(havings, " AND ")
	}
	if len(orders) > 0 {
		q += " ORDER BY " + strings.Join(orders, ", ")
	}
	return normalize(q)
}

var (
	inPhRe = regexp.MustCompile(`(?i)\b(IN|NOT IN)\s+\?`)
	idPhRe = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s+\?`)
	kwSet  = map[string]bool{"IN": true, "NOT": true, "LIKE": true, "AND": true, "OR": true,
		"IS": true, "BETWEEN": true, "EXISTS": true, "THEN": true, "WHEN": true, "ELSE": true}
	barePhRe = regexp.MustCompile(`\?`)
)

func normalize(q string) string {
	q = inPhRe.ReplaceAllString(q, "$1 (1)")
	q = idPhRe.ReplaceAllStringFunc(q, func(m string) string {
		parts := idPhRe.FindStringSubmatch(m)
		if kwSet[strings.ToUpper(parts[1])] {
			return m
		}
		return parts[1] + " = 1"
	})
	q = barePhRe.ReplaceAllString(q, "1")
	return q
}

// scan AST 扫描：叶子语句级链识别 + 独立 Exec/Raw（if/for 容器不整块收集——防多链缝合）。
func scan(root string) ([]chain, []frag) {
	var chains []chain
	var standalone []frag
	tableOf := structTableNames()

	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil
		}
		var stmts []ast.Stmt
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Body != nil {
				collectStmts(fn.Body, &stmts)
			}
		}
		for _, stmt := range stmts {
			if c := analyzeStmt(stmt, tableOf); c != nil {
				c.file, c.line = path, fset.Position(stmt.Pos()).Line
				chains = append(chains, *c)
			}
			ast.Inspect(stmt, func(m ast.Node) bool {
				if ce, ok := m.(*ast.CallExpr); ok {
					if se, ok := ce.Fun.(*ast.SelectorExpr); ok && (se.Sel.Name == "Exec" || se.Sel.Name == "Raw") && len(ce.Args) > 0 {
						if lit := stringLitOrFormat(ce.Args[0]); lit != "" {
							standalone = append(standalone, frag{file: path, line: fset.Position(ce.Pos()).Line, kind: se.Sel.Name, sql: normalize(lit)})
						}
					}
				}
				return true
			})
		}
		return nil
	})
	return chains, standalone
}

// collectStmts 深度收集叶子语句（表达式/赋值/return/send）——容器语句只递归不收集。
func collectStmts(b *ast.BlockStmt, out *[]ast.Stmt) {
	for _, s := range b.List {
		collectFrom(s, out)
	}
}

func collectFrom(s ast.Stmt, out *[]ast.Stmt) {
	switch v := s.(type) {
	case nil:
	case *ast.ExprStmt, *ast.AssignStmt, *ast.SendStmt:
		*out = append(*out, s)
	case *ast.ReturnStmt:
		*out = append(*out, s)
	case *ast.IfStmt:
		collectFrom(v.Init, out)
		collectStmts(v.Body, out)
		collectFrom(v.Else, out)
	case *ast.ForStmt:
		collectFrom(v.Init, out)
		collectStmts(v.Body, out)
	case *ast.RangeStmt:
		collectStmts(v.Body, out)
	case *ast.SwitchStmt:
		collectFrom(v.Init, out)
		collectStmts(v.Body, out)
	case *ast.TypeSwitchStmt:
		collectStmts(v.Body, out)
	case *ast.CaseClause:
		for _, s2 := range v.Body {
			collectFrom(s2, out)
		}
	case *ast.BlockStmt:
		collectStmts(v, out)
	case *ast.LabeledStmt:
		collectFrom(v.Stmt, out)
	case *ast.DeferStmt:
		*out = append(*out, &ast.ExprStmt{X: v.Call})
	case *ast.GoStmt:
		*out = append(*out, &ast.ExprStmt{X: v.Call})
	}
}

// structTableNames model 结构体名 → 表名
func structTableNames() map[string]string {
	m := map[string]string{}
	for _, t := range model.AllModels() {
		s, err := schema.Parse(t, &sync.Map{}, schema.NamingStrategy{})
		if err == nil {
			m[s.Name] = s.Table
		}
	}
	return m
}

// analyzeStmt 叶子语句内找链头（Model/Table）与片段
func analyzeStmt(stmt ast.Stmt, tableOf map[string]string) *chain {
	var table string
	var frags []frag

	ast.Inspect(stmt, func(n ast.Node) bool {
		ce, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		se, ok := ce.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		switch se.Sel.Name {
		case "Model":
			if len(ce.Args) == 1 {
				if t := modelName(ce.Args[0]); t != "" {
					if tb, ok := tableOf[t]; ok && table == "" {
						table = tb
					}
				}
			}
		case "Table":
			if len(ce.Args) == 1 {
				if lit := stringLitOrFormat(ce.Args[0]); lit != "" && table == "" {
					table = lit
				}
			}
		case "Where", "Select", "Group", "Order", "Having", "Joins", "Pluck":
			if len(ce.Args) > 0 {
				if lit := stringLitOrFormat(ce.Args[0]); lit != "" {
					frags = append(frags, frag{kind: strings.ToLower(se.Sel.Name), sql: normalize(lit)})
				}
			}
		case "Updates":
			if len(ce.Args) > 0 {
				if keys := mapKeys(ce.Args[0]); len(keys) > 0 {
					frags = append(frags, frag{kind: "setkeys", sql: strings.Join(keys, ", ")})
				}
			}
		case "Update":
			if len(ce.Args) >= 2 {
				if lit := stringLitOrFormat(ce.Args[0]); lit != "" {
					frags = append(frags, frag{kind: "setkeys", sql: normalize(lit)})
				}
			}
		case "AssignmentColumns":
			for _, a := range ce.Args {
				for _, k := range stringSliceElems(a) {
					frags = append(frags, frag{kind: "setkeys", sql: normalize(k)})
				}
			}
		}
		return true
	})
	ast.Inspect(stmt, func(n ast.Node) bool {
		if cl, ok := n.(*ast.CompositeLit); ok {
			if se, ok := cl.Type.(*ast.SelectorExpr); ok && se.Sel.Name == "Column" {
				for _, elt := range cl.Elts {
					if kv, ok := elt.(*ast.KeyValueExpr); ok {
						if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Name" {
							if lit := stringLitOrFormat(kv.Value); lit != "" {
								frags = append(frags, frag{kind: "setkeys", sql: normalize(lit)})
							}
						}
					}
				}
			}
		}
		return true
	})
	if table == "" || len(frags) == 0 {
		return nil
	}
	return &chain{table: table, frags: frags}
}

// mapKeys 提取 map[string]interface{}{...} 字面量全部字符串键
func mapKeys(ex ast.Expr) []string {
	cl, ok := ex.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	var keys []string
	for _, elt := range cl.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if lit, ok := kv.Key.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				keys = append(keys, unquote(lit.Value))
			}
		}
	}
	return keys
}

// stringSliceElems 提取 []string{...} 字面量元素
func stringSliceElems(ex ast.Expr) []string {
	cl, ok := ex.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	var out []string
	for _, elt := range cl.Elts {
		if lit, ok := elt.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			out = append(out, unquote(lit.Value))
		}
	}
	return out
}

func modelName(ex ast.Expr) string {
	u, ok := ex.(*ast.UnaryExpr)
	if !ok || u.Op.String() != "&" {
		return ""
	}
	cl, ok := u.X.(*ast.CompositeLit)
	if !ok {
		return ""
	}
	if se, ok := cl.Type.(*ast.SelectorExpr); ok {
		return se.Sel.Name
	}
	if id, ok := cl.Type.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// stringLitOrFormat 取常量字符串；fmt.Sprintf 取 format 串（动词替换 1）
func stringLitOrFormat(ex ast.Expr) string {
	if lit, ok := ex.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		return normalize(unquote(lit.Value))
	}
	if ce, ok := ex.(*ast.CallExpr); ok {
		if se, ok := ce.Fun.(*ast.SelectorExpr); ok && se.Sel.Name == "Sprintf" && len(ce.Args) > 0 {
			if lit, ok := ce.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				s := unquote(lit.Value)
				s = regexp.MustCompile(`%[sdvfqt]`).ReplaceAllString(s, "1")
				return normalize(s)
			}
		}
	}
	return ""
}

func unquote(s string) string {
	if len(s) >= 2 {
		return s[1 : len(s)-1]
	}
	return s
}
