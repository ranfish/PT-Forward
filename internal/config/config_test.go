package config

import (
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestValidate_OK(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config should be valid: %v", err)
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.Port = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for port 0")
	}

	cfg.Server.Port = 99999
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for port 99999")
	}
}

func TestValidate_InvalidMode(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.Mode = "test"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestValidate_InvalidDriver(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Database.Driver = "postgres"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for invalid driver")
	}
}

func TestValidate_MySQLNoDSN(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Database.Driver = "mysql"
	cfg.Database.MySQLDSN = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for mysql without dsn")
	}
}

func TestValidate_MySQLWithDSN(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Database.Driver = "mysql"
	cfg.Database.MySQLDSN = "user:pass@tcp(localhost:3306)/db"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("mysql with dsn should be valid: %v", err)
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Log.Level = "verbose"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for invalid log level")
	}
}

func TestValidate_LoginMaxRetries(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Security.LoginLockoutEnabled = true
	cfg.Security.LoginMaxRetries = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for login_max_retries < 1")
	}
}

func TestValidate_LoginLockoutMin(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Security.LoginLockoutEnabled = true
	cfg.Security.LoginLockoutMin = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for login_lockout_min < 1")
	}
}

func TestValidate_LoginLockoutDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Security.LoginLockoutEnabled = false
	cfg.Security.LoginMaxRetries = 0
	cfg.Security.LoginLockoutMin = 0
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected no error when lockout disabled, got %v", err)
	}
}

func TestDefaultConfig_Values(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Server.Port != 8765 {
		t.Errorf("expected port 8765, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.Database.Driver != "sqlite" {
		t.Errorf("expected driver sqlite, got %s", cfg.Database.Driver)
	}
	if cfg.Server.Mode != "release" {
		t.Errorf("expected mode release, got %s", cfg.Server.Mode)
	}
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load("", zap.NewNop())
	if err != nil {
		t.Fatalf("empty path should use defaults: %v", err)
	}
	if cfg.Server.Port != 8765 {
		t.Errorf("expected default port 8765, got %d", cfg.Server.Port)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml", zap.NewNop())
	if err != nil {
		t.Fatalf("missing file should use defaults: %v", err)
	}
	if cfg.Server.Port != 8765 {
		t.Errorf("expected default port, got %d", cfg.Server.Port)
	}
}

func TestLoad_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := []byte("server:\n  port: 9999\n  mode: debug\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath, zap.NewNop())
	if err != nil {
		t.Fatalf("valid yaml should load: %v", err)
	}
	if cfg.Server.Port != 9999 {
		t.Errorf("expected port 9999, got %d", cfg.Server.Port)
	}
	if cfg.Server.Mode != "debug" {
		t.Errorf("expected mode debug, got %s", cfg.Server.Mode)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("{{invalid yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath, zap.NewNop())
	if err == nil {
		t.Fatal("expected error for invalid yaml")
	}
}

func TestLoad_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := []byte("server:\n  port: 0\n")
	if err := os.WriteFile(cfgPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath, zap.NewNop())
	if err == nil {
		t.Fatal("expected error for invalid config values")
	}
}

func TestConfigError(t *testing.T) {
	err := configError(42000, "test message", nil)
	if err.Code != 42000 {
		t.Errorf("expected code 42000, got %d", err.Code)
	}
	if err.Message != "test message" {
		t.Errorf("expected 'test message', got %s", err.Message)
	}
}
