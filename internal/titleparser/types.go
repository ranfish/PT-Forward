package titleparser

// TitleComponents 标题解析后的结构化组件
// 对应 PTNexus simple_components.go 的 16 个组件
type TitleComponents struct {
	MainTitle       string `json:"main_title"`        // 主标题
	SeasonEpisode   string `json:"season_episode"`    // 季集 (S01E02)
	Year            string `json:"year"`              // 年份
	SeriesStatus    string `json:"series_status"`     // 剧集状态 (Complete)
	ReleaseVersion  string `json:"release_version"`   // 发布版本 (PROPER/REPACK)
	Resolution      string `json:"resolution"`        // 分辨率 (1080p/2160p)
	RegionCode      string `json:"region_code"`       // 地区码 (§59.97 附 ITA/USA/JPN/HKG——v1.05 #6)
	FrameRate       string `json:"frame_rate"`        // 帧率 (24fps)
	SourcePlatform  string `json:"source_platform"`   // 片源平台 (NF/AMZN/DSNP)
	Medium          string `json:"medium"`            // 媒介 (BluRay/WEB-DL/Remux)
	HDRFormat       string `json:"hdr_format"`        // HDR格式 (DV/HDR10+)
	VideoCodec      string `json:"video_codec"`       // 视频编码 (x264/HEVC/AV1)
	BitDepth        string `json:"bit_depth"`         // 色深 (8bit/10bit)
	AudioCodec      string `json:"audio_codec"`       // 音频编码 (DTS-HD MA/TrueHD)
	ReleaseGroup    string `json:"release_group"`     // 制作组 (CMCT/PTer)
	Unrecognized    string `json:"unrecognized"`      // 无法识别的部分
	ChinesePrefix   string `json:"chinese_prefix"`    // [中文名] 前缀
}
