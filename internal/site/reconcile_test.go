package site

// §59.512 幽灵站点三防线锚：启动对账删除/支持集站不误删/断言 fail-fast。
import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func reconcileDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Site{}, &model.ReleaseGroupMapping{}, &model.OperationAuditLog{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestSeedSitesReconcileDeletesGhost(t *testing.T) {
	db := reconcileDB(t)
	// 幽灵行（不在支持集·enabled=1 潜伏形态——243 pt.cdy.skin 案）
	ghost := &model.Site{Name: "传道院", Domain: "pt.cdy.skin", BaseURL: "https://pt.cdy.skin",
		Framework: "nexusphp", AuthType: "cookie"}
	if err := db.Create(ghost).Error; err != nil {
		t.Fatal(err)
	}
	// 支持集对照行（hdtime.org 在支持集·不该被动）
	okRow := &model.Site{Name: "时光", Domain: "hdtime.org", BaseURL: "https://hdtime.org",
		Framework: "nexusphp", AuthType: "cookie"}
	if err := db.Create(okRow).Error; err != nil {
		t.Fatal(err)
	}

	if err := SeedSites(db); err != nil {
		t.Fatal(err)
	}
	// 幽灵被删
	var n int64
	db.Model(&model.Site{}).Where("domain = ?", "pt.cdy.skin").Count(&n)
	if n != 0 {
		t.Error("幽灵行应被启动对账删除")
	}
	// 审计留痕在
	var auditN int64
	db.Model(&model.OperationAuditLog{}).Where("target_id = ?", "pt.cdy.skin").Count(&auditN)
	if auditN != 1 {
		t.Errorf("幽灵删除应有审计留痕, got %d", auditN)
	}
	// 支持集行不动
	db.Model(&model.Site{}).Where("domain = ?", "hdtime.org").Count(&n)
	if n != 1 {
		t.Error("支持集行不应被对账触碰")
	}
}

func TestSeedSitesReconcileKeepsSupportedOnlySite(t *testing.T) {
	db := reconcileDB(t)
	// 修道院：仅在 supported_sites.json（105）不在 data/sites.json（104）
	// ——由 migration33 建行。判定源若误用 seed 数据该行会被误删
	// （用户配置丢失）。本锚锁死判定源=supported 集。
	xdy := &model.Site{Name: "修道院", Domain: "xdypt.vip", BaseURL: "https://xdypt.vip",
		Framework: "nexusphp", AuthType: "cookie", Passkey: "user-config-value"}
	if err := db.Create(xdy).Error; err != nil {
		t.Fatal(err)
	}
	if err := SeedSites(db); err != nil {
		t.Fatal(err)
	}
	var got model.Site
	if err := db.Where("domain = ?", "xdypt.vip").First(&got).Error; err != nil {
		t.Fatal("修道院行不应被对账删除（判定源=supported·非 seed 数据）")
	}
	if got.Passkey != "user-config-value" {
		t.Error("对账不应触碰用户凭证字段")
	}
}
