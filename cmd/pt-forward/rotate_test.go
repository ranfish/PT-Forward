package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/ranfish/pt-forward/internal/config"
	"github.com/ranfish/pt-forward/internal/crypto"
	"github.com/ranfish/pt-forward/internal/setting"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&setting.Setting{}); err != nil {
		t.Fatalf("migrate system_settings: %v", err)
	}
	// 轮换遍历的最小表结构（真实表由生产 AutoMigrate 建·此处按需最小集）
	for _, ddl := range []string{
		"CREATE TABLE IF NOT EXISTS sites (id INTEGER PRIMARY KEY, passkey TEXT, cookie TEXT, api_key TEXT, bearer_token TEXT, auth_key TEXT, auth_hash TEXT, rss_key TEXT)",
		"CREATE TABLE IF NOT EXISTS clients (id INTEGER PRIMARY KEY, password TEXT)",
		"CREATE TABLE IF NOT EXISTS cookie_cloud_configs (id INTEGER PRIMARY KEY, password TEXT)",
		"CREATE TABLE IF NOT EXISTS iyuu_configs (id INTEGER PRIMARY KEY, token TEXT)",
		"CREATE TABLE IF NOT EXISTS notification_channels (id INTEGER PRIMARY KEY, config TEXT)",
		"CREATE TABLE IF NOT EXISTS site_config_overrides (id INTEGER PRIMARY KEY, field_value TEXT)",
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatalf("ddl: %v", err)
		}
	}
	return db
}

// TestRotateKeyEndToEnd：旧密钥加密数据 → 轮换 → 新密钥可解+config 替换+DB 行清理
func TestRotateKeyEndToEnd(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()

	oldKey, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatal(err)
	}
	oldEnc, _ := crypto.NewCredentialEncryptor(oldKey)

	// 造数据：sites 两条（加密+明文各一）+ clients 一条
	encPass, _ := oldEnc.Encrypt("secret-passkey-1")
	encAPI, _ := oldEnc.Encrypt("secret-api-1")
	db.Exec("INSERT INTO sites (id, passkey, api_key) VALUES (1, ?, ?)", encPass, encAPI)
	db.Exec("INSERT INTO sites (id, passkey, api_key) VALUES (2, 'plain-legacy', '')")
	encPwd, _ := oldEnc.Encrypt("client-password")
	db.Exec("INSERT INTO clients (id, password) VALUES (1, ?)", encPwd)

	// config 文件（旧密钥）
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte("log:\n  level: info\n\nsecurity:\n  encryption_key: "+oldKey+"\n"), 0640)

	// sqlite 文件路径（backup 用）——内存库场景给临时文件占位（backup 会失败？看 driver）
	cfg := &config.Config{}
	cfg.Security.EncryptionKey = oldKey
	cfg.Database.Driver = "sqlite"
	cfg.Database.SQLitePath = filepath.Join(dir, "pt.db")
	os.WriteFile(cfg.Database.SQLitePath, []byte("placeholder"), 0600)

	log := zap.NewNop()
	if err := runRotateKey(cfg, cfgPath, db, log); err != nil {
		t.Fatalf("runRotateKey: %v", err)
	}

	// ① config 已替换为新密钥（非追加·只有一个 encryption_key）
	content, _ := os.ReadFile(cfgPath)
	s := string(content)
	if s == oldKey || !containsOnce(s, "encryption_key:") {
		t.Fatalf("config 未正确替换:\n%s", s)
	}
	newKey := extractKey(s)
	if newKey == "" || newKey == oldKey {
		t.Fatalf("新密钥未写入或与旧相同: %q", newKey)
	}

	// ② DB 数据用新密钥可解
	newEnc, _ := crypto.NewCredentialEncryptor(newKey)
	var p1 string
	db.Raw("SELECT passkey FROM sites WHERE id = 1").Scan(&p1)
	if got, err := newEnc.Decrypt(p1); err != nil || got != "secret-passkey-1" {
		t.Fatalf("sites.passkey 新密钥解密失败: %v / %q", err, got)
	}
	var c1 string
	db.Raw("SELECT password FROM clients WHERE id = 1").Scan(&c1)
	if got, err := newEnc.Decrypt(c1); err != nil || got != "client-password" {
		t.Fatalf("clients.password 新密钥解密失败: %v / %q", err, got)
	}

	// ③ 明文行保持明文（不被破坏）
	var p2 string
	db.Raw("SELECT passkey FROM sites WHERE id = 2").Scan(&p2)
	if p2 != "plain-legacy" {
		t.Fatalf("明文行被改动: %q", p2)
	}

	// ④ DB 密钥残留行已清
	stored, _ := setting.NewRepository(db).Get(context.Background(), "encryption_key")
	if stored != "" {
		t.Fatalf("DB 密钥行未清理: %q", stored)
	}

	// ⑤ 旧密钥解不开新数据（轮换有效性）
	if _, err := oldEnc.Decrypt(p1); err == nil {
		t.Fatal("旧密钥仍能解密新数据——轮换无效")
	}
}

// TestInitEncryptionMigrateFromDB：密钥在 DB（历史 fallback）→ 启动自动迁移到 config+删行
func TestInitEncryptionMigrateFromDB(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()

	key, _ := crypto.GenerateRandomKey()
	setting.NewRepository(db).Set(context.Background(), "encryption_key", key)

	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte("log:\n  level: info\n"), 0640)

	cfg := &config.Config{} // Security.EncryptionKey 空 → 走 DB fallback
	log := zap.NewNop()
	if err := initEncryption(cfg, cfgPath, db, log); err != nil {
		t.Fatalf("initEncryption: %v", err)
	}

	content, _ := os.ReadFile(cfgPath)
	if k := extractKey(string(content)); k != key {
		t.Fatalf("密钥未迁移到 config: %q", k)
	}
	stored, _ := setting.NewRepository(db).Get(context.Background(), "encryption_key")
	if stored != "" {
		t.Fatalf("DB 密钥行未删除: %q", stored)
	}
}

// TestInitEncryptionClearLegacy：config 有密钥+DB 有旧残留 → 启动清理 DB 行
func TestInitEncryptionClearLegacy(t *testing.T) {
	db := openTestDB(t)
	cfgKey, _ := crypto.GenerateRandomKey()
	dbKey, _ := crypto.GenerateRandomKey()
	setting.NewRepository(db).Set(context.Background(), "encryption_key", dbKey)

	cfg := &config.Config{}
	cfg.Security.EncryptionKey = cfgKey
	if err := initEncryption(cfg, "", db, zap.NewNop()); err != nil {
		t.Fatalf("initEncryption: %v", err)
	}
	stored, _ := setting.NewRepository(db).Get(context.Background(), "encryption_key")
	if stored != "" {
		t.Fatalf("DB 残留未清理: %q", stored)
	}
}

func containsOnce(s, sub string) bool {
	c := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			c++
		}
	}
	return c == 1
}

func extractKey(cfgText string) string {
	lines := splitLines(cfgText)
	inSec := false
	for _, l := range lines {
		t := trimSpace(l)
		if t == "security:" {
			inSec = true
			continue
		}
		if inSec && hasPrefix(t, "encryption_key:") {
			return trimSpace(afterColon(t))
		}
	}
	return ""
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }

func afterColon(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return s[i+1:]
		}
	}
	return s
}

// TestInitEncryptionMigrateExistingSection：config 已有 security 段（key 缺失）
// → 迁移后单段单键（§仓库安全-12 回归审核修③的锚）
func TestInitEncryptionMigrateExistingSection(t *testing.T) {
	db := openTestDB(t)
	dir := t.TempDir()
	key, _ := crypto.GenerateRandomKey()
	setting.NewRepository(db).Set(context.Background(), "encryption_key", key)
	cfgPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgPath, []byte("log:\n  level: info\n\nsecurity:\n  # placeholder\n"), 0640)
	if err := initEncryption(&config.Config{}, cfgPath, db, zap.NewNop()); err != nil {
		t.Fatalf("initEncryption: %v", err)
	}
	content, _ := os.ReadFile(cfgPath)
	s := string(content)
	if !containsOnce(s, "security:") || !containsOnce(s, "encryption_key:") {
		t.Fatalf("迁移产生双段/双键:\n%s", s)
	}
	if k := extractKey(s); k != key {
		t.Fatalf("密钥未正确迁移: %q", k)
	}
}
