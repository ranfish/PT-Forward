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
	// §59.226 附二十一: 帧率 MI 优先（标题 60fps 形态兜底——技术参数族）
	if mi.FrameRate != "" {
		p.FrameRate = mi.FrameRate
	}
	// §59.151: MI 编码铁证传递（IsEncode MI 驱动）
	p.MIEncoded = mi.Encoded
	p.MIHasVideo = mi.Encoded || mi.Resolution != "" || mi.VideoCodec != ""
}

// MergeDOMInto 将详情页 DOM 字段合并到 TechProfile（§56.34 三源合并的第三源）。
//
// 合并优先级（§59.226 附五 重构——用户定案 MI>标题>DOM）：
//   - 媒介/分类（SourceType/Specification/Medium）：标题主值 + DOM fallback
//     （标题无值时填充——DOM 下拉值选错无人核对沉淀脏数据 §59.166 From S04/
//     Blu-ray 原盘覆盖案实证；标题是脸面（审核必看）存活的基本正确）
//   - 技术参数（Resolution/VideoCodec/AudioCodec）：仅在 MediaInfo 和标题都没值时填充（DOM 是 fallback——现状不变）
//
// §59.34 审计防御：DOM medium 解析不出有效片源/规格时（如未映射的 UNK* key），
// 不覆盖标题派生值——避免垃圾 DOM 值抹空 title 解析结果。
func MergeDOMInto(p *TechProfile, medium, resolution, videoCodec, audioCodec string) {
	if p == nil {
		return
	}
	// 媒介/分类：标题主值 + DOM fallback（§59.226 附五 对调——原 DOM 直接覆盖）
	if medium != "" {
		sourceType, specification := splitMedium(medium)
		if sourceType != "" || specification != "" {
			if p.SourceType == "" && p.Specification == "" {
				p.SourceType = sourceType
				p.Specification = specification
				p.Medium = medium
			}
		}
	}
	// 技术参数：仅在为空时填充（DOM 是最低优先级 fallback）
	if p.Resolution == "" && resolution != "" {
		p.Resolution = resolution
	}
	if p.VideoCodec == "" && videoCodec != "" {
		p.VideoCodec = videoCodec
	}
	if p.AudioCodec == "" && audioCodec != "" {
		p.AudioCodec = audioCodec
	}
}
