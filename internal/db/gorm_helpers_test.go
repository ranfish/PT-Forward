package db

import (
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
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.ClientConfig{},
		&model.Site{},
		&model.FilterRule{},
		&model.NotificationChannel{},
		&model.SeedingClientConfig{},
		&model.DeleteRule{},
		&model.RSSSubscription{},
		&model.ClientPublishTarget{},
	))
	return db
}

func TestForceCreate_BoolZero(t *testing.T) {
	db := setupTestDB(t)

	site := &model.Site{
		Name: "test-site", Domain: "test.com", BaseURL: "https://test.com",
		Framework: "nexusphp", Enabled: false, ParticipateAutoPublish: false,
	}
	require.NoError(t, ForceCreate(db, site))

	var result model.Site
	require.NoError(t, db.First(&result, site.ID).Error)
	assert.False(t, result.Enabled, "Enabled should be false, not GORM default true")
}

func TestForceCreate_IntZero(t *testing.T) {
	db := setupTestDB(t)

	rule := &model.FilterRule{
		Name: "test-rule", Priority: 0, Enabled: false, RuleType: "accept",
	}
	require.NoError(t, ForceCreate(db, rule))

	var result model.FilterRule
	require.NoError(t, db.First(&result, rule.ID).Error)
	assert.False(t, result.Enabled, "Enabled should be false")
	assert.Equal(t, 0, result.Priority, "Priority should be 0, not GORM default 100")
}

func TestForceCreate_FloatZero(t *testing.T) {
	db := setupTestDB(t)

	config := &model.SeedingClientConfig{
		ClientID: "test-client", Enabled: false,
		MinDiskSpaceGB: 0, EmergencyBuffer: 0, EmaAlpha: 0,
		SpaceAlarmGB: 0, MinDiskSpacePercent: 0,
	}
	require.NoError(t, ForceCreate(db, config))

	var result model.SeedingClientConfig
	require.NoError(t, db.First(&result, config.ID).Error)
	assert.False(t, result.Enabled, "Enabled should be false")
	assert.Equal(t, 0.0, result.MinDiskSpaceGB, "MinDiskSpaceGB should be 0")
	assert.Equal(t, 0.0, result.EmergencyBuffer, "EmergencyBuffer should be 0")
}

func TestForceCreate_DeleteRuleDeleteCompanions(t *testing.T) {
	db := setupTestDB(t)

	rule := &model.DeleteRule{
		Alias: "safe-rule", Enabled: false, DeleteCompanions: false,
		ReannounceBefore: false, Type: "normal", Action: "delete",
	}
	require.NoError(t, ForceCreate(db, rule))

	var result model.DeleteRule
	require.NoError(t, db.First(&result, rule.ID).Error)
	assert.False(t, result.Enabled, "Enabled should be false")
	assert.False(t, result.DeleteCompanions, "DeleteCompanions should be false — DATA LOSS PREVENTION")
	assert.False(t, result.ReannounceBefore, "ReannounceBefore should be false")
}

func TestForceCreate_NormalCreate(t *testing.T) {
	db := setupTestDB(t)

	site := &model.Site{
		Name: "enabled-site", Domain: "enabled.com", BaseURL: "https://enabled.com",
		Framework: "nexusphp", Enabled: true,
	}
	require.NoError(t, ForceCreate(db, site))

	var result model.Site
	require.NoError(t, db.First(&result, site.ID).Error)
	assert.True(t, result.Enabled, "Enabled should remain true when set to true")
}

func TestGORMCreate_Baseline(t *testing.T) {
	db := setupTestDB(t)

	site := &model.Site{
		Name: "broken-site", Domain: "broken.com", BaseURL: "https://broken.com",
		Framework: "nexusphp", Enabled: false,
	}
	require.NoError(t, db.Create(site).Error)

	var result model.Site
	require.NoError(t, db.First(&result, site.ID).Error)
	assert.True(t, result.Enabled, "GORM default Create overwrites false→true (baseline proof)")
}
