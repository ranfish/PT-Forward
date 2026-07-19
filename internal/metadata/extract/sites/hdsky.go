package sites

import "github.com/ranfish/pt-forward/internal/metadata/extract"

// hdSkyExtractor 天空站特殊提取器。
// PTNexus sites/hdsky.go（走公共 + 少量覆盖）。
type hdSkyExtractor struct{}

func newHDSkyExtractor() *hdSkyExtractor { return &hdSkyExtractor{} }

func (e *hdSkyExtractor) Name() string { return "hdsky_special" }

func (e *hdSkyExtractor) Extract(input extract.Input) (extract.SeedData, error) {
	seed, err := baseExtract(input)
	if err != nil {
		return seed, err
	}
	// TODO(2b.7): HDSky 特殊覆盖（PTNexus 实现简单，走公共 + 少量覆盖）
	// 等真实样本验证后确认是否有特殊点。
	return seed, nil
}
