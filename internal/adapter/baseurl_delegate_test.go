package adapter

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// 08 刀2 回归锚：baseURLOf 委托 resolveBaseURL 后的最终 URL 快照
// 锁定（四态对照·08 §4 验证口径）。
// §59.578 附·08 回归审核修正（用户实证）：委托后 baseURLOf 输出
// 已全部规范化（https:// 开头·无尾斜杠）——BuildNexus 兜底在本链
// 成死路径（幂等分支恒不触发）。本锚锁定=最终 URL 快照一致·
// 【不】承诺防「BuildNexus 兜底被改」（那由
// TestBuildNexusDownloadURL_Idempotence 独立锚锁——兜底删掉则
// 裸域输入不补·幂等锚红）·也【不】承诺防「新增消费点」——委托后
// baseURLOf 返回规范 URL·任何消费点安全（附录G §2.2 选用规则
// 已同步更新·新增消费点无需评估）。
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

// §59.578 附·08 回归审核补锚：BuildNexusDownloadURL 幂等契约——
// 裸域 vs 已规范域输入产出同一 URL。幂等补协议+去斜杠是公共契约
// （baseURLOf 委托链的历史兜底层·也是未来任何裸取调用方的兜底）——
// 兜底被删则本锚红（裸域输入不再补协议）。与四态快照锚分工：
// 本锚锁 BuildNexus 自身特性·快照锚锁 baseURLOf 委托行为。
func TestBuildNexusDownloadURL_Idempotence(t *testing.T) {
	bare := BuildNexusDownloadURL("pt.site.com", "1", "pk")
	norm := BuildNexusDownloadURL("https://pt.site.com", "1", "pk")
	trail := BuildNexusDownloadURL("https://pt.site.com/", "1", "pk")
	if bare != norm {
		t.Errorf("幂等补协议失效：裸域 %q vs 规范域 %q", bare, norm)
	}
	if trail != norm {
		t.Errorf("幂等去斜杠失效：带尾斜杠 %q vs 规范域 %q", trail, norm)
	}
}
