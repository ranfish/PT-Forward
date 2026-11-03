package db

import (
	"gorm.io/gorm"
)

// migration136 §59.454: 城市（hdcity.city）发布表单配置分发——
// **两步上传站**（NexusPHP 深度定制 ProCity 主题——全库唯一
// "先上传种子文件再填表单"站，用户逐点讨论定案+第一步实测授权）。
// 依据：docs/32/hdcity.md 556 行读全+第一步实测（302 Location
// tfu/tn/f/s/n/ih 参数解明）+第二步表单实时实证（45.9KB 控件
// 与文档完全吻合）+t-68751/t-68754 详情页实态（bigname 双填）。
// 核心机制：
//   - **两步上传 adapter**：hdcityUploadTwoStep（第一步 GET /upload
//     解析 vf 动态字段→POST 种子到跨域接收器→302 Location 解析
//     →第二步 POST 表单到 /upload?tfu=...）
//   - **bigname/small_descr 双填副标题**（站方实态双行同值——空态
//     拼装退化"译名 | MainTitle | Year"）
//   - **codec 混音视频 17 项单槽**：视频项配 video.* 键（影视类
//     通用链）+音频项配 audio.* 键（音频类 hdcityCodecOption 特化
//     切换数据源——§2.7 站方编码-媒介对应表权威锚）
//   - **tag1ing 题材注入**（hdcityTag1ingValue——◎类别 100% 必有
//     29 库 315/315 实证；27 项映射表+行内多题材按序首中）
//   - **字幕四 checkbox 全自动**（sub_soft MI Text 轨/sub_hard
//     三信号反推（无轨∧无文件∧中字标签）/sub_incl 文件扫描/
//     chnize 标签——hdcityTagDependentFill）
//   - **processing_sel 3D 特化**（HOU→1/HSBS→2/SBS·OU→5 Alt/
//     空→不投——交织(3)/红蓝(4)无词形预览人工）
//   - **team 恒不投递**（官组 NoVA/NoPA/NoTA/NoXA+0DAY 全
//     label-only——非官组不得 HDCITY- 前缀+非 Scene 不选 0DAY）
//   - **tag2ing 音乐流派 46 项 label-only**（发布链无流派数据源
//     ——音乐种预览人工）
//   - 字段：type/medium_sel/codec_sel/standard_sel/processing_sel/
//     team_sel/name/bigname/small_descr/url(IMDb)/posterimg/descr/
//     mediainfo（非 technical_info）/uplver；tag1ing/tag2ing/
//     sub_soft/sub_hard/sub_incl/chnize 全特化直写不进 form_fields
//     （无域常量+值形态特殊——字符串 value 下拉/独立 checkbox）
//   - IMDb 必填（站方规则：电影/剧集必须填）——form_fields 配
//     imdb_url 域
// 站规人工把关：Dupe 规则（§2.5）/RMVB·CAM 禁/无 CUE 的 CD 映像
//   禁/体育节目禁泄结果/简介必含海报+文件详情。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(136, "hdcity_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'hdcity.city' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'hdcity.city'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel", "codec": "codec_sel", "standard": "standard_sel", "team": "team_sel", "name": "name", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Movies/电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "Series/剧集", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "Shows/节目", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "Doc/档案记录", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "Anim/动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "MV/音乐视频", "value": "406", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "Sports/体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "Audio/音频", "value": "408", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "Other/其他", "value": "409", "standard_keys": ["category.other"]}, {"label": "XXX/家长指引", "value": "727"}, {"label": "Edu/文档/教材", "value": "728", "standard_keys": ["category.education", "category.ebook"]}, {"label": "Soft/软件", "value": "729", "standard_keys": ["category.software"]}], "medium": [{"label": "BD/蓝光原盘", "value": "1", "standard_keys": ["medium.bluray", "medium.uhd_bluray"]}, {"label": "HDDVD原盘", "value": "2", "standard_keys": ["medium.hddvd"]}, {"label": "Remux/重混流", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "MiniBD/微蓝光", "value": "4", "standard_keys": ["medium.minibd"]}, {"label": "HDTV/SNG/原始录制", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "DVD原盘", "value": "6", "standard_keys": ["medium.dvd", "medium.dvdr"]}, {"label": "Encode/重编码", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "CD/音乐/有声读物", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "Track/外挂音轨", "value": "9", "standard_keys": ["medium.track"]}, {"label": "Ebook/文档/图库", "value": "10"}, {"label": "Rec/视频教材", "value": "11"}, {"label": "Joy/游戏", "value": "12"}, {"label": "Prog/程序", "value": "13"}], "codec": [{"label": "H.264/AVC", "value": "1", "standard_keys": ["video.h264"]}, {"label": "H.265/HEVC", "value": "13", "standard_keys": ["video.h265"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "DivX/XviD", "value": "3", "standard_keys": ["video.xvid"]}, {"label": "WMV/VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "AV1", "value": "16", "standard_keys": ["video.av1"]}, {"label": "WebM/VP", "value": "17", "standard_keys": ["video.vp9"]}, {"label": "WMA/WMA-LL", "value": "14"}, {"label": "FLAC", "value": "5", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "6", "standard_keys": ["audio.ape"]}, {"label": "DTS/DTS-ES", "value": "7", "standard_keys": ["audio.dts"]}, {"label": "Dolby AC3", "value": "8", "standard_keys": ["audio.ac3", "audio.dd"]}, {"label": "TrueHD/Atmos", "value": "15", "standard_keys": ["audio.truehd", "audio.atmos"]}, {"label": "WAV/Raw", "value": "10", "standard_keys": ["audio.wav"]}, {"label": "MP3/MP2", "value": "11", "standard_keys": ["audio.mp3", "audio.mp2"]}, {"label": "AAC/M4A", "value": "12", "standard_keys": ["audio.aac"]}, {"label": "Other", "value": "9", "standard_keys": ["video.other"]}], "standard": [{"label": "8K-4320p", "value": "11", "standard_keys": ["resolution.r4320p"]}, {"label": "8K-4320i", "value": "10"}, {"label": "4K-2160p", "value": "9", "standard_keys": ["resolution.r2160p"]}, {"label": "4K-2160i", "value": "8"}, {"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "720i", "value": "7"}, {"label": "540p", "value": "6"}, {"label": "480p", "value": "5"}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}], "processing": [{"label": "3D H-OU/上下半宽", "value": "1"}, {"label": "3D H-SBS/左右半宽", "value": "2"}, {"label": "3D Interleaved/交织", "value": "3"}, {"label": "3D Red-blue/红蓝", "value": "4"}, {"label": "3D Alt/其他3D", "value": "5"}], "team": [{"label": "HDCITY-NoVA", "value": "1"}, {"label": "HDCITY-NoPA", "value": "14"}, {"label": "HDCITY-NoTA", "value": "15"}, {"label": "HDCITY-NoXA", "value": "17"}, {"label": "0DAY", "value": "9"}]}}`).Error
	})
}
