package extract

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// §56.10 决策 3：先硬编码 + TODO 接入 §56.1 standard_keys 表。
// v0.0.238: defaultBasicInfoLabels 作为 fallback，PTNexus 配置（site_config.go）优先。
// v0.0.252: source 字段移除 "来源"/"來源"（与 medium 字段歧义），
// 让 site-specific 配置（1ptba/hitpt/discfan 的 medium="来源"）控制，
// 避免壹吧/百川/碟粉的 "来源: WEB-DL" 被误归到 source（产地）字段。
var defaultBasicInfoLabels = map[string][]string{
	"type":        {"类型", "類型", "類別", "类别", "category", "type"},
	"medium":      {"媒介", "媒体", "媒體", "格式", "medium"},
	"video_codec": {"视频编码", "視頻編碼", "编码", "編碼", "视频编碼", "視頻编码", "video codec", "codec"},
	"audio_codec": {"音频编码", "音頻編碼", "音频", "音頻", "audio codec", "audio"},
	"resolution":  {"分辨率", "解析度", "resolution", "分辨率"},
	"team":        {"制作组", "製作組", "团队", "團隊", "制作組", "製作组", "team"},
	"source":      {"产地", "產地", "地区", "地區", "处理", "處理", "source"},
}

// fillBasicInfoFields 从详情页基本信息表填充结构化字段。
// 支持 5 种 HTML 模式（按精准度优先级）：
//  1. td.rowhead + td.rowfollow（NexusPHP 种子信息表标准，PTer/HDSky 等用此模式）
//     - 子模式 a: rowfollow 纯文本（PTer "大小：18.53 GB 类型: 电视剧"）
//     - 子模式 b: <span title="字段名">值</span>（SSD/Audiences）
//     - 子模式 c: <b>字段名:</b>值 聚合 inline（HDSky/HDFans/CarPT/HDArea/HDTime 等 11 站）
//  2. dt/dd 模式（HTML5 description list）
//  3. th/td 模式（table header + cell）
//  4. td/td 相邻模式（普通表格，限定不在 colhead 表内避免文件列表表头误匹配）
//
// v0.0.238: 按 site_code 加载 PTNexus 移植的 source_key 配置（覆盖默认 label）。
// v0.0.253: 改用 domain 主键（解决 site_code 冲突）。
// 字段值经 LookupStandardKey 映射到标准键（如 "电视剧 (TV Series)" → "category.tv_series"）。
func (p *PublicExtractor) fillBasicInfoFields(doc *goquery.Document, seed *SeedData) {
	p.fillBasicInfoFieldsWithCode(doc, seed, p.domain, p.siteCode)
}

// fillBasicInfoFieldsWithCode 显式传入 domain + siteCode 的版本（并发安全）。
func (p *PublicExtractor) fillBasicInfoFieldsWithCode(doc *goquery.Document, seed *SeedData, domain, siteCode string) {
	// 每个字段的候选 labels：site-specific source_key 优先 + default 变体
	fieldLabels := buildFieldLabels(domain, siteCode)
	values := map[string]string{}

	// 模式 1: td.rowhead + td.rowfollow（NexusPHP 种子信息表，最精准）
	// 同时支持三种 rowfollow 内容形式：
	//   a) 纯文本（如 PTer "大小：18.53 GB 类型: 电视剧"）
	//   b) <span title="字段名">值</span>（如 SSD "基本信息"中的 span 列表）
	//   c) <b>字段名:</b>值 聚合 inline（HDSky/HDFans/CarPT 等 11 站）
	doc.Find(`td[class*="rowhead"]`).Each(func(_ int, head *goquery.Selection) {
		headText := strings.TrimSpace(head.Text())
		// 先看 rowhead 本身是否就是字段标签（如 "类型"/"媒介"）
		if field := matchFieldWithLabels(headText, fieldLabels); field != "" {
			follow := head.NextFiltered(`td[class*="rowfollow"]`)
			if follow.Length() == 0 {
				follow = head.NextFiltered("td")
			}
			if follow.Length() == 0 {
				return
			}
			value := strings.TrimSpace(follow.Text())
			if value == "" || len([]rune(value)) > 80 {
				return
			}
			if _, exists := values[field]; !exists {
				values[field] = value
			}
			return
		}
		// rowhead 本身不是字段标签（如 "基本信息"），看 rowfollow 内的子模式 b/c
		if headText == "基本信息" || headText == "基本資料" || headText == "基本资料" || headText == "基本資訊" {
			follow := head.NextFiltered(`td[class*="rowfollow"]`)
			if follow.Length() == 0 {
				follow = head.NextFiltered("td")
			}
			if follow.Length() == 0 {
				return
			}
			// 子模式 b: <span title="...">值</span>（SSD/HDArea 等用此模式）
			follow.Find(`span[title]`).Each(func(_ int, span *goquery.Selection) {
				spanTitle, _ := span.Attr("title")
				spanTitle = strings.TrimSpace(spanTitle)
				field := matchFieldWithLabels(spanTitle, fieldLabels)
				if field == "" {
					return
				}
				value := strings.TrimSpace(span.Text())
				if value == "" || len([]rune(value)) > 80 {
					return
				}
				if _, exists := values[field]; !exists {
					values[field] = value
				}
			})
			// 子模式 c: <b>字段名:</b>值 聚合 inline（HDSky/HDFans/CarPT 等用此模式）
			// 遍历所有 <b>，去掉末尾冒号后匹配字段标签，值取自 <b> 的下一个兄弟节点（文本或 <span>）
			// v0.0.252 修复：
			//   1) 优先看 <b title="..."> 的 title 属性（织梦 zmpt.cc <b title="媒介">视频类:</b><span>WEB-DL</span>）
			//   2) value 长度判断用 rune 计数（星湾 xingwan.cc 中文值 "Blu-ray / BD（蓝光原盘 ...）" UTF-8 字节超 50 被误过滤）
			//   3) 兼容 <b>label</b><span>value</span> 结构（值在 span 里，非文本节点）
			follow.Find("b").Each(func(_ int, b *goquery.Selection) {
				// 1) 优先用 <b title="..."> 的 title 属性作 label
				label, _ := b.Attr("title")
				label = strings.TrimSpace(label)
				if label == "" {
					// 退化：用 b 的 text 去末尾冒号
					bText := strings.TrimSpace(b.Text())
					if bText == "" {
						return
					}
					trimmed := strings.TrimRight(bText, ":：")
					if trimmed == bText {
						return // 没冒号，不是字段标签
					}
					label = strings.TrimSpace(trimmed)
				}
				field := matchFieldWithLabels(label, fieldLabels)
				if field == "" {
					return
				}
				// 2) 取值：先看下一个兄弟节点
				if node := b.Nodes[0]; node != nil && node.NextSibling != nil {
					next := node.NextSibling
					var value string
					switch next.Type {
					case html.TextNode:
						// 文本节点：直接取 Data（已含实体解码后的文本）
						value = strings.TrimSpace(strings.TrimLeft(next.Data, " \t\n\r\u00a0"))
					case html.ElementNode:
						// 元素节点（如 <span>value</span>）：取元素内文本
						value = strings.TrimSpace(b.Next().Text())
					}
					if value == "" || len([]rune(value)) > 80 {
						return
					}
					if _, exists := values[field]; !exists {
						values[field] = value
					}
				}
			})
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
		if len([]rune(value)) > 80 {
			return
		}
		if _, exists := values[field]; !exists {
			values[field] = value
		}
	})

	// 填充 seed + 标准化字段值
	// v0.0.253: standardizeFieldValue 优先用站点特定 standard_keys（按 domain）
	for field, val := range values {
		standardized := standardizeFieldValueWithDomain(domain, siteCode, field, val)
		switch field {
		case "type":
			seed.Type = standardized
		case "medium":
			seed.Medium = standardized
		case "video_codec":
			seed.VideoCodec = standardized
		case "audio_codec":
			seed.AudioCodec = standardized
		case "resolution":
			seed.Resolution = standardized
		case "team":
			seed.ReleaseGroup = standardized
		case "source":
			seed.Source = standardized
		}
	}
}

// standardizeFieldValueWithDomain 优先用站点特定 standard_keys（v0.0.253）。
// 查找顺序：site-specific by domain > site-specific by site_code > global standard_keys > 原值。
func standardizeFieldValueWithDomain(domain, siteCode, category, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// 1. site-specific by domain
	if siteStd := LookupSiteStandardKeys(domain, siteCode); siteStd != nil {
		if m, ok := siteStd[category]; ok && m != nil {
			if std, ok := m[raw]; ok && std != "" {
				return std
			}
			// 包含匹配（最长 key 优先）
			bestKey, bestStd := "", ""
			for k, v := range m {
				if v == "" || len(k) < len(bestKey) {
					continue
				}
				if strings.Contains(raw, k) {
					bestKey, bestStd = k, v
				}
			}
			if bestStd != "" {
				return bestStd
			}
		}
	}
	// 2. 全局 standard_keys
	if std := LookupStandardKey(category, raw); std != "" {
		return std
	}
	return raw
}

// buildFieldLabels 构建每个字段的候选 label 列表（site-specific source_key 优先 + default 变体）。
// 返回 map：field → 候选 labels 切片。
// v0.0.253: 加 domain 参数（优先按 domain 查）。
func buildFieldLabels(domain, siteCode string) map[string][]string {
	result := map[string][]string{}
	for field := range defaultBasicInfoLabels {
		labels := []string{}
		// 1. site-specific source_key（最优先，按 domain）
		if sk := LookupSiteSourceKey(domain, siteCode, field); sk != "" {
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
	return matchFieldWithLabels(label, buildFieldLabels("", ""))
}
