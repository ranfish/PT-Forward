package api

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"github.com/ranfish/pt-forward/internal/setting"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// §59.516 v0.0.1461 锚：简介引用清洗任务——残留行净化（引用块整删·正文
// 保留）·干净行零触碰·幂等（二轮 changed=0）。
func TestDeclBackfillTask(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.TorrentMetadata{}, &setting.Setting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	f := publish.NewDeclarationFilter(setting.NewRepository(db), nil)
	h := &PublishTorrentsHandler{db: db, declFilter: f}
	rows := []model.TorrentMetadata{
		{InfoHash: "aa", Statement: "[quote][color=#ff0000]您的保种是(1PTBA)站长久发展的重要保证！自觉保种[/color][/quote]正文声明保留"},
		{InfoHash: "bb", Description: "[quote]郑重声明：本站提供的所有作品均是用户自行搜集并且上传[/quote]简介正文保留"},
		{InfoHash: "cc", Statement: "干净的声明"},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	ctx := context.Background()
	patterns := f.GetPatterns(ctx)
	if len(patterns) == 0 {
		t.Fatal("前置：生效词表为空")
	}
	h.declBackfill.total = len(rows)
	h.runDeclBackfillTask(rows, patterns)

	// ① 残留净化：quote 块剥除·正文保留
	var aa model.TorrentMetadata
	db.Where("info_hash = ?", "aa").First(&aa)
	if aa.Statement != "正文声明保留" {
		t.Errorf("aa statement=%q（应剥 quote 块留正文）", aa.Statement)
	}
	var bb model.TorrentMetadata
	db.Where("info_hash = ?", "bb").First(&bb)
	if bb.Description != "简介正文保留" {
		t.Errorf("bb description=%q（应剥 quote 块留正文）", bb.Description)
	}
	// ② 干净行零触碰
	var cc model.TorrentMetadata
	db.Where("info_hash = ?", "cc").First(&cc)
	if cc.Statement != "干净的声明" {
		t.Errorf("cc statement=%q（干净行不应被改）", cc.Statement)
	}
	// ③ 计数：changed=2·done=3·failed=0
	if h.declBackfill.changed != 2 || h.declBackfill.done != 3 || h.declBackfill.failed != 0 {
		t.Errorf("计数 changed=%d done=%d failed=%d（应 2/3/0）", h.declBackfill.changed, h.declBackfill.done, h.declBackfill.failed)
	}
	// ④ 幂等：二轮（重新查询——生产语义=任务启动时读库·已净化行零变化）
	var fresh []model.TorrentMetadata
	db.Where("info_hash IN ?", []string{"aa", "bb", "cc"}).Find(&fresh)
	h.declBackfill.changed = 0
	h.declBackfill.done = 0
	h.declBackfill.active = true
	h.declBackfill.finished = false
	h.declBackfill.total = len(fresh)
	h.runDeclBackfillTask(fresh, patterns)
	if h.declBackfill.changed != 0 || h.declBackfill.done != 3 {
		t.Errorf("幂等失败：changed=%d done=%d（应 0/3）", h.declBackfill.changed, h.declBackfill.done)
	}
}
