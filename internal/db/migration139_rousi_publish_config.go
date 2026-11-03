package db

import (
	"gorm.io/gorm"
)

// migration139 §59.458: 肉丝目标站适配开闸——publish_form_config+is_target。
//
// 架构（API JSON 第二站·馒头模式扩展 §59.458-①）：
//   - form_fields 域→JSON 字段名（type→category/standard→resolution/
//     medium→medium（值=④矩阵语义值 adapter 分发 source/release-type）/
//     small_descr→subtitle/description→description/technical_info→media_info）
//   - genre/region/poster 非标准域——executor 特化注入 ExtraFields
//     （rs_genre/rs_region/rs_poster——§59.458-②③）
//   - imdb/douban 顶层 URL 直投（⑥——站方双形态任一）
//   - 无 codec/audiocodec/team/tags 槽（站方无此维度——§59.458-①）
//
// value_mappings 要点：
//   - type 七类（六影视+music；software/ebook miss 预览人工；
//     9kg 永不配——⑩双保险）
//   - documentary 单复数双键（§59.376 铁律）
//   - medium encode 族 value=blu-ray（④方案 A 来源跟随——与原盘行
//     同 value 两行）；remux 族 value=remux（rt 集合流）
//   - standard 8K→other（§59.458 边界定案 A 宁模糊勿虚标）+
//     r480p/r576p→sd 归并
//
// 幂等：条件写（is_target=0 或配置空才 UPDATE——用户手动改过不覆盖）。
func init() {
	RegisterMigration(139, "rousi_publish_config", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites SET is_target = 1, publish_form_config = ?,
			updated_at = datetime('now')
			WHERE domain = 'rousi.pro' AND (is_target = 0 OR publish_form_config IS NULL OR publish_form_config = '')`,
			`{"enabled": true, "anonymous": true, "framework": "rousi", "form_fields": {"type": "category", "medium": "medium", "standard": "resolution", "small_descr": "subtitle", "description": "description", "technical_info": "media_info"}, "value_mappings": {"type": [{"label": "电影", "value": "movie", "standard_keys": ["category.movie"]}, {"label": "电视剧", "value": "tv", "standard_keys": ["category.tv_series"]}, {"label": "纪录片", "value": "documentary", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "动漫", "value": "animation", "standard_keys": ["category.animation"]}, {"label": "综艺", "value": "variety", "standard_keys": ["category.tv_shows"]}, {"label": "体育", "value": "sports", "standard_keys": ["category.sports"]}, {"label": "音乐", "value": "music", "standard_keys": ["category.music", "category.lossless_music"]}], "standard": [{"label": "4K / 2160p", "value": "2160p", "standard_keys": ["resolution.r2160p"]}, {"label": "1080p", "value": "1080p", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "1080i", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "720p", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "sd", "standard_keys": ["resolution.sd", "resolution.r480p", "resolution.r576p"]}, {"label": "其它(含 8K)", "value": "other", "standard_keys": ["resolution.r4320p", "resolution.other"]}], "medium": [{"label": "Blu-ray", "value": "blu-ray", "standard_keys": ["medium.bluray"]}, {"label": "UHD Blu-ray", "value": "uhd-blu-ray", "standard_keys": ["medium.uhd_bluray"]}, {"label": "Remux", "value": "remux", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "WEB-DL", "value": "web-dl", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "HDTV", "value": "hdtv", "standard_keys": ["medium.hdtv", "medium.uhdtv"]}, {"label": "DVDRip", "value": "dvdrip", "standard_keys": ["medium.dvdrip"]}, {"label": "Encode(BD 来源)", "value": "blu-ray", "standard_keys": ["medium.encode", "medium.bdrip", "medium.minibd"]}, {"label": "DVD/其它碟源", "value": "other", "standard_keys": ["medium.dvd", "medium.dvdr", "medium.hddvd", "medium.other"]}]}}`).Error
	})
}
