package site

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/adapter"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupProviderDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Site{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func createTestSite(t *testing.T, db *gorm.DB) *model.Site {
	t.Helper()
	site := &model.Site{
		Name:      "test-site",
		Domain:    "test.example.com",
		BaseURL:   "https://test.example.com",
		Framework: "nexusphp",
		Enabled:   true,
		Passkey:   "pk123",
		Cookie:    "sid=abc",
	}
	if err := db.Create(site).Error; err != nil {
		t.Fatalf("create site: %v", err)
	}
	return site
}

func TestProvider_GetSiteInfo(t *testing.T) {
	db := setupProviderDB(t)
	createTestSite(t, db)
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())

	info, err := p.GetSiteInfo(context.Background(), "test-site")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "test-site" {
		t.Errorf("expected test-site, got %s", info.Name)
	}
	if info.Framework != "nexusphp" {
		t.Errorf("expected nexusphp, got %s", info.Framework)
	}
	if !info.Enabled {
		t.Error("expected enabled")
	}
}

func TestProvider_GetSiteInfo_NotFound(t *testing.T) {
	db := setupProviderDB(t)
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())

	_, err := p.GetSiteInfo(context.Background(), "nonexist")
	if err == nil {
		t.Fatal("expected error")
	}
	appErr, ok := err.(*model.AppError)
	if !ok || appErr.Code != 12001 {
		t.Errorf("expected AppError 12001, got %v", err)
	}
}

func TestProvider_GetSiteConfig(t *testing.T) {
	db := setupProviderDB(t)
	createTestSite(t, db)
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())

	config, err := p.GetSiteConfig(context.Background(), "test.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if config.Domain != "test.example.com" {
		t.Errorf("expected test.example.com, got %s", config.Domain)
	}
	if config.Passkey != "pk123" {
		t.Errorf("expected pk123, got %s", config.Passkey)
	}
	if config.Cookie != "sid=abc" {
		t.Errorf("expected sid=abc, got %s", config.Cookie)
	}
}

func TestProvider_GetSiteConfig_NotFound(t *testing.T) {
	db := setupProviderDB(t)
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())

	_, err := p.GetSiteConfig(context.Background(), "nope.com")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProvider_GetSiteDefault(t *testing.T) {
	db := setupProviderDB(t)
	createTestSite(t, db)
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())

	def, err := p.GetSiteDefault(context.Background(), "test.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if def.Domain != "test.example.com" {
		t.Errorf("expected test.example.com, got %s", def.Domain)
	}
	if def.Framework != "nexusphp" {
		t.Errorf("expected nexusphp, got %s", def.Framework)
	}
}

func TestProvider_GetSiteDefault_UnknownFramework(t *testing.T) {
	db := setupProviderDB(t)
	site := &model.Site{
		Name: "unknown-fw", Domain: "unknown.com", BaseURL: "https://unknown.com",
		Framework: "custom_unknown", Enabled: true,
	}
	db.Create(site)
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())

	def, err := p.GetSiteDefault(context.Background(), "unknown.com")
	if err != nil {
		t.Fatal(err)
	}
	if def.Framework != "custom_unknown" {
		t.Errorf("expected custom_unknown, got %s", def.Framework)
	}
}

func TestProvider_GetAdapter(t *testing.T) {
	db := setupProviderDB(t)
	createTestSite(t, db)
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())

	a, err := p.GetAdapter(context.Background(), "test.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatal("expected non-nil adapter")
	}
	if a.Framework() != "nexusphp" {
		t.Errorf("expected nexusphp adapter, got %s", a.Framework())
	}
}

func TestProvider_GetAdapter_Cached(t *testing.T) {
	db := setupProviderDB(t)
	createTestSite(t, db)
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())

	a1, _ := p.GetAdapter(context.Background(), "test.example.com")
	a2, _ := p.GetAdapter(context.Background(), "test.example.com")
	if a1 != a2 {
		t.Error("expected same adapter instance (cached)")
	}
}

func TestProvider_GetAdapter_NotFound(t *testing.T) {
	db := setupProviderDB(t)
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())

	_, err := p.GetAdapter(context.Background(), "nope.com")
	if err == nil {
		t.Fatal("expected error for unknown domain")
	}
}

func TestProvider_InvalidateAdapter(t *testing.T) {
	db := setupProviderDB(t)
	createTestSite(t, db)
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())

	a1, _ := p.GetAdapter(context.Background(), "test.example.com")
	p.InvalidateAdapter("test.example.com")
	a2, _ := p.GetAdapter(context.Background(), "test.example.com")
	if a1 == a2 {
		t.Error("expected different adapter after invalidation")
	}
}

func TestProvider_ListSites(t *testing.T) {
	db := setupProviderDB(t)
	createTestSite(t, db)
	db.Create(&model.Site{
		Name: "site2", Domain: "site2.com", BaseURL: "https://site2.com",
		Framework: "tnode", Enabled: false,
	})
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())

	sites, err := p.ListSites(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 2 {
		t.Fatalf("expected 2 sites, got %d", len(sites))
	}
}

func TestProvider_ListSites_Empty(t *testing.T) {
	db := setupProviderDB(t)
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())

	sites, err := p.ListSites(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 0 {
		t.Errorf("expected 0 sites, got %d", len(sites))
	}
}

func TestProvider_GetSiteInfoByURL(t *testing.T) {
	db := setupProviderDB(t)
	createTestSite(t, db)
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())

	info, err := p.GetSiteInfoByURL(context.Background(), "https://test.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "test-site" {
		t.Errorf("expected test-site, got %s", info.Name)
	}
}

func TestProvider_GetSiteInfoByURL_NotFound(t *testing.T) {
	db := setupProviderDB(t)
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())

	_, err := p.GetSiteInfoByURL(context.Background(), "https://nope.com")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProvider_DetectFramework(t *testing.T) {
	db := setupProviderDB(t)
	createTestSite(t, db)
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())

	result, err := p.DetectFramework(context.Background(), "test.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if result.Framework != "nexusphp" {
		t.Errorf("expected nexusphp, got %s", result.Framework)
	}
	if result.Confidence != 1.0 {
		t.Errorf("expected confidence 1.0, got %f", result.Confidence)
	}
}

func TestProvider_DetectFramework_NotFound(t *testing.T) {
	db := setupProviderDB(t)
	p := NewProvider(db, adapter.NewFactory(zap.NewNop()), zap.NewNop())

	_, err := p.DetectFramework(context.Background(), "nope.com")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestProvider_SiteToInfo_FieldMapping(t *testing.T) {
	s := &model.Site{
		Name:      "mapped",
		Domain:    "mapped.com",
		BaseURL:   "https://mapped.com",
		Framework: "gazelle",
		Enabled:   true,
		Passkey:   "pass1",
		Cookie:    "cookie1",
		APIKey:    "apikey1",
		UserID:    42,
	}
	info := siteToInfo(s)
	if info.Name != "mapped" {
		t.Errorf("name mismatch")
	}
	if info.BaseURL != "https://mapped.com" {
		t.Errorf("baseURL mismatch")
	}
	if info.Passkey != "pass1" {
		t.Errorf("passkey mismatch")
	}
}

func TestProvider_SiteToConfig_FieldMapping(t *testing.T) {
	s := &model.Site{
		Domain:    "cfg.com",
		Framework: "unit3d",
		Enabled:   true,
		IsSource:  true,
		IsTarget:  false,
		Passkey:   "p1",
		Cookie:    "c1",
		AuthKey:   "ak1",
	}
	config := siteToConfig(s)
	if config.Domain != "cfg.com" {
		t.Errorf("domain mismatch")
	}
	if !config.IsSource {
		t.Error("expected IsSource=true")
	}
	if config.IsTarget {
		t.Error("expected IsTarget=false")
	}
	if config.Passkey != "p1" {
		t.Errorf("passkey mismatch")
	}
}

func TestDefaultPaths(t *testing.T) {
	tests := []struct {
		framework string
		upload    string
		browse    string
	}{
		{"nexusphp", "/upload.php", "/browse.php"},
		{"unit3d", "/upload", "/torrents"},
		{"gazelle", "/upload.php", "/torrents.php"},
		{"tnode", "/api/torrent/upload", "/torrent/list"},
		{"mteam", "/upload.php", "/browse"},
		{"rousi", "/api/v1/torrents", "/api/v1/torrents"},
		{"generic", "/upload.php", "/browse.php"},
	}
	for _, tt := range tests {
		p := defaultPaths(tt.framework)
		if p.Upload != tt.upload {
			t.Errorf("defaultPaths(%q).Upload = %q, want %q", tt.framework, p.Upload, tt.upload)
		}
		if p.Browse != tt.browse {
			t.Errorf("defaultPaths(%q).Browse = %q, want %q", tt.framework, p.Browse, tt.browse)
		}
	}
}

func TestDefaultPublishConfig(t *testing.T) {
	np := defaultPublishConfig("nexusphp")
	if np.FormFields["category"] != "type" {
		t.Errorf("nexusphp category should map to type, got %q", np.FormFields["category"])
	}
	if np.FormFields["codec"] != "codec_sel" {
		t.Errorf("nexusphp codec should map to codec_sel, got %q", np.FormFields["codec"])
	}

	mt := defaultPublishConfig("mteam")
	if mt.FormFields["codec"] != "videoCodec" {
		t.Errorf("mteam codec should map to videoCodec, got %q", mt.FormFields["codec"])
	}

	u3d := defaultPublishConfig("unit3d")
	if u3d.FormFields["category"] != "category_id" {
		t.Errorf("unit3d category should map to category_id, got %q", u3d.FormFields["category"])
	}

	gz := defaultPublishConfig("gazelle")
	if gz.FormFields["codec"] != "format" {
		t.Errorf("gazelle codec should map to format, got %q", gz.FormFields["codec"])
	}
}

func TestSiteToConfig_Paths(t *testing.T) {
	s := &model.Site{Domain: "test.com", Framework: "nexusphp", BaseURL: "https://test.com"}
	config := siteToConfig(s)

	if config.Paths.Upload != "/upload.php" {
		t.Errorf("nexusphp upload path: got %q", config.Paths.Upload)
	}
	if config.Paths.TakeUpload != "/takeupload.php" {
		t.Errorf("nexusphp takeupload path: got %q", config.Paths.TakeUpload)
	}
}

func TestSiteToConfig_PublishFormFields(t *testing.T) {
	s := &model.Site{Domain: "test.com", Framework: "mteam", BaseURL: "https://test.com"}
	config := siteToConfig(s)

	if config.Publish.FormFields["standard"] != "standard" {
		t.Errorf("mteam standard mapping: got %q", config.Publish.FormFields["standard"])
	}
}
