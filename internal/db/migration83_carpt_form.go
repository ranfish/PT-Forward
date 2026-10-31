package db

import (
	"gorm.io/gorm"
)

// migration83 §59.397: 车站（carpt.net）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（46.6KB 直连抓取）+ docs/32/carpt.md
// 522 行读全（含 rules.php 发种规则/dupe/打包试行）。
// 实时 HTML 与 sites.json 4 月资产全槽零漂移（第七个零漂移站）；
// 文档"audiocodec 11 个"为含占位 0 计数——实际 10 项（资产正确）。
// 单模式站（type 仅 1 个，无 [5] 槽）；NexusPHP 老版 XHTML 框架。
// 槽位特化：
//   - type 7 项极简：404 纪录双键+406 Music=category.music+407 Other
//     配键；无体育（归其他）/无短剧/无 MV
//   - medium 9 项**乱序**（1=Encode/7=Blu-ray/8=Blu-rayUHD/9=Remux
//     ——值不按质量排列文档权威警告）；WEB(2) 承接 [webdl,webrip]；
//     **Blu-ray(7) 与 Blu-rayUHD(8) 分立两键**（非归并——与大青虫
//     分立同款，与织梦/浦园归并形态相反）；Remux(9) 归并 uhd_remux；
//     DVDRip(4) 精确键（≠DVDR 原盘——站方无原盘项落 Other）
//   - codec 6 项**合并命名**（H.264/AVC/x264 一项含 CLI 词——label
//     非 x264 精确形态压制联动 lookupCLIVariant 不触发 ✓ 站方语义
//     本就是合并项）；无 AV1；Xvid 配键
//   - audiocodec 10 项：DTS-HD/DTS(2) 合并项承接 DTS 家族四键
//     [dts,dts_hd_ma,dts_hd_hr,dtsx]（大青虫 AC3 承接三键同款——
//     DTS:X 属 DTS-HD 家族演进）；AC3(3) 承接 [ac3,dd,ddp] 三键
//     （无 DDP 独立项）；mp3/wav label 照抄站方小写原文；无
//     DDP/TrueHD Atmos 独立
//   - standard 5 项极简：4K_UHD(1)/1080p/i(2) 双键/720p/i(3)
//     单键（r720i 无 canonical 键）/SD(4)=resolution.sd/
//     Other(5)=resolution.other
//   - team 5 项全 label-only：**站方表单含它站官组 WiKi/CMCT/
//     M-team**（team 域 EqualFold label 直配——它站官组种子自动
//     命中 ✓）；CarPT 站规"非管理组禁选"（label-only 转载永不选
//     ——仅种子真为 CarPT 组时命中）
//   - tags 6 项（全库最少之一）：首发 label-only 铁律+禁转/国语/
//     中字/DIY 配键+HDR label-only（惯例）；值 3 跳过（站方历史
//     遗留）；无完结/分集/特效/DV
//   - 字段：url（IMDb 真字段 data-pt-gen——url×2 第二个是编辑器
//     按钮，浦园/末日同款）+pt_gen+nfo+uplver；**无 technical_info**
//     （MediaInfo 写简介——文档实证）
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(83, "carpt_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'carpt.net' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'carpt.net'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影/Movies", "value": "401", "standard_keys": ["category.movie"]}, {"label": "连续剧/TV-Series", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "动漫/Animation", "value": "403", "standard_keys": ["category.animation"]}, {"label": "纪录片/Documentary", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "综艺/TV-Show", "value": "405", "standard_keys": ["category.tv_shows"]}, {"label": "音乐/Music", "value": "406", "standard_keys": ["category.music"]}, {"label": "其他/Other", "value": "407", "standard_keys": ["category.other"]}], "medium": [{"label": "Encode", "value": "1", "standard_keys": ["medium.encode"]}, {"label": "WEB", "value": "2", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "HDTV", "value": "3", "standard_keys": ["medium.hdtv"]}, {"label": "DVDRip", "value": "4", "standard_keys": ["medium.dvdrip"]}, {"label": "CD", "value": "5", "standard_keys": ["medium.cd"]}, {"label": "Other", "value": "6", "standard_keys": ["medium.other"]}, {"label": "Blu-ray", "value": "7", "standard_keys": ["medium.bluray"]}, {"label": "Blu-rayUHD", "value": "8", "standard_keys": ["medium.uhd_bluray"]}, {"label": "Remux", "value": "9", "standard_keys": ["medium.remux", "medium.uhd_remux"]}], "codec": [{"label": "H.264/AVC/x264", "value": "1", "standard_keys": ["video.h264"]}, {"label": "H.265/HEVC/x265", "value": "2", "standard_keys": ["video.h265"]}, {"label": "MPEG-2", "value": "3", "standard_keys": ["video.mpeg2"]}, {"label": "VC-1", "value": "4", "standard_keys": ["video.vc1"]}, {"label": "Xvid", "value": "5", "standard_keys": ["video.xvid"]}, {"label": "Other", "value": "6", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "TrueHD", "value": "1", "standard_keys": ["audio.truehd"]}, {"label": "DTS-HD/DTS", "value": "2", "standard_keys": ["audio.dts", "audio.dts_hd_ma", "audio.dts_hd_hr", "audio.dtsx"]}, {"label": "AC3", "value": "3", "standard_keys": ["audio.ac3", "audio.dd", "audio.ddp"]}, {"label": "LPCM", "value": "4", "standard_keys": ["audio.lpcm"]}, {"label": "FLAC", "value": "5", "standard_keys": ["audio.flac"]}, {"label": "mp3", "value": "6", "standard_keys": ["audio.mp3"]}, {"label": "AAC", "value": "7", "standard_keys": ["audio.aac"]}, {"label": "APE", "value": "8", "standard_keys": ["audio.ape"]}, {"label": "wav", "value": "10", "standard_keys": ["audio.wav"]}, {"label": "Other", "value": "9", "standard_keys": ["audio.other"]}], "standard": [{"label": "4K_UHD", "value": "1", "standard_keys": ["resolution.r2160p"]}, {"label": "1080p/i", "value": "2", "standard_keys": ["resolution.r1080p", "resolution.r1080i"]}, {"label": "720p/i", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "Other", "value": "5", "standard_keys": ["resolution.other"]}], "team": [{"label": "CarPT", "value": "1"}, {"label": "WiKi", "value": "2"}, {"label": "CMCT", "value": "3"}, {"label": "M-team", "value": "4"}, {"label": "Other", "value": "5"}], "tags": [{"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "首发", "value": "2"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR", "value": "7"}]}}`).Error
	})
}
