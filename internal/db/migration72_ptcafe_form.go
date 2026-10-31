package db

import (
	"gorm.io/gorm"
)

// migration72 §59.385: 咖啡（ptcafe.club）发布表单配置分发。
// 依据：站方 upload.php?mode=advanced 实时 HTML（44.9KB——§59.384 附
// 教训直接落实：label 源=站方 HTML 原文）+ sites.json form 资产
// （实时对照七槽零漂移——4 月资产可信）。
// 本站特殊背景：**docs/32 无文档**（流水线首例）——完整表单在
// upload.php?mode=advanced（默认页是外部发布器简版形态 10.9KB
// 仅 file/generator/name/nfo/small_descr/descr）。
// 槽位特化：
//   - **表单无 url/pt_gen 字段**（实时 HTML 实证 input 全集）——
//     form_fields 不配 imdb_url/pt_gen（简介贴 descr）
//   - source_sel[4] 产区槽（大陆/港台/欧美/日/韩/印/其他）**不配**：
//     发布链三层能力缺失（meta 无 Country 列/canonical 无 area 键族/
//     发布链无产区消费）+表单无 required（"请选择一项"占位非强制）
//     ——§59.208 纪律零案例不预实施；首颗真实发布若站方校验拒再议
//   - medium 13 项 UHD/BD 双线分离：UHD 原盘(1)/UHD DIY(2) label-only/
//     **UHD Remux(3) 独立**（大青虫/冬樱 [remux,uhd_remux] 并键一项——
//     咖啡分立两键两值更精确）；BD 原盘(4)/BD DIY(5) label-only；
//     WEB-DL(8) 承接 [webdl,webrip]（无独立 Webrip 项先例）
//   - codec H 标准名(1/2)+X 编码器名(3/4) 并存=百川/冬樱/五二同款
//     ——**§59.380 压制联动第四站**（lookupCLIVariant 大写 X265/X264
//     匹配自动选压制形态）；VP9(9)/DIVX(10) 独立项
//   - audiocodec 18 项 DTS 五级细分+**独立 DTS-X(5)**（冬樱 dtsx 给
//     HDMA:X 是因无独立项——咖啡 dtsx→5 更准）；HDMA:X 7.1(1)
//     label-only（组合粒度 meta 单键不可达——保守不猜）；AC3(7)
//     承接 [ac3,dd,ddp]（无 DDP 独立项——大青虫先例）；Atmos(8)
//     泛项承接 [atmos,truehd_atmos,ddp_atmos]
//   - standard 6 项**合并形态**：1080p/1080i 同项(3) 双键
//     [r1080p,r1080i]；720p/720i 同项(4)（r720i 无键归 r720p）；
//     4320P(1)/2160P(2)/360p SD(5)/Other(6)
//   - team 30 项（全库最多）：CafeTV(8)/CafeWEB(9)/PTCafe(25) 官组
//     +FRDS(14) 等外站组全 label-only
//   - tags 15 项：官方(1)/首发(2) label-only（铁律+财神官种案同款
//     防触雷）；完结(3)→tag.complete；禁转(5)/国语(7)/粤语(8)/
//     中字(9)/DIY(13) 配键；备胎/杜比视界/HDR/应求/高码高帧/月月
//     label-only
//   - type 406 演唱→category.concert（咖啡独立演唱项非 MV——无
//     music_video 项）；408 音乐→category.music
//   - 分叉族双键即防：404 纪录
//   - 有 uplver（anonymous: true）
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(72, "ptcafe_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'ptcafe.club' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'ptcafe.club'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "剧集", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "纪录", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "演唱", "value": "406", "standard_keys": ["category.concert"]}, {"label": "体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "音乐", "value": "408", "standard_keys": ["category.music"]}, {"label": "其他", "value": "409", "standard_keys": ["category.other"]}], "medium": [{"label": "UHD Blu-ray 原盘", "value": "1", "standard_keys": ["medium.uhd_bluray"]}, {"label": "UHD Blu-ray DIY", "value": "2"}, {"label": "UHD Remux", "value": "3", "standard_keys": ["medium.uhd_remux"]}, {"label": "Blu-ray 原盘", "value": "4", "standard_keys": ["medium.bluray"]}, {"label": "Blu-ray DIY", "value": "5"}, {"label": "Remux", "value": "6", "standard_keys": ["medium.remux"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "WEB-DL", "value": "8", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "TV", "value": "9", "standard_keys": ["medium.hdtv"]}, {"label": "DVD", "value": "10", "standard_keys": ["medium.dvdr", "medium.dvd"]}, {"label": "CD", "value": "11", "standard_keys": ["medium.cd"]}, {"label": "Track", "value": "12", "standard_keys": ["medium.track"]}, {"label": "Other", "value": "13", "standard_keys": ["medium.other"]}], "codec": [{"label": "H.265/HEVC", "value": "1", "standard_keys": ["video.h265"]}, {"label": "H.264/AVC", "value": "2", "standard_keys": ["video.h264"]}, {"label": "X265", "value": "3"}, {"label": "X264", "value": "4"}, {"label": "VC-1", "value": "5", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2", "value": "6", "standard_keys": ["video.mpeg2"]}, {"label": "MPEG-4", "value": "7", "standard_keys": ["video.mpeg4"]}, {"label": "XVID", "value": "8", "standard_keys": ["video.xvid"]}, {"label": "VP9", "value": "9", "standard_keys": ["video.vp9"]}, {"label": "DIVX", "value": "10", "standard_keys": ["video.divx"]}, {"label": "Other", "value": "11", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "DTS-HDMA:X 7.1", "value": "1"}, {"label": "DTS-HDMA", "value": "2", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "DTS-HDHR", "value": "3", "standard_keys": ["audio.dts_hd_hr"]}, {"label": "DTS-HD", "value": "4", "standard_keys": ["audio.dts_hd"]}, {"label": "DTS-X", "value": "5", "standard_keys": ["audio.dtsx"]}, {"label": "LPCM", "value": "6", "standard_keys": ["audio.lpcm"]}, {"label": "AC3", "value": "7", "standard_keys": ["audio.ac3", "audio.dd", "audio.ddp"]}, {"label": "Atmos", "value": "8", "standard_keys": ["audio.atmos", "audio.truehd_atmos", "audio.ddp_atmos"]}, {"label": "AAC", "value": "9", "standard_keys": ["audio.aac"]}, {"label": "TrueHD", "value": "10", "standard_keys": ["audio.truehd"]}, {"label": "DTS", "value": "11", "standard_keys": ["audio.dts"]}, {"label": "FLAC", "value": "12", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "13", "standard_keys": ["audio.ape"]}, {"label": "MP3", "value": "14", "standard_keys": ["audio.mp3"]}, {"label": "WAV", "value": "15", "standard_keys": ["audio.wav"]}, {"label": "OPUS", "value": "16", "standard_keys": ["audio.opus"]}, {"label": "OGG", "value": "17", "standard_keys": ["audio.ogg"]}, {"label": "Other", "value": "18", "standard_keys": ["audio.other"]}], "standard": [{"label": "4320P/8K/FUHD", "value": "1", "standard_keys": ["resolution.r4320p"]}, {"label": "2160P/4K/UHD", "value": "2", "standard_keys": ["resolution.r2160p"]}, {"label": "1080p/1080i/FHD", "value": "3", "standard_keys": ["resolution.r1080p", "resolution.r1080i"]}, {"label": "720p/720i/HD", "value": "4", "standard_keys": ["resolution.r720p"]}, {"label": "360p/360i/SD", "value": "5", "standard_keys": ["resolution.sd"]}, {"label": "Other", "value": "6", "standard_keys": ["resolution.other"]}], "team": [{"label": "ADE", "value": "1"}, {"label": "ADWeb", "value": "2"}, {"label": "Audies", "value": "3"}, {"label": "beAst", "value": "4"}, {"label": "BeiTai", "value": "5"}, {"label": "BeyondHD", "value": "6"}, {"label": "BtsTV", "value": "7"}, {"label": "CafeTV", "value": "8"}, {"label": "CafeWEB", "value": "9"}, {"label": "CHDBits", "value": "10"}, {"label": "CHDWEB", "value": "11"}, {"label": "CMCT", "value": "12"}, {"label": "DJWEB", "value": "13"}, {"label": "FRDS", "value": "14"}, {"label": "HDCTV", "value": "15"}, {"label": "HDH", "value": "16"}, {"label": "HDHome", "value": "17"}, {"label": "HDSky", "value": "18"}, {"label": "HDSWEB", "value": "19"}, {"label": "HHWEB", "value": "20"}, {"label": "MTeam", "value": "21"}, {"label": "MWeb", "value": "22"}, {"label": "OurBits", "value": "23"}, {"label": "OurTV", "value": "24"}, {"label": "PTCafe", "value": "25"}, {"label": "PTerWEB", "value": "26"}, {"label": "QHstudIo", "value": "27"}, {"label": "TTG", "value": "28"}, {"label": "WiKi", "value": "29"}, {"label": "Other", "value": "30"}], "tags": [{"label": "官方", "value": "1"}, {"label": "首发", "value": "2"}, {"label": "完结", "value": "3", "standard_keys": ["tag.complete"]}, {"label": "原创", "value": "4"}, {"label": "禁转", "value": "5", "standard_keys": ["tag.restricted"]}, {"label": "国语", "value": "7", "standard_keys": ["tag.chinese_audio"]}, {"label": "粤语", "value": "8", "standard_keys": ["tag.cantonese_audio"]}, {"label": "中字", "value": "9", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "备胎", "value": "10"}, {"label": "杜比视界", "value": "11"}, {"label": "HDR", "value": "12"}, {"label": "DIY", "value": "13", "standard_keys": ["tag.diy"]}, {"label": "应求", "value": "14"}, {"label": "高码高帧", "value": "15"}, {"label": "月月", "value": "16"}]}}`).Error
	})
}
