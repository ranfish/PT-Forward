package db

import (
	"gorm.io/gorm"
)

// migration103 §59.419: 红豆饭（hdfans.org）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（59.7KB 代理抓取）+ docs/32/hdfans.md
// 760 行读全（rules.php+论坛 topicid=2522 发种规范+§5.1 媒介细分
// 权威规则+§8.1 制作组黑名单+H&R V2.0）。
// 实时 vs 文档+资产：**全槽零漂移**（第十个零漂移站——含 416 综艺/
// 417 动漫非标值集/407 MV 与 408 Concert 分立/medium 19 项中文细分）。
// 槽位映射：
//   - type 16 项全库最多之一：**407 Music Videos 与 408 Concert 分立
//     首站**（407=[mv,music_video] 双键+408=[concert] 独立单键——历史
//     站均为合并项三键承接，本站真分立）+416 综艺=[tv_shows]/417
//     动漫=[animation] 非标值集+403 纪录双键+406 音乐双键+404
//     Education/405 audiobook/418 sports/419 software/421 game/
//     423 ebook 全配+409 Drama/410 Others。
//   - medium 19 项中文细分（全库最多之一）：UHD/BD 四分（原盘/DIY/
//     Remux/压制）+**压制按分辨率四分（20 UHD压制/24 1080P/i压制/
//     25 720P压制/26 MiniSD）+音乐系（SACD/CD+VCD/CD+DVD/黑胶）**——
//     §59.407 DIY 联动自动生效（18/22 label-only 动态检测）+
//     **★resolutionEncodeOption 压制分辨率分发新实施**（本函数外
//     executor.go §59.419——站方 §5.1 权威规则锚：Encode+分辨率→
//     对应压制项；§59.387 库非挂账第二站转通用实施）；encode 键不
//     配（无通用 Encode 项——细分分发接管）；DVD=[dvd,dvdr] 双键
//     承接；MiniSD/SACD/CD+VCD/CD+DVD/黑胶 label-only。
//   - codec 10 项 **H/X 分立——§59.380 压制联动第八站**（x264(2)/
//     x265(4) label-only 联动通道自动生效——站方 §5.2 编码细分规则
//     印证）+MPEG-4/Xvid/AV1/VC-1 全配。
//   - standard 7 项 p/i 分立全配键：8K/4K/1080P/1080i/720P/SD/
//     Other=[resolution.other]。
//   - **audiocodec 23 项全配键（全库最多级第三例——好大 28 后）**：
//     **DD(19)/DDP(21)/AC3(10) 三分立**（各自单键——多数站合并）+
//     TrueHD Atmos=[atmos,truehd_atmos] 双键+DDP Atmos=[ddp_atmos]+
//     DTS:X=[dtsx]+DSD/TTA/MPEG/ALAC/m4a 全独立。
//   - team 29 项 label-only：HDFans(9) 官组+**FRDS(6) 直中第九站**+
//     国际组（PTP/BTN/NTb/EPSiLON/FraMeSToR/DIC/Red/GGN）+
//     Others(27) 兜底；**§8.1 黑名单 11 公网组**（Hao4K/Mp4ba/rag/
//     rarbg/rartv/FGT/xiaomi/huawei/GPTHD/DreamHD/SONYHD）发布
//     预览人工规避（公网组不在官组表——实际低风险）。
//   - processing_sel 12 项=产区槽不配（咖啡定案同名异义——13城同款）。
//   - tags 27 项配键 9：**中英双语=[bilingual_audio] 第二站**（垃圾
//     堆后——站方 §5.4 双语标签增强规则印证：双语字幕语义精确）+
//     禁转=restricted+HDR 四键泛家族第十二站+Dolby Vision 第八站+
//     DIY/国语/中字/粤语/完结；首发(15) 铁律 label-only+官字组(14)
//     财神雷区+Atmos(12)/CC收藏(22) **dict 死键不配**（extended 零
//     产出——配键无收益 label-only）+微光星辰/甄选/驻站/原创/限转/
//     源站转发/原生/特效/4K/8K/Hi-Res/刮削/AI修复/保种 label-only。
//   - 字段：url+pt_gen+technical_info+uplver；nfo 不配（13城教训）。
// 站规人工把关：全站 H&R V2.0（考核+HP 系统）+盒子规则+黑名单组+
// 游戏候选制+Dupe 媒介优先级+色情禁。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(103, "hdfans_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'hdfans.org' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'hdfans.org'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Movies/电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "TV Series/电视剧", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "Documentaries/纪录片", "value": "403", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "Music/音乐", "value": "406", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "TV Shows/综艺", "value": "416", "standard_keys": ["category.tv_shows"]}, {"label": "Animations/动漫、动画", "value": "417", "standard_keys": ["category.animation"]}, {"label": "Music Videos/音乐视频", "value": "407", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "Concert/演唱会", "value": "408", "standard_keys": ["category.concert"]}, {"label": "Education/教育", "value": "404", "standard_keys": ["category.education"]}, {"label": "Audio Books/有声读物", "value": "405", "standard_keys": ["category.audiobook"]}, {"label": "Drama/戏剧", "value": "409"}, {"label": "Sports/体育", "value": "418", "standard_keys": ["category.sports"]}, {"label": "Software/软件", "value": "419", "standard_keys": ["category.software"]}, {"label": "Games/游戏", "value": "421", "standard_keys": ["category.game"]}, {"label": "E-Books/电子书", "value": "423", "standard_keys": ["category.ebook"]}, {"label": "Others/其他", "value": "410", "standard_keys": ["category.other"]}], "medium": [{"label": "UHD原盘", "value": "17", "standard_keys": ["medium.uhd_bluray"]}, {"label": "UHD DIY", "value": "18"}, {"label": "UHD Remux", "value": "19", "standard_keys": ["medium.uhd_remux"]}, {"label": "UHD压制", "value": "20"}, {"label": "BD原盘", "value": "21", "standard_keys": ["medium.bluray"]}, {"label": "BD DIY", "value": "22"}, {"label": "BD Remux", "value": "23", "standard_keys": ["medium.remux"]}, {"label": "1080P/i压制", "value": "24"}, {"label": "720P压制", "value": "25"}, {"label": "MiniSD", "value": "26"}, {"label": "WEB-DL", "value": "5", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "HDTV", "value": "6", "standard_keys": ["medium.hdtv"]}, {"label": "DVD", "value": "7", "standard_keys": ["medium.dvd", "medium.dvdr"]}, {"label": "CD", "value": "9", "standard_keys": ["medium.cd"]}, {"label": "SACD", "value": "16"}, {"label": "CD+VCD", "value": "30"}, {"label": "CD+DVD", "value": "27"}, {"label": "黑胶", "value": "28"}, {"label": "Other", "value": "10", "standard_keys": ["medium.other"]}], "codec": [{"label": "H.264/AVC", "value": "1", "standard_keys": ["video.h264"]}, {"label": "x264", "value": "2"}, {"label": "H.265/HEVC", "value": "3", "standard_keys": ["video.h265"]}, {"label": "x265", "value": "4"}, {"label": "VC-1", "value": "5", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2", "value": "10", "standard_keys": ["video.mpeg2"]}, {"label": "MPEG-4", "value": "11", "standard_keys": ["video.mpeg4"]}, {"label": "Xvid", "value": "12", "standard_keys": ["video.xvid"]}, {"label": "AV1", "value": "14", "standard_keys": ["video.av1"]}, {"label": "Other", "value": "13", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "TrueHD Atmos", "value": "1", "standard_keys": ["audio.atmos", "audio.truehd_atmos"]}, {"label": "DTS:X", "value": "3", "standard_keys": ["audio.dtsx"]}, {"label": "True-HD", "value": "6", "standard_keys": ["audio.truehd"]}, {"label": "DTS-HDMA", "value": "4", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "DTS-HD HR", "value": "5", "standard_keys": ["audio.dts_hd_hr"]}, {"label": "DTS", "value": "2", "standard_keys": ["audio.dts"]}, {"label": "LPCM", "value": "7", "standard_keys": ["audio.lpcm"]}, {"label": "WAV", "value": "14", "standard_keys": ["audio.wav"]}, {"label": "APE", "value": "13", "standard_keys": ["audio.ape"]}, {"label": "FLAC", "value": "12", "standard_keys": ["audio.flac"]}, {"label": "DSD", "value": "20", "standard_keys": ["audio.dsd"]}, {"label": "DDP Atmos", "value": "23", "standard_keys": ["audio.ddp_atmos"]}, {"label": "DDP/DD+/EAC3", "value": "21", "standard_keys": ["audio.ddp"]}, {"label": "Dolby Digital/DD", "value": "19", "standard_keys": ["audio.dd"]}, {"label": "MPEG", "value": "22", "standard_keys": ["audio.mpeg"]}, {"label": "OGG", "value": "15", "standard_keys": ["audio.ogg"]}, {"label": "AAC", "value": "11", "standard_keys": ["audio.aac"]}, {"label": "AC3", "value": "10", "standard_keys": ["audio.ac3"]}, {"label": "TTA", "value": "16", "standard_keys": ["audio.tta"]}, {"label": "MP3", "value": "17", "standard_keys": ["audio.mp3"]}, {"label": "ALAC", "value": "25", "standard_keys": ["audio.alac"]}, {"label": "m4a", "value": "24", "standard_keys": ["audio.m4a"]}, {"label": "Other", "value": "18", "standard_keys": ["audio.other"]}], "standard": [{"label": "8K/4320P", "value": "1", "standard_keys": ["resolution.r4320p"]}, {"label": "4K/UHD/2160P", "value": "2", "standard_keys": ["resolution.r2160p"]}, {"label": "1080P", "value": "3", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "4", "standard_keys": ["resolution.r1080i"]}, {"label": "720P", "value": "5", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "6", "standard_keys": ["resolution.sd"]}, {"label": "Other", "value": "7", "standard_keys": ["resolution.other"]}], "team": [{"label": "HDFans", "value": "9"}, {"label": "CHDBits", "value": "1"}, {"label": "HDC", "value": "2"}, {"label": "TTG", "value": "19"}, {"label": "WiKi", "value": "3"}, {"label": "beAst", "value": "4"}, {"label": "CMCT", "value": "5"}, {"label": "FRDS", "value": "6"}, {"label": "HDS", "value": "7"}, {"label": "OurBits", "value": "17"}, {"label": "PTer", "value": "20"}, {"label": "Audies", "value": "29"}, {"label": "Ubits", "value": "41"}, {"label": "HHClub", "value": "34"}, {"label": "HDHome", "value": "18"}, {"label": "PTHome", "value": "16"}, {"label": "QHstudIo", "value": "40"}, {"label": "Hares", "value": "28"}, {"label": "TLF", "value": "8"}, {"label": "PTP", "value": "37"}, {"label": "BTN/NTb", "value": "32"}, {"label": "EPSiLON", "value": "30"}, {"label": "FraMeSToR", "value": "31"}, {"label": "OpenCD", "value": "33"}, {"label": "DIC", "value": "35"}, {"label": "Red", "value": "36"}, {"label": "GGN", "value": "39"}, {"label": "LemonHD", "value": "26"}, {"label": "Others", "value": "27"}], "tags": [{"label": "微光星辰", "value": "2"}, {"label": "官字组", "value": "14"}, {"label": "甄选", "value": "16"}, {"label": "驻站", "value": "24"}, {"label": "首发", "value": "15"}, {"label": "原创", "value": "13"}, {"label": "禁转", "value": "3", "standard_keys": ["tag.restricted"]}, {"label": "限转", "value": "8"}, {"label": "源站转发", "value": "29"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "原生", "value": "21"}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "粤语", "value": "10", "standard_keys": ["tag.cantonese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "中英双语", "value": "17", "standard_keys": ["tag.bilingual_audio"]}, {"label": "特效", "value": "18"}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}, {"label": "Dolby Vision", "value": "9", "standard_keys": ["tag.dolby_vision"]}, {"label": "Atmos", "value": "12"}, {"label": "4K", "value": "19"}, {"label": "8K", "value": "20"}, {"label": "CC收藏", "value": "22"}, {"label": "Hi-Res", "value": "30"}, {"label": "完结", "value": "23", "standard_keys": ["tag.complete"]}, {"label": "刮削", "value": "28"}, {"label": "AI修复", "value": "27"}, {"label": "保种", "value": "26"}]}}`).Error
	})
}
