package adapter

import (
	"os"
	"strings"
	"testing"
)


// §59.332: keepfrds 新版（2026-10-05 tailwind 改版）统计提取——
// 首页实录样本锚：做种数 title 形态 + 新版 span 上传量。
func TestKeepfrdsNewFrontendStats(t *testing.T) {
	data, err := os.ReadFile("testdata/keepfrds_index_new.html")
	if err != nil {
		t.Skip("testdata missing")
	}
	html := string(data)
	// 做种数（730——title="当前做种" 图标后数字）
	m := reKfNewSeedingCount.FindStringSubmatch(html)
	if len(m) < 2 || m[1] != "730" {
		t.Fatalf("新版做种数应提取 730, got %v", m)
	}
	// 上传量（392.107 TiB——span.uploaded）
	m2 := reKfNewUploaded.FindStringSubmatch(html)
	if len(m2) < 2 || !strings.Contains(m2[1], "392.107") {
		t.Fatalf("新版上传量应提取 392.107 TiB, got %v", m2)
	}
}
