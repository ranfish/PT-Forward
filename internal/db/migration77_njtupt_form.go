package db

import (
	"gorm.io/gorm"
)

// migration77 §59.392: 浦园（njtupt.top）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（45.7KB 直连——与 sites.json 4 月
// 资产全槽零漂移）+ docs/32/njtupt.md 341 行读全。
// 槽位：type 12/medium 7/codec 6/standard 5/tags 8（无 audiocodec/
// team/source/processing）；字段 url（IMDb）+pt_gen（五来源）+
// technical_info+uplver 全齐（url×2 中第二个是编辑器按钮非字段）。
// 槽位特化：
//   - type 406 演出→category.concert（库非/咖啡同键第三站复用）；
//     404 纪录双键；415 资料 label-only（canonical 无 materials 类键）
//   - medium 7 项**值乱序**（9,8,5,7,6,2,10——财神同款乱序值用
//     实时 HTML）；Blu-ray(9) 归并 [bluray,uhd_bluray]+Remux(8)
//     归并 [remux,uhd_remux]（站方无独立 UHD 项）+WEB-DL(5) 承接
//     webrip；无 MiniBD/DIY 等特化项
//   - codec 6 项无 X26x——§59.380 压制联动零触发（站方语义）；
//     AV1(6) 配键
//   - tags 8 项：首发(2)/官方(3)（财神官种案同款防触雷）/自购(8)/
//     应求(11)/保种(10) label-only；禁转/国语/中字配键
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(77, "njtupt_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'njtupt.top' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'njtupt.top'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "standard": "standard_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "剧集", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "演出", "value": "406", "standard_keys": ["category.concert"]}, {"label": "音乐", "value": "422", "standard_keys": ["category.music"]}, {"label": "纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "资料", "value": "415"}, {"label": "游戏", "value": "425", "standard_keys": ["category.game"]}, {"label": "软件", "value": "428", "standard_keys": ["category.software"]}, {"label": "其他", "value": "409", "standard_keys": ["category.other"]}], "medium": [{"label": "Blu-ray", "value": "9", "standard_keys": ["medium.bluray", "medium.uhd_bluray"]}, {"label": "Remux", "value": "8", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "WEB-DL", "value": "5", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "HDTV", "value": "6", "standard_keys": ["medium.hdtv"]}, {"label": "DVD", "value": "2", "standard_keys": ["medium.dvdr", "medium.dvd"]}, {"label": "Other", "value": "10", "standard_keys": ["medium.other"]}], "codec": [{"label": "H.264/AVC", "value": "1", "standard_keys": ["video.h264"]}, {"label": "H.265/HEVC", "value": "3", "standard_keys": ["video.h265"]}, {"label": "AV1", "value": "6", "standard_keys": ["video.av1"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}], "standard": [{"label": "2160p", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}], "tags": [{"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "首发", "value": "2"}, {"label": "官方", "value": "3"}, {"label": "自购", "value": "8"}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "应求", "value": "11"}, {"label": "保种", "value": "10"}]}}`).Error
	})
}
