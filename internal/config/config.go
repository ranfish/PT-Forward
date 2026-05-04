package config

import (
	"fmt"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
)

type Config struct {
	Server       model.ServerConfig       `yaml:"server" mapstructure:"server"`
	Database     model.DatabaseConfig     `yaml:"database" mapstructure:"database"`
	Log          model.LogConfig          `yaml:"log" mapstructure:"log"`
	Scheduler    model.SchedulerConfig    `yaml:"scheduler" mapstructure:"scheduler"`
	Notification model.NotificationConfig `yaml:"notification" mapstructure:"notification"`
	Security     model.SecurityConfig     `yaml:"security" mapstructure:"security"`
	CookieCloud  model.CookieCloudConfig  `yaml:"cookiecloud" mapstructure:"cookiecloud"`
	SiteMonitor  model.SiteMonitorConfig  `yaml:"site_monitor" mapstructure:"site_monitor"`
	DataCleanup  model.DataCleanupConfig  `yaml:"data_cleanup" mapstructure:"data_cleanup"`
	Stats        model.StatsConfig        `yaml:"stats" mapstructure:"stats"`
	IYUU         model.IYUUSection        `yaml:"iyuu" mapstructure:"iyuu"`
	Memory       model.MemoryConfig       `yaml:"memory" mapstructure:"memory"`
}

func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be 1-65535, got %d", c.Server.Port)
	}
	if c.Server.Mode != "debug" && c.Server.Mode != "release" {
		return fmt.Errorf("server.mode must be 'debug' or 'release', got %q", c.Server.Mode)
	}
	if c.Database.Driver != "sqlite" && c.Database.Driver != "mysql" {
		return fmt.Errorf("database.driver must be 'sqlite' or 'mysql', got %q", c.Database.Driver)
	}
	if c.Database.Driver == "mysql" && c.Database.MySQLDSN == "" {
		return fmt.Errorf("database.mysql_dsn is required when driver is mysql")
	}
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[strings.ToLower(c.Log.Level)] {
		return fmt.Errorf("log.level must be debug/info/warn/error, got %q", c.Log.Level)
	}
	if c.Security.LoginMaxRetries < 1 {
		return fmt.Errorf("security.login_max_retries must be >= 1, got %d", c.Security.LoginMaxRetries)
	}
	if c.Security.LoginLockoutMin < 1 {
		return fmt.Errorf("security.login_lockout_min must be >= 1, got %d", c.Security.LoginLockoutMin)
	}
	if c.Memory.MaxTotalMB < 0 {
		return fmt.Errorf("memory.max_total_mb must be >= 0, got %d", c.Memory.MaxTotalMB)
	}
	if c.Memory.WarnPercent <= 0 || c.Memory.WarnPercent > 1 {
		return fmt.Errorf("memory.warn_percent must be 0-1, got %f", c.Memory.WarnPercent)
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
		Scheduler: model.SchedulerConfig{
			MaxWorkers:     3,
			TaskTimeoutMin: 10,
			HTTPTimoutSec:  30,
		},
		Security: model.SecurityConfig{
			RateLimitEnabled:  true,
			RateLimitGlobal:   100,
			RateLimitWrite:    30,
			RateLimitDownload: 10,
			LoginMaxRetries:   5,
			LoginLockoutMin:   5,
		},
		Memory: model.MemoryConfig{
			MaxTotalMB:  256,
			WarnPercent: 0.7,
		},
	}
}
