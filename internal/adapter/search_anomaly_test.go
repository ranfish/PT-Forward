package adapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

// §59.199 异常页检测——0 行且无空结果标记 = 站点错误（并发限流轻量页），
// 不再静默当"确认无结果"（生死格斗案：Phase-2 恒 0 行零日志盲区）。
func TestSearchAnomalyPageDetected(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantErr string
	}{
		{
			name:    "异常页(无标记无行)",
			body:    "<html><body>Service degraded</body></html>",
			wantErr: "search page anomaly",
		},
		{
			name:    "真空页(含标记)",
			body:    "<html><body>没有种子。请用准确的关键字重试。</body></html>",
			wantErr: "",
		},
	}
	for _, c := range cases {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(c.body))
		}))
		defer srv.Close()
		a := NewNexusPHPAdapter(&HTTPDoer{Client: srv.Client()}, zap.NewNop())
		cfg := &model.SiteConfig{Domain: strings.TrimPrefix(srv.URL, "http://"), Cookie: "c=1"}
		res, err := a.SearchTorrents(context.Background(), cfg, "测试 关键词", nil)
		if c.wantErr == "" {
			if err != nil {
				t.Errorf("%s: unexpected err %v", c.name, err)
			}
			if len(res) != 0 {
				t.Errorf("%s: expect 0 rows, got %d", c.name, len(res))
			}
		} else if err == nil || !strings.Contains(err.Error(), c.wantErr) {
			t.Errorf("%s: expect err containing %q, got %v", c.name, c.wantErr, err)
		}
	}
}
