package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.491: detailURLOf 协议根修锚——裸 Domain 产无协议 URL（前端拼系统地址案）
func TestDetailURLOfProtocol(t *testing.T) {
	cases := []struct {
		domain, tid, want string
	}{
		{"www.musopia.vip", "15141", "https://www.musopia.vip/details.php?id=15141"},
		{"https://www.musopia.vip/", "25174", "https://www.musopia.vip/details.php?id=25174"},
		{"hdhome.org", "", ""}, // 空 tid 短路
	}
	for _, c := range cases {
		got := detailURLOf(&model.SiteConfig{Domain: c.domain}, c.tid)
		if got != c.want {
			t.Fatalf("detailURLOf(%q,%q)=%q want %q", c.domain, c.tid, got, c.want)
		}
	}
}
