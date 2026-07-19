package sites

import "github.com/ranfish/pt-forward/internal/metadata/extract"

// keepfrdsExtractor keepfrds 站特殊提取器。
// PTNexus sites/keepfrds.go（229 行）。
//
// 注意：PT-Forward DB 中 pt.keepfrds.com 的站名是"朋友"。
// 设计文档 §56.13 标注 keepfrds → "三月（别名）"，但实际 DB 中
// "三月"对应 duckboobee.org，"朋友"对应 pt.keepfrds.com。
// site_code 迁移时需确认正确站名。
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
