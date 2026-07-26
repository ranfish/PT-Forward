package imagehost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"time"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"go.uber.org/zap"
)

// rePixhostDirect 从 show 页面 HTML 提取直链。
// 同时匹配 .to 和 .cc 域名，支持 jpg/jpeg/png 扩展名。
var rePixhostDirect = regexp.MustCompile(`https://img\d+\.pixhost\.(?:to|cc)/images/[^"'\s]+\.(?:jpg|jpeg|png)`)

// PixhostHost 支持 pixhost.cc / pixhost.to 双域名 fallback。
// domains 按优先级排列：第一个可用域名优先使用。
type PixhostHost struct {
	client  *http.Client
	logger  *zap.Logger
	domains []string
}

func NewPixhostHost(logger *zap.Logger) *PixhostHost {
	return &PixhostHost{
		client:  &http.Client{Timeout: 180 * time.Second},
		logger:  logger,
		domains: []string{"pixhost.cc", "pixhost.to"},
	}
}

func (p *PixhostHost) Name() string { return "pixhost" }

func (p *PixhostHost) Upload(ctx context.Context, data []byte, filename string) (*UploadResult, error) {
	var lastErr error
	for _, domain := range p.domains {
		result, err := p.uploadToDomain(ctx, domain, data, filename)
		if err == nil {
			return result, nil
		}
		lastErr = err
		p.logger.Debug("pixhost upload failed on domain, trying next",
			zap.String("domain", domain),
			zap.Error(err))
	}
	return nil, fmt.Errorf("pixhost upload failed on all domains: %w", lastErr)
}

func (p *PixhostHost) uploadToDomain(ctx context.Context, domain string, data []byte, filename string) (*UploadResult, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("img", filename)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return nil, fmt.Errorf("write image data: %w", err)
	}
	if err := writer.WriteField("content_type", "1"); err != nil {
		return nil, fmt.Errorf("write content_type: %w", err)
	}
	if err := writer.WriteField("max_th_size", "320"); err != nil {
		return nil, fmt.Errorf("write max_th_size: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close writer: %w", err)
	}

	apiURL := fmt.Sprintf("https://api.%s/images", domain)
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload to %s: %w", domain, err)
	}
	defer func() { httpclient.DrainBody(resp) }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upload status %d from %s: %s", resp.StatusCode, domain, string(body))
	}

	var uploadResp struct {
		Status   string `json:"status"`
		ShowURL  string `json:"show_url"`
		ThumbURL string `json:"thumb_url"`
	}
	if err := json.Unmarshal(body, &uploadResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if uploadResp.ShowURL == "" {
		return nil, fmt.Errorf("empty show_url from %s", domain)
	}

	directURL, err := p.resolveDirectURL(ctx, uploadResp.ShowURL)
	if err != nil {
		directURL = uploadResp.ShowURL
	}

	return &UploadResult{
		URL:      directURL,
		BBCode:   fmt.Sprintf("[img]%s[/img]", directURL),
		HostName: "pixhost",
	}, nil
}

func (p *PixhostHost) Rehost(ctx context.Context, sourceURL string) (*UploadResult, error) {
	data, filename, err := downloadURL(ctx, sourceURL)
	if err != nil {
		return nil, fmt.Errorf("rehost download: %w", err)
	}
	return p.Upload(ctx, data, filename)
}

func (p *PixhostHost) HealthCheck(ctx context.Context) error {
	var lastErr error
	for _, domain := range p.domains {
		err := p.healthCheckDomain(ctx, domain)
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return fmt.Errorf("pixhost all domains unhealthy: %w", lastErr)
}

func (p *PixhostHost) healthCheckDomain(ctx context.Context, domain string) error {
	req, err := http.NewRequestWithContext(ctx, "HEAD", fmt.Sprintf("https://%s/", domain), nil)
	if err != nil {
		return err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { httpclient.DrainBody(resp) }()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("pixhost.%s unavailable: HTTP %d", domain, resp.StatusCode)
	}
	return nil
}

func (p *PixhostHost) resolveDirectURL(ctx context.Context, showURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", showURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64)")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { httpclient.DrainBody(resp) }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return "", err
	}

	match := rePixhostDirect.FindString(string(body))
	if match == "" {
		return "", fmt.Errorf("direct URL not found")
	}
	return match, nil
}
