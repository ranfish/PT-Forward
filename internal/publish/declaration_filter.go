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
	// §59.265: 站点固定资源声明整块剥除（quote 块短锚——按内容子串整块删）。
	// 包子 BaoziPT（头行"BaoziPT · 资源声明"精确锚）；农场头行存在空格变体
	// （"自由农场␣␣-␣资源声明"双空格实证）——用站名锚免疫排版漂移。
	"BaoziPT · 资源声明",
	"自由农场",

	// v0.0.1456（2026-11-12 用户定案·9 环境全库扫描）：
	// ① 防自身「禁转PTT」声明与站方「禁转PTTime」重复。
	"禁转PTTime",
	// ② 音乐站「试听/免责」声明族（皇后/乌托邦/我堡/红豆饭模板变体群——
	//   短锚按内容子串整块删·299+226+203+93+53+40… 颗 4 环境实证）。
	"仅用于试听",
	"请于试听后",
	"试听后请购买",
	"仅做宽带测试使用",
	"宽带测试、学习交流",
	"仅供个人音像制作的学习",
	"所有资源仅供试听",
	"版权属于创造者及唱片公司",
	"若鉴赏高品质音乐请购买正版",
	"资源来自互联网，感谢国外网友分享",
	"原始档案来自互联网",
	"文件来源与网络",
	// ③ 壹吧 1PTBA（站点渲染层插入 kdescr 头部——库内干净·采集回流防御：
	//   声明+小提示两段 fieldset·2026-11-12 详情页实证）。
	"在网上搜集且由用户上传",
	"您的保种是",
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

func (f *DeclarationFilter) GetRawPatterns(ctx context.Context) (string, error) {
	return f.repo.Get(ctx, "declaration_filter_patterns")
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
