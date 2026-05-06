package seeding

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupRuleEvalTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.DeleteRule{},
		&model.SeedingTorrentRecord{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestRuleEvaluator_EvaluateRules_NoRules(t *testing.T) {
	db := setupRuleEvalTestDB(t)
	re := NewRuleEvaluator(db, zap.NewNop())

	matches, err := re.EvaluateRulesSimple(context.Background(), "c1")
	if err != nil {
		t.Fatal(err)
	}
	if matches != nil {
		t.Errorf("expected nil, got %v", matches)
	}
}

func TestRuleEvaluator_EvaluateRules_NoRecords(t *testing.T) {
	db := setupRuleEvalTestDB(t)
	ctx := context.Background()

	db.Create(&model.DeleteRule{
		Alias:      "r1",
		Priority:   10,
		Enabled:    true,
		Conditions: `[{"field":"site_name","operator":"equals","value":"site1"}]`,
		DeleteNum:  1,
	})

	re := NewRuleEvaluator(db, zap.NewNop())
	matches, err := re.EvaluateRulesSimple(ctx, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if matches != nil {
		t.Errorf("expected nil, got %v", matches)
	}
}

func TestRuleEvaluator_EvaluateRules_SiteMatch(t *testing.T) {
	db := setupRuleEvalTestDB(t)
	ctx := context.Background()

	db.Create(&model.DeleteRule{
		Alias:      "match-site",
		Priority:   10,
		Enabled:    true,
		Conditions: `[{"field":"site_name","operator":"equals","value":"site1"}]`,
		DeleteNum:  0,
	})
	db.Create(&model.SeedingTorrentRecord{
		ClientID:  "c1",
		InfoHash:  "h1",
		SiteName:  "site1",
		TorrentID: "1",
		Status:    model.SeedingStatusSeeding,
	})
	db.Create(&model.SeedingTorrentRecord{
		ClientID:  "c1",
		InfoHash:  "h2",
		SiteName:  "site2",
		TorrentID: "2",
		Status:    model.SeedingStatusSeeding,
	})

	re := NewRuleEvaluator(db, zap.NewNop())
	matches, err := re.EvaluateRulesSimple(ctx, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if len(matches[0].Records) != 1 {
		t.Errorf("expected 1 record, got %d", len(matches[0].Records))
	}
	if matches[0].Records[0].SiteName != "site1" {
		t.Errorf("expected site1, got %s", matches[0].Records[0].SiteName)
	}
}

func TestRuleEvaluator_EvaluateRules_DeleteNum(t *testing.T) {
	db := setupRuleEvalTestDB(t)
	ctx := context.Background()

	db.Create(&model.DeleteRule{
		Alias:      "limit",
		Priority:   10,
		Enabled:    true,
		Conditions: `[{"field":"site_name","operator":"equals","value":"site1"}]`,
		DeleteNum:  1,
	})
	for i := 0; i < 3; i++ {
		db.Create(&model.SeedingTorrentRecord{
			ClientID:  "c1",
			InfoHash:  fmt.Sprintf("h%d", i),
			SiteName:  "site1",
			TorrentID: fmt.Sprintf("%d", i),
			Status:    model.SeedingStatusSeeding,
		})
	}

	re := NewRuleEvaluator(db, zap.NewNop())
	matches, err := re.EvaluateRulesSimple(ctx, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if len(matches[0].Records) != 1 {
		t.Errorf("expected 1 record (DeleteNum=1), got %d", len(matches[0].Records))
	}
}

func TestRuleEvaluator_EvaluateRules_PriorityOrder(t *testing.T) {
	db := setupRuleEvalTestDB(t)
	ctx := context.Background()

	db.Create(&model.DeleteRule{
		Alias:      "low",
		Priority:   1,
		Enabled:    true,
		Conditions: `[{"field":"site_name","operator":"contains","value":"site"}]`,
		DeleteNum:  0,
	})
	db.Create(&model.DeleteRule{
		Alias:      "high",
		Priority:   10,
		Enabled:    true,
		Conditions: `[{"field":"site_name","operator":"contains","value":"site"}]`,
		DeleteNum:  0,
	})
	db.Create(&model.SeedingTorrentRecord{
		ClientID:  "c1",
		InfoHash:  "h1",
		SiteName:  "site1",
		TorrentID: "1",
		Status:    model.SeedingStatusSeeding,
	})

	re := NewRuleEvaluator(db, zap.NewNop())
	matches, err := re.EvaluateRulesSimple(ctx, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if matches[0].Priority < matches[1].Priority {
		t.Error("expected DESC priority order")
	}
}

func TestRuleEvaluator_EvaluateRules_EmptyConditions(t *testing.T) {
	db := setupRuleEvalTestDB(t)
	ctx := context.Background()

	db.Create(&model.DeleteRule{
		Alias:     "empty",
		Priority:  10,
		Enabled:   true,
		DeleteNum: 0,
	})
	db.Create(&model.SeedingTorrentRecord{
		ClientID:  "c1",
		InfoHash:  "h1",
		SiteName:  "site1",
		TorrentID: "1",
		Status:    model.SeedingStatusSeeding,
	})

	re := NewRuleEvaluator(db, zap.NewNop())
	matches, err := re.EvaluateRulesSimple(ctx, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for empty conditions, got %d", len(matches))
	}
}

func TestParseConditions(t *testing.T) {
	valid := `[{"field":"site_name","operator":"equals","value":"s1"}]`
	conds := ParseConditions(valid)
	if len(conds) != 1 {
		t.Fatalf("expected 1, got %d", len(conds))
	}
	if conds[0].Field != "site_name" {
		t.Errorf("expected site_name, got %s", conds[0].Field)
	}

	conds = ParseConditions("")
	if conds != nil {
		t.Errorf("expected nil for empty, got %v", conds)
	}

	conds = ParseConditions("not-json")
	if conds != nil {
		t.Errorf("expected nil for invalid json, got %v", conds)
	}
}

func TestEvalCondition_AllOperators(t *testing.T) {
	rec := model.SeedingTorrentRecord{
		SiteName: "site1",
		IsFree:   true,
		HasHR:    false,
		Discount: model.DiscountFree,
	}

	tests := []struct {
		name  string
		cond  ruleCondition
		match bool
	}{
		{"equals match", ruleCondition{Field: "site_name", Operator: "equals", Value: "site1"}, true},
		{"equals no match", ruleCondition{Field: "site_name", Operator: "equals", Value: "site2"}, false},
		{"not_equals match", ruleCondition{Field: "site_name", Operator: "not_equals", Value: "site2"}, true},
		{"not_equals no match", ruleCondition{Field: "site_name", Operator: "not_equals", Value: "site1"}, false},
		{"contains match", ruleCondition{Field: "site_name", Operator: "contains", Value: "ite"}, true},
		{"contains no match", ruleCondition{Field: "site_name", Operator: "contains", Value: "xyz"}, false},
		{"not_contains match", ruleCondition{Field: "site_name", Operator: "not_contains", Value: "xyz"}, true},
		{"not_contains no match", ruleCondition{Field: "site_name", Operator: "not_contains", Value: "ite"}, false},
		{"bool equals", ruleCondition{Field: "is_free", Operator: "equals", Value: "true"}, true},
		{"discount equals", ruleCondition{Field: "discount", Operator: "equals", Value: "FREE"}, true},
		{"unknown field", ruleCondition{Field: "unknown", Operator: "equals", Value: "x"}, true},
		{"unknown operator", ruleCondition{Field: "site_name", Operator: "regex", Value: "site1"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := &RuleContext{Record: &rec}
			got := evalCondition(rc, tt.cond)
			if got != tt.match {
				t.Errorf("got %v, want %v", got, tt.match)
			}
		})
	}
}

func TestEvalCondition_NumericOperators(t *testing.T) {
	ti := model.TorrentInfo{
		Hash:        "abc",
		Ratio:       2.5,
		Uploaded:    1024 * 1024 * 1024,
		UploadSpeed: 1024000,
		TotalSize:   5 * 1024 * 1024 * 1024,
		SeedTime:    86400,
		NumComplete: 42,
	}
	rec := model.SeedingTorrentRecord{
		ClientID:    "c1",
		InfoHash:    "abc",
		SiteName:    "site1",
		TorrentID:   "1",
		Status:      model.SeedingStatusSeeding,
		HasHR:       true,
		HRSeedTimeH: 72,
		IsFree:      true,
	}
	freeEnd := time.Now().Add(3600 * time.Second)
	rec.FreeEndAt = &freeEnd

	rc := &RuleContext{Record: &rec, Torrent: &ti, FreeSpace: 10 * 1024 * 1024 * 1024, Now: time.Now()}

	tests := []struct {
		name  string
		cond  ruleCondition
		match bool
	}{
		{"ratio bigger", ruleCondition{Field: "ratio", Operator: "bigger", Value: "2.0"}, true},
		{"ratio smaller", ruleCondition{Field: "ratio", Operator: "smaller", Value: "3.0"}, true},
		{"ratio bigger fail", ruleCondition{Field: "ratio", Operator: "bigger", Value: "3.0"}, false},
		{"seeder bigger", ruleCondition{Field: "seeder", Operator: "bigger", Value: "40"}, true},
		{"seeder smaller", ruleCondition{Field: "seeder", Operator: "smaller", Value: "50"}, true},
		{"freeSpace bigger 1GB", ruleCondition{Field: "freeSpace", Operator: "bigger", Value: "1073741824"}, true},
		{"uploadSpeed smaller 2MB", ruleCondition{Field: "uploadSpeed", Operator: "smaller", Value: "2048000"}, true},
		{"hrRemainSec bigger 0", ruleCondition{Field: "hrRemainSec", Operator: "bigger", Value: "0"}, true},
		{"freeRemainSec bigger 0", ruleCondition{Field: "freeRemainSec", Operator: "bigger", Value: "0"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evalCondition(rc, tt.cond)
			if got != tt.match {
				t.Errorf("evalCondition(%+v) = %v, want %v", tt.cond, got, tt.match)
			}
		})
	}
}

func TestEvalCondition_IncludeIn(t *testing.T) {
	ti := model.TorrentInfo{Hash: "abc", State: "uploading", Tags: []string{"pt-forward", "test"}}
	rec := model.SeedingTorrentRecord{InfoHash: "abc", SiteName: "site1"}

	rc := &RuleContext{Record: &rec, Torrent: &ti, Now: time.Now()}

	tests := []struct {
		name  string
		cond  ruleCondition
		match bool
	}{
		{"includeIn match", ruleCondition{Field: "state", Operator: "includeIn", Value: "uploading,stalledUP"}, true},
		{"includeIn no match", ruleCondition{Field: "state", Operator: "includeIn", Value: "stalledUP,paused"}, false},
		{"notIncludeIn match", ruleCondition{Field: "state", Operator: "notIncludeIn", Value: "paused,stalledDL"}, true},
		{"notIncludeIn no match", ruleCondition{Field: "state", Operator: "notIncludeIn", Value: "uploading,paused"}, false},
		{"site includeIn", ruleCondition{Field: "site_name", Operator: "includeIn", Value: "site1,site2,site3"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evalCondition(rc, tt.cond)
			if got != tt.match {
				t.Errorf("got %v, want %v", got, tt.match)
			}
		})
	}
}

func TestEvalCondition_RegExp(t *testing.T) {
	ti := model.TorrentInfo{Hash: "abc", Name: "[PT-Forward] Some.Movie.2024.1080p.BluRay"}
	rec := model.SeedingTorrentRecord{InfoHash: "abc", SiteName: "site1"}
	rc := &RuleContext{Record: &rec, Torrent: &ti, Now: time.Now()}

	tests := []struct {
		name  string
		cond  ruleCondition
		match bool
	}{
		{"regExp match brackets", ruleCondition{Field: "name", Operator: "regExp", Value: `^\[.*\]`}, true},
		{"regExp no match", ruleCondition{Field: "name", Operator: "regExp", Value: `^m-team`}, false},
		{"notRegExp match", ruleCondition{Field: "name", Operator: "notRegExp", Value: `^m-team`}, true},
		{"notRegExp no match", ruleCondition{Field: "name", Operator: "notRegExp", Value: `^\[.*\]`}, false},
		{"regExp site", ruleCondition{Field: "site_name", Operator: "regExp", Value: `^site\d+$`}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evalCondition(rc, tt.cond)
			if got != tt.match {
				t.Errorf("got %v, want %v", got, tt.match)
			}
		})
	}
}

func TestEvalCondition_TorrentFields(t *testing.T) {
	now := time.Now()
	ti := model.TorrentInfo{
		Hash:        "abc",
		Name:        "test-movie",
		TotalSize:   1024,
		Progress:    0.95,
		State:       "uploading",
		Uploaded:    2048,
		UploadSpeed: 100,
		Category:    "movies",
		Tags:        []string{"pt-forward"},
		SavePath:    "/data/torrents",
		NumComplete: 10,
		AddedAt:     now.Add(-24 * time.Hour),
	}
	rec := model.SeedingTorrentRecord{InfoHash: "abc"}
	rc := &RuleContext{Record: &rec, Torrent: &ti, Now: now}

	keyTests := []struct {
		field string
		want  string
	}{
		{"name", "test-movie"},
		{"state", "uploading"},
		{"category", "movies"},
		{"tags", "pt-forward"},
		{"savePath", "/data/torrents"},
		{"seeder", "10"},
		{"hour", fmt.Sprintf("%d", now.Hour())},
	}

	for _, tt := range keyTests {
		t.Run(tt.field, func(t *testing.T) {
			got, known := rc.fieldValue(tt.field)
			if !known {
				t.Errorf("field %s should be known", tt.field)
			}
			if got != tt.want {
				t.Errorf("field %s = %q, want %q", tt.field, got, tt.want)
			}
		})
	}
}

func TestEvalCondition_NoTorrent(t *testing.T) {
	rec := model.SeedingTorrentRecord{SiteName: "test", InfoHash: "abc"}
	rc := &RuleContext{Record: &rec, Torrent: nil, Now: time.Now()}

	got := evalCondition(rc, ruleCondition{Field: "site_name", Operator: "equals", Value: "test"})
	if !got {
		t.Error("should match site_name without torrent")
	}

	got = evalCondition(rc, ruleCondition{Field: "name", Operator: "equals", Value: "test"})
	if !got {
		t.Error("unknown torrent field without torrent should pass")
	}
}

func TestExprEngine_RatioAndTime(t *testing.T) {
	db := setupRuleEvalTestDB(t)
	ctx := context.Background()

	db.Create(&model.DeleteRule{
		Alias: "expr-ratio-time", Type: "expr", Priority: 10, Enabled: true,
		Expr: "ratio > 2.0 && addedTime > 7*24*3600",
	})

	rec := model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "h1", SiteName: "site1",
		Status: model.SeedingStatusSeeding, Discount: model.DiscountNone,
	}
	rec.CreatedAt = time.Now().Add(-8 * 24 * time.Hour)
	db.Create(&rec)

	ti := &model.TorrentInfo{
		Hash: "h1", Name: "test", Ratio: 2.5,
		AddedAt: time.Now().Add(-8 * 24 * time.Hour),
	}
	torrentMap := map[string]*model.TorrentInfo{"h1": ti}

	re := NewRuleEvaluator(db, zap.NewNop())
	matches, err := re.EvaluateRules(ctx, "c1", torrentMap, -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].Rule.Alias != "expr-ratio-time" {
		t.Errorf("wrong rule matched: %s", matches[0].Rule.Alias)
	}
}

func TestExprEngine_FreeSpaceCheck(t *testing.T) {
	db := setupRuleEvalTestDB(t)
	ctx := context.Background()

	db.Create(&model.DeleteRule{
		Alias: "expr-space", Type: "expr", Priority: 10, Enabled: true,
		Expr: "freeSpace < 50*1024*1024*1024 && seedingTime > 48*3600",
	})

	rec := model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "h1", SiteName: "site1",
		Status: model.SeedingStatusSeeding, Discount: model.DiscountNone,
	}
	db.Create(&rec)

	ti := &model.TorrentInfo{
		Hash: "h1", Name: "test", SeedTime: 50 * 3600,
	}
	torrentMap := map[string]*model.TorrentInfo{"h1": ti}

	re := NewRuleEvaluator(db, zap.NewNop())
	matches, err := re.EvaluateRules(ctx, "c1", torrentMap, 10*1024*1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
}

func TestExprEngine_SiteAndFreeAndHR(t *testing.T) {
	db := setupRuleEvalTestDB(t)
	ctx := context.Background()

	db.Create(&model.DeleteRule{
		Alias: "expr-site-hr", Type: "expr", Priority: 10, Enabled: true,
		Expr: `siteName == "mteam" && isFree && hrRemainSec > 0`,
	})

	freeEnd := time.Now().Add(1 * time.Hour)
	rec := model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "h1", SiteName: "mteam",
		Status: model.SeedingStatusSeeding, Discount: model.DiscountFree,
		IsFree: true, HasHR: true, HRSeedTimeH: 72, FreeEndAt: &freeEnd,
	}
	db.Create(&rec)

	ti := &model.TorrentInfo{Hash: "h1", Name: "test", SeedTime: 1000}
	torrentMap := map[string]*model.TorrentInfo{"h1": ti}

	re := NewRuleEvaluator(db, zap.NewNop())
	matches, err := re.EvaluateRules(ctx, "c1", torrentMap, -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
}

func TestExprEngine_NoMatch(t *testing.T) {
	db := setupRuleEvalTestDB(t)
	ctx := context.Background()

	db.Create(&model.DeleteRule{
		Alias: "expr-nomatch", Type: "expr", Priority: 10, Enabled: true,
		Expr: "ratio > 5.0 && uploadSpeed == 0",
	})

	rec := model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "h1", SiteName: "site1",
		Status: model.SeedingStatusSeeding, Discount: model.DiscountNone,
	}
	db.Create(&rec)

	ti := &model.TorrentInfo{Hash: "h1", Name: "test", Ratio: 1.5, UploadSpeed: 1000}
	torrentMap := map[string]*model.TorrentInfo{"h1": ti}

	re := NewRuleEvaluator(db, zap.NewNop())
	matches, err := re.EvaluateRules(ctx, "c1", torrentMap, -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(matches))
	}
}

func TestExprEngine_CompileError(t *testing.T) {
	err := ValidateExpr("invalid [syntax")
	if err == nil {
		t.Error("expected compile error for invalid expr")
	}
}

func TestExprEngine_ValidCompile(t *testing.T) {
	err := ValidateExpr("ratio > 2.0 && siteName == 'test'")
	if err != nil {
		t.Errorf("valid expr should compile: %v", err)
	}
}

func TestExprEngine_StateInList(t *testing.T) {
	db := setupRuleEvalTestDB(t)
	ctx := context.Background()

	db.Create(&model.DeleteRule{
		Alias: "expr-state", Type: "expr", Priority: 10, Enabled: true,
		Expr: `state in ["uploading", "stalledUP"]`,
	})

	rec := model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "h1", SiteName: "site1",
		Status: model.SeedingStatusSeeding, Discount: model.DiscountNone,
	}
	db.Create(&rec)

	ti := &model.TorrentInfo{Hash: "h1", Name: "test", State: "stalledUP"}
	torrentMap := map[string]*model.TorrentInfo{"h1": ti}

	re := NewRuleEvaluator(db, zap.NewNop())
	matches, err := re.EvaluateRules(ctx, "c1", torrentMap, -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match for state in list, got %d", len(matches))
	}
}

func TestExprEngine_NightProtection(t *testing.T) {
	db := setupRuleEvalTestDB(t)
	ctx := context.Background()

	db.Create(&model.DeleteRule{
		Alias: "expr-night", Type: "expr", Priority: 10, Enabled: true,
		Expr: "hour >= 23 || hour < 6",
	})

	rec := model.SeedingTorrentRecord{
		ClientID: "c1", InfoHash: "h1", SiteName: "site1",
		Status: model.SeedingStatusSeeding, Discount: model.DiscountNone,
	}
	db.Create(&rec)

	ti := &model.TorrentInfo{Hash: "h1", Name: "test"}
	torrentMap := map[string]*model.TorrentInfo{"h1": ti}

	re := NewRuleEvaluator(db, zap.NewNop())
	matches, err := re.EvaluateRules(ctx, "c1", torrentMap, -1)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	isNight := now.Hour() >= 23 || now.Hour() < 6
	if isNight && len(matches) != 1 {
		t.Errorf("night time should match, got %d", len(matches))
	}
	if !isNight && len(matches) != 0 {
		t.Errorf("day time should not match, got %d", len(matches))
	}
}
