package db

import (
	"gorm.io/gorm"
)

// migration130 §59.447: 萝莉（mua.xloli.cc）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（55.5KB 代理）+ docs/32/xloli.md
// 564 行读全（"没有规则的规则"+禁发制作组黑名单完整采集）。
// **实时 vs 资产差异**：**team -ADE(55)/@ADE(56) 新增**（资产 13→
// 16——@ADE @子组形态双行式）+**bangumi_url/tmdb_id/mal_id 三番组
// 编号字段资产未捕获**（实时发现——发布链无域不配熊猫 tmdb 定案）；
// 其余全槽一致。
// 槽位映射：
//   - **双模式站**（type×2 同 name 裸名：mode=4 普通区 10 项✓/
//     成人区 12 项 418-429 AV/里番/H 系/写真/音声⛔——§59.372 双
//     type 互斥双防线+§59.388 模式后缀过滤双防线自动护——**[6] 槽
//     全套混入**（source/medium/codec/audiocodec/standard/team/
//     tags 七槽成人区控件一并过滤）；发布链走普通区）。
//   - type 10（普通区）：**410 舞台演出=[concert] 单键**+408 音乐
//     双键+**411 漫画/CG杂图/动漫杂志 label-only**（无键）+404
//     双键+430 综艺非标值=[tv_shows]。
//   - medium 9：UHD Blu-ray(14)/Blu-ray(1) 分立各单键+Remux(3)
//     双键归并+Encode+HDTV+WEB-DL(12)=[webdl,webrip] 双键承接+
//     **DVD(2)=[dvd] 单键（无 DVDR 项）**+CD+Other。
//   - codec 6 合并命名零联动：**AVC/x264(9)=[h264]+HEVC/x265(8)=
//     [h265] 合并项语义第八站反证**（§59.380 label 精确匹配 x264/
//     x265 不触发=站方合并项语义——车站先例）+AV1/VC-1/MPEG-2/
//     Other。
//   - audiocodec 12 全配键：**AC3/DD(13)=[ac3,dd] 双键+DDP/E-AC3
//     (20)=[ddp] 单键（eac3 无键 ddp 承接——Nova 先例）**+DTS:X(23)
//     =[dtsx]+**OPUS(24) ext 活键**+AAC/MP3/FLAC/DTS/TrueHD/
//     LPCM/DTS-HD MA/Other。
//   - standard 6：2160p/4K(5) 斜杠合并+1080p/i 分立+720p+SD+
//     Other(9)=[resolution.other]。
//   - team 16：**VCB-Studio/7³ACG/AI-Raws/ANK-Raws/LittleBakas!/
//     mawen1250/jsum@U2/Moozzi2/Snow-Raws/-ADE/@ADE/CMCT/
//     GodDramas/beAst 动漫 raws 组大表 label-only**（-ADE/@ADE
//     实时新增 @子组形态双行式）+Other(5)=[team.other] 兜底。
//   - **tags 21 五绝不配**（R18(8)/禁转(1)/原创(20)/自购(15)/
//     首发(2)——财神雷区全集+R18 成人标记）+配键 8：国语/粤语/
//     中字/**ENSub(18)=[english_subtitle]（英文 label 直配——藏宝
//     阁首配键复利）**/特效=[special_effects_subs]/DIY/DoVi=
//     [dolby_vision]/**HDR10(7)+HDR10+(14) 两族独立**+原生原盘(9)=
//     [original_disc]+分集(21)=[ongoing]；**Atmos(16) label-only
//     （dict 无 tag.atmos 红豆饭定案）**+LOLI(11)/无对白(24)/
//     GalGame(19)/其他(10) label-only。
//   - **source_sel 10=产区槽不配**（咖啡定案——日本/大陆/欧美等
//     产区数据无静态投递能力预览人工）。
//   - 字段：type/medium_sel[4]/codec_sel[4]/audiocodec_sel[4]/
//     standard_sel[4]/team_sel[4]/tags[4][]/small_descr/descr/
//     technical_info+url（IMDb）+**douban_id=豆瓣ID 编号（DoubanID
//     域——家面/家园先例）**+pt_gen+uplver；**bangumi_url/tmdb_id/
//     mal_id 三番组编号字段不配**（发布链无域——熊猫 tmdb 首见
//     定案）；nfo 不配（13城先例）；bangumi_match_option/
//     disable_bangumi_match（番组匹配 UI 字段发布链不处理）。
// 站规人工把关：**§4 禁发制作组黑名单**（公网 BT 盗转组——文档
//   §4.1 完整列表发布预览人工规避）+低分电视剧禁发（§3.8）+
//   分集资源规则（§3.4——§59.424 系统级禁发分集铁律已覆盖）+
//   已完结资源规则（§3.5）+打包合集规则+语言要求（§3.7）+
//   违规做种处罚（§3.9-3.10）。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(130, "xloli_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'mua.xloli.cc' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'mua.xloli.cc'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "technical_info": "technical_info", "imdb_url": "url", "douban_id": "douban_id", "pt_gen": "pt_gen", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影 (Movie)", "value": "401", "standard_keys": ["category.movie"]}, {"label": "电视剧(TV Series)", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "综艺(TV Show)", "value": "430", "standard_keys": ["category.tv_shows"]}, {"label": "动画 (Animation)", "value": "405", "standard_keys": ["category.animation"]}, {"label": "音乐 (Music)", "value": "408", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "舞台演出 (Stage Performance)", "value": "410", "standard_keys": ["category.concert"]}, {"label": "纪录片 (Documentary)", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "游戏 (Game)", "value": "412", "standard_keys": ["category.game"]}, {"label": "软件 (Software)", "value": "413", "standard_keys": ["category.software"]}, {"label": "漫画/CG杂图/动漫杂志", "value": "411"}], "medium": [{"label": "UHD Blu-ray", "value": "14", "standard_keys": ["medium.uhd_bluray"]}, {"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "WEB-DL", "value": "12", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "DVD", "value": "2", "standard_keys": ["medium.dvd"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "Other", "value": "11", "standard_keys": ["medium.other"]}], "codec": [{"label": "AVC/x264", "value": "9", "standard_keys": ["video.h264"]}, {"label": "HEVC/x265", "value": "8", "standard_keys": ["video.h265"]}, {"label": "AV1", "value": "15", "standard_keys": ["video.av1"]}, {"label": "VC-1", "value": "12", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2", "value": "13", "standard_keys": ["video.mpeg2"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "AC3/DD", "value": "13", "standard_keys": ["audio.ac3", "audio.dd"]}, {"label": "DDP/E-AC3", "value": "20", "standard_keys": ["audio.ddp"]}, {"label": "MP3", "value": "21", "standard_keys": ["audio.mp3"]}, {"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "DTS", "value": "3", "standard_keys": ["audio.dts"]}, {"label": "TrueHD", "value": "9", "standard_keys": ["audio.truehd"]}, {"label": "LPCM", "value": "15", "standard_keys": ["audio.lpcm"]}, {"label": "DTS-HD MA", "value": "10", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "DTS:X", "value": "23", "standard_keys": ["audio.dtsx"]}, {"label": "OPUS", "value": "24", "standard_keys": ["audio.opus"]}, {"label": "Other", "value": "14", "standard_keys": ["audio.other"]}], "standard": [{"label": "2160p/4K", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "Other", "value": "9", "standard_keys": ["resolution.other"]}], "team": [{"label": "VCB-Studio", "value": "11"}, {"label": "7³ACG", "value": "15"}, {"label": "AI-Raws", "value": "42"}, {"label": "ANK-Raws", "value": "48"}, {"label": "LittleBakas!", "value": "49"}, {"label": "mawen1250", "value": "50"}, {"label": "jsum@U2", "value": "40"}, {"label": "Moozzi2", "value": "53"}, {"label": "Snow-Raws", "value": "39"}, {"label": "-ADE", "value": "55"}, {"label": "@ADE", "value": "56"}, {"label": "CMCT", "value": "46"}, {"label": "GodDramas", "value": "41"}, {"label": "beAst", "value": "47"}, {"label": "Other", "value": "5", "standard_keys": ["team.other"]}], "tags": [{"label": "R18", "value": "8"}, {"label": "禁转", "value": "1"}, {"label": "原创", "value": "20"}, {"label": "自购", "value": "15"}, {"label": "首发", "value": "2"}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "粤语", "value": "23", "standard_keys": ["tag.cantonese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "ENSub", "value": "18", "standard_keys": ["tag.english_subtitle"]}, {"label": "特效", "value": "22", "standard_keys": ["tag.special_effects_subs"]}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "DoVi", "value": "13", "standard_keys": ["tag.dolby_vision"]}, {"label": "HDR10", "value": "7", "standard_keys": ["tag.hdr10"]}, {"label": "HDR10+", "value": "14", "standard_keys": ["tag.hdr10_plus"]}, {"label": "Atmos", "value": "16"}, {"label": "原生原盘", "value": "9", "standard_keys": ["tag.original_disc"]}, {"label": "LOLI", "value": "11"}, {"label": "无对白", "value": "24"}, {"label": "GalGame", "value": "19"}, {"label": "分集", "value": "21", "standard_keys": ["tag.ongoing"]}, {"label": "其他", "value": "10"}]}}`).Error
	})
}
