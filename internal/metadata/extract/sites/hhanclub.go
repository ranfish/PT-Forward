package sites

import "github.com/ranfish/pt-forward/internal/metadata/extract"

// hHanClubExtractor 憨憨站特殊提取器。
// PTNexus sites/hhanclub.go（641 行，最复杂）。
type hHanClubExtractor struct{}

func newHHanClubExtractor() *hHanClubExtractor { return &hHanClubExtractor{} }

func (e *hHanClubExtractor) Name() string { return "hhanclub_special" }

func (e *hHanClubExtractor) Extract(input extract.Input) (extract.SeedData, error) {
	seed, err := baseExtract(input)
	if err != nil {
		return seed, err
	}
	// TODO(2b.7): HHanClub 特殊覆盖
	//   - DOM 树解析（x/net/html 直接 Node API，goquery 不足）
	//   - section div 容器结构
	//   - nexus-media-info-raw 容器（MediaInfo 原始数据）
	//   - powered by nexusphp 包装剥离（憨憨站特殊包装层）
	// PTNexus 实现最复杂（641 行），需要真实样本逐个验证。
	return seed, nil
}
