package seeding

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/pusher"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestCreateRecordFromPush_MarksRSSTorrentSeenPushed 验证 §55.5 P1 修复：
// 推送成功后 rss_torrent_seen.status 应回写为 "pushed"，使下轮 RSS IsSeen 认它，避免重复投递。
func TestCreateRecordFromPush_MarksRSSTorrentSeenPushed(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(uniqueSQLiteDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.SeedingTorrentRecord{}, &model.RSSTorrentSeen{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	e := NewEngine(db, zap.NewNop())

	// 预置 rss_torrent_seen status="seen"（模拟 RSS 已发现、等待推送）
	if err := db.Create(&model.RSSTorrentSeen{
		SiteName:       "site1",
		TorrentID:      "t100",
		SubscriptionID: "1",
		Status:         "seen",
	}).Error; err != nil {
		t.Fatalf("create seen: %v", err)
	}

	// 调用 createRecordFromPush（模拟推送成功路径）
	e.createRecordFromPush(context.Background(), "c1", &pusher.PushedEvent{
		ClientID:  "c1",
		SiteName:  "site1",
		TorrentID: "t100",
		InfoHash:  "hash100",
		Size:      1024,
	}, "")

	// 验证 rss_torrent_seen.status 升级为 "pushed"
	var seen model.RSSTorrentSeen
	if err := db.Where("site_name = ? AND torrent_id = ?", "site1", "t100").First(&seen).Error; err != nil {
		t.Fatalf("query seen: %v", err)
	}
	if seen.Status != "pushed" {
		t.Errorf("expected rss_torrent_seen.status=pushed, got %q", seen.Status)
	}

	// 验证 seeding record 也已创建
	var rec model.SeedingTorrentRecord
	if err := db.Where("client_id = ? AND info_hash = ?", "c1", "hash100").First(&rec).Error; err != nil {
		t.Fatalf("query record: %v", err)
	}
	if rec.Status != model.SeedingStatusSeeding {
		t.Errorf("expected record status=seeding, got %s", rec.Status)
	}
}
