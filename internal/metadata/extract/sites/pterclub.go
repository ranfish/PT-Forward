package sites

import "github.com/ranfish/pt-forward/internal/metadata/extract"

// pterClubExtractor 猫站特殊提取器。
// PTNexus sites/pterclub.go（194 行）。
type pterClubExtractor struct{}

func newPTerClubExtractor() *pterClubExtractor { return &pterClubExtractor{} }

func (e *pterClubExtractor) Name() string { return "pterclub_special" }

func (e *pterClubExtractor) Extract(input extract.Input) (extract.SeedData, error) {
	seed, err := baseExtract(input)
	if err != nil {
		return seed, err
	}
	// TODO(2b.7 逐站实施): PTer 特殊覆盖
	//   - rePTerMediaInfoBlock: 内嵌 MediaInfo 块剥离（PTer 站点 MediaInfo 嵌在简介正文中）
	//   - <td>豆瓣链接</td> 表格行 URL 提取（PTer 表格行字段名特殊）
	//   - <td>IMDb链接</td> 表格行 URL 提取
	// 等真实样本验证后实施。
	return seed, nil
}
