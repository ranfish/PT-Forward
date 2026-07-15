package util

import (
	"bytes"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestSanitizerCore_MasksSensitiveFields(t *testing.T) {
	var buf bytes.Buffer
	encoder := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	core := zapcore.NewCore(encoder, zapcore.AddSync(&buf), zap.DebugLevel)
	sanitizer := NewSanitizerCore(core)
	logger := zap.New(sanitizer)

	tests := []struct {
		name  string
		input string
	}{
		{"passkey", "passkey=abc123secret"},
		{"cookie", "cookie=sid_abc123"},
		{"password", "password=MyS3cr3t!"},
		{"api_key", "api_key=key123"},
		{"apikey", "apikey=key456"},
		{"bearer_token", "bearer_token=bt_xyz"},
		{"encryption_key", "encryption_key=enc_abc"},
		{"rsskey", "rsskey=rss_abc"},
		{"rss_key", "rss_key=rss_def"},
		{"authkey", "authkey=auth_abc"},
		{"auth_key", "auth_key=auth_def"},
		{"secret", "secret=s3cr3t"},
		{"token", "token=tok_abc123"},
		{"url_with_passkey", "https://site.com/download.php?id=123&passkey=abc123def"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			logger.Info("test", zap.String("data", tt.input))
			output := buf.String()
			if containsAny(output, tt.input) {
				t.Errorf("output contains sensitive data: %s", output)
			}
		})
	}
}

func TestSanitizerCore_PassesNormalStrings(t *testing.T) {
	var buf bytes.Buffer
	encoder := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	core := zapcore.NewCore(encoder, zapcore.AddSync(&buf), zap.DebugLevel)
	sanitizer := NewSanitizerCore(core)
	logger := zap.New(sanitizer)

	logger.Info("test", zap.String("data", "normal log message"))
	output := buf.String()
	if !containsAny(output, "normal log message") {
		t.Errorf("normal text should pass through: %s", output)
	}
}

func TestSanitizerCore_Patterns(t *testing.T) {
	if len(defaultSensitivePatterns) == 0 {
		t.Fatal("defaultSensitivePatterns should not be empty")
	}

	keywords := []string{"passkey", "cookie", "api_key", "apikey", "bearer_token", "password", "encryption_key", "rsskey", "rss_key", "authkey", "auth_key", "secret", "token"}
	for _, kw := range keywords {
		found := false
		for _, p := range defaultSensitivePatterns {
			if p.MatchString(kw + "=test123") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no pattern matches %q", kw)
		}
	}
}

func containsAny(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
