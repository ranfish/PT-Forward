package db

import (
	"gorm.io/gorm"
)

// migration142 §59.460: 可乐架构名称独立——framework mteam→cola（存量刷正）。
// 可乐是 reelcore 自研架构（三面分离：www Web 外壳+automation 上传面/
// api.colapt.com M-Team 兼容面），仅实现层复用 MTeamAdapter；此前骨架
// 收录时 framework 误用 mteam，前端站点详情页架构标签随之显示 M-Team
// （两家各自架构不应混名）。adapter 路由/RSS 策略/默认配置均已加 cola
// 分支，本 migration 刷存量行对齐。
//
// 幂等：条件 framework='mteam'（已是 cola 或用户改过则跳过）。
func init() {
	RegisterMigration(142, "cola_framework", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites SET
			framework = 'cola',
			updated_at = datetime('now')
			WHERE domain = 'www.colapt.com'
			AND framework = 'mteam'`).Error
	})
}
