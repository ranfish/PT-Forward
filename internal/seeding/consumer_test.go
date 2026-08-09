package seeding

import (
	"context"
	"fmt"
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
		ClientID:       "c1",
		SiteName:       "site1",
		TorrentID:      "t100",
		SubscriptionID: "1",
		InfoHash:       "hash100",
		Size:           1024,
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

// TestLoadScoringConfig_ReadsEmbeddedScoringConfig 验证 §55.10 修复：
// loadScoringConfig 应从 RSSSubscription.ScoringConfig（embedded，存于 rss_subscriptions 表）读取，
// 而非从不存在的 seeding_scoring_configs 独立表读。否则用户启用的评分永远读不到（Enabled=false）。
func TestLoadScoringConfig_ReadsEmbeddedScoringConfig(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(uniqueSQLiteDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.RSSSubscription{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	e := NewEngine(db, zap.NewNop())

	// 创建订阅，评分启用 + 自定义参数（模拟前端 PUT scoring-config 保存的结果）
	sub := &model.RSSSubscription{
		Name:     "test-scoring-sub",
		Enabled:  true,
		SiteName: "site1",
		ClientID: "c1",
		ScoringConfig: model.SeedingScoringConfig{
			Enabled:       true,
			HalfLifeHours: 3.5,
			MinScore:      0.5,
		},
	}
	if err := db.Create(sub).Error; err != nil {
		t.Fatalf("create sub: %v", err)
	}

	cfg := e.loadScoringConfig(context.Background(), fmt.Sprintf("%d", sub.ID))
	if !cfg.Enabled {
		t.Errorf("expected Enabled=true（评分已启用但引擎读不到，配置脱节 bug 回归），got false")
	}
	if cfg.HalfLifeHours != 3.5 {
		t.Errorf("expected HalfLifeHours=3.5, got %v", cfg.HalfLifeHours)
	}
	if cfg.MinScore != 0.5 {
		t.Errorf("expected MinScore=0.5, got %v", cfg.MinScore)
	}
}

func setupConfigTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(uniqueSQLiteDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.SeedingClientConfig{}, &model.DownloadClientConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// TestLoadActiveClientConfig 验证统一表配置加载。
// 合并后 download_client_configs 数据已迁入 seeding_client_configs（Role=download）。
func TestLoadActiveClientConfig(t *testing.T) {
	t.Run("seeding config hit", func(t *testing.T) {
		db := setupConfigTestDB(t)
		db.Create(&model.SeedingClientConfig{ClientID: "c1", Enabled: true, Role: "seeding", MaxActiveSeeding: 7, MinDiskSpaceGB: 20})
		e := NewEngine(db, zap.NewNop())
		cfg, ok := e.LoadActiveClientConfig(context.Background(), "c1")
		if !ok {
			t.Fatal("expected ok=true for seeding config")
		}
		if cfg.ClientID != "c1" || cfg.MaxActiveSeeding != 7 || cfg.MinDiskSpaceGB != 20 {
			t.Errorf("unexpected seeding cfg: client=%s max=%d min=%v",
				cfg.ClientID, cfg.MaxActiveSeeding, cfg.MinDiskSpaceGB)
		}
	})

	t.Run("download role config hit", func(t *testing.T) {
		db := setupConfigTestDB(t)
		db.Create(&model.SeedingClientConfig{
			ClientID:           "qb-dl",
			Enabled:            true,
			Role:               "download",
			MaxActiveSeeding:   5,
			MinDiskSpaceGB:     30,
			DiskProtectEnabled: true,
		})
		e := NewEngine(db, zap.NewNop())
		cfg, ok := e.LoadActiveClientConfig(context.Background(), "qb-dl")
		if !ok {
			t.Fatal("expected ok=true for download role config")
		}
		if cfg.MaxActiveSeeding != 5 {
			t.Errorf("expected MaxActiveSeeding=5, got %d", cfg.MaxActiveSeeding)
		}
		if cfg.MinDiskSpaceGB != 30 || !cfg.DiskProtectEnabled {
			t.Errorf("disk protect fields not mapped: min=%v protect=%v",
				cfg.MinDiskSpaceGB, cfg.DiskProtectEnabled)
		}
	})

	t.Run("neither config found", func(t *testing.T) {
		db := setupConfigTestDB(t)
		e := NewEngine(db, zap.NewNop())
		_, ok := e.LoadActiveClientConfig(context.Background(), "ghost")
		if ok {
			t.Fatal("expected ok=false when no config exists")
		}
	})

	t.Run("disabled config skipped", func(t *testing.T) {
		db := setupConfigTestDB(t)
		// 用 map 创建绕过 gorm default:true tag 对 bool 零值的覆盖
		db.Model(&model.SeedingClientConfig{}).Create(map[string]interface{}{"client_id": "c2", "enabled": false})
		db.Model(&model.DownloadClientConfig{}).Create(map[string]interface{}{"client_id": "c2", "enabled": false})
		e := NewEngine(db, zap.NewNop())
		_, ok := e.LoadActiveClientConfig(context.Background(), "c2")
		if ok {
			t.Fatal("expected ok=false for disabled configs")
		}
	})
}
