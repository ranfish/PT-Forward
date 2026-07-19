package sites

import "github.com/ranfish/pt-forward/internal/metadata/extract"

// audiencesExtractor 人人站特殊提取器。
// PTNexus sites/audiences.go（103 行）。
type audiencesExtractor struct{}

func newAudiencesExtractor() *audiencesExtractor { return &audiencesExtractor{} }

func (e *audiencesExtractor) Name() string { return "audiences_special" }

func (e *audiencesExtractor) Extract(input extract.Input) (extract.SeedData, error) {
	seed, err := baseExtract(input)
	if err != nil {
		return seed, err
	}
	// TODO(2b.7): Audiences 特殊覆盖
	//   - showCodeMain / CodeMain MediaInfo 容器
	//   - BDInfo SUBTITLES 到 FILES 段截断（特殊段处理）
	// 等真实样本验证后实施。
	return seed, nil
}
