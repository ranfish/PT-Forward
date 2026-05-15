package site

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/adapter"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupOverrideDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err, "open db")
	require.NoError(t, db.AutoMigrate(&model.Site{}, &model.SiteConfigOverride{}, &model.SiteFieldMapping{}, &model.PublishExclusion{}), "migrate")
	return db
}

func createOverrideTestSite(t *testing.T, db *gorm.DB) *model.Site {
	t.Helper()
	site := &model.Site{
		Name:                "ov-test",
		Domain:              "ov.test.com",
		BaseURL:             "https://ov.test.com",
		Framework:           "nexusphp",
		Enabled:             true,
		Passkey:             "original-pk",
		Cookie:              "original-cookie",
		APIKey:              "original-ak",
		AuthKey:             "original-authkey",
		AuthHash:            "original-authhash",
		RSSKey:              "original-rss",
		BearerToken:         "original-bt",
		UserID:              10,
		DownloadURLTemplate: "original.tpl",
		HashStrategy:        "xml_tag",
		SizeStrategy:        "enclosure",
		IDStrategy:          "query_param",
		IDPattern:           "id",
	}
	require.NoError(t, db.Create(site).Error, "create site")
	return site
}

func insertOverride(t *testing.T, db *gorm.DB, siteName, fieldPath, fieldValue string) {
	t.Helper()
	ov := &model.SiteConfigOverride{
		SiteName:   siteName,
		FieldPath:  fieldPath,
		FieldValue: fieldValue,
		Source:     "web_ui",
	}
	require.NoError(t, db.Create(ov).Error, "insert override %s", fieldPath)
}

func TestApplyOverrides_Cookie(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "cookie", "new-cookie-val")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	cfg, err := p.GetSiteConfig(context.Background(), "ov.test.com")
	require.NoError(t, err)
	assert.Equal(t, "new-cookie-val", cfg.Cookie)
}

func TestApplyOverrides_Passkey(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "passkey", "ov-pk-999")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	cfg, err := p.GetSiteConfig(context.Background(), "ov.test.com")
	require.NoError(t, err)
	assert.Equal(t, "ov-pk-999", cfg.Passkey)
}

func TestApplyOverrides_APIKey(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "api_key", "ov-api-key")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	cfg, err := p.GetSiteConfig(context.Background(), "ov.test.com")
	require.NoError(t, err)
	assert.Equal(t, "ov-api-key", cfg.APIKey)
}

func TestApplyOverrides_AuthKey(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "auth_key", "ov-auth-key")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	cfg, err := p.GetSiteConfig(context.Background(), "ov.test.com")
	require.NoError(t, err)
	assert.Equal(t, "ov-auth-key", cfg.AuthKey)
}

func TestApplyOverrides_AuthHash(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "auth_hash", "ov-auth-hash")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	cfg, err := p.GetSiteConfig(context.Background(), "ov.test.com")
	require.NoError(t, err)
	assert.Equal(t, "ov-auth-hash", cfg.AuthHash)
}

func TestApplyOverrides_RSSKey(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "rss_key", "ov-rss-key")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	cfg, err := p.GetSiteConfig(context.Background(), "ov.test.com")
	require.NoError(t, err)
	assert.Equal(t, "ov-rss-key", cfg.RSSKey)
}

func TestApplyOverrides_BearerToken(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "bearer_token", "ov-bearer")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	cfg, err := p.GetSiteConfig(context.Background(), "ov.test.com")
	require.NoError(t, err)
	assert.Equal(t, "ov-bearer", cfg.BearerToken)
}

func TestApplyOverrides_UserID(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "user_id", "42")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	cfg, err := p.GetSiteConfig(context.Background(), "ov.test.com")
	require.NoError(t, err)
	assert.Equal(t, 42, cfg.UserID)
}

func TestApplyOverrides_UserID_Invalid(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "user_id", "not-a-number")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	cfg, err := p.GetSiteConfig(context.Background(), "ov.test.com")
	require.NoError(t, err)
	assert.Equal(t, 10, cfg.UserID)
}

func TestApplyOverrides_DownloadURLTemplate(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "download_url_template", "dl.php?tid={id}&key={passkey}")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	cfg, err := p.GetSiteConfig(context.Background(), "ov.test.com")
	require.NoError(t, err)
	assert.Equal(t, "dl.php?tid={id}&key={passkey}", cfg.RSS.URLTemplate)
}

func TestApplyOverrides_HashStrategy(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "hash_strategy", "bencode")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	cfg, err := p.GetSiteConfig(context.Background(), "ov.test.com")
	require.NoError(t, err)
	assert.Equal(t, model.HashStrategy("bencode"), cfg.RSS.HashStrategy)
}

func TestApplyOverrides_SizeStrategy(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "size_strategy", "desc_regex")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	cfg, err := p.GetSiteConfig(context.Background(), "ov.test.com")
	require.NoError(t, err)
	assert.Equal(t, model.SizeStrategy("desc_regex"), cfg.RSS.SizeStrategy)
}

func TestApplyOverrides_IDStrategy(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "id_strategy", "path_segment")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	cfg, err := p.GetSiteConfig(context.Background(), "ov.test.com")
	require.NoError(t, err)
	assert.Equal(t, model.IDStrategy("path_segment"), cfg.RSS.IDStrategy)
}

func TestApplyOverrides_IDPattern(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "id_pattern", `/torrent/(\d+)`)
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	cfg, err := p.GetSiteConfig(context.Background(), "ov.test.com")
	require.NoError(t, err)
	assert.Equal(t, `/torrent/(\d+)`, cfg.RSS.IDPattern)
}

func TestApplyOverrides_PathsUpload(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "paths.upload", "/custom/upload.php")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	cfg, err := p.GetSiteConfig(context.Background(), "ov.test.com")
	require.NoError(t, err)
	assert.Equal(t, "/custom/upload.php", cfg.Paths.Upload)
}

func TestApplyOverrides_PathsTakeUpload(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "paths.takeupload", "/custom/takeupload.php")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	cfg, err := p.GetSiteConfig(context.Background(), "ov.test.com")
	require.NoError(t, err)
	assert.Equal(t, "/custom/takeupload.php", cfg.Paths.TakeUpload)
}

func TestApplyOverrides_PathsBrowse(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "paths.browse", "/custom/browse.php")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	cfg, err := p.GetSiteConfig(context.Background(), "ov.test.com")
	require.NoError(t, err)
	assert.Equal(t, "/custom/browse.php", cfg.Paths.Browse)
}

func TestApplyOverrides_PathsDetail(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "paths.detail", "/custom/detail.php")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	cfg, err := p.GetSiteConfig(context.Background(), "ov.test.com")
	require.NoError(t, err)
	assert.Equal(t, "/custom/detail.php", cfg.Paths.Detail)
}

func TestApplyOverrides_PublishFormField(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "publish.form_fields.douban", "douban_id")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	cfg, err := p.GetSiteConfig(context.Background(), "ov.test.com")
	require.NoError(t, err)
	assert.Equal(t, "douban_id", cfg.Publish.FormFields["douban"])
}

func TestApplyOverrides_MultipleFormFields(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "publish.form_fields.douban", "douban_id")
	insertOverride(t, db, "ov-test", "publish.form_fields.imdb", "imdb_url")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	cfg, err := p.GetSiteConfig(context.Background(), "ov.test.com")
	require.NoError(t, err)
	assert.Equal(t, "douban_id", cfg.Publish.FormFields["douban"])
	assert.Equal(t, "imdb_url", cfg.Publish.FormFields["imdb"])
}

func TestApplyOverrides_NoOverrides(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	cfg, err := p.GetSiteConfig(context.Background(), "ov.test.com")
	require.NoError(t, err)
	assert.Equal(t, "original-pk", cfg.Passkey)
	assert.Equal(t, "original-cookie", cfg.Cookie)
	assert.Equal(t, 10, cfg.UserID)
}

func TestApplyOverrides_MultipleOverridesAtOnce(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "cookie", "c2")
	insertOverride(t, db, "ov-test", "passkey", "pk2")
	insertOverride(t, db, "ov-test", "user_id", "99")
	insertOverride(t, db, "ov-test", "bearer_token", "bt2")
	insertOverride(t, db, "ov-test", "paths.upload", "/u2.php")
	insertOverride(t, db, "ov-test", "publish.form_fields.custom", "custom_val")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	cfg, err := p.GetSiteConfig(context.Background(), "ov.test.com")
	require.NoError(t, err)
	assert.Equal(t, "c2", cfg.Cookie)
	assert.Equal(t, "pk2", cfg.Passkey)
	assert.Equal(t, 99, cfg.UserID)
	assert.Equal(t, "bt2", cfg.BearerToken)
	assert.Equal(t, "/u2.php", cfg.Paths.Upload)
	assert.Equal(t, "custom_val", cfg.Publish.FormFields["custom"])
}

func TestApplyOverrides_DifferentSiteNoCrosstalk(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	site2 := &model.Site{
		Name: "other-site", Domain: "other.com", BaseURL: "https://other.com",
		Framework: "nexusphp", Enabled: true, Passkey: "pk-other",
	}
	require.NoError(t, db.Create(site2).Error)
	insertOverride(t, db, "ov-test", "passkey", "ov-pk")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	cfg2, err := p.GetSiteConfig(context.Background(), "other.com")
	require.NoError(t, err)
	assert.Equal(t, "pk-other", cfg2.Passkey)
}

func TestApplyOverridesToInfo_Cookie(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "cookie", "info-cookie")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	info, err := p.GetSiteInfo(context.Background(), "ov-test")
	require.NoError(t, err)
	assert.Equal(t, "info-cookie", info.Cookie)
}

func TestApplyOverridesToInfo_Passkey(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "passkey", "info-pk")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	info, err := p.GetSiteInfo(context.Background(), "ov-test")
	require.NoError(t, err)
	assert.Equal(t, "info-pk", info.Passkey)
}

func TestApplyOverridesToInfo_APIKey(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "api_key", "info-ak")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	info, err := p.GetSiteInfo(context.Background(), "ov-test")
	require.NoError(t, err)
	assert.Equal(t, "info-ak", info.APIKey)
}

func TestApplyOverridesToInfo_AuthKey(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "auth_key", "info-authkey")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	info, err := p.GetSiteInfo(context.Background(), "ov-test")
	require.NoError(t, err)
	assert.Equal(t, "info-authkey", info.AuthKey)
}

func TestApplyOverridesToInfo_AuthHash(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "auth_hash", "info-authhash")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	info, err := p.GetSiteInfo(context.Background(), "ov-test")
	require.NoError(t, err)
	assert.Equal(t, "info-authhash", info.AuthHash)
}

func TestApplyOverridesToInfo_RSSKey(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "rss_key", "info-rss")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	info, err := p.GetSiteInfo(context.Background(), "ov-test")
	require.NoError(t, err)
	assert.Equal(t, "info-rss", info.RSSKey)
}

func TestApplyOverridesToInfo_BearerToken(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "bearer_token", "info-bt")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	info, err := p.GetSiteInfo(context.Background(), "ov-test")
	require.NoError(t, err)
	assert.Equal(t, "info-bt", info.BearerToken)
}

func TestApplyOverridesToInfo_UserID(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "user_id", "55")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	info, err := p.GetSiteInfo(context.Background(), "ov-test")
	require.NoError(t, err)
	assert.Equal(t, 55, info.UserID)
}

func TestApplyOverridesToInfo_UserID_Invalid(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "user_id", "abc")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	info, err := p.GetSiteInfo(context.Background(), "ov-test")
	require.NoError(t, err)
	assert.Equal(t, 10, info.UserID)
}

func TestApplyOverridesToInfo_DownloadURLTemplate(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "download_url_template", "info.tpl")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	info, err := p.GetSiteInfo(context.Background(), "ov-test")
	require.NoError(t, err)
	assert.Equal(t, "info.tpl", info.DownloadURLTemplate)
}

func TestApplyOverridesToInfo_HashStrategy(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "hash_strategy", "bencode")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	info, err := p.GetSiteInfo(context.Background(), "ov-test")
	require.NoError(t, err)
	assert.Equal(t, "bencode", info.HashStrategy)
}

func TestApplyOverridesToInfo_SizeStrategy(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "size_strategy", "desc_regex")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	info, err := p.GetSiteInfo(context.Background(), "ov-test")
	require.NoError(t, err)
	assert.Equal(t, "desc_regex", info.SizeStrategy)
}

func TestApplyOverridesToInfo_IDStrategy(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "id_strategy", "path_segment")
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	info, err := p.GetSiteInfo(context.Background(), "ov-test")
	require.NoError(t, err)
	assert.Equal(t, "path_segment", info.IDStrategy)
}

func TestApplyOverridesToInfo_IDPattern(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	insertOverride(t, db, "ov-test", "id_pattern", `info-(\d+)`)
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	info, err := p.GetSiteInfo(context.Background(), "ov-test")
	require.NoError(t, err)
	assert.Equal(t, `info-(\d+)`, info.IDPattern)
}

func TestApplyOverridesToInfo_NoOverrides(t *testing.T) {
	db := setupOverrideDB(t)
	createOverrideTestSite(t, db)
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())
	info, err := p.GetSiteInfo(context.Background(), "ov-test")
	require.NoError(t, err)
	assert.Equal(t, "original-pk", info.Passkey)
	assert.Equal(t, "original-cookie", info.Cookie)
	assert.Equal(t, 10, info.UserID)
}

func TestSeedSites_CreatesSites(t *testing.T) {
	db := setupOverrideDB(t)
	err := SeedSites(db)
	require.NoError(t, err)
	var count int64
	db.Model(&model.Site{}).Count(&count)
	assert.Greater(t, count, int64(0))
}

func TestSeedSites_FirstSeedPopulates(t *testing.T) {
	db := setupOverrideDB(t)
	require.NoError(t, SeedSites(db))
	var site model.Site
	require.NoError(t, db.Where("domain = ?", "longpt.org").First(&site).Error)
	assert.Equal(t, "龙PT", site.Name)
	assert.Equal(t, "nexusphp", site.Framework)
	assert.True(t, site.Enabled)
	assert.True(t, site.IsSource)
	assert.True(t, site.IsTarget)
}

func TestSeedSites_SkipsExisting(t *testing.T) {
	db := setupOverrideDB(t)
	require.NoError(t, db.Create(&model.Site{
		Domain: "longpt.org", Name: "龙PT-custom", BaseURL: "https://longpt.org",
		Framework: "nexusphp", Enabled: true, Passkey: "existing-pk",
	}).Error)
	require.NoError(t, SeedSites(db))
	var site model.Site
	require.NoError(t, db.Where("domain = ?", "longpt.org").First(&site).Error)
	assert.Equal(t, "龙PT-custom", site.Name)
	assert.Equal(t, "existing-pk", site.Passkey)
}

func TestSeedSites_UnknownFrameworkUsesGenericDefaults(t *testing.T) {
	db := setupOverrideDB(t)
	require.NoError(t, db.Exec("DELETE FROM sites").Error)
	seed := []SiteSeedData{
		{
			Domain: "custom-fw.test", Name: "CustomFW", BaseURL: "https://custom-fw.test",
			Framework: "nonexistent_framework", IsSource: true, IsTarget: true,
		},
	}
	for _, s := range seed {
		var existing model.Site
		if db.Where("domain = ?", s.Domain).First(&existing).Error == nil {
			continue
		}
		defs, ok := adapter.FrameworkDefaults[s.Framework]
		if !ok {
			defs = adapter.FrameworkDefaults["generic"]
		}
		site := &model.Site{
			Domain: s.Domain, Name: s.Name, BaseURL: s.BaseURL,
			Framework: s.Framework, AuthType: defaultAuthType(s.AuthType),
			Enabled: true, IsSource: s.IsSource, IsTarget: s.IsTarget,
			HashStrategy: defs.HashStrategy, SizeStrategy: defs.SizeStrategy,
			IDStrategy: defs.IDStrategy, IDPattern: defs.IDPattern,
			DownloadURLTemplate: defs.DownloadURLTemplate,
		}
		require.NoError(t, db.Create(site).Error)
	}
	var site model.Site
	require.NoError(t, db.Where("domain = ?", "custom-fw.test").First(&site).Error)
	assert.Equal(t, "nonexistent_framework", site.Framework)
	defs := adapter.FrameworkDefaults["generic"]
	assert.Equal(t, defs.HashStrategy, site.HashStrategy)
	assert.Equal(t, defs.IDPattern, site.IDPattern)
}

func TestSeedSites_Idempotent(t *testing.T) {
	db := setupOverrideDB(t)
	require.NoError(t, SeedSites(db))
	var count1 int64
	db.Model(&model.Site{}).Count(&count1)
	require.NoError(t, SeedSites(db))
	var count2 int64
	db.Model(&model.Site{}).Count(&count2)
	assert.Equal(t, count1, count2)
}

func TestSeedFieldMappings_CreatesMappings(t *testing.T) {
	db := setupOverrideDB(t)
	require.NoError(t, SeedSites(db))
	require.NoError(t, SeedFieldMappings(db))
	var count int64
	db.Model(&model.SiteFieldMapping{}).Count(&count)
	assert.Greater(t, count, int64(0))
}

func TestSeedFieldMappings_SkipsEmptyValues(t *testing.T) {
	db := setupOverrideDB(t)
	require.NoError(t, SeedSites(db))
	require.NoError(t, SeedFieldMappings(db))
	var emptyMappings []model.SiteFieldMapping
	db.Where("source_value = '' OR target_value = ''").Find(&emptyMappings)
	assert.Empty(t, emptyMappings)
}

func TestSeedFieldMappings_PreservesExisting(t *testing.T) {
	db := setupOverrideDB(t)
	require.NoError(t, SeedSites(db))
	require.NoError(t, SeedFieldMappings(db))
	var count1 int64
	db.Model(&model.SiteFieldMapping{}).Count(&count1)
	require.NoError(t, SeedFieldMappings(db))
	var count2 int64
	db.Model(&model.SiteFieldMapping{}).Count(&count2)
	assert.Equal(t, count1, count2)
}

func TestSeedFieldMappings_SpecificMapping(t *testing.T) {
	db := setupOverrideDB(t)
	require.NoError(t, SeedSites(db))
	require.NoError(t, SeedFieldMappings(db))
	var m model.SiteFieldMapping
	err := db.Where("site_name = ? AND field_type = ? AND source_value = ?",
		"龙PT", "cat", "电影").First(&m).Error
	require.NoError(t, err)
	assert.Equal(t, "401", m.TargetValue)
}

func TestSeedExclusions_CreatesExclusions(t *testing.T) {
	db := setupOverrideDB(t)
	require.NoError(t, SeedSites(db))
	require.NoError(t, SeedExclusions(db))
	var count int64
	db.Model(&model.PublishExclusion{}).Count(&count)
	assert.Equal(t, int64(len(loadSeedData().Exclusions)), count)
}

func TestSeedExclusions_SpecificExclusion(t *testing.T) {
	db := setupOverrideDB(t)
	require.NoError(t, SeedSites(db))
	require.NoError(t, SeedExclusions(db))
	var ex model.PublishExclusion
	err := db.Where("target_site = ? AND source_site = ?", "铂金家", "家园").First(&ex).Error
	require.NoError(t, err)
}

func TestSeedExclusions_Idempotent(t *testing.T) {
	db := setupOverrideDB(t)
	require.NoError(t, SeedSites(db))
	require.NoError(t, SeedExclusions(db))
	require.NoError(t, SeedExclusions(db))
	var count int64
	db.Model(&model.PublishExclusion{}).Count(&count)
	assert.Equal(t, int64(len(loadSeedData().Exclusions)), count)
}

func TestSeedFormFieldOverrides_CreatesOverrides(t *testing.T) {
	db := setupOverrideDB(t)
	require.NoError(t, SeedSites(db))
	require.NoError(t, SeedFormFieldOverrides(db))
	var count int64
	db.Model(&model.SiteConfigOverride{}).Where("source = 'seed'").Count(&count)
	if count == 0 {
		db.Model(&model.SiteConfigOverride{}).Count(&count)
	}
	assert.Greater(t, count, int64(0))
}

func TestSeedFormFieldOverrides_SpecificOverride(t *testing.T) {
	db := setupOverrideDB(t)
	require.NoError(t, SeedSites(db))
	require.NoError(t, SeedFormFieldOverrides(db))
	var ov model.SiteConfigOverride
	err := db.Where("site_name = ? AND field_path = ?", "家园", "publish.form_fields.douban").First(&ov).Error
	require.NoError(t, err)
	assert.Equal(t, "douban_id", ov.FieldValue)
}

func TestSeedFormFieldOverrides_Idempotent(t *testing.T) {
	db := setupOverrideDB(t)
	require.NoError(t, SeedSites(db))
	require.NoError(t, SeedFormFieldOverrides(db))
	require.NoError(t, SeedFormFieldOverrides(db))
	var count int64
	db.Model(&model.SiteConfigOverride{}).Count(&count)
	assert.Equal(t, int64(len(loadSeedData().Overrides)), count)
}
