package db

import (
	"gorm.io/gorm"
)

// migration129 §59.446: HDVideo（hdvideo.top）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（112.5KB 代理）+ docs/32/hdvideo.md
// 669 行读全（rules.php+标题命名规范完整采集）。
// **实时 vs 资产差异**：**team HDVTV(6) 新增**（资产 4 项 groups 有
// DHVTV 疑笔误——实时为准）+**tags AI仿真(32) 新增**（资产 27→
// 28——label-only 防松查乱挂财神 Ai 先例）+**region_sel 17 项产区
// 槽+style_sel 18 项题材槽资产未捕获**（实时发现新槽）。
// 槽位映射：
//   - type 8：408 音乐MV=[mv,music_video] 双键+406 音轨=[music]
//     （音轨≈音频类单键——无 lossless_music 细分保守单键）+404
//     双键；无短剧/游戏/软件类目。
//   - medium 9（值 10-20 乱序）：UHD Blu-ray(10)=[uhd_bluray]+
//     FHD Blu-ray(11)=[bluray] 原盘分立（FHD 命名特有形态）+
//     Remux(12)=[remux,uhd_remux] 双键归并+Encode(13)+WEB-DL(14)
//     =[webdl,webrip] 双键+**UHDTV/HDTV(16)=[hdtv,uhdtv] 双键
//     （UHD/HDTV 合并项——我堡 UHDTV 键复利）**+DVD(18)=[dvd,
//     dvdr] 双键宽语义+CD(20)+Other(19)。
//   - codec 8 合并命名零联动（AVC/H.264/x264 一项三命名——站方
//     合并项语义第七站反证）+VP8/9=[vp9]+AV1/S=[av1]+ProRes ext
//     （星湾首配键复利）+VC-1/MPEG-2/Other。
//   - **audiocodec 20 全配键**：**DDP Atmos/EAC3(23)=[ddp_atmos,
//     atmos] 双键（红豆饭先例）+TrueHD_Atmos(14)=[truehd_atmos]
//     单键（站方 label 下划线形态精确）+DDP/EAC3(10)=[ddp,eac3]
//     双键（eac3 ext 首配键）+DTS_HD|MA(3)=[dts_hd_ma]+DTS_HD|HR
//     (15)=[dts_hd_hr] 分立各单键+LPCM/PCM(8)=[lpcm,pcm] 合并项
//     双键（咖啡先例）+DD/AC3(9)=[dd,ac3]+DTS_X(16)=[dtsx]+DTS+
//     MP2/3(18)=[mp3,mp2] 双键+Opus/WAV/AAC/APE/OGG/FLAC/Other**；
//     TAA(19)/MPEG(22) label-only（TAA 疑 TTA 误写保守不猜+
//     mpeg_audio 无键）。
//   - standard 6：4320p/8K(5)=[r4320p]+2160p/4K(6)=[r2160p] 斜杠
//     合并形态+1080p/i 分立+720p+Other(11)=[resolution.other]。
//   - team 6：**HDV/HDVWEB/HDVMV/HDVTV 官组系 label-only**（HDVTV
//     实时新增——资产 groups DHVTV 疑笔误实时为准）+Other(4)=
//     [team.other] 兜底。
//   - **tags 28 五绝不配**（禁转/限转/官方/原创/首发——财神雷区
//     全集）+**HDR 五族细分独立第六站**（Dolby Vision/HDR10/
//     HDR10+/HDR Vivid/HLG 五键各独立——熊猫/优堡同款）+连载=
//     [ongoing]+完结=[complete]+国语/粤语/中字/特效/DIY 九键配；
//     源码(8)/金种(16)/应求(21)/零魔(15)/**AI仿真(32) label-only
//     （实时新增财神 Ai 先例）**/微短剧(31)/漫剧(30)（tag dict
//     无 short_drama 键——category 域键不混用）/MV/LIVE现场/
//     演唱会/音乐专辑音乐标签 label-only。
//   - **region_sel 17=产区槽不配**（咖啡定案——产区数据无静态
//     投递能力预览人工）+**style_sel 18=题材槽不配**（§59.400
//     定案：题材不进 tag dict 走站方特化通道非配置映射）。
//   - 字段：type/medium_sel[4]/codec_sel[4]/audiocodec_sel[4]/
//     standard_sel[4]/team_sel[4]/tags[4][]/small_descr/descr+
//     url（IMDb）+**douban_url=豆瓣条目链接（dburl 域——天空
//     FieldDomainDoubanURL 先例复用）**+pt_gen+uplver；nfo 不配
//     （13城先例）；**无 technical_info**（MI 塞简介）；search_
//     mode（搜索 UI 字段发布链不处理）。
// 站规人工把关：上传规则（§6.4）+标题命名规范（§6.5 rules.php
//   原文）+账号保留规则+金种/源码官方标记语义。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(129, "hdvideo_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'hdvideo.top' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'hdvideo.top'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "dburl": "douban_url", "pt_gen": "pt_gen", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "电视剧", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "音乐MV", "value": "408", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "音轨", "value": "406", "standard_keys": ["category.music"]}], "medium": [{"label": "WEB-DL", "value": "14", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "UHDTV/HDTV", "value": "16", "standard_keys": ["medium.hdtv", "medium.uhdtv"]}, {"label": "UHD Blu-ray", "value": "10", "standard_keys": ["medium.uhd_bluray"]}, {"label": "FHD Blu-ray", "value": "11", "standard_keys": ["medium.bluray"]}, {"label": "Encode", "value": "13", "standard_keys": ["medium.encode"]}, {"label": "Remux", "value": "12", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "DVD", "value": "18", "standard_keys": ["medium.dvd", "medium.dvdr"]}, {"label": "CD", "value": "20", "standard_keys": ["medium.cd"]}, {"label": "Other", "value": "19", "standard_keys": ["medium.other"]}], "codec": [{"label": "ProRes", "value": "14", "standard_keys": ["video.prores"]}, {"label": "AV1/S", "value": "12", "standard_keys": ["video.av1"]}, {"label": "VP8/9", "value": "10", "standard_keys": ["video.vp9"]}, {"label": "VC-1", "value": "8", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2", "value": "9", "standard_keys": ["video.mpeg2"]}, {"label": "AVC/H.264/x264", "value": "7", "standard_keys": ["video.h264"]}, {"label": "HEVC/H.265/x265", "value": "6", "standard_keys": ["video.h265"]}, {"label": "Other", "value": "11", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "DDP Atmos/EAC3", "value": "23", "standard_keys": ["audio.ddp_atmos", "audio.atmos"]}, {"label": "TrueHD_Atmos", "value": "14", "standard_keys": ["audio.truehd_atmos"]}, {"label": "LPCM/PCM", "value": "8", "standard_keys": ["audio.lpcm", "audio.pcm"]}, {"label": "DDP/EAC3", "value": "10", "standard_keys": ["audio.ddp", "audio.eac3"]}, {"label": "DTS_HD|MA", "value": "3", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "DTS_HD|HR", "value": "15", "standard_keys": ["audio.dts_hd_hr"]}, {"label": "DD/AC3", "value": "9", "standard_keys": ["audio.dd", "audio.ac3"]}, {"label": "TrueHD", "value": "11", "standard_keys": ["audio.truehd"]}, {"label": "DTS_X", "value": "16", "standard_keys": ["audio.dtsx"]}, {"label": "MP2/3", "value": "18", "standard_keys": ["audio.mp3", "audio.mp2"]}, {"label": "MPEG", "value": "22"}, {"label": "Opus", "value": "5", "standard_keys": ["audio.opus"]}, {"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "OGG", "value": "20", "standard_keys": ["audio.ogg"]}, {"label": "WAV", "value": "12", "standard_keys": ["audio.wav"]}, {"label": "DTS", "value": "17", "standard_keys": ["audio.dts"]}, {"label": "TAA", "value": "19"}, {"label": "AAC", "value": "21", "standard_keys": ["audio.aac"]}, {"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}], "standard": [{"label": "4320p/8K", "value": "5", "standard_keys": ["resolution.r4320p"]}, {"label": "2160p/4K", "value": "6", "standard_keys": ["resolution.r2160p"]}, {"label": "1080p", "value": "8", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "9", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "10", "standard_keys": ["resolution.r720p"]}, {"label": "Other", "value": "11", "standard_keys": ["resolution.other"]}], "team": [{"label": "HDVMV", "value": "2"}, {"label": "HDVWEB", "value": "1"}, {"label": "HDVTV", "value": "6"}, {"label": "HDV", "value": "5"}, {"label": "Other", "value": "4", "standard_keys": ["team.other"]}], "tags": [{"label": "禁转", "value": "1"}, {"label": "限转", "value": "9"}, {"label": "官方", "value": "3"}, {"label": "连载", "value": "28", "standard_keys": ["tag.ongoing"]}, {"label": "完结", "value": "17", "standard_keys": ["tag.complete"]}, {"label": "原创", "value": "18"}, {"label": "首发", "value": "29"}, {"label": "微短剧", "value": "31"}, {"label": "漫剧", "value": "30"}, {"label": "AI仿真", "value": "32"}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "粤语", "value": "10", "standard_keys": ["tag.cantonese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "特效", "value": "19", "standard_keys": ["tag.special_effects_subs"]}, {"label": "DIY", "value": "22", "standard_keys": ["tag.diy"]}, {"label": "Dolby Vision", "value": "12", "standard_keys": ["tag.dolby_vision"]}, {"label": "HDR10", "value": "11", "standard_keys": ["tag.hdr10"]}, {"label": "HDR10+", "value": "20", "standard_keys": ["tag.hdr10_plus"]}, {"label": "HDR Vivid", "value": "27", "standard_keys": ["tag.hdr_vivid"]}, {"label": "HLG", "value": "13", "standard_keys": ["tag.hlg"]}, {"label": "源码", "value": "8"}, {"label": "金种", "value": "16"}, {"label": "应求", "value": "21"}, {"label": "零魔", "value": "15"}, {"label": "MV", "value": "23"}, {"label": "LIVE现场", "value": "24"}, {"label": "演唱会", "value": "26"}, {"label": "音乐专辑", "value": "25"}]}}`).Error
	})
}
