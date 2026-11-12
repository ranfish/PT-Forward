package publish

import (
	"context"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// v0.0.1455 锚（Caddo Lake/Dead Man 双案）：补推成功后原记录收敛——
// uploaded 行→pushed+seeded；duplicate 行→仅 seeded（status 保持
// 「站上已有」事实）；已 seeded 的 duplicate 行幂等不动。
func TestRepushConverge(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.PublishResultRecord{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	seed := []model.PublishResultRecord{
		{TargetSite: "好大", TorrentID: "449228", Status: "duplicate", Seeded: false}, // Caddo 案形态（同调用 tid）
		{TargetSite: "好大", TorrentID: "449228", Status: "uploaded", Seeded: false}, // 漏种行形态
		{TargetSite: "好大", TorrentID: "449228", Status: "duplicate", Seeded: true}, // 已收敛行（幂等）
		{TargetSite: "瓷器", TorrentID: "449228", Status: "duplicate", Seeded: false}, // 异站同 tid 不动
	}
	for i := range seed {
		if err := db.Create(&seed[i]).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	ex := &PublishExecutor{db: db, logger: zap.NewNop()}
	ex.repushConverge(context.Background(), ExecuteInput{
		TargetSite: "好大", TorrentID: "449228",
	}, time.Now())

	var rows []model.PublishResultRecord
	db.Find(&rows)
	byID := map[uint]model.PublishResultRecord{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	// ① duplicate 行（tid=446950）：seeded 置位·status 保持 duplicate
	r1 := byID[seed[0].ID]  // tid=449228 与调用一致
	if r1.Status != "duplicate" || !r1.Seeded {
		t.Errorf("duplicate 行: status=%q seeded=%v want duplicate/true", r1.Status, r1.Seeded)
	}
	// ② uploaded 行（tid=449228）：status→pushed+seeded
	r2 := byID[seed[1].ID]
	if r2.Status != "pushed" || !r2.Seeded {
		t.Errorf("uploaded 行: status=%q seeded=%v want pushed/true", r2.Status, r2.Seeded)
	}
	// ③ 已 seeded duplicate 行：幂等不动（seeded_at 不刷新验证成本高·只验状态）
	r3 := byID[seed[2].ID]
	if r3.Status != "duplicate" || !r3.Seeded {
		t.Errorf("已收敛行被意外改动: %+v", r3)
	}
	// ④ 异站同 tid 行：不受影响
	r4 := byID[seed[3].ID]
	if r4.Status != "duplicate" || r4.Seeded {
		t.Errorf("异站行被误收敛: %+v", r4)
	}
}
