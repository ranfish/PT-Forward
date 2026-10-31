package db_test

// TestFreshInstallChain §59.379: 新装链配置交付回归（CI 固化——新装即拿到从约定变机制）。
//
// 背景：§59.378 发现新装链缺口（RunMigrations 先于 seed——sites 表数据类
// migration 全 miss 却记录已执行→publish_form_config 永远空），修正后
// RunMigrations 挪至 seed 之后。本测试固化修正后的完整新装序列：
//
//	AutoMigrate → seed 全链（SeedSites/FieldMappings/GroupMappings/...）→ RunMigrations
//
// 核心断言：**所有 publish_supported=true 的已适配站，新装环境必须拿到
// 非空 publish_form_config**——未来新增适配站（开闸+form_config migration）
// 或新增配置类 migration 时，若破坏新装交付链（时序回退/漏分发/依赖错误
// 假设），本测试即红。
//
// 放 package db_test（外部测试包）：db→site 存在依赖（site 引 db），
// 同包测试引 site 会 import cycle——外部测试包合法引用依赖被测包的包。

import (
	"path/filepath"
	"testing"

	"github.com/ranfish/pt-forward/internal/db"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/site"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestFreshInstallChain(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fresh_install.db")
	gdb, err := gorm.Open(sqlite.Open(dbPath+"?_journal_mode=WAL&_busy_timeout=5000"), &gorm.Config{
		Logger: gormlogger.Discard,
	})
	if err != nil {
		t.Fatalf("open temp db: %v", err)
	}
	log, _ := zap.NewProduction()

	// ① AutoMigrate（= main 启动序列第一步）
	if err := model.AutoMigrate(gdb); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	// ② seed 全链（= main seed goroutine 完整序列——RunMigrations 之前）
	if err := site.SeedSites(gdb); err != nil {
		t.Fatalf("SeedSites: %v", err)
	}
	if err := site.SyncSiteTemplates(gdb); err != nil {
		t.Fatalf("SyncSiteTemplates: %v", err)
	}
	if err := site.SeedFieldMappings(gdb); err != nil {
		t.Fatalf("SeedFieldMappings: %v", err)
	}
	if err := site.SeedExclusions(gdb); err != nil {
		t.Fatalf("SeedExclusions: %v", err)
	}
	if err := site.SeedFormFieldOverrides(gdb); err != nil {
		t.Fatalf("SeedFormFieldOverrides: %v", err)
	}
	if err := site.SeedGroupMappings(gdb); err != nil {
		t.Fatalf("SeedGroupMappings: %v", err)
	}

	// ③ RunMigrations（= §59.378 修正后位置——seed 之后）
	if err := db.RunMigrations(gdb, log); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// ④ 断言一：所有 publish_supported 站拿到非空发布配置
	for _, supp := range site.ListSupportedSites() {
		if !supp.PublishSupported {
			continue
		}
		var s model.Site
		if err := gdb.Where("domain = ?", supp.Domain).First(&s).Error; err != nil {
			t.Errorf("已适配站 %s(%s) 新装链站行缺失: %v", supp.NameCN, supp.Domain, err)
			continue
		}
		if s.PublishFormConfig == "" {
			t.Errorf("已适配站 %s(%s) 新装链 publish_form_config 为空——配置交付断裂（检查该站 form_config migration 是否存在/是否依赖 seed 先行）", supp.NameCN, supp.Domain)
		}
	}

	// ⑤ 断言二：采集侧资产（site_field_mappings）非空
	var fmCount int64
	gdb.Model(&model.SiteFieldMapping{}).Count(&fmCount)
	if fmCount == 0 {
		t.Error("新装链 site_field_mappings 为空——采集侧表单资产未随 seed 分发")
	}

	// ⑥ 断言三：官组映射资产非空
	var gmCount int64
	gdb.Model(&model.ReleaseGroupMapping{}).Count(&gmCount)
	if gmCount == 0 {
		t.Error("新装链 release_group_mappings 为空——官组映射资产未随 seed 分发")
	}

	// ⑦ 断言四：RunMigrations 幂等（二跑零副作用——重启等价）
	if err := db.RunMigrations(gdb, log); err != nil {
		t.Fatalf("RunMigrations 二跑（幂等性）: %v", err)
	}
	var cfgAfter string
	gdb.Model(&model.Site{}).Where("domain = ?", "52pt.site").Pluck("publish_form_config", &cfgAfter)
	if cfgAfter == "" {
		t.Error("幂等二跑后配置丢失")
	}
}
