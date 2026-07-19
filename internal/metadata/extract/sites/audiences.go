package sites

import (
	"regexp"

	"github.com/ranfish/pt-forward/internal/metadata/extract"
)

// audiencesExtractor 人人站特殊提取器。
// PTNexus sites/audiences.go（103 行）。
type audiencesExtractor struct{}

func newAudiencesExtractor() *audiencesExtractor { return &audiencesExtractor{} }

func (e *audiencesExtractor) Name() string { return "audiences_special" }

// audiencesBdinfoCutRe BDInfo SUBTITLES → FILES 段截断。
// Audiences 的 BDInfo 中 SUBTITLES 到 FILES 之间是字幕列表（非 BDInfo 正文），需移除。
var audiencesBdinfoCutRe = regexp.MustCompile(`(?is)(SUBTITLES:.*?)(FILES:)`)

func (e *audiencesExtractor) Extract(input extract.Input) (extract.SeedData, error) {
	seed, err := baseExtract(input)
	if err != nil {
		return seed, err
	}
	// Audiences 特殊: BDInfo SUBTITLES → FILES 段截断
	// 移除 SUBTITLES 到 FILES 之间的字幕列表内容
	if seed.BDInfo != "" {
		seed.BDInfo = audiencesBdinfoCutRe.ReplaceAllString(seed.BDInfo, "$2")
	}
	return seed, nil
}
