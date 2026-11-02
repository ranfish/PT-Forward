package db

import (
	"gorm.io/gorm"
)

// migration126 §59.443: 包子（p.t-baozi.cc）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（74.9KB 直连）+ docs/32/baozi.md
// 425 行读全（rules.php+Dupe+促销规则完整采集）。
// **实时 vs 资产 vs 文档一处差异：tags 22 项实时新增 SDR(25)**
// （资产/文档 21 项无——真改版 label-only 防松查乱挂）；文档 tags
// 计数 22 实列 21（计数错实时仲裁）。
// 槽位映射：
//   - type 10：410 短剧第十七站+408 高品质音频=[music,lossless_music]
//     双键+406 音乐视频=[mv,music_video] 双键+404 双键；无游戏/软件。
//   - **medium 13 原盘×DIY 四分（海的宝同款——§59.407 diyMediumOption
//     自动生效第九站）**：UHD 原盘(11)=[uhd_bluray] 分立单键+UHD
//     DIY(12) label-only+Blu-ray 原盘(1)=[bluray] 分立单键+Blu-ray
//     DIY(13) label-only（DIY 2 项动态分发标题词形）+Remux(3)=
//     [remux,uhd_remux] 双键+WEB-DL(10)=[webdl,webrip] 双键承接+
//     MiniBD/HD DVD/HDTV/**DVDR(6)=[dvdr] 单键精确（无 DVD 项）**+
//     Encode/CD/Track；无 Other 项 miss 落空预览人工。
//   - codec 6 全配键：H.265/H.264/VC-1/MPEG-2/AV1/Other——无 X26x
//     零联动（合并写法站方语义）。
//   - **audiocodec 15 全配键**：DTS:X(8)=[dtsx]+TrueHD Atmos(9)=
//     [atmos,truehd_atmos] 双键（躺平模式）+DTS-HD MA/TrueHD 分立+
//     **DD/AC3(1)=[ac3,dd,ddp] 三键承接（无 DDP 分立——大青虫先例）**
//     +**OPUS(2)/M4A(5) ext 活键**（青蛙 OPUS 先例）+LPCM/DTS/AAC/
//     FLAC/APE/WAV/MP3/Other。
//   - standard 7：8K/4K/1080p/1080i/720p/SD 全配+None(7) label-only
//     （非视频资源语义）。
//   - team 7：**BAOZIWEB(6)/Baozi(7) 站方官组 label-only**+HDS/CHD/
//     MySiLU/WiKi 外站 label-only+Other(5)=[team.other] 兜底。
//   - **tags 22 五绝不配**（原创 9/首发 2/禁转 1/限转 16/成人 23
//     ——官方专属财神雷区+成人区标记）+配键 8：国语/粤语/中字/完结/
//     DIY/**Dolby Vision(13)=[dolby_vision]+HDR10(14)=[hdr10]+
//     HDR10+(15)=[hdr10_plus] 三族独立**+**7 HDR=[hdr_vivid,hlg]
//     泛剩余键（星湾同款精细化分配——独立项各归位后泛项只承接
//     剩余 Vivid/HLG）**；11 动画/17 应求/18 MV/19 卡拉OK/20 LIVE
//     现场/21 演唱会/22 音乐专辑 label-only（分类/音乐语义 dict 无
//     键）+**25 SDR label-only（实时新增真改版）**。
//   - 字段：type/medium_sel[4]/codec_sel[4]/audiocodec_sel[4]/
//     standard_sel[4]/team_sel[4]/tags[4][]/small_descr/descr/
//     technical_info+url+pt_gen+uplver+anonymous:true；nfo 不配
//     （13城先例域表无此域）。
// 站规人工把关：Dupe 规则（标准 HD 站规则）+资源打包规则+促销规则+
//   账号保留规则；成人(23) 标签绝不投递。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(126, "baozi_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'p.t-baozi.cc' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'p.t-baozi.cc'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "technical_info": "technical_info", "imdb_url": "url", "pt_gen": "pt_gen", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "剧集", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "音乐视频", "value": "406", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "体育运动", "value": "407", "standard_keys": ["category.sports"]}, {"label": "高品质音频", "value": "408", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "短剧", "value": "410", "standard_keys": ["category.short_drama"]}, {"label": "其他", "value": "409", "standard_keys": ["category.other"]}], "medium": [{"label": "Blu-ray DIY", "value": "13"}, {"label": "Blu-ray 原盘", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "UHD Blu-ray DIY", "value": "12"}, {"label": "UHD Blu-ray 原盘", "value": "11", "standard_keys": ["medium.uhd_bluray"]}, {"label": "WEB-DL", "value": "10", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "Track", "value": "9", "standard_keys": ["medium.track"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "DVDR", "value": "6", "standard_keys": ["medium.dvdr"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "MiniBD", "value": "4", "standard_keys": ["medium.minibd"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "HD DVD", "value": "2", "standard_keys": ["medium.hddvd"]}], "codec": [{"label": "H.265(HEVC)", "value": "6", "standard_keys": ["video.h265"]}, {"label": "H.264(AVC)", "value": "1", "standard_keys": ["video.h264"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "AV1", "value": "3", "standard_keys": ["video.av1"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "DTS:X", "value": "8", "standard_keys": ["audio.dtsx"]}, {"label": "TrueHD Atmos", "value": "9", "standard_keys": ["audio.atmos", "audio.truehd_atmos"]}, {"label": "DTS-HD MA", "value": "10", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "TrueHD", "value": "11", "standard_keys": ["audio.truehd"]}, {"label": "LPCM", "value": "12", "standard_keys": ["audio.lpcm"]}, {"label": "DTS", "value": "3", "standard_keys": ["audio.dts"]}, {"label": "DD/AC3", "value": "1", "standard_keys": ["audio.ac3", "audio.dd", "audio.ddp"]}, {"label": "OPUS", "value": "2", "standard_keys": ["audio.opus"]}, {"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "FLAC", "value": "13", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "14", "standard_keys": ["audio.ape"]}, {"label": "WAV", "value": "15", "standard_keys": ["audio.wav"]}, {"label": "MP3", "value": "4", "standard_keys": ["audio.mp3"]}, {"label": "M4A", "value": "5", "standard_keys": ["audio.m4a"]}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}], "standard": [{"label": "8K", "value": "6", "standard_keys": ["resolution.r4320p"]}, {"label": "4K", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "None", "value": "7"}], "team": [{"label": "BAOZIWEB", "value": "6"}, {"label": "Baozi", "value": "7"}, {"label": "HDS", "value": "1"}, {"label": "CHD", "value": "2"}, {"label": "MySiLU", "value": "3"}, {"label": "WiKi", "value": "4"}, {"label": "Other", "value": "5", "standard_keys": ["team.other"]}], "tags": [{"label": "原创", "value": "9"}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "粤语", "value": "8", "standard_keys": ["tag.cantonese_audio"]}, {"label": "首发", "value": "2"}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "完结", "value": "12", "standard_keys": ["tag.complete"]}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "动画", "value": "11"}, {"label": "Dolby Vision", "value": "13", "standard_keys": ["tag.dolby_vision"]}, {"label": "HDR10", "value": "14", "standard_keys": ["tag.hdr10"]}, {"label": "HDR10+", "value": "15", "standard_keys": ["tag.hdr10_plus"]}, {"label": "禁转", "value": "1"}, {"label": "限转", "value": "16"}, {"label": "应求", "value": "17"}, {"label": "MV", "value": "18"}, {"label": "卡拉OK", "value": "19"}, {"label": "LIVE现场", "value": "20"}, {"label": "演唱会", "value": "21"}, {"label": "成人", "value": "23"}, {"label": "SDR", "value": "25"}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr_vivid", "tag.hlg"]}, {"label": "音乐专辑", "value": "22"}]}}`).Error
	})
}
