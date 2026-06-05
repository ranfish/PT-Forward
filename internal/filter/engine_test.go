package filter

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	if err := repo.Create(context.Background(), &model.FilterRule{
		Name:     "test",
		RuleType: "accept",
		Priority: 10,
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareContain, Value: "HD"},
		},
		Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(repo, nil)
	result, err := engine.Match(context.Background(), &EvalContext{Title: "HD-Movie"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Matched {
		t.Error("should match")
	}
	if result.Reject {
		t.Error("accept rule should not be treated as reject")
	}
}

func TestEngine_MatchReject(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	if err := repo.Create(context.Background(), &model.FilterRule{
		Name:     "block",
		RuleType: "reject",
		Priority: 5,
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareRegExp, Value: `[xX]265`},
		},
		Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}

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
	if err := repo.Create(context.Background(), &model.FilterRule{
		Name:     "test",
		RuleType: "accept",
		Priority: 10,
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareEquals, Value: "exact"},
		},
		Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}

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
	if err := repo.Create(context.Background(), &model.FilterRule{
		Name: "reject-x265", RuleType: "reject", Priority: 5,
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareContain, Value: "x265"},
		},
		Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(context.Background(), &model.FilterRule{
		Name: "accept-hd", RuleType: "accept", Priority: 10,
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareContain, Value: "HD"},
		},
		Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	engine := NewEngine(repo, nil)
	result, _ := engine.Match(context.Background(), &EvalContext{Title: "HD x265 Movie"})
	if !result.Reject {
		t.Error("reject (priority 5) should fire before accept (priority 10)")
	}
}

func TestMatchConditionExport_NotRegExp(t *testing.T) {
	cond := model.RuleCondition{Key: "title", CompareType: model.CompareNotRegExp, Value: `[xX]\.?265`}
	ctx := &EvalContext{Title: "Movie x264 4K"}
	assert.True(t, MatchConditionExport(cond, ctx), "x264 should NOT match x265 regex")

	ctx2 := &EvalContext{Title: "Movie x265 4K"}
	assert.False(t, MatchConditionExport(cond, ctx2), "x265 should match x265 regex")
}

func TestMatchConditionExport_NotRegExpInvalid(t *testing.T) {
	cond := model.RuleCondition{Key: "title", CompareType: model.CompareNotRegExp, Value: `[invalid`}
	ctx := &EvalContext{Title: "anything"}
	assert.False(t, MatchConditionExport(cond, ctx), "invalid regex should return false")
}

func TestMatchConditionExport_NotIncludeIn(t *testing.T) {
	cond := model.RuleCondition{Key: "category", CompareType: model.CompareNotIncludeIn, Value: "movie,tv,music"}
	ctx := &EvalContext{Category: "anime"}
	assert.True(t, MatchConditionExport(cond, ctx), "anime NOT in list should return true")

	ctx2 := &EvalContext{Category: "movie"}
	assert.False(t, MatchConditionExport(cond, ctx2), "movie IS in list should return false")

	ctx3 := &EvalContext{Category: "tv"}
	assert.False(t, MatchConditionExport(cond, ctx3), "tv IS in list should return false")
}

func TestMatchConditionExport_UnknownCompareType(t *testing.T) {
	cond := model.RuleCondition{Key: "title", CompareType: "unknown_type", Value: "test"}
	ctx := &EvalContext{Title: "test"}
	assert.False(t, MatchConditionExport(cond, ctx), "unknown compare type should return false")
}

func TestEngine_MatchAcceptAndReject(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	require.NoError(t, repo.Create(context.Background(), &model.FilterRule{
		Name:     "aar",
		RuleType: "accept_and_reject",
		Priority: 10,
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareContain, Value: "HD"},
		},
		Enabled: true,
	}))

	engine := NewEngine(repo, nil)
	result, err := engine.Match(context.Background(), &EvalContext{Title: "HD-Movie"})
	require.NoError(t, err)
	assert.True(t, result.Matched, "should match rule")
	assert.True(t, result.Reject, "accept_and_reject rule should still reject")
}

func TestEngine_MatchAcceptWithSavePathCategoryTags(t *testing.T) {
	repo := NewRepository(setupTestDB(t))
	require.NoError(t, repo.Create(context.Background(), &model.FilterRule{
		Name:     "accept-fields",
		RuleType: "accept",
		Priority: 10,
		SavePath: "/downloads/hd",
		Category: "movies",
		Tags:     "hd,4k",
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareContain, Value: "HD"},
		},
		Enabled: true,
	}))

	engine := NewEngine(repo, nil)
	result, err := engine.Match(context.Background(), &EvalContext{Title: "HD-Movie"})
	require.NoError(t, err)
	require.True(t, result.Matched)
	assert.False(t, result.Reject, "accept rule should not reject")
	assert.Equal(t, "accept-fields", result.RuleName)
	assert.Equal(t, "/downloads/hd", result.SavePath)
	assert.Equal(t, "movies", result.Category)
	assert.Equal(t, "hd,4k", result.Tags)
}

func TestGetField_Uploader(t *testing.T) {
	ctx := &EvalContext{Uploader: "alice"}
	assert.Equal(t, "alice", getField(ctx, "uploader"))
}

func TestGetField_SiteName(t *testing.T) {
	ctx := &EvalContext{SiteName: "MySite"}
	assert.Equal(t, "MySite", getField(ctx, "sitename"))
	assert.Equal(t, "MySite", getField(ctx, "site"))
}

func TestGetField_Free(t *testing.T) {
	ctx := &EvalContext{Free: true}
	assert.Equal(t, "true", getField(ctx, "free"))
}

func TestGetField_Tags(t *testing.T) {
	ctx := &EvalContext{Tags: []string{"hd", "4k"}}
	assert.Equal(t, "hd,4k", getField(ctx, "tags"))
}

func TestGetField_Unknown(t *testing.T) {
	ctx := &EvalContext{Title: "test"}
	assert.Equal(t, "", getField(ctx, "unknown_key"))
}

func TestGetField_DiscountLevel(t *testing.T) {
	ctx := &EvalContext{DiscountLevel: "FREE"}
	assert.Equal(t, "FREE", getField(ctx, "discount_level"))

	empty := &EvalContext{}
	assert.Equal(t, "", getField(empty, "discount_level"))
}

func TestMatchCondition_DiscountLevel(t *testing.T) {
	cond := model.RuleCondition{Key: "discount_level", CompareType: model.CompareEquals, Value: "FREE"}
	assert.True(t, MatchConditionExport(cond, &EvalContext{DiscountLevel: "FREE"}))
	assert.False(t, MatchConditionExport(cond, &EvalContext{DiscountLevel: "NONE"}))
	assert.False(t, MatchConditionExport(cond, &EvalContext{DiscountLevel: ""}))
}

func TestCompareNumbers_NonNumericFallback(t *testing.T) {
	assert.Equal(t, 1, compareNumbers("zzz", "aaa"), "string compare zzz > aaa")
	assert.Equal(t, -1, compareNumbers("aaa", "zzz"), "string compare aaa < zzz")
	assert.Equal(t, 0, compareNumbers("abc", "abc"), "string compare abc == abc")
}

func TestEngine_MatchRepoError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.Close()

	repo := NewRepository(db)
	engine := NewEngine(repo, nil)
	result, err := engine.Match(context.Background(), &EvalContext{Title: "test"})
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestEngine_MatchByIDs_RejectMatch(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	engine := NewEngine(repo, nil)

	rule := &model.FilterRule{
		Name:     "reject-big",
		RuleType: "reject",
		Enabled:  true,
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareContain, Value: "spam"},
		},
	}
	require.NoError(t, repo.Create(context.Background(), rule))

	result, err := engine.MatchByIDs(context.Background(), []uint{rule.ID}, &EvalContext{Title: "this is spam torrent"})
	require.NoError(t, err)
	assert.True(t, result.Matched)
	assert.True(t, result.Reject)
	assert.Equal(t, "reject-big", result.RuleName)
}

func TestEngine_MatchByIDs_NoMatch(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	engine := NewEngine(repo, nil)

	rule := &model.FilterRule{
		Name:     "reject-big",
		RuleType: "reject",
		Enabled:  true,
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareContain, Value: "spam"},
		},
	}
	require.NoError(t, repo.Create(context.Background(), rule))

	result, err := engine.MatchByIDs(context.Background(), []uint{rule.ID}, &EvalContext{Title: "clean torrent"})
	require.NoError(t, err)
	assert.False(t, result.Matched)
}

func TestEngine_MatchByIDs_AcceptRule(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	engine := NewEngine(repo, nil)

	rule := &model.FilterRule{
		Name:     "accept-free",
		RuleType: "accept",
		Enabled:  true,
		SavePath: "/data/free",
		Conditions: []model.RuleCondition{
			{Key: "discount_level", CompareType: model.CompareEquals, Value: "FREE"},
		},
	}
	require.NoError(t, repo.Create(context.Background(), rule))

	result, err := engine.MatchByIDs(context.Background(), []uint{rule.ID}, &EvalContext{DiscountLevel: "FREE"})
	require.NoError(t, err)
	assert.True(t, result.Matched)
	assert.False(t, result.Reject)
	assert.Equal(t, "/data/free", result.SavePath)
}

func TestEngine_MatchByIDs_EmptyIDs(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	engine := NewEngine(repo, nil)

	result, err := engine.MatchByIDs(context.Background(), []uint{}, &EvalContext{Title: "test"})
	require.NoError(t, err)
	assert.False(t, result.Matched)
}

func TestEngine_MatchByIDs_NonexistentID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	engine := NewEngine(repo, nil)

	result, err := engine.MatchByIDs(context.Background(), []uint{9999}, &EvalContext{Title: "test"})
	require.NoError(t, err)
	assert.False(t, result.Matched)
}

func TestEngine_Match_AcceptRuleNotRejected(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	engine := NewEngine(repo, nil)

	rule := &model.FilterRule{
		Name:     "accept-free",
		RuleType: "accept",
		Enabled:  true,
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareContain, Value: "Ubuntu"},
		},
	}
	require.NoError(t, repo.Create(context.Background(), rule))

	result, err := engine.Match(context.Background(), &EvalContext{Title: "Ubuntu 24.04"})
	require.NoError(t, err)
	assert.True(t, result.Matched)
	assert.False(t, result.Reject, "accept rule should not be treated as reject")
	assert.Equal(t, "accept", result.RuleType)
}

func TestEngine_Match_RejectRuleRejected(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	engine := NewEngine(repo, nil)

	rule := &model.FilterRule{
		Name:     "reject-spam",
		RuleType: "reject",
		Enabled:  true,
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareContain, Value: "spam"},
		},
	}
	require.NoError(t, repo.Create(context.Background(), rule))

	result, err := engine.Match(context.Background(), &EvalContext{Title: "spam content"})
	require.NoError(t, err)
	assert.True(t, result.Matched)
	assert.True(t, result.Reject)
	assert.Equal(t, "reject", result.RuleType)
}
