package db

import (
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
