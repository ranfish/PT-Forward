package db

import (
	"gorm.io/gorm"
)

// migration95 §59.411: 藏宝阁（cangbao.ge）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（54.5KB 直连抓取）+ docs/32/cangbao.md
// 641 行读全（MetaFlow 自动填表/发种规则帖/多季合集规范）。实时=文档
// §3.7 终版一致（tags 19 项——文档 4.6"跳过 17"说法有误，实时 17=DIY
// 在）；[4] 后缀标准形态；无 Cloudflare。
// 槽位特化：
//   - **全字段值倒序排列（全库唯一）**：媒介 UHD=9→VCD=1/编码 AV1=9→
//     H.261=1/音频 PCM=9→MP3=1/分辨率 4320p=7→480P=1——值越大质量越高，
//     映射不能假设常见站值序。
//   - type 10 项全非标值集（第四套）：403 短剧（第七站）+**404 动画/
//     405 动漫分立——站方多集信号双规则校验（动画检出多集=错/动漫无
//     多集=错），cbgCategoryValue 特化分发（typeMappingOf 第三站——
//     SeasonEpisode 非空→405/空→404；404 配 animation 键作单集通用
//     兜底+405 label-only 由特化按 value 取）**+406 儿童 label-only
//     （genre.children 是题材域键不配）+408 纪录双键+410 书籍=ebook。
//   - medium 9 项：WEB-DL 承接 webrip（无独立 WEBRip）+UHDTV 配键
//     （农场先例第二站）+Remux/DVDR 归并；**VCD label-only（无键）**；
//     无 MiniBD/CD/Track/DIY。
//   - codec 9 项：AVC/HEVC 简短命名（无 X26x 联动零触发）+AV1/VP9+
//     MPEG-4 配键；**MPEG-1/H.261 label-only（无键）**；无 Xvid。
//   - audiocodec 9 项：PCM=[lpcm]（dict 无 pcm 键）+Opus 配键（青蛙
//     先例）+TrueHD 承接 truehd_atmos（站方无独立 Atmos 项——atmos
//     泛键 miss 保守）+**DTS 单项承接家族四键**（车站式——站方不分
//     细分，DTS-HD MA/HR/X 皆落 DTS）+AC3=[ac3,dd]；无 DDP Atmos 项。
//   - standard 7 项倒序：4320p/2160p/1440p/1080p/720p 全配+**480P=
//     [sd] 单配（r540p 无键——540p label-only 防一键多值）**。
//   - team 3 项 label-only 全库最少之一：NONE(1) 兜底语义（非官组选
//     NONE——miss→nil 不填=站方表单默认态）+CBGWT/CBGER 官组在
//     release_group_mappings（is_official=1——label 直配自动命中）。
//   - tags 19 项（值 6 缺）：**配键 11 个**——完结/分集/中字/国语/
//     **英字=english_subtitle（首次配键站）**/禁转/**HDR 四键泛家族
//     第七站**/**DV=dolby_vision（第二站——优堡后）**/**原盘=
//     original_disc**/DIY/**合集=collection**；首发不配（铁律）+
//     官方不配（财神官种案雷区）+7 项 label-only（自购/驻站/零魔/
//     高码/特效/原声/540p 同理）。
//   - 字段：url+pt_gen（四源）+technical_info+uplver；nfo 不配
//     （13城教训——发布链不投递+域表无此域）；无 processing/source。
//   - 站规注记：剧集转载必须完结标签严禁分集（官组除外）+低于 1080p
//     电影慎重+多季合集四统一（分辨率/格式/制作组/完整性）+音乐/
//     书籍豁免质量字段——发布预览人工把关。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(95, "cangbao_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'cangbao.ge' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'cangbao.ge'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "剧集", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "短剧", "value": "403", "standard_keys": ["category.short_drama"]}, {"label": "动画", "value": "404", "standard_keys": ["category.animation"]}, {"label": "动漫", "value": "405"}, {"label": "儿童", "value": "406"}, {"label": "综艺", "value": "407", "standard_keys": ["category.tv_shows"]}, {"label": "纪录", "value": "408", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "音乐", "value": "409", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "书籍", "value": "410", "standard_keys": ["category.ebook"]}], "medium": [{"label": "UHD", "value": "9", "standard_keys": ["medium.uhd_bluray"]}, {"label": "Remux", "value": "8", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "Blu-ray", "value": "7", "standard_keys": ["medium.bluray"]}, {"label": "WEB-DL", "value": "6", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "UHDTV", "value": "5", "standard_keys": ["medium.uhdtv"]}, {"label": "HDTV", "value": "4", "standard_keys": ["medium.hdtv"]}, {"label": "Encode", "value": "3", "standard_keys": ["medium.encode"]}, {"label": "DVD", "value": "2", "standard_keys": ["medium.dvdr", "medium.dvd"]}, {"label": "VCD", "value": "1"}], "codec": [{"label": "AV1", "value": "9", "standard_keys": ["video.av1"]}, {"label": "HEVC", "value": "8", "standard_keys": ["video.h265"]}, {"label": "VP9", "value": "7", "standard_keys": ["video.vp9"]}, {"label": "AVC", "value": "6", "standard_keys": ["video.h264"]}, {"label": "VC-1", "value": "5", "standard_keys": ["video.vc1"]}, {"label": "MPEG-4", "value": "4", "standard_keys": ["video.mpeg4"]}, {"label": "MPEG-2", "value": "3", "standard_keys": ["video.mpeg2"]}, {"label": "MPEG-1", "value": "2"}, {"label": "H.261", "value": "1"}], "audiocodec": [{"label": "PCM", "value": "9", "standard_keys": ["audio.lpcm"]}, {"label": "TrueHD", "value": "8", "standard_keys": ["audio.truehd", "audio.truehd_atmos"]}, {"label": "FLAC", "value": "7", "standard_keys": ["audio.flac"]}, {"label": "Opus", "value": "6", "standard_keys": ["audio.opus"]}, {"label": "DDP", "value": "5", "standard_keys": ["audio.ddp"]}, {"label": "AAC", "value": "4", "standard_keys": ["audio.aac"]}, {"label": "DTS", "value": "3", "standard_keys": ["audio.dts", "audio.dts_hd_ma", "audio.dts_hd_hr", "audio.dtsx"]}, {"label": "AC3", "value": "2", "standard_keys": ["audio.ac3", "audio.dd"]}, {"label": "MP3", "value": "1", "standard_keys": ["audio.mp3"]}], "standard": [{"label": "4320p", "value": "7", "standard_keys": ["resolution.r4320p"]}, {"label": "2160p", "value": "6", "standard_keys": ["resolution.r2160p"]}, {"label": "1440p", "value": "5", "standard_keys": ["resolution.r1440p"]}, {"label": "1080p", "value": "4", "standard_keys": ["resolution.r1080p"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "540p", "value": "2"}, {"label": "480P", "value": "1", "standard_keys": ["resolution.sd"]}], "team": [{"label": "NONE", "value": "1"}, {"label": "CBGWT", "value": "2"}, {"label": "CBGER", "value": "3"}], "tags": [{"label": "首发", "value": "1"}, {"label": "自购", "value": "2"}, {"label": "完结", "value": "3", "standard_keys": ["tag.complete"]}, {"label": "分集", "value": "4", "standard_keys": ["tag.ongoing"]}, {"label": "驻站", "value": "5"}, {"label": "中字", "value": "7", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "国语", "value": "8", "standard_keys": ["tag.chinese_audio"]}, {"label": "英字", "value": "9", "standard_keys": ["tag.english_subtitle"]}, {"label": "禁转", "value": "10", "standard_keys": ["tag.restricted"]}, {"label": "零魔", "value": "11"}, {"label": "高码", "value": "12"}, {"label": "HDR", "value": "13", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}, {"label": "DV", "value": "14", "standard_keys": ["tag.dolby_vision"]}, {"label": "原盘", "value": "15", "standard_keys": ["tag.original_disc"]}, {"label": "特效", "value": "16"}, {"label": "DIY", "value": "17", "standard_keys": ["tag.diy"]}, {"label": "原声", "value": "18"}, {"label": "官方", "value": "19"}, {"label": "合集", "value": "20", "standard_keys": ["tag.collection"]}]}}`).Error
	})
}
