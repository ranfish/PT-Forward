package db

import (
	"gorm.io/gorm"
)

// migration128 §59.445: 轨道炮（bilibili.download）发布表单配置分发。
// 依据：站方 upload.php?mode=advanced 实时 HTML（46.2KB 直连）+
// docs/32/railgunpt.md 533 行读全（B 站备份站特色+特殊区禁止）。
// **咖啡式简版第三例**（默认页 28KB 基础字段→mode=advanced 46.2KB
// 完整表单——咖啡/慕雪阁同款）；**资产 tags 槽缺口实时补全 7 项**
// （None→7）；**实时=资产=文档三源全槽一致**。
// 槽位映射：
//   - **双模式站**（mode=4 种子区 id=browsecat 14 项✓/mode=5 特殊区
//     name=type 413-418 六项成人⛔——§59.372 双 type 互斥双防线+
//     §59.388 [5] 槽模式过滤双防线自动护）；**420 下架视频备份**
//     （B 站备份站特色分类）+419 漫画 label-only（无键）+406 双键+
//     408 双键+404 双键。
//   - medium 9（倒序乱序）：Blu-ray(1)=[bluray]/UHD(2)=[uhd_bluray]
//     分立各单键+Remux(3)=[remux,uhd_remux] 双键归并（无 UHD Remux
//     项）+WEB-DL(4)=[webdl,webrip] 双键承接+**DVD(6)=[dvd,dvdr]
//     双键宽语义**（无 DVDR 项垃圾堆同款）+Encode/HDTV/CD/Track；
//     无 Other 项 miss 落空预览人工。
//   - codec 6 简短命名全配（H264/H265 无点+XVID 大写）——无 AV1
//     无 X26x 零联动。
//   - **audiocodec 10 合并项承接**：**TrueHD/Atmos(1)=[truehd,
//     atmos,truehd_atmos] 三键承接**（站方 TrueHD+Atmos 合并项——
//     历史双键模式扩展）+**DTS-HD/DTS-HDMA(2)=[dts,dts_hd_ma,
//     dts_hd_hr] 三键承接**（DTS 基础无独立项归并——车站家族式
//     承接无 dtsx）+**AC3(3)=[ac3,dd,ddp] 三键承接**（无 DDP 分立
//     ——大青虫先例）+LPCM/Flac/MP3/AAC/APE/Other/WAV。
//   - standard 6：4K(1)/1080p/i(2)=[r1080p,r1080i] 合并双键（咖啡
//     同款）/720p/SD/Other(5)=[resolution.other]/**2K(6)=[r1440p]
//     置尾乱序**。
//   - **无 team_sel（第三站）**（海胆 team_suffix/南洋无 team 后）。
//   - tags 7：禁转(1)/首发(2)/官方(3) 三绝不配（财神雷区）+DIY/
//     国语/中字/HDR 四键泛家族 四键配；无 DoVi/HDR10 细分。
//   - 字段：type/medium_sel[4]/codec_sel[4]/standard_sel[4]/
//     audiocodec_sel[4]/tags[4][]/small_descr/descr+url（IMDb）+
//     pt_gen+uplver+anonymous:true；**无 technical_info**（文档
//     §1.2 实证——MI 塞简介）；nfo 不配（13城先例）。
// 站规人工把关：特殊区 413-418 六项成人全局禁止（§30.3 用户指令）
//   +下架视频备份(420) 版权敏感性预览人工核对。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(128, "railgunpt_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'bilibili.download' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'bilibili.download'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "剧集", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "MV", "value": "406", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "音乐", "value": "408", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "misc", "value": "409", "standard_keys": ["category.other"]}, {"label": "软件", "value": "410", "standard_keys": ["category.software"]}, {"label": "学习", "value": "411", "standard_keys": ["category.education"]}, {"label": "游戏", "value": "412", "standard_keys": ["category.game"]}, {"label": "漫画", "value": "419"}, {"label": "下架视频备份", "value": "420"}], "medium": [{"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "UHD", "value": "2", "standard_keys": ["medium.uhd_bluray"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "WEB-DL", "value": "4", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "DVD", "value": "6", "standard_keys": ["medium.dvd", "medium.dvdr"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "Track", "value": "9", "standard_keys": ["medium.track"]}], "codec": [{"label": "H264", "value": "1", "standard_keys": ["video.h264"]}, {"label": "H265", "value": "2", "standard_keys": ["video.h265"]}, {"label": "VC-1", "value": "3", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "XVID", "value": "5", "standard_keys": ["video.xvid"]}, {"label": "Other", "value": "6", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "TrueHD/Atmos", "value": "1", "standard_keys": ["audio.truehd", "audio.atmos", "audio.truehd_atmos"]}, {"label": "DTS-HD/DTS-HDMA", "value": "2", "standard_keys": ["audio.dts", "audio.dts_hd_ma", "audio.dts_hd_hr"]}, {"label": "AC3", "value": "3", "standard_keys": ["audio.ac3", "audio.dd", "audio.ddp"]}, {"label": "LPCM", "value": "4", "standard_keys": ["audio.lpcm"]}, {"label": "Flac", "value": "5", "standard_keys": ["audio.flac"]}, {"label": "MP3", "value": "6", "standard_keys": ["audio.mp3"]}, {"label": "AAC", "value": "7", "standard_keys": ["audio.aac"]}, {"label": "APE", "value": "8", "standard_keys": ["audio.ape"]}, {"label": "Other", "value": "9", "standard_keys": ["audio.other"]}, {"label": "WAV", "value": "10", "standard_keys": ["audio.wav"]}], "standard": [{"label": "4K", "value": "1", "standard_keys": ["resolution.r2160p"]}, {"label": "1080p/i", "value": "2", "standard_keys": ["resolution.r1080p", "resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "Other", "value": "5", "standard_keys": ["resolution.other"]}, {"label": "2K", "value": "6", "standard_keys": ["resolution.r1440p"]}], "tags": [{"label": "禁转", "value": "1"}, {"label": "首发", "value": "2"}, {"label": "官方", "value": "3"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}]}}`).Error
	})
}
