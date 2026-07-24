package titleparser

// TechProfile 种子技术特征统一模型（§56.34，18 字段 + 过渡字段）。
//
// 作为标题解析 / MediaInfo / DOM 三源技术特征的单一事实来源（SSOT），
// 取代 TitleComponents 的割裂描述，按青蛙站 v1.05 视频标题命名规范统一建模。
//
// 字段权威源（决策 4）：
//   - 技术参数（8 字段）：MediaInfo > 标题
//   - 标题特有（8 字段）：标题 > DOM
//   - 媒介/分类（2 字段）：DOM > 标题
type TechProfile struct {
	// === 标题特有字段（权威源：标题解析，8 字段）===
	MainTitle      string `json:"main_title"`      // 1. 主标题（剧名）
	SeasonEpisode  string `json:"season_episode"`  // 2. 季集信息（S01 / S01E03）
	Year           string `json:"year"`            // 3. 年份
	EditionInfo    string `json:"edition_info"`    // 4. 其他信息（IMAX/Extended/Remastered...）
	RegionCode     string `json:"region_code"`     // 6. 地区码（ITA/USA/JPN，仅原盘类）
	SourcePlatform string `json:"source_platform"` // 7. 内容分发方（NF/AMZN/DSNP）
	ReleaseGroup   string `json:"release_group"`   // 17. 制作组
	ChinesePrefix  string `json:"chinese_prefix"`  // 18. 中文名前缀 [中文名]

	// === 技术参数字段（权威源：MediaInfo，8 字段）===
	Resolution      string `json:"resolution"`       // 5. 分辨率（4320p/2160p/1080p/720p/480p/1440p）
	HDR             string `json:"hdr"`              // 10. HDR 类型（HDR10/HDR10+/DoVi/DoVi HDR/HDR Vivid/HLG）
	BitDepth        string `json:"bit_depth"`        // 11. bit 信息（8bit/10bit）
	VideoCodec      string `json:"video_codec"`      // 12. 视频编码（x264/x265/HEVC/AVC/AV1...）
	AudioCodec      string `json:"audio_codec"`      // 13. 音频编码（DDP/DD/DTS/DTS-HD MA/TrueHD/FLAC/AAC...）
	AudioChannels   string `json:"audio_channels"`   // 14. 声道数（2.0/5.1/7.1）
	AudioTechnology string `json:"audio_technology"` // 15. 对象信息（Atmos）
	AudioTracks     int    `json:"audio_tracks"`     // 16. 音轨数

	// === 媒介/分类字段（权威源：DOM，2 字段）===
	SourceType    string `json:"source_type"`   // 8. 片源类型（Blu-ray/UHD Blu-ray/BluRay/DVDRip）
	Specification string `json:"specification"` // 9. 规格（Remux/WEB-DL/WEBRip/HDTV）

	// === 过渡兼容字段（步骤 2/3 增强后迁移到正式字段）===
	ReleaseVersion string `json:"release_version,omitempty"` // 发布版本（PROPER/REPACK），步骤 3 合并到 EditionInfo
	Medium         string `json:"medium,omitempty"`          // 媒介（组合值），步骤 3 拆分到 SourceType + Specification
}

// TechProfileFromTitle 从 TitleComponents 构造 TechProfile（仅标题源，无 MediaInfo 纠正）。
//
// 步骤 1 过渡实现：只映射 TitleComponents 中已有的字段。
// EditionInfo / RegionCode / SourceType / Specification / AudioChannels /
// AudioTechnology / AudioTracks 留待步骤 2（ParseTitle 增强）填充。
func TechProfileFromTitle(c TitleComponents) TechProfile {
	return TechProfile{
		MainTitle:       c.MainTitle,
		SeasonEpisode:   c.SeasonEpisode,
		Year:            c.Year,
		ReleaseVersion:  c.ReleaseVersion,
		Resolution:      c.Resolution,
		SourcePlatform:  c.SourcePlatform,
		HDR:             c.HDRFormat,
		VideoCodec:      c.VideoCodec,
		BitDepth:        c.BitDepth,
		AudioCodec:      c.AudioCodec,
		ReleaseGroup:    c.ReleaseGroup,
		ChinesePrefix:   c.ChinesePrefix,
		Medium:          c.Medium,
	}
}
