package db

import (
	"gorm.io/gorm"
)

// migration93 §59.409: 优堡（ubits.club）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（52.1KB 直连抓取）+ docs/32/ubits.md
// 381 行读全（候选制/转载互斥/黑名单组/标签规范 topicid=1495/促销）。
// **实时 HTML vs 4 月资产三处真改版**：
//   ① type 新增 415 有声书（9→10 项）
//   ② team_sel 新增 8 驻站（4→5 项）
//   ③ tags 官方(3) 站方已移除（18→17 项——财神官种案雷区标签站方
//     主动下线；资产该值实时为准不配）
// **`_token` CSRF 控件首见**（历史 25 站无）——发布链 UploadTorrent
// 不处理（挂账：首颗真实发布验证站方是否校验；若校验需表单页
// 预取 token 通用增强另案）。
// 槽位特化：
//   - **候选制站**（积分<10 自动候选区）：asoffer 不勾不配；
//     **转载互斥+黑名单**（CMCT 系组禁发——发布预览人工规避；
//     旧稿 HDSky 为杜撰已勘误 rules.php 实时无）
//   - type 10 项：409 演唱会(Misic Videos)=[concert,mv,music_video]
//     三键承接（站方拼写错误 Misic 原文——克隆同款合并语义）+
//     **415 有声书=audiobook（实时新增首配）**+406 音乐CD=
//     [music,lossless_music]+408 Other；404 纪录双键
//   - medium 10 项：8 Lossless Music 无 canonical 键 label-only
//     （音乐媒介 miss 落空）；DVDR 归并+WEB-DL 承接 webrip
//   - codec 9 项合并命名（H265(HEVC/x265)——无独立 X26x 联动零触发）
//     +AV1/AVS/MPEG-4 配键
//   - audiocodec 14 项细分全独立：Dolby Atmos/DTS:X/TrueHD/DD+/DD
//     五独立+**DTS-HD MA/HR 合并双键 [dts_hd_ma,dts_hd_hr]**
//     （站方合并项——首次遇到；区别于其它站分立）
//   - standard 7 项**p/i 分立+1440p 全七项**（4320p/2160p/1440p/
//     1080p/1080i/720p/SD——无 Other）
//   - source_sel 产区槽不配（§59.385 咖啡定案——必填与否首颗真实
//     发布验证）
//   - team 5 项 label-only：UBits/UBWEB/UBTV 官组+驻站（8 新增）+
//     Other（非管理组禁选官组）
//   - **tags 17 项——HDR 五族细分独立标签站（首个）**：杜比视界(8)=
//     dolby_vision/HDR10+(10)=hdr10_plus/HDR10(9)=hdr10/菁彩HDR(18)=
//     hdr_vivid/HLG(19)=hlg **五键各配独立**（泛家族四键归一在此站
//     不适用——站方有独立项精确配）；原生原盘(17)=original_disc+
//     连载=ongoing+合集=collection+禁转/国语/粤语/中字/DIY 配键；
//     官方已移除✓；高分国剧(21 限官组)/特效字幕/自购/菜单修改
//     label-only；HDR 互斥（杜比可共存/HDR* 互斥）与原盘互斥
//     （原生/DIY/菜单修改）与分集互斥（连载/合集）——站方规则
//     发布预览人工确认
//   - 字段：url（IMDb 必填）+pt_gen（豆瓣必填）+technical_info
//     （2025.04.11 后必填）+uplver；url×2 第二个是编辑器按钮
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(93, "ubits_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'ubits.club' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'ubits.club'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影(Movie)", "value": "401", "standard_keys": ["category.movie"]}, {"label": "纪录片(Documentaries)", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "动漫(Animations)", "value": "405", "standard_keys": ["category.animation"]}, {"label": "电视剧(TV Series)", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "综艺（TV Shows）", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "演唱会(Misic Videos)", "value": "409", "standard_keys": ["category.concert", "category.mv", "category.music_video"]}, {"label": "体育节目(Sports)", "value": "407", "standard_keys": ["category.sports"]}, {"label": "音乐CD(Music CD Tracker)", "value": "406", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "有声书", "value": "415", "standard_keys": ["category.audiobook"]}, {"label": "Other", "value": "408", "standard_keys": ["category.other"]}], "medium": [{"label": "4K UHD原盘(UltraHD Blu-ray)", "value": "10", "standard_keys": ["medium.uhd_bluray"]}, {"label": "蓝光原盘(Blu-ray)", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "流媒体(WEB-DL)", "value": "4", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "REMUX", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "压制(Encode)", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "HD DVD", "value": "2", "standard_keys": ["medium.hddvd"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "DVDR", "value": "6", "standard_keys": ["medium.dvdr", "medium.dvd"]}, {"label": "Lossless Music", "value": "8"}, {"label": "Track", "value": "9", "standard_keys": ["medium.track"]}], "codec": [{"label": "H265(HEVC/x265)", "value": "7", "standard_keys": ["video.h265"]}, {"label": "H264(AVC/x264)", "value": "1", "standard_keys": ["video.h264"]}, {"label": "AV1", "value": "11", "standard_keys": ["video.av1"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "AVS", "value": "10", "standard_keys": ["video.avs"]}, {"label": "Xvid", "value": "3", "standard_keys": ["video.xvid"]}, {"label": "MPEG-4", "value": "9", "standard_keys": ["video.mpeg4"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "Dolby Atmos", "value": "8", "standard_keys": ["audio.atmos", "audio.truehd_atmos"]}, {"label": "DTS:X", "value": "9", "standard_keys": ["audio.dtsx"]}, {"label": "TrueHD", "value": "10", "standard_keys": ["audio.truehd"]}, {"label": "DTS-HD MA/HR", "value": "11", "standard_keys": ["audio.dts_hd_ma", "audio.dts_hd_hr"]}, {"label": "LPCM", "value": "13", "standard_keys": ["audio.lpcm"]}, {"label": "DD+(Dolby Digital Plus)", "value": "12", "standard_keys": ["audio.ddp"]}, {"label": "DD(AC3)", "value": "14", "standard_keys": ["audio.ac3", "audio.dd"]}, {"label": "DTS", "value": "3", "standard_keys": ["audio.dts"]}, {"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "OGG", "value": "5", "standard_keys": ["audio.ogg"]}, {"label": "MP3", "value": "4", "standard_keys": ["audio.mp3"]}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}], "standard": [{"label": "4320p", "value": "6", "standard_keys": ["resolution.r4320p"]}, {"label": "2160p", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "1440p", "value": "7", "standard_keys": ["resolution.r1440p"]}, {"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}], "team": [{"label": "UBits", "value": "1"}, {"label": "UBWEB", "value": "6"}, {"label": "UBTV", "value": "7"}, {"label": "驻站", "value": "8"}, {"label": "Other", "value": "5"}], "tags": [{"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "原生原盘", "value": "17", "standard_keys": ["tag.original_disc"]}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "粤语", "value": "11", "standard_keys": ["tag.cantonese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "高分国剧", "value": "21"}, {"label": "特效字幕", "value": "12"}, {"label": "杜比视界", "value": "8", "standard_keys": ["tag.dolby_vision"]}, {"label": "HDR10+", "value": "10", "standard_keys": ["tag.hdr10_plus"]}, {"label": "菁彩HDR", "value": "18", "standard_keys": ["tag.hdr_vivid"]}, {"label": "HDR10", "value": "9", "standard_keys": ["tag.hdr10"]}, {"label": "连载", "value": "23", "standard_keys": ["tag.ongoing"]}, {"label": "合集", "value": "20", "standard_keys": ["tag.collection"]}, {"label": "HLG", "value": "19", "standard_keys": ["tag.hlg"]}, {"label": "自购", "value": "22"}, {"label": "菜单修改", "value": "14"}]}}`).Error
	})
}
