package httpclient

// 12 刀3（v0.0.1654）: 限流器状态机契约三锚——V-01 域键契约 / V-02 字段
// 竞态 / V-02 取消不阻塞。锚对应 12 篇三处离线驱动探针（复现于
// evidence/domain-limiter-repro.txt·修复前均红）。
//
// 阴性对照（08 篇方法论——锚必须可触发）：
//   - 退 OriginKey 为恒等（去规范化）→ 锚1 FAIL；
//   - 恢复 concurrentCh 热替换（SetDomainConfig 换引用）→ 锚2 -race 报
//     DATA RACE·锚3 卡死超时 FAIL。

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestLimiterDomainKeyContract 12 刀3 锚1: 域键契约——配置写入 key 与请求
// Acquire key 必须同桶（V-01 域键口径分裂防回归：裸 domain 配置对
// https://host 请求失效·站点 rps 配置静默失败〔keepfrds 4/min 跑成
// 30/min=7.5 倍超速〕）。
func TestLimiterDomainKeyContract(t *testing.T) {
	// 形态矩阵：配置侧输入形态 × 请求侧 key 形态——全部须落同一桶。
	forms := []struct {
		name       string
		configKey  string
		acquireKey string
	}{
		{"裸domain→https请求（V-01原案）", "example.com", "https://example.com"},
		{"带scheme→裸domain请求", "https://scheme.example.com", "scheme.example.com"},
		{"带路径→https请求", "path.example.com/torrents", "https://path.example.com"},
		{"带端口→https同端口请求", "port.example.com:8443", "https://port.example.com:8443"},
	}
	for _, f := range forms {
		t.Run(f.name, func(t *testing.T) {
			l := NewDomainRateLimiter(0.5)
			l.SetDomainConfig(f.configKey, DomainLimitConfig{
				MaxReqs:       1, // 1 req/60s——第二次必须被限流
				WindowSecs:    60,
				MaxConcurrent: 2,
			})
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			if err := l.Acquire(ctx, f.acquireKey); err != nil {
				t.Fatalf("第一次 Acquire 不应失败: %v", err)
			}
			l.Release(f.acquireKey)
			// 第二次：若 key 分裂（配置桶≠请求桶），走默认 30/min——立即
			// 成功=配置失效；key 一致则撞 MaxReqs=1 窗口——超时报错。
			err := l.Acquire(ctx, f.acquireKey)
			if err == nil {
				t.Errorf("域键分裂：SetDomainConfig(%q) 未影响 Acquire(%q)——站点 rps 配置失效", f.configKey, f.acquireKey)
			}
		})
	}
}

// TestLimiterConcurrentHotSwapRace 12 刀3 锚2: 并发槽字段竞态——并发
// Acquire/Release 交叉 SetDomainConfig 热替换，-race 下零 DATA RACE
// （V-02 原案：concurrentCh 持锁写 :83 vs 无锁读 :94——-race 实测
// DATA RACE·map 外字段竞态与 10 篇 map fatal 正交）。
func TestLimiterConcurrentHotSwapRace(t *testing.T) {
	l := NewDomainRateLimiter(100)
	l.SetDomainConfig("race.local", DomainLimitConfig{MaxConcurrent: 2})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				if err := l.Acquire(context.Background(), "race.local"); err == nil {
					l.Release("race.local")
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 50; j++ {
			l.SetDomainConfig("race.local", DomainLimitConfig{MaxConcurrent: 4})
			l.SetDomainConfig("race.local", DomainLimitConfig{MaxConcurrent: 2})
		}
	}()
	wg.Wait()
	// -race 层由 CI test-race job 兜底（本包在清单内）——本锚在 -race
	// 跑时若出现锁外替换即报 DATA RACE 红。
}

// TestLimiterCancelDuringHotSwap 12 刀3 锚3: 取消不阻塞——冻结等待中
// 的请求在热替换后取消，必须在超时内返回且槽位计数归零（V-02 取消
// 路径：原实现从替换后的新空 channel 阻塞接收——goroutine 永久卡死
// +幽灵 token 占坑）。
func TestLimiterCancelDuringHotSwap(t *testing.T) {
	l := NewDomainRateLimiter(100)
	l.SetDomainConfig("cancel.local", DomainLimitConfig{MaxConcurrent: 2})
	// 先冻结——Acquire 占坑后卡在冻结等待分支。
	l.Freeze("cancel.local", 10*time.Minute)
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		done <- l.Acquire(ctx, "cancel.local")
	}()
	time.Sleep(50 * time.Millisecond) // 确保 goroutine 已占坑并进入冻结等待
	// 热替换并发能力（触发 broadcast 唤醒路径与在途请求交叉）。
	l.SetDomainConfig("cancel.local", DomainLimitConfig{MaxConcurrent: 4})
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Error("取消后 Acquire 不应成功返回")
		}
	case <-time.After(2 * time.Second):
		t.Error("取消路径永久阻塞（V-02：从新空 channel 阻塞接收）")
	}
	// 归还守卫：取消归还必须落在当前计数（无幽灵占用）。
	l.mu.Lock()
	st := l.domains[OriginKey("cancel.local")]
	l.mu.Unlock()
	st.mu.Lock()
	c := st.concurrent
	st.mu.Unlock()
	if c != 0 {
		t.Errorf("取消后并发计数=%d（应 0——幽灵占用未归还）", c)
	}
}
