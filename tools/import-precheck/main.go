// import-precheck 导入预检工具（§59.543 项 5·2026-11-20 用户定案工具化）。
//
// 背景：sites 导入=1:1 整行镜像（35 列含零值·§59.543 追记二定案不改）——
// 源环境没有的配置会把目标环境的静默清零（api_key 事故实证）。预检是
// 使用侧的唯一防线：导入前看清「哪些列将被清零/哪些带来增益」。
//
// 两模式（与 siteprobe 同构：主模块成员·导入 internal/ 生产链·无独立 go.mod）：
//
//	导出本环境凭证矩阵（只含长度·不含明文·不泄密）：
//	  go run ./tools/import-precheck -db data/pt-forward.db \
//	    -config configs/config.yaml -export /tmp/cred-29.json
//
//	对比两份矩阵（本地 diff·零网络）：
//	  go run ./tools/import-precheck -diff /tmp/cred-29.json,/tmp/cred-243.json
//
// 标准流程：29 export → 243 export（各自环境跑）→ 本地 diff → 看清三清单 → 再导。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/ranfish/pt-forward/internal/crypto"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/site"
	"gopkg.in/yaml.v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	dbPath   = flag.String("db", "data/pt-forward.db", "SQLite DB 路径（只读）")
	cfgPath  = flag.String("config", "configs/config.yaml", "config.yaml（加密密钥·进程内读取不回显）")
	doExport = flag.String("export", "", "导出凭证矩阵 JSON 到此路径")
	doDiff   = flag.String("diff", "", "对比两份矩阵：路径A,路径B（A=导入源·B=导入目标）")
)

type secConfig struct {
	Security struct {
		EncryptionKey string `yaml:"encryption_key"`
	} `yaml:"security"`
}

// credRow §59.543 本案实测形态——只含长度·凭证永不出明文。
type credRow struct {
	Name       string `json:"name"`
	Domain     string `json:"domain"`
	Auth       string `json:"auth"`
	Framework  string `json:"fw"`
	APIKey     int    `json:"api_key"`
	Cookie     int    `json:"cookie"`
	Passkey    int    `json:"passkey"`
	Bearer     int    `json:"bearer"`
	RSSKey     int    `json:"rss_key"`
	UploadAuth int    `json:"upload_auth"`
}

type matrix struct {
	ExportedAt string    `json:"exported_at"`
	Env        string    `json:"env,omitempty"`
	Rows       []credRow `json:"rows"`
}

var credCols = []string{"api_key", "cookie", "passkey", "bearer", "rss_key", "upload_auth"}

func main() {
	flag.Parse()
	switch {
	case *doExport != "":
		runExport()
	case *doDiff != "":
		runDiff()
	default:
		fmt.Fprintln(os.Stderr, "用法：-export <输出路径> 或 -diff <源矩阵>,<目标矩阵>（详见文件头注释）")
		os.Exit(1)
	}
}

func runExport() {
	raw, err := os.ReadFile(*cfgPath)
	if err != nil {
		fatal("读 config 失败: %v", err)
	}
	var sc secConfig
	if err := yaml.Unmarshal(raw, &sc); err != nil || len(sc.Security.EncryptionKey) < 16 {
		fatal("config 无有效 encryption_key（<16 字符）")
	}
	gdb, err := gorm.Open(sqlite.Open(*dbPath+"?mode=ro"), &gorm.Config{})
	if err != nil {
		fatal("打开 DB 失败: %v", err)
	}
	enc, err := crypto.NewCredentialEncryptor(sc.Security.EncryptionKey)
	if err != nil {
		fatal("构造解密器失败: %v", err)
	}
	if err := crypto.RegisterCallbacks(gdb, enc, nil); err != nil {
		fatal("注册解密回调失败: %v", err)
	}
	var sites []model.Site
	if err := gdb.Where("enabled = ?", true).Find(&sites).Error; err != nil {
		fatal("查询站点失败: %v", err)
	}
	m := matrix{ExportedAt: time.Now().Format("2006-01-02 15:04:05")}
	for i := range sites {
		s := &sites[i]
		// §59.512: 与导出同款幽灵过滤——只有这些站会进导入文件。
		if _, ok := site.GetSupportedSite(s.Domain); !ok {
			continue
		}
		m.Rows = append(m.Rows, credRow{
			Name: s.Name, Domain: s.Domain, Auth: s.AuthType, Framework: s.Framework,
			APIKey: len(s.APIKey), Cookie: len(s.Cookie), Passkey: len(s.Passkey),
			Bearer: len(s.BearerToken), RSSKey: len(s.RSSKey), UploadAuth: len(s.UploadAuth),
		})
	}
	b, _ := json.MarshalIndent(m, "", " ")
	if err := os.WriteFile(*doExport, b, 0o600); err != nil {
		fatal("写矩阵失败: %v", err)
	}
	fmt.Printf("✓ %d 站（enabled+supported·与导出同域）→ %s\n", len(m.Rows), *doExport)
	fmt.Println("（矩阵只含凭证长度·不含明文——文件可安全流转）")
}

func runDiff() {
	var pa, pb string
	if n, _ := fmt.Sscanf(*doDiff, "%s", &pa); n != 1 {
		fatal("-diff 格式：路径A,路径B")
	}
	if i := indexByte(*doDiff, ','); i >= 0 {
		pa, pb = (*doDiff)[:i], (*doDiff)[i+1:]
	}
	if pa == "" || pb == "" {
		fatal("-diff 需要：源矩阵,目标矩阵 两个路径（逗号分隔）")
	}
	ma, err := loadMatrix(pa)
	if err != nil {
		fatal("读源矩阵: %v", err)
	}
	mb, err := loadMatrix(pb)
	if err != nil {
		fatal("读目标矩阵: %v", err)
	}
	A := indexByDomain(ma)
	B := indexByDomain(mb)

	fmt.Printf("源 %d 站 · 目标 %d 站", len(A), len(B))
	if onlyB := len(B) - common(A, B); onlyB > 0 {
		fmt.Printf("（目标 %d 站不在源导出集·不受导入影响）", onlyB)
	}
	fmt.Println()

	// ① 将清零（源空 × 目标有）——导入的伤害面
	fmt.Println("== ① 将被清零（源空 × 目标有）==")
	harm := 0
	for _, dom := range sortedCommon(A, B) {
		var cols []string
		for _, c := range credCols {
			if val(A[dom], c) == 0 && val(B[dom], c) > 0 {
				cols = append(cols, fmt.Sprintf("%s:%d→0", c, val(B[dom], c)))
			}
		}
		if len(cols) > 0 {
			harm++
			fmt.Printf("  ✗ %-8s %-26s %v\n", B[dom].Name, dom, cols)
		}
	}
	if harm == 0 {
		fmt.Println("  （零——导入无凭证损伤）")
	}

	// ② 增益（源有 × 目标无）
	fmt.Println("== ② 带来增益（源有 × 目标无）==")
	gain := 0
	for _, dom := range sortedCommon(A, B) {
		var cols []string
		for _, c := range credCols {
			if val(A[dom], c) > 0 && val(B[dom], c) == 0 {
				cols = append(cols, fmt.Sprintf("%s:0→%d", c, val(A[dom], c)))
			}
		}
		if len(cols) > 0 {
			gain++
			fmt.Printf("  + %-8s %-26s %v\n", B[dom].Name, dom, cols)
		}
	}
	if gain == 0 {
		fmt.Println("  （零）")
	}

	// ③ 值变化（都有但长度不同——提示复核·可能是环境特定值）
	fmt.Println("== ③ 长度变化（都有但不同——环境特定值提示）==")
	chg := 0
	for _, dom := range sortedCommon(A, B) {
		var cols []string
		for _, c := range credCols {
			if val(A[dom], c) > 0 && val(B[dom], c) > 0 && val(A[dom], c) != val(B[dom], c) {
				cols = append(cols, fmt.Sprintf("%s:%d→%d", c, val(B[dom], c), val(A[dom], c)))
			}
		}
		if len(cols) > 0 {
			chg++
			fmt.Printf("  ≠ %-8s %-26s %v\n", B[dom].Name, dom, cols)
		}
	}
	if chg == 0 {
		fmt.Println("  （零）")
	}

	fmt.Println()
	if harm > 0 {
		fmt.Printf("⚠ %d 站有凭证将被清零——导入前补齐源环境或知情接受\n", harm)
	} else {
		fmt.Println("✓ 预检通过：无凭证清零风险")
	}
}

func loadMatrix(p string) (*matrix, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var m matrix
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func indexByDomain(m *matrix) map[string]credRow {
	out := make(map[string]credRow, len(m.Rows))
	for _, r := range m.Rows {
		out[r.Domain] = r
	}
	return out
}

func common(a, b map[string]credRow) int {
	n := 0
	for d := range a {
		if _, ok := b[d]; ok {
			n++
		}
	}
	return n
}

func sortedCommon(a, b map[string]credRow) []string {
	var out []string
	for d := range a {
		if _, ok := b[d]; ok {
			out = append(out, d)
		}
	}
	sort.Strings(out)
	return out
}

func val(r credRow, col string) int {
	switch col {
	case "api_key":
		return r.APIKey
	case "cookie":
		return r.Cookie
	case "passkey":
		return r.Passkey
	case "bearer":
		return r.Bearer
	case "rss_key":
		return r.RSSKey
	case "upload_auth":
		return r.UploadAuth
	}
	return 0
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func fatal(f string, a ...any) {
	fmt.Fprintf(os.Stderr, f+"\n", a...)
	os.Exit(1)
}
