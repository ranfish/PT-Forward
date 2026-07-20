// Package extract 配置加载（v0.0.253 重构：每站一个 JSON + domain 索引）。
//
// 数据源（按优先级）：
//  1. data/sites_source_keys/<domain>.json: 每站独立配置（source_keys + standard_keys）
//     主键 = domain（如 "pterclub.net"），fallback key = site_code（如 "pterclub"）
//  2. data/extended_standard_keys.json: 全局标准化键变体表（所有站共用）
//
// 用法：
//   siteLabel := LookupSiteSourceKey("pterclub.net", "pterclub", "type")  // "类型"
//   stdCode := LookupStandardKey("type", "电视剧")                        // "category.tv_series"
package extract

import (
	"embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed data/sites_source_keys/*.json
var sitesSourceKeysFS embed.FS

//go:embed data/extended_standard_keys.json
var extendedStandardKeysJSON []byte

// siteConfigEntry 单个站点的配置。
type siteConfigEntry struct {
	Domain       string                       `json:"domain"`
	SiteCode     string                       `json:"site_code"`
	SiteName     string                       `json:"site_name"`
	SourceKeys   map[string]string            `json:"source_keys"`
	StandardKeys map[string]map[string]string `json:"standard_keys"`
}

var (
	configLoadOnce sync.Once
	sitesByDomain  map[string]siteConfigEntry // key = domain（小写）
	sitesByCode    map[string]siteConfigEntry // key = site_code（小写，fallback）
	standardKeys   map[string]map[string]string

	defaultSourceKeys = map[string]string{
		"type":        "类型",
		"medium":      "媒介",
		"video_codec": "视频编码",
		"audio_codec": "音频编码",
		"resolution":  "分辨率",
		"team":        "制作组",
		"source":      "产地",
	}
)

func loadConfig() {
	configLoadOnce.Do(func() {
		sitesByDomain = map[string]siteConfigEntry{}
		sitesByCode = map[string]siteConfigEntry{}

		// 加载 data/sites_source_keys/*.json（每站一个文件）
		entries, err := sitesSourceKeysFS.ReadDir("data/sites_source_keys")
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
					continue
				}
				data, err := sitesSourceKeysFS.ReadFile("data/sites_source_keys/" + entry.Name())
				if err != nil {
					continue
				}
				var cfg siteConfigEntry
				if err := json.Unmarshal(data, &cfg); err != nil {
					continue
				}
				if cfg.Domain != "" {
					sitesByDomain[strings.ToLower(strings.TrimSpace(cfg.Domain))] = cfg
				}
				if cfg.SiteCode != "" {
					code := strings.ToLower(strings.TrimSpace(cfg.SiteCode))
					// site_code 可能冲突（多 domain 简写成同 code），仅当不冲突时建立索引
					if _, exists := sitesByCode[code]; !exists {
						sitesByCode[code] = cfg
					}
				}
			}
		}

		// 加载全局 standard_keys
		standardKeys = map[string]map[string]string{}
		_ = json.Unmarshal(extendedStandardKeysJSON, &standardKeys)
	})
}

// LookupSiteSourceKey 按 domain（主）+ site_code（fallback）+ field 查 source_key。
// 找不到时返回 defaultSourceKeys[field]（NexusPHP 通用默认）。
// field: "type" / "medium" / "video_codec" / "audio_codec" / "resolution" / "team" / "source"
//
// v0.0.253: 改为优先按 domain 查找（解决 site_code 冲突，如 20+ 个 pt.* 域名都算 "pt"）。
func LookupSiteSourceKey(domain, siteCode, field string) string {
	loadConfig()
	// 1. 优先按 domain 查（最唯一）
	if domain != "" {
		if cfg, ok := sitesByDomain[strings.ToLower(strings.TrimSpace(domain))]; ok {
			if v, ok := cfg.SourceKeys[field]; ok && v != "" {
				return v
			}
		}
	}
	// 2. fallback 按 site_code 查
	if siteCode != "" {
		if cfg, ok := sitesByCode[strings.ToLower(strings.TrimSpace(siteCode))]; ok {
			if v, ok := cfg.SourceKeys[field]; ok && v != "" {
				return v
			}
		}
	}
	// 3. 默认值
	if d, ok := defaultSourceKeys[field]; ok {
		return d
	}
	return ""
}

// LookupSiteStandardKeys 返回指定站点的 standard_keys 映射表（用于值标准化）。
// 优先返回站点特定映射，没配置时返回 nil（用全局 standard_keys 兜底）。
func LookupSiteStandardKeys(domain, siteCode string) map[string]map[string]string {
	loadConfig()
	if domain != "" {
		if cfg, ok := sitesByDomain[strings.ToLower(strings.TrimSpace(domain))]; ok {
			if len(cfg.StandardKeys) > 0 {
				return cfg.StandardKeys
			}
		}
	}
	if siteCode != "" {
		if cfg, ok := sitesByCode[strings.ToLower(strings.TrimSpace(siteCode))]; ok {
			if len(cfg.StandardKeys) > 0 {
				return cfg.StandardKeys
			}
		}
	}
	return nil
}

// LookupStandardKey 把字段原始值映射到标准键（如 "电视剧 (TV Series)" → "category.tv_series"）。
// category: "type" / "medium" / "video_codec" / "audio_codec" / "resolution" / "team" / "source"
// 返回空串表示未匹配（调用方保留原始值）。
//
// 匹配策略（按优先级）：
//  1. exact match（原始值 == key）
//  2. 包含匹配（原始值包含 key，如 "电视剧 (TV Series)" 包含 "电视剧"）
//     优先匹配较长的 key（更具体）
func LookupStandardKey(category, raw string) string {
	loadConfig()
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	m, ok := standardKeys[category]
	if !ok || len(m) == 0 {
		return ""
	}
	// 1. exact
	if std, ok := m[raw]; ok && std != "" {
		return std
	}
	// 2. 包含匹配（最长 key 优先，避免 "电视" 错配 "电视剧"）
	bestKey := ""
	bestStd := ""
	for k, v := range m {
		if v == "" || len(k) < len(bestKey) {
			continue
		}
		if strings.Contains(raw, k) {
			bestKey = k
			bestStd = v
		}
	}
	return bestStd
}
