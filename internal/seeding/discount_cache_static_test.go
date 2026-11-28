package seeding

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// §59.592（审计 19 刀2）: 静态锚——全库生产源码禁止绕过 discountCacheKey
// 的裸 `discountCache[` 索引形态（读写必须经 get/set 单点→键经 helper）。
// 合法位置=flush.go（helper 与 get/set 实现所在文件·集合锁 3 处：
// get 索引+get delete+set 写入）；其它任何文件出现即违规。
//
// 沿用 16/17/18 篇静态锚惯例：仓库根 Walk+一级目录白名单（internal/cmd/
// tools）+注释截断+测试文件豁免。
//
// 阴性对照（刀3 C）：向非 flush.go 生产文件注入 `e.discountCache[x]`
// 形态 → 本锚点名 FAIL。
func TestNoBareDiscountCacheIndex(t *testing.T) {
	root := filepath.Join("..", "..")
	const legalFile = "internal/seeding/flush.go"
	const legalCount = 2 // helper 实现内：get 索引+set 写入（delete 是函数调用非索引形态）

	legalHits := 0
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
			if !strings.Contains(line, "discountCache[") {
				continue
			}
			if rel == legalFile {
				legalHits++
				continue
			}
			violations = append(violations, rel+":"+itoa19(i+1)+" "+strings.TrimSpace(line))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if legalHits != legalCount {
		t.Errorf("flush.go 内 discountCache[ 索引计数=%d want %d（helper 实现集合锁——新增合法索引须同步更新 legalCount 并说明用途）", legalHits, legalCount)
	}
	if len(violations) > 0 {
		t.Errorf("发现绕过 discountCacheKey 的裸 discountCache[ 索引（§59.592 复合键单点：读写必须经 get/set→helper·跨站隔离 V-01 根修形态）:\n%s",
			strings.Join(violations, "\n"))
	}
}

func itoa19(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa19(n/10) + string(rune('0'+n%10))
}
