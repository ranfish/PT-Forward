package sites

import "github.com/ranfish/pt-forward/internal/metadata/extract"

// chdBITSExtractor 彩虹岛站特殊提取器。
// PTNexus sites/chdbits.go（实现简单，走公共 + 少量覆盖）。
type chdBITSExtractor struct{}

func newCHDBitsExtractor() *chdBITSExtractor { return &chdBITSExtractor{} }

func (e *chdBITSExtractor) Name() string { return "chdbits_special" }

func (e *chdBITSExtractor) Extract(input extract.Input) (extract.SeedData, error) {
	seed, err := baseExtract(input)
	if err != nil {
		return seed, err
	}
	// TODO(2b.7): CHDBits 特殊覆盖（PTNexus 实现简单，走公共 + 少量覆盖）
	// 等真实样本验证后确认是否有特殊点。
	return seed, nil
}
