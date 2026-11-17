package db

import (
	"gorm.io/gorm"
)

// migration174 §59.541: 二师兄（piggo）值集刀——form_config 全量 + 开闸。
//
// 值集来源：catalog 端点实测快照（2026-11-16·piggo.md §59.541 权威表）。
// 结构（站方种实测定锚·Ronin/Fiksik/变形金刚三形态）：
//   - section_id 单值分区（type 域映射；animation 组合判据覆写走
//     executor piggoPublishInject——产地日本→7 / genre 家庭·儿童→5 / 默认 7）
//   - category_ids 体裁多选数组（piggoPublishInject 生成——不走值映射）
//   - taxonomy_ids 八维扁平数组（8 个虚拟字段各自值映射，PiggoAdapter
//     合并组装；空维兜 Other/SDR——站方 MI 纠正盖住手填·Ronin 八维全填实证）
//   - tag_ids 多选数组（tags 域值映射→TagArrayFields→adapter 聚合 JSON 串）
//
// 值映射键形（62 站惯例对齐）：
//   - source 域配 medium.* 键（海豹 migration145 先例——来源/媒介两维分片
//     不重叠：bluray 族来源=Blu-ray；untouched/remux/encode 媒介细分）
//   - processing 域配 hdr.* 键（§59.538 定案 SDR/HDR10/HDR10+/DV 一一配对）
//   - HDR 双通道：taxonomy 处理+tags（5/7/10）两边都写（Ronin 实证站方实务）
//
// tags 12 项接线（§59.541 用户逐项拍板；16 分集/1 官方/2 禁转等 8 项不配键；
// 20 备胎/22 丐帮挂账——组映射补全轮再议）。
//
// 幂等：publish_form_config 整体替换（enabled 开闸），is_target 同步开。
// publish_supported 门槛在 supported_sites.json（§59.318 语义·非 sites 表列）
// ——本 migration 随版双 JSON 同步置 true。
func init() {
	RegisterMigration(174, "piggo_form_config", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'piggo.me'`, `{
"anonymous": true,
"enabled": true,
"framework": "piggo",
"form_fields": {
	"type": "section_id",
	"source": "source_id",
	"medium": "medium_id",
	"codec": "codec_id",
	"audiocodec": "audio_id",
	"standard": "resolution_id",
	"processing": "processing_id",
	"team": "team_id",
	"region": "region_id",
	"tags": "tag_ids",
	"small_descr": "small_descr",
	"description": "descr",
	"technical_info": "media_info"
},
"value_mappings": {
	"type": [
		{"label": "电影", "value": "1", "standard_keys": ["category.movie"]},
		{"label": "电视剧", "value": "2", "standard_keys": ["category.tv_series"]},
		{"label": "综艺", "value": "3", "standard_keys": ["category.tv_shows"]},
		{"label": "纪录片", "value": "6", "standard_keys": ["category.documentary", "category.documentaries"]},
		{"label": "动漫", "value": "7", "standard_keys": ["category.animation"]}
	],
	"source": [
		{"label": "DVD", "value": "32", "standard_keys": ["medium.dvd", "medium.dvdr"]},
		{"label": "TV", "value": "33", "standard_keys": ["medium.tvrip", "medium.hdtv", "medium.uhdtv"]},
		{"label": "WEB-DL", "value": "34", "standard_keys": ["medium.webdl", "medium.webrip"]},
		{"label": "Blu-ray", "value": "35", "standard_keys": ["medium.bluray", "medium.uhd_bluray", "medium.remux", "medium.uhd_remux", "medium.encode", "medium.bdrip", "medium.minibd"]},
		{"label": "Other", "value": "36"}
	],
	"medium": [
		{"label": "Untouched", "value": "27", "standard_keys": ["medium.bluray", "medium.uhd_bluray"]},
		{"label": "Remux", "value": "28", "standard_keys": ["medium.remux", "medium.uhd_remux"]},
		{"label": "Encode", "value": "29", "standard_keys": ["medium.encode", "medium.webdl", "medium.webrip", "medium.hdtv", "medium.uhdtv", "medium.bdrip", "medium.minibd", "medium.dvdr", "medium.tvrip"]},
		{"label": "DIY", "value": "30"},
		{"label": "Other", "value": "31"}
	],
	"codec": [
		{"label": "H.264/X.264", "value": "24", "standard_keys": ["video.h264", "video.x264"]},
		{"label": "H.265/X.265", "value": "25", "standard_keys": ["video.h265", "video.x265"]},
		{"label": "H.266/VVC", "value": "63", "standard_keys": ["video.h266"]},
		{"label": "MPEG-2", "value": "64", "standard_keys": ["video.mpeg2"]},
		{"label": "VC-1", "value": "65", "standard_keys": ["video.vc1"]},
		{"label": "VP9", "value": "66", "standard_keys": ["video.vp9"]},
		{"label": "AV1", "value": "67", "standard_keys": ["video.av1"]},
		{"label": "Other", "value": "26"}
	],
	"audiocodec": [
		{"label": "FLAC", "value": "14", "standard_keys": ["audio.flac"]},
		{"label": "APE", "value": "15", "standard_keys": ["audio.ape"]},
		{"label": "DTS", "value": "16", "standard_keys": ["audio.dts"]},
		{"label": "MP3", "value": "17", "standard_keys": ["audio.mp3"]},
		{"label": "AAC", "value": "18", "standard_keys": ["audio.aac"]},
		{"label": "Other", "value": "19"},
		{"label": "AC-3", "value": "20", "standard_keys": ["audio.ac3", "audio.dd"]},
		{"label": "DTS-HD MA", "value": "21", "standard_keys": ["audio.dts_hd_ma", "audio.dts_hd_hr"]},
		{"label": "TrueHD", "value": "22", "standard_keys": ["audio.truehd"]},
		{"label": "LPCM", "value": "23", "standard_keys": ["audio.lpcm"]},
		{"label": "DDP", "value": "62", "standard_keys": ["audio.ddp"]}
	],
	"standard": [
		{"label": "other", "value": "10", "standard_keys": ["resolution.other", "resolution.sd"]},
		{"label": "4K", "value": "11", "standard_keys": ["resolution.r2160p", "resolution.r4320p"]},
		{"label": "1080p/i", "value": "12", "standard_keys": ["resolution.r1080p", "resolution.r1080i", "resolution.r1440p"]},
		{"label": "720p", "value": "13", "standard_keys": ["resolution.r720p"]}
	],
	"processing": [
		{"label": "SDR", "value": "6", "standard_keys": ["hdr.sdr"]},
		{"label": "HDR10", "value": "7", "standard_keys": ["hdr.hdr10", "hdr.hdr"]},
		{"label": "HDR10+", "value": "8", "standard_keys": ["hdr.hdr10plus"]},
		{"label": "DV（杜比视界)", "value": "9", "standard_keys": ["hdr.dv", "hdr.dv_hdr", "hdr.dv_hdr10plus"]}
	],
	"team": [
		{"label": "PigoWeb", "value": "1"},
		{"label": "PigoHD", "value": "2"},
		{"label": "PigoNF", "value": "3"},
		{"label": "PigoAD", "value": "4"},
		{"label": "Other", "value": "5", "standard_keys": ["team.other"]}
	],
	"region": [
		{"label": "内地", "value": "37", "standard_keys": ["region.cn"]},
		{"label": "中国香港", "value": "38", "standard_keys": ["region.hk"]},
		{"label": "中国台湾", "value": "39", "standard_keys": ["region.tw"]},
		{"label": "美国", "value": "40", "standard_keys": ["region.us"]},
		{"label": "英国", "value": "41", "standard_keys": ["region.gb"]},
		{"label": "韩国", "value": "42", "standard_keys": ["region.kr"]},
		{"label": "日本", "value": "43", "standard_keys": ["region.jp"]},
		{"label": "法国", "value": "44", "standard_keys": ["region.fr"]},
		{"label": "德国", "value": "45", "standard_keys": ["region.de"]},
		{"label": "意大利", "value": "46", "standard_keys": ["region.it"]},
		{"label": "西班牙", "value": "47", "standard_keys": ["region.es"]},
		{"label": "加拿大", "value": "48", "standard_keys": ["region.ca"]},
		{"label": "澳大利亚", "value": "49", "standard_keys": ["region.au"]},
		{"label": "印度", "value": "50", "standard_keys": ["region.in"]},
		{"label": "泰国", "value": "51", "standard_keys": ["region.th"]},
		{"label": "俄罗斯", "value": "52", "standard_keys": ["region.ru"]},
		{"label": "其他", "value": "53"}
	],
	"tags": [
		{"label": "国语", "value": "3", "standard_keys": ["tag.chinese_audio"]},
		{"label": "中字", "value": "4", "standard_keys": ["tag.chinese_subtitle"]},
		{"label": "HDR 10", "value": "5", "standard_keys": ["tag.hdr10"]},
		{"label": "3D", "value": "6", "standard_keys": ["tag.3d"]},
		{"label": "Dolby Vision", "value": "7", "standard_keys": ["tag.dolby_vision"]},
		{"label": "特效", "value": "8", "standard_keys": ["tag.special_effects_subs"]},
		{"label": "合集", "value": "9", "standard_keys": ["tag.collection"]},
		{"label": "Dolby Atmos", "value": "10", "standard_keys": ["tag.dolby_atmos"]},
		{"label": "完结", "value": "11", "standard_keys": ["tag.complete"]},
		{"label": "追更", "value": "12", "standard_keys": ["tag.ongoing"]},
		{"label": "中英双语", "value": "19", "standard_keys": ["tag.bilingual_subtitle_chs_eng"]}
	]
}
}`).Error
	})
}
