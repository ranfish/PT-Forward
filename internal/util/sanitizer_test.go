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

// §59.262: 时间量词字符集补"月"——PT地带 50% 促销 51 天倒计时形态"1月21天"
// （NexusPHP >30 天促销渲染为 N月M天），QHstudIo 243 案族三失配致标题尾巴残留。
func TestStripSiteOperationMarkers_MonthTimeTail(t *testing.T) {
	cases := []struct{ in, want string }{
		{"The First Jasmine 2026 S01E17-E18 1080p WEB-DL HEVC AAC-QHstudIo    [50%] 剩余时间：1月21天",
			"The First Jasmine 2026 S01E17-E18 1080p WEB-DL HEVC AAC-QHstudIo"},
		{"Some Title 2020 1080p BluRay x264-GRP 剩余时间：2月", "Some Title 2020 1080p BluRay x264-GRP"},
		{"Some Title 2020 1080p BluRay x264-GRP [免费] 剩余时间：10天5时", "Some Title 2020 1080p BluRay x264-GRP"},
	}
	for _, tc := range cases {
		if got := StripSiteOperationMarkers(tc.in); got != tc.want {
			t.Errorf("in=%q got=%q want=%q", tc.in, got, tc.want)
		}
	}
}

// §59.283: 时间量词补"钟"——"分钟"词尾（UBWEB 系脏尾 "-UBWEB    [免费]
// 剩余时间：2时28分钟 (通过)" 243 PT0 4011 行实证：charset 有分无钟击穿族三/族四）
func TestStripSiteOperationMarkers_ZhongTail(t *testing.T) {
	cases := []struct{ in, want string }{
		{"BRAT 2025 1080P AMZN WEB-DL H.264 DDP2.0 5Audio-SHB931@UBWEB    [免费] 剩余时间：2时28分钟 (通过)",
			"BRAT 2025 1080P AMZN WEB-DL H.264 DDP2.0 5Audio-SHB931@UBWEB"},
		{"96 2018 HINDI 1080P AMZN WEB-DL H265 DDP5.1-SHB931@UBWEB    [免费] 剩余时间：21时52分钟 (通过)",
			"96 2018 HINDI 1080P AMZN WEB-DL H265 DDP5.1-SHB931@UBWEB"},
		{"Some.Title.2020.1080p.x264-GRP 剩余时间：3分钟", "Some.Title.2020.1080p.x264-GRP"},
	}
	for _, tc := range cases {
		if got := StripSiteOperationMarkers(tc.in); got != tc.want {
			t.Errorf("in=%q got=%q want=%q", tc.in, got, tc.want)
		}
	}
}

// §59.284 族五: 锚后全剥兜底——形态判据反转（标题词视角），新造标注词免疫
func TestStripSiteOperationMarkers_AnchorFallback(t *testing.T) {
	cases := []struct{ in, want string }{
		// 新造量词（未来变体免疫——钟/月之后的任何词）
		{"Movie.Name.2020.1080p.x264-GRP    [免费] 剩余时间：2兆13秒", "Movie.Name.2020.1080p.x264-GRP"},
		// 禁转括号可剥（flags 通道单责——§59.136 折中由 §59.284 模型取代）
		{"Movie.Name.2020.1080p.x264-GRP [禁转]", "Movie.Name.2020.1080p.x264-GRP"},
		// UBWEB 实案形态
		{"BRAT 2025 1080P AMZN WEB-DL H.264 DDP2.0 5Audio-SHB931@UBWEB    [免费] 剩余时间：2时28分钟 (通过)",
			"BRAT 2025 1080P AMZN WEB-DL H.264 DDP2.0 5Audio-SHB931@UBWEB"},
		// 保险丝：锚后含括号外标题词（3+ 字母）不剥
		{"Movie.Name.2020.1080p.x264-GRP.Extended.Cut", "Movie.Name.2020.1080p.x264-GRP.Extended.Cut"},
		// 纯数字尾（续集 "-2"）不锚——无字母 token 跳过
		{"Wendy.Williams.The.Movie-2.2011.1080p", "Wendy.Williams.The.Movie-2.2011.1080p"},
		// NOGROUP 锚
		{"Some.Show.S01.2020.1080p.WEB-DL.x264-NOGROUP [促销]", "Some.Show.S01.2020.1080p.WEB-DL.x264-NOGROUP"},
		// 组名即末尾——原样
		{"Clean.Title.2020.1080p.x264-GRP", "Clean.Title.2020.1080p.x264-GRP"},
	}
	for _, tc := range cases {
		if got := StripSiteOperationMarkers(tc.in); got != tc.want {
			t.Errorf("in=%q\n got=%q\nwant=%q", tc.in, got, tc.want)
		}
	}
}
