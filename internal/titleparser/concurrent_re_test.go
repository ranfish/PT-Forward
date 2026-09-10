package titleparser

import (
	"sync"
	"testing"
)

// §59.186: 冷启动并发首写回归——tokenReC2/inferReCache 无锁时代
// 双 goroutine 并发编译曾 fatal（sync.Map 修复）。-race 下此测试可复现旧雷。
func TestTokenReConcurrentColdStart(t *testing.T) {
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, tok := range DictTokens("audio") {
				_ = tok.re()
			}
			for _, tok := range DictTokens("platform") {
				_ = tok.re()
			}
			_ = compileInferRe(`\bflac\b`)
		}()
	}
	wg.Wait()
}
