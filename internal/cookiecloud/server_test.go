package cookiecloud

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// §59.542 追记十锚（V-01 整改路线 1）: 写入端绑定配置行——
// 四态：uuid 匹配写入 / uuid 不匹配拒收 / 无配置拒绝 / 非 builtin 拒绝。
// 关键断言：盲写不得污染 store[cfg.UUID] 与 last_encrypted。
func TestHandleUpdateConfigBinding(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.CookieCloudConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	s := NewServer(nil, db, zap.NewNop())

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/cookiecloud/update", strings.NewReader(body))
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)
		return rec
	}
	loadCfg := func() model.CookieCloudConfig {
		var cfg model.CookieCloudConfig
		db.First(&cfg)
		return cfg
	}

	t.Run("无配置拒绝", func(t *testing.T) {
		rec := post(`{"encrypted":"E1","uuid":"u-any"}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("code=%d（无配置应 400）body=%s", rec.Code, rec.Body.String())
		}
		if _, ok := s.store.Load("u-any"); ok {
			t.Error("无配置时不应写 store")
		}
		var n int64
		db.Model(&model.CookieCloudConfig{}).Count(&n)
		if n != 0 {
			t.Errorf("无配置时不应写 DB（n=%d）", n)
		}
	})

	t.Run("非 builtin 拒绝", func(t *testing.T) {
		db.Create(&model.CookieCloudConfig{Mode: "remote", UUID: "u-cfg", Password: "p"})
		rec := post(`{"encrypted":"E1","uuid":"u-cfg"}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("code=%d（remote 模式应 400）body=%s", rec.Code, rec.Body.String())
		}
		cfg := loadCfg()
		if cfg.LastEncrypted != "" {
			t.Errorf("remote 模式不应写 last_encrypted（=%q）", cfg.LastEncrypted)
		}
		if _, ok := s.store.Load("u-cfg"); ok {
			t.Error("remote 模式不应写 store")
		}
		db.Where("1=1").Delete(&model.CookieCloudConfig{})
	})

	t.Run("uuid 不匹配拒收+零污染", func(t *testing.T) {
		db.Create(&model.CookieCloudConfig{Mode: "builtin", UUID: "u-cfg", Password: "p", LastEncrypted: "E-old"})
		s.store.Store("u-cfg", "E-old")

		rec := post(`{"encrypted":"E-evil","uuid":"u-attacker"}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("code=%d（uuid 不匹配应 400）body=%s", rec.Code, rec.Body.String())
		}
		// store[cfg.UUID] 不被覆盖
		if v, ok := s.store.Load("u-cfg"); !ok || v.(string) != "E-old" {
			t.Errorf("store[cfg.UUID] 被污染（=%v·应保持 E-old）", v)
		}
		// last_encrypted 不被覆盖（全表写缺陷的核心断言）
		if cfg := loadCfg(); cfg.LastEncrypted != "E-old" {
			t.Errorf("last_encrypted 被污染（=%q·应保持 E-old）", cfg.LastEncrypted)
		}
		// 攻击者 uuid 不落 store
		if _, ok := s.store.Load("u-attacker"); ok {
			t.Error("攻击者 uuid 不应进 store")
		}
		db.Where("1=1").Delete(&model.CookieCloudConfig{})
		s.store.Delete("u-cfg")
	})

	t.Run("uuid 匹配写入 store+DB", func(t *testing.T) {
		db.Create(&model.CookieCloudConfig{Mode: "builtin", UUID: "u-cfg", Password: "p"})
		rec := post(`{"encrypted":"E-new","uuid":"u-cfg"}`)
		if rec.Code != http.StatusOK {
			t.Errorf("code=%d（匹配应 200）body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"done"`) {
			t.Errorf("body=%q（应含 action:done）", rec.Body.String())
		}
		if v, ok := s.store.Load("u-cfg"); !ok || v.(string) != "E-new" {
			t.Errorf("store[cfg.UUID]=%v（应 E-new）", v)
		}
		if cfg := loadCfg(); cfg.LastEncrypted != "E-new" {
			t.Errorf("last_encrypted=%q（应 E-new）", cfg.LastEncrypted)
		}
	})

	t.Run("get 语义不变", func(t *testing.T) {
		// 配置 uuid 可读·未知 uuid 404
		req := httptest.NewRequest(http.MethodGet, "/cookiecloud/get/u-cfg", nil)
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "E-new") {
			t.Errorf("get cfg.uuid: code=%d body=%s", rec.Code, rec.Body.String())
		}
		req2 := httptest.NewRequest(http.MethodGet, "/cookiecloud/get/u-other", nil)
		rec2 := httptest.NewRecorder()
		s.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusNotFound {
			t.Errorf("get unknown: code=%d（应 404）", rec2.Code)
		}
	})
}

// §59.542 追记十锚: updateConfigSyncTime 精确更新——多行配置只动配置行。
func TestUpdateConfigSyncTimePrecise(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.CookieCloudConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := &SyncService{db: db, logger: zap.NewNop()}

	var rowA, rowB model.CookieCloudConfig
	db.Create(&rowA)
	db.Create(&rowB)

	svc.updateConfigSyncTime(context.Background())

	var count int64
	db.Model(&model.CookieCloudConfig{}).Where("last_sync_at IS NOT NULL").Count(&count)
	if count == 2 {
		// 单配置模型下 First 取第一行——但 Where(1=1) 旧实现会两行都写。
		// 精确更新应只写一行（First 取到的那行）。
		t.Errorf("精确更新后 last_sync_at 非空行数=%d（应 1——只动配置行）", count)
	}
}
