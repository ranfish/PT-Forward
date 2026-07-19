package sites

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ranfish/pt-forward/internal/metadata/extract"
)

// findTestdata 查找 adapter testdata 目录。
func findTestdata() string {
	candidates := []string{
		"../../../adapter/testdata",
		"../../../../adapter/testdata",
	}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "hhanclub.net.html")); err == nil {
			return c
		}
	}
	return ""
}

func TestHHanClub_RealHTML(t *testing.T) {
	testdataDir := findTestdata()
	if testdataDir == "" {
		t.Skip("testdata not found, skipping real HTML test")
	}

	htmlBytes, err := os.ReadFile(filepath.Join(testdataDir, "hhanclub.net.html"))
	if err != nil {
		t.Skipf("read testdata failed: %v", err)
	}
	htmlStr := string(htmlBytes)

	ex := newHHanClubExtractor()
	seed, err := ex.Extract(extract.Input{
		SiteCode:     "hhanclub",
		SiteNickname: "憨憨",
		PageHTML:     htmlStr,
	})
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// 验证标题（从 <title> 引号提取）
	if seed.Title == "" {
		t.Error("Title should not be empty")
	}
	t.Logf("Title: %q", seed.Title)

	// 验证副标题
	t.Logf("Subtitle: %q", seed.Subtitle)

	// 验证海报（l_ratio_poster）
	t.Logf("Poster: %q", seed.Intro.Poster)

	// 验证 IMDb
	t.Logf("IMDb: %q", seed.IMDbLink)

	// 验证豆瓣
	t.Logf("Douban: %q", seed.DoubanLink)

	// 验证简介
	if len(seed.Intro.Body) > 0 {
		t.Logf("Body (前100): %q", seed.Intro.Body[:min(100, len(seed.Intro.Body))])
	} else {
		t.Log("Body: empty")
	}

	// 验证 MediaInfo
	t.Logf("MediaInfo (前80): %q", seed.MediaInfo[:min(80, len(seed.MediaInfo))])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
