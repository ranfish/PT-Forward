package db

import (
	"gorm.io/gorm"
)

// migration110 §59.426: 时光（hdtime.org）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（46.8KB 直连）+ docs/32/hdtime.md
// 307 行读全（转发规则/命名规范/rules.php 完整站规）。
// 实时 vs 文档 vs 4 月资产全槽一致。
// 槽位映射：
//   - type 13：**424 Blu-Ray原盘独立分类**（站内实态仲裁——424 列表页
//     62 种全原盘标题实证：原盘资源发 424 而非 401+medium）——
//     hdtimeCategoryValue typeMappingOf 第七站（媒介维度路由：影视类 ∧
//     原盘族 bluray/uhd_bluray/hddvd→424；Remux/Encode 非原盘不触发；
//     424 label-only 特化独占）；401/402/403/404 双键/405/406 双键/
//     407/408 双键/409/410/414 标准键直配；411 文档 label-only
//     （语义模糊——materials 键不猜）。
//   - **无 standard/processing/source 槽**（分辨率仅标题传递——文档 1.8
//     实证）。
//   - medium 10：Blu-ray=[bluray,uhd_bluray] 归并（无 UHD 独立项——浦园
//     式）+Remux=[remux,uhd_remux] 归并+MiniBD 精确键+HD DVD=[hddvd]+
//     WEB-DL 承接 webrip+DVDR=[dvdr,dvd]。
//   - codec 11 H/X 合一（AVC/H.264/x264=[h264]+HEVC/H.265/x265=[h265]
//     ——压制联动零触发站方合并语义）+**x264-10bit=[x264_10bit]
//     extended 独有键首配**+AVS3=[avs3]+VVC/H.266/x266=[h266]+VP8/VP9=
//     [vp9]+AV1+AVS3 三新代编码全配键。
//   - audiocodec 15：TrueHD Atmos 双键+DTS-HD=[dts_hd_ma,dts_hd_hr]
//     优堡先例+DD/AC3=[ac3,dd]+DDP/EAC3=[ddp]+**LPCM/PCM=[lpcm,pcm]
//     合并项双键**+**Audio Vivid/AV3A=[av3a]**+OPUS 全配键。
//   - team 13 label-only：HDTime 系官组（HDTime/PADTime/HDT/VTime）+
//     外站组（CHD/beAst/WiKi/CMCT/QHstudIo/AilMWeb/HHWEB——直配先例）
//     +个人原创+Other(5) 兜底。
//   - tags 8 配键 7：禁转=[restricted]+国语+中字+DIY+HDR 四键泛家族+
//     杜比视界=[dolby_vision]；首发铁律+官方(3) 雷区 label-only。
//   - 字段：url+pt_gen（四来源）+technical_info+uplver+anonymous:true
//     （§59.422 附二默认）；nfo 不配（13城定案——实时有 nfo file 控件）。
// 站规人工把关：转发规则（不得去掉原创组标识/严禁转独占禁转限转）+
// <100MB 禁/upscale 禁/RMVB 禁/硬盘版高压版游戏禁/RAR 禁/游戏上传员
// 等级/dupe（断种 45 天·18 个月豁免）/候选制（导航栏有入口）。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(110, "hdtime_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'hdtime.org' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'hdtime.org'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "其他", "value": "409", "standard_keys": ["category.other"]}, {"label": "文档", "value": "411"}, {"label": "游戏", "value": "410", "standard_keys": ["category.game"]}, {"label": "MV", "value": "406", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "音乐", "value": "408", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "软件", "value": "414", "standard_keys": ["category.software"]}, {"label": "动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "剧集", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "Blu-Ray原盘", "value": "424"}], "medium": [{"label": "Track", "value": "9", "standard_keys": ["medium.track"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "DVDR", "value": "6", "standard_keys": ["medium.dvdr", "medium.dvd"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "MiniBD", "value": "4", "standard_keys": ["medium.minibd"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "HD DVD", "value": "2", "standard_keys": ["medium.hddvd"]}, {"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray", "medium.uhd_bluray"]}, {"label": "WEB-DL", "value": "10", "standard_keys": ["medium.webdl", "medium.webrip"]}], "codec": [{"label": "AVC/H.264/x264", "value": "1", "standard_keys": ["video.h264"]}, {"label": "HEVC/H.265/x265", "value": "12", "standard_keys": ["video.h265"]}, {"label": "x264-10bit", "value": "11", "standard_keys": ["video.x264_10bit"]}, {"label": "AV1", "value": "15", "standard_keys": ["video.av1"]}, {"label": "VP8/VP9", "value": "14", "standard_keys": ["video.vp9"]}, {"label": "AVS3", "value": "17", "standard_keys": ["video.avs3"]}, {"label": "xvid", "value": "3", "standard_keys": ["video.xvid"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "VVC/H.266/x266", "value": "16", "standard_keys": ["video.h266"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "TrueHD Atmos", "value": "8", "standard_keys": ["audio.atmos", "audio.truehd_atmos"]}, {"label": "TrueHD", "value": "9", "standard_keys": ["audio.truehd"]}, {"label": "DTS-HD", "value": "10", "standard_keys": ["audio.dts_hd_ma", "audio.dts_hd_hr"]}, {"label": "DTS", "value": "3", "standard_keys": ["audio.dts"]}, {"label": "DD/AC3", "value": "11", "standard_keys": ["audio.ac3", "audio.dd"]}, {"label": "DDP/EAC3", "value": "12", "standard_keys": ["audio.ddp"]}, {"label": "LPCM/PCM", "value": "13", "standard_keys": ["audio.lpcm", "audio.pcm"]}, {"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "Audio Vivid/AV3A", "value": "14", "standard_keys": ["audio.av3a"]}, {"label": "OPUS", "value": "15", "standard_keys": ["audio.opus"]}, {"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "MP3", "value": "4", "standard_keys": ["audio.mp3"]}, {"label": "OGG", "value": "5", "standard_keys": ["audio.ogg"]}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}], "team": [{"label": "个人原创", "value": "9"}, {"label": "Other", "value": "5"}, {"label": "CHD", "value": "2"}, {"label": "beAst", "value": "3"}, {"label": "WiKi", "value": "4"}, {"label": "CMCT", "value": "8"}, {"label": "PADTime", "value": "7"}, {"label": "VTime", "value": "15"}, {"label": "HDT", "value": "12"}, {"label": "HDTime", "value": "6"}, {"label": "QHstudIo", "value": "16"}, {"label": "AilMWeb", "value": "17"}, {"label": "HHWEB", "value": "18"}], "tags": [{"label": "杜比视界", "value": "8", "standard_keys": ["tag.dolby_vision"]}, {"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "首发", "value": "2"}, {"label": "官方", "value": "3"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}]}}`).Error
	})
}
