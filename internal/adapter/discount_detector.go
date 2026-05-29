package adapter

import (
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
	{"免费", model.DiscountFree},
	{"free", model.DiscountFree},
}

func DetectDiscountFromHTML(html string, cfg *model.SiteDiscountDetectionConfig) *model.DiscountResult {
	if cfg != nil && cfg.DiscountClassMapping != nil {
		return detectFromClassMapping(html, cfg.DiscountClassMapping)
	}
	return detectFromDefaultRules(html)
}

func detectFromClassMapping(html string, mapping map[string]string) *model.DiscountResult {
	lower := strings.ToLower(html)
	for cssClass, levelStr := range mapping {
		if strings.Contains(lower, strings.ToLower(cssClass)) {
			level := model.DiscountLevel(strings.ToUpper(levelStr))
			if level.IsValid() {
				return &model.DiscountResult{Level: level}
			}
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
