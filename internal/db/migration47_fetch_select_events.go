package db

import (
	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/gorm"
)

// §59.331: 层选事件表——error 级日志环境的层选可观测。
func init() {
	RegisterMigration(47, "fetch_select_events", func(gormDB *gorm.DB) error {
		return gormDB.AutoMigrate(&model.FetchSelectEvent{})
	})
}
