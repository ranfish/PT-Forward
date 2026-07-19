package sites

import "github.com/ranfish/pt-forward/internal/metadata/extract"

// hdDolbyExtractor 不可杜站特殊提取器。
// PTNexus sites/hddolby.go（220 行）。
type hdDolbyExtractor struct{}

func newHDDolbyExtractor() *hdDolbyExtractor { return &hdDolbyExtractor{} }

func (e *hdDolbyExtractor) Name() string { return "hddolby_special" }

func (e *hdDolbyExtractor) Extract(input extract.Input) (extract.SeedData, error) {
	seed, err := baseExtract(input)
	if err != nil {
		return seed, err
	}
	// TODO(2b.7): HDDolby 特殊覆盖
	//   - l_ratio_poster 海报识别（与 SSD 类似）
	//   - div#kimdb 抓 IMDb 链接（HDDolby 特有 IMDb 容器）
	//   - 自定义 MediaInfo 容器
	// 等真实样本验证后实施。
	return seed, nil
}
