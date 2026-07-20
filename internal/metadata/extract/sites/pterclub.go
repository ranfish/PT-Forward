package sites

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/ranfish/pt-forward/internal/metadata/extract"
)

// pterClubExtractor 猫站特殊提取器。
// PTNexus sites/pterclub.go（194 行）。
//
// PTer 详情页结构（基于 torrent_id=859990 真实样本分析）：
//   - 种子信息表用 td.rowhead + td.rowfollow 模式（PublicExtractor.fillBasicInfoFields 已支持）
//   - "类别与标签" rowfollow 内含多个 <img alt="电视剧 (TV Series)"> <img alt="WEB-DL">
//     这些 img alt 是种子的实际类别/媒介/产地（不是侧栏 checkbox，侧栏 checkbox 全部 unchecked）
//   - "基本信息" rowfollow 是一段内联文本（"大小：18.53 GB  类型: 电视剧 (TV Series) ..."）
//     需要专门解析
//   - 简介区 #kdescr 含 fieldset 引用块（官组声明）+ poster + screenshots + mediainfo 折叠块
//
// 关键设计：**绝不**扫描侧栏 checkbox 区（cat401/source5/team1 等），它们是搜索过滤器
// 而非种子属性。也**绝不**匹配文件列表表头 colhead（"类型/标题/大小"）。
type pterClubExtractor struct{}

func newPTerClubExtractor() *pterClubExtractor { return &pterClubExtractor{} }

func (e *pterClubExtractor) Name() string { return "pterclub_special" }

func (e *pterClubExtractor) Extract(input extract.Input) (extract.SeedData, error) {
	seed, err := baseExtract(input)
	if err != nil {
		return seed, err
	}

	doc, parseErr := goquery.NewDocumentFromReader(strings.NewReader(input.PageHTML))
	if parseErr != nil {
		// 解析失败不阻塞，返回公共提取器结果
		return seed.NormalizeWithFallback(input.FallbackTitle), nil
	}

	// §56.13 PTer 特殊覆盖 1: 从"类别与标签" rowfollow 内的 <img alt> 提取类别/媒介/产地
	// 这些 img alt 比 fillBasicInfoFields 的 td.rowhead 解析更精准
	// （因为 rowfollow 的纯文本可能是聚合的"中字英字"，而 img alt 才是分类标记）
	if categoryImgs := extractPTerCategoryIcons(doc); len(categoryImgs) > 0 {
		for _, alt := range categoryImgs {
			classification := classifyPTerIconAlt(alt)
			if classification == "" {
				continue
			}
			// 按优先级填空字段（不覆盖非空）
			switch classification {
			case "type":
				if seed.Type == "" {
					seed.Type = alt
				}
			case "medium":
				if seed.Medium == "" {
					seed.Medium = alt
				}
			case "source":
				if seed.Source == "" {
					seed.Source = alt
				}
			}
		}
	}

	// §56.13 PTer 特殊覆盖 2: 从"基本信息" rowfollow 内联文本解析
	// 样例："大小：18.53 GB  类型: 电视剧 (TV Series)  产地：欧美  分辨率：1080p"
	if basicText := extractPTerBasicInfoText(doc); basicText != "" {
		fillFromPTerBasicInfoText(&seed, basicText)
	}

	return seed.NormalizeWithFallback(input.FallbackTitle), nil
}

// extractPTerCategoryIcons 从"类别与标签" rowfollow 中提取所有 <img alt> 文本。
// PTer 详情页 td.rowhead="类别与标签" 的下一个 td.rowfollow 含分类图标 img。
// 注意：只在 rowhead="类别与标签" 的 rowfollow 内查（避免误匹配其他 img）。
func extractPTerCategoryIcons(doc *goquery.Document) []string {
	var icons []string
	doc.Find(`td[class*="rowhead"]`).Each(func(_ int, head *goquery.Selection) {
		headText := strings.TrimSpace(head.Text())
		if headText != "类别与标签" && headText != "類別與標籤" {
			return
		}
		follow := head.NextFiltered(`td[class*="rowfollow"]`)
		if follow.Length() == 0 {
			return
		}
		follow.Find("img").Each(func(_ int, img *goquery.Selection) {
			alt, _ := img.Attr("alt")
			alt = strings.TrimSpace(alt)
			if alt == "" {
				return
			}
			// 排除 UI 装饰图标（如 alt="" 的占位图）
			if len(alt) < 2 {
				return
			}
			icons = append(icons, alt)
		})
	})
	return icons
}

// classifyPTerIconAlt 将 PTer 分类图标 alt 文本映射到字段类型。
// alt 样例：
//   - "电视剧 (TV Series)" / "电影 (Movie)" / "动画 (Animation)" → type
//   - "WEB-DL" / "Blu-ray" / "Remux" / "HDTV" / "Encode" → medium
//   - "ENG" / "CHN" / "中字" / "英字" → 音频语言（暂不分类，跳过）
//   - "大陆 (Mainland,CHN)" / "欧美 (Western)" → source（产地）
func classifyPTerIconAlt(alt string) string {
	lower := strings.ToLower(alt)
	switch {
	// type
	case strings.Contains(alt, "电影") || strings.Contains(alt, "电视剧") ||
		strings.Contains(alt, "动画") || strings.Contains(alt, "综艺") ||
		strings.Contains(alt, "纪录") || strings.Contains(alt, "体育") ||
		strings.Contains(alt, "音乐") || strings.Contains(alt, "MV") ||
		strings.Contains(alt, "舞台") || strings.Contains(alt, "游戏") ||
		strings.Contains(alt, "软件") || strings.Contains(alt, "学习") ||
		strings.Contains(alt, "电子书") || strings.Contains(lower, "movie") ||
		strings.Contains(lower, "tv series") || strings.Contains(lower, "animation"):
		return "type"
	// medium
	case alt == "WEB-DL" || alt == "Blu-ray" || alt == "Remux" ||
		alt == "HDTV" || alt == "Encode" || alt == "UHD Discs" ||
		alt == "BD Discs" || alt == "DVD Discs" || strings.HasPrefix(alt, "Blu"):
		return "medium"
	// source（产地）
	case strings.Contains(alt, "大陆") || strings.Contains(alt, "香港") ||
		strings.Contains(alt, "台湾") || strings.Contains(alt, "欧美") ||
		strings.Contains(alt, "韩国") || strings.Contains(alt, "日本") ||
		strings.Contains(alt, "印度") || strings.Contains(alt, "其它"):
		return "source"
	}
	return ""
}

// extractPTerBasicInfoText 提取"基本信息" rowfollow 的纯文本。
// 样例："大小：18.53 GB  类型: 电视剧 (TV Series)  产地：欧美  分辨率：1080p"
func extractPTerBasicInfoText(doc *goquery.Document) string {
	var text string
	doc.Find(`td[class*="rowhead"]`).Each(func(_ int, head *goquery.Selection) {
		if text != "" {
			return
		}
		headText := strings.TrimSpace(head.Text())
		if headText != "基本信息" && headText != "基本資料" && headText != "基本资料" {
			return
		}
		follow := head.NextFiltered(`td[class*="rowfollow"]`)
		if follow.Length() == 0 {
			return
		}
		text = strings.TrimSpace(follow.Text())
	})
	return text
}

// fillFromPTerBasicInfoText 从"基本信息"聚合文本解析各字段。
// PTer 详情页"基本信息"实际格式（基于 torrent_id=859990 真实样本）：
//   "<b>大小：</b>18.53 GB  <b>类型:</b> 电视剧 (TV Series)  <b>质量: </b>WEB-DL  <b>地区: </b>欧美 (Western)"
// 注意：PTer 用"质量"代替"媒介"，用"地区"代替"产地"（与 PTNexus yaml 配置不符）
// 字段间用 \u00a0（NBSP）或多个空格分隔。
var rePTerTypeField = regexp.MustCompile(`类型\s*[:：]\s*(\S+(?:\s*\([^)]+\))?)`)
var rePTerMediumField = regexp.MustCompile(`(?:媒介|媒体|媒體|质量|質量)\s*[:：]\s*(\S+)`)
var rePTerCodecField = regexp.MustCompile(`(?:编码|視頻編碼|视频编码)\s*[:：]\s*(\S+)`)
var rePTerResolutionField = regexp.MustCompile(`分辨率\s*[:：]\s*(\S+)`)
var rePTerTeamField = regexp.MustCompile(`(?:制作组|製作組|团队|團隊)\s*[:：]\s*(\S+)`)
var rePTerSourceField = regexp.MustCompile(`(?:产地|產地|地区|地區|区域|區域)\s*[:：]\s*(\S+(?:\s*\([^)]+\))?)`)

func fillFromPTerBasicInfoText(seed *extract.SeedData, text string) {
	text = strings.ReplaceAll(text, "\u00a0", " ")
	if m := rePTerTypeField.FindStringSubmatch(text); len(m) > 1 && seed.Type == "" {
		seed.Type = strings.TrimSpace(m[1])
	}
	if m := rePTerMediumField.FindStringSubmatch(text); len(m) > 1 && seed.Medium == "" {
		seed.Medium = strings.TrimSpace(m[1])
	}
	if m := rePTerCodecField.FindStringSubmatch(text); len(m) > 1 && seed.VideoCodec == "" {
		seed.VideoCodec = strings.TrimSpace(m[1])
	}
	if m := rePTerResolutionField.FindStringSubmatch(text); len(m) > 1 && seed.Resolution == "" {
		seed.Resolution = strings.TrimSpace(m[1])
	}
	if m := rePTerTeamField.FindStringSubmatch(text); len(m) > 1 && seed.ReleaseGroup == "" {
		seed.ReleaseGroup = strings.TrimSpace(m[1])
	}
	if m := rePTerSourceField.FindStringSubmatch(text); len(m) > 1 && seed.Source == "" {
		seed.Source = strings.TrimSpace(m[1])
	}
}
