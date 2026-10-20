package httpclient

import (
	"context"
	"net"
	"sync"
	"time"
)

// §59.334: 智能双栈拨号——并发竞速 + 健康缓存
//
// 案例（243 keepfrds）：Cloudflare 4 个边缘 IP 质量两极分化且动态变化
// （v4 104.21 间歇通 / v4 172.67 断 / v6 ac43 间歇 / v6 6815 通），
// 原 dialContextIPv4First 只取每族第一个 IP 串行尝试——大概率碰到
// 坏节点后超时。
//
// 设计（A+B，用户定案）：
//   A 并发竞速：所有 DNS 返回 IP（v4+v6）同时拨号，谁先成功用谁
//   B 健康缓存：记住最近成功的 IP（5min TTL），后续直连零开销
//
// 取代 dialContextIPv4First（§59.319 附二十四）——串行 v4 优先废止。

// healthyEntry §59.334: 健康缓存条目
type healthyEntry struct {
	ip     string
	at     time.Time
}

var (
	healthyMu    sync.RWMutex
	healthyCache = map[string]healthyEntry{} // host → 最近成功 IP
)

const healthyTTL = 5 * time.Minute

// dialContextSmart §59.334: 智能拨号入口
func dialContextSmart(dial func(ctx context.Context, network, addr string) (net.Conn, error)) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return dial(ctx, network, addr)
		}
		// IP 字面量直通
		if ip := net.ParseIP(host); ip != nil {
			return dial(ctx, network, addr)
		}

		// ① B：健康缓存——直连最近成功 IP（零额外开销）
		healthyMu.RLock()
		entry, ok := healthyCache[host]
		healthyMu.RUnlock()
		if ok && time.Since(entry.at) < healthyTTL {
			conn, err := dial(ctx, "tcp", net.JoinHostPort(entry.ip, port))
			if err == nil {
				return conn, nil
			}
			// 缓存 IP 失败——清除，继续竞速
			healthyMu.Lock()
			delete(healthyCache, host)
			healthyMu.Unlock()
		}

		// ② DNS 解析全部 IP
		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil || len(addrs) == 0 {
			return dial(ctx, network, addr) // 解析失败退回默认行为
		}

		// 单地址直拨
		if len(addrs) == 1 {
			conn, err := dial(ctx, "tcp", net.JoinHostPort(addrs[0].IP.String(), port))
			if err == nil {
				rememberHealthy(host, addrs[0].IP.String())
			}
			return conn, err
		}

		// ③ A：并发竞速——所有 IP 同时拨号，谁先成功用谁
		return raceDial(ctx, host, port, addrs, dial)
	}
}

// raceResult §59.334: 竞速结果
type raceResult struct {
	conn net.Conn
	ip   string
}

// raceDial §59.334 A: 并发拨号所有候选 IP，首个成功返回
func raceDial(ctx context.Context, host, port string, addrs []net.IPAddr,
	dial func(ctx context.Context, network, addr string) (net.Conn, error)) (net.Conn, error) {

	// 竞速超时：外层 ctx 或 5s 上限（取较短的）
	raceTimeout := 5 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < raceTimeout {
			raceTimeout = remaining
		}
	}
	raceCtx, raceCancel := context.WithTimeout(ctx, raceTimeout)
	defer raceCancel()

	resultCh := make(chan raceResult, len(addrs))
	var wg sync.WaitGroup

	for _, addr := range addrs {
		wg.Add(1)
		go func(a net.IPAddr) {
			defer wg.Done()
			conn, err := dial(raceCtx, "tcp", net.JoinHostPort(a.IP.String(), port))
			if err == nil {
				resultCh <- raceResult{conn: conn, ip: a.IP.String()}
			}
		}(addr)
	}

	// 等待首个成功或全部失败
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	select {
	case r, ok := <-resultCh:
		if ok {
			// 成功——更新健康缓存，关闭其他竞争者的连接
			rememberHealthy(host, r.ip)
			go drainLosers(resultCh)
			return r.conn, nil
		}
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: errAllFailed{host: host}}
	case <-raceCtx.Done():
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: errRaceTimeout{host: host}}
	}
}

// drainLosers §59.334: 竞速败者连接回收
func drainLosers(ch <-chan raceResult) {
	for r := range ch {
		if r.conn != nil {
			_ = r.conn.Close()
		}
	}
}

// rememberHealthy §59.334 B: 更新健康缓存
func rememberHealthy(host, ip string) {
	healthyMu.Lock()
	healthyCache[host] = healthyEntry{ip: ip, at: time.Now()}
	healthyMu.Unlock()
}

// errAllFailed / errRaceTimeout 自定义错误
type errAllFailed struct{ host string }
func (e errAllFailed) Error() string { return "all addresses failed for " + e.host }

type errRaceTimeout struct{ host string }
func (e errRaceTimeout) Error() string { return "race timeout for " + e.host }
