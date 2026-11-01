package db

import (
	"gorm.io/gorm"
)

// migration100 §59.416: 躺平（www.tangpt.top）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（74.8KB 直连）+ docs/32/tangpt.md
// 462 行读全（rules.php 规则/dupe 优先级/打包规范/标题规范）。
// 实时 vs 文档+资产：全槽一致，唯一漂移 **tags 30→28 项**（站方已删
// 自由(36)/纯享(22)）；type select 实名 name=type（文档称 browsecat
// 为 id 形态——单 type 无双区混入）；文档三处表数勘误（team 称 32
// 实 31/audio 称 19 实 18/medium 称 12 实 11）。
// 槽位映射：
//   - type 9 乱序（414 在 409 前）：414 其他=[other] 非标值+404 纪录
//     双键+406 MV 三键第七站 [mv,music_video,concert]+409 音乐双键。
//   - medium 11 全 canonical：**UHD 独立分立**（1 Blu-ray=[bluray]
//     单键+10 UHD Blu-ray=[uhd_bluray] 单键——浦园/海胆式有独立项
//     分立不归并）+Remux 双键归并+WEB-DL 承接 webrip+CD=[cd]。
//   - codec 8 合并命名（H.264/AVC+H.265/HEVC——无 X26x 联动零触发）
//     +VP8/9=[vp9]。
//   - standard 7 **i/p 合并形态**（咖啡同款）：1080 项双键
//     [r1080p,r1080i]+720/1440/2160/4320 各 i 形态无键单配 p+
//     **480i/480P=[sd]**（extended 活键——织梦/三月先例）+
//     **Other=[resolution.other]**（extended 活键）。
//   - **audiocodec 18 项零 label-only 全配键（全库少有）**：
//     TrueHD Atmos=[atmos,truehd_atmos] 双键（好大先例——组合路径
//     活键：audioMappingOf combined "TrueHD Atmos"→extended 命中）+
//     DTS:X=[dtsx]（好大先例）+**AV3V=[av3v]**（extended 活键——
//     站方独有拼写）+M4A=[m4a]+LPCM=[lpcm]+DD/AC3=[ac3,dd] 双键+
//     DTS/DTS-HD MA/TrueHD/FLAC/APE/MP3/OGG/AAC/WAV/OPUS 全独立。
//   - team 31 label-only：TPWEB(39) 官组+**FRDS(8) 在表直中第八站**
//     +LUCK 系三组+Other(5) 兜底。
//   - tags 28 配键 10：**大包=[big_pack] dict 有键无站消费首配**+
//     **禁转=[restricted]**（extended tag 域活键——禁转六词形）+
//     HDR 四键泛家族第九站+**DV=[dolby_vision] 第六站**+DIY/国语/
//     中字/粤语/完结/分集；首发(45) 铁律 label-only+演唱会/特效/
//     动画/短视频/情色/自制/软件/游戏/书籍/恐怖/娱乐/玄学/写真/
//     cosplay/NSFW/秀人网 18 label-only。
//   - 字段：url+technical_info+uplver；**nfo 不配**（13城教训）+
//     无 pt_gen（站方真无）；候选制走 offers.php 独立页（表单无
//     asoffer 控件）。
// 站规人工把关（垃圾堆系同款）：<100MB 禁+upscale 禁+RMVB 禁+游戏
// 候选制（上传员级）+RAR 禁+Dupe 媒介优先级+断种豁免（45 日/18 月）+
// <5.1 有损音频禁（国粤 2.0 例外）。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(100, "tangpt_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'www.tangpt.top' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'www.tangpt.top'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "电视剧", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "MV", "value": "406", "standard_keys": ["category.mv", "category.music_video", "category.concert"]}, {"label": "体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "其他", "value": "414", "standard_keys": ["category.other"]}, {"label": "音乐", "value": "409", "standard_keys": ["category.music", "category.lossless_music"]}], "medium": [{"label": "Other", "value": "9", "standard_keys": ["medium.other"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "DVDR", "value": "6", "standard_keys": ["medium.dvdr"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "MiniBD", "value": "4", "standard_keys": ["medium.minibd"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "HD DVD", "value": "2", "standard_keys": ["medium.hddvd"]}, {"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "UHD Blu-ray", "value": "10", "standard_keys": ["medium.uhd_bluray"]}, {"label": "WEB-DL", "value": "11", "standard_keys": ["medium.webdl", "medium.webrip"]}], "codec": [{"label": "H.264/AVC", "value": "1", "standard_keys": ["video.h264"]}, {"label": "H.265/HEVC", "value": "6", "standard_keys": ["video.h265"]}, {"label": "AV1", "value": "7", "standard_keys": ["video.av1"]}, {"label": "VP8/9", "value": "10", "standard_keys": ["video.vp9"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "Xvid", "value": "3", "standard_keys": ["video.xvid"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "TrueHD Atmos", "value": "9", "standard_keys": ["audio.atmos", "audio.truehd_atmos"]}, {"label": "DTS-HD MA", "value": "16", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "DTS:X", "value": "15", "standard_keys": ["audio.dtsx"]}, {"label": "DTS", "value": "14", "standard_keys": ["audio.dts"]}, {"label": "TrueHD", "value": "12", "standard_keys": ["audio.truehd"]}, {"label": "LPCM", "value": "19", "standard_keys": ["audio.lpcm"]}, {"label": "WAV", "value": "13", "standard_keys": ["audio.wav"]}, {"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "DD/AC3", "value": "11", "standard_keys": ["audio.ac3", "audio.dd"]}, {"label": "DDP/E-AC3", "value": "10", "standard_keys": ["audio.ddp"]}, {"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "OGG", "value": "5", "standard_keys": ["audio.ogg"]}, {"label": "MP3", "value": "4", "standard_keys": ["audio.mp3"]}, {"label": "M4A", "value": "17", "standard_keys": ["audio.m4a"]}, {"label": "OPUS", "value": "18", "standard_keys": ["audio.opus"]}, {"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "AV3V", "value": "8", "standard_keys": ["audio.av3v"]}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}], "standard": [{"label": "1080i/1080P", "value": "1", "standard_keys": ["resolution.r1080p", "resolution.r1080i"]}, {"label": "2K/1440i/1440P", "value": "4", "standard_keys": ["resolution.r1440p"]}, {"label": "4K/2160i/2160P", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "8K/4320i/4320P", "value": "6", "standard_keys": ["resolution.r4320p"]}, {"label": "Other", "value": "7", "standard_keys": ["resolution.other"]}, {"label": "720i/720P", "value": "2", "standard_keys": ["resolution.r720p"]}, {"label": "480i/480P", "value": "3", "standard_keys": ["resolution.sd"]}], "team": [{"label": "TPWEB", "value": "39"}, {"label": "LUCKMUSIC", "value": "43"}, {"label": "LUCKWEB", "value": "42"}, {"label": "LUCKDIY", "value": "41"}, {"label": "AilMWeb", "value": "40"}, {"label": "QHstudIo", "value": "37"}, {"label": "CHD", "value": "2"}, {"label": "HDS", "value": "1"}, {"label": "U2", "value": "26"}, {"label": "MySiLU", "value": "3"}, {"label": "WiKi", "value": "4"}, {"label": "Other", "value": "5"}, {"label": "StarfallWeb", "value": "6"}, {"label": "CSWEB", "value": "38"}, {"label": "AGSVWEB", "value": "7"}, {"label": "UBWEB", "value": "36"}, {"label": "ZmWeb", "value": "35"}, {"label": "HHWEB", "value": "34"}, {"label": "ADWeb", "value": "31"}, {"label": "FRDS", "value": "8"}, {"label": "OpenCD", "value": "24"}, {"label": "beAst", "value": "23"}, {"label": "HDArea", "value": "21"}, {"label": "FFansDIY", "value": "19"}, {"label": "CMCT", "value": "17"}, {"label": "OurTV", "value": "16"}, {"label": "BtsHD", "value": "15"}, {"label": "HDHome", "value": "14"}, {"label": "PTerWEB", "value": "12"}, {"label": "HDSWEB", "value": "11"}, {"label": "MWeb", "value": "10"}], "tags": [{"label": "首发", "value": "45"}, {"label": "玄学", "value": "35"}, {"label": "自制", "value": "28"}, {"label": "演唱会", "value": "1"}, {"label": "cosplay", "value": "40"}, {"label": "禁转", "value": "39", "standard_keys": ["tag.restricted"]}, {"label": "写真", "value": "38"}, {"label": "NSFW", "value": "41"}, {"label": "讲座", "value": "23"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "粤语", "value": "10", "standard_keys": ["tag.cantonese_audio"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "完结", "value": "19", "standard_keys": ["tag.complete"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}, {"label": "秀人网", "value": "42"}, {"label": "娱乐", "value": "34"}, {"label": "恐怖", "value": "32"}, {"label": "书籍", "value": "31"}, {"label": "游戏", "value": "30"}, {"label": "软件", "value": "29"}, {"label": "情色", "value": "27"}, {"label": "短视频", "value": "24"}, {"label": "分集", "value": "21", "standard_keys": ["tag.ongoing"]}, {"label": "大包", "value": "20", "standard_keys": ["tag.big_pack"]}, {"label": "动画", "value": "15"}, {"label": "DV", "value": "9", "standard_keys": ["tag.dolby_vision"]}, {"label": "特效", "value": "8"}]}}`).Error
	})
}
