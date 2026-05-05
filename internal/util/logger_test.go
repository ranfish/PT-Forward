package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitLogger_JSON(t *testing.T) {
	logger, err := InitLogger(LogConfig{
		Level:  "info",
		Format: "json",
	})
	if err != nil {
		t.Fatal(err)
	}
	if logger == nil {
		t.Fatal("logger should not be nil")
	}
	_ = logger.Sync()
}

func TestInitLogger_Console(t *testing.T) {
	logger, err := InitLogger(LogConfig{
		Level:  "debug",
		Format: "console",
	})
	if err != nil {
		t.Fatal(err)
	}
	if logger == nil {
		t.Fatal("logger should not be nil")
	}
	_ = logger.Sync()
}

func TestInitLogger_InvalidLevel(t *testing.T) {
	logger, err := InitLogger(LogConfig{
		Level:  "invalid_level",
		Format: "json",
	})
	if err != nil {
		t.Fatal(err)
	}
	if logger == nil {
		t.Fatal("should fallback to info level")
	}
	_ = logger.Sync()
}

func TestInitLogger_WithFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "logs")
	logger, err := InitLogger(LogConfig{
		Level:     "info",
		Format:    "json",
		Directory: dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	logger.Info("test message")
	_ = logger.Sync()

	logFile := filepath.Join(dir, "pt-forward.log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Fatal("log file should exist")
	}
}
