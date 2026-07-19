package sites

import (
	"regexp"
	"strings"

	"github.com/ranfish/pt-forward/internal/metadata/extract"
)

// hHanClubExtractor 憨憨站特殊提取器。
// PTNexus sites/hhanclub.go（641 行）。
//
// HHanClub 使用 Tailwind CSS + 自定义 DOM（无标准 #kdescr 容器）。
// 标题在 <title> 引号中，副标题在 font-bold leading-6 div 中，
// 简介区在 </tbody> 后直接流式输出（无容器包裹）。
type hHanClubExtractor struct{}

func newHHanClubExtractor() *hHanClubExtractor { return &hHanClubExtractor{} }

func (e *hHanClubExtractor) Name() string { return "hhanclub_special" }

// HHanClub <title> 格式: HHCLUB :: 种子详情 &quot;English Title&quot; - Powered by NexusPHP
// 支持 &quot; 和 " 两种引号
var hhanclubTitleQuotedRe = regexp.MustCompile(`<title>[^<]*(?:&quot;|")([^"&<]+)(?:&quot;|")[^<]*</title>`)
var hhanclubTitleFallbackRe = regexp.MustCompile(`<title>([^<]+)</title>`)

// 副标题在 font-bold leading-6 的 div 中（第一个长文本）
var hhanclubSubtitleRe = regexp.MustCompile(`class="font-bold leading-6"[^>]*>([^<]{10,})`)

// 海报 l_ratio_poster 图片
var hhanclubPosterRe = regexp.MustCompile(`src="(https?://[^"]*l_ratio_poster[^"]*)"`)

// 简介区域：l_ratio_poster 图片后的内容（到 </body> 前）
var hhanclubDescStartRe = regexp.MustCompile(`(?is)src="https?://[^"]*l_ratio_poster[^"]*"[^>]*>(.*?)(?:</div>\s*</div>\s*<div\s+class="footer|</body>)`)

// IMDb/豆瓣链接正则（从 Body 补充提取）
var hhanclubImdbRe = regexp.MustCompile(`https?://(?:www\.)?imdb\.com/title/(tt\d+)`)
var hhanclubDoubanRe = regexp.MustCompile(`https?://(?:www\.)?(?:movie\.)?douban\.com/(?:subject|movie)/(\d+)`)

func (e *hHanClubExtractor) Extract(input extract.Input) (extract.SeedData, error) {
	seed, err := baseExtract(input)
	if err != nil {
		return seed, err
	}

	// HHanClub 特殊 1: 标题从 <title> 引号提取
	if title := extractHHanClubTitle(input.PageHTML); title != "" {
		seed.Title = title
	}

	// HHanClub 特殊 2: 副标题从 font-bold leading-6 div 提取
	if seed.Subtitle == "" {
		if m := hhanclubSubtitleRe.FindStringSubmatch(input.PageHTML); len(m) > 1 {
			seed.Subtitle = strings.TrimSpace(m[1])
		}
	}

	// HHanClub 特殊 3: 海报从 l_ratio_poster 图片提取
	if seed.Intro.Poster == "" {
		if m := hhanclubPosterRe.FindStringSubmatch(input.PageHTML); len(m) > 1 {
			seed.Intro.Poster = m[1]
		}
	}

	// HHanClub 特殊 4: 简介区域提取（无标准容器，从海报图片后开始）
	if seed.Intro.Body == "" {
		if body := extractHHanClubDesc(input.PageHTML); body != "" {
			seed.Intro.Body = body
		}
	}

	// HHanClub 特殊 5: MediaInfo 提取（在简介区域中搜索）
	if seed.MediaInfo == "" {
		seed.MediaInfo, seed.BDInfo = extract.ExtractMediaInfo(seed.Intro.Body, seed.Intro.Body, "hhanclub")
	}

	// HHanClub 特殊 6: IMDb/豆瓣从 Body 补充提取（公共提取器可能找不到）
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

	return seed, nil
}

// extractHHanClubTitle 从 <title> 标签提取标题。
// 格式: HHCLUB :: 种子详情 "English Title" - Powered by NexusPHP
func extractHHanClubTitle(htmlStr string) string {
	// 优先从引号中提取
	if m := hhanclubTitleQuotedRe.FindStringSubmatch(htmlStr); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	// fallback: 去掉站点后缀
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

// extractHHanClubDesc 从海报图片后提取简介区域。
// HHanClub 简介无容器包裹，直接是 <img> + <br/> + <a> 标签。
func extractHHanClubDesc(htmlStr string) string {
	m := hhanclubDescStartRe.FindStringSubmatch(htmlStr)
	if len(m) < 2 {
		return ""
	}
	// 将渲染后的 HTML 转换为近似 BBCode
	desc := m[1]
	// <br/> → \n
	desc = strings.ReplaceAll(desc, "<br />", "\n")
	desc = strings.ReplaceAll(desc, "<br/>", "\n")
	desc = strings.ReplaceAll(desc, "<br>", "\n")
	// <img src="xxx"> → [img]xxx[/img]
	imgRe := regexp.MustCompile(`(?i)<img[^>]*src="([^"]+)"[^>]*>`)
	desc = imgRe.ReplaceAllString(desc, "[img]$1[/img]")
	// <a href="xxx">text</a> → [url=xxx]text[/url]
	linkRe := regexp.MustCompile(`(?i)<a[^>]*href='([^']+)'[^>]*>([^<]*)</a>`)
	desc = linkRe.ReplaceAllString(desc, "[url=$1]$2[/url]")
	linkRe2 := regexp.MustCompile(`(?i)<a[^>]*href="([^"]+)"[^>]*>([^<]*)</a>`)
	desc = linkRe2.ReplaceAllString(desc, "[url=$1]$2[/url]")
	// 移除剩余 HTML 标签
	tagRe := regexp.MustCompile(`<[^>]+>`)
	desc = tagRe.ReplaceAllString(desc, "")
	// 清理多余空白
	desc = strings.TrimSpace(desc)
	if len(desc) < 10 {
		return ""
	}
	return desc
}
