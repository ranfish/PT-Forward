package adapter

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// 08 刀2 回归锚：baseURLOf 委托 resolveBaseURL 后的端到端等价——
// baseURLOf 的全部消费点（generic:109/nexusphp_upload:57）都经
// BuildNexusDownloadURL 的幂等补协议+去斜杠兜底，helper 层差异被吞掉。
// 验证口径=最终 URL 四态对照（08 §4：对照两函数原始返回值会失败——
// helper 层本就不等价·baseURLOf 曾是裸取旧形态）。
// 防回归场景：BuildNexusDownloadURL 改掉幂等兜底·或 baseURLOf 被
// 新增消费点直接使用（不经 BuildNexus）——任一发生本锚红。
func TestBaseURLOfEndToEndEquivalence(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		domain  string
		wantURL string
	}{
		{"裸 Domain", "", "pt.site.com", "https://pt.site.com/download.php?id=123&passkey=pk"},
		{"含协议 BaseURL", "https://api.site.com", "www.site.com", "https://api.site.com/download.php?id=123&passkey=pk"},
		{"带尾斜杠 BaseURL", "https://api.site.com/", "www.site.com", "https://api.site.com/download.php?id=123&passkey=pk"},
		{"BaseURL 不含协议", "api.site.com", "www.site.com", "https://api.site.com/download.php?id=123&passkey=pk"},
	}
	for _, c := range cases {
		cfg := &model.SiteConfig{BaseURL: c.baseURL, Domain: c.domain}
		got := BuildNexusDownloadURL(baseURLOf(cfg), "123", "pk")
		if got != c.wantURL {
			t.Errorf("%s: got %q, want %q", c.name, got, c.wantURL)
		}
	}
}
