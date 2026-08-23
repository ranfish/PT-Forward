package db

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"go.uber.org/zap"
	"gorm.io/gorm/logger"
)

// §59.61 管道层: migration 19 = snapshots 加 comment 列 + 簇复合索引
func TestMigration19_SnapshotComment(t *testing.T) {
	gormDB, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrate(gormDB); err != nil {
		t.Fatal(err)
	}
	_ = zap.NewNop()
	// comment 列可写可读
	row := model.TorrentSnapshot{Hash: "aabb", ClientID: "PT0", Name: "n", SavePath: "/x", Comment: "https://pt.keepfrds.com/details.php?id=1"}
	if err := gormDB.Create(&row).Error; err != nil {
		t.Fatalf("comment 列缺失或不可写: %v", err)
	}
	var got model.TorrentSnapshot
	if err := gormDB.Where("hash = ?", "aabb").First(&got).Error; err != nil || got.Comment != "https://pt.keepfrds.com/details.php?id=1" {
		t.Fatalf("comment 读取失败: %v %q", err, got.Comment)
	}
	// 复合索引存在
	var cnt int64
	gormDB.Table("sqlite_master").Where("type='index' AND name='idx_snapshots_cluster'").Count(&cnt)
	if cnt == 0 {
		t.Error("复合索引 idx_snapshots_cluster 未创建")
	}
}
