package db

import (
	"gorm.io/gorm"
)

// migration179 §59.541 附十一: 劳改所 form_fields 补 cover+screenshots 两域。
//
// 根因：migration121 适配期注释写明「+cover+screenshots+technical_info
// （必填三件）」但 SQL 的 form_fields 实际只写了 technical_info——
// 实施遗漏（注释与落地不一致·§59.1512 空键族：form_fields 缺域=文本域
// 断链）。生产形态：截图 5 张全部内嵌 descr（站方审核「其它信息不能带
// 图」——用户手工逐颗补救）+cover 框靠站方 pt_gen 联动自动填。
//
// 修复配套（代码侧）：
//   - executor setForm(FieldDomainScreenshot, URL 一行一张)
//   - assembleDescription 双分流：配 screenshots 域→descr 不内嵌截图；
//     配 cover 域→descr 剥简介首行海报 [img]（走独立框·descr 无图形态）
//
// 幂等：json_set 精确补两键·已配环境覆盖为同值无害。
func init() {
	RegisterMigration(179, "ptlgs_cover_screenshots_fields", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites SET publish_form_config = json_set(
			publish_form_config,
			'$.form_fields.screenshots', 'screenshots',
			'$.form_fields.cover', 'cover'
		), updated_at = datetime('now') WHERE domain = 'ptlgs.org'`).Error
	})
}
