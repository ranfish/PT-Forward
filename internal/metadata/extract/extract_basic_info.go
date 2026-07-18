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
// 扫描 dt/dd 或 th/td，按 defaultBasicInfoLabels 匹配标签。
func (p *PublicExtractor) fillBasicInfoFields(doc *goquery.Document, seed *SeedData) {
	values := map[string]string{}

	// dt/dd 模式
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

	// th/td 模式
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
