package titleparser

// StandardizeTechProfile 从 TechProfile 映射到标准键（§56.34 步骤 3）。
//
// 与 Standardize(TitleComponents) 的区别：
//   - HDR 从 TechProfile.HDR 取（MediaInfo 为准，而非标题）
//   - Medium 从 SourceType + Specification 复合拼接后映射
//   - AudioCodec 只取编码（不含 Atmos，Atmos 通过 media_tag 映射）
func StandardizeTechProfile(p TechProfile) (StandardParams, error) {
	ensureMappings()
	if loadErr != nil {
		return StandardParams{}, loadErr
	}

	sp := StandardParams{
		Resolution: lookupStandard(mappings.Resolution, p.Resolution),
		VideoCodec: lookupStandard(mappings.VideoCodec, p.VideoCodec),
		AudioCodec: lookupStandard(mappings.AudioCodec, p.AudioCodec),
		HDR:        lookupStandard(mappings.HDR, p.HDR),
	}
	sp.Medium = standardizeMedium(composeMedium(p))

	return sp, nil
}
