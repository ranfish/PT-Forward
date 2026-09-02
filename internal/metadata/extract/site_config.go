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
	"fmt"
	"strconv"
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
	// §59.146: 表单值映射（发布链 TagApplier 消费）——域→{standard_key→站表单值}
	// 39 站 tag 域（tag.国语→yes/数字）、36 站 team 域。数值统一字符串化。
	FormValueMappings map[string]map[string]interface{} `json:"form_value_mappings,omitempty"`
	// v0.0.254: 站点特殊提取规则（替代 Go 代码的特殊提取器）
	Extractors *siteExtractors `json:"extractors,omitempty"`
}

// siteExtractors 站点特殊提取规则（按需启用，全部可选）。
// 设计原则：每个字段对应一种"无法用通用 selector 表达的 DOM 解析"。
type siteExtractors struct {
	// CategoryFromIcons 从 rowfollow 内 <img alt> 提取字段（PTer 模式）。
	// container_text: rowhead 文本（如"类别与标签"），限定查找范围
	// alt_to_field: alt 文本 → 字段名（"type"/"medium"/"source"/...），值再走 standard_keys 标准化
	CategoryFromIcons *categoryIconsConfig `json:"category_from_icons,omitempty"`

	// TitleFromQuoted 从带引号的 <title> 提取标题（HHanClub 模式）。
	// quoted_pattern: 优先匹配引号内标题的正则
	// fallback_pattern: fallback 正则
	// strip_suffix: 要去除的后缀列表（如 " :: "）
	TitleFromQuoted *titleQuotedConfig `json:"title_from_quoted,omitempty"`

	// DescriptionFromRange 从 HTML 区间提取简介（HHanClub 模式）。
	// start_pattern: 简介 start 标记的正则
	// end_pattern: 简介 end 标记的正则
	DescriptionFromRange *descriptionRangeConfig `json:"description_from_range,omitempty"`

	// BasicInfoDivLabel 从 div 标签提取字段（HHanClub Tailwind 模式）。
	// label_selector: 字段标签的 selector（如 "div[class*='font-bold'][class*='leading-6']"）
	// value_getter: 值获取方式，目前支持 "next_sibling"（标签的下一个兄弟元素）
	BasicInfoDivLabel *basicInfoDivLabelConfig `json:"basic_info_div_label,omitempty"`

	// PosterFromPattern 从 URL 正则提取海报（HHanClub l_ratio_poster 模式）。
	PosterFromPattern string `json:"poster_from_pattern,omitempty"`
}

type categoryIconsConfig struct {
	ContainerText string            `json:"container_text"` // 如 "类别与标签"
	AltToField    map[string]string `json:"alt_to_field"`   // alt → "type"/"medium"/"source"/...
}

type titleQuotedConfig struct {
	QuotedPattern   string   `json:"quoted_pattern"`
	FallbackPattern string   `json:"fallback_pattern"`
	StripSuffix     []string `json:"strip_suffix"`
}

type descriptionRangeConfig struct {
	StartPattern string `json:"start_pattern"`
	EndPattern   string `json:"end_pattern"`
}

type basicInfoDivLabelConfig struct {
	LabelSelector string `json:"label_selector"` // 如 "div[class*='font-bold'][class*='leading-6']"
	ValueGetter   string `json:"value_getter"`   // "next_sibling" / "next_span_pair" / "container_label_grid_span"
	// v0.0.254 增：用于 HHanClub 新版基本信息 grid 模式
	// 当 ValueGetter="container_label_grid_span" 时：
	//   - 找 LabelSelector 文本 == ContainerLabel 的元素
	//   - 取下一个 grid 容器
	//   - 遍历内部 div，每个 div 内 span.{LabelClass} 是字段标签，下一个 span 是值
	ContainerLabel string `json:"container_label,omitempty"` // 如 "基本信息"
	LabelClass     string `json:"label_class,omitempty"`     // 如 "font-bold"
}

var (
	configLoadOnce sync.Once
	sitesByDomain  map[string]siteConfigEntry // key = domain（小写）
	sitesByCode    map[string]siteConfigEntry // key = site_code（小写，fallback）
	standardKeys   map[string]map[string]string
	standardKeyEntries map[string][]standardKeyEntry

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
		// §59.164: entries 索引（LookupStandardKeyLoose 的 aliases 匹配——
		// json 到 map 的加载丢 aliases，重建结构化索引）
		standardKeyEntries = map[string][]standardKeyEntry{}
		var raw []struct {
			Category string   `json:"category"`
			Key      string   `json:"key"`
			Code     string   `json:"code"`
			Aliases  []string `json:"aliases"`
		}
		if err := json.Unmarshal(extendedStandardKeysJSON, &raw); err == nil {
			for _, e := range raw {
				standardKeyEntries[e.Category] = append(standardKeyEntries[e.Category],
					standardKeyEntry{Key: e.Key, Code: e.Code, Aliases: e.Aliases})
			}
		}
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

// LookupSiteExtractors 返回站点特殊提取规则（v0.0.254）。
// 没配置时返回 nil（走 PublicExtractor 标准流程）。
func LookupSiteExtractors(domain, siteCode string) *siteExtractors {
	loadConfig()
	if domain != "" {
		if cfg, ok := sitesByDomain[strings.ToLower(strings.TrimSpace(domain))]; ok {
			return cfg.Extractors
		}
	}
	if siteCode != "" {
		if cfg, ok := sitesByCode[strings.ToLower(strings.TrimSpace(siteCode))]; ok {
			return cfg.Extractors
		}
	}
	return nil
}

// LookupFormValueMappings §59.146: 查站点表单值映射（域→{standard_key→站值字符串}）。
// 数值统一 String 化（BTSchool span 数字值等）。站点无该域时返回 nil。
func LookupFormValueMappings(domain, siteCode string) map[string]map[string]string {
	loadConfig()
	var cfg siteConfigEntry
	var ok bool
	if domain != "" {
		cfg, ok = sitesByDomain[strings.ToLower(strings.TrimSpace(domain))]
	}
	if !ok && siteCode != "" {
		cfg, ok = sitesByCode[strings.ToLower(strings.TrimSpace(siteCode))]
	}
	if !ok || len(cfg.FormValueMappings) == 0 {
		return nil
	}
	out := make(map[string]map[string]string, len(cfg.FormValueMappings))
	for domain, m := range cfg.FormValueMappings {
		vals := make(map[string]string, len(m))
		for k, v := range m {
			switch tv := v.(type) {
			case string:
				vals[k] = tv
			case float64:
				if tv == float64(int64(tv)) {
					vals[k] = strconv.FormatInt(int64(tv), 10)
				} else {
					vals[k] = strconv.FormatFloat(tv, 'f', -1, 64)
				}
			case bool:
				if tv {
					vals[k] = "yes"
				} else {
					vals[k] = "no"
				}
			default:
				vals[k] = fmt.Sprintf("%v", v)
			}
		}
		out[domain] = vals
	}
	return out
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

// standardKeyEntry §59.164: 词条结构化（key/code/aliases——Loose 匹配用）
type standardKeyEntry struct {
	Key     string
	Code    string
	Aliases []string
}

// LookupStandardKeyLoose 宽松标准键匹配（§59.164 form_config parse 词条补
// StandardKeys 专用——站方 label 形态各异："4K/2160i/2160P"/"电影（Movies）"）。
// LookupStandardKey 基础上追加：①大小写不敏感包含 ②aliases 匹配。
func LookupStandardKeyLoose(category, raw string) string {
	if std := LookupStandardKey(category, raw); std != "" {
		return std
	}
	rawLower := strings.ToLower(strings.TrimSpace(raw))
	if rawLower == "" {
		return ""
	}
	loadConfig()
	bestLen, best := 0, ""
	for k, v := range standardKeys[category] {
		if v == "" {
			continue
		}
		if len(k) > bestLen && strings.Contains(rawLower, strings.ToLower(k)) {
			bestLen, best = len(k), v
		}
	}
	if best != "" {
		return best
	}
	bestALen, bestA := 0, ""
	for _, e := range standardKeyEntries[category] {
		for _, a := range e.Aliases {
			if len(a) > bestALen && strings.Contains(rawLower, strings.ToLower(a)) {
				bestALen, bestA = len(a), e.Code
			}
		}
	}
	return bestA
}
