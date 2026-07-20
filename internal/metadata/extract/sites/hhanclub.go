package sites

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/ranfish/pt-forward/internal/metadata/extract"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

// hHanClubExtractor 憨憨站特殊提取器。
// 基于 auto_feed_js auto_feed_2.1.1.2.js line 9514-9536 的 DOM 逻辑。
//
// HHanClub 使用 Tailwind CSS + 自定义 DOM（无标准 NexusPHP #kdescr 容器）。
// 字段标签在 div.font-bold.leading-6 中，值在 .next() 兄弟 div 中。
// 基本信息是聚合文本（div:contains(基本信息):last → .next().text()）。
// 字段名来自 PTNexus hhanclub.yaml source_parsers.source_params。
type hHanClubExtractor struct{}

func newHHanClubExtractor() *hHanClubExtractor { return &hHanClubExtractor{} }

func (e *hHanClubExtractor) Name() string { return "hhanclub_special" }

var hhanclubTitleQuotedRe = regexp.MustCompile(`<title>[^<]*(?:&quot;|")([^"&<]+)(?:&quot;|")[^<]*</title>`)
var hhanclubTitleFallbackRe = regexp.MustCompile(`<title>([^<]+)</title>`)
var hhanclubPosterRe = regexp.MustCompile(`src="(https?://[^"]*l_ratio_poster[^"]*)"`)
var hhanclubDescStartRe = regexp.MustCompile(`(?is)src="https?://[^"]*l_ratio_poster[^"]*"[^>]*>(.*?)(?:</div>\s*</div>\s*<div\s+class="footer|</body>)`)
var hhanclubImdbRe = regexp.MustCompile(`https?://(?:www\.)?imdb\.com/title/(tt\d+)`)
var hhanclubDoubanRe = regexp.MustCompile(`https?://(?:www\.)?(?:movie\.)?douban\.com/(?:subject|movie)/(\d+)`)

func (e *hHanClubExtractor) Extract(input extract.Input) (extract.SeedData, error) {
	seed, err := baseExtract(input)
	if err != nil {
		return seed, err
	}

	if title := extractHHanClubTitle(input.PageHTML); title != "" {
		seed.Title = title
	}

	if seed.Intro.Poster == "" {
		if m := hhanclubPosterRe.FindStringSubmatch(input.PageHTML); len(m) > 1 {
			seed.Intro.Poster = m[1]
		}
	}

	if seed.Intro.Body == "" {
		if body := extractHHanClubDesc(input.PageHTML); body != "" {
			seed.Intro.Body = body
		}
	}

	if seed.MediaInfo == "" {
		seed.MediaInfo, seed.BDInfo = extract.ExtractMediaInfo(seed.Intro.Body, seed.Intro.Body, "hhanclub")
	}

	if seed.IMDbLink == "" && seed.Intro.Body != "" {
		if m := hhanclubImdbRe.FindString(seed.Intro.Body); m != "" {
			seed.IMDbLink = m
		}
	}
	if seed.DoubanLink == "" && seed.Intro.Body != "" {
		if m := hhanclubDoubanRe.FindString(seed.Intro.Body); m != "" {
			seed.DoubanLink = m
		}
	}

	// §56.13 HHanClub Tailwind CSS 字段提取（移植 auto_feed_js line 9514-9536）
	doc, parseErr := goquery.NewDocumentFromReader(strings.NewReader(input.PageHTML))
	if parseErr != nil {
		return seed.NormalizeWithFallback(input.FallbackTitle), nil
	}

	// 方式 1: div.font-bold.leading-6 = 字段标签，.next() = 值
	doc.Find(`div[class*="font-bold"][class*="leading-6"]`).Each(func(_ int, label *goquery.Selection) {
		labelText := strings.TrimSpace(label.Text())
		next := label.Next()
		if next.Length() == 0 {
			return
		}
		valueText := strings.TrimSpace(next.Text())
		if valueText == "" || len(valueText) > 200 {
			return
		}
		assignHHanClubField(&seed, labelText, valueText)
	})

	// 方式 2: 基本信息聚合文本（div:contains(基本信息) → .next().text()）
	doc.Find(`div:contains("基本信息")`).Each(func(_ int, div *goquery.Selection) {
		if strings.TrimSpace(div.Text()) != "基本信息" {
			return
		}
		next := div.Next()
		if next.Length() == 0 {
			return
		}
		infoText := strings.TrimSpace(next.Text())
		if infoText == "" {
			return
		}
		fillHHanClubFromBasicInfo(&seed, infoText, input.FallbackTitle)
	})

	return seed.NormalizeWithFallback(input.FallbackTitle), nil
}

var hhanclubFieldLabels = map[string]string{
	"标题": "title", "標題": "title",
	"副标题": "subtitle", "副標題": "subtitle",
	"类型": "type", "類型": "type",
	"媒介": "medium", "媒體": "medium",
	"视频编码": "video_codec", "視頻編碼": "video_codec",
	"音频编码": "audio_codec", "音頻編碼": "audio_codec",
	"分辨率": "resolution", "解析度": "resolution",
	"制作组": "team", "製作組": "team",
	"处理": "source", "處理": "source",
}

func assignHHanClubField(seed *extract.SeedData, label, value string) {
	field, ok := hhanclubFieldLabels[label]
	if !ok {
		return
	}
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

func fillHHanClubFromBasicInfo(seed *extract.SeedData, infoText, fallbackTitle string) {
	if seed.Type == "" {
		if v := titleparser.ExtractType(infoText); v != "" {
			seed.Type = v
		}
	}
	if seed.Medium == "" {
		if v := titleparser.ExtractMedium(infoText, fallbackTitle); v != "" {
			seed.Medium = v
		}
	}
	if seed.VideoCodec == "" {
		if v := titleparser.ExtractCodec(infoText); v != "" {
			seed.VideoCodec = v
		}
	}
	if seed.AudioCodec == "" {
		if v := titleparser.ExtractAudioCodec(infoText); v != "" {
			seed.AudioCodec = v
		}
	}
	if seed.Resolution == "" {
		if v := titleparser.ExtractResolution(infoText); v != "" {
			seed.Resolution = v
		}
	}
	if seed.Source == "" {
		if v := titleparser.ExtractSource(infoText); v != "" {
			seed.Source = v
		}
	}
}

func extractHHanClubTitle(htmlStr string) string {
	if m := hhanclubTitleQuotedRe.FindStringSubmatch(htmlStr); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	m := hhanclubTitleFallbackRe.FindStringSubmatch(htmlStr)
	if len(m) < 2 {
		return ""
	}
	title := m[1]
	for _, sep := range []string{" :: ", " - Powered"} {
		if idx := strings.Index(title, sep); idx > 0 {
			title = title[:idx]
		}
	}
	return strings.TrimSpace(title)
}

func extractHHanClubDesc(htmlStr string) string {
	m := hhanclubDescStartRe.FindStringSubmatch(htmlStr)
	if len(m) < 2 {
		return ""
	}
	desc := m[1]
	desc = strings.ReplaceAll(desc, "<br />", "\n")
	desc = strings.ReplaceAll(desc, "<br/>", "\n")
	desc = strings.ReplaceAll(desc, "<br>", "\n")
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
