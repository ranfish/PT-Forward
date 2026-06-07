package adapter

import (
	"regexp"
	"strings"

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
	{"ico_50p.gif", model.DiscountPercent50},
	{"ico_30p.gif", model.DiscountPercent30},
}

var h1Re = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)

func DetectDiscountFromHTML(html string, cfg *model.SiteDiscountDetectionConfig) *model.DiscountResult {
	if cfg != nil && cfg.DiscountClassMapping != nil {
		return detectFromClassMapping(html, cfg)
	}
	return detectFromDefaultRules(html)
}

func DetectDiscountFromDetailsPage(html string, cfg *model.SiteDiscountDetectionConfig) *model.DiscountResult {
	targetHTML := html
	if m := h1Re.FindStringSubmatch(html); len(m) > 1 {
		targetHTML = m[1]
	}
	result := DetectDiscountFromHTML(targetHTML, cfg)
	if result.Level != model.DiscountNone {
		return result
	}
	// Fallback: scan full HTML for image-based markers (e.g. TTG /pic/ico_free.gif)
	// which live outside the h1 element.
	return detectFromImageRules(html)
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
