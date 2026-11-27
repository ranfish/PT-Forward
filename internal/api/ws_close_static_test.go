package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 18 刀3（v0.0.1671）: 静态锚——全库生产源码禁止 `close(x.send)` 形态
// （V-01 根修后 send 永不被关·close 责任收口到 done/shutdown——
// 唯一 owner 是 Hub.shutdown）。防新增代码重犯 send-on-closed。
// 注释行截断防误报（16 篇同款）；测试文件豁免。
func TestNoBareCloseSendChannel(t *testing.T) {
	root := filepath.Join("..", "..")
	var violations []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
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
		for i, line := range strings.Split(string(data), "\n") {
			if idx := strings.Index(line, "//"); idx >= 0 {
				line = line[:idx]
			}
			// 指纹：close( ... .send) 形态
			if strings.Contains(line, "close(") && strings.Contains(line, ".send)") {
				violations = append(violations, rel+":"+itoa18(i+1)+" "+strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(violations) > 0 {
		t.Errorf("发现 close(x.send) 禁用形态（18 刀1：send 永不被关·close 责任在 Hub.shutdown/done——send-on-closed panic 根修）:\n%s",
			strings.Join(violations, "\n"))
	}
}

func itoa18(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa18(n/10) + string(rune('0'+n%10))
}
