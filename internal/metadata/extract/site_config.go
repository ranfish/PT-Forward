// Package extract 配置加载（§56.13 + PTNexus 移植）。
// 从 embed JSON 加载：
//   - sites_source_keys.json: 每站的 source_parsers.source_params.<field>.source_key
//   - extended_standard_keys.json: 全局 + 各站合并的标准键变体表
//
// 用法（PublicExtractor.fillBasicInfoFields）：
//   siteLabel := LookupSiteSourceKey(siteCode, "type")     // e.g. "类型"
//   stdCode := LookupStandardKey("type", "电视剧 (TV Series)") // e.g. "category.tv_series"
package extract

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed data/sites_source_keys.json
var sitesSourceKeysJSON []byte

//go:embed data/extended_standard_keys.json
var extendedStandardKeysJSON []byte

// siteConfigEntry 单个站点的 source_key + standard_keys 配置。
type siteConfigEntry struct {
	SiteName     string                       `json:"site_name"`
	SourceKeys   map[string]string            `json:"source_keys"`
	StandardKeys map[string]map[string]string `json:"standard_keys"`
}

var (
	configLoadOnce sync.Once
	sitesConfig    map[string]siteConfigEntry // key = site_code（如 "pterclub"）
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
		sitesConfig = map[string]siteConfigEntry{}
		var sitesData struct {
			Sites map[string]siteConfigEntry `json:"sites"`
		}
		_ = json.Unmarshal(sitesSourceKeysJSON, &sitesData)
		for k, v := range sitesData.Sites {
			sitesConfig[strings.ToLower(k)] = v
		}

		standardKeys = map[string]map[string]string{}
		_ = json.Unmarshal(extendedStandardKeysJSON, &standardKeys)
	})
}

// LookupSiteSourceKey 按 site_code + field 查 source_key（如 "类型"/"媒介"）。
// 找不到时返回 defaultSourceKeys[field]（NexusPHP 通用默认）。
// field: "type" / "medium" / "video_codec" / "audio_codec" / "resolution" / "team" / "source"
func LookupSiteSourceKey(siteCode, field string) string {
	loadConfig()
	if cfg, ok := sitesConfig[strings.ToLower(strings.TrimSpace(siteCode))]; ok {
		if v, ok := cfg.SourceKeys[field]; ok && v != "" {
			return v
		}
	}
	if d, ok := defaultSourceKeys[field]; ok {
		return d
	}
	return ""
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
