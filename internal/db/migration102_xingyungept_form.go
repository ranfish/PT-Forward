package db

import (
	"gorm.io/gorm"
)

// migration102 §59.418: 星陨阁（pt.xingyungept.org）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（47.9KB 直连）+ docs/32/xingyungept.md
// 336 行读全（rules.php 规则/dupe 优先级/打包规范/标题规范）。
// 实时 vs 文档+资产两处漂移（实时为准）：
//   - **tags 24→22 项**：站方已删 完结(11)/分集(10)——complete/ongoing
//     键 miss 落空（配键 13 非文档口径的 15）。
//   - **实时无 uplver**（文档称有"匿名发布 uplver"——控件实证无，
//     不配）。
// 槽位映射：
//   - type 10 项倒序：406 MV 三键第九站 [mv,music_video,concert]+
//     408 音频=[music,lossless_music] 双键+410 短剧第十一站+404
//     纪录双键+全配键。
//   - medium 10 项乱序：**UHD 独立分立**（Blu-ray(1)/UHD Blu-ray(2)
//     各单键——躺平同款）+Remux 双键+WEB-DL(4) 承接 webrip+**DVD(6)
//     单项宽语义=[dvd,dvdr] 双键承接**（站方允许 DVDR/DVDISO/
//     DVDRip——无独立 DVDR 项归并惯例）+CD/Track 全配；无 MiniBD/
//     HD DVD（miss 落空）。
//   - codec 6 项全配键（H.264/AVC+H.265/HEVC 合并命名——无 X26x
//     联动零触发）+AV1；无 Xvid/VP9/MPEG-4（miss 落空）。
//   - standard 6 项 i/p 合并形态（咖啡/躺平同款）：1080 双键
//     [r1080p,r1080i]+720/2160/4320 各 p 单键（i 形态无键）+
//     480p/480i=[sd]+Other=[resolution.other]。
//   - **audiocodec 18 项全配键（躺平后第二例零 label-only）**：
//     TrueHD Atmos=[atmos,truehd_atmos] 双键（组合路径活键）+
//     DTS:X=[dtsx]+**AV3V=[av3v]**+**ALAC=[alac]**（dict 长尾键）+
//     M4A/LPCM+DD/AC3=[ac3,dd] 双键+全独立。
//   - team 12 项 label-only：StarfallWeb(8) 官组+Pure@StarfallWeb(12)
//     @子组+rain(6)/rainweb(7)/NatureWeb(11)/Starfall(10) 站组系+
//     AGSVWEB(9)+HDS/CHD/MySiLU/WiKi 传统组+Other(5) 兜底（FRDS
//     不在表）。
//   - **tags 22 项配键 13（dict 长尾键丰收站）**：**英字=
//     english_subtitle 第二站+特效字幕=special_effects_subs 首配**
//     （"特效字幕"明确字幕语义——藏宝阁/慕雪阁"特效字慕"错别字
//     歧义未配对照）+**超分=upscale 首配**（超分=upscale 中文——
//     站方特色标签标 AI 超分修复版）+**高帧率=high_frame_rate 首配**
//     （原生高刷——与补帧后处理分立：补帧 label-only）+**高码率=
//     high_bitrate 首配**（三月漏配教训本站配）+大包=big_pack
//     第二站+杜比视界第七站+HDR 四键泛家族第十一站+禁转=restricted+
//     DIY/国语/中字/粤语；首发(2) 铁律 label-only+驻站/去头尾广告
//     纯净版/补帧/特效/应求/美剧/韩剧/原生（original_audio/disc
//     二义）8 项 label-only。
//   - 字段：url+pt_gen+technical_info；nfo 不配（13城教训）+无
//     uplver（实时漂移）。
// 站规人工把关（躺平系同款）：<100MB 禁+upscale 禁+RMVB 禁+游戏
// 候选制（上传员级）+RAR 禁+Dupe 媒介优先级+断种豁免（45 日/18 月）+
// <5.1 有损音频禁。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(102, "xingyungept_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'pt.xingyungept.org' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'pt.xingyungept.org'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "technical_info": "technical_info"}, "framework": "np", "value_mappings": {"type": [{"label": "其他", "value": "409", "standard_keys": ["category.other"]}, {"label": "音频", "value": "408", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "MV", "value": "406", "standard_keys": ["category.mv", "category.music_video", "category.concert"]}, {"label": "体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "短剧", "value": "410", "standard_keys": ["category.short_drama"]}, {"label": "纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "电视剧", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "电影", "value": "401", "standard_keys": ["category.movie"]}], "medium": [{"label": "Other", "value": "10", "standard_keys": ["medium.other"]}, {"label": "Track", "value": "9", "standard_keys": ["medium.track"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "DVD", "value": "6", "standard_keys": ["medium.dvd", "medium.dvdr"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "WEB-DL", "value": "4", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "UHD Blu-ray", "value": "2", "standard_keys": ["medium.uhd_bluray"]}], "codec": [{"label": "H.264/AVC", "value": "1", "standard_keys": ["video.h264"]}, {"label": "H.265/HEVC", "value": "2", "standard_keys": ["video.h265"]}, {"label": "VC-1", "value": "3", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "AV1", "value": "5", "standard_keys": ["video.av1"]}, {"label": "Other", "value": "6", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "ALAC", "value": "15", "standard_keys": ["audio.alac"]}, {"label": "AAC", "value": "14", "standard_keys": ["audio.aac"]}, {"label": "APE", "value": "13", "standard_keys": ["audio.ape"]}, {"label": "TrueHD Atmos", "value": "12", "standard_keys": ["audio.atmos", "audio.truehd_atmos"]}, {"label": "DDP/E-AC3", "value": "11", "standard_keys": ["audio.ddp"]}, {"label": "DD/AC3", "value": "10", "standard_keys": ["audio.ac3", "audio.dd"]}, {"label": "LPCM", "value": "9", "standard_keys": ["audio.lpcm"]}, {"label": "TrueHD", "value": "8", "standard_keys": ["audio.truehd"]}, {"label": "DTS:X", "value": "7", "standard_keys": ["audio.dtsx"]}, {"label": "DTS-HD MA", "value": "6", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "DTS", "value": "5", "standard_keys": ["audio.dts"]}, {"label": "M4A", "value": "4", "standard_keys": ["audio.m4a"]}, {"label": "WAV", "value": "3", "standard_keys": ["audio.wav"]}, {"label": "MP3", "value": "2", "standard_keys": ["audio.mp3"]}, {"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "Other", "value": "16", "standard_keys": ["audio.other"]}, {"label": "OPUS", "value": "17", "standard_keys": ["audio.opus"]}, {"label": "AV3V", "value": "18", "standard_keys": ["audio.av3v"]}], "standard": [{"label": "480p/480i", "value": "1", "standard_keys": ["resolution.sd"]}, {"label": "720p/720i", "value": "2", "standard_keys": ["resolution.r720p"]}, {"label": "1080p/1080i", "value": "3", "standard_keys": ["resolution.r1080p", "resolution.r1080i"]}, {"label": "4K/2160p/2160i", "value": "4", "standard_keys": ["resolution.r2160p"]}, {"label": "8k/4320p/4320i", "value": "5", "standard_keys": ["resolution.r4320p"]}, {"label": "Other", "value": "6", "standard_keys": ["resolution.other"]}], "team": [{"label": "StarfallWeb", "value": "8"}, {"label": "Pure@StarfallWeb", "value": "12"}, {"label": "WiKi", "value": "4"}, {"label": "MySiLU", "value": "3"}, {"label": "HDS", "value": "1"}, {"label": "CHD", "value": "2"}, {"label": "Other", "value": "5"}, {"label": "rainweb", "value": "7"}, {"label": "rain", "value": "6"}, {"label": "AGSVWEB", "value": "9"}, {"label": "Starfall", "value": "10"}, {"label": "NatureWeb", "value": "11"}], "tags": [{"label": "驻站", "value": "23"}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "去头尾广告纯净版", "value": "25"}, {"label": "补帧", "value": "20"}, {"label": "超分", "value": "19", "standard_keys": ["tag.upscale"]}, {"label": "特效", "value": "18"}, {"label": "大包", "value": "17", "standard_keys": ["tag.big_pack"]}, {"label": "应求", "value": "16"}, {"label": "英字", "value": "15", "standard_keys": ["tag.english_subtitle"]}, {"label": "韩剧", "value": "14"}, {"label": "美剧", "value": "13"}, {"label": "粤语", "value": "12", "standard_keys": ["tag.cantonese_audio"]}, {"label": "特效字幕", "value": "9", "standard_keys": ["tag.special_effects_subs"]}, {"label": "杜比视界", "value": "8", "standard_keys": ["tag.dolby_vision"]}, {"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "首发", "value": "2"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}, {"label": "高码率", "value": "27", "standard_keys": ["tag.high_bitrate"]}, {"label": "高帧率", "value": "26", "standard_keys": ["tag.high_frame_rate"]}, {"label": "原生", "value": "24"}]}}`).Error
	})
}
