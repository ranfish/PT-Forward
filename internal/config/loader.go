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

	if err := cfg.Validate(); err != nil {
		return nil, configError(ErrConfigValidate, "validate config", err)
	}

	return cfg, nil
}
