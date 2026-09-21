package ptgen

import (
	"go.uber.org/zap"
	"context"

	"net/http"
	"os"
	"net/url"
	"testing"
	"time"
)

// §59.254 项2 端点集成验证（真实豆影+财神——网络测试，短超时防挂）
func TestQueryIMDbForEnglish_Live(t *testing.T) {
	// 网络测试默认跳过（CI/无代理环境）——PTF_LIVE_TEST=1 显式触发
	if os.Getenv("PTF_LIVE_TEST") == "" {
		t.Skip("live test disabled (set PTF_LIVE_TEST=1)")
	}
	p := NewProvider(nil, zap.NewNop())
	// 29 开发机直连外网需代理（生产容器直连）
	p.SetHTTPClientForTest(&http.Client{Timeout: 60 * time.Second, Transport: &http.Transport{Proxy: func(*http.Request) (*url.URL, error) { return url.Parse("http://10.0.2.5:7897") }}})
	p.SetEndpoints("https://doubaninfo.com/api/v1_douban.php|a838fdca9c4793073e6452c0cc52b0f2f4fa66dc406a51dbefd715fcbd72cbe8#https://cspt.top/api/ptgen/query|0bc98da4901c979b362f3f2648a26e047e1bde4f3e38ba77048d4f3a145b0442")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	name, kind := p.QueryIMDbForEnglish(ctx, "https://movie.douban.com/subject/24284175/")
	if name != "Twilight of the Warriors: Walled In" || kind != "movie" {
		t.Fatalf("got name=%q kind=%q", name, kind)
	}
	
}
