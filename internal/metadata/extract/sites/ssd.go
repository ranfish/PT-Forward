package sites

import "github.com/ranfish/pt-forward/internal/metadata/extract"

// ssdExtractor 不可说站特殊提取器。
// PTNexus sites/ssd.go（360 行）。
type ssdExtractor struct{}

func newSSDExtractor() *ssdExtractor { return &ssdExtractor{} }

func (e *ssdExtractor) Name() string { return "ssd_special" }

func (e *ssdExtractor) Extract(input extract.Input) (extract.SeedData, error) {
	seed, err := baseExtract(input)
	if err != nil {
		return seed, err
	}
	// TODO(2b.7): SSD 特殊覆盖
	//   - <span id="torrent-name"> 主标题（SSD 特有 selector）
	//   - dot 分隔归一化（标题中用 · 或 • 分隔字段）
	//   - l_ratio_poster 海报识别（SSD 特有海报 URL 模式）
	//   - Mediainfo 块特殊提取（SSD 的 MediaInfo 容器结构特殊）
	// 等真实样本验证后实施。
	return seed, nil
}
