package seeding

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// §59.594（审计 20 刀2 锚③）: 静态三构造块锚——free_wait.go 的
// Add 内存 entry、Add 的 dbEntry、RecoverOnStartup 的 freeWaitEntry
// 三处构造都必须出现 MinRemainMin（三处同批配对纪律的源码级锁定）。
// 沿 16/17/18/19 篇惯例：注释截断+块边界 } 判定。
// 〔复审补强：原锚漏锁 Add 内存块（:105）——该行虽是修复前已有代码·
// 本次未动·误删风险低，但锚既然锁定配对纪律就应三块全覆盖〕
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

	// 扫描三个构造块的 MinRemainMin 出现
	addMemHas := false
	addDbHas := false
	recoverHas := false
	inAddMemEntry := false
	inAddDbEntry := false
	inRecoverEntry := false
	var violations []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 构造块开始标记
		if strings.Contains(trimmed, "entry := &freeWaitEntry{") {
			inAddMemEntry = true
			continue
		}
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
			inAddMemEntry = false
			inAddDbEntry = false
			inRecoverEntry = false
			continue
		}
		// 块内 MinRemainMin
		if (inAddMemEntry || inAddDbEntry || inRecoverEntry) && strings.Contains(trimmed, "MinRemainMin") {
			if inAddMemEntry {
				addMemHas = true
			}
			if inAddDbEntry {
				addDbHas = true
			}
			if inRecoverEntry {
				recoverHas = true
			}
		}
	}

	if !addMemHas {
		violations = append(violations, target+" Add 的内存 entry 构造块缺 MinRemainMin（运行时字段被删——CheckOnce 闸门失输入）")
	}
	if !addDbHas {
		violations = append(violations, target+" Add 的 dbEntry 构造块缺 MinRemainMin（DB 侧写入被删）")
	}
	if !recoverHas {
		violations = append(violations, target+" RecoverOnStartup 的 freeWaitEntry 构造块缺 MinRemainMin（回填被删——重启归零根因）")
	}
	if len(violations) > 0 {
		t.Errorf("MinRemainMin 配对纪律断裂（三处同批·§59.594 刀1）:\n%s",
			strings.Join(violations, "\n"))
	}
}
