package db

import (
	"encoding/json"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// §59.289: 修道院纪录片映射双键幂等 migration
func TestMigration40DocumentaryDualKey(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := gdb.AutoMigrate(&model.Site{}); err != nil {
		t.Fatal(err)
	}
	base := `{"enabled":true,"form_fields":{"type":"type"},"value_mappings":{"type":[{"label":"纪录片（Documentaries）","value":"404","standard_keys":["category.documentaries"]},{"label":"电影（Movies）","value":"401","standard_keys":["category.movie"]}]}}`
	gdb.Create(&model.Site{Name: "修道院", PublishFormConfig: base})

	if err := gdb.AutoMigrate(&model.SchemaMigration{}); err != nil {
		t.Fatal(err)
	}
	var mig Migration
	for _, m := range registeredMigrations {
		if m.Version == 40 {
			mig = m
		}
	}
	if mig.Up == nil {
		t.Fatal("migration 40 未注册")
	}
	if err := mig.Up(gdb); err != nil {
		t.Fatal(err)
	}
	var site model.Site
	gdb.Where("name = ?", "修道院").First(&site)
	var cfg model.PublishFormConfig
	if err := json.Unmarshal([]byte(site.PublishFormConfig), &cfg); err != nil {
		t.Fatal(err)
	}
	for _, m := range cfg.ValueMappings["type"] {
		if m.Label == "纪录片（Documentaries）" {
			if len(m.StandardKeys) != 2 || m.StandardKeys[0] != "category.documentary" {
				t.Errorf("双键未补: %v", m.StandardKeys)
			}
		}
	}
	// 幂等
	if err := mig.Up(gdb); err != nil {
		t.Fatal(err)
	}
	var site2 model.Site
	gdb.Where("name = ?", "修道院").First(&site2)
	var cfg2 model.PublishFormConfig
	_ = json.Unmarshal([]byte(site2.PublishFormConfig), &cfg2)
	for _, m := range cfg2.ValueMappings["type"] {
		if m.Label == "纪录片（Documentaries）" && len(m.StandardKeys) != 2 {
			t.Errorf("幂等破坏: %v", m.StandardKeys)
		}
	}
}
