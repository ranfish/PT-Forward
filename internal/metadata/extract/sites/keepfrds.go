package sites

import "github.com/ranfish/pt-forward/internal/metadata/extract"

// keepfrdsExtractor keepfrds 站特殊提取器。
// PTNexus sites/keepfrds.go（229 行）。
//
// 站名对照（生产 DB 验证 + 用户确认）：
//   pt.keepfrds.com → PT-Forward 站名"朋友" → siteCode "keepfrds"
//   duckboobee.org  → PT-Forward 站名"三月"（独立站点，非 keepfrds）
type keepfrdsExtractor struct{}

func newKeepfrdsExtractor() *keepfrdsExtractor { return &keepfrdsExtractor{} }

func (e *keepfrdsExtractor) Name() string { return "keepfrds_special" }

func (e *keepfrdsExtractor) Extract(input extract.Input) (extract.SeedData, error) {
	seed, err := baseExtract(input)
	if err != nil {
		return seed, err
	}
	// TODO(2b.7): keepfrds 特殊覆盖
	//   - 主副标题对调（副标题字段实为中文名，需交换 Title/Subtitle）
	//   - 嵌套 div pre 块 MediaInfo 提取
	// 等真实样本验证后实施。
	return seed, nil
}
