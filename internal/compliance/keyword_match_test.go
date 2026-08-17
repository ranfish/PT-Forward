package compliance

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// §56.2x 修正: 关键词层防误伤——ASCII 词边界 + 中文误报排除。
// 案例：The Pornographers（人類学入門，今村昌平 1966）曾被 "Porn" 子串误标系统禁转。
func TestKeywordMatchWordBoundary(t *testing.T) {
	cases := []struct {
		name, kw, text string
		want           bool
	}{
		// ASCII 词边界
		{"词中不命中", "Porn", "The Pornographers 1966", false},
		{"独立词命中", "Porn", "Some Porn Movie 2024", true},
		{"点分隔命中", "Porn", "Movie.Porn.2024", true},
		{"连字符分隔命中", "Porn", "Movie-Porn-2024", true},
		{"行首命中", "Porn", "Porn Movie 2024", true},
		{"行尾命中", "Porn", "Movie 2024 Porn", true},
		{"连写规避不命中", "Porn", "Porn4K Site", false},
		{"数字粘连不命中", "Adult", "Movie Adult2024", false},
		{"大小写不敏感", "PORN", "some porn movie", true},
		// 中文子串（保持）
		{"中文子串命中", "无码", "无码中文字幕", true},
		{"中文不命中", "无码", "普通电影", false},
		// 空值
		{"空关键词", "", "any", false},
		{"空文本", "Porn", "", false},
	}
	for _, c := range cases {
		if got := matchKeyword(c.kw, c.text, c.text); got != c.want {
			t.Errorf("%s: matchKeyword(%q, %q) = %v, want %v", c.name, c.kw, c.text, got, c.want)
		}
	}
}

// §56.2x: 中文误报排除（设计稿 chineseFalsePositivePatterns 补实现）
func TestChineseFalsePositiveExclusion(t *testing.T) {
	cases := []struct {
		name, kw, text string
		want           bool
	}{
		{"成人教育放行", "成人", "成人高考辅导课程", true},
		{"成人教育放行2", "成人", "成人教育学院", true},
		{"真成人内容不排除", "成人", "成人影片精选", false},
		{"其他关键词不触发", "无码", "成人高考辅导", false},
	}
	for _, c := range cases {
		if got := isChineseFalsePositive(c.text, c.kw); got != c.want {
			t.Errorf("%s: isChineseFalsePositive(%q, %q) = %v, want %v", c.name, c.text, c.kw, got, c.want)
		}
	}
}

// §56.2x: 端到端锚定——真实 DB 规则（含 builtin "Porn"）下的判定
func TestPornographersEndToEnd(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("/home/incast/PT-Forward/data/pt-forward.db?mode=ro"), &gorm.Config{})
	if err != nil {
		t.Skipf("无 29 环境 DB: %v", err)
	}
	_ = model.ComplianceRule{}
	c := NewChecker(db, zap.NewNop())
	ctx := context.Background()

	title := "Erogotoshi-tachi yori: Jinruigaku nyûmon AKA The Pornographers 1966 UHD BluRay 1080p x265 10bit FLAC MNHD-FRDS"
	if r := c.CheckWithSite(ctx, title, "朋友"); !r.Passed {
		t.Errorf("Pornographers 应放行: cat=%q reason=%q", r.Category, r.Reason)
	}
	if r := c.Check(ctx, "Some Porn Movie 2024 1080p"); r.Passed {
		t.Error("真成人标题应拦截（词边界仍命中独立词 Porn）")
	}
	if r := c.Check(ctx, "成人高考辅导课程"); !r.Passed {
		t.Errorf("成人高考应放行（中文误报排除）: cat=%q reason=%q", r.Category, r.Reason)
	}
	if r := c.Check(ctx, "禁转资源 Movie 2024"); r.Passed {
		t.Error("禁转关键词应拦截")
	}
}

