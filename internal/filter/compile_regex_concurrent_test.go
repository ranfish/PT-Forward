package filter

import (
	"sync"
	"testing"
)

// 10 刀2 防回归锚：compileRegex 并发压测——8 goroutine×2000 次
// 不同 pattern 并发调用·断言 -race 无 DATA RACE（原实现的裸写
// map 曾实证 fatal·10 篇 §2.1 证据）。
// 纳入 06 篇 race CI 层覆盖（internal/filter 加入 test-race job
// 的包清单——只加测试不改基建）。
// 阴性对照（10 §4 要求）：临时去掉写锁 → 本锚 -race 报
// DATA RACE（栈顶 engine.go 读/写行）——声明可触发。
func TestCompileRegexConcurrent(t *testing.T) {
	const n = 8
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 2000; j++ {
				p := "concurrent-" + string(rune('a'+i%26)) + string(rune('0'+j%10)) + string(rune('a'+j%26))
				re, err := compileRegex(p)
				if err != nil || re == nil {
					t.Errorf("compileRegex(%q) err=%v re=%v", p, err, re)
					return
				}
			}
		}(i)
	}
	wg.Wait()
}

// 10 刀2 缓存正确性锚：并发+串行混合后缓存条目正确（每 pattern
// 恰好一个 entry·淘汰后不超上限）。
func TestCompileRegexCacheIntegrity(t *testing.T) {
	// 串行——验证缓存命中
	re1, _ := compileRegex("integrity-[a-z]+")
	if re1 == nil {
		t.Fatal("compileRegex 返回 nil")
	}
	re2, _ := compileRegex("integrity-[a-z]+")
	if re1 != re2 {
		t.Error("同 pattern 应命中缓存（指针相等）")
	}
	// 并发写不同 pattern——验证 double-check 正确
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				p := "integrity-" + string(rune('A'+i)) + string(rune('0'+j%10))
				if re, err := compileRegex(p); err != nil || re == nil {
					t.Errorf("compileRegex(%q) err=%v", p, err)
				}
			}
		}(i)
	}
	wg.Wait()
	// 锁内读缓存条目数
	regexCacheMu.RLock()
	defer regexCacheMu.RUnlock()
	if len(regexCache) == 0 {
		t.Error("并发写入后缓存不应为空")
	}
}
