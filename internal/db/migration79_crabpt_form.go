package db

import (
	"gorm.io/gorm"
)

// migration79 §59.394: 蟹黄堡（crabpt.vip）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（67.4KB 直连）+ docs/32/crabpt.md
// 667 行读全。
// **双区互斥站**（browsecat mode=4 影视/specialcat mode=6 特别区——
// 壹吧/聆音同款；[6] 槽 5 个由 §59.388 模式互斥过滤自动跳过 ✓）。
// 实时 HTML 多处漂移（§59.384 附双源教训）：type label 带空格
// "电影 / Movies"、processing 全角括号、team 实时 36 项（sites.json
// 32——新增 MTeam/HDVWEB/HDHome/QHstudIo/beAst/tlf/FROG 系 9 项+
// 站方四组重复值 FRDS 7/28、ZmWeb 16/31、WiKi 4/26、QHstudIo
// 34/24——首值配语义重复 label-only）、tags 37 项（新增未完结 59/
// ASMR 50）。
// 槽位特化：
//   - medium→source_sel[4]（财神/青蛙后第三站）；MVC(9)/ProRes(10)/
//     Xvid(11) 媒介槽罕见项 label-only；Remux 归并 uhd_remux+WEB-DL
//     承接 webrip
//   - codec **H/X 合并形态**（"AVC/H.264/x264" 一项含三词——压制
//     联动 label 精确匹配 miss→标准键命中同项=零区分站方语义 ✓）；
//     H.266/VVC(4) 独有项双键 [h266,vvc]；MPEG(15) 泛指承接
//     [mpeg2,mpeg4]
//   - standard 5 项：1080p/i 合并双键+8K；**无 SD 项**（sd miss
//     落空——库非先例）
//   - audiocodec 21 项：DTS-HD 双值 15 首值配键 22 label-only
//     （库非 OGG 双值同款）+ALAC/DSD/Opus/Atmos 配键
//   - processing_sel[4] 产区槽**不配**（发布链无产区组装路径+
//     文档实证非必填——咖啡定案；§59.391 葡萄产区特化是 type 槽
//     产区分类，本站产区是独立槽零案例不预实施）
//   - type 413 短剧=short_drama（财神键第三站）+404/406 双键
//   - team 36 项全 label-only（FRDS(7) 首值在表——DryRun 直中可期）
//   - tags 37 项（全库最多）：禁转/原盘DIY/国语/粤语/中字/完结配键；
//     无首发项✓；自购/分集/特效字幕/HDR/DV/Dolby Atmos/合集大包/
//     伦理/驻站/Audiences/AGSV/短剧/影视类型 17 项/未完结/ASMR
//     label-only
//   - 字段：url（IMDb 真字段——控件排除清单误滤复核教训）+
//     technical_info+uplver；**无 pt_gen/dburl**（实时实证）
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(79, "crabpt_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'crabpt.vip' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'crabpt.vip'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "source_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影 / Movies", "value": "401", "standard_keys": ["category.movie"]}, {"label": "电视剧 / TVSeries", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "短剧 / Playlet", "value": "413", "standard_keys": ["category.short_drama"]}, {"label": "音乐 / Music", "value": "408", "standard_keys": ["category.music"]}, {"label": "动漫 / Anime", "value": "405", "standard_keys": ["category.animation"]}, {"label": "MV", "value": "406", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "综艺 / TV Shows", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "体育竞技 / Sports", "value": "407", "standard_keys": ["category.sports"]}, {"label": "其他 / Other", "value": "409", "standard_keys": ["category.other"]}, {"label": "纪录片 / Documentary", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}], "medium": [{"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "HDTV", "value": "7", "standard_keys": ["medium.hdtv"]}, {"label": "WEB-DL", "value": "6", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "Encode", "value": "5", "standard_keys": ["medium.encode"]}, {"label": "Remux", "value": "4", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "UHD Blu-ray", "value": "3", "standard_keys": ["medium.uhd_bluray"]}, {"label": "BluRay", "value": "2", "standard_keys": ["medium.bluray"]}, {"label": "Other", "value": "1", "standard_keys": ["medium.other"]}, {"label": "MVC", "value": "9"}, {"label": "ProRes", "value": "10"}, {"label": "Xvid", "value": "11"}], "codec": [{"label": "AV1", "value": "6", "standard_keys": ["video.av1"]}, {"label": "VP9", "value": "5", "standard_keys": ["video.vp9"]}, {"label": "H.266/VVC", "value": "4", "standard_keys": ["video.h266", "video.vvc"]}, {"label": "HEVC/H.265/x265", "value": "3", "standard_keys": ["video.h265"]}, {"label": "AVC/H.264/x264", "value": "2", "standard_keys": ["video.h264"]}, {"label": "Other", "value": "1", "standard_keys": ["video.other"]}, {"label": "VC-1", "value": "14", "standard_keys": ["video.vc1"]}, {"label": "MPEG", "value": "15", "standard_keys": ["video.mpeg2", "video.mpeg4"]}], "audiocodec": [{"label": "ALAC", "value": "16", "standard_keys": ["audio.alac"]}, {"label": "APE", "value": "12", "standard_keys": ["audio.ape"]}, {"label": "WAV", "value": "11", "standard_keys": ["audio.wav"]}, {"label": "FLAC", "value": "10", "standard_keys": ["audio.flac"]}, {"label": "MPEG", "value": "9", "standard_keys": ["audio.mpeg"]}, {"label": "DTS:X", "value": "8", "standard_keys": ["audio.dtsx"]}, {"label": "LPCM", "value": "7", "standard_keys": ["audio.lpcm"]}, {"label": "TrueHD", "value": "6", "standard_keys": ["audio.truehd"]}, {"label": "DTS", "value": "5", "standard_keys": ["audio.dts"]}, {"label": "DDP/E-AC3", "value": "4", "standard_keys": ["audio.ddp"]}, {"label": "DD/AC3", "value": "3", "standard_keys": ["audio.ac3", "audio.dd"]}, {"label": "AAC", "value": "2", "standard_keys": ["audio.aac"]}, {"label": "Other", "value": "1", "standard_keys": ["audio.other"]}, {"label": "DTS-HD", "value": "15", "standard_keys": ["audio.dts_hd"]}, {"label": "DTS-HD MA", "value": "17", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "OGG", "value": "21", "standard_keys": ["audio.ogg"]}, {"label": "DTS-HD", "value": "22"}, {"label": "DSD", "value": "23", "standard_keys": ["audio.dsd"]}, {"label": "Opus", "value": "24", "standard_keys": ["audio.opus"]}, {"label": "Atmos", "value": "26", "standard_keys": ["audio.atmos", "audio.truehd_atmos", "audio.ddp_atmos"]}], "standard": [{"label": "8K", "value": "5", "standard_keys": ["resolution.r4320p"]}, {"label": "4K/2160p", "value": "4", "standard_keys": ["resolution.r2160p"]}, {"label": "1080p/1080i", "value": "3", "standard_keys": ["resolution.r1080p", "resolution.r1080i"]}, {"label": "720p", "value": "2", "standard_keys": ["resolution.r720p"]}, {"label": "Other", "value": "1", "standard_keys": ["resolution.other"]}], "team": [{"label": "AGSVWEB", "value": "15"}, {"label": "DYZ-WEB", "value": "11"}, {"label": "DYZ-Movie", "value": "12"}, {"label": "DYZ-TV", "value": "13"}, {"label": "UBits", "value": "9"}, {"label": "HHWEB", "value": "8"}, {"label": "FRDS", "value": "7"}, {"label": "XHB", "value": "6"}, {"label": "OurBits", "value": "5"}, {"label": "WiKi", "value": "4"}, {"label": "HDS", "value": "3"}, {"label": "CHD", "value": "2"}, {"label": "Other", "value": "1"}, {"label": "Audiences", "value": "10"}, {"label": "ZmWeb", "value": "16"}, {"label": "MTeam", "value": "37"}, {"label": "HDVWEB", "value": "36"}, {"label": "HDHome", "value": "35"}, {"label": "QHstudIo", "value": "34"}, {"label": "beAst", "value": "33"}, {"label": "tlf", "value": "32"}, {"label": "ZmWeb", "value": "31"}, {"label": "FROGWeb", "value": "30"}, {"label": "FROG", "value": "29"}, {"label": "FRDS", "value": "28"}, {"label": "CMCT", "value": "27"}, {"label": "WiKi", "value": "26"}, {"label": "ADWeb", "value": "25"}, {"label": "QHstudIo", "value": "24"}, {"label": "PiGoNF", "value": "23"}, {"label": "PigoWeb", "value": "22"}, {"label": "PigoHD", "value": "21"}, {"label": "HDFans", "value": "20"}, {"label": "UBWEB", "value": "19"}, {"label": "FFans", "value": "18"}, {"label": "Pter", "value": "17"}], "tags": [{"label": "合集大包", "value": "19"}, {"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "自购", "value": "2"}, {"label": "原盘DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "粤语", "value": "6", "standard_keys": ["tag.cantonese_audio"]}, {"label": "中字", "value": "7", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "分集", "value": "9"}, {"label": "完结", "value": "8", "standard_keys": ["tag.complete"]}, {"label": "HDR", "value": "11"}, {"label": "特效字幕", "value": "10"}, {"label": "Dolby Vision", "value": "12"}, {"label": "Dolby Atmos", "value": "13"}, {"label": "未完结", "value": "59"}, {"label": "ASMR", "value": "50"}, {"label": "武侠", "value": "48"}, {"label": "灾难", "value": "47"}, {"label": "冒险", "value": "46"}, {"label": "奇幻", "value": "45"}, {"label": "犯罪", "value": "44"}, {"label": "战争", "value": "43"}, {"label": "历史", "value": "42"}, {"label": "纪录片", "value": "41"}, {"label": "恐怖", "value": "40"}, {"label": "惊悚", "value": "39"}, {"label": "悬疑", "value": "38"}, {"label": "动画", "value": "37"}, {"label": "科幻", "value": "36"}, {"label": "爱情", "value": "35"}, {"label": "动作", "value": "34"}, {"label": "喜剧", "value": "33"}, {"label": "剧情", "value": "32"}, {"label": "短剧", "value": "25"}, {"label": "AGSV", "value": "24"}, {"label": "Audiences", "value": "23"}, {"label": "驻站", "value": "22"}, {"label": "伦理", "value": "20"}]}}`).Error
	})
}
