package config

import (
	"fmt"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
)

type Config struct {
	Server       model.ServerConfig          `yaml:"server" mapstructure:"server"`
	Database     model.DatabaseConfig        `yaml:"database" mapstructure:"database"`
	Log          model.LogConfig             `yaml:"log" mapstructure:"log"`
	Notification model.NotificationConfig    `yaml:"notification" mapstructure:"notification"`
	Security     model.SecurityConfig        `yaml:"security" mapstructure:"security"`
	CookieCloud  model.CookieCloudYAMLConfig `yaml:"cookiecloud" mapstructure:"cookiecloud"`
	DataCleanup  model.DataCleanupConfig     `yaml:"data_cleanup" mapstructure:"data_cleanup"`
	IYUU         model.IYUUSection           `yaml:"iyuu" mapstructure:"iyuu"`
}

func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return configError(ErrConfigValidate, fmt.Sprintf("server.port must be 1-65535, got %d", c.Server.Port), nil)
	}
	if c.Server.Mode != "debug" && c.Server.Mode != "release" {
		return configError(ErrConfigValidate, fmt.Sprintf("server.mode must be 'debug' or 'release', got %q", c.Server.Mode), nil)
	}
	if c.Database.Driver != "sqlite" && c.Database.Driver != "mysql" {
		return configError(ErrConfigValidate, fmt.Sprintf("database.driver must be 'sqlite' or 'mysql', got %q", c.Database.Driver), nil)
	}
	if c.Database.Driver == "mysql" && c.Database.MySQLDSN == "" {
		return configError(ErrConfigValidate, "database.mysql_dsn is required when driver is mysql", nil)
	}
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[strings.ToLower(c.Log.Level)] {
		return configError(ErrConfigValidate, fmt.Sprintf("log.level must be debug/info/warn/error, got %q", c.Log.Level), nil)
	}
	if c.Security.LoginLockoutEnabled {
		if c.Security.LoginMaxRetries < 1 {
			return configError(ErrConfigValidate, fmt.Sprintf("security.login_max_retries must be >= 1, got %d", c.Security.LoginMaxRetries), nil)
		}
		if c.Security.LoginLockoutMin < 1 {
			return configError(ErrConfigValidate, fmt.Sprintf("security.login_lockout_min must be >= 1, got %d", c.Security.LoginLockoutMin), nil)
		}
	}
	return nil
}

func DefaultConfig() *Config {
	return &Config{
		Server: model.ServerConfig{
			Host:        "0.0.0.0",
			Port:        8765,
			Mode:        "release",
			CORSOrigins: []string{"*"},
		},
		Database: model.DatabaseConfig{
			Driver:       "sqlite",
			SQLitePath:   "./data/pt-forward.db",
			MaxOpenConns: 10,
			MaxIdleConns: 5,
			LogLevel:     "warn",
		},
		Log: model.LogConfig{
			Directory:  "./logs",
			Level:      "info",
			MaxSizeMB:  10,
			MaxAgeDays: 30,
			MaxBackups: 10,
			Compress:   true,
			Format:     "json",
		},
		Security: model.SecurityConfig{
			RateLimitEnabled:    true,
			RateLimitGlobal:     100,
			RateLimitWrite:      30,
			RateLimitDownload:   10,
			LoginMaxRetries:     5,
			LoginLockoutMin:     5,
			LoginLockoutEnabled: true,
		},
	}
}
