package sites

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/metadata/extract"
)

func TestNewSpecialExtractors_Count(t *testing.T) {
	specials := NewSpecialExtractors()
	expected := []string{
		"pterclub", "hdsky", "hhanclub", "ssd",
		"keepfrds", "audiences", "hddolby", "chdbits",
	}
	if len(specials) != len(expected) {
		t.Fatalf("expected %d specials, got %d", len(expected), len(specials))
	}
	for _, code := range expected {
		if _, ok := specials[code]; !ok {
			t.Errorf("missing site_code %q", code)
		}
	}
}

func TestSpecialExtractors_Names(t *testing.T) {
	specials := NewSpecialExtractors()
	expectedNames := map[string]string{
		"pterclub":  "pterclub_special",
		"hdsky":     "hdsky_special",
		"hhanclub":  "hhanclub_special",
		"ssd":       "ssd_special",
		"keepfrds":  "keepfrds_special",
		"audiences": "audiences_special",
		"hddolby":   "hddolby_special",
		"chdbits":   "chdbits_special",
	}
	for code, wantName := range expectedNames {
		ex, ok := specials[code]
		if !ok {
			t.Errorf("missing %q", code)
			continue
		}
		if got := ex.Name(); got != wantName {
			t.Errorf("site %q Name() = %q, want %q", code, got, wantName)
		}
	}
}

func TestSpecialExtractors_ImplementsInterface(t *testing.T) {
	specials := NewSpecialExtractors()
	for code, ex := range specials {
		var _ extract.Extractor = ex
		_ = code
	}
}

// TestSpecialExtractor_DelegatesToPublic 验证 stub 能正确委托给公共提取器。
func TestSpecialExtractor_DelegatesToPublic(t *testing.T) {
	html := `<html><body>
<h1 id="top">测试电影 2024</h1>
<table><tr><th>大小</th><td>1.5 GB</td></tr></table>
<div id="kdescr"><p>简介正文</p></div>
</body></html>`

	specials := NewSpecialExtractors()
	for code, ex := range specials {
		seed, err := ex.Extract(extract.Input{
			SiteCode:      code,
			SiteNickname:  code,
			PageHTML:      html,
			FallbackTitle: "fallback",
		})
		if err != nil {
			t.Errorf("site %q Extract failed: %v", code, err)
			continue
		}
		if !seed.IsMeaningful() {
			t.Errorf("site %q: seed should be meaningful", code)
		}
		if seed.Title == "" {
			t.Errorf("site %q: Title should not be empty", code)
		}
	}
}

func TestSpecialExtractor_EmptyHTML(t *testing.T) {
	specials := NewSpecialExtractors()
	for code, ex := range specials {
		_, err := ex.Extract(extract.Input{
			SiteCode: code,
			PageHTML: "",
		})
		if err == nil {
			t.Errorf("site %q: expected error for empty HTML", code)
		}
	}
}

// TestEngine_RegistersSpecials 验证 Engine 能正确注册和路由特殊提取器。
func TestEngine_RegistersSpecials(t *testing.T) {
	public := extract.NewPublicExtractor("", "")
	specials := NewSpecialExtractors()
	engine := extract.NewEngine(public, specials)

	// 用 pterclub 触发特殊提取器路由
	seed, meta := engine.Extract(extract.Input{
		SiteCode:     "pterclub",
		SiteNickname: "猫",
		PageHTML:     `<html><body><h1 id="top">标题</h1></body></html>`,
	})
	if !seed.IsMeaningful() {
		t.Error("seed should be meaningful")
	}
	// 因为 stub 直接委托给公共提取器，ExtractorName 应该是 "pterclub_special"
	if meta.ExtractorName != "pterclub_special" {
		t.Errorf("expected pterclub_special, got %q", meta.ExtractorName)
	}
	if meta.UsedFallback {
		t.Error("should not use fallback (special succeeded)")
	}
}

func TestEngine_UnknownSite_UsesPublic(t *testing.T) {
	public := extract.NewPublicExtractor("", "")
	specials := NewSpecialExtractors()
	engine := extract.NewEngine(public, specials)

	seed, meta := engine.Extract(extract.Input{
		SiteCode:     "unknown-site",
		SiteNickname: "未知站",
		PageHTML:     `<html><body><h1 id="top">标题</h1></body></html>`,
	})
	if !seed.IsMeaningful() {
		t.Error("seed should be meaningful")
	}
	// 无 special → 直接用 public（UsedFallback=false）
	if meta.ExtractorName != "public" {
		t.Errorf("expected public, got %q", meta.ExtractorName)
	}
	if meta.UsedFallback {
		t.Error("should not use fallback when no special found")
	}
}
