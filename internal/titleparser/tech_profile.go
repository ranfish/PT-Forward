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

	// === PTGen 数据资产字段（权威源：description◎行提取，§59.168）===
	ChineseTitle string `json:"chinese_title"` // ◎片名（中文名资产——Tab1/cnname/副标题共用）
	EnglishTitle string `json:"english_title"` // ◎译名首段英文（含冒号复合——主标题组装主路径 V1.05 #80）
	Genre        string `json:"genre"`         // ◎类别（JSON 数组 '["惊悚","恐怖"]'）

	// === 技术参数字段（权威源：MediaInfo，8 字段）===
	Resolution      string `json:"resolution"`       // 5. 分辨率（4320p/2160p/1080p/720p/480p/1440p）
	HDR             string `json:"hdr"`              // 10. HDR 类型（HDR10/HDR10+/DoVi/DoVi HDR/HDR Vivid/HLG）
	BitDepth        string `json:"bit_depth"`        // 11. bit 信息（8bit/10bit）
	MIEncoded       bool   `json:"-"`                // §59.151: MI Writing library 重编码铁证
	MIHasVideo      bool   `json:"-"`                // §59.151: MI 存在 Video 层（判据可信前提）
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

	// === 音乐字段（§56.35 阶段 3，仅音乐类种子有值，视频类为空）===
	Artist        string `json:"artist,omitempty"`         // 艺术家名
	Album         string `json:"album,omitempty"`          // 专辑名
	ChineseArtist string `json:"chinese_artist,omitempty"` // 中文艺术家名
	ChineseAlbum  string `json:"chinese_album,omitempty"`  // 中文专辑名
	SampleRate    string `json:"sample_rate,omitempty"`    // 采样率（如 "96.0"）
	AudioBitDepth string `json:"audio_bit_depth,omitempty"` // 位深（如 "24"）
}

// IsEncode 判定 Encode（压制）资源（v1.05：Encode 规格为空，由片源写法/编码族区分）。
//
// 判定信号（任一命中且非 Remux/WEB/HDTV 规格）：
//   - 片源为压制写法（BluRay/UHD BluRay/3D BluRay，无连字符）
//   - 视频编码为压制族（x264/x265/Xvid）
func IsEncode(p TechProfile) bool {
	// §59.151: MI Writing library 铁证优先（Encoded=有编码器写入痕迹——
	// WEB-DL 同 spec 双种子 MI 一编码一原生的实证；spec/标题循环依赖废除）
	if p.MIEncoded {
		return true
	}
	if p.MIHasVideo && !p.MIEncoded {
		// MI 存在 Video 层且无编码痕迹 = 原生（原盘直流/未重编码 WEB-DL）
		if p.Specification != "BDRip" && p.Specification != "DVDRip" && p.Specification != "TVRip" {
			return false
		}
	}
	switch p.Specification {
	case "Remux", "WEB-DL", "WEBRip", "HDTV", "UHDTV", "BDRip", "DVDRip", "TVRip":
		return false
	}
	switch p.SourceType {
	case "BluRay", "UHD BluRay", "3D BluRay":
		return true
	}
	switch p.VideoCodec {
	case "x264", "x265", "Xvid":
		// 编码族为压制，但片源写法为原盘（Blu-ray 带连字符）时以片源为准
		return p.SourceType != "Blu-ray" && p.SourceType != "UHD Blu-ray" && p.SourceType != "3D Blu-ray"
	}
	return false
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

// TechProfileToComponents 从 TechProfile 回填 TitleComponents（兼容旧接口）。
//
// 用于 InferCategory（只依赖 SeasonEpisode）和前端兼容（result["title_components"]）。
// 技术参数字段（Resolution/VideoCodec/AudioCodec/HDR/BitDepth）是 MediaInfo 合并后的值。
func TechProfileToComponents(p TechProfile) TitleComponents {
	return TitleComponents{
		MainTitle:      p.MainTitle,
		SeasonEpisode:  p.SeasonEpisode,
		Year:           p.Year,
		ReleaseVersion: p.ReleaseVersion,
		Resolution:     p.Resolution,
		SourcePlatform: p.SourcePlatform,
		Medium:         p.Medium,
		HDRFormat:      p.HDR,
		VideoCodec:     p.VideoCodec,
		BitDepth:       p.BitDepth,
		AudioCodec:     p.AudioCodec,
		ReleaseGroup:   p.ReleaseGroup,
		ChinesePrefix:  p.ChinesePrefix,
	}
}
