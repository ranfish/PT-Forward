// Package extract 站点特殊提取规则执行器（v0.0.254）。
//
// 替代 internal/metadata/extract/sites/ 下的 Go 特殊提取器。
// 每个站点通过 sites_source_keys/<domain>.json 的 extractors 字段配置启用。
// PublicExtractor.Extract 流程会在标准提取后调用 applySiteExtractors 应用特殊规则。
package extract

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// applySiteExtractors 按站点配置应用特殊提取规则（v0.0.254）。
// 在 PublicExtractor 标准提取流程后调用，覆盖/补齐字段。
// 不破坏标准流程已提取的字段（仅填空或显式覆盖）。
func (p *PublicExtractor) applySiteExtractors(
	doc *goquery.Document,
	seed *SeedData,
	pageHTML, fallbackTitle, domain, siteCode string,
) {
	ext := LookupSiteExtractors(domain, siteCode)
	if ext == nil {
		return
	}

	// 1. category_from_icons: 从 rowfollow 内 <img alt> 提取字段（PTer 模式）
	if ext.CategoryFromIcons != nil {
		p.extractCategoryFromIcons(doc, seed, ext.CategoryFromIcons, domain, siteCode)
	}

	// 2. title_from_quoted: 从带引号的 <title> 提取标题（HHanClub 模式）
	if ext.TitleFromQuoted != nil && pageHTML != "" {
		if title := extractTitleFromQuoted(pageHTML, ext.TitleFromQuoted); title != "" {
			seed.Title = title // 显式覆盖（HHanClub <title> 比 h1 准）
		}
	}

	// 3. description_from_range: 从 HTML 区间提取简介（HHanClub 模式）
	if ext.DescriptionFromRange != nil && pageHTML != "" && seed.Intro.Body == "" {
		if body := extractDescriptionFromRange(pageHTML, ext.DescriptionFromRange); body != "" {
			seed.Intro.Body = body
			// 从提取的简介里补 IMDb/Douban/TMDb 链接（HHanClub 模式：链接在简介文本里）
			if seed.IMDbLink == "" {
				if m := imdbURLRe.FindString(body); m != "" {
					seed.IMDbLink = m
				}
			}
			if seed.DoubanLink == "" {
				if m := doubanURLRe.FindString(body); m != "" {
					seed.DoubanLink = m
				}
			}
			if seed.TMDbLink == "" {
				if m := tmdbURLRe.FindString(body); m != "" {
					seed.TMDbLink = m
				}
			}
		}
	}

	// 4. poster_from_pattern: 从 URL 正则提取海报
	if ext.PosterFromPattern != "" && pageHTML != "" && seed.Intro.Poster == "" {
		re, err := regexp.Compile(ext.PosterFromPattern)
		if err == nil {
			if m := re.FindStringSubmatch(pageHTML); len(m) > 1 {
				posterURL := m[1]
				if strings.HasPrefix(posterURL, "/") && domain != "" {
					posterURL = "https://" + domain + posterURL
				}
				seed.Intro.Poster = posterURL
			} else if m := re.FindString(pageHTML); m != "" {
				seed.Intro.Poster = m
			}
		}
	}

	// 5. basic_info_div_label: 从 div 标签提取字段（Tailwind CSS 模式）
	if ext.BasicInfoDivLabel != nil {
		p.extractBasicInfoFromDivLabel(doc, seed, ext.BasicInfoDivLabel, domain, siteCode)
	}
}

// extractCategoryFromIcons 从 rowfollow 内 <img alt> 提取字段（PTer 模式）。
func (p *PublicExtractor) extractCategoryFromIcons(doc *goquery.Document, seed *SeedData, cfg *categoryIconsConfig, domain, siteCode string) {
	if cfg.ContainerText == "" || len(cfg.AltToField) == 0 {
		return
	}
	doc.Find(`td[class*="rowhead"]`).Each(func(_ int, head *goquery.Selection) {
		headText := strings.TrimSpace(head.Text())
		if headText != cfg.ContainerText {
			return
		}
		follow := head.NextFiltered(`td[class*="rowfollow"]`)
		if follow.Length() == 0 {
			return
		}
		follow.Find("img").Each(func(_ int, img *goquery.Selection) {
			alt, _ := img.Attr("alt")
			alt = strings.TrimSpace(alt)
			if alt == "" || len(alt) < 2 {
				return
			}
			field, ok := cfg.AltToField[alt]
			if !ok {
				// 模糊匹配（alt 包含 key 或 key 包含 alt）
				for k, v := range cfg.AltToField {
					if strings.Contains(alt, k) || strings.Contains(k, alt) {
						field = v
						break
					}
				}
				if field == "" {
					return
				}
			}
			// 填空字段（不覆盖非空）+ 标准化
			standardized := standardizeFieldValueWithDomain(domain, siteCode, field, alt)
			switch field {
			case "type":
				if seed.Type == "" {
					seed.Type = standardized
				}
			case "medium":
				if seed.Medium == "" {
					seed.Medium = standardized
				}
			case "source":
				if seed.Source == "" {
					seed.Source = standardized
				}
			case "video_codec":
				if seed.VideoCodec == "" {
					seed.VideoCodec = standardized
				}
			case "audio_codec":
				if seed.AudioCodec == "" {
					seed.AudioCodec = standardized
				}
			case "resolution":
				if seed.Resolution == "" {
					seed.Resolution = standardized
				}
			case "team":
				if seed.ReleaseGroup == "" {
					seed.ReleaseGroup = standardized
				}
			}
		})
	})
}

// extractTitleFromQuoted 从带引号的 <title> 提取标题（HHanClub 模式）。
func extractTitleFromQuoted(pageHTML string, cfg *titleQuotedConfig) string {
	// 优先用引号模式
	if cfg.QuotedPattern != "" {
		re, err := regexp.Compile(cfg.QuotedPattern)
		if err == nil {
			if m := re.FindStringSubmatch(pageHTML); len(m) > 1 {
				title := strings.TrimSpace(m[1])
				return stripSuffixes(title, cfg.StripSuffix)
			}
		}
	}
	// fallback
	if cfg.FallbackPattern != "" {
		re, err := regexp.Compile(cfg.FallbackPattern)
		if err == nil {
			if m := re.FindStringSubmatch(pageHTML); len(m) > 1 {
				title := strings.TrimSpace(m[1])
				return stripSuffixes(title, cfg.StripSuffix)
			}
		}
	}
	return ""
}

func stripSuffixes(title string, suffixes []string) string {
	for _, sep := range suffixes {
		if idx := strings.Index(title, sep); idx > 0 {
			title = title[:idx]
		}
	}
	return strings.TrimSpace(title)
}

// extractDescriptionFromRange 从 HTML 区间提取简介（HHanClub 模式）。
func extractDescriptionFromRange(pageHTML string, cfg *descriptionRangeConfig) string {
	startRe, err := regexp.Compile(cfg.StartPattern)
	if err != nil {
		return ""
	}
	endRe, err := regexp.Compile(cfg.EndPattern)
	if err != nil {
		return ""
	}
	startIdx := startRe.FindStringIndex(pageHTML)
	if len(startIdx) < 2 {
		return ""
	}
	rest := pageHTML[startIdx[1]:]
	endIdx := endRe.FindStringIndex(rest)
	var desc string
	if len(endIdx) >= 2 {
		desc = rest[:endIdx[0]]
	} else {
		desc = rest
	}
	// 简单清洗：br → 换行，去 HTML 标签
	desc = strings.ReplaceAll(desc, "<br />", "\n")
	desc = strings.ReplaceAll(desc, "<br/>", "\n")
	desc = strings.ReplaceAll(desc, "<br>", "\n")
	// img/a 转 BBCode
	imgRe := regexp.MustCompile(`(?i)<img[^>]*src="([^"]+)"[^>]*>`)
	desc = imgRe.ReplaceAllString(desc, "[img]$1[/img]")
	linkRe := regexp.MustCompile(`(?i)<a[^>]*href='([^']+)'[^>]*>([^<]*)</a>`)
	desc = linkRe.ReplaceAllString(desc, "[url=$1]$2[/url]")
	linkRe2 := regexp.MustCompile(`(?i)<a[^>]*href="([^"]+)"[^>]*>([^<]*)</a>`)
	desc = linkRe2.ReplaceAllString(desc, "[url=$1]$2[/url]")
	tagRe := regexp.MustCompile(`<[^>]+>`)
	desc = tagRe.ReplaceAllString(desc, "")
	desc = strings.TrimSpace(desc)
	if len(desc) < 10 {
		return ""
	}
	return desc
}

// extractBasicInfoFromDivLabel 从 div 标签提取字段（Tailwind CSS 模式，HHanClub）。
// 支持两种 ValueGetter：
//   - "next_sibling": 标签的下一个兄弟元素（HHanClub 旧版）
//   - "container_label_grid_span": 找 ContainerLabel 元素后的 grid，遍历 span pair（HHanClub 新版）
func (p *PublicExtractor) extractBasicInfoFromDivLabel(doc *goquery.Document, seed *SeedData, cfg *basicInfoDivLabelConfig, domain, siteCode string) {
	if cfg.LabelSelector == "" {
		return
	}
	fieldLabels := buildFieldLabels(domain, siteCode)

	switch cfg.ValueGetter {
	case "container_label_grid_span":
		// HHanClub 新版：基本信息 → grid 内 span.font-bold + 下一个 span
		p.extractBasicInfoFromGridSpan(doc, seed, cfg, fieldLabels, domain, siteCode)
		// v0.0.264: 不 return，继续跑 next_sibling 处理容器外的 sibling pair
		// （HHanClub "副标题"等容器外 label+value，DOM 结构相同）
		fallthrough
	case "next_sibling", "":
		// HHanClub 旧版 / Tailwind 标签 + next div
	}

	doc.Find(cfg.LabelSelector).Each(func(_ int, label *goquery.Selection) {
		labelText := strings.TrimSpace(label.Text())
		if labelText == "" {
			return
		}
		// v0.0.254 修复：不强制要求冒号（HHanClub Tailwind 标签 "标题"/"副标题" 不带冒号）
		field := matchFieldWithLabels(labelText, fieldLabels)
		if field == "" {
			trimmed := strings.TrimRight(labelText, ":：")
			if trimmed != labelText {
				field = matchFieldWithLabels(strings.TrimSpace(trimmed), fieldLabels)
			}
		}
		if field == "" {
			return
		}
		next := label.Next()
		if next.Length() == 0 {
			return
		}
		value := strings.TrimSpace(next.Text())
		if value == "" || len([]rune(value)) > 200 {
			return
		}
		assignExtractorField(seed, field, standardizeFieldValueWithDomain(domain, siteCode, field, value))
	})
}

// extractBasicInfoFromGridSpan 从 container label 后的 grid 提取 span pair（HHanClub 新版模式）。
// DOM 结构：
//
//	<div class="font-bold leading-6">基本信息</div>
//	<div class="grid ...">
//	  <div><span class="font-bold">类型:</span><span class="">电视剧</span></div>
//	  <div><span class="font-bold">媒介:</span><span class="">WEB-DL</span></div>
//	  ...
//	</div>
func (p *PublicExtractor) extractBasicInfoFromGridSpan(doc *goquery.Document, seed *SeedData, cfg *basicInfoDivLabelConfig, fieldLabels map[string][]string, domain, siteCode string) {
	if cfg.ContainerLabel == "" {
		return
	}
	labelClass := cfg.LabelClass
	if labelClass == "" {
		labelClass = "font-bold"
	}

	// 找到文本等于 ContainerLabel 的元素
	doc.Find(cfg.LabelSelector).Each(func(_ int, label *goquery.Selection) {
		if strings.TrimSpace(label.Text()) != cfg.ContainerLabel {
			return
		}
		// 取下一个 grid 容器
		grid := label.Next()
		if grid.Length() == 0 {
			return
		}
		// 遍历 grid 内的 div（每个 div 含一对 span: label + value）
		grid.Find("div").Each(func(_ int, div *goquery.Selection) {
			// div 内的 span.font-bold 是 label
			labelSpan := div.Find("span." + labelClass).First()
			if labelSpan.Length() == 0 {
				return
			}
			labelText := strings.TrimSpace(labelSpan.Text())
			if labelText == "" {
				return
			}
			// 去掉末尾冒号 + nbsp
			labelText = strings.TrimRight(strings.TrimRight(labelText, ":："), " \u00a0\t")
			labelText = strings.TrimSpace(labelText)
			field := matchFieldWithLabels(labelText, fieldLabels)
			if field == "" {
				return
			}
			// 取 label span 的下一个兄弟 span（value）
			valueSpan := labelSpan.Next()
			if valueSpan.Length() == 0 {
				// 或者 labelSpan 的父 div 内第二个 span
				valueSpan = labelSpan.Siblings().Filter("span").First()
			}
			if valueSpan.Length() == 0 {
				return
			}
			value := strings.TrimSpace(valueSpan.Text())
			if value == "" || len([]rune(value)) > 200 {
				return
			}
			assignExtractorField(seed, field, standardizeFieldValueWithDomain(domain, siteCode, field, value))
		})
	})
}

// assignExtractorField 把字段值填到 seed（不覆盖非空字段）。
func assignExtractorField(seed *SeedData, field, value string) {
	switch field {
	case "title":
		if seed.Title == "" {
			seed.Title = value
		}
	case "subtitle":
		if seed.Subtitle == "" {
			seed.Subtitle = value
		}
	case "type":
		if seed.Type == "" {
			seed.Type = value
		}
	case "medium":
		if seed.Medium == "" {
			seed.Medium = value
		}
	case "video_codec":
		if seed.VideoCodec == "" {
			seed.VideoCodec = value
		}
	case "audio_codec":
		if seed.AudioCodec == "" {
			seed.AudioCodec = value
		}
	case "resolution":
		if seed.Resolution == "" {
			seed.Resolution = value
		}
	case "team":
		if seed.ReleaseGroup == "" {
			seed.ReleaseGroup = value
		}
	case "source":
		if seed.Source == "" {
			seed.Source = value
		}
	}
}
