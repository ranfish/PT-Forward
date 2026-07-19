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
// 支持 4 种 HTML 模式（按精准度优先级）：
//  1. td.rowhead + td.rowfollow（NexusPHP 种子信息表标准，PTer/HDSky 等用此模式）
//  2. dt/dd 模式（HTML5 description list）
//  3. th/td 模式（table header + cell）
//  4. td/td 相邻模式（普通表格，限定不在 colhead 表内避免文件列表表头误匹配）
//
// 经验：PTer 详情页 table[10] 用 td.rowhead + td.rowfollow 模式，
// 包含"副标题/类别与标签/基本信息/IMDb链接/豆瓣链接"等键值对。
// 而 td.colhead 是文件列表表头（"类型/标题/大小"），不能用作 label-value。
func (p *PublicExtractor) fillBasicInfoFields(doc *goquery.Document, seed *SeedData) {
	values := map[string]string{}

	// 模式 1: td.rowhead + td.rowfollow（NexusPHP 种子信息表，最精准）
	// 同时也匹配 td.rowhead_text（部分站变体）
	doc.Find(`td[class*="rowhead"]`).Each(func(_ int, head *goquery.Selection) {
		headText := strings.TrimSpace(head.Text())
		field := matchBasicInfoLabel(headText)
		if field == "" {
			return
		}
		// 找同 tr 内下一个 td.rowfollow
		follow := head.NextFiltered(`td[class*="rowfollow"]`)
		if follow.Length() == 0 {
			// 兜底：找下一个 td（不带 class 限制）
			follow = head.NextFiltered("td")
		}
		if follow.Length() == 0 {
			return
		}
		// 对 PTer 等站，rowfollow 可能含多个 img/a 子元素，取纯文本
		value := strings.TrimSpace(follow.Text())
		if value == "" {
			return
		}
		// 排除"基本信息"这种聚合 td（值太长，含多个字段）
		// 让 PTer 站点提取器单独解析这种聚合行
		if field == "type" && len(value) > 50 {
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
		if field := matchBasicInfoLabel(dtText); field != "" {
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
		if field := matchBasicInfoLabel(thText); field != "" {
			if _, exists := values[field]; !exists {
				values[field] = tdText
			}
		}
	})

	// 模式 4: td/td 相邻（普通表格 fallback）
	// 加严格守卫：当前 td 不能是 colhead（文件列表表头），且不含子 td
	doc.Find("td").Each(func(_ int, td *goquery.Selection) {
		tdText := strings.TrimSpace(td.Text())
		if tdText == "" {
			return
		}
		cls, _ := td.Attr("class")
		if strings.Contains(cls, "colhead") {
			return // 跳过文件列表表头
		}
		field := matchBasicInfoLabel(tdText)
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
		if field == "type" && len(value) > 50 {
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
