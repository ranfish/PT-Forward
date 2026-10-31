package db

import (
	"gorm.io/gorm"
)

// migration67 §59.381: 财神（cspt.top）发布表单配置分发。
// 依据 docs/32/cspt.md（1289 行——最长站点文档，含 upload.php HTML 表单值
// 权威汇总 §6.x + 油猴脚本值对照警告）+ sites.json form 资产。
// 槽位特化（文档 §7 实现要点）：
//   - 单模式站（type data-mode=4 全分类共享质量字段——无双区）
//   - medium→source_sel[4]（壹吧/百川先例——字段名 source 语义媒介）
//   - **HTML 值乱序/跳号**：媒介 7-16 非连续、codec AV1=6（4→6→5 乱序）、
//     分辨率 2K=10（4,5,6,10,7,8,9 乱序）——全部采用 upload.php HTML 表单值
//     （油猴脚本映射值完全忽略——文档 §7.1 适配器实现建议）
//   - codec 仅 6 项无 X264/X265 CLI 形态——§59.380 压制联动自然零触发
//   - audiocodec 16 项精确细分：DDP(12)/DD(13) 独立——audio.ddp→12、
//     [dd,ac3]→13 精确对齐（无需 DDP 并入 AC3 的妥协）
//   - standard 1080p/i 合一项→[r1080p,r1080i] 双键；480p→sd；Other 承接 other
//     （2K=10 独立选项→r1440p 精确）
//   - team 5 项（4 官组 CSPT/CSWEB/HSPT/HSWEB+Other）全 label-only——转发
//     外站组（FRDS 等）恒落 Other(5)
//   - tags 24 项：首发(2) label-only（禁勾首发铁律）+官种(3) label-only
//     （**站方双向校验**：官种必选/非官种不可选——错误级！外站组转发
//     恒非官种，配键恐自动勾触雷）+禁转/国语/中字/粤语/DIY/完结配键
//     （完结=tag.complete——财神完结资源优先 Dupe 规则正向）
//   - 隐含规则：简介含 IMDb 时 url 必填（发布链恒填 url ✓ 天然满足）
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(67, "cspt_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'cspt.top' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'cspt.top'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "source_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "电视剧", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "MV", "value": "406", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "HQ音乐", "value": "408", "standard_keys": ["category.music"]}, {"label": "其他", "value": "409", "standard_keys": ["category.other"]}], "medium": [{"label": "Blu-ray", "value": "7", "standard_keys": ["medium.bluray"]}, {"label": "UHD Blu-ray", "value": "8", "standard_keys": ["medium.uhd_bluray"]}, {"label": "Remux", "value": "9", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "Encode", "value": "10", "standard_keys": ["medium.encode"]}, {"label": "WEB-DL", "value": "11", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "HDTV", "value": "12", "standard_keys": ["medium.hdtv"]}, {"label": "DVD", "value": "13", "standard_keys": ["medium.dvdr", "medium.dvd"]}, {"label": "CD", "value": "14", "standard_keys": ["medium.cd"]}, {"label": "Track", "value": "15", "standard_keys": ["medium.track"]}, {"label": "Other", "value": "16", "standard_keys": ["medium.other"]}], "codec": [{"label": "H.264/AVC", "value": "1", "standard_keys": ["video.h264"]}, {"label": "H.265/HEVC", "value": "2", "standard_keys": ["video.h265"]}, {"label": "VC-1", "value": "3", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "AV1", "value": "6", "standard_keys": ["video.av1"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "ALAC", "value": "8", "standard_keys": ["audio.alac"]}, {"label": "AAC", "value": "9", "standard_keys": ["audio.aac"]}, {"label": "APE", "value": "10", "standard_keys": ["audio.ape"]}, {"label": "TrueHD Atmos", "value": "11", "standard_keys": ["audio.truehd_atmos"]}, {"label": "DDP/E-AC3", "value": "12", "standard_keys": ["audio.ddp", "audio.ddp_atmos"]}, {"label": "DD/AC3", "value": "13", "standard_keys": ["audio.dd", "audio.ac3"]}, {"label": "LPCM", "value": "14", "standard_keys": ["audio.lpcm"]}, {"label": "TrueHD", "value": "15", "standard_keys": ["audio.truehd"]}, {"label": "DTS:X", "value": "16", "standard_keys": ["audio.dtsx"]}, {"label": "DTS-HD MA", "value": "17", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "DTS", "value": "18", "standard_keys": ["audio.dts"]}, {"label": "M4A", "value": "19"}, {"label": "WAV", "value": "20", "standard_keys": ["audio.wav"]}, {"label": "MP3", "value": "21", "standard_keys": ["audio.mp3"]}, {"label": "FLAC", "value": "22", "standard_keys": ["audio.flac"]}, {"label": "Other", "value": "23", "standard_keys": ["audio.other"]}], "standard": [{"label": "480p/480i", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "720p/720i", "value": "5", "standard_keys": ["resolution.r720p"]}, {"label": "1080p/1080i", "value": "6", "standard_keys": ["resolution.r1080p", "resolution.r1080i"]}, {"label": "2K/1440p/1440i", "value": "10", "standard_keys": ["resolution.r1440p"]}, {"label": "4K/2160p/2160i", "value": "7", "standard_keys": ["resolution.r2160p"]}, {"label": "8K/4320p/4320i", "value": "8", "standard_keys": ["resolution.r4320p"]}, {"label": "Other", "value": "9", "standard_keys": ["resolution.other"]}], "team": [{"label": "CSPT", "value": "19"}, {"label": "CSWEB", "value": "18"}, {"label": "HSPT", "value": "8"}, {"label": "HSWEB", "value": "9"}, {"label": "Other", "value": "5"}], "tags": [{"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "首发", "value": "2"}, {"label": "官种", "value": "3"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR", "value": "7"}, {"label": "喜剧", "value": "8"}, {"label": "完结", "value": "9", "standard_keys": ["tag.complete"]}, {"label": "Remux", "value": "10"}, {"label": "杜比", "value": "11"}, {"label": "粤语", "value": "12", "standard_keys": ["tag.cantonese_audio"]}, {"label": "独家", "value": "13"}, {"label": "自压", "value": "14"}, {"label": "重制", "value": "15"}, {"label": "外挂字幕", "value": "16"}, {"label": "大包", "value": "17"}, {"label": "超分", "value": "18"}, {"label": "补帧", "value": "19"}, {"label": "特效", "value": "20"}, {"label": "分集", "value": "21"}, {"label": "零魔", "value": "22"}, {"label": "驻站", "value": "23"}, {"label": "儿童", "value": "24"}]}}`).Error
	})
}
