package site

import (
	"net/url"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/gorm"
)

// TrackerMatcher 从种子 tracker URL 匹配站点名。
// 移植自 PTNexus refreshSiteMatcher（refresh_sync.go:507）。
// 匹配策略：
//  1. 精确域名匹配（hostMap[完整域名] → 站点名）
//   2. 核心域名匹配（uniqueCoreMap[核心域名] → 站点名，仅保留唯一映射）
type TrackerMatcher struct {
	hostMap       map[string]string
	uniqueCoreMap map[string]string
}

func NewTrackerMatcher(db *gorm.DB) *TrackerMatcher {
	m := &TrackerMatcher{
		hostMap:       map[string]string{},
		uniqueCoreMap: map[string]string{},
	}

	var sites []model.Site
	if err := db.Where("enabled = ? AND domain != ''", true).Find(&sites).Error; err != nil {
		return m
	}

	coreMap := map[string]string{}
	for _, s := range sites {
		if s.Name == "" || s.Domain == "" {
			continue
		}
		candidates := []string{s.Domain}
		if s.BaseURL != "" {
			if u, err := url.Parse(s.BaseURL); err == nil && u.Hostname() != "" {
				candidates = append(candidates, u.Hostname())
			}
		}
		for _, host := range candidates {
			host = normalizeTrackerHost(host)
			if host == "" {
				continue
			}
			if _, exists := m.hostMap[host]; !exists {
				m.hostMap[host] = s.Name
			}
			core := extractTrackerCoreDomain(host)
			if core != "" {
				if existing, exists := coreMap[core]; exists {
					if existing != s.Name {
						coreMap[core] = ""
					}
				} else {
					coreMap[core] = s.Name
				}
			}
		}
	}
	for core, name := range coreMap {
		if name != "" {
			m.uniqueCoreMap[core] = name
		}
	}
	return m
}

func (m *TrackerMatcher) Match(trackerURL string) string {
	host := parseTrackerHost(trackerURL)
	if host == "" {
		return ""
	}
	if name, ok := m.hostMap[host]; ok {
		return name
	}
	core := extractTrackerCoreDomain(host)
	if name, ok := m.uniqueCoreMap[core]; ok {
		return name
	}
	return ""
}

// MatchAll 遍历多个 tracker URL，返回所有匹配到的站点名（去重，保持顺序）。
// v0.0.265: 用于 publish/torrents 列表展示"做种站点"列（一个种子可能在多站做种）。
func (m *TrackerMatcher) MatchAll(trackerURLs []string) []string {
	if len(trackerURLs) == 0 {
		return nil
	}
	seen := map[string]bool{}
	result := []string{}
	for _, u := range trackerURLs {
		name := m.Match(u)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		result = append(result, name)
	}
	return result
}

func parseTrackerHost(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		if u, err := url.Parse(trimmed); err == nil {
			return normalizeTrackerHost(u.Hostname())
		}
	}
	for _, prefix := range []string{"udp://", "ws://", "wss://"} {
		trimmed = strings.TrimPrefix(trimmed, prefix)
	}
	if idx := strings.Index(trimmed, "/"); idx > 0 {
		trimmed = trimmed[:idx]
	}
	if idx := strings.Index(trimmed, ":"); idx > 0 {
		trimmed = trimmed[:idx]
	}
	return normalizeTrackerHost(trimmed)
}

func normalizeTrackerHost(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	return strings.TrimPrefix(h, "www.")
}

func extractTrackerCoreDomain(host string) string {
	cleaned := normalizeTrackerHost(host)
	if cleaned == "" {
		return ""
	}
	for _, prefix := range []string{"tracker.", "kp.", "pt.", "t.", "ipv4.", "ipv6.", "on.", "daydream."} {
		cleaned = strings.TrimPrefix(cleaned, prefix)
	}
	parts := strings.Split(cleaned, ".")
	if len(parts) == 0 {
		return ""
	}
	if len(parts) >= 3 {
		last := parts[len(parts)-1]
		prev := parts[len(parts)-2]
		if len(last) <= 3 && len(prev) <= 3 {
			return parts[len(parts)-3] + "." + prev + "." + last
		}
	}
	if len(parts) > 1 {
		return parts[len(parts)-2] + "." + parts[len(parts)-1]
	}
	return parts[0]
}
