package util

import (
	"regexp"
	"strings"

	"go.uber.org/zap/zapcore"
)

var defaultSensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(passkey[=:]\s*)\S+`),
	regexp.MustCompile(`(?i)(cookie[=:]\s*)\S+`),
	regexp.MustCompile(`(?i)(api[_]?key[=:]\s*)\S+`),
	regexp.MustCompile(`(?i)(bearer_token[=:]\s*)\S+`),
	regexp.MustCompile(`(?i)(password[=:]\s*)\S+`),
	regexp.MustCompile(`(?i)(encryption_key[=:]\s*)\S+`),
	regexp.MustCompile(`(?i)(rss[_]?key[=:]\s*)\S+`),
	regexp.MustCompile(`(?i)(auth[_]?key[=:]\s*)\S+`),
	regexp.MustCompile(`(?i)(secret[=:]\s*)\S+`),
	regexp.MustCompile(`(?i)(token[=:]\s*)\S+`),
}

type SanitizerCore struct {
	zapcore.Core
	patterns []*regexp.Regexp
}

func NewSanitizerCore(core zapcore.Core) *SanitizerCore {
	return &SanitizerCore{
		Core:     core,
		patterns: defaultSensitivePatterns,
	}
}

func (s *SanitizerCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	for i := range fields {
		if fields[i].Type == zapcore.StringType {
			for _, p := range s.patterns {
				if p.MatchString(fields[i].String) {
					fields[i].String = p.ReplaceAllString(fields[i].String, "${1}***")
					break
				}
			}
		}
	}
	return s.Core.Write(entry, fields)
}

func (s *SanitizerCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if s.Enabled(entry.Level) {
		return ce.AddCore(entry, s)
	}
	return ce
}

// siteOpMarkerTailRe §59.99/§59.136: 站点运营标记（副标题/标题尾部）——
// 朋友站 "[中性种子(NL)]"（零魔力, NBSP 前缀）、百分比/返利族 "[50%]" "[30%]" "[75%]" "[2X 50%]"、
// 免费/促销族。非内容数据, 转发引用无意义, 采集层剥除。
// 混排保留组: 尾部 [禁转] 族（合规判据, extractFlags 消费）不剥——"[50%] [禁转]" 只剥 [50%]。
var siteOpMarkerTailRe = regexp.MustCompile(`(?i)([\s\x{00a0}]*\[(?:中性种子|免费|free|2x|促销|普通|hot|经典|限免|密封|seeding|leeching|\d+(?:[Xx×]\s*\d+)?\s*%)[^\]]*\])+(\s*\[(?:禁转|谢绝转载|严禁转载|禁止转载)[^\]]*\])?\s*$`)

// StripSiteOperationMarkers 剥除标题/副标题尾部的站点运营标记（§59.64 副标题侧同族）。
// §59.136 提升公共方法至 util（metadata 采集层与 db migration 清存量同一实现——
// metadata→reseed→db 依赖环, 不能放 metadata）。迭代处理多标记混排。
// siteStateTailRe §59.248 族二: 圆括号站方状态标注（标题尾部）——织梦系
// "(已审)" 等（fnos 378 行实证）。判据：括号内首字 ∈ {已,待,未}（审核
// 状态语义锚）——内容性括号（"(港)" 地区译名标注）不剥。
var siteStateTailRe = regexp.MustCompile(`(?i)([\s\x{00a0}]*\((?:已|待|未|中性种子|免费|free|2x|促销|普通|hot|经典|限免|密封|seeding|leeching|\d+(?:[Xx×]\s*\d+)?\s*%)[^)]*\))+\s*$`)

// StripSiteOperationMarkers 剥除标题/副标题尾部一切【与标题无关的站方标注】
// （§59.248 泛化为标题净化公共方法）——两族词表：族一方括号运营标记
// （§59.99/§59.136）+族二圆括号站方状态（已审/已复核/待审）。迭代处理
// 多标记混排+跨族混排。存量清除重获自愈（无需 migration）。
func StripSiteOperationMarkers(s string) string {
	s = strings.TrimSpace(s)
	for {
		// 族一（运营标记——禁转保留组 $2）
		ns := strings.TrimSpace(siteOpMarkerTailRe.ReplaceAllString(s, "$2"))
		// 族二（站方状态——无保留组直接剥）
		ns = strings.TrimSpace(siteStateTailRe.ReplaceAllString(ns, ""))
		if ns == s {
			return ns
		}
		s = ns
	}
}
