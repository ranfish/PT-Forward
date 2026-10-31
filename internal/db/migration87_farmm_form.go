package db

import (
	"gorm.io/gorm"
)

// migration87 §59.401: 农场（pt.0ff.cc）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（39.2KB 直连抓取）+ docs/32/farmm.md
// 750 行读全（双区域/教育特色/Cloudflare 防护）。
// 实时 HTML 与 sites.json 4 月资产全槽零漂移（第九个零漂移站；
// audio label "TrueHD Atmos\t" 站方原文带 tab 尾——strip 后一致）。
// **双区域互斥站**（种子区 mode=4 +特别区 mode=5 儿童内容——
// [5] 五槽+tags[5][] 8 项由 §59.388 模式过滤自动跳过 ✓ 四站复利）。
// 槽位特化：
//   - type 14 项：406 MV 双键+407 HQ Audio=lossless_music+427
//     Education=education+412 book=ebook+410 Resources label-only
//     （网络资源无 canonical 键）+409 待定 label-only；403 纪录双键
//   - medium 16 项**全 canonical 配键**（第三个零 label-only 媒介站
//     ——大青虫/末日后）：UHDTV/IPTV/SACD/3D Blu-ray 全配
//   - codec 14 项**H/X 分立+HEVC 独立**：X264(4)/X265(3) label-only
//     （§59.380 压制联动通道——label 精确匹配 CLI 词触发，百川
//     先例；压制种→X265/原盘→H.265 站方语义）；5 HEVC label-only
//     （与 1 H.265/HEVC 语义重叠站方未区分——保守不猜）；MVC/
//     ProRes/VP8/9 配键
//   - audiocodec 19 项：DTS 六级细分全库最全之一（DTS/HD MA/
//     HD HR/DTS-X/HDMA:X 7.1 组合 label-only 咖啡先例）+AC3|DD
//     承接 [ac3,dd,ddp] 三键（无 DDP 独立项大青虫同款）+DSD/PCM/
//     OPUS 配键+TrueHD Atmos 双键
//   - standard 8 项**p/i 分立双值**（1080P(3) 与 1080I(5) 独立——
//     非合并形态）+1440P+8 项全配键
//   - team 2 项全 label-only（FFWEB 官组+Other——全库最少之一）
//   - tags 10 项：HDR→[hdr10,hdr10_plus,hdr_vivid,hlg] 四键泛
//     HDR 家族（财神 §59.400 同款；农场无杜比标签 DV 落空零半径）
//     +3D→tag.3d+连载→ongoing 配键+完结/禁转/国语/中字/DIY 配键；
//     首发 label-only 铁律+短剧 label-only
//   - source_sel=产区槽不配（咖啡先例——同名异义第二例记录：
//     source_sel 在此站=地区非媒介）+processing_sel=教育年级槽
//     不配（影视发布无关）
//   - 字段：url（IMDb data-pt-gen）+pt_gen+technical_info+nfo+
//     uplver——字段最全站之一；无 doubanid/imdbid（编号形态
//     不涉及）
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(87, "farmm_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'pt.0ff.cc' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'pt.0ff.cc'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Movies|电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "TV Series|电视剧", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "Documentaries|纪录片", "value": "403", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "TV Shows|综艺", "value": "404", "standard_keys": ["category.tv_shows"]}, {"label": "Animations|动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "Music Videos|音乐视频", "value": "406", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "HQ Audio|无损音乐", "value": "407", "standard_keys": ["category.lossless_music"]}, {"label": "Sports|体育", "value": "408", "standard_keys": ["category.sports"]}, {"label": "Education|学习教育", "value": "427", "standard_keys": ["category.education"]}, {"label": "book|书籍", "value": "412", "standard_keys": ["category.ebook"]}, {"label": "Game|游戏", "value": "411", "standard_keys": ["category.game"]}, {"label": "Resources|网络资源", "value": "410"}, {"label": "待定", "value": "409"}, {"label": "Other|其他", "value": "428", "standard_keys": ["category.other"]}], "medium": [{"label": "UHD Blu-ray", "value": "1", "standard_keys": ["medium.uhd_bluray"]}, {"label": "Blu-ray", "value": "2", "standard_keys": ["medium.bluray"]}, {"label": "UHDTV", "value": "3", "standard_keys": ["medium.uhdtv"]}, {"label": "IPTV", "value": "4", "standard_keys": ["medium.iptv"]}, {"label": "Remux", "value": "5", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "Encode", "value": "6", "standard_keys": ["medium.encode"]}, {"label": "HDTV", "value": "7", "standard_keys": ["medium.hdtv"]}, {"label": "DVDR", "value": "8", "standard_keys": ["medium.dvdr", "medium.dvd"]}, {"label": "CD", "value": "9", "standard_keys": ["medium.cd"]}, {"label": "MiniBD", "value": "10", "standard_keys": ["medium.minibd"]}, {"label": "Track", "value": "11", "standard_keys": ["medium.track"]}, {"label": "WEB-DL", "value": "12", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "SACD", "value": "13", "standard_keys": ["medium.sacd"]}, {"label": "HD DVD", "value": "14", "standard_keys": ["medium.hddvd"]}, {"label": "3D Blu-ray", "value": "15", "standard_keys": ["medium.3d_bluray"]}, {"label": "其它 (Other)", "value": "21", "standard_keys": ["medium.other"]}], "codec": [{"label": "H.265/HEVC", "value": "1", "standard_keys": ["video.h265"]}, {"label": "H.264/AVC", "value": "2", "standard_keys": ["video.h264"]}, {"label": "X265", "value": "3"}, {"label": "X264", "value": "4"}, {"label": "HEVC", "value": "5"}, {"label": "VC-1", "value": "6", "standard_keys": ["video.vc1"]}, {"label": "MPEG-4", "value": "7", "standard_keys": ["video.mpeg4"]}, {"label": "MPEG-2", "value": "8", "standard_keys": ["video.mpeg2"]}, {"label": "Xvid", "value": "9", "standard_keys": ["video.xvid"]}, {"label": "MVC", "value": "10", "standard_keys": ["video.mvc"]}, {"label": "ProRes", "value": "11", "standard_keys": ["video.prores"]}, {"label": "VP8/9", "value": "12", "standard_keys": ["video.vp9"]}, {"label": "AV1", "value": "13", "standard_keys": ["video.av1"]}, {"label": "其它 (Other)", "value": "14", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "TrueHD Atmos", "value": "1", "standard_keys": ["audio.atmos", "audio.truehd_atmos"]}, {"label": "DTS-HD|MA", "value": "2", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "DTS-HD|HR", "value": "3", "standard_keys": ["audio.dts_hd_hr"]}, {"label": "AC3|DD", "value": "4", "standard_keys": ["audio.ac3", "audio.dd", "audio.ddp"]}, {"label": "TrueHD", "value": "5", "standard_keys": ["audio.truehd"]}, {"label": "DTS-X", "value": "6", "standard_keys": ["audio.dtsx"]}, {"label": "LPCM", "value": "7", "standard_keys": ["audio.lpcm"]}, {"label": "OPUS", "value": "8", "standard_keys": ["audio.opus"]}, {"label": "FLAC", "value": "9", "standard_keys": ["audio.flac"]}, {"label": "DTS-HDMA:X 7.1", "value": "10"}, {"label": "WAV", "value": "11", "standard_keys": ["audio.wav"]}, {"label": "MP3", "value": "12", "standard_keys": ["audio.mp3"]}, {"label": "AAC", "value": "13", "standard_keys": ["audio.aac"]}, {"label": "DTS", "value": "14", "standard_keys": ["audio.dts"]}, {"label": "APE", "value": "15", "standard_keys": ["audio.ape"]}, {"label": "DSD", "value": "16", "standard_keys": ["audio.dsd"]}, {"label": "PCM", "value": "17", "standard_keys": ["audio.pcm"]}, {"label": "OGG", "value": "21", "standard_keys": ["audio.ogg"]}, {"label": "其它 (Other)", "value": "22", "standard_keys": ["audio.other"]}], "standard": [{"label": "8K/4320P", "value": "1", "standard_keys": ["resolution.r4320p"]}, {"label": "4K/2160P", "value": "2", "standard_keys": ["resolution.r2160p"]}, {"label": "1080P", "value": "3", "standard_keys": ["resolution.r1080p"]}, {"label": "1440P", "value": "4", "standard_keys": ["resolution.r1440p"]}, {"label": "1080I", "value": "5", "standard_keys": ["resolution.r1080i"]}, {"label": "720P", "value": "6", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "7", "standard_keys": ["resolution.sd"]}, {"label": "其它 (Other)", "value": "14", "standard_keys": ["resolution.other"]}], "team": [{"label": "FFWEB", "value": "15"}, {"label": "其它 (Other)", "value": "14"}], "tags": [{"label": "连载", "value": "12", "standard_keys": ["tag.ongoing"]}, {"label": "完结", "value": "10", "standard_keys": ["tag.complete"]}, {"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "首发", "value": "2"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}, {"label": "短剧", "value": "8"}, {"label": "3D", "value": "11", "standard_keys": ["tag.3d"]}]}}`).Error
	})
}
