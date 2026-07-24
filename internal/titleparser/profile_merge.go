package titleparser

// MergeTechProfile 三源字段级合并 → TechProfile（§56.34 决策 4）。
//
// 合并优先级（按字段类别）：
//   - 技术参数（8 字段）：MediaInfo > 标题
//   - 标题特有（8 字段）：标题（步骤 2 接入 DOM 后变 标题 > DOM）
//   - 媒介/分类（2 字段）：标题（步骤 2 接入 DOM 后变 DOM > 标题）
//
// 步骤 1 实现：标题 + MediaInfo 合并。
// mi 为 nil 时返回纯标题解析结果（无纠正）。
func MergeTechProfile(tc TitleComponents, mi *MediaInfoTech) TechProfile {
	p := TechProfileFromTitle(tc)
	if mi == nil {
		return p
	}

	// 技术参数：MediaInfo 非空则覆盖标题
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

	return p
}
