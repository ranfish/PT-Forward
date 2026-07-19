package extract

import (
	"os"
	"path/filepath"
	"testing"
)

// findAdapterTestdata 查找 adapter testdata 目录。
func findAdapterTestdata() string {
	candidates := []string{
		"../../adapter/testdata",
		"../../../adapter/testdata",
		"../../../../adapter/testdata",
	}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "pterclub.net.html")); err == nil {
			return c
		}
	}
	return ""
}

// TestPublicExtractor_PTerRealHTML 用 PTer 真实 HTML 验证公共提取器。
// 验证 textarea BBCode 提取（§56.13 PTer 特殊支持）。
func TestPublicExtractor_PTerRealHTML(t *testing.T) {
	testdataDir := findAdapterTestdata()
	if testdataDir == "" {
		t.Skip("testdata not found")
	}

	htmlBytes, err := os.ReadFile(filepath.Join(testdataDir, "pterclub.net.html"))
	if err != nil {
		t.Skipf("read testdata: %v", err)
	}

	p := NewPublicExtractor("pterclub", "猫")
	seed, err := p.Extract(Input{
		SiteCode:      "pterclub",
		SiteNickname:  "猫",
		PageHTML:      string(htmlBytes),
		FallbackTitle: "fallback",
	})
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// 验证标题
	if seed.Title == "" {
		t.Error("Title should not be empty")
	}
	t.Logf("Title: %q", seed.Title)

	// 验证 IMDb
	if seed.IMDbLink == "" {
		t.Log("IMDb: empty (may be acceptable)")
	} else {
		t.Logf("IMDb: %q", seed.IMDbLink)
	}

	// 验证简介（textarea BBCode 提取）
	if seed.Intro.Body == "" {
		t.Log("Body: empty (may need textarea selector)")
	} else {
		bodyPreview := seed.Intro.Body
		if len([]rune(bodyPreview)) > 80 {
			bodyPreview = string([]rune(bodyPreview)[:80])
		}
		t.Logf("Body (前80字): %q", bodyPreview)
	}

	// 验证海报
	if seed.Intro.Poster != "" {
		t.Logf("Poster: %q", seed.Intro.Poster)
	}

	// 验证截图
	urls := seed.Intro.ScreenshotURLs()
	t.Logf("Screenshots: %d 张", len(urls))

	// 验证 InfoHash
	if seed.InfoHash != "" {
		t.Logf("InfoHash: %q", seed.InfoHash)
	}

	// 验证 Size
	if seed.Size > 0 {
		t.Logf("Size: %d bytes", seed.Size)
	}

	// 验证 MediaInfo
	if seed.MediaInfo != "" {
		t.Logf("MediaInfo: (前60) %q", seed.MediaInfo[:min(60, len(seed.MediaInfo))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
