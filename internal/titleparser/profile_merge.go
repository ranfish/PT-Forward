package titleparser

// MergeTechProfile 三源字段级合并 → TechProfile（§56.34 决策 4）。
//
// 步骤 1 兼容接口：从 TitleComponents + MediaInfo 构造 TechProfile。
// 步骤 2 起推荐用 ParseTitleTech + MergeMediaInfoInto（支持增强字段）。
func MergeTechProfile(tc TitleComponents, mi *MediaInfoTech) TechProfile {
	p := TechProfileFromTitle(tc)
	MergeMediaInfoInto(&p, mi)
	return p
}

// MergeMediaInfoInto 将 MediaInfo 技术参数合并到已有的 TechProfile。
//
// 决策 4：技术参数 MediaInfo 为准。MediaInfo 非空字段覆盖标题值。
// mi 为 nil 时不修改 TechProfile（保留标题解析结果）。
func MergeMediaInfoInto(p *TechProfile, mi *MediaInfoTech) {
	if p == nil || mi == nil {
		return
	}
	if mi.Resolution != "" {
		p.Resolution = mi.Resolution
	}
	if mi.VideoCodec != "" {
		p.VideoCodec = mi.VideoCodec
	}
	if mi.AudioCodec != "" {
		p.AudioCodec = mi.AudioCodec
	}
	if mi.AudioChannels != "" {
		p.AudioChannels = mi.AudioChannels
	}
	if mi.AudioTechnology != "" {
		p.AudioTechnology = mi.AudioTechnology
	}
	if mi.AudioTracks > 0 {
		p.AudioTracks = mi.AudioTracks
	}
	if mi.HDR != "" {
		p.HDR = mi.HDR
	}
	if mi.BitDepth != "" {
		p.BitDepth = mi.BitDepth
	}
}
