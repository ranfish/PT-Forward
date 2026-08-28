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
var siteOpMarkerTailRe = regexp.MustCompile(`([\s\x{00a0}]*\[(?:中性种子|免费|Free|2X|促销|\d+(?:[Xx×]\s*\d+)?\s*%)[^\]]*\])+(\s*\[(?:禁转|谢绝转载|严禁转载|禁止转载)[^\]]*\])?\s*$`)

// StripSiteOperationMarkers 剥除标题/副标题尾部的站点运营标记（§59.64 副标题侧同族）。
// §59.136 提升公共方法至 util（metadata 采集层与 db migration 清存量同一实现——
// metadata→reseed→db 依赖环, 不能放 metadata）。迭代处理多标记混排。
func StripSiteOperationMarkers(s string) string {
	s = strings.TrimSpace(s)
	for {
		ns := strings.TrimSpace(siteOpMarkerTailRe.ReplaceAllString(s, "$2"))
		if ns == s {
			return ns
		}
		s = ns
	}
}
