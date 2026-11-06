package adapter

import (
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.473: 海胆组行搜索锚——双层链接 torrent_id 提取（altLink 捕组 id 错投）。
func TestParseHaidanGroupRow(t *testing.T) {
	html := `<table><tr>
		<a href="details.php?group_id=55185">Zhizn I Sudba 组</a>
		<a href="details.php?group_id=55185&torrent_id=80385">Zhizn I Sudba 2012 1080p</a>
		<a href="download.php?id=80385&passkey=xxx">dl</a>
		</tr></table>`
	results := parseNexusPHPBrowse(html, &model.SiteConfig{Domain: "www.haidan.cc"})
	if len(results) == 0 {
		t.Fatal("海胆组行应解析出结果")
	}
	if results[0].TorrentID != "80385" {
		t.Errorf("tid = %q, want 80385（torrent_id 双层·非组 id 55185）", results[0].TorrentID)
	}
	if !strings.Contains(results[0].Title, "Zhizn") {
		t.Errorf("Title = %q", results[0].Title)
	}
}
