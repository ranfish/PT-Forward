package db

import (
	"gorm.io/gorm"
)

// migration115 §59.431: 奥申（www.oshen.win）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（40.6KB 直连）+ docs/32/oshen.md
// 355 行读全（rules.php 站规+Dupe/打包/促销/账号保留）。
// 实时 vs 4 月资产全槽一致（零漂移站）；**文档 §1.3"无 url"为漂移**
// （实时实证 url 真字段在——data-pt-gen="url"；实时为准配 imdb_url 域）。
// 槽位映射：
//   - type 10：**408 高清音乐/409 音乐分立各单键**（[lossless_music]/
//     [music]——站方无损/一般音乐分立，历史站合并项双键不适用）+
//     406 双键+404 双键+410=[game]+402 连续剧=[tv_series]。
//   - **medium 9 全配键零 label-only（大青虫后第二例媒介全 canonical）**：
//     倒序乱序（Track 9 首→Blu-ray 1 尾）+HD DVD/DVDR/MiniBD 全在——
//     Blu-ray=[bluray,uhd_bluray]/Remux=[remux,uhd_remux] 双键归并
//     （站方无 UHD 分立项——好大/浦园先例）+Track/CD/DVDR/HDTV/MiniBD/
//     Encode/HD DVD 各单键。
//   - codec 6 简短命名（藏宝阁同款）：H.264/H.265-HEVC（**值 10 非序号
//     连续——文档 §5.2 警示**）/VC-1/Xvid/MPEG-2/Other 全配键；无 X26x
//     CLI 形态（压制联动零触发=站方语义）。
//   - standard 5：4K/UHD 置尾乱序（5）+1080p/i 分立全配键。
//   - **无 audiocodec 槽第四站**（GTK/克隆/三月后——站方真无）。
//   - team 11：OshenPT 官组+HDS/CHD/MySiLU/WiKi 标准四组+CMCT/SCSG/
//     HDTIME/PTHOME/52pt 外站八组 label-only（组映射层自动命中）+
//     Other(5)=[team.other] 兜底。
//   - tags 7：**首发(2)/官方(3) 绝不配键（财神雷区）**+禁转(1) 不配+
//     DIY=[tag.diy]/国语=[tag.chinese_audio]/中字=[tag.chinese_subtitle]/
//     HDR 四键泛家族（16 站）。
//   - 字段：url（data-pt-gen 真字段——文档漂移实时仲裁）+uplver+
//     anonymous:true；无 pt_gen/technical_info（实时实证）；nfo 不配。
// 站规人工把关：Dupe 标准 HD 规则（媒介优先级 Blu-ray>HD DVD>HDTV>DVD
// +同媒介同分辨率一组优先）+游戏 5 题候选+RAR 禁+总量≥400MB（音乐
// 豁免）+促销自动 2xFree+吸血封号+H&R 无强制。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(115, "oshen_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'www.oshen.win' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'www.oshen.win'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "游戏/Game", "value": "410", "standard_keys": ["category.game"]}, {"label": "高清音乐", "value": "408", "standard_keys": ["category.lossless_music"]}, {"label": "音乐", "value": "409", "standard_keys": ["category.music"]}, {"label": "体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "MV(音乐视频)", "value": "406", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "综艺节目", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "连续剧", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "电影", "value": "401", "standard_keys": ["category.movie"]}], "medium": [{"label": "Track", "value": "9", "standard_keys": ["medium.track"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "DVDR", "value": "6", "standard_keys": ["medium.dvdr"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "MiniBD", "value": "4", "standard_keys": ["medium.minibd"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "HD DVD", "value": "2", "standard_keys": ["medium.hddvd"]}, {"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray", "medium.uhd_bluray"]}], "codec": [{"label": "H.264", "value": "1", "standard_keys": ["video.h264"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "Xvid", "value": "3", "standard_keys": ["video.xvid"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}, {"label": "H.265/HEVC", "value": "10", "standard_keys": ["video.h265"]}], "standard": [{"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "4K/UHD", "value": "5", "standard_keys": ["resolution.r2160p"]}], "team": [{"label": "SCSG", "value": "7"}, {"label": "CMCT", "value": "6"}, {"label": "PTHOME", "value": "9"}, {"label": "HDTIME", "value": "8"}, {"label": "OshenPT", "value": "10"}, {"label": "HDS", "value": "1"}, {"label": "CHD", "value": "2"}, {"label": "MySiLU", "value": "3"}, {"label": "WiKi", "value": "4"}, {"label": "Other", "value": "5", "standard_keys": ["team.other"]}, {"label": "52pt", "value": "11"}], "tags": [{"label": "禁转", "value": "1"}, {"label": "首发", "value": "2"}, {"label": "官方", "value": "3"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}]}}`).Error
	})
}
