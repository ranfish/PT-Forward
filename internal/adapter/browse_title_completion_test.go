package adapter

import (
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.182: 优堡 @ 子组署名（-DIY@UBits）——hasGroupSuffix 必须识别，
// 否则 §59.26 补全误触发，中文副标题覆盖英文主标题（Just Mercy tid=109323 实证）。
func TestHasGroupSuffix_AtSubGroup(t *testing.T) {
	cases := []struct {
		title string
		want  bool
	}{
		{"Just Mercy 2019 2160p UHD Blu-ray DoVi HDR10 HEVC Atmos TrueHD 7.1-DIY@UBits", true},
		{"Just Mercy 2019 2160p UHD Blu-ray HEVC Atmos TrueHD7.1-DiY@HDHome", true},
		{"Snatch.2000.UHD.BluRay.2160p.DV.HDR.HEVC.TrueHD.5.1-mUHD-FRDS", true},
		{"Just Mercy 2019 2160p UHD BluRay REMUX DoVi HDR10 HEVC Atmos TrueHD 7.1-UBits", true},
		{"全程战课示威者中字超帅4k杜比视界版", false},
		{"Just Mercy 2019 2160p UHD", false},
	}
	for _, c := range cases {
		if got := hasGroupSuffix(c.title); got != c.want {
			t.Errorf("hasGroupSuffix(%q) = %v, want %v", c.title, got, c.want)
		}
	}
}

// §59.182: 优堡搜索结果——<a> 内英文主标题含 @ 子组后缀，</a> 后是中文副标题+标签。
// 修复前：hasGroupSuffix("@组")=false → 补全用中文副标题（含 "4K UHD" 命中分辨率词）覆盖英文。
// 修复后：英文主标题保留。
func TestParseNexusPHPBrowse_UbitsAtGroupKeepsEnglishTitle(t *testing.T) {
	html := `<table><tr>` +
		`<td class="rowfollow nowrap"><a href="torrents.php?cat=401"><img src="pic/cattrans.gif" alt="电影" /></a></td>` +
		`<td class="rowfollow" width="100%" align="left" style='padding: 0px'><table class="torrentname" width="100%"><tr><td class="embedded" style='padding-left: 5px'>` +
		`<a title="Just Mercy 2019 2160p UHD Blu-ray DoVi HDR10 HEVC Atmos TrueHD 7.1-DIY@UBits"  href="details.php?id=109323&amp;hit=1"><b>Just Mercy 2019 2160p UHD Blu-ray DoVi HDR10 HEVC Atmos TrueHD 7.1-DIY@UBits</b></a>` +
		` <b>[<font class='hot'>热门</font>]</b><span title="通过"></span><br />【DIY 原盘 00884】正义的慈悲 / 以公义之名(港) 4K UHD 原盘 DIY 简体/繁体/简英双语 保留杜比视界<br>` +
		`<a href="?team1=1"><span class="tag">UBits</span></a><a href="?tag_id3=1"><span class="tag">官方</span></a><a href="?tag_id4=1"><span class="tag">DIY</span></a><a href="?tag_id8=1"><span class="tag">杜比视界</span></a>` +
		`</td></tr></table></td>` +
		`<td class="rowfollow">2024-08-25</td><td class="rowfollow">67.65<br />GB</td>` +
		`<td class="rowfollow" align="center"><b><a href="details.php?id=109323&amp;hit=1&amp;dllist=1#seeders">13</a></b></td>` +
		`</tr></table>`

	results := parseNexusPHPBrowse(html, &model.SiteConfig{Domain: "https://ubits.club"})
	if len(results) != 1 {
		t.Fatalf("expect 1 result, got %d: %+v", len(results), results)
	}
	r := results[0]
	if r.TorrentID != "109323" {
		t.Fatalf("tid = %s, want 109323", r.TorrentID)
	}
	if !strings.Contains(r.Title, "Just Mercy 2019 2160p UHD Blu-ray DoVi") || !strings.HasSuffix(r.Title, "-DIY@UBits") {
		t.Fatalf("title should keep English main title, got %q", r.Title)
	}
	if strings.Contains(r.Title, "正义的慈悲") {
		t.Fatalf("title must not contain Chinese subtitle, got %q", r.Title)
	}
	if r.Size != 72638634393 { // 67.65 GB
		t.Fatalf("size = %d, want 72638634393", r.Size)
	}
}

// §59.26 回归：keepfrds 场景——<a> 内只有中文格式化标题，原始英文名（含 -FRDS）在 </a> 后。
// 补全必须继续工作。
func TestParseNexusPHPBrowse_KeepfrdsCompletionStillWorks(t *testing.T) {
	html := `<table><tr>` +
		`<td class="rowfollow" width="100%" align="left" style='padding: 0px'><table class="torrentname" width="100%"><tr><td class="embedded" style='padding-left: 5px'>` +
		`<a href="details.php?id=2782311&amp;hit=1"><b>全程战课示威者中字超帅4k杜比视界版</b></a>` +
		`<br />Snatch.2000.UHD.BluRay.2160p.DV.HDR.HEVC.TrueHD.5.1-mUHD-FRDS` + "\n" +
		`</td></tr></table></td>` + "\n" +
		`<td class="rowfollow">55.5<br />GB</td>` + "\n" +
		`</tr></table>`

	results := parseNexusPHPBrowse(html, &model.SiteConfig{Domain: "https://keepfrds.com"})
	if len(results) != 1 {
		t.Fatalf("expect 1 result, got %d: %+v", len(results), results)
	}
	r := results[0]
	if !strings.HasSuffix(strings.TrimSpace(r.Title), "-FRDS") {
		t.Fatalf("completion should capture English original name ending -FRDS, got %q", r.Title)
	}
}
