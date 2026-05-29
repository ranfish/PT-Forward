package integration

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/cookiecloud"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func ccMd5(inputs ...string) string {
	h := md5.New()
	for _, s := range inputs {
		fmt.Fprintf(h, "%s", s)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func encryptCookieCloud(t *testing.T, keyPassword string, data map[string]map[string][]cookiecloud.CookieData) string {
	t.Helper()
	plainBytes, err := json.Marshal(data)
	require.NoError(t, err)

	salt := make([]byte, 8)
	_, _ = io.ReadFull(rand.Reader, salt)

	const (
		keyLen = 32
		bl     = 16
	)

	var concat []byte
	var lastHash []byte
	h := md5.New()
	for ; len(concat) < keyLen+bl; h.Reset() {
		h.Write(append(lastHash, append([]byte(keyPassword), salt...)...))
		lastHash = h.Sum(nil)
		concat = append(concat, lastHash...)
	}
	key := concat[:keyLen]
	iv := concat[keyLen : keyLen+bl]

	block, err := aes.NewCipher(key)
	require.NoError(t, err)

	padLen := bl - len(plainBytes)%bl
	padded := make([]byte, len(plainBytes)+padLen)
	copy(padded, plainBytes)
	copy(padded[len(plainBytes):], bytes.Repeat([]byte{byte(padLen)}, padLen))

	ct := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ct, padded)

	raw := append([]byte("Salted__"), salt...)
	raw = append(raw, ct...)
	return base64.StdEncoding.EncodeToString(raw)
}

func setupCookieCloudServer(t *testing.T, encrypted string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"encrypted": encrypted})
	}))
}

func seedCookieCloudConfig(t *testing.T, db *gorm.DB, serverURL, uuid, password string, enabled bool) {
	t.Helper()
	cfg := &model.CookieCloudConfig{
		ServerURL:   serverURL,
		UUID:        uuid,
		Password:    password,
		SyncEnabled: enabled,
		SyncInterval: 60,
	}
	require.NoError(t, db.Create(cfg).Error)
}

func seedSiteWithCC(t *testing.T, db *gorm.DB, name, domain string, ccSync bool, ccDomain string, altDomains string) {
	t.Helper()
	site := &model.Site{
		Name:               name,
		Domain:             domain,
		BaseURL:            "https://" + domain,
		AuthType:           "cookie",
		Enabled:            true,
		CookieCloudSync:    ccSync,
		CookieCloudDomain:  ccDomain,
		AlternativeDomains: altDomains,
	}
	require.NoError(t, db.Create(site).Error)
}

// F9: FullSync
func TestScenario_F9_FullSync(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	password := "test-pw-1234"
	keyPassword := ccMd5("uuid-sync", "-", password)[:16]
	cookieData := map[string]map[string][]cookiecloud.CookieData{
		"cookie_data": {
			"sitea.com": {
				{Name: "session", Value: "sess-a-001", Domain: ".sitea.com", Path: "/"},
				{Name: "uid", Value: "12345", Domain: ".sitea.com", Path: "/"},
			},
			"siteb.org": {
				{Name: "token", Value: "tok-b-999", Domain: ".siteb.org", Path: "/"},
			},
		},
	}
	encrypted := encryptCookieCloud(t, keyPassword, cookieData)
	srv := setupCookieCloudServer(t, encrypted)
	defer srv.Close()

	seedCookieCloudConfig(t, db, srv.URL, "uuid-sync", password, true)
	seedSiteWithCC(t, db, "SiteA", "sitea.com", true, "", "")
	seedSiteWithCC(t, db, "SiteB", "siteb.org", true, "", "")

	svc := cookiecloud.NewSyncService(db, zap.NewNop())
	history, err := svc.SyncAll(ctx)
	require.NoError(t, err)

	assert.Equal(t, "completed", history.Status)
	assert.Equal(t, 2, history.SyncedSites)
	assert.Equal(t, 0, history.SkippedSites)
	assert.Greater(t, history.SyncDuration, time.Duration(0))

	var siteA model.Site
	require.NoError(t, db.Where("name = ?", "SiteA").First(&siteA).Error)
	assert.Contains(t, siteA.Cookie, "session=sess-a-001")
	assert.Contains(t, siteA.Cookie, "uid=12345")

	var siteB model.Site
	require.NoError(t, db.Where("name = ?", "SiteB").First(&siteB).Error)
	assert.Contains(t, siteB.Cookie, "token=tok-b-999")

	var configs []model.CookieCloudConfig
	require.NoError(t, db.Find(&configs).Error)
	assert.NotNil(t, configs[0].LastSyncAt)

	var histories []model.CookieCloudSyncHistory
	require.NoError(t, db.Find(&histories).Error)
	assert.Len(t, histories, 1)
	assert.Equal(t, "completed", histories[0].Status)

	t.Logf("PASS F9: full sync synced=%d skipped=%d duration=%v", history.SyncedSites, history.SkippedSites, history.SyncDuration)
}

// F9: 无配置 → 同步失败
func TestScenario_F9_NoConfig(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	seedSiteWithCC(t, db, "SiteA", "sitea.com", true, "", "")

	svc := cookiecloud.NewSyncService(db, zap.NewNop())
	history, err := svc.SyncAll(ctx)
	assert.Error(t, err)
	assert.Equal(t, "failed", history.Status)

	var histories []model.CookieCloudSyncHistory
	require.NoError(t, db.Find(&histories).Error)
	assert.Len(t, histories, 1)

	t.Logf("PASS F9: no config → failed correctly")
}

// F9: 配置禁用 → 同步失败
func TestScenario_F9_DisabledConfig(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	seedCookieCloudConfig(t, db, "http://localhost", "uuid", "pw", false)
	seedSiteWithCC(t, db, "SiteA", "sitea.com", true, "", "")

	svc := cookiecloud.NewSyncService(db, zap.NewNop())
	history, err := svc.SyncAll(ctx)
	assert.Error(t, err)
	assert.Equal(t, "failed", history.Status)

	t.Logf("PASS F9: disabled config → failed correctly")
}

// F9: 无站点 → 成功但 synced=0
func TestScenario_F9_NoSites(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	seedCookieCloudConfig(t, db, "http://localhost", "uuid", "pw", true)

	svc := cookiecloud.NewSyncService(db, zap.NewNop())
	history, err := svc.SyncAll(ctx)
	require.NoError(t, err)
	assert.Equal(t, "completed", history.Status)
	assert.Equal(t, 0, history.SyncedSites)

	t.Logf("PASS F9: no sites → completed, synced=0")
}

// F9: 站点未启用 CookieCloudSync → 跳过
func TestScenario_F9_SiteNotSynced(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	password := "pw-skip"
	keyPassword := ccMd5("uuid-skip", "-", password)[:16]
	cookieData := map[string]map[string][]cookiecloud.CookieData{
		"cookie_data": {
			"sitea.com": {
				{Name: "session", Value: "should-not-write", Domain: ".sitea.com", Path: "/"},
			},
		},
	}
	encrypted := encryptCookieCloud(t, keyPassword, cookieData)
	srv := setupCookieCloudServer(t, encrypted)
	defer srv.Close()

	seedCookieCloudConfig(t, db, srv.URL, "uuid-skip", password, true)
	seedSiteWithCC(t, db, "SiteA", "sitea.com", false, "", "")

	svc := cookiecloud.NewSyncService(db, zap.NewNop())
	history, err := svc.SyncAll(ctx)
	require.NoError(t, err)
	assert.Equal(t, "completed", history.Status)
	assert.Equal(t, 0, history.SyncedSites)

	var site model.Site
	require.NoError(t, db.Where("name = ?", "SiteA").First(&site).Error)
	assert.Empty(t, site.Cookie)

	t.Logf("PASS F9: site not synced → cookie untouched")
}

// F9: CookieCloudDomain 自定义域名匹配
func TestScenario_F9_CustomDomainMatch(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	password := "pw-custom"
	keyPassword := ccMd5("uuid-custom", "-", password)[:16]
	cookieData := map[string]map[string][]cookiecloud.CookieData{
		"cookie_data": {
			"custom-domain.net": {
				{Name: "auth", Value: "custom-token", Domain: ".custom-domain.net", Path: "/"},
			},
		},
	}
	encrypted := encryptCookieCloud(t, keyPassword, cookieData)
	srv := setupCookieCloudServer(t, encrypted)
	defer srv.Close()

	seedCookieCloudConfig(t, db, srv.URL, "uuid-custom", password, true)
	seedSiteWithCC(t, db, "CustomSite", "mysite.com", true, "custom-domain.net", "")

	svc := cookiecloud.NewSyncService(db, zap.NewNop())
	history, err := svc.SyncAll(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, history.SyncedSites)

	var site model.Site
	require.NoError(t, db.Where("name = ?", "CustomSite").First(&site).Error)
	assert.Contains(t, site.Cookie, "auth=custom-token")

	t.Logf("PASS F9: custom domain match → cookie updated")
}

// F9: AlternativeDomains 备用域名匹配
func TestScenario_F9_AlternativeDomain(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	password := "pw-alt"
	keyPassword := ccMd5("uuid-alt", "-", password)[:16]
	cookieData := map[string]map[string][]cookiecloud.CookieData{
		"cookie_data": {
			"alt-site.io": {
				{Name: "sid", Value: "alt-sid-001", Domain: ".alt-site.io", Path: "/"},
			},
		},
	}
	encrypted := encryptCookieCloud(t, keyPassword, cookieData)
	srv := setupCookieCloudServer(t, encrypted)
	defer srv.Close()

	alts, _ := json.Marshal([]string{"https://alt-site.io"})
	seedCookieCloudConfig(t, db, srv.URL, "uuid-alt", password, true)
	seedSiteWithCC(t, db, "AltSite", "mainsite.com", true, "", string(alts))

	svc := cookiecloud.NewSyncService(db, zap.NewNop())
	history, err := svc.SyncAll(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, history.SyncedSites)

	var site model.Site
	require.NoError(t, db.Where("name = ?", "AltSite").First(&site).Error)
	assert.Contains(t, site.Cookie, "sid=alt-sid-001")

	t.Logf("PASS F9: alternative domain → cookie matched via alt")
}

// F9: CookieCloud 服务器不可达 → 失败
func TestScenario_F9_ServerUnreachable(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	seedCookieCloudConfig(t, db, "http://127.0.0.1:1", "uuid", "pw", true)
	seedSiteWithCC(t, db, "SiteA", "sitea.com", true, "", "")

	svc := cookiecloud.NewSyncService(db, zap.NewNop())
	history, err := svc.SyncAll(ctx)
	assert.Error(t, err)
	assert.Equal(t, "failed", history.Status)
	assert.Contains(t, history.ErrorMessage, "fetch/decrypt")

	t.Logf("PASS F9: server unreachable → failed")
}

// F9: 站点无匹配 Cookie → 跳过
func TestScenario_F9_NoMatchingCookies(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	password := "pw-nomatch"
	keyPassword := ccMd5("uuid-nomatch", "-", password)[:16]
	cookieData := map[string]map[string][]cookiecloud.CookieData{
		"cookie_data": {
			"other-site.xyz": {
				{Name: "session", Value: "other-val", Domain: ".other-site.xyz", Path: "/"},
			},
		},
	}
	encrypted := encryptCookieCloud(t, keyPassword, cookieData)
	srv := setupCookieCloudServer(t, encrypted)
	defer srv.Close()

	seedCookieCloudConfig(t, db, srv.URL, "uuid-nomatch", password, true)
	seedSiteWithCC(t, db, "SiteA", "sitea.com", true, "", "")

	svc := cookiecloud.NewSyncService(db, zap.NewNop())
	history, err := svc.SyncAll(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, history.SyncedSites)
	assert.Equal(t, 1, history.SkippedSites)

	var site model.Site
	require.NoError(t, db.Where("name = ?", "SiteA").First(&site).Error)
	assert.Empty(t, site.Cookie)

	t.Logf("PASS F9: no matching cookies → skipped=%d", history.SkippedSites)
}

// F9: 多次同步 → 历史记录累积
func TestScenario_F9_MultipleSyncHistory(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	password := "pw-multi"
	keyPassword := ccMd5("uuid-multi", "-", password)[:16]
	cookieData := map[string]map[string][]cookiecloud.CookieData{
		"cookie_data": {
			"sitea.com": {
				{Name: "session", Value: "val1", Domain: ".sitea.com", Path: "/"},
			},
		},
	}
	encrypted := encryptCookieCloud(t, keyPassword, cookieData)
	srv := setupCookieCloudServer(t, encrypted)
	defer srv.Close()

	seedCookieCloudConfig(t, db, srv.URL, "uuid-multi", password, true)
	seedSiteWithCC(t, db, "SiteA", "sitea.com", true, "", "")

	svc := cookiecloud.NewSyncService(db, zap.NewNop())

	for i := 0; i < 3; i++ {
		history, err := svc.SyncAll(ctx)
		require.NoError(t, err)
		assert.Equal(t, "completed", history.Status)
	}

	var histories []model.CookieCloudSyncHistory
	require.NoError(t, db.Order("id ASC").Find(&histories).Error)
	assert.Len(t, histories, 3)
	for _, h := range histories {
		assert.Equal(t, "completed", h.Status)
		assert.Equal(t, 1, h.SyncedSites)
	}

	t.Logf("PASS F9: 3 sync runs → %d history records", len(histories))
}

// F9: Cookie 更新覆盖旧值
func TestScenario_F9_CookieOverwrite(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	password := "pw-overwrite"
	keyPassword := ccMd5("uuid-ow", "-", password)[:16]
	cookieData := map[string]map[string][]cookiecloud.CookieData{
		"cookie_data": {
			"sitea.com": {
				{Name: "session", Value: "new-session-val", Domain: ".sitea.com", Path: "/"},
			},
		},
	}
	encrypted := encryptCookieCloud(t, keyPassword, cookieData)
	srv := setupCookieCloudServer(t, encrypted)
	defer srv.Close()

	seedCookieCloudConfig(t, db, srv.URL, "uuid-ow", password, true)

	site := &model.Site{
		Name: "SiteA", Domain: "sitea.com", BaseURL: "https://sitea.com",
		AuthType: "cookie", Enabled: true, CookieCloudSync: true,
		Cookie: "session=old-value; other=keep",
	}
	require.NoError(t, db.Create(site).Error)

	svc := cookiecloud.NewSyncService(db, zap.NewNop())
	history, err := svc.SyncAll(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, history.SyncedSites)

	var updated model.Site
	require.NoError(t, db.Where("name = ?", "SiteA").First(&updated).Error)
	assert.Contains(t, updated.Cookie, "session=new-session-val")
	assert.NotContains(t, updated.Cookie, "old-value")

	t.Logf("PASS F9: cookie overwritten old→new")
}

// F9: 站点禁用 → 不参与同步
func TestScenario_F9_DisabledSite(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	password := "pw-disabled-site"
	keyPassword := ccMd5("uuid-ds", "-", password)[:16]
	cookieData := map[string]map[string][]cookiecloud.CookieData{
		"cookie_data": {
			"sitea.com": {
				{Name: "session", Value: "val-a", Domain: ".sitea.com", Path: "/"},
			},
			"siteb.com": {
				{Name: "session", Value: "val-b", Domain: ".siteb.com", Path: "/"},
			},
		},
	}
	encrypted := encryptCookieCloud(t, keyPassword, cookieData)
	srv := setupCookieCloudServer(t, encrypted)
	defer srv.Close()

	seedCookieCloudConfig(t, db, srv.URL, "uuid-ds", password, true)
	seedSiteWithCC(t, db, "SiteA", "sitea.com", true, "", "")

	disabledSite := &model.Site{
		Name: "SiteB", Domain: "siteb.com", BaseURL: "https://siteb.com",
		AuthType: "cookie", CookieCloudSync: true,
	}
	require.NoError(t, db.Create(disabledSite).Error)
	require.NoError(t, db.Model(disabledSite).Update("enabled", false).Error)

	svc := cookiecloud.NewSyncService(db, zap.NewNop())
	history, err := svc.SyncAll(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, history.SyncedSites)

	var siteB model.Site
	require.NoError(t, db.Where("name = ?", "SiteB").First(&siteB).Error)
	assert.Empty(t, siteB.Cookie)

	t.Logf("PASS F9: disabled site excluded from sync")
}

// F9: 纯单元级 — FetchAndDecrypt 完整往返
func TestScenario_F9_FetchAndDecryptRoundTrip(t *testing.T) {
	password := "roundtrip-pw"
	uuid := "uuid-rt"
	keyPassword := ccMd5(uuid, "-", password)[:16]

	original := map[string]map[string][]cookiecloud.CookieData{
		"cookie_data": {
			"example.com": {
				{Name: "cookie1", Value: "v1", Domain: ".example.com", Path: "/"},
				{Name: "cookie2", Value: "v2", Domain: ".example.com", Path: "/sub"},
			},
		},
	}

	encrypted := encryptCookieCloud(t, keyPassword, original)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"encrypted": encrypted})
	}))
	defer srv.Close()

	result, err := cookiecloud.FetchAndDecrypt(srv.URL, uuid, password)
	require.NoError(t, err)
	assert.Len(t, result["example.com"], 2)

	cookies := cookiecloud.FilterCookiesByDomain(result, "example.com")
	assert.Len(t, cookies, 2)

	cookieStr := cookiecloud.CookiesToString(cookies)
	assert.Contains(t, cookieStr, "cookie1=v1")
	assert.Contains(t, cookieStr, "cookie2=v2")

	t.Logf("PASS F9: FetchAndDecrypt roundtrip → %d domains, %d cookies", len(result), len(cookies))
}
