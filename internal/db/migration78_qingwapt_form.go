package db

import (
	"gorm.io/gorm"
)

// migration78 §59.393: 青蛙（www.qingwapt.com）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（70KB 直连）+ docs/32/qingwapt.md
// 2187 行读全（站规大文档——标题 0day 命名法+转种自查+流媒体缩写）。
// 槽位（七下拉+tags 16——sites.json 4 月资产未捕获 tags/standard 乱序
// 值已实证）：type/source_sel[4]/codec_sel[4]/audiocodec_sel[4]/
// standard_sel[4]/team_sel[4]/tags[4][]。
// 槽位特化：
//   - **medium→source_sel[4]**（财神 §59.381 同款槽位特化——站方媒介
//     下拉叫 source_sel）；11 项含 MiniBD 精确键（§59.325）；Remux
//     归并 uhd_remux；WEB-DL 承接 webrip
//   - type **412 短剧→category.short_drama**（财神 §59.381 附键第二站
//     复用）；404 纪录双键+406 MV 双键
//   - codec 8 项无 X26x——§59.380 压制联动零触发；VP9(8) 配键
//   - audiocodec 18 项：DTS:X(9)=dtsx 独立+DTS-HD HRA(21)=dts_hd_hr
//     （HRA=High Resolution Audio 同语义）+DD/AC3(15)=[ac3,dd] 承接
//     与 DDP/E-AC3(16) 独立分立+M4A/OPUS 配键；AV3A(22) 罕见格式
//     label-only
//   - standard 8 项**首个 2K/1440p 项站**（全分辨率覆盖 8K→SD+Other）
//   - team 5 项全 label-only（FROG/FROGE/FROGWeb/CatEDU 官组保护）
//   - tags 16 项：禁转/国语/粤语/中字/DIY/完结(14)配键；首发无此项✓；
//     VCB-Studio(2)/儿童片(17)/LGBTQ+(19)/特效字幕(20)/系列合集(10)/
//     原生原盘(11)/Remux(15)/杜比视界(12)/HDR10+(13)/HDR(7) label-only
//   - 字段：url（IMDb——url×2 第二个是编辑器按钮）+pt_gen（五来源）+
//     technical_info+uplver；无独立 douban 字段（用 pt_gen）
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(78, "qingwapt_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'www.qingwapt.com' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'www.qingwapt.com'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "source_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "剧集", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "MV", "value": "406", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "音乐", "value": "408", "standard_keys": ["category.music"]}, {"label": "短剧", "value": "412", "standard_keys": ["category.short_drama"]}, {"label": "其他", "value": "409", "standard_keys": ["category.other"]}], "medium": [{"label": "UHD Blu-ray", "value": "1", "standard_keys": ["medium.uhd_bluray"]}, {"label": "Blu-ray", "value": "8", "standard_keys": ["medium.bluray"]}, {"label": "Remux", "value": "9", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "Encode", "value": "10", "standard_keys": ["medium.encode"]}, {"label": "WEB-DL", "value": "7", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "HDTV", "value": "4", "standard_keys": ["medium.hdtv"]}, {"label": "DVD", "value": "2", "standard_keys": ["medium.dvdr", "medium.dvd"]}, {"label": "CD", "value": "3", "standard_keys": ["medium.cd"]}, {"label": "MiniBD", "value": "11", "standard_keys": ["medium.minibd"]}, {"label": "Track", "value": "5", "standard_keys": ["medium.track"]}, {"label": "Other", "value": "6", "standard_keys": ["medium.other"]}], "codec": [{"label": "H.264/AVC", "value": "1", "standard_keys": ["video.h264"]}, {"label": "H.265/HEVC", "value": "6", "standard_keys": ["video.h265"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "AV1", "value": "7", "standard_keys": ["video.av1"]}, {"label": "MPEG-4", "value": "3", "standard_keys": ["video.mpeg4"]}, {"label": "VP9", "value": "8", "standard_keys": ["video.vp9"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "DTS:X", "value": "9", "standard_keys": ["audio.dtsx"]}, {"label": "DTS", "value": "14", "standard_keys": ["audio.dts"]}, {"label": "DTS-HD MA", "value": "10", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "DTS-HD HRA", "value": "21", "standard_keys": ["audio.dts_hd_hr"]}, {"label": "TrueHD Atmos", "value": "11", "standard_keys": ["audio.atmos", "audio.truehd_atmos"]}, {"label": "TrueHD", "value": "12", "standard_keys": ["audio.truehd"]}, {"label": "LPCM", "value": "13", "standard_keys": ["audio.lpcm"]}, {"label": "DD/AC3", "value": "15", "standard_keys": ["audio.ac3", "audio.dd"]}, {"label": "DDP/E-AC3", "value": "16", "standard_keys": ["audio.ddp"]}, {"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "AAC", "value": "17", "standard_keys": ["audio.aac"]}, {"label": "APE", "value": "18", "standard_keys": ["audio.ape"]}, {"label": "WAV", "value": "19", "standard_keys": ["audio.wav"]}, {"label": "MP3", "value": "4", "standard_keys": ["audio.mp3"]}, {"label": "M4A", "value": "8", "standard_keys": ["audio.m4a"]}, {"label": "OPUS", "value": "20", "standard_keys": ["audio.opus"]}, {"label": "AV3A", "value": "22"}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}], "standard": [{"label": "8K/4320p", "value": "6", "standard_keys": ["resolution.r4320p"]}, {"label": "4K/2160p", "value": "7", "standard_keys": ["resolution.r2160p"]}, {"label": "2K/1440p", "value": "8", "standard_keys": ["resolution.r1440p"]}, {"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "Other", "value": "5", "standard_keys": ["resolution.other"]}], "team": [{"label": "FROG", "value": "6"}, {"label": "FROGE", "value": "7"}, {"label": "FROGWeb", "value": "8"}, {"label": "CatEDU", "value": "10"}, {"label": "Other", "value": "5"}], "tags": [{"label": "VCB-Studio", "value": "2"}, {"label": "儿童片", "value": "17"}, {"label": "LGBTQ+", "value": "19"}, {"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "特效字幕", "value": "20"}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "粤语", "value": "8", "standard_keys": ["tag.cantonese_audio"]}, {"label": "完结", "value": "14", "standard_keys": ["tag.complete"]}, {"label": "系列合集", "value": "10"}, {"label": "原生原盘", "value": "11"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "Remux", "value": "15"}, {"label": "杜比视界", "value": "12"}, {"label": "HDR10+", "value": "13"}, {"label": "HDR", "value": "7"}]}}`).Error
	})
}
