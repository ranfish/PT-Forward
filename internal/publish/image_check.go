// Package publish 图床兼容性检查 + 截图失效检测（§56.17 决策 4 + Q7）。
package publish

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

// 分级失效检测时间阈值（§56.17 Q7）。
const (
	ScreenshotFreshThreshold = 5 * time.Minute  // ≤5min: 直接复用
	ScreenshotRecentThreshold = 24 * time.Hour  // 5min~24h: HEAD 检查
	// >24h: 重新上传
)

// CheckImageCompatibility 检查截图 URL 是否兼容目标站（§56.17 决策 4）。
// 返回：(兼容的 URL, 需要转存的 URL)
//   - 白名单制度：URL host 在白名单内 → compatible，否则 → incompatible
//   - 黑名单制度：URL host 在黑名单内 → incompatible，否则 → compatible
//   - 无限制：全部 compatible
func CheckImageCompatibility(screenshotURLs []string, targetSite *model.Site) (compatible, incompatible []string) {
	if targetSite == nil {
		return screenshotURLs, nil
	}
	whitelist := parseHostList(targetSite.ImageWhitelist)
	blacklist := parseHostList(targetSite.ImageBlacklist)

	for _, u := range screenshotURLs {
		host := extractHost(u)
		// 白名单制度优先
		if len(whitelist) > 0 {
			if isHostInList(host, whitelist) {
				compatible = append(compatible, u)
			} else {
				incompatible = append(incompatible, u)
			}
			continue
		}
		// 黑名单制度
		if len(blacklist) > 0 {
			if isHostInList(host, blacklist) {
				incompatible = append(incompatible, u)
			} else {
				compatible = append(compatible, u)
			}
			continue
		}
		// 无限制
		compatible = append(compatible, u)
	}
	return compatible, incompatible
}

// ResolveScreenshots §56.17 Q7: 分级失效检测。
//   ≤5min: 直接复用（本次任务刚上传）
//   5min~24h: HEAD 检查第 1 张可达性
//   >24h: 重新上传（返回空切片，调用方触发上传）
//   cached 为空: 直接上传
func ResolveScreenshots(ctx context.Context, cached []string, generatedAt time.Time, logger *zap.Logger) (reuse bool) {
	if len(cached) == 0 {
		return false // 无缓存，需上传
	}
	age := time.Since(generatedAt)
	switch {
	case age <= ScreenshotFreshThreshold:
		// 直接复用，无 HEAD 检查
		return true
	case age <= ScreenshotRecentThreshold:
		// HEAD 检查第 1 张
		if isURLReachable(ctx, cached[0]) {
			return true
		}
		if logger != nil {
			logger.Info("screenshots expired (HEAD check failed), reuploading",
				zap.Duration("age", age),
				zap.String("url", cached[0]))
		}
		return false
	default:
		// 长期缓存，直接重新上传
		if logger != nil {
			logger.Info("screenshots stale (>24h), reuploading",
				zap.Duration("age", age))
		}
		return false
	}
}

// isURLReachable HEAD 请求检查 URL 可达性（5s 超时）。
func isURLReachable(ctx context.Context, rawURL string) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "HEAD", rawURL, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// extractHost 从 URL 提取 host（小写）。
func extractHost(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Host)
}

// parseHostList 解析逗号分隔的 host 列表。
func parseHostList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var hosts []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(strings.ToLower(part))
		if part != "" {
			hosts = append(hosts, part)
		}
	}
	return hosts
}

// isHostInList 判断 host 是否在列表中（支持子域名匹配）。
func isHostInList(host string, list []string) bool {
	if host == "" {
		return false
	}
	for _, item := range list {
		if host == item || strings.HasSuffix(host, "."+item) {
			return true
		}
	}
	return false
}
