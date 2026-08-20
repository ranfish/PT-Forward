package extract

import (
	"os"
	"strings"
	"testing"
)

// §59.48 端到端调试：Intro.Screenshots 空 → ScreenshotURLs 空 → 落库截图哪来的？
func TestKFExpandE2E(t *testing.T) {
	html, err := os.ReadFile("/tmp/opencode/kf_waywetalk.html")
	if err != nil {
		t.Skip("无页面文件")
	}
	engine := NewEngine(NewPublicExtractor("pt.keepfrds.com", "朋友"), nil)
	input := Input{
		SiteCode:     "keepfrds",
		Domain:       "pt.keepfrds.com",
		SiteNickname: "朋友",
		TorrentID:    "2782310",
		PageHTML:     string(html),
	}
	seed, meta := engine.Extract(input)
	t.Logf("extractor=%s fallback=%v", meta.ExtractorName, meta.UsedFallback)
	t.Logf("Intro.Screenshots 长度=%d Body 长度=%d", len(seed.Intro.Screenshots), len(seed.Intro.Body))
	// 如果 Intro.Screenshots 空，看 body 里是否有 [img]
	if seed.Intro.Screenshots == "" {
		t.Logf("Body 含 [img]: %v", strings.Contains(seed.Intro.Body, "[img]"))
		i := strings.Index(seed.Intro.Body, "[img]")
		if i >= 0 {
			t.Logf("Body img 头: %s", seed.Intro.Body[i:i+150])
		}
	}
	detail := SeedToDetail(seed)
	t.Logf("Detail.Screenshots 数=%d", len(detail.Screenshots))
	if len(detail.Screenshots) > 0 {
		t.Logf("Detail.Screenshots[0]: %s", detail.Screenshots[0][:110])
		if strings.Contains(detail.Screenshots[0], "resize:fill") {
			t.Log("❌ 未展开")
		} else {
			t.Log("✓ 已展开")
		}
	}
}
