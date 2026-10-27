package db

import (
	"gorm.io/gorm"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.359: 孤儿跨目录做种记录表——恢复链预查命中落表，扫描侧读表
// 为孤儿标"已在其它目录做种"提示。
func init() {
	RegisterMigration(52, "orphan_cross_seed_records", func(gormDB *gorm.DB) error {
		return gormDB.AutoMigrate(&model.OrphanCrossSeedRecord{})
	})
}
