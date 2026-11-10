package extract

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)


// §59.510 hdwing：<i><b>大小：</b>799.12 GB</i> 结构（li 聚合基本信息区）。
// 修前：三结构 miss → fallback 正则错抓页面首个"数字+单位"（用户上传量
// 57.553 TB·size 错 70 倍）。
func TestExtractSizeHdwingBoldKey(t *testing.T) {
	html := `<html><body>
	  <p>分享率：13.652 上传量：57.553 TB 下载量： 4.216 TB</p>
	  <ul><li class="left">基本信息</li><li class="right bspace"><i><b>类型：</b>体育节目(Sports)</i><i><b>大小：</b>799.12 GB</i></li></ul>
	</body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	p := &PublicExtractor{}
	got := p.extractSize(doc, html)
	fwant := 799.12 * (1 << 30)
	want := int64(fwant)
	if got != want {
		t.Errorf("size 实得 %d 期望 %d（上传量 57.553TB 不应被错抓）", got, want)
	}
}
