package db

import (
	"gorm.io/gorm"
)

// migration73 §59.387: 库非（kufei.org）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（64.7KB 代理抓取——§59.384 附教训
// label 源=站方 HTML 原文）+ docs/32/kufei.md 498 行读全。
// 实时对照发现（sites.json 4 月资产三处漂移+一处 value 级错误）：
//   - **codec 10 站方原文 "Othe"**（拼写截断——sites.json "Other" 被
//     好心修正，§59.384 附同款）
//   - type 406 "音乐、视频"/405 "动画、动漫"（顿号全形态）
//   - **Encode 值=18 非 17**（sites.json 抄错值——17 缺失跳号处错读；
//     value 级错误比 label 更严重，提交值错误）
//   - tags 15 濒危 实时新增（sites.json 13 项无）
// 槽位特化：
//   - **无 url/imdb、无 technical_info 字段**（实时 HTML input 全集
//     实证——文档 §7.1/7.2 正确）；有 pt_gen（4 源）+uplver
//   - category 16 项（全库最多之一）：影视+游戏 410/电子书 411/软件
//     412/教育 413/演唱会 414/戏剧 415/有声读物 416 全配 canonical
//   - medium 17 项（全库最多）：UHD/BD 各分原盘/DIY/Remux 三级+
//     **压制按分辨率细分三维**（UHD 压制 7/1080P/i 压制 8/720P 压制 9）
//     +独立 Encode(18) 通用项——medium.encode→18 安全兜底（站方后加
//     通用项，值跳号 16→18 暗示）；7/8/9 label-only 预览手调；若站方
//     预检要求按分辨率选压制项，首颗真实发布暴露后做 resolution×medium
//     联动（§59.380 同款挂账）；MiniSD(10) 独有键 medium.minisd
//   - codec H/X 并存——**§59.380 压制联动第五站**（X264(2)/X265(4)
//     大写匹配）；AV1(9)
//   - audiocodec 22 项：**DDP/DD+(15)/Dolby Digital/DD(16)/AC3(17)
//     三项分立**（多数站合并 DD/AC3——库非三分 dd/ddp/ac3 各自单键）；
//     DTS X(10) 独立=dtsx；OGG 双值（5/21 站方真重复——5 配键 21
//     label-only 防一键两值）；TTA(22)/MPEG(23) 罕见格式配键
//   - standard 7 项乱序（8K(5) 在 4K(6) 前）：值用实时 HTML
//   - team 5 项极简（HDS/CHD/MySiLU/WiKi/Other）全 label-only
//   - tags 14 项：首发(2) label-only 铁律+**英语(8)=tag.english_audio
//     配键**（幸运站同款语义 §59.326 附三——有英语证据即提交）+禁转/
//     国语/中字/DIY/完结配键+HDR/动漫/音乐/书籍/应求/游戏/濒危
//     label-only
//   - 分叉族双键即防：404 纪录+406 MV
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(73, "kufei_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'kufei.org' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'kufei.org'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "pt_gen": "pt_gen", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Movies/电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "TV Series/电视剧", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "TV Shows/综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "Documentaries/纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "Animations/动画、动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "Music Videos/音乐、视频", "value": "406", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "Sports/体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "Music/音乐", "value": "408", "standard_keys": ["category.music"]}, {"label": "Others/其他", "value": "409", "standard_keys": ["category.other"]}, {"label": "Games/游戏", "value": "410", "standard_keys": ["category.game"]}, {"label": "E-Books/电子书", "value": "411", "standard_keys": ["category.ebook"]}, {"label": "Software/软件", "value": "412", "standard_keys": ["category.software"]}, {"label": "Education/教育", "value": "413", "standard_keys": ["category.education"]}, {"label": "Concern/演唱会", "value": "414", "standard_keys": ["category.concert"]}, {"label": "Drama/戏剧", "value": "415", "standard_keys": ["category.drama"]}, {"label": "Audio Books/有声读物", "value": "416", "standard_keys": ["category.audiobook"]}], "medium": [{"label": "UHD原盘", "value": "1", "standard_keys": ["medium.uhd_bluray"]}, {"label": "UHD DIY", "value": "2"}, {"label": "UHD Remux", "value": "3", "standard_keys": ["medium.uhd_remux"]}, {"label": "BD 原盘", "value": "4", "standard_keys": ["medium.bluray"]}, {"label": "BD DIY", "value": "5"}, {"label": "BD Remux", "value": "6", "standard_keys": ["medium.remux"]}, {"label": "UHD 压制", "value": "7"}, {"label": "1080P/i 压制", "value": "8"}, {"label": "720P 压制", "value": "9"}, {"label": "MiniSD", "value": "10", "standard_keys": ["medium.minisd"]}, {"label": "WEB-DL", "value": "11", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "HDTV", "value": "12", "standard_keys": ["medium.hdtv"]}, {"label": "DVD", "value": "13", "standard_keys": ["medium.dvdr", "medium.dvd"]}, {"label": "CD", "value": "14", "standard_keys": ["medium.cd"]}, {"label": "SACD", "value": "15", "standard_keys": ["medium.sacd"]}, {"label": "Others", "value": "16", "standard_keys": ["medium.other"]}, {"label": "Encode", "value": "18", "standard_keys": ["medium.encode"]}], "codec": [{"label": "H.264/AVC", "value": "1", "standard_keys": ["video.h264"]}, {"label": "X264", "value": "2"}, {"label": "H.265/HEVC", "value": "3", "standard_keys": ["video.h265"]}, {"label": "X265", "value": "4"}, {"label": "VC-1", "value": "5", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2", "value": "6", "standard_keys": ["video.mpeg2"]}, {"label": "MPEG-4", "value": "7", "standard_keys": ["video.mpeg4"]}, {"label": "Xvid", "value": "8", "standard_keys": ["video.xvid"]}, {"label": "AV1", "value": "9", "standard_keys": ["video.av1"]}, {"label": "Othe", "value": "10", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "MP3", "value": "4", "standard_keys": ["audio.mp3"]}, {"label": "OGG", "value": "5", "standard_keys": ["audio.ogg"]}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}, {"label": "TrueHD Atmos", "value": "8", "standard_keys": ["audio.atmos", "audio.truehd_atmos"]}, {"label": "DTS", "value": "9", "standard_keys": ["audio.dts"]}, {"label": "DTS X", "value": "10", "standard_keys": ["audio.dtsx"]}, {"label": "DTS-HDMA", "value": "11", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "DTS-HD HR", "value": "12", "standard_keys": ["audio.dts_hd_hr"]}, {"label": "True-HD", "value": "13", "standard_keys": ["audio.truehd"]}, {"label": "LPCM", "value": "14", "standard_keys": ["audio.lpcm"]}, {"label": "DDP/DD+", "value": "15", "standard_keys": ["audio.ddp"]}, {"label": "Dolby Digital/DD", "value": "16", "standard_keys": ["audio.dd"]}, {"label": "AC3", "value": "17", "standard_keys": ["audio.ac3"]}, {"label": "AAC", "value": "18", "standard_keys": ["audio.aac"]}, {"label": "WAV", "value": "19", "standard_keys": ["audio.wav"]}, {"label": "DSD", "value": "20", "standard_keys": ["audio.dsd"]}, {"label": "OGG", "value": "21"}, {"label": "TTA", "value": "22", "standard_keys": ["audio.tta"]}, {"label": "MPEG", "value": "23", "standard_keys": ["audio.mpeg"]}, {"label": "DDP Atmos", "value": "24", "standard_keys": ["audio.ddp_atmos"]}], "standard": [{"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "8K/4320P", "value": "5", "standard_keys": ["resolution.r4320p"]}, {"label": "4K/UHD/2160P", "value": "6", "standard_keys": ["resolution.r2160p"]}, {"label": "Other", "value": "7", "standard_keys": ["resolution.other"]}], "team": [{"label": "HDS", "value": "1"}, {"label": "CHD", "value": "2"}, {"label": "MySiLU", "value": "3"}, {"label": "WiKi", "value": "4"}, {"label": "Other", "value": "5"}], "tags": [{"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "首发", "value": "2"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR", "value": "7"}, {"label": "英语", "value": "8", "standard_keys": ["tag.english_audio"]}, {"label": "动漫", "value": "9"}, {"label": "音乐", "value": "10"}, {"label": "书籍", "value": "11"}, {"label": "完结", "value": "12", "standard_keys": ["tag.complete"]}, {"label": "应求", "value": "13"}, {"label": "游戏", "value": "14"}, {"label": "濒危", "value": "15"}]}}`).Error
	})
}
