package rss

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/event"
	"github.com/ranfish/pt-forward/internal/filter"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"github.com/ranfish/pt-forward/internal/seeding"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupPipelineDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	models := []interface{}{
		&model.Site{},
		&model.RSSSubscription{},
		&model.RSSTorrentSeen{},
		&model.FilterRule{},
		&model.TorrentEvent{},
		&model.SeedingTorrentRecord{},
		&model.PublishCandidate{},
	}
	for _, m := range models {
		if err := db.AutoMigrate(m); err != nil {
			t.Fatalf("migrate %T: %v", m, err)
		}
	}
	return db
}

func TestPipelineIntegration_FilterReject(t *testing.T) {
	db := setupPipelineDB(t)
	logger := zap.NewNop()

	db.Create(&model.Site{Name: "testsite", Domain: "testsite.com"})

	db.Create(&model.FilterRule{
		Name:     "reject-adult",
		RuleType: "reject",
		Enabled:  true,
		Priority: 1,
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareContain, Value: "XXX"},
		},
	})

	seedingEngine := seeding.NewEngine(db, logger)
	if err := seedingEngine.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	publishPipeline := publish.NewPipeline(db, logger)

	dispatcher := event.NewDispatcher(logger)
	dispatcher.Register("rss_new", seedingEngine)
	dispatcher.Register("rss_new", publishPipeline)

	filterEngine := filter.NewEngine(filter.NewRepository(db), logger)

	evalCtx := &filter.EvalContext{
		Title:    "Some XXX Content",
		Size:     1000,
		SiteName: "testsite",
	}

	result, err := filterEngine.Match(context.Background(), evalCtx)
	if err != nil {
		t.Fatalf("match: %v", err)
	}

	if !result.Reject {
		t.Error("expected reject for XXX content")
	}

	evalCtx2 := &filter.EvalContext{
		Title:    "Ubuntu 24.04",
		Size:     1000,
		SiteName: "testsite",
	}
	result2, err := filterEngine.Match(context.Background(), evalCtx2)
	if err != nil {
		t.Fatalf("match2: %v", err)
	}
	if result2.Reject {
		t.Error("expected no reject for Ubuntu content")
	}
	if result2.Matched {
		t.Error("expected no match for Ubuntu content (no reject rule matched)")
	}
}
