package extract

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)


// §59.510 hdwing（瓷器）：site-key 优先层——"格式"显式配置 resolution
// 压过 default medium（SSD/碟粉语义不破坏）。
func TestMatchFieldWithLabels_SiteKeyPriority(t *testing.T) {
	labels := buildFieldLabels("hdwing.org", "hdwing")
	// hdwing: "格式"应投 resolution（site key 首位优先）
	if f := matchFieldWithLabels("格式", labels); f != "resolution" {
		t.Errorf("hdwing 格式应=resolution（site key 优先），实得 %s", f)
	}
	// 无 site key 站（SSD 形态）："格式"仍走 default→medium
	labelsSSD := buildFieldLabels("pt.soulvoice.club", "ssd")
	if f := matchFieldWithLabels("格式", labelsSSD); f != "medium" {
		t.Errorf("SSD 形态 格式应=medium（default 层），实得 %s", f)
	}
}

// §59.510 hdwing li 聚合结构提取（模式 1.5）。
func TestBasicInfoLiStructureHdwing(t *testing.T) {
	html := `<html><body><ul>
	  <li class="left">基本信息</li>
	  <li class="right bspace"><i><b>类型：</b>体育节目(Sports)</i><i><b>媒介：</b>HDTV</i><i><b>编码：</b>x265/HEVC</i><i><b>音频编码：</b>AAC</i><i><b>格式：</b>4K</i><br /><i><b>大小：</b>799.12 GB</i><i><b>制作组：</b>HDCTV</i></li>
	</ul></body></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	p := &PublicExtractor{}
	seed := &SeedData{}
	p.fillBasicInfoFieldsWithCode(doc, seed, "hdwing.org", "hdwing")
	if seed.Type != "category.sports" { // standard_keys 映射生效（原始值 体育节目(Sports)）
		t.Errorf("category 实得 %q", seed.Type)
	}
	if seed.Medium != "medium.hdtv" { // 同上（原始值 HDTV）
		t.Errorf("medium 实得 %q", seed.Medium)
	}
	if seed.VideoCodec != "video.h265" { // 同上（原始值 x265/HEVC）
		t.Errorf("video_codec 实得 %q", seed.VideoCodec)
	}
	// resolution/team 在 fillBasicInfoFieldsWithCode 内即 standardize
	// （hdwing.json standard_keys 映射生效——格式 4K→r2160p/HDCTV→team.hdctv）
	if seed.Resolution != "resolution.r2160p" {
		t.Errorf("resolution（格式）实得 %q", seed.Resolution)
	}
	if seed.ReleaseGroup != "team.hdctv" {
		t.Errorf("team 实得 %q", seed.ReleaseGroup)
	}
}
