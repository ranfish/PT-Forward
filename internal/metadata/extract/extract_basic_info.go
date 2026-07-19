package extract

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// §56.10 决策 3：先硬编码 + TODO 接入 §56.1 standard_keys 表。
var defaultBasicInfoLabels = map[string][]string{
	"type":        {"类型", "類型", "category", "type"},
	"medium":      {"媒介", "媒体", "媒體", "medium"},
	"video_codec": {"视频编码", "視頻編碼", "编码", "編碼", "video codec", "codec"},
	"audio_codec": {"音频编码", "音頻編碼", "音频", "音頻", "audio codec", "audio"},
	"resolution":  {"分辨率", "解析度", "resolution"},
	"team":        {"制作组", "製作組", "团队", "團隊", "team"},
	"source":      {"产地", "產地", "地区", "地區", "来源", "來源", "处理", "處理", "source"},
}

// TODO: §56.1 standard_keys 表 DB 驱动后替换此 const

// fillBasicInfoFields 从详情页基本信息表填充结构化字段。
// 支持 3 种 HTML 模式（参考 PTNexus findDetailValueCellByLabels）：
//  1. dt/dd 模式（HTML5 description list）
//  2. th/td 模式（table header + cell）
//  3. td/td 相邻模式（NexusPHP 标准种子信息表，如 <tr><td>类型</td><td>电视剧</td>...</tr>）
//
// 第 3 种是 PTer/HDSky/HHanClub 等 NexusPHP 站的主流模式，缺失会导致
// medium/video_codec/resolution/release_group 全空（完整度从 80%+ 跌到 50%）。
func (p *PublicExtractor) fillBasicInfoFields(doc *goquery.Document, seed *SeedData) {
	values := map[string]string{}

	// 模式 1: dt/dd
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
		if field := matchBasicInfoLabel(dtText); field != "" {
			if _, exists := values[field]; !exists {
				values[field] = ddText
			}
		}
	})

	// 模式 2: th/td
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
		if field := matchBasicInfoLabel(thText); field != "" {
			if _, exists := values[field]; !exists {
				values[field] = tdText
			}
		}
	})

	// 模式 3: td/td 相邻（NexusPHP 标准，参考 PTNexus review_extract.go:999）
	// 遍历所有 td，如果文本匹配 label，取下一个兄弟 td 的文本作为值。
	doc.Find("td").Each(func(_ int, td *goquery.Selection) {
		tdText := strings.TrimSpace(td.Text())
		if tdText == "" {
			return
		}
		field := matchBasicInfoLabel(tdText)
		if field == "" {
			return
		}
		// 仅当当前 td 没有子 td 时才视为"标签 td"（避免父容器误匹配）
		if td.Find("td").Length() > 0 {
			return
		}
		nextTD := td.NextFiltered("td")
		if nextTD.Length() == 0 {
			return
		}
		value := strings.TrimSpace(nextTD.Text())
		if value == "" {
			return
		}
		// 值 td 也不能含子 td（否则是嵌套表格）
		if nextTD.Find("td").Length() > 0 {
			return
		}
		if _, exists := values[field]; !exists {
			values[field] = value
		}
	})

	if v, ok := values["type"]; ok {
		seed.Type = v
	}
	if v, ok := values["medium"]; ok {
		seed.Medium = v
	}
	if v, ok := values["video_codec"]; ok {
		seed.VideoCodec = v
	}
	if v, ok := values["audio_codec"]; ok {
		seed.AudioCodec = v
	}
	if v, ok := values["resolution"]; ok {
		seed.Resolution = v
	}
	if v, ok := values["team"]; ok {
		seed.ReleaseGroup = v
	}
	if v, ok := values["source"]; ok {
		seed.Source = v
	}
}

// matchBasicInfoLabel 判断标签文本是否匹配某个字段（exact 或 partial）。
func matchBasicInfoLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return ""
	}
	for field, candidates := range defaultBasicInfoLabels {
		for _, c := range candidates {
			if label == c {
				return field
			}
		}
	}
	// partial：去掉结尾冒号后再匹配
	trimmed := strings.TrimRight(label, ":：")
	for field, candidates := range defaultBasicInfoLabels {
		for _, c := range candidates {
			if trimmed == c {
				return field
			}
		}
	}
	return ""
}
