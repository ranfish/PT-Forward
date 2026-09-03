package config

import (
	"os"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func Load(configPath string, logger *zap.Logger) (*Config, error) {
	cfg := DefaultConfig()

	v := viper.New()
	v.SetEnvPrefix("PTF")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			if _, ok := err.(*os.PathError); !ok {
				return nil, configError(ErrConfigLoad, "read config", err)
			}
			logger.Warn("config file not found, using defaults", zap.String("path", configPath))
		}
	}

	if err := v.Unmarshal(cfg); err != nil {
		return nil, configError(ErrConfigLoad, "unmarshal config", err)
	}

	// §59.167 viper AutomaticEnv 对嵌套 key（log.level → PTF_LOG_LEVEL）在
	// Unmarshal 时不生效（PT31 实战：Dockerfile ENV PTF_LOG_LEVEL=error 无效
	// ——info 全量输出）。显式 BindEnv 后重读。
	_ = v.BindEnv("log.level", "PTF_LOG_LEVEL")
	_ = v.BindEnv("database.log_level", "PTF_DATABASE_LOGLEVEL")
	if lv := v.GetString("log.level"); lv != "" {
		cfg.Log.Level = lv
	}
	if dl := v.GetString("database.log_level"); dl != "" {
		cfg.Database.LogLevel = dl
	}

	if err := cfg.Validate(); err != nil {
		return nil, configError(ErrConfigValidate, "validate config", err)
	}

	return cfg, nil
}
