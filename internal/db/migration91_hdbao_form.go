package db

import (
	"gorm.io/gorm"
)

// migration91 §59.406: 海的宝（hdbao.cc）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（43.3KB 直连抓取）。
// **docs/32 无文档站**（§59.385 咖啡路径第二站——sites.json form+
// 实时 HTML 为源）。实时 HTML 与 4 月资产全槽一致；**tags 资产缺口
// 实时补全**（None→6 项）；[4] 后缀标准形态；单模式站。
// 槽位特化：
//   - **无 url/dburl/durl/imdbid 字段**（实证——控件唯一 url 是编辑器
//     按钮；IMDb/豆瓣信息走 pt_gen）；有 pt_gen+nfo+technical_info+
//     uplver
//   - type 11 项：**408 Concert/演唱会=category.concert 第三站**
//     （浦园/咖啡后）+406 MV 双键+409 Misc/音乐=lossless_music+
//     410 Tutorial=education+411 Others=other；404 纪录双键；
//     405 label 含 HTML 实体（动漫&amp;动画——解码后配 &）
//   - medium 10 项**原盘×DIY 四分**（UHD 原盘/UHD DIY/BD 原盘/
//     BD DIY——DIY 特化 label-only 冬樱先例；原盘两项分立配键）+
//     REMUX 归并+WEB-DL 承接 webrip+DVD/CD 归并 [dvdr,dvd]
//   - codec 8 项合并命名（H.264/AVC/x264 一项——无独立 X26x 联动
//     零触发站方合并语义）；无 VP9；AV1 配键
//   - audiocodec 13 项：DTS:X/TrueHD Atmos/DTS-HD MA/TrueHD
//     独立+DD/AC3 承接 [ac3,dd,ddp] 三键+OGG/APE 配键
//   - standard 6 项**乱序+p/i 分立第五站**（5 4K/8 1080p/2 1080i/
//     7 720p/10 SD/11 Other——值乱序）
//   - team 10 项 label-only：HDBao 官组+外站八组（CHD/MySiLU/
//     WiKi/HDC/CMCT/HDSky/HHClub/HD4FANS）+Other（外站组复利链）
//   - tags 6 项：HDR→四键泛家族第四站（财神/农场/GTK 后）+首发
//     label-only 铁律+禁转/国语/中字/DIY 配键
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(91, "hdbao_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'hdbao.cc' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'hdbao.cc'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "pt_gen": "pt_gen", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Movies/电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "TV Series/电视剧", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "TV Shows/综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "Documentaries/纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "Animations/动漫&动画", "value": "405", "standard_keys": ["category.animation"]}, {"label": "Music Videos/音乐MV", "value": "406", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "Sports/赛事", "value": "407", "standard_keys": ["category.sports"]}, {"label": "Concert/演唱会", "value": "408", "standard_keys": ["category.concert"]}, {"label": "Misc/音乐", "value": "409", "standard_keys": ["category.lossless_music"]}, {"label": "Tutorial/教程", "value": "410", "standard_keys": ["category.education"]}, {"label": "Others/其他", "value": "411", "standard_keys": ["category.other"]}], "medium": [{"label": "UHD Blu-ray 原盘", "value": "1", "standard_keys": ["medium.uhd_bluray"]}, {"label": "UHD Blu-ray DIY", "value": "2"}, {"label": "Blu-ray 原盘", "value": "3", "standard_keys": ["medium.bluray"]}, {"label": "Blu-ray DIY", "value": "4"}, {"label": "REMUX", "value": "5", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "Encode", "value": "6", "standard_keys": ["medium.encode"]}, {"label": "WEB-DL", "value": "7", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "HDTV", "value": "8", "standard_keys": ["medium.hdtv"]}, {"label": "DVD/CD", "value": "12", "standard_keys": ["medium.dvdr", "medium.dvd"]}, {"label": "Other", "value": "13", "standard_keys": ["medium.other"]}], "codec": [{"label": "H.264/AVC/x264", "value": "1", "standard_keys": ["video.h264"]}, {"label": "H.265/HEVC/x265", "value": "7", "standard_keys": ["video.h265"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "MPEG-4", "value": "9", "standard_keys": ["video.mpeg4"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "Xvid", "value": "3", "standard_keys": ["video.xvid"]}, {"label": "AV1", "value": "10", "standard_keys": ["video.av1"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "DTS:X", "value": "8", "standard_keys": ["audio.dtsx"]}, {"label": "TrueHD Atmos", "value": "9", "standard_keys": ["audio.atmos", "audio.truehd_atmos"]}, {"label": "DTS-HD MA", "value": "11", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "TrueHD", "value": "12", "standard_keys": ["audio.truehd"]}, {"label": "LPCM", "value": "13", "standard_keys": ["audio.lpcm"]}, {"label": "DTS", "value": "3", "standard_keys": ["audio.dts"]}, {"label": "FLAC", "value": "15", "standard_keys": ["audio.flac"]}, {"label": "AAC", "value": "1", "standard_keys": ["audio.aac"]}, {"label": "DD/AC3", "value": "2", "standard_keys": ["audio.ac3", "audio.dd", "audio.ddp"]}, {"label": "MP3", "value": "4", "standard_keys": ["audio.mp3"]}, {"label": "OGG", "value": "5", "standard_keys": ["audio.ogg"]}, {"label": "APE", "value": "6", "standard_keys": ["audio.ape"]}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}], "standard": [{"label": "4K/2160p", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "1080p", "value": "8", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "7", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "10", "standard_keys": ["resolution.sd"]}, {"label": "Other", "value": "11", "standard_keys": ["resolution.other"]}], "team": [{"label": "HDBao", "value": "1"}, {"label": "HD4FANS", "value": "10"}, {"label": "CHD", "value": "2"}, {"label": "MySiLU", "value": "3"}, {"label": "WiKi", "value": "4"}, {"label": "HDC", "value": "6"}, {"label": "CMCT", "value": "7"}, {"label": "HDSky", "value": "8"}, {"label": "HHClub", "value": "9"}, {"label": "Other", "value": "5"}], "tags": [{"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "首发", "value": "2"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}]}}`).Error
	})
}
