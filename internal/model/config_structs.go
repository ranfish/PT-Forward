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
	EncryptionKey  string   `yaml:"encryption_key"       mapstructure:"encryption_key"`
	TrustedProxies []string `yaml:"trusted_proxies"      mapstructure:"trusted_proxies"`
}
