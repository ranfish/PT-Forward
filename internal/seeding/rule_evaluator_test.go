package seeding

import (
	"context"
	"fmt"
	"testing"

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
	db.AutoMigrate(
		&model.DeleteRule{},
		&model.SeedingTorrentRecord{},
	)
	return db
}

func TestRuleEvaluator_EvaluateRules_NoRules(t *testing.T) {
	db := setupRuleEvalTestDB(t)
	re := NewRuleEvaluator(db, zap.NewNop())

	matches, err := re.EvaluateRules(context.Background(), "c1")
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
	matches, err := re.EvaluateRules(ctx, "c1")
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
	matches, err := re.EvaluateRules(ctx, "c1")
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
	matches, err := re.EvaluateRules(ctx, "c1")
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
	matches, err := re.EvaluateRules(ctx, "c1")
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
	matches, err := re.EvaluateRules(ctx, "c1")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for empty conditions, got %d", len(matches))
	}
}

func TestParseConditions(t *testing.T) {
	valid := `[{"field":"site_name","operator":"equals","value":"s1"}]`
	conds := parseConditions(valid)
	if len(conds) != 1 {
		t.Fatalf("expected 1, got %d", len(conds))
	}
	if conds[0].Field != "site_name" {
		t.Errorf("expected site_name, got %s", conds[0].Field)
	}

	conds = parseConditions("")
	if conds != nil {
		t.Errorf("expected nil for empty, got %v", conds)
	}

	conds = parseConditions("not-json")
	if conds != nil {
		t.Errorf("expected nil for invalid json, got %v", conds)
	}
}

func TestRuleEvaluator_MatchCondition_Operators(t *testing.T) {
	re := NewRuleEvaluator(nil, zap.NewNop())

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
			got := re.matchCondition(rec, tt.cond)
			if got != tt.match {
				t.Errorf("got %v, want %v", got, tt.match)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s      string
		sub    string
		expect bool
	}{
		{"hello", "ell", true},
		{"hello", "world", false},
		{"hello", "hello", true},
		{"hello", "", true},
		{"", "", true},
		{"", "a", false},
	}

	for _, tt := range tests {
		got := contains(tt.s, tt.sub)
		if got != tt.expect {
			t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.sub, got, tt.expect)
		}
	}
}
