package titleparser

// StandardParams 标准化后的参数（标准键）
type StandardParams struct {
	Type       string `json:"type"`        // category.movie
	Medium     string `json:"medium"`      // medium.bluray
	VideoCodec string `json:"video_codec"` // video.x264
	AudioCodec string `json:"audio_codec"` // audio.dts_hd_ma
	Resolution string `json:"resolution"`  // resolution.r1080p
	HDR        string `json:"hdr"`         // hdr.dv
	Source     string `json:"source"`      // source.china
	Team       string `json:"team"`        // team.cmct
}

// StandardizeTechProfile 从 TechProfile 映射到标准键（§56.34 步骤 3）。
//
// §59.35：查询切公共字典 LookupDictKey（exact → ci → contains 最长）。
//
// 与 Standardize(TitleComponents) 的区别：
//   - HDR 从 TechProfile.HDR 取（MediaInfo 为准，而非标题）
//   - Medium 从 SourceType + Specification 复合拼接后映射
//   - AudioCodec 只取编码（不含 Atmos，Atmos 通过 media_tag 映射）
func StandardizeTechProfile(p TechProfile) (StandardParams, error) {
	sp := StandardParams{
		Resolution: LookupDictKey("resolution", p.Resolution),
		VideoCodec: LookupDictKey("video_codec", p.VideoCodec),
		AudioCodec: LookupDictKey("audio_codec", p.AudioCodec),
		HDR:        LookupDictKey("hdr", p.HDR),
	}
	sp.Medium = LookupDictKey("medium", composeMedium(p))

	return sp, nil
}
