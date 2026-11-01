package db

import (
	"gorm.io/gorm"
)

// migration107 §59.422 附二: 匿名发布全系统默认开启存量刷正（用户定案）。
// 用户权威定案：发布匿名=全系统所有站点默认勾选开启（系统默认决策），
// 是否取消匿名由用户经站点详情页开关自主决策（§59.159 set-anonymous）。
//
// 与既有惯例一致性：36 个适配 migration 模板全部写 "anonymous": true
// ——本定案即把模板惯例升格为明文系统决策。
//
// 存量差异三站成因：
//   - 五二/财神：refresh migration（65/68 整配置重写）不带 anonymous
//     键→静默重置 false——§59.421 附 MergeDraftWithCurrent 布尔重置
//     同族第三/四案（refresh 重写链不携带布尔即重置）；源码已同步补。
//   - 麒麟：§59.422 附回归审核误移除（当时 grep 转义错误得出"历史
//     migration 无一写 anonymous"的错误结论——反引号原生串不匹配
//     \"anonymous\" 转义形态；DB 33/36 站已是 true 的现实证伪）。
//
// 无 uplver 字段站（慕雪阁/星陨阁 anon=true、冬樱 false）不动：
// executor 消费点 form_fields["uplver"] 不存在不注入——零行为差异。
//
// 防线：TestFreshInstallChain 断言"publish_supported+uplver 站
// anonymous 默认 true"——未来新站漏配/refresh 再丢键即 CI 红。
// 幂等：三站内容即终态无条件 UPDATE。
func init() {
	RegisterMigration(107, "anonymous_default_true", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites
			SET publish_form_config = json_set(publish_form_config, '$.anonymous', json('true')),
			    updated_at = datetime('now')
			WHERE domain IN ('52pt.site', 'cspt.top', 'www.hdkyl.in')`).Error
	})
}
