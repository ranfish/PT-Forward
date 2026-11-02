package db

import (
	"gorm.io/gorm"
)

// migration121 §59.438: 劳改所（ptlgs.org）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（70.7KB 直连）+ docs/32/ptlgs.md
// 739 行读全（rules 完整采集+审核脚本标签检测规则）。
// **4 月资产完全失效（末日式第三例）**：七槽值集全变无一幸存——
// type 408 Audio→411 字幕组替换+medium 全套乱序（Blu-ray=14/Remux=8/
// 压制四分立 BDRip/WEBRiP/TVRip/DVDRiP）+audiocodec DTS=19 罕见值+
// team 全组换值+tags 20 项全套换值+standard 无 8K；实时=文档映射表
// 一致（文档后期抓取）——双源可覆盖资产完全失效场景（§59.396 定案
// 持续实证）。
// 槽位映射：
//   - type 9：406 音乐=[music,lossless_music] 双键（实时无 MV 分立）
//     +411 字幕组 label-only（dict 无键——资源形态独有）+404 双键。
//   - **medium 12 乱序+压制四分立（百川/13城同族强化）**：Blu-ray(14)=
//     [bluray,uhd_bluray] 双键归并+Remux(8)=[remux,uhd_remux] 双键+
//     **BDRip(5)=[bdrip]（13城独立键先例）+WEB-DL(4)=[webdl]+WEBRiP(7)=
//     [webrip] 精确分立+TVRip(2)=[tvrip]+DVD(1)=[dvd]+DVDRiP(11)=[dvdr]
//     压制/原盘分立各单键**（16 键 12 项——webdl/webrip 不并键站方
//     语义分立）+CD(15)=[cd]+Other(12)=[other]。
//   - codec 5：H.265(6)/H.264(7)/VC-1/MPEG-2/Other 全配键——无 X26x
//     无 AV1（压制联动零触发+站方无 AV1 选项）。
//   - audiocodec 11：**DTS-HD(10)=[dts_hd_ma,dts_hd_hr] 双键承接**
//     （无 MA/HR 分立——优堡首例）+**DTS=19 罕见值直配**（标准站 3
//     本站 19）+AC-3=[ac3,dd,ddp] 三键承接（无 DDP 分立）+TrueHD
//     无 Atmos 分立=[truehd]+LPCM/FLAC/APE/WAV/**MP3 extended 活键**/
//     AAC/Other；无 DTS:X/Atmos/DDP。
//   - standard 6：2160p(6)/1080p(1)/1080i/720p/SD(4)=[sd]/Other(7)
//     =[resolution.other]——无 8K（乱序非递增非递减）。
//   - team 7：DYZ-WEB/DYZ-Movie/DYZ-TV 官组系+Eleph/beAst 外站+ZmWeb
//     织梦官组 label-only（官组绝不配键）+Other(13)=[team.other] 兜底。
//   - tags 20：**22 独家/17 驻站/1 禁转/2 首发/3 官方 五绝不配**（
//     官方专属财神雷区）+12 键配：**4 DIY/9 原盘=[original_disc]/11
//     REMUX=[remux]（dict 有键无站消费 40+ 站后首配）**+6 中字/5 国语/
//     8 DoVi=[dolby_vision]/7 HDR 四键泛家族（18 站）/10 特效字幕=
//     [special_effects_subs]/15 完结=[complete]/16 分集=[ongoing]/**
//     18 高码率=[high_bitrate]+19 高帧率=[high_frame_rate]（星陨阁
//     首配键复利）**+20 3D/12 SDR/14 WEB-DL label-only（无键）。
//   - 字段：**pt_gen 真字段**（IMDb/豆瓣统一走 pt_gen——无独立 url/
//     dburl 文档 §197 实证）+cover+screenshots+technical_info（必填
//     三件——发布链截图链产出）+uplver+anonymous:true；pos_state/
//     pos_state_until/hr[4]（促销持久字段）不配；无 nfo。
// 站规人工把关：**候选制（10 次候选通过后直发）**+审核脚本标签
//   交叉验证（中字=MI 字幕区/国语=MI 音频/DoVi/HDR=MI format/原盘=
//   副标题"版原盘"/完结=副标题"合集/集全/期全"——站方检测在审核侧
//   非提交侧+发布预览对齐）+非官组禁选官方组+H&R 规则+标题规范。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(121, "ptlgs_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'ptlgs.org' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'ptlgs.org'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "pt_gen": "pt_gen", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "其他", "value": "409", "standard_keys": ["category.other"]}, {"label": "体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "音乐", "value": "406", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "剧集", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "字幕组", "value": "411"}], "medium": [{"label": "Blu-ray", "value": "14", "standard_keys": ["medium.bluray", "medium.uhd_bluray"]}, {"label": "Remux", "value": "8", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "BDRip", "value": "5", "standard_keys": ["medium.bdrip"]}, {"label": "WEB-DL", "value": "4", "standard_keys": ["medium.webdl"]}, {"label": "WEBRiP", "value": "7", "standard_keys": ["medium.webrip"]}, {"label": "HDTV", "value": "3", "standard_keys": ["medium.hdtv"]}, {"label": "TVRip", "value": "2", "standard_keys": ["medium.tvrip"]}, {"label": "DVD", "value": "1", "standard_keys": ["medium.dvd"]}, {"label": "DVDRiP", "value": "11", "standard_keys": ["medium.dvdr"]}, {"label": "CD", "value": "15", "standard_keys": ["medium.cd"]}, {"label": "Other", "value": "12", "standard_keys": ["medium.other"]}], "codec": [{"label": "H.265/HEVC", "value": "6", "standard_keys": ["video.h265"]}, {"label": "H.264/AVC", "value": "7", "standard_keys": ["video.h264"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "DTS-HD", "value": "10", "standard_keys": ["audio.dts_hd_ma", "audio.dts_hd_hr"]}, {"label": "TrueHD", "value": "12", "standard_keys": ["audio.truehd"]}, {"label": "LPCM", "value": "8", "standard_keys": ["audio.lpcm"]}, {"label": "DTS", "value": "19", "standard_keys": ["audio.dts"]}, {"label": "AC-3", "value": "9", "standard_keys": ["audio.ac3", "audio.dd", "audio.ddp"]}, {"label": "AAC", "value": "11", "standard_keys": ["audio.aac"]}, {"label": "FLAC", "value": "13", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "14", "standard_keys": ["audio.ape"]}, {"label": "WAV", "value": "1", "standard_keys": ["audio.wav"]}, {"label": "MP3", "value": "6", "standard_keys": ["audio.mp3"]}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}], "standard": [{"label": "2160p", "value": "6", "standard_keys": ["resolution.r2160p"]}, {"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "Other", "value": "7", "standard_keys": ["resolution.other"]}], "team": [{"label": "DYZ-WEB", "value": "17"}, {"label": "DYZ-Movie", "value": "15"}, {"label": "DYZ-TV", "value": "14"}, {"label": "Eleph", "value": "18"}, {"label": "beAst", "value": "9"}, {"label": "ZmWeb", "value": "11"}, {"label": "Other", "value": "13", "standard_keys": ["team.other"]}], "tags": [{"label": "独家", "value": "22"}, {"label": "驻站", "value": "17"}, {"label": "禁转", "value": "1"}, {"label": "首发", "value": "2"}, {"label": "官方", "value": "3"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "3D", "value": "20"}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "原盘", "value": "9", "standard_keys": ["tag.original_disc"]}, {"label": "REMUX", "value": "11", "standard_keys": ["tag.remux"]}, {"label": "WEB-DL", "value": "14"}, {"label": "DoVi", "value": "8", "standard_keys": ["tag.dolby_vision"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}, {"label": "SDR", "value": "12"}, {"label": "特效字幕", "value": "10", "standard_keys": ["tag.special_effects_subs"]}, {"label": "完结", "value": "15", "standard_keys": ["tag.complete"]}, {"label": "分集", "value": "16", "standard_keys": ["tag.ongoing"]}, {"label": "高码率", "value": "18", "standard_keys": ["tag.high_bitrate"]}, {"label": "高帧率", "value": "19", "standard_keys": ["tag.high_frame_rate"]}]}}`).Error
	})
}
