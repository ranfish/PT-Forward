package sites

import "github.com/ranfish/pt-forward/internal/metadata/extract"

// NewSpecialExtractors 返回所有特殊站点的 Extractor 映射。
// key = site_code（如 "pterclub"），value = Extractor 实现。
// Engine 通过 NewEngine(public, specials) 注册。
//
// 8 站清单（§56.13 实施顺序）：
//   pterclub / hdsky / hhanclub / ssd / keepfrds / audiences / hddolby / chdbits
func NewSpecialExtractors() map[string]extract.Extractor {
	return map[string]extract.Extractor{
		"pterclub":  newPTerClubExtractor(),
		"hdsky":     newHDSkyExtractor(),
		"hhanclub":  newHHanClubExtractor(),
		"ssd":       newSSDExtractor(),
		"keepfrds":  newKeepfrdsExtractor(),
		"audiences": newAudiencesExtractor(),
		"hddolby":   newHDDolbyExtractor(),
		"chdbits":   newCHDBitsExtractor(),
	}
}
