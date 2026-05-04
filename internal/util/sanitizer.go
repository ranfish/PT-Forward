package util

import (
	"regexp"

	"go.uber.org/zap/zapcore"
)

var defaultSensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(passkey[=:]\s*)\S+`),
	regexp.MustCompile(`(?i)(cookie[=:]\s*)\S+`),
	regexp.MustCompile(`(?i)(api_key[=:]\s*)\S+`),
	regexp.MustCompile(`(?i)(bearer_token[=:]\s*)\S+`),
	regexp.MustCompile(`(?i)(password[=:]\s*)\S+`),
	regexp.MustCompile(`(?i)(encryption_key[=:]\s*)\S+`),
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
