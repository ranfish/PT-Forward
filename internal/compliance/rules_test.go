package compliance

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLoadDefaultRules_Count(t *testing.T) {
	rules, err := LoadDefaultRules()
	if err != nil {
		t.Fatalf("LoadDefaultRules failed: %v", err)
	}
	if len(rules) == 0 {
		t.Error("should have default rules")
	}
	// 13 adult + 12 forbidden_keyword（含繁体） + 1 forbidden_group = 26
	if len(rules) < 25 {
		t.Errorf("expected ≥25 rules, got %d", len(rules))
	}
}

func TestLoadDefaultRules_Types(t *testing.T) {
	rules, _ := LoadDefaultRules()
	typeCount := map[string]int{}
	for _, r := range rules {
		typeCount[r.RuleType]++
	}
	if typeCount["adult"] < 13 {
		t.Errorf("adult rules < 13, got %d", typeCount["adult"])
	}
	if typeCount["forbidden_keyword"] < 7 {
		t.Errorf("forbidden_keyword rules < 7, got %d", typeCount["forbidden_keyword"])
	}
	if typeCount["forbidden_group"] < 1 {
		t.Errorf("forbidden_group rules < 1, got %d", typeCount["forbidden_group"])
	}
}

func TestLoadDefaultRules_HasTraditional(t *testing.T) {
	rules, _ := LoadDefaultRules()
	traditionalSeen := false
	for _, r := range rules {
		if r.RuleType == "forbidden_keyword" {
			for _, trad := range []string{"禁轉", "嚴禁轉載", "謝絕轉載", "獨占"} {
				if r.Pattern == trad {
					traditionalSeen = true
				}
			}
		}
	}
	if !traditionalSeen {
		t.Error("should contain traditional chinese forbidden keywords")
	}
}

func TestSyncComplianceRulesToDB(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db failed: %v", err)
	}
	if err := db.AutoMigrate(&model.ComplianceRule{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	if err := SyncComplianceRulesToDB(db); err != nil {
		t.Fatalf("SyncComplianceRulesToDB failed: %v", err)
	}

	var count int64
	db.Model(&model.ComplianceRule{}).Count(&count)
	if count == 0 {
		t.Error("should have rules in DB after sync")
	}
}

func TestSyncComplianceRulesToDB_Idempotent(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.AutoMigrate(&model.ComplianceRule{})

	// 第一次同步
	SyncComplianceRulesToDB(db)
	var firstCount int64
	db.Model(&model.ComplianceRule{}).Count(&firstCount)

	// 第二次同步（幂等，不应重复创建）
	SyncComplianceRulesToDB(db)
	var secondCount int64
	db.Model(&model.ComplianceRule{}).Count(&secondCount)

	if secondCount != firstCount {
		t.Errorf("sync should be idempotent: first=%d second=%d", firstCount, secondCount)
	}
}

func TestSyncComplianceRulesToDB_UserRulesNotOverwritten(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	db.AutoMigrate(&model.ComplianceRule{})

	// 预插入 user 规则
	db.Create(&model.ComplianceRule{
		RuleType: "forbidden_keyword",
		Pattern:  "禁转",
		Scope:    "download", // 用户自定义 scope
		Source:   "user",
	})

	SyncComplianceRulesToDB(db)

	// user 规则不应被覆盖
	var rule model.ComplianceRule
	db.Where("rule_type = ? AND pattern = ? AND source = ?", "forbidden_keyword", "禁转", "user").First(&rule)
	if rule.Scope != "download" {
		t.Errorf("user rule scope should not be overwritten, got %q", rule.Scope)
	}
}
