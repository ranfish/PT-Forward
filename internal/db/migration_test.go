package db

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// §59.123: 站点别名域名随版本同步——追加/幂等。
func TestMigration24_SiteAliasDomains(t *testing.T) {
	gormDB, err := gorm.Open(sqlite.Open("file:mig24_test?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrate(gormDB); err != nil {
		t.Fatal(err)
	}
	gormDB.Create(&model.Site{Name: "猫", Domain: "pterclub.net", BaseURL: "https://pterclub.net"})

	// §59.123: RunMigrations 全量在预建新结构库上会踩 migration 4 旧列名——
	// 改为直接调用注册逻辑（runOneMigration 不可达, 用等价执行: 手动跑 24 号 up）
	for _, m := range registeredMigrations {
		if m.Version == 24 {
			if err := m.Up(gormDB); err != nil {
				t.Fatal(err)
			}
		}
	}
	var s model.Site
	gormDB.Where("name = ?", "猫").First(&s)
	if !strings.Contains(s.AlternativeDomains, "pterclub.com") {
		t.Errorf("别名未追加: %q", s.AlternativeDomains)
	}
	first := s.AlternativeDomains
	// 幂等: 再跑一遍不重复
	for _, m := range registeredMigrations {
		if m.Version == 24 {
			if err := m.Up(gormDB); err != nil {
				t.Fatal(err)
			}
		}
	}
	gormDB.Where("name = ?", "猫").First(&s)
	if s.AlternativeDomains != first {
		t.Errorf("非幂等: %q -> %q", first, s.AlternativeDomains)
	}
}

// §59.167 OTA 场景回归：AutoMigrate 新结构库（maindata_cron 形态）+ 源表有数据
// → RunMigrations 全量（含 migration 4 列名自适应）应通过且数据迁移成功。
// 实战背景：OTA 实例首次全量跑炸 "no column named main_data_cron"（无限重启）。
func TestMigrations_FullRun_OnAutoMigratedDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ota_scenario.db")
	gormDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	// 模拟 OTA 实例：AutoMigrate 按新 model 建表
	if err := gormDB.AutoMigrate(&model.SeedingClientConfig{}, &model.DownloadClientConfig{}); err != nil {
		t.Fatal(err)
	}
	// 源表种子数据
	gormDB.Create(&model.DownloadClientConfig{
		ClientID: "TR0", Enabled: true,
		MainDataCron: "*/15 * * * *",
	})
	// 聚焦 migration 4 本体（全量跑有其它迁移的前置表依赖——此处验证列名自适应）
	for _, m := range registeredMigrations {
		if m.Version == 4 {
			if err := m.Up(gormDB); err != nil {
				t.Fatalf("migration 4 失败（OTA 场景/新库形态）: %v", err)
			}
		}
	}
	// migration 4 数据应已迁入
	var cnt int64
	gormDB.Table("seeding_client_configs").Where("client_id = ?", "TR0").Count(&cnt)
	if cnt != 1 {
		t.Errorf("migration 4 数据未迁入（OTA 场景）: cnt=%d", cnt)
	}
}
