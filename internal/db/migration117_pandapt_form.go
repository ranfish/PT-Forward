package db

import (
	"gorm.io/gorm"
)

// migration117 §59.434: 熊猫（pandapt.net）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（63KB 直连）+ docs/32/pandapt.md
// 397 行读全。实时 vs 4 月资产全槽一致（零漂移站）；文档 §60"站点
// 特色：无 AV1"为过时表述（实时+资产 codec 均含 AV1(8)）。
// 槽位映射（双模式站 mode=4 种子区/5 特别区——§59.372 browsecat/
// specialcat 判别+§59.388 模式互斥过滤自动护；发布链走 mode=4 种子区，
// 特别区电子书/有声书/MV/音乐 type miss 落空预览人工——我的PT/OKPT
// 同款定案；team 种子区/特别区同名不同 ID——配置只做 [4] 区）：
//   - type 11：**415 短剧第十四站**+413 演唱会/音乐会=[concert] 单键
//     （我堡 419 同款独立分类）+412=[software]+411=[game]+404 双键。
//   - **medium 12 全配键零 label-only（大青虫/奥申后第三例媒介全
//     canonical 大满贯）**：UHD/Blu-ray 分立+UHDTV/MiniBD/HD DVD/
//     DVDR/Track 全在+Remux=[remux,uhd_remux] 双键归并（无独立 UHD
//     Remux 项）+WEB-DL=[webdl,webrip]。
//   - codec 8：HEVC/H.265/x265+AVC/H.264/x264 合并命名（零联动=站方
//     合并语义）+VP8/VP9=[vp9]+VC-1/Xvid/MPEG-2/AV1/Other 全配。
//   - audiocodec 13：AV3A=[av3a]（extended 活键——躺平/麒麟先例）+
//     **TrueHD Atmos/TrueHD 分立+DTS-HD/DTS-HR 分立**（站方细粒度
//     分立各单键——无需家族承接）+DTS-X=[dtsx]+DD/AC3=[ac3,dd]+
//     DDP/EAC3=[ddp]+LPCM/PCM=[lpcm,pcm] 双键合并+AAC/FLAC/Other。
//   - standard 6：4320k/8K 首乱序+SD(7) 尾全配键。
//   - team 9：Panda 系官组（原盘 diy/Remux/压制）+AilM 系（Web/TV/
//     Upscale）+CatEDU（部分禁转）+Red Leaves 红叶 label-only（官组
//     绝不配键——组映射层 label 直配，PANDA/AilM 系资产在表）+
//     Other(5)=[team.other] 兜底。
//   - source_sel[4]=产区槽不配（CHN/EU/HK/JPN/KOR/IND/SEA 8 区——
//     产区语义历史定案预览人工；壹吧"质量"语义不同族）。
//   - tags 19 配键 11 项：DIY/完结=[complete]/分集=[ongoing]/国语/
//     粤语/中字/特效+**HDR 四族细分独立（优堡/天空/家园/我堡后第五
//     站）**：杜比视界=[dolby_vision]/HDR10+=[hdr10_plus]/HDR10=
//     [hdr10]/菁彩HDR=[hdr_vivid]/HLG=[hlg]；禁转/驻站（官方专属）/
//     纯净版/横屏HOR/竖屏VER/应求/自购（官方专属）label-only。
//   - 字段：**tmdb 独立字段首见**（data-pt-gen="tmdb"——发布链无
//     tmdb 域不配，TMDB 信息走 pt_gen 链）+url（imdb_url）+**pt_gen
//     真字段**（data-pt-gen="pt_gen" 带获取按钮）+technical_info+
//     uplver+anonymous:true；approval（候选发布勾选——默认不勾直发，
//     不配同 offers 定案）；nfo 不配。
// 站规人工把关：标题规范（电影/剧集/音乐类命名）+简介格式（影视类
// 引用 PTGen）+站免池+复活区+官方组导航（DIY/Encode/REMUX/WEB-DL/
// HDTV/Upscale 按媒介筛选）+CatEDU 部分禁转。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(117, "pandapt_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'pandapt.net' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'pandapt.net'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "电视剧", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "短剧", "value": "415", "standard_keys": ["category.short_drama"]}, {"label": "动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "软件", "value": "412", "standard_keys": ["category.software"]}, {"label": "游戏", "value": "411", "standard_keys": ["category.game"]}, {"label": "演唱会/音乐会", "value": "413", "standard_keys": ["category.concert"]}, {"label": "其他", "value": "409", "standard_keys": ["category.other"]}], "medium": [{"label": "UHD Blu-ray", "value": "11", "standard_keys": ["medium.uhd_bluray"]}, {"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "Track", "value": "9", "standard_keys": ["medium.track"]}, {"label": "WEB-DL", "value": "10", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "DVDR", "value": "6", "standard_keys": ["medium.dvdr"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "UHDTV", "value": "12", "standard_keys": ["medium.uhdtv"]}, {"label": "MiniBD", "value": "4", "standard_keys": ["medium.minibd"]}, {"label": "HD DVD", "value": "2", "standard_keys": ["medium.hddvd"]}], "codec": [{"label": "HEVC/H.265/x265", "value": "6", "standard_keys": ["video.h265"]}, {"label": "AVC/H.264/x264", "value": "1", "standard_keys": ["video.h264"]}, {"label": "VP8/VP9", "value": "7", "standard_keys": ["video.vp9"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "Xvid", "value": "3", "standard_keys": ["video.xvid"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}, {"label": "AV1", "value": "8", "standard_keys": ["video.av1"]}], "audiocodec": [{"label": "AV3A", "value": "19", "standard_keys": ["audio.av3a"]}, {"label": "TrueHD Atmos", "value": "8", "standard_keys": ["audio.truehd_atmos"]}, {"label": "TrueHD", "value": "1", "standard_keys": ["audio.truehd"]}, {"label": "DTS-X", "value": "2", "standard_keys": ["audio.dtsx"]}, {"label": "DTS-HD", "value": "3", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "DTS-HR", "value": "18", "standard_keys": ["audio.dts_hd_hr"]}, {"label": "DTS", "value": "4", "standard_keys": ["audio.dts"]}, {"label": "DD/AC3", "value": "5", "standard_keys": ["audio.ac3", "audio.dd"]}, {"label": "DDP/EAC3", "value": "6", "standard_keys": ["audio.ddp"]}, {"label": "AAC", "value": "7", "standard_keys": ["audio.aac"]}, {"label": "FLAC", "value": "11", "standard_keys": ["audio.flac"]}, {"label": "LPCM/PCM", "value": "9", "standard_keys": ["audio.lpcm", "audio.pcm"]}, {"label": "Other", "value": "16", "standard_keys": ["audio.other"]}], "standard": [{"label": "4320k/8K", "value": "6", "standard_keys": ["resolution.r4320p"]}, {"label": "2160p/4K", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "7", "standard_keys": ["resolution.sd"]}], "team": [{"label": "Panda(原盘diy组)", "value": "6"}, {"label": "Panda(原盘Remux组)", "value": "10"}, {"label": "Panda(压制组)", "value": "1"}, {"label": "AilMWeb(流媒体组)", "value": "7"}, {"label": "AilMTV(电视录制组)", "value": "8"}, {"label": "AilMUpscale(超分视频组)", "value": "14"}, {"label": "CatEDU(部分禁转)", "value": "15"}, {"label": "Red Leaves (红叶)", "value": "22"}, {"label": "Other", "value": "5", "standard_keys": ["team.other"]}], "tags": [{"label": "禁转", "value": "1"}, {"label": "驻站", "value": "20"}, {"label": "纯净版", "value": "16"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "完结", "value": "10", "standard_keys": ["tag.complete"]}, {"label": "分集", "value": "17", "standard_keys": ["tag.ongoing"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "粤语", "value": "13", "standard_keys": ["tag.cantonese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "特效", "value": "12", "standard_keys": ["tag.special_effects_subs"]}, {"label": "杜比视界", "value": "8", "standard_keys": ["tag.dolby_vision"]}, {"label": "HDR10+", "value": "9", "standard_keys": ["tag.hdr10_plus"]}, {"label": "HDR10", "value": "7", "standard_keys": ["tag.hdr10"]}, {"label": "菁彩HDR", "value": "15", "standard_keys": ["tag.hdr_vivid"]}, {"label": "HLG", "value": "22", "standard_keys": ["tag.hlg"]}, {"label": "横屏HOR", "value": "2"}, {"label": "竖屏VER", "value": "23"}, {"label": "应求", "value": "11"}, {"label": "自购", "value": "14"}]}}`).Error
	})
}
