package imagehost

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/ranfish/pt-forward/internal/httpclient"
)

// toURL 把字符串转 *url.URL（供 Manager.SetProxy 用）
func toURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	return u
}

type ImageHost interface {
	Name() string
	Upload(ctx context.Context, data []byte, filename string) (*UploadResult, error)
	Rehost(ctx context.Context, sourceURL string) (*UploadResult, error)
	HealthCheck(ctx context.Context) error
	// SetTransport 09 刀1: Manager.Register 时一次性注入 transportSwitcher
	// （此后 Transport 字段只读——零并发写 race·动态代理切换由 switcher
	// 内部锁保证）。替代 v0.0.256 的全局 DefaultTransport 临时替换。
	SetTransport(rt http.RoundTripper)
}

type UploadResult struct {
	URL       string `json:"url"`
	BBCode    string `json:"bbcode"`
	DeleteKey string `json:"delete_key,omitempty"`
	HostName  string `json:"host_name"`
}

// downloadURL 09 刀1: rt 入参（host 的 switcher 注入——Rehost 下载源图
// 走代理与上传同链·不再依赖全局 DefaultTransport 被临时替换的副作用）。
func downloadURL(ctx context.Context, url string, rt http.RoundTripper) ([]byte, string, error) {
	client := &http.Client{Timeout: 60 * time.Second, Transport: rt}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", httpclient.BrowserUserAgent())

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download: %w", err)
	}
	// 09 刀2: 排空套 LimitReader（原无界 io.Copy——远端可驱动的持续推流
	// 放大；上限与 httpclient.maxDrainSize 对齐）
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
	if err != nil {
		return nil, "", fmt.Errorf("read body: %w", err)
	}

	filename := "image.jpg"
	if ct := resp.Header.Get("Content-Type"); ct == "image/png" {
		filename = "image.png"
	}

	return data, filename, nil
}
