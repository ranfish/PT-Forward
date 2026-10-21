package browser

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"go.uber.org/zap"
)

// §59.336: 浏览器 fallback 获取器——真实 Chromium 加载页面，
// 绕过 CF 对非浏览器 TLS/H2 指纹的详情页检测（keepfrds 案实证：
// azuretls Chrome 首页/RSS/stats 能通但详情页被拒；Playwright +
// Cookie 一次通过 46KB 完整页面）。
//
// 架构（用户定案："如 MPV 一样在 Docker 里内置浏览器"）：
//   Docker 内嵌 Chromium → chromedp 控制 → 按需启动用完即关
//   站点级开关 browser_fallback（通用能力非 keepfrds 专属）

// Fetcher §59.336: 浏览器获取器
type Fetcher struct {
	logger *zap.Logger
}

// NewFetcher 创建浏览器获取器
func NewFetcher(logger *zap.Logger) *Fetcher {
	return &Fetcher{logger: logger}
}

// FetchHTML 用 Chromium 加载页面并返回完整 HTML。
// cookie 格式：与 PT-Forward 站点 cookie 一致（"k=v; k2=v2"）。
func (f *Fetcher) FetchHTML(ctx context.Context, url, cookie, userAgent string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if userAgent == "" {
		userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent(userAgent),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	browserCtx, timeoutCancel := context.WithTimeout(browserCtx, timeout)
	defer timeoutCancel()

	var html string
	tasks := chromedp.Tasks{
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.OuterHTML("html", &html),
	}
	if cookie != "" {
		tasks = prependCookieTask(url, cookie, tasks)
	}
	if err := chromedp.Run(browserCtx, tasks); err != nil {
		return "", fmt.Errorf("browser navigate: %w", err)
	}

	f.logger.Debug("browser fetch success",
		zap.String("url", url),
		zap.Int("html_len", len(html)))
	return html, nil
}

// prependCookieTask 在导航前插入 cookie 设置任务：
// NexusPHP 登录态 cookie（c_secure_*）必须在首个请求就带上，
// 否则详情页 302 到 login.php。做法：导航到同域轻量路径 →
// CDP 设置 cookie → 后续真实导航自动携带。
func prependCookieTask(rawURL, cookie string, next chromedp.Tasks) chromedp.Tasks {
	domain := extractDomain(rawURL)
	scheme := "https"
	if strings.HasPrefix(rawURL, "http://") {
		scheme = "http"
	}
	var params []*network.CookieParam
	for _, pair := range strings.Split(cookie, ";") {
		kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
		if len(kv) == 2 {
			params = append(params, &network.CookieParam{
				Name:   kv[0],
				Value:  kv[1],
				Domain: domain,
				Path:   "/",
			})
		}
	}
	pre := chromedp.Tasks{
		chromedp.Navigate(scheme + "://" + domain + "/favicon.ico"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return network.SetCookies(params).Do(ctx)
		}),
	}
	return append(pre, next...)
}

// extractDomain 从 URL 提取域名（去 scheme/路径/端口）
func extractDomain(rawURL string) string {
	s := strings.TrimPrefix(rawURL, "https://")
	s = strings.TrimPrefix(s, "http://")
	if idx := strings.Index(s, "/"); idx > 0 {
		s = s[:idx]
	}
	if idx := strings.LastIndex(s, ":"); idx > 0 {
		s = s[:idx]
	}
	return s
}
