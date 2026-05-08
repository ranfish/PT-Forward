package filter

import (
	"context"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRepo(t *testing.T) *Repository {
	t.Helper()
	return NewRepository(setupTestDB(t))
}

func TestRepository_List_Empty(t *testing.T) {
	repo := newTestRepo(t)
	rules, err := repo.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, rules, 0)
}

func TestRepository_List_WithRules(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &model.FilterRule{
		Name: "rule-a", RuleType: "accept", Priority: 20, Enabled: true,
	}))
	require.NoError(t, repo.Create(ctx, &model.FilterRule{
		Name: "rule-b", RuleType: "reject", Priority: 10, Enabled: true,
	}))

	rules, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, rules, 2)
	assert.Equal(t, "rule-b", rules[0].Name)
	assert.Equal(t, "rule-a", rules[1].Name)
}

func TestRepository_List_ExcludesSoftDeleted(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &model.FilterRule{
		Name: "keep", RuleType: "accept", Priority: 10, Enabled: true,
	}))
	require.NoError(t, repo.Create(ctx, &model.FilterRule{
		Name: "delete-me", RuleType: "reject", Priority: 20, Enabled: true,
	}))

	rules, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, rules, 2)

	require.NoError(t, repo.Delete(ctx, rules[1].ID))

	rules, err = repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, rules, 1)
	assert.Equal(t, "keep", rules[0].Name)
}

func TestRepository_GetByID_Found(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	created := &model.FilterRule{
		Name: "find-me", RuleType: "accept", Priority: 5, Enabled: true,
	}
	require.NoError(t, repo.Create(ctx, created))
	assert.NotZero(t, created.ID)

	rule, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "find-me", rule.Name)
	assert.Equal(t, "accept", rule.RuleType)
	assert.Equal(t, 5, rule.Priority)
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	repo := newTestRepo(t)
	_, err := repo.GetByID(context.Background(), 99999)
	assert.Error(t, err)
}

func TestRepository_Create_Basic(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	rule := &model.FilterRule{
		Name: "new-rule", RuleType: "accept", Priority: 10, Enabled: true,
	}
	require.NoError(t, repo.Create(ctx, rule))
	assert.NotZero(t, rule.ID)
	assert.NotZero(t, rule.CreatedAt)
}

func TestRepository_Create_WithConditions(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	rule := &model.FilterRule{
		Name:     "cond-rule",
		RuleType: "reject",
		Priority: 5,
		Enabled:  true,
		Conditions: []model.RuleCondition{
			{Key: "title", CompareType: model.CompareContain, Value: "HD"},
			{Key: "size", CompareType: model.CompareBigger, Value: "1073741824"},
		},
	}
	require.NoError(t, repo.Create(ctx, rule))

	found, err := repo.GetByID(ctx, rule.ID)
	require.NoError(t, err)
	require.Len(t, found.Conditions, 2)
	assert.Equal(t, "title", found.Conditions[0].Key)
	assert.Equal(t, model.CompareContain, found.Conditions[0].CompareType)
	assert.Equal(t, "HD", found.Conditions[0].Value)
	assert.Equal(t, "size", found.Conditions[1].Key)
	assert.Equal(t, model.CompareBigger, found.Conditions[1].CompareType)
}

func TestRepository_Update_Name(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	rule := &model.FilterRule{
		Name: "original", RuleType: "accept", Priority: 10, Enabled: true,
	}
	require.NoError(t, repo.Create(ctx, rule))

	rule.Name = "updated"
	require.NoError(t, repo.Update(ctx, rule))

	found, err := repo.GetByID(ctx, rule.ID)
	require.NoError(t, err)
	assert.Equal(t, "updated", found.Name)
}

func TestRepository_Update_Conditions(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	rule := &model.FilterRule{
		Name:       "cond-update",
		RuleType:   "accept",
		Priority:   10,
		Enabled:    true,
		Conditions: []model.RuleCondition{{Key: "title", CompareType: model.CompareEquals, Value: "old"}},
	}
	require.NoError(t, repo.Create(ctx, rule))

	rule.Conditions = []model.RuleCondition{
		{Key: "title", CompareType: model.CompareContain, Value: "new"},
		{Key: "size", CompareType: model.CompareSmaller, Value: "536870912"},
	}
	require.NoError(t, repo.Update(ctx, rule))

	found, err := repo.GetByID(ctx, rule.ID)
	require.NoError(t, err)
	require.Len(t, found.Conditions, 2)
	assert.Equal(t, model.CompareContain, found.Conditions[0].CompareType)
	assert.Equal(t, "new", found.Conditions[0].Value)
}

func TestRepository_Update_Priority(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	rule := &model.FilterRule{
		Name: "prio", RuleType: "accept", Priority: 100, Enabled: true,
	}
	require.NoError(t, repo.Create(ctx, rule))

	rule.Priority = 1
	require.NoError(t, repo.Update(ctx, rule))

	found, err := repo.GetByID(ctx, rule.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, found.Priority)
}

func TestRepository_Delete_SoftDelete(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	rule := &model.FilterRule{
		Name: "to-delete", RuleType: "accept", Priority: 10, Enabled: true,
	}
	require.NoError(t, repo.Create(ctx, rule))

	require.NoError(t, repo.Delete(ctx, rule.ID))

	_, err := repo.GetByID(ctx, rule.ID)
	assert.Error(t, err)

	rules, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, rules, 0)
}

func TestRepository_Delete_NonExistent(t *testing.T) {
	repo := newTestRepo(t)
	err := repo.Delete(context.Background(), 99999)
	assert.NoError(t, err)
}

func TestRepository_ExistsByName_Exists(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &model.FilterRule{
		Name: "unique-name", RuleType: "accept", Priority: 10, Enabled: true,
	}))

	exists, err := repo.ExistsByName(ctx, "unique-name", 0)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestRepository_ExistsByName_NotExists(t *testing.T) {
	repo := newTestRepo(t)
	exists, err := repo.ExistsByName(context.Background(), "no-such-name", 0)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestRepository_ExistsByName_WithExcludeID(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	rule := &model.FilterRule{
		Name: "exclude-test", RuleType: "accept", Priority: 10, Enabled: true,
	}
	require.NoError(t, repo.Create(ctx, rule))

	exists, err := repo.ExistsByName(ctx, "exclude-test", rule.ID)
	require.NoError(t, err)
	assert.False(t, exists)

	exists, err = repo.ExistsByName(ctx, "exclude-test", 0)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestRepository_ListEnabled_AllEnabled(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &model.FilterRule{
		Name: "e1", RuleType: "accept", Priority: 20, Enabled: true,
	}))
	require.NoError(t, repo.Create(ctx, &model.FilterRule{
		Name: "e2", RuleType: "reject", Priority: 10, Enabled: true,
	}))

	rules, err := repo.ListEnabled(ctx, "")
	require.NoError(t, err)
	require.Len(t, rules, 2)
	assert.Equal(t, "e2", rules[0].Name)
	assert.Equal(t, "e1", rules[1].Name)
}

func TestRepository_ListEnabled_FilterByRuleType(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &model.FilterRule{
		Name: "accept-1", RuleType: "accept", Priority: 10, Enabled: true,
	}))
	require.NoError(t, repo.Create(ctx, &model.FilterRule{
		Name: "reject-1", RuleType: "reject", Priority: 5, Enabled: true,
	}))

	rules, err := repo.ListEnabled(ctx, "accept")
	require.NoError(t, err)
	require.Len(t, rules, 1)
	assert.Equal(t, "accept-1", rules[0].Name)

	rules, err = repo.ListEnabled(ctx, "reject")
	require.NoError(t, err)
	require.Len(t, rules, 1)
	assert.Equal(t, "reject-1", rules[0].Name)
}

func TestRepository_ListEnabled_ExcludesDisabled(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &model.FilterRule{
		Name: "on", RuleType: "accept", Priority: 10, Enabled: true,
	}))
	offRule := &model.FilterRule{
		Name: "off", RuleType: "accept", Priority: 20, Enabled: true,
	}
	require.NoError(t, repo.Create(ctx, offRule))
	offRule.Enabled = false
	require.NoError(t, repo.Update(ctx, offRule))

	rules, err := repo.ListEnabled(ctx, "")
	require.NoError(t, err)
	require.Len(t, rules, 1)
	assert.Equal(t, "on", rules[0].Name)
}

func TestRepository_ListEnabled_ExcludesSoftDeleted(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	rule := &model.FilterRule{
		Name: "will-delete", RuleType: "accept", Priority: 10, Enabled: true,
	}
	require.NoError(t, repo.Create(ctx, rule))
	require.NoError(t, repo.Delete(ctx, rule.ID))

	rules, err := repo.ListEnabled(ctx, "")
	require.NoError(t, err)
	assert.Len(t, rules, 0)
}

func TestRepository_Delete_DoesNotAffectOtherRules(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	keep := &model.FilterRule{
		Name: "keep", RuleType: "accept", Priority: 10, Enabled: true,
	}
	remove := &model.FilterRule{
		Name: "remove", RuleType: "reject", Priority: 20, Enabled: true,
	}
	require.NoError(t, repo.Create(ctx, keep))
	require.NoError(t, repo.Create(ctx, remove))

	require.NoError(t, repo.Delete(ctx, remove.ID))

	found, err := repo.GetByID(ctx, keep.ID)
	require.NoError(t, err)
	assert.Equal(t, "keep", found.Name)

	_, err = repo.GetByID(ctx, remove.ID)
	assert.Error(t, err)
}

func TestRepository_GetByID_SkipsSoftDeleted(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	rule := &model.FilterRule{
		Name: "deleted-rule", RuleType: "accept", Priority: 10, Enabled: true,
	}
	require.NoError(t, repo.Create(ctx, rule))
	require.NoError(t, repo.Delete(ctx, rule.ID))

	_, err := repo.GetByID(ctx, rule.ID)
	assert.Error(t, err)
}

func TestRepository_Create_AllFields(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	rule := &model.FilterRule{
		Name:       "full-rule",
		RuleType:   "accept",
		Priority:   42,
		SavePath:   "/downloads/movies",
		Category:   "movie",
		Tags:       "hd,4k",
		Enabled:    true,
		Conditions: []model.RuleCondition{{Key: "title", CompareType: model.CompareRegExp, Value: `(?i)4k`}},
	}
	require.NoError(t, repo.Create(ctx, rule))

	found, err := repo.GetByID(ctx, rule.ID)
	require.NoError(t, err)
	assert.Equal(t, "full-rule", found.Name)
	assert.Equal(t, "/downloads/movies", found.SavePath)
	assert.Equal(t, "movie", found.Category)
	assert.Equal(t, "hd,4k", found.Tags)
	assert.True(t, found.Enabled)
	require.Len(t, found.Conditions, 1)
	assert.Equal(t, model.CompareRegExp, found.Conditions[0].CompareType)
}

func TestRepository_List_OrderByPriorityThenID(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &model.FilterRule{
		Name: "c", RuleType: "accept", Priority: 10, Enabled: true,
	}))
	require.NoError(t, repo.Create(ctx, &model.FilterRule{
		Name: "a", RuleType: "accept", Priority: 5, Enabled: true,
	}))
	require.NoError(t, repo.Create(ctx, &model.FilterRule{
		Name: "b", RuleType: "accept", Priority: 5, Enabled: true,
	}))

	rules, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, rules, 3)
	assert.Equal(t, "a", rules[0].Name)
	assert.Equal(t, "b", rules[1].Name)
	assert.Equal(t, "c", rules[2].Name)
}

func TestRepository_Delete_SetsDeletedAt(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	rule := &model.FilterRule{
		Name: "soft-del", RuleType: "accept", Priority: 10, Enabled: true,
	}
	require.NoError(t, repo.Create(ctx, rule))
	require.NoError(t, repo.Delete(ctx, rule.ID))

	var raw model.FilterRule
	err := repo.db.WithContext(ctx).First(&raw, rule.ID).Error
	require.NoError(t, err)
	assert.NotEqual(t, time.Time{}, raw.DeletedAt)
}
