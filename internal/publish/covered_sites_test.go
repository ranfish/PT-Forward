package publish

import (
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/site"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestCoveredSitesOfCluster §59.535 刀5/6: 三面并集判据单点四态。
func TestCoveredSitesOfCluster(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.TorrentSnapshot{}, &model.SiteCoverageCache{},
		&model.Site{}, &model.PublishCandidate{}, &model.PublishResultRecord{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// 站点索引（tracker matcher 数据）
	db.Create(&model.Site{Name: "站T", Domain: "t.example.com", BaseURL: "https://t.example.com", Framework: "nexusphp"})
	tm := site.NewTrackerMatcher(db)

	const hA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const hB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" // 同簇第二副本

	// ① hash 不在快照：空集合（零跳过·发布链自有 dedup）
	if got := CoveredSitesOfCluster(nil, db, tm, "cccccccccccccccccccccccccccccccccccccccc"); len(got) != 0 {
		t.Errorf("① 快照 miss 应空: %v", got)
	}

	// ② 三面并集：tracker 面（hB 挂站T）∪覆盖面（hA 覆盖行站C）∪发布面（hA 发布史站P）
	db.Exec("INSERT INTO torrent_snapshots (hash,client_uid,save_path,name,tracker_domain,state,progress,is_hidden,last_seen,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,datetime('now'),datetime('now'),datetime('now'))",
		hA, 7, "/data/簇", "簇名种", "", "seeding", 1.0, 0)
	db.Exec("INSERT INTO torrent_snapshots (hash,client_uid,save_path,name,tracker_domain,state,progress,is_hidden,last_seen,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,datetime('now'),datetime('now'),datetime('now'))",
		hB, 7, "/data/簇", "簇名种", "t.example.com", "seeding", 1.0, 0)
	db.Exec("INSERT INTO site_coverage_cache (info_hash,site_name,status,source,confidence,queried_at,expires_at) VALUES (?,?,?,?,?,datetime('now'),datetime('now','+1 day'))",
		hA, "站C", model.CoverageConfirmedHas, "fetch", 1.0)
	db.Exec("INSERT INTO site_coverage_cache (info_hash,site_name,status,source,confidence,queried_at,expires_at) VALUES (?,?,?,?,?,datetime('now'),datetime('now','+1 day'))",
		hA, "站N", model.CoverageConfirmedNot, "pieces_hash", 0.95) // confirmed_not 不进
	cand := model.PublishCandidate{InfoHash: hA, TorrentName: "x"}
	db.Create(&cand)
	db.Create(&model.PublishResultRecord{CandidateID: cand.ID, TargetSite: "站P", Status: "pushed"})
	db.Create(&model.PublishResultRecord{CandidateID: cand.ID, TargetSite: "站F", Status: "failed"}) // failed 不进

	got := CoveredSitesOfCluster(nil, db, tm, hA)
	for _, want := range []string{"站T", "站C", "站P"} {
		if !got[want] {
			t.Errorf("② 应含 %s: %v", want, got)
		}
	}
	if got["站N"] || got["站F"] {
		t.Errorf("② confirmed_not/failed 不应进: %v", got)
	}
	if len(got) != 3 {
		t.Errorf("② 集合大小: %v", got)
	}

	// ③ HashCoveredBySite 轻量单对
	if !HashCoveredBySite(got, "站T") || HashCoveredBySite(got, "不存在站") {
		t.Errorf("③ 单对判定")
	}
	_ = time.Now()
	_ = zap.NewNop()
}
