package coverage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 17 刀3（v0.0.1668）: 静态锚——全库生产源码禁止 `Assign(x).FirstOrCreate(x)`
// 同指针形态（GORM finisher_api.go:354 Find(dest) 污染同指针 assigns→
// 已存在行刷新静默失效。§56.33/17 篇两处已修·防新增重犯）。
//
// 检测：同语句链内 Assign( 与 FirstOrCreate( 的参数文本相同（修剪空白后
// 逐字比对——`record`/`record` 与 `&row`/`&row` 命中；`&attrs`/`meta`
// 不命中）。注释行截断防误报（16 篇 Truncate 锚同款）；测试文件豁免。
//
// 阴性对照：任一生产文件注入 Assign(rec).FirstOrCreate(rec) → 本锚
// 点名 FAIL。
func TestNoBareAssignFirstOrCreateSamePointer(t *testing.T) {
	// 17 篇用户复审提示：扫描根扩展到仓库根（internal/+cmd/+tools/
	// ——同 16 篇 Truncate 锚同批扩展·防未来 cmd/tools 新增 ORM
	// 写入点漏网）。
	root := filepath.Join("..", "..")
	var violations []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// 一级目录白名单（17 篇扩根）：root 的直接子目录限定
			// internal/cmd/tools——examples/ 是第三方参考代码
			// （autobrr/pt-tools/qui vendored 非维护面·扩根首跑实证
			// 3 处形态全在其中）；web/frontend/.git 等跳过。二级及
			// 以下（internal/util、cmd/pt-forward 等包目录）全放行
			// ——目录名匹配形态会误跳包子目录（debug 实证 ENTER cmd
			// 后 pt-forward 子目录被 Skip）。
			if path == root {
				return nil
			}
			if filepath.Dir(path) == root {
				switch info.Name() {
				case "internal", "cmd", "tools":
					return nil
				default:
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		lines := strings.Split(string(data), "\n")
		for i := 0; i < len(lines); i++ {
			line := lines[i]
			// 注释截断
			if idx := strings.Index(line, "//"); idx >= 0 {
				line = line[:idx]
			}
			// 语句链检测：Assign( 在 FirstOrCreate( 前 0-6 行内的链式调用
			if !strings.Contains(line, "Assign(") {
				continue
			}
			// 提取 Assign 参数
			argA := extractCallArg(line, "Assign(")
			if argA == "" {
				continue
			}
			// 向后扫描链式调用（最多 6 行——同语句链）
			for j := i; j < i+6 && j < len(lines); j++ {
				l2 := lines[j]
				if idx := strings.Index(l2, "//"); idx >= 0 {
					l2 = l2[:idx]
				}
				if !strings.Contains(l2, "FirstOrCreate(") {
					continue
				}
				argF := extractCallArg(l2, "FirstOrCreate(")
				if argF == "" {
					continue
				}
				if argA == argF {
					violations = append(violations, rel+":"+itoa17(i+1)+" Assign("+argA+")/FirstOrCreate("+argF+") 同指针")
				}
				break
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(violations) > 0 {
		t.Errorf("发现 Assign(x).FirstOrCreate(x) 同指针形态（已存在行刷新静默失效——attrs 用值副本 §56.33 修复形态）:\n%s",
			strings.Join(violations, "\n"))
	}
}

// extractCallArg 提取 call( 参数文本（到右括号或行尾——链式调用中参数
// 不跨行·修剪空白）。
func extractCallArg(line, call string) string {
	idx := strings.Index(line, call)
	if idx < 0 {
		return ""
	}
	rest := line[idx+len(call):]
	// 截到闭括号或行尾
	if end := strings.Index(rest, ")"); end >= 0 {
		rest = rest[:end]
	}
	return strings.TrimSpace(rest)
}

func itoa17(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa17(n/10) + string(rune('0'+n%10))
}
