package util

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type LogConfig struct {
	Directory  string
	Level      string
	MaxSizeMB  int
	MaxAgeDays int
	MaxBackups int
	Compress   bool
	Format     string
}

func InitLogger(cfg LogConfig) (*zap.Logger, error) {
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		level = zap.InfoLevel
	}

	atomicLevel := zap.NewAtomicLevelAt(level)

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "ts"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	var encoder zapcore.Encoder
	if cfg.Format == "console" {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	var cores []zapcore.Core

	consoleCore := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		atomicLevel,
	)
	cores = append(cores, NewSanitizerCore(consoleCore))

	if cfg.Directory != "" {
		if err := os.MkdirAll(cfg.Directory, 0755); err == nil {
			if f, err := os.OpenFile(cfg.Directory+"/pt-forward.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
				fileCore := zapcore.NewCore(
					zapcore.NewJSONEncoder(encoderConfig),
					zapcore.AddSync(f),
					atomicLevel,
				)
				cores = append(cores, NewSanitizerCore(fileCore))
			}
		}
	}

	core := zapcore.NewTee(cores...)
	return zap.New(core, zap.AddStacktrace(zap.ErrorLevel)), nil
}
