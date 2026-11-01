package db

import (
	"gorm.io/gorm"
)

// migration96 §59.412: 垃圾堆（pt.lajidui.top）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（49.7KB 直连抓取——CF 未拦）+
// docs/32/lajidui.md 568 行读全（促销/Dupe 优先级/打包/命名规范）。
// 实时=文档全槽一致；**文档漂移一处：douban_id 实时无**（文档 10.1
// 有——不配）；文档 tags 标题"17 个"计数错误（列表实际 16 项）。
// [4] 后缀标准形态；单模式站。
// 槽位特化：
//   - **type 16 项全库最多之一**：412 短剧（第八站）+406 Music Videos
//     三键 [mv,music_video,concert]（演唱会视频归属）+408 Audio=
//     [music,lossless_music]（音频总类）+410 Cartoon 少儿动画
//     label-only（无键）+411 Ebook+413 Game+414 APP=software+
//     415 Education+416 Audiobook 全配；404 纪录双键。
//   - **processing_sel=文件格式**（MKV/MP4/EPUB/ISO——第三种语义
//     特化站：咖啡 source_sel=地区/YHPP processing_sel=地区后）：
//     发布链无容器格式判定维度——不配（按需生长）；source_sel=
//     地区不配（咖啡定案）。
//   - medium 11 项全配键：无 UHD 独立——Blu-ray=[bluray,uhd_bluray]
//     双键归并（浦园式近亲归并）+Remux 双键+WEB-DL 承接 webrip。
//   - codec 7 项乱序（AV1=6/H.265=7 在尾）全配：H.264/H.265 独立
//     命名（无 X26x 联动零触发）+Xvid。
//   - audiocodec 13 项全配：WAV 配键（车站先例第二站）+DTS-HD(9)=
//     [dts_hd_ma,dts_hd_hr] 双键（无 MA/HR 细分）+DTS(3)=[dts,dtsx]
//     +E-AC-3=[ddp]+AC-3=[ac3,dd]+TrueHD 承接 Atmos。
//   - standard 8 项乱序（8k=7 首位）全配：2k=[r1440p]+8k/4k/SD/
//     Other+p/i 分立——全分辨率覆盖第二站。
//   - team 22 项 label-only：**FRDS(9) 在表直中第七站**+Other 兜底+
//     外站官组十八个+原创(3)；"原创"非官组语义。
//   - tags 16 项配键 11：**杜比(10)=[dolby_vision] 第三站**+**国英
//     双语(15)=[bilingual_audio] 精确配键首站**（13城多语不配因双语
//     非多语——本站 label 即双语精确匹配）+**HDR 四键泛家族第八站**
//     +英语=[english_audio]+单集=[ongoing]（分集语义）+禁转/DIY/
//     国语/中字/粤语/完结；首发不配（铁律）+4 项 label-only
//     （已刮削/三级/简英双语字幕/多音轨）。
//   - 字段：url+pt_gen+technical_info+uplver；nfo/douban_id 不配
//     （13城教训+实时无）。
//   - 站规注记：<100MB 禁发（软件文档/单曲例外）+标清 upscale 禁+
//     RMVB 禁+游戏类候选制（上传员以下须候选区）+RAR 压缩包禁+
//     Dupe 媒介优先级——发布预览人工把关。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(96, "lajidui_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'pt.lajidui.top' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'pt.lajidui.top'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Movies/电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "TV Series/电视剧", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "TV Shows/综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "Documentaries/纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "Animations/动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "Music Videos/音乐视频", "value": "406", "standard_keys": ["category.mv", "category.music_video", "category.concert"]}, {"label": "Sports/体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "Audio/音频", "value": "408", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "Misc/其他", "value": "409", "standard_keys": ["category.other"]}, {"label": "Cartoon/少儿动画", "value": "410"}, {"label": "Ebook/电子书", "value": "411", "standard_keys": ["category.ebook"]}, {"label": "ShortDrama/短剧", "value": "412", "standard_keys": ["category.short_drama"]}, {"label": "Game/游戏", "value": "413", "standard_keys": ["category.game"]}, {"label": "APP/软件", "value": "414", "standard_keys": ["category.software"]}, {"label": "Education/教育视频", "value": "415", "standard_keys": ["category.education"]}, {"label": "Audiobook/有声书", "value": "416", "standard_keys": ["category.audiobook"]}], "medium": [{"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray", "medium.uhd_bluray"]}, {"label": "HD DVD", "value": "2", "standard_keys": ["medium.hddvd"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "MiniBD", "value": "4", "standard_keys": ["medium.minibd"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "DVDR", "value": "6", "standard_keys": ["medium.dvdr", "medium.dvd"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "Track", "value": "9", "standard_keys": ["medium.track"]}, {"label": "WEB-DL", "value": "10", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "Other", "value": "11", "standard_keys": ["medium.other"]}], "codec": [{"label": "H.264", "value": "1", "standard_keys": ["video.h264"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "Xvid", "value": "3", "standard_keys": ["video.xvid"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}, {"label": "AV1", "value": "6", "standard_keys": ["video.av1"]}, {"label": "H.265", "value": "7", "standard_keys": ["video.h265"]}], "audiocodec": [{"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "DTS", "value": "3", "standard_keys": ["audio.dts", "audio.dtsx"]}, {"label": "MP3", "value": "4", "standard_keys": ["audio.mp3"]}, {"label": "OGG", "value": "5", "standard_keys": ["audio.ogg"]}, {"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}, {"label": "WAV", "value": "8", "standard_keys": ["audio.wav"]}, {"label": "DTS-HD", "value": "9", "standard_keys": ["audio.dts_hd_ma", "audio.dts_hd_hr"]}, {"label": "TrueHD", "value": "10", "standard_keys": ["audio.truehd", "audio.truehd_atmos"]}, {"label": "LPCM", "value": "11", "standard_keys": ["audio.lpcm"]}, {"label": "E-AC-3", "value": "12", "standard_keys": ["audio.ddp"]}, {"label": "AC-3", "value": "13", "standard_keys": ["audio.ac3", "audio.dd"]}], "standard": [{"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "2k", "value": "5", "standard_keys": ["resolution.r1440p"]}, {"label": "4k", "value": "6", "standard_keys": ["resolution.r2160p"]}, {"label": "8k", "value": "7", "standard_keys": ["resolution.r4320p"]}, {"label": "Other", "value": "8", "standard_keys": ["resolution.other"]}], "team": [{"label": "HDSky", "value": "1"}, {"label": "CHD", "value": "2"}, {"label": "原创", "value": "3"}, {"label": "WiKi", "value": "4"}, {"label": "Other", "value": "5"}, {"label": "HHWEB", "value": "6"}, {"label": "ADE", "value": "7"}, {"label": "CMCT", "value": "8"}, {"label": "FRDS", "value": "9"}, {"label": "TJUPT", "value": "10"}, {"label": "UBits", "value": "11"}, {"label": "Ourbits", "value": "12"}, {"label": "QHstudIo", "value": "13"}, {"label": "HDHome", "value": "14"}, {"label": "AGSVWEB", "value": "15"}, {"label": "Pter", "value": "16"}, {"label": "CatEDU", "value": "17"}, {"label": "beAst", "value": "18"}, {"label": "LHD", "value": "19"}, {"label": "BMDru", "value": "20"}, {"label": "BeiTai", "value": "21"}, {"label": "GodDramas", "value": "22"}], "tags": [{"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "首发", "value": "2"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}, {"label": "已刮削", "value": "8"}, {"label": "完结", "value": "9", "standard_keys": ["tag.complete"]}, {"label": "杜比", "value": "10", "standard_keys": ["tag.dolby_vision"]}, {"label": "粤语", "value": "11", "standard_keys": ["tag.cantonese_audio"]}, {"label": "单集", "value": "12", "standard_keys": ["tag.ongoing"]}, {"label": "三级", "value": "13"}, {"label": "英语", "value": "14", "standard_keys": ["tag.english_audio"]}, {"label": "国英双语", "value": "15", "standard_keys": ["tag.bilingual_audio"]}, {"label": "简英双语字幕", "value": "16"}, {"label": "多音轨", "value": "17"}]}}`).Error
	})
}
