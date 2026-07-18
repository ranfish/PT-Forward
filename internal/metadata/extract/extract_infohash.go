package extract

import (
	"regexp"

	"github.com/PuerkitoBio/goquery"
)

// 4 个 InfoHash 正则（PT-Forward 现有 hashRowPatterns）。
var hashRowRegexps = []*regexp.Regexp{
	regexp.MustCompile(`(?i)种子Hash[：:](?:<[^>]*>|&nbsp;|\s)*([a-fA-F0-9]{40})`),
	regexp.MustCompile(`(?i)种子散列值[：:](?:<[^>]*>|&nbsp;|\s)*([a-fA-F0-9]{40})`),
	regexp.MustCompile(`(?i)Hash码(?:<[^>]*>)?[：:]?(?:<[^>]*>|&nbsp;|\s)*([a-fA-F0-9]{40})`),
	regexp.MustCompile(`data-hash="([a-fA-F0-9]{40})"`),
}

var infoHashWordRe = regexp.MustCompile(`(?i)info_hash[^\w]{1,5}([a-fA-F0-9]{40})`)
var hashBareRe = regexp.MustCompile(`\b([a-fA-F0-9]{40})\b`)

// extractInfoHash 从详情页 HTML 中提取 40 位 InfoHash。
// 主路径：goquery 找 dt/dd 或 th/td 含 "Hash" 的行。
// Fallback：4 个正则 + 单词边界裸 Hash。
func (p *PublicExtractor) extractInfoHash(htmlStr string, doc *goquery.Document) string {
	// 主路径 1: goquery dt/dd（NexusPHP 基本信息表）
	if hash := findHashInRows(doc); hash != "" {
		return hash
	}
	// 主路径 2: [data-hash] 属性
	if hash, _ := doc.Attr("data-hash"); len(hash) == 40 {
		// doc 根节点无 data-hash，应该搜元素
	}
	doc.Find(`[data-hash]`).Each(func(_ int, s *goquery.Selection) {
		// 在 Each 中无法返回，提前用 First
	})
	if v, ok := doc.Find(`[data-hash]`).First().Attr("data-hash"); ok && len(v) == 40 {
		return v
	}

	// Fallback: 正则
	for _, re := range hashRowRegexps {
		if m := re.FindStringSubmatch(htmlStr); len(m) > 1 {
			return m[1]
		}
	}
	if m := infoHashWordRe.FindStringSubmatch(htmlStr); len(m) > 1 {
		return m[1]
	}
	return ""
}

// findHashInRows 在 NexusPHP 基本信息表格中扫描含 "Hash" 的 dt/dd 或 th/td 行。
func findHashInRows(doc *goquery.Document) string {
	var found string
	// dt/dd 模式
	doc.Find("dt").Each(func(_ int, dt *goquery.Selection) {
		if found != "" {
			return
		}
		if !containsInsensitive(dt.Text(), "hash") {
			return
		}
		dd := dt.NextFiltered("dd")
		if dd.Length() == 0 {
			return
		}
		text := dd.Text()
		if m := hashBareRe.FindStringSubmatch(text); len(m) > 1 {
			found = m[1]
		}
	})
	if found != "" {
		return found
	}
	// th/td 模式
	doc.Find("th").Each(func(_ int, th *goquery.Selection) {
		if found != "" {
			return
		}
		if !containsInsensitive(th.Text(), "hash") {
			return
		}
		td := th.NextFiltered("td")
		if td.Length() == 0 {
			return
		}
		text := td.Text()
		if m := hashBareRe.FindStringSubmatch(text); len(m) > 1 {
			found = m[1]
		}
	})
	return found
}

func containsInsensitive(s, sub string) bool {
	return len(s) >= len(sub) && regexp.MustCompile(`(?i)`+regexp.QuoteMeta(sub)).MatchString(s)
}
