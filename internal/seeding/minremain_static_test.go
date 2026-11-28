package seeding

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// §59.594（审计 20 刀2 锚③）: 静态双构造块锚——free_wait.go 的
// Add（dbEntry 构造）与 RecoverOnStartup（freeWaitEntry 构造）都必须
// 出现 MinRemainMin（三处同批配对纪律的源码级锁定）。
// 沿 16/17/18/19 篇惯例：仓库根 Walk+一级目录白名单+注释截断+测试豁免。
//
// 阴性对照（刀 3 C）：删任一构造块的 MinRemainMin 行 → 本锚点名 FAIL。
func TestFreeWaitMinRemainStaticPairing(t *testing.T) {
	root := filepath.Join("..", "seeding")
	const target = "free_wait.go"

	data, err := os.ReadFile(filepath.Join(root, target))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(string(data), "\n")

	// 扫描两个构造块的 MinRemainMin 出现（Add 的 dbEntry + Recover 的 entry）
	addBlockHas := false
	recoverBlockHas := false
	inAddDbEntry := false
	inRecoverEntry := false
	var violations []string

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 构造块开始标记
		if strings.Contains(trimmed, "dbEntry := &model.FreeWaitEntry{") {
			inAddDbEntry = true
			continue
		}
		if strings.Contains(trimmed, "m.pending[key] = &freeWaitEntry{") {
			inRecoverEntry = true
			continue
		}
		// 构造块结束（行首 }）
		if trimmed == "}" {
			inAddDbEntry = false
			inRecoverEntry = false
			continue
		}
		// 块内 MinRemainMin
		if (inAddDbEntry || inRecoverEntry) && strings.Contains(trimmed, "MinRemainMin") {
			if inAddDbEntry {
				addBlockHas = true
			}
			if inRecoverEntry {
				recoverBlockHas = true
			}
		}
		_ = i
	}

	if !addBlockHas {
		violations = append(violations, target+" Add 的 dbEntry 构造块缺 MinRemainMin（DB 侧写入被删）")
	}
	if !recoverBlockHas {
		violations = append(violations, target+" RecoverOnStartup 的 freeWaitEntry 构造块缺 MinRemainMin（回填被删——重启归零根因）")
	}
	if len(violations) > 0 {
		t.Errorf("MinRemainMin 配对纪律断裂（三处同批·§59.594 刀1）:\n%s",
			strings.Join(violations, "\n"))
	}
}
