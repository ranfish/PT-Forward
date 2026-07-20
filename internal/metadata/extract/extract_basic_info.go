package extract

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// §56.10 决策 3：先硬编码 + TODO 接入 §56.1 standard_keys 表。
// v0.0.238: defaultBasicInfoLabels 作为 fallback，PTNexus 配置（site_config.go）优先。
var defaultBasicInfoLabels = map[string][]string{
	"type":        {"类型", "類型", "category", "type"},
	"medium":      {"媒介", "媒体", "媒體", "medium"},
	"video_codec": {"视频编码", "視頻編碼", "编码", "編碼", "video codec", "codec"},
	"audio_codec": {"音频编码", "音頻編碼", "音频", "音頻", "audio codec", "audio"},
	"resolution":  {"分辨率", "解析度", "resolution"},
	"team":        {"制作组", "製作組", "团队", "團隊", "team"},
	"source":      {"产地", "產地", "地区", "地區", "来源", "來源", "处理", "處理", "source"},
}

// fillBasicInfoFields 从详情页基本信息表填充结构化字段。
// 支持 4 种 HTML 模式（按精准度优先级）：
//  1. td.rowhead + td.rowfollow（NexusPHP 种子信息表标准，PTer/HDSky 等用此模式）
//  2. dt/dd 模式（HTML5 description list）
//  3. th/td 模式（table header + cell）
//  4. td/td 相邻模式（普通表格，限定不在 colhead 表内避免文件列表表头误匹配）
//
// v0.0.238: 按 site_code 加载 PTNexus 移植的 source_key 配置（覆盖默认 label）。
// 字段值经 LookupStandardKey 映射到标准键（如 "电视剧 (TV Series)" → "category.tv_series"）。
func (p *PublicExtractor) fillBasicInfoFields(doc *goquery.Document, seed *SeedData) {
	// 每个字段的候选 labels：site-specific source_key 优先 + default 变体
	fieldLabels := buildFieldLabels(p.siteCode)
	values := map[string]string{}

	// 模式 1: td.rowhead + td.rowfollow（NexusPHP 种子信息表，最精准）
	doc.Find(`td[class*="rowhead"]`).Each(func(_ int, head *goquery.Selection) {
		headText := strings.TrimSpace(head.Text())
		field := matchFieldWithLabels(headText, fieldLabels)
		if field == "" {
			return
		}
		follow := head.NextFiltered(`td[class*="rowfollow"]`)
		if follow.Length() == 0 {
			follow = head.NextFiltered("td")
		}
		if follow.Length() == 0 {
			return
		}
		value := strings.TrimSpace(follow.Text())
		if value == "" {
			return
		}
		// 排除聚合 td（值太长，含多个字段，让站点提取器单独解析）
		if len(value) > 50 {
			return
		}
		if _, exists := values[field]; !exists {
			values[field] = value
		}
	})

	// 模式 2: dt/dd
	doc.Find("dt").Each(func(_ int, dt *goquery.Selection) {
		dtText := strings.TrimSpace(dt.Text())
		dd := dt.NextFiltered("dd")
		if dd.Length() == 0 {
			return
		}
		ddText := strings.TrimSpace(dd.Text())
		if ddText == "" {
			return
		}
		if field := matchFieldWithLabels(dtText, fieldLabels); field != "" {
			if _, exists := values[field]; !exists {
				values[field] = ddText
			}
		}
	})

	// 模式 3: th/td
	doc.Find("th").Each(func(_ int, th *goquery.Selection) {
		thText := strings.TrimSpace(th.Text())
		td := th.NextFiltered("td")
		if td.Length() == 0 {
			return
		}
		tdText := strings.TrimSpace(td.Text())
		if tdText == "" {
			return
		}
		if field := matchFieldWithLabels(thText, fieldLabels); field != "" {
			if _, exists := values[field]; !exists {
				values[field] = tdText
			}
		}
	})

	// 模式 4: td/td 相邻（普通表格 fallback）
	doc.Find("td").Each(func(_ int, td *goquery.Selection) {
		tdText := strings.TrimSpace(td.Text())
		if tdText == "" {
			return
		}
		cls, _ := td.Attr("class")
		if strings.Contains(cls, "colhead") {
			return
		}
		field := matchFieldWithLabels(tdText, fieldLabels)
		if field == "" {
			return
		}
		if td.Find("td").Length() > 0 {
			return
		}
		nextTD := td.NextFiltered("td")
		if nextTD.Length() == 0 {
			return
		}
		nextCls, _ := nextTD.Attr("class")
		if strings.Contains(nextCls, "colhead") {
			return
		}
		value := strings.TrimSpace(nextTD.Text())
		if value == "" {
			return
		}
		if nextTD.Find("td").Length() > 0 {
			return
		}
		if len(value) > 50 {
			return
		}
		if _, exists := values[field]; !exists {
			values[field] = value
		}
	})

	// 填充 seed + 标准化字段值
	if v, ok := values["type"]; ok {
		seed.Type = standardizeFieldValue("type", v)
	}
	if v, ok := values["medium"]; ok {
		seed.Medium = standardizeFieldValue("medium", v)
	}
	if v, ok := values["video_codec"]; ok {
		seed.VideoCodec = standardizeFieldValue("video_codec", v)
	}
	if v, ok := values["audio_codec"]; ok {
		seed.AudioCodec = standardizeFieldValue("audio_codec", v)
	}
	if v, ok := values["resolution"]; ok {
		seed.Resolution = standardizeFieldValue("resolution", v)
	}
	if v, ok := values["team"]; ok {
		seed.ReleaseGroup = standardizeFieldValue("team", v)
	}
	if v, ok := values["source"]; ok {
		seed.Source = standardizeFieldValue("source", v)
	}
}

// buildFieldLabels 构建每个字段的候选 label 列表（site-specific source_key 优先 + default 变体）。
// 返回 map：field → 候选 labels 切片。
func buildFieldLabels(siteCode string) map[string][]string {
	result := map[string][]string{}
	for field := range defaultBasicInfoLabels {
		labels := []string{}
		// 1. site-specific source_key（最优先）
		if sk := LookupSiteSourceKey(siteCode, field); sk != "" {
			labels = append(labels, sk)
		}
		// 2. default 变体
		labels = append(labels, defaultBasicInfoLabels[field]...)
		// 去重
		seen := map[string]bool{}
		deduped := labels[:0]
		for _, l := range labels {
			if !seen[l] {
				seen[l] = true
				deduped = append(deduped, l)
			}
		}
		result[field] = deduped
	}
	return result
}

// matchFieldWithLabels 用指定 labels 判断标签文本是否匹配某字段。
func matchFieldWithLabels(label string, fieldLabels map[string][]string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return ""
	}
	for field, candidates := range fieldLabels {
		for _, c := range candidates {
			if label == c {
				return field
			}
		}
	}
	// partial：去掉结尾冒号后再匹配
	trimmed := strings.TrimRight(label, ":：")
	for field, candidates := range fieldLabels {
		for _, c := range candidates {
			if trimmed == c {
				return field
			}
		}
	}
	return ""
}

// standardizeFieldValue 把字段原始值映射到标准键（如 "电视剧 (TV Series)" → "category.tv_series"）。
// 找不到映射时返回原值（保留可读性）。
func standardizeFieldValue(category, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if std := LookupStandardKey(category, raw); std != "" {
		return std
	}
	return raw
}

// matchBasicInfoLabel 兼容旧调用（已弃用，新代码用 matchFieldWithLabels）。
func matchBasicInfoLabel(label string) string {
	return matchFieldWithLabels(label, buildFieldLabels(""))
}
