package db

import "gorm.io/gorm"

// migration183 §59.572: 好学发布链关闸（等站方 API 完善）。
// 首颗实测（243·tid=23941）三根因：
//
//	① 终态误判——adapter 漏设 Success（v0.0.1618 刀1 已修）
//	② 元数据契约错位——multipart 表单字段被站方忽略·应走查询参数
//	   （v0.0.1618 刀2 已修）
//	③ 站方 open API 上传不注入 source 字段（web 路径注入·指纹库跨站对照
//	   实证）→ 站内种子与源站同 info_hash → 下载器去重（qB/TR 均不建
//	   第二任务）→ 无法做种。③是站方问题·我方无法修——关闸停发。
//
// 重启前置：站方 API 补 source 注入（与 web 路径对齐）→ 用户前端重开
// 目标站开关（publishSupported 仍 true·json 声明未撤——软关闸设计）。
func init() {
	RegisterMigration(183, "hxpt_publish_gate_off", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'www.hxpt.org'").Scan(&count)
		if count == 0 {
			return nil
		}
		if err := gormDB.Exec(`UPDATE sites SET is_target = 0, updated_at = datetime('now')
			WHERE domain = 'www.hxpt.org'`).Error; err != nil {
			return err
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = json_set(
				publish_form_config, '$.enabled', json('false'))
			WHERE domain = 'www.hxpt.org' AND publish_form_config IS NOT NULL AND publish_form_config != ''`).Error
	})
}
