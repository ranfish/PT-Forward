package config

import (
	"testing"
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
	cfg.Security.LoginMaxRetries = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for login_max_retries < 1")
	}
}

func TestValidate_LoginLockoutMin(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Security.LoginLockoutMin = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for login_lockout_min < 1")
	}
}

func TestValidate_MemoryMaxTotalMB(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Memory.MaxTotalMB = -1
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for negative max_total_mb")
	}
}

func TestValidate_MemoryWarnPercent(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Memory.WarnPercent = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for warn_percent 0")
	}

	cfg.Memory.WarnPercent = 1.5
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for warn_percent > 1")
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
