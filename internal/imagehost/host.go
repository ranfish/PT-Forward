package imagehost

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ImageHost interface {
	Name() string
	Upload(ctx context.Context, data []byte, filename string) (*UploadResult, error)
	Rehost(ctx context.Context, sourceURL string) (*UploadResult, error)
	HealthCheck(ctx context.Context) error
}

type UploadResult struct {
	URL       string `json:"url"`
	BBCode    string `json:"bbcode"`
	DeleteKey string `json:"delete_key,omitempty"`
	HostName  string `json:"host_name"`
}

func downloadURL(ctx context.Context, url string) ([]byte, string, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download: %w", err)
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

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
