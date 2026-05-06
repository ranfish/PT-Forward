package model

type ServerConfig struct {
	Host        string   `yaml:"host"         mapstructure:"host"`
	Port        int      `yaml:"port"         mapstructure:"port"`
	Mode        string   `yaml:"mode"         mapstructure:"mode"`
	CORSOrigins []string `yaml:"cors_origins" mapstructure:"cors_origins"`
	APIToken    string   `yaml:"api_token"    mapstructure:"api_token"`
	TLSEnabled  bool     `yaml:"tls_enabled"  mapstructure:"tls_enabled"`
	TLSCert     string   `yaml:"tls_cert"     mapstructure:"tls_cert"`
	TLSKey      string   `yaml:"tls_key"      mapstructure:"tls_key"`
}

type DatabaseConfig struct {
	Driver       string `yaml:"driver"          mapstructure:"driver"`
	SQLitePath   string `yaml:"sqlite_path"     mapstructure:"sqlite_path"`
	MySQLDSN     string `yaml:"mysql_dsn"       mapstructure:"mysql_dsn"`
	MaxOpenConns int    `yaml:"max_open_conns"  mapstructure:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"  mapstructure:"max_idle_conns"`
	LogLevel     string `yaml:"log_level"       mapstructure:"log_level"`
}

type LogConfig struct {
	Directory  string `yaml:"directory"    mapstructure:"directory"`
	Level      string `yaml:"level"        mapstructure:"level"`
	MaxSizeMB  int    `yaml:"max_size_mb"  mapstructure:"max_size_mb"`
	MaxAgeDays int    `yaml:"max_age_days" mapstructure:"max_age_days"`
	MaxBackups int    `yaml:"max_backups"  mapstructure:"max_backups"`
	Compress   bool   `yaml:"compress"     mapstructure:"compress"`
	Format     string `yaml:"format"       mapstructure:"format"`
}

type SecurityConfig struct {
	EncryptionKey     string `yaml:"encryption_key"       mapstructure:"encryption_key"`
	RateLimitEnabled  bool   `yaml:"rate_limit_enabled"   mapstructure:"rate_limit_enabled"`
	RateLimitGlobal   int    `yaml:"rate_limit_global"    mapstructure:"rate_limit_global"`
	RateLimitWrite    int    `yaml:"rate_limit_write"     mapstructure:"rate_limit_write"`
	RateLimitDownload int    `yaml:"rate_limit_download"  mapstructure:"rate_limit_download"`
	LoginMaxRetries   int    `yaml:"login_max_retries"    mapstructure:"login_max_retries"`
	LoginLockoutMin   int    `yaml:"login_lockout_min"    mapstructure:"login_lockout_min"`
}

type NotificationConfig struct {
	Enabled        bool `yaml:"enabled"         mapstructure:"enabled"`
	MaxRetries     int  `yaml:"max_retries"     mapstructure:"max_retries"`
	RetryIntervalS int  `yaml:"retry_interval_s" mapstructure:"retry_interval_s"`
	BatchSize      int  `yaml:"batch_size"      mapstructure:"batch_size"`
}

type DataCleanupConfig struct {
	TorrentEventRetainDays   int `yaml:"torrent_event_retain_days"   mapstructure:"torrent_event_retain_days"`
	PublishResultRetainDays  int `yaml:"publish_result_retain_days"  mapstructure:"publish_result_retain_days"`
	SeenRecordRetainDays     int `yaml:"seen_record_retain_days"     mapstructure:"seen_record_retain_days"`
	NotificationRetainDays   int `yaml:"notification_retain_days"    mapstructure:"notification_retain_days"`
	PublishTaskRetainDays    int `yaml:"publish_task_retain_days"    mapstructure:"publish_task_retain_days"`
	TorrentFileRetainDays    int `yaml:"torrent_file_retain_days"    mapstructure:"torrent_file_retain_days"`
	ReseedMatchRetainDays    int `yaml:"reseed_match_retain_days"    mapstructure:"reseed_match_retain_days"`
	PTGenCacheRetainDays     int `yaml:"ptgen_cache_retain_days"     mapstructure:"ptgen_cache_retain_days"`
	SeedingArchiveRetainDays int `yaml:"seeding_archive_retain_days" mapstructure:"seeding_archive_retain_days"`
	AuditLogRetainDays       int `yaml:"audit_log_retain_days"       mapstructure:"audit_log_retain_days"`
}

type IYUUSection struct {
	Enabled bool   `yaml:"enabled"     mapstructure:"enabled"`
	APIBase string `yaml:"api_base"    mapstructure:"api_base"`
}
