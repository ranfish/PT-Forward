package db

import (
	"gorm.io/gorm"
)

// migration141 §59.459 回归审核：野马 tags 三连缺口刷正——
// 首版 migration140 漏 form_fields tags 槽+漏 value_mappings tags 组
// （③定案的 tag 19 项映射表未落 migration）→ executor tags 走
// TagArrayFields 通道（Key 需 form_fields 映射）→ tagList 恒空标签全丢。
// DryRun 假阴性：借壳 Alice 无标签命中掩盖（tags: None 双解释）。
//
// 幂等：JSON_SET 条件刷正（tags 键不存在才写——用户手动改过不覆盖）。
func init() {
	RegisterMigration(141, "yema_tags_fix", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites SET
			publish_form_config = json_set(publish_form_config,
				'$.form_fields.tags', 'tagList',
				'$.value_mappings.tags', json('[{"label":"DIY","value":"4","standard_keys":["tag.diy"]},{"label":"国语","value":"5","standard_keys":["tag.chinese_audio"]},{"label":"中字","value":"6","standard_keys":["tag.chinese_subtitle"]},{"label":"粤语","value":"7","standard_keys":["tag.cantonese_audio"]},{"label":"英字","value":"8","standard_keys":["tag.english_subtitle"]},{"label":"HDR10","value":"9","standard_keys":["tag.hdr10"]},{"label":"杜比视界","value":"10","standard_keys":["tag.dolby_vision"]},{"label":"完结","value":"12","standard_keys":["tag.complete"]},{"label":"HDR10+","value":"14","standard_keys":["tag.hdr10_plus"]},{"label":"DTS-X","value":"16","standard_keys":["tag.dts_x"]},{"label":"完结全集","value":"18","standard_keys":["tag.collection"]},{"label":"多国字幕","value":"13"},{"label":"杜比全景声(Atmos)","value":"15"},{"label":"5.1/7.1声道","value":"17"},{"label":"SP/剧场版/OVA","value":"19"}]')),
			updated_at = datetime('now')
			WHERE domain = 'www.yemapt.org'
			AND json_extract(publish_form_config, '$.form_fields.tags') IS NULL`).Error
	})
}
