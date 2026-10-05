package db

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// §59.318 D2: 语义拆分迁移——继承/收敛/死列清理三断言 + 44 传道院清行。
func TestMigration43SemanticSplit(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open("file:mig43_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrate(gdb); err != nil {
		t.Fatal(err)
	}
	// 模拟存量三态：A=旧探测站（is_target=1 无配置）；B=已适配发布（配置 enabled=true）；
	// C=无配置未开。target_types 列由 AutoMigrate 前旧模型遗留——手动补列模拟存量。
	if err := gdb.Exec("ALTER TABLE sites ADD COLUMN target_types text").Error; err != nil {
		t.Fatal(err)
	}
	seed := []*model.Site{
		{Name: "A", Domain: "a.com", IsTarget: true},
		{Name: "B", Domain: "b.com", IsTarget: true, PublishFormConfig: `{"enabled":true,"form_fields":{}}`},
		{Name: "C", Domain: "c.com"},
	}
	for _, s := range seed {
		if err := gdb.Create(s).Error; err != nil {
			t.Fatal(err)
		}
	}
	gdb.Exec("UPDATE sites SET is_target = 1 WHERE domain IN ('a.com','b.com')")
	gdb.Exec("UPDATE sites SET target_types = '[\"publish\"]' WHERE domain = 'a.com'")

	var m43 *Migration
		for _, m := range registeredMigrations {
			if m.Version == 43 {
				m43 = &m
				break
			}
		}
		if m43 == nil {
			t.Fatal("migration43 未注册")
		}
		if err := m43.Up(gdb); err != nil {
			t.Fatalf("migration43: %v", err)
		}
	type row struct {
		Domain          string
		IsTarget        int
		IsReseedTarget  int
	}
	var rows []row
	gdb.Raw("SELECT domain, is_target, is_reseed_target FROM sites ORDER BY domain").Scan(&rows)
	got := map[string][2]int{}
	for _, r := range rows {
		got[r.Domain] = [2]int{r.IsTarget, r.IsReseedTarget}
	}
	// A：探测继承 true；发布收敛 false（无配置 COALESCE→0）
	if got["a.com"] != [2]int{0, 1} {
		t.Errorf("a.com = %v, want [0 1]", got["a.com"])
	}
	// B：发布真值保留 true；探测也继承 true
	if got["b.com"] != [2]int{1, 1} {
		t.Errorf("b.com = %v, want [1 1]", got["b.com"])
	}
	// C：双 false
	if got["c.com"] != [2]int{0, 0} {
		t.Errorf("c.com = %v, want [0 0]", got["c.com"])
	}
	var hasTT int64
	gdb.Raw("SELECT COUNT(*) FROM pragma_table_info('sites') WHERE name='target_types'").Scan(&hasTT)
	if hasTT != 0 {
		t.Error("target_types 列应已 DROP")
	}
}

// §59.318 N2: 传道院清行。
func TestMigration44CdyPurge(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open("file:mig44_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrate(gdb); err != nil {
		t.Fatal(err)
	}
	gdb.Create(&model.Site{Name: "传道院", Domain: "pt.cdy.skin", Enabled: false})
	gdb.Create(&model.Site{Name: "修道院", Domain: "xdypt.vip"})
	var m44 *Migration
		for _, m := range registeredMigrations {
			if m.Version == 44 {
				m44 = &m
				break
			}
		}
		if m44 == nil {
			t.Fatal("migration44 未注册")
		}
		if err := m44.Up(gdb); err != nil {
			t.Fatalf("migration44: %v", err)
		}
	var n int64
	gdb.Model(&model.Site{}).Where("domain = 'pt.cdy.skin'").Count(&n)
	if n != 0 {
		t.Error("pt.cdy.skin 应已删除")
	}
	gdb.Model(&model.Site{}).Where("domain = 'xdypt.vip'").Count(&n)
	if n != 1 {
		t.Error("修道院应保留")
	}
}
