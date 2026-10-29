package adapter

import (
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/ranfish/pt-forward/internal/model"
)

type CSSDiscountRule struct {
	CSSClass string
	Level    model.DiscountLevel
}

var defaultNexusPHPRules = []CSSDiscountRule{
	{CSSClass: "pro_free2up", Level: model.Discount2xFree},
	{CSSClass: "pro_2up", Level: model.Discount2xUp},
	{CSSClass: "pro_free", Level: model.DiscountFree},
	{CSSClass: "pro_50p", Level: model.DiscountPercent50},
	{CSSClass: "pro_30p", Level: model.DiscountPercent30},
	{CSSClass: "pro_2x50pctdown", Level: model.Discount2x50},
}

// nativeNexusPHPRules covers NexusPHP standard template CSS classes that don't
// have the pro_ prefix. Matched against quote-normalized HTML (double quotes
// converted to single quotes) so the quoted class name won't false-match
// unrelated substrings like "Nofree".
var nativeNexusPHPRules = []CSSDiscountRule{
	{CSSClass: "'twoupfree'", Level: model.Discount2xFree},
	{CSSClass: "'twouphalfdown'", Level: model.Discount2x50},
	{CSSClass: "'thirtypercent'", Level: model.DiscountPercent30},
	{CSSClass: "'halfdown'", Level: model.DiscountPercent50},
	{CSSClass: "'promotion-tag-free'", Level: model.DiscountFree},
	{CSSClass: "'free'", Level: model.DiscountFree},
}

var defaultKeywordRules = []struct {
	Keyword string
	Level   model.DiscountLevel
}{
	{"2x免费", model.Discount2xFree},
	{"2x free", model.Discount2xFree},
	{"2xfree", model.Discount2xFree},
	{"double upload", model.Discount2xUp},
	{"2x upload", model.Discount2xUp},
	{"50% free", model.DiscountPercent50},
	{"half download", model.DiscountPercent50},
	{"freeleech", model.DiscountFree},
}

// imageRules covers sites that mark discount status via icon images instead of
// CSS classes (e.g. TTG uses <img src="/pic/ico_free.gif">). These patterns
// are matched against the full HTML since the icon is typically outside h1.
var imageRules = []struct {
	Pattern string
	Level   model.DiscountLevel
}{
	{"ico_free.gif", model.DiscountFree},
	{"ico_2xfree.gif", model.Discount2xFree},
	{"ico_2x.gif", model.Discount2xUp},
	{"ico_half.gif", model.DiscountPercent50},
	{"ico_30.gif", model.DiscountPercent30},
}

var h1Re = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)

// freeEndAtRe matches NexusPHP discount expiry markers like:
//
//	剩余时间：<span title="2026-05-08 13:50:58">1天15时</span>
//	剩余时间：<b><span title="2026-05-08 13:50:58">1天15时</span></b>
//	优惠剩余时间：<b><span title="2026-05-07 17:10:15">17时47分</span></b>
//	(限时<span title="2026-06-08 14:16:41">6时40分</span>)        (彩虹岛 ptchdbits.co)
//
// Captures the ISO-ish timestamp in the title attribute. Lives near (often
// inside) the h1 element on most NP-based sites.
var freeEndAtRe = regexp.MustCompile(`(?is)(?:剩余时间|优惠剩余时间|限时)[^<]{0,20}(?:<b>)?\s*<span[^>]*title=["']([0-9]{4}-[0-9]{2}-[0-9]{2}\s+[0-9]{2}:[0-9]{2}:[0-9]{2})["']`)

func DetectDiscountFromHTML(html string, cfg *model.SiteDiscountDetectionConfig) *model.DiscountResult {
	if cfg != nil && cfg.DiscountClassMapping != nil {
		return detectFromClassMapping(html, cfg)
	}
	return detectFromDefaultRules(html)
}

func DetectDiscountFromDetailsPage(html string, cfg *model.SiteDiscountDetectionConfig) *model.DiscountResult {
	targetHTML := html
	h1Scope := ""
	if m := h1Re.FindStringSubmatch(html); len(m) > 1 {
		h1Scope = m[1]
		targetHTML = h1Scope
	}
	result := DetectDiscountFromHTML(targetHTML, cfg)
	// freeEndAt 解析范围：随折扣命中的扫描范围走（h1 或紧邻兄弟）。
	endScope := targetHTML
	if result.Level == model.DiscountNone {
		// §59.367: h1 内 miss → 扩扫 h1 紧邻兄弟元素。NP 新版 details-*
		// 前端（深度站 dstudio.me 实证）把促销标记放 h1 兄弟节点
		// （details-title-statuses div：`<font class='free'>免费</font>` +
		// 剩余时间）——h1 内文干净，class 规则（'free'/'halfdown'）齐备
		// 但范围扫不到 → 全判 NONE → RSS 免费订阅零获取。
		// 防误伤双门（相似种子区的促销标记不得误判本种——
		// IgnoresSimilarTorrents/TTG 测试锚定的场景）：
		//   ① 只扫紧邻兄弟（NextAdjacent）——相似区在 h2/远表不扫；
		//   ② 长度门 2000B——促销标记区物理上小，大块相似表跳过。
		// 标准 NP 促销在 h1 内首轮命中不进本分支零行为变化。
		if sibling := h1PromotionSiblingScope(html); sibling != "" {
			siblingResult := DetectDiscountFromHTML(sibling, cfg)
			if siblingResult.Level != model.DiscountNone {
				result = siblingResult
				endScope = sibling
			}
		}
	}
	if result.Level == model.DiscountNone {
		// Fallback: scan full HTML for image-based markers (e.g. TTG /pic/ico_free.gif)
		// which live outside the h1 element.
		result = detectFromImageRules(html)
	}
	if result.Level != model.DiscountNone {
		// Parse discount expiry only when a discount was detected, otherwise
		// we'd pick up unrelated <span title="..."> attributes.
		if t, ok := parseFreeEndAt(endScope); ok {
			result.FreeEndAt = &t
		}
	}
	return result
}

// h1PromotionSiblingScope §59.367: 提取第一个 h1 紧邻兄弟元素（NextAdjacent）
// 的 outer HTML——NP 新版前端的促销标记区（details-title-statuses）。
// 双防御：无兄弟/兄弟超 2000B（相似种子大表形态）/解析失败均返回空串
// （调用方跳过扩展分支）。
func h1PromotionSiblingScope(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ""
	}
	h1 := doc.Find("h1").First()
	if h1.Length() == 0 {
		return ""
	}
	next := h1.Next()
	if next.Length() == 0 {
		return ""
	}
	out, err := goquery.OuterHtml(next)
	if err != nil || len(out) == 0 || len(out) > 2000 {
		return ""
	}
	return out
}

// parseFreeEndAt extracts the discount end timestamp from "剩余时间：<span title=...>"
// markers. Returns ok=false if not found or unparseable.
func parseFreeEndAt(scope string) (time.Time, bool) {
	m := freeEndAtRe.FindStringSubmatch(scope)
	if len(m) < 2 {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", m[1], time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func detectFromImageRules(html string) *model.DiscountResult {
	lower := strings.ToLower(html)
	for _, rule := range imageRules {
		if strings.Contains(lower, rule.Pattern) {
			return &model.DiscountResult{Level: rule.Level}
		}
	}
	return &model.DiscountResult{Level: model.DiscountNone}
}

func detectFromClassMapping(html string, cfg *model.SiteDiscountDetectionConfig) *model.DiscountResult {
	lower := strings.ToLower(html)
	normalized := strings.ReplaceAll(lower, `"`, `'`)
	for cssClass, levelStr := range cfg.DiscountClassMapping {
		if strings.Contains(lower, strings.ToLower(cssClass)) {
			level := model.DiscountLevel(strings.ToUpper(levelStr))
			if level.IsValid() {
				return &model.DiscountResult{Level: level}
			}
		}
	}

	for _, rule := range nativeNexusPHPRules {
		if strings.Contains(normalized, rule.CSSClass) {
			return &model.DiscountResult{Level: rule.Level}
		}
	}

	for _, rule := range defaultNexusPHPRules {
		if strings.Contains(lower, rule.CSSClass) {
			return &model.DiscountResult{Level: rule.Level}
		}
	}

	for _, kw := range defaultKeywordRules {
		if strings.Contains(lower, strings.ToLower(kw.Keyword)) {
			return &model.DiscountResult{Level: kw.Level}
		}
	}

	return &model.DiscountResult{Level: model.DiscountNone}
}

func detectFromDefaultRules(html string) *model.DiscountResult {
	lower := strings.ToLower(html)
	normalized := strings.ReplaceAll(lower, `"`, `'`)

	for _, rule := range nativeNexusPHPRules {
		if strings.Contains(normalized, rule.CSSClass) {
			return &model.DiscountResult{Level: rule.Level}
		}
	}

	for _, rule := range defaultNexusPHPRules {
		if strings.Contains(lower, rule.CSSClass) {
			return &model.DiscountResult{Level: rule.Level}
		}
	}

	for _, kw := range defaultKeywordRules {
		if strings.Contains(lower, strings.ToLower(kw.Keyword)) {
			return &model.DiscountResult{Level: kw.Level}
		}
	}

	return &model.DiscountResult{Level: model.DiscountNone}
}
