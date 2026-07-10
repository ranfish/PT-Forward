package publish

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/ranfish/pt-forward/internal/setting"
	"go.uber.org/zap"
)

var defaultDeclarationPatterns = []string{
	"ARDTU工具自动发布",
	"CSAUTO工具自动发布",
	"FWAUTO工具自动发布",
	"郑重声明：",
	"本站提供的所有作品均是用户自行搜集并且上传",
	"用户自行搜集",
	"禁止任何涉及商业盈利目的使用",
	"请在下载后24小时内尽快删除",
	"不可用于任何形式的商业盈利活动",
	"| A | By ATU",
	"| By ARDTU",
	"| ARDTU",
	"By ARDTU",
	"By CSAUTO",
	"By FWAUTO",
	".Release.Info",
}

var quoteBlockRe = regexp.MustCompile(`(?s)\[quote\](.*?)\[/quote\]`)
var byARDTURe = regexp.MustCompile(`(?i)\s*By ARDTU\s*`)

type DeclarationFilter struct {
	repo   *setting.Repository
	logger *zap.Logger
}

func NewDeclarationFilter(repo *setting.Repository, logger *zap.Logger) *DeclarationFilter {
	return &DeclarationFilter{repo: repo, logger: logger}
}

func (f *DeclarationFilter) GetPatterns(ctx context.Context) []string {
	val, err := f.repo.Get(ctx, "declaration_filter_patterns")
	if err != nil || val == "" {
		return defaultDeclarationPatterns
	}
	var patterns []string
	if err := json.Unmarshal([]byte(val), &patterns); err != nil || len(patterns) == 0 {
		return defaultDeclarationPatterns
	}
	return patterns
}

func (f *DeclarationFilter) SetPatterns(ctx context.Context, patterns []string) error {
	data, _ := json.Marshal(patterns)
	return f.repo.Set(ctx, "declaration_filter_patterns", string(data))
}

type FilterResult struct {
	CleanedText      string
	RemovedDecls     []string
}

func (f *DeclarationFilter) Filter(text string, patterns []string) FilterResult {
	if text == "" {
		return FilterResult{}
	}
	if len(patterns) == 0 {
		patterns = defaultDeclarationPatterns
	}

	patternLower := make([]string, len(patterns))
	for i, p := range patterns {
		patternLower[i] = strings.ToLower(p)
	}

	var removed []string
	cleaned := text

	matches := quoteBlockRe.FindAllStringSubmatchIndex(text, -1)
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		fullStart, fullEnd := m[0], m[1]
		contentStart, contentEnd := m[2], m[3]
		content := text[contentStart:contentEnd]
		contentLower := strings.ToLower(content)

		matched := false
		for _, p := range patternLower {
			if strings.Contains(contentLower, p) {
				matched = true
				break
			}
		}

		if matched {
			removed = append([]string{content}, removed...)
			cleaned = cleaned[:fullStart] + cleaned[fullEnd:]
		} else if byARDTURe.MatchString(content) {
			cleaned = byARDTURe.ReplaceAllString(cleaned, "")
		}
	}

	cleaned = strings.TrimSpace(cleaned)
	if len(removed) > 0 {
		cleaned = strings.TrimRight(cleaned, "\n\r ")
	}

	return FilterResult{
		CleanedText:  cleaned,
		RemovedDecls: removed,
	}
}
