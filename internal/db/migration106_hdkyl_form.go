package db

import (
	"gorm.io/gorm"
)

// migration106 §59.422: 麒麟（www.hdkyl.in）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（56.8KB 代理抓取）+ docs/32/hdkyl.md
// 1295 行读全（rules+审核脚本完整逆向 20+ 校验规则+黑名单 23 组+
// 官种/驻站识别体系+标题解析逻辑）。
// **实时 vs 4 月资产完全失效第二例**（末日式——七槽值集全变无一
// 幸存：type 409 Misc→420 TV Shows 新值集/medium 1-10→24-34/
// codec 合并 MPEG/source 1-18→14-35/team 全换 HDK 系/tags 值全变）
// ——文档（后期抓取）与实时基本一致；实时 vs 文档微漂移：tags 16→18
// （实时多 零魔(13)/麒麟火(17)）+4 K(11) 带空格。
// 槽位映射：
//   - type 14 项：404 Record Education 纪录教育=[documentary,
//     documentaries] 双键+408 HQ Audio 双键+**421 Playlet 短剧
//     第十二站**+406 Music Videos 三键第十站+419 Study=[education]
//     +411 software/412 Game/413 Ebook/409 Misc/420 TV Shows 全配。
//   - processing_sel=**年份槽**（2025→2016→Earlier 11 项——NP 同名
//     字段第五种语义）不配（慕雪阁定案：发布链无年份投递）。
//   - source_sel=产区槽 18 项（18 站全库最细之一）不配（咖啡定案
//     同名异义）。
//   - medium 10 项值 24-34：**UHD Blu-ray(24)/Blu-ray(25) 分立各
//     单键**+DVD(原盘)=[dvd,dvdr] 双键+REMUX=[remux,uhd_remux]
//     双键归并+WEB-DL 承接 webrip+Track/CD；无 MiniBD/HD DVD
//     （miss 落空）。
//   - codec 7 项合并命名零联动：MPEG-2/MPEG-4=[mpeg2,mpeg4] 双键
//     +AV1/Xvid/VC-1。
//   - standard 7 项 i/p 合并形态（咖啡/躺平同款）：8K/4K/2K/1080
//     双键+720p/480p=[sd]/Other=[resolution.other]。
//   - **audiocodec 18 项全配键第四例**：TrueHD Atmos=[atmos,
//     truehd_atmos] 双键+DTS:X=[dtsx]+DTS 四级细分全配+M4A/MPEG/
//     Opus+DD/AC3=[ac3,dd] 双键+DDP 分立。
//   - team 9 项 label-only：HDK 系官组（HDK/HDKWeb/HDKGame）+
//     Kylin/RedLeaves/CatEDU/GodDramas/StarfallWeb+Other(5) 兜底
//     （FRDS 不在表）；**审核脚本黑名单 23 组**（fgt/rarbg/hao4k/
//     xiaomi 等——发布预览人工规避）。
//   - **tags 18 项配键 9**：**英字=[english_subtitle] 第三站**+
//     **特效字幕=[special_effects_subs] 第二站**+**DV=[dolby_
//     vision] 第十站**+**HDR 四键泛家族第十三站**+禁转=restricted+
//     DIY/国语/中字/完结/分集；**官种(3) 绝不配键**（审核脚本非官
//     种禁选错误级——财神雷区+站方标题含 HDK 判官种双向校验）+
//     首发(2) 铁律+驻站/Blu-ray/LIVE现场/4 K（带空格零产出）/
//     零魔/麒麟火 label-only。
//   - 字段：url+pt_gen+technical_info+uplver；nfo 不配（13城教训）；
//     pos_state 促销字段不配（GTK 同款）。
// 站规人工把关：黑名单 23 组+官种/驻站标签规则矩阵（HDK 官种→
// 官方+麒麟火必选/GodDramas 驻站→麒麟火必选/非官种禁选官方）+
// 审核 20+ 规则+标题规范。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(106, "hdkyl_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'www.hdkyl.in' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'www.hdkyl.in'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Movies/电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "TV Series/电视剧", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "Record Education/纪录教育", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "HQ Audio/音乐", "value": "408", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": " Playlet/短剧", "value": "421", "standard_keys": ["category.short_drama"]}, {"label": "Animations/动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "Study/学习", "value": "419", "standard_keys": ["category.education"]}, {"label": "Sports/体育运动", "value": "407", "standard_keys": ["category.sports"]}, {"label": "Music Videos/音乐视频", "value": "406", "standard_keys": ["category.mv", "category.music_video", "category.concert"]}, {"label": "software/软件", "value": "411", "standard_keys": ["category.software"]}, {"label": "Game/游戏", "value": "412", "standard_keys": ["category.game"]}, {"label": "Ebook/电子书", "value": "413", "standard_keys": ["category.ebook"]}, {"label": "Misc/其他", "value": "409", "standard_keys": ["category.other"]}, {"label": "TV Shows/综艺", "value": "420", "standard_keys": ["category.tv_shows"]}], "medium": [{"label": "UHD Blu-ray", "value": "24", "standard_keys": ["medium.uhd_bluray"]}, {"label": "Blu-ray(原盘)", "value": "25", "standard_keys": ["medium.bluray"]}, {"label": "DVD(原盘)", "value": "27", "standard_keys": ["medium.dvd", "medium.dvdr"]}, {"label": "HDTV", "value": "28", "standard_keys": ["medium.hdtv"]}, {"label": "Encode", "value": "29", "standard_keys": ["medium.encode"]}, {"label": "REMUX", "value": "30", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "WEB-DL", "value": "31", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "Track", "value": "32", "standard_keys": ["medium.track"]}, {"label": "CD", "value": "33", "standard_keys": ["medium.cd"]}, {"label": "Other", "value": "34", "standard_keys": ["medium.other"]}], "codec": [{"label": "H.265/HEVC", "value": "6", "standard_keys": ["video.h265"]}, {"label": "H.264/AVC", "value": "1", "standard_keys": ["video.h264"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "Xvid", "value": "3", "standard_keys": ["video.xvid"]}, {"label": "MPEG-2/MPEG-4", "value": "4", "standard_keys": ["video.mpeg2", "video.mpeg4"]}, {"label": "AV1", "value": "7", "standard_keys": ["video.av1"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "DTS-HD MA", "value": "8", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "DTS:X", "value": "16", "standard_keys": ["audio.dtsx"]}, {"label": "DTS-HD HR", "value": "19", "standard_keys": ["audio.dts_hd_hr"]}, {"label": "DTS", "value": "3", "standard_keys": ["audio.dts"]}, {"label": "TrueHD", "value": "9", "standard_keys": ["audio.truehd"]}, {"label": "TrueHD Atmos", "value": "15", "standard_keys": ["audio.atmos", "audio.truehd_atmos"]}, {"label": "DD/AC3", "value": "11", "standard_keys": ["audio.ac3", "audio.dd"]}, {"label": "DDP/E-AC3", "value": "17", "standard_keys": ["audio.ddp"]}, {"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "LPCM", "value": "10", "standard_keys": ["audio.lpcm"]}, {"label": "APE", "value": "12", "standard_keys": ["audio.ape"]}, {"label": "WAV", "value": "13", "standard_keys": ["audio.wav"]}, {"label": "M4A", "value": "14", "standard_keys": ["audio.m4a"]}, {"label": "MPEG", "value": "18", "standard_keys": ["audio.mpeg"]}, {"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "MP3", "value": "4", "standard_keys": ["audio.mp3"]}, {"label": "Opus", "value": "20", "standard_keys": ["audio.opus"]}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}], "standard": [{"label": "8K/4320p/4320i", "value": "7", "standard_keys": ["resolution.r4320p"]}, {"label": "4K/2160p/2160i", "value": "6", "standard_keys": ["resolution.r2160p"]}, {"label": "2K/1440p/1440i", "value": "10", "standard_keys": ["resolution.r1440p"]}, {"label": "1080p/1080i", "value": "1", "standard_keys": ["resolution.r1080p", "resolution.r1080i"]}, {"label": "720p/720i", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "480p/480i", "value": "8", "standard_keys": ["resolution.sd"]}, {"label": "Other", "value": "9", "standard_keys": ["resolution.other"]}], "team": [{"label": "HDK", "value": "6"}, {"label": "HDKWeb", "value": "7"}, {"label": "HDKGame", "value": "8"}, {"label": "Kylin", "value": "12"}, {"label": "RedLeaves", "value": "14"}, {"label": "CatEDU", "value": "10"}, {"label": "GodDramas", "value": "9"}, {"label": "StarfallWeb", "value": "13"}, {"label": "Other", "value": "5"}], "tags": [{"label": "官种", "value": "3"}, {"label": "驻站", "value": "15"}, {"label": "首发", "value": "2"}, {"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "特效字幕", "value": "21", "standard_keys": ["tag.special_effects_subs"]}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "英字", "value": "18", "standard_keys": ["tag.english_subtitle"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}, {"label": "Dolby Vision", "value": "8", "standard_keys": ["tag.dolby_vision"]}, {"label": "Blu-ray", "value": "9"}, {"label": "LIVE现场", "value": "10"}, {"label": "4 K", "value": "11"}, {"label": "零魔", "value": "13"}, {"label": "完结", "value": "16", "standard_keys": ["tag.complete"]}, {"label": "分集", "value": "19", "standard_keys": ["tag.ongoing"]}, {"label": "麒麟火", "value": "17"}]}}`).Error
	})
}
