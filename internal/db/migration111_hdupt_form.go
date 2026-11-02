package db

import (
	"gorm.io/gorm"
)

// migration111 §59.427: 好多油（pt.hdupt.com）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（44.2KB 直连）+ docs/32/hdupt.md
// 426 行读全（rules.php+论坛 #304 发种细则全八类）。
// 实时 vs 文档 vs 4 月资产全槽一致。
// 槽位映射：
//   - type 10：401/402/403/404 双键/405/406 双键/407/408 双键/411/410
//     标准键直配（402 剧集单分类——TV 分流在 medium 域）。
//   - **medium 15 电影/剧集 TV 双轨**（站方媒介 Remux/Encode/WEB-DL 各带
//     TV 后缀分立项——§59.427 hduptTVMediumOption 挂 mediumMappingOf：
//     tv_series ∧ remux→12 Remux TV/uhd_remux→16 UHD Remux TV/encode→14
//     Encode TV/webdl·webrip→13 WEB-DL TV；电影形态主键归 Remux/Encode/
//     WEB-DL——TV 4 项 label-only 特化独占；原盘/CD 等无 TV 分立走通用）。
//     mediumMappingOf 签名扩展 siteName+meta（§59.380 codecMappingOf 同款）。
//   - codec 7 **x264-only 半分立**（x264(16) label-only 压制联动通道+
//     **无 x265 项**——压制 h264 联动 x264/压制 h265 lookupCLIVariant
//     miss→H.265/HEVC 通用兜底=站方语义）+MPEG/MPEG-2=[mpeg2]。
//   - standard 6：1080p/1080i/4K/720p/SD+iPad(6) label-only（无键）。
//   - audiocodec 12：**AC3/EAC3=[ac3,dd,ddp] 三键合并**（无 DDP 分立——
//     车站家族承接式）+**TrueHD=[truehd,truehd_atmos] 双键**（无 Atmos
//     分立——Atmos 种归 TrueHD 项家族承接）+DTS:X/DTS-HDMA/LPCM/DTS/
//     AAC/FLAC/APE/WAV/MPEG 全配键。
//   - team 2 label-only 全库最少之一：HDU 官组+Other(5) 兜底。
//   - **processing_sel=地区槽不配**（FieldDomainProcessing 常量不存在——
//     发布链无投递能力，历史定案持续；8 地区含 India/SEA 全库最细——
//     预览人工选）。
//   - **无 tags 槽**（实时资产双 None）；**无 url/pt_gen/technical_info**
//     （实时实证——简介-only 形态，海的宝同款）；nfo 不配；uplver+
//     anonymous:true。
// 站规人工把关：**Dupe 严格**（已有 BluRay Encode 禁发 WEB-DL/HDTV 同
// 分辨率单份例外 264/265×HDR/SDR 各一/每季完结后禁发单集——§59.424
// 系统分集拦截上游已拦 ✓/官方小组优先一切）+禁二次编码 BR/HR-HDTV+
// 禁 WEB-DL 重编码+禁非官方打包合集+H&R 1:1 或 72h+新人候选制+转载
// 须注明来源感谢原后缀。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(111, "hdupt_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'pt.hdupt.com' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'pt.hdupt.com'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel", "codec": "codec_sel", "audiocodec": "audiocodec_sel", "standard": "standard_sel", "team": "team_sel", "small_descr": "small_descr", "description": "descr", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Movies/电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "TV Series/电视剧", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "TV Shows/综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "Documentaries/纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "Animations/动画", "value": "405", "standard_keys": ["category.animation"]}, {"label": "Music Videos/音乐 MV", "value": "406", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "Sports/体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "HQ Audio/无损音乐", "value": "408", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "Misc/其他", "value": "411", "standard_keys": ["category.other"]}, {"label": "Games/游戏", "value": "410", "standard_keys": ["category.game"]}], "medium": [{"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "UHD Blu-ray", "value": "11", "standard_keys": ["medium.uhd_bluray"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "DVD", "value": "6", "standard_keys": ["medium.dvd", "medium.dvdr"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux"]}, {"label": "UHD Remux", "value": "15", "standard_keys": ["medium.uhd_remux"]}, {"label": "UHD Remux TV", "value": "16"}, {"label": "Remux TV", "value": "12"}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "Encode TV", "value": "14"}, {"label": "WEB-DL/WEBRip", "value": "10", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "WEB-DL/WEBRip TV", "value": "13"}, {"label": "MiniBD", "value": "4", "standard_keys": ["medium.minibd"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "Track", "value": "9", "standard_keys": ["medium.track"]}], "codec": [{"label": "H.264/AVC", "value": "1", "standard_keys": ["video.h264"]}, {"label": "H.265/HEVC", "value": "14", "standard_keys": ["video.h265"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "x264", "value": "16"}, {"label": "Xvid", "value": "3", "standard_keys": ["video.xvid"]}, {"label": "MPEG/MPEG-2", "value": "18", "standard_keys": ["video.mpeg2"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "DTS:X", "value": "16", "standard_keys": ["audio.dtsx"]}, {"label": "DTS-HDMA", "value": "1", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "TrueHD", "value": "3", "standard_keys": ["audio.truehd", "audio.truehd_atmos"]}, {"label": "LPCM", "value": "11", "standard_keys": ["audio.lpcm"]}, {"label": "DTS", "value": "4", "standard_keys": ["audio.dts"]}, {"label": "AC3/EAC3", "value": "2", "standard_keys": ["audio.ac3", "audio.dd", "audio.ddp"]}, {"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "FLAC", "value": "7", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "10", "standard_keys": ["audio.ape"]}, {"label": "WAV", "value": "17", "standard_keys": ["audio.wav"]}, {"label": "MPEG", "value": "18", "standard_keys": ["audio.mpeg"]}, {"label": "Other", "value": "13", "standard_keys": ["audio.other"]}], "standard": [{"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "4K/2160p", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "iPad", "value": "6"}], "team": [{"label": "HDU", "value": "2"}, {"label": "Other", "value": "5"}]}}`).Error
	})
}
