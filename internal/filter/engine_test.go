package filter

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.FilterRule{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestMatchRule_Equals(t *testing.T) {
	rule := &model.FilterRule{
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareEquals, Value: "test"},
		},
	}
	ctx := &EvalContext{Title: "test"}
	if !matchRule(rule, ctx) {
		t.Error("should match equals")
	}
	ctx2 := &EvalContext{Title: "other"}
	if matchRule(rule, ctx2) {
		t.Error("should not match equals")
	}
}

func TestMatchRule_Contain(t *testing.T) {
	rule := &model.FilterRule{
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareContain, Value: "HD"},
		},
	}
	if !matchRule(rule, &EvalContext{Title: "HD-Movie"}) {
		t.Error("should match contain")
	}
	if matchRule(rule, &EvalContext{Title: "SD-Movie"}) {
		t.Error("should not match contain")
	}
}

func TestMatchRule_NotContain(t *testing.T) {
	rule := &model.FilterRule{
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareNotContain, Value: "x265"},
		},
	}
	if !matchRule(rule, &EvalContext{Title: "HD-Movie"}) {
		t.Error("should match not_contain")
	}
	if matchRule(rule, &EvalContext{Title: "Movie.x265"}) {
		t.Error("should not match not_contain")
	}
}

func TestMatchRule_Bigger(t *testing.T) {
	rule := &model.FilterRule{
		Conditions: []model.RuleCondition{
			{Key: "size", CompareType: model.CompareBigger, Value: "1073741824"},
		},
	}
	if !matchRule(rule, &EvalContext{Size: 2147483648}) {
		t.Error("2GB should be bigger than 1GB")
	}
	if matchRule(rule, &EvalContext{Size: 536870912}) {
		t.Error("512MB should not be bigger than 1GB")
	}
}

func TestMatchRule_Smaller(t *testing.T) {
	rule := &model.FilterRule{
		Conditions: []model.RuleCondition{
			{Key: "size", CompareType: model.CompareSmaller, Value: "1073741824"},
		},
	}
	if !matchRule(rule, &EvalContext{Size: 536870912}) {
		t.Error("512MB should be smaller than 1GB")
	}
	if matchRule(rule, &EvalContext{Size: 2147483648}) {
		t.Error("2GB should not be smaller than 1GB")
	}
}

func TestMatchRule_RegExp(t *testing.T) {
	rule := &model.FilterRule{
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareRegExp, Value: `[xX]\.?265`},
		},
	}
	if !matchRule(rule, &EvalContext{Title: "Movie x265 4K"}) {
		t.Error("should match regexp")
	}
	if !matchRule(rule, &EvalContext{Title: "Movie X.265"}) {
		t.Error("should match regexp X.265")
	}
	if matchRule(rule, &EvalContext{Title: "Movie x264"}) {
		t.Error("should not match regexp for x264")
	}
}

func TestMatchRule_IncludeIn(t *testing.T) {
	rule := &model.FilterRule{
		Conditions: []model.RuleCondition{
			{Key: "category", CompareType: model.CompareIncludeIn, Value: "movie,tv,music"},
		},
	}
	if !matchRule(rule, &EvalContext{Category: "movie"}) {
		t.Error("movie should be included")
	}
	if !matchRule(rule, &EvalContext{Category: "tv"}) {
		t.Error("tv should be included")
	}
	if matchRule(rule, &EvalContext{Category: "anime"}) {
		t.Error("anime should not be included")
	}
}

func TestMatchRule_MultipleConditions(t *testing.T) {
	rule := &model.FilterRule{
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareContain, Value: "HD"},
			{Key: "size", CompareType: model.CompareBigger, Value: "1073741824"},
		},
	}
	if !matchRule(rule, &EvalContext{Title: "HD-Movie", Size: 2147483648}) {
		t.Error("should match both conditions")
	}
	if matchRule(rule, &EvalContext{Title: "HD-Movie", Size: 536870912}) {
		t.Error("should fail size condition")
	}
	if matchRule(rule, &EvalContext{Title: "SD-Movie", Size: 2147483648}) {
		t.Error("should fail title condition")
	}
}

func TestMatchRule_NoConditions(t *testing.T) {
	rule := &model.FilterRule{Conditions: []model.RuleCondition{}}
	if matchRule(rule, &EvalContext{}) {
		t.Error("empty conditions should not match")
	}
}

func TestEngine_MatchAccept(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	repo.Create(context.Background(), &model.FilterRule{
		Name:     "test",
		RuleType: "accept",
		Priority: 10,
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareContain, Value: "HD"},
		},
		Enabled: true,
	})

	engine := NewEngine(repo, nil)
	result, err := engine.Match(context.Background(), &EvalContext{Title: "HD-Movie"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Matched {
		t.Error("should match")
	}
	if result.Reject {
		t.Error("accept rule should not reject")
	}
}

func TestEngine_MatchReject(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	repo.Create(context.Background(), &model.FilterRule{
		Name:     "block",
		RuleType: "reject",
		Priority: 5,
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareRegExp, Value: `[xX]265`},
		},
		Enabled: true,
	})

	engine := NewEngine(repo, nil)
	result, err := engine.Match(context.Background(), &EvalContext{Title: "Movie x265"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Matched {
		t.Error("should match reject rule")
	}
	if !result.Reject {
		t.Error("should be rejected")
	}
}

func TestEngine_NoMatch(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	repo.Create(context.Background(), &model.FilterRule{
		Name:     "test",
		RuleType: "accept",
		Priority: 10,
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareEquals, Value: "exact"},
		},
		Enabled: true,
	})

	engine := NewEngine(repo, nil)
	result, err := engine.Match(context.Background(), &EvalContext{Title: "other"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Matched {
		t.Error("should not match")
	}
}

func TestEngine_RejectPriority(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	repo.Create(context.Background(), &model.FilterRule{
		Name: "reject-x265", RuleType: "reject", Priority: 5,
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareContain, Value: "x265"},
		},
		Enabled: true,
	})
	repo.Create(context.Background(), &model.FilterRule{
		Name: "accept-hd", RuleType: "accept", Priority: 10,
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareContain, Value: "HD"},
		},
		Enabled: true,
	})

	engine := NewEngine(repo, nil)
	result, _ := engine.Match(context.Background(), &EvalContext{Title: "HD x265 Movie"})
	if !result.Reject {
		t.Error("reject (priority 5) should fire before accept (priority 10)")
	}
}
