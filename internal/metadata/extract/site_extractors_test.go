package extract

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSiteExtractors_PTer v0.0.254 验证 PTer 配置化提取（替代 sites/pterclub.go）。
// 关键：category_from_icons 模式 + source_keys.medium=质量/source=地区（站点特殊 label）。
func TestSiteExtractors_PTer(t *testing.T) {
	testdataDir := findAdapterTestdata()
	if testdataDir == "" {
		t.Skip("testdata not found")
	}
	htmlBytes, err := os.ReadFile(filepath.Join(testdataDir, "pterclub.net.html"))
	if err != nil {
		t.Skipf("read testdata: %v", err)
	}

	p := NewPublicExtractor("pterclub", "")
	seed, err := p.Extract(Input{
		Domain:       "pterclub.net",
		SiteCode:     "pterclub",
		PageHTML:     string(htmlBytes),
		FallbackTitle: "fallback",
	})
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// 验证关键字段
	if seed.Title == "" {
		t.Error("Title should not be empty")
	}
	t.Logf("Title: %s", seed.Title)
	t.Logf("Type: %s", seed.Type)
	t.Logf("Medium: %s", seed.Medium)
	t.Logf("Source: %s", seed.Source)

	// category_from_icons 应至少填充 type 或 medium
	if seed.Type == "" && seed.Medium == "" {
		t.Error("category_from_icons should fill Type or Medium")
	}
}

// TestSiteExtractors_HHanClub v0.0.254 验证 HHanClub 配置化提取（替代 sites/hhanclub.go）。
// 关键：title_from_quoted + description_from_range + basic_info_div_label + poster_from_pattern。
func TestSiteExtractors_HHanClub(t *testing.T) {
	testdataDir := findAdapterTestdata()
	if testdataDir == "" {
		t.Skip("testdata not found")
	}
	htmlBytes, err := os.ReadFile(filepath.Join(testdataDir, "hhanclub.net.html"))
	if err != nil {
		t.Skipf("read testdata: %v", err)
	}

	p := NewPublicExtractor("hhanclub", "")
	seed, err := p.Extract(Input{
		Domain:       "hhanclub.net",
		SiteCode:     "hhanclub",
		PageHTML:     string(htmlBytes),
		FallbackTitle: "fallback",
	})
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// 验证关键字段
	if seed.Title == "" {
		t.Error("Title should not be empty")
	}
	t.Logf("Title: %s", seed.Title)
	t.Logf("Subtitle: %s", seed.Subtitle)
	t.Logf("Poster: %s", seed.Intro.Poster)
	t.Logf("IMDb: %s", seed.IMDbLink)
	t.Logf("Douban: %s", seed.DoubanLink)
	if seed.Intro.Body != "" {
		t.Logf("Body (前80): %s", truncateStr(seed.Intro.Body, 80))
	}

	// title_from_quoted 应提取带引号的标题
	if seed.Title == "" || seed.Title == "fallback" {
		t.Error("title_from_quoted should extract title")
	}
	t.Logf("Type: %s", seed.Type)
	t.Logf("Medium: %s", seed.Medium)
	// basic_info_div_label (container_label_grid_span) 应至少填充 type/medium/resolution
	if seed.Type == "" && seed.Medium == "" {
		t.Error("basic_info_div_label should fill Type or Medium")
	}
}

func truncateStr(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
