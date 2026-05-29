package publish

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"os"
	"regexp"
	"time"

	"go.uber.org/zap"
)

var rePixhostDirectURL = regexp.MustCompile(`https://img\d+\.pixhost\.to/images/[^"'\s]+\.jpg`)

type ImageHostUploader struct {
	maxRetries int
	client     *http.Client
	logger     *zap.Logger
}

func NewImageHostUploader(logger *zap.Logger) *ImageHostUploader {
	return &ImageHostUploader{
		maxRetries: 3,
		client: &http.Client{
			Timeout: 180 * time.Second,
		},
		logger: logger,
	}
}

type UploadResult struct {
	ShowURL    string `json:"show_url"`
	DirectURL  string `json:"direct_url"`
	ThumbURL   string `json:"thumb_url"`
}

func (u *ImageHostUploader) UploadPixHost(ctx context.Context, imagePath string) (*UploadResult, error) {
	var lastErr error

	for attempt := 0; attempt < u.maxRetries; attempt++ {
		result, err := u.doPixHostUpload(ctx, imagePath)
		if err == nil {
			return result, nil
		}
		lastErr = err
		u.logger.Warn("pixhost upload attempt failed",
			zap.Int("attempt", attempt+1),
			zap.Error(err))
		if attempt < u.maxRetries-1 {
			time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
		}
	}

	return nil, fmt.Errorf("pixhost upload failed after %d retries: %w", u.maxRetries, lastErr)
}

func (u *ImageHostUploader) UploadMultiple(ctx context.Context, imagePaths []string) ([]string, error) {
	var urls []string
	for _, path := range imagePaths {
		result, err := u.UploadPixHost(ctx, path)
		if err != nil {
			u.logger.Warn("image upload skipped",
				zap.String("path", path),
				zap.Error(err))
			continue
		}
		if result.DirectURL != "" {
			urls = append(urls, result.DirectURL)
		}
	}
	if len(urls) == 0 && len(imagePaths) > 0 {
		return nil, fmt.Errorf("all image uploads failed")
	}
	return urls, nil
}

func (u *ImageHostUploader) doPixHostUpload(ctx context.Context, imagePath string) (*UploadResult, error) {
	f, err := os.Open(imagePath) //nolint:gosec // image path is user-controlled
	if err != nil {
		return nil, fmt.Errorf("open image: %w", err)
	}
	defer func() { _ = f.Close() }()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("img", "screenshot.jpg")
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, fmt.Errorf("copy image data: %w", err)
	}
	if err := writer.WriteField("content_type", "1"); err != nil {
		return nil, fmt.Errorf("write content_type field: %w", err)
	}
	if err := writer.WriteField("max_th_size", "320"); err != nil {
		return nil, fmt.Errorf("write max_th_size field: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://pixhost.to/upload/api", &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload request: %w", err)
	}
	defer func() { httpclient.DrainBody(resp) }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upload status %d: %s", resp.StatusCode, string(body))
	}

	var uploadResp struct {
		Status    string `json:"status"`
		ShowURL   string `json:"show_url"`
		ThumbURL  string `json:"thumb_url"`
	}
	if err := json.Unmarshal(body, &uploadResp); err != nil {
		return nil, fmt.Errorf("parse upload response: %w", err)
	}

	if uploadResp.ShowURL == "" {
		return nil, fmt.Errorf("empty show_url in response")
	}

	directURL, err := u.resolveDirectURL(ctx, uploadResp.ShowURL)
	if err != nil {
		u.logger.Warn("failed to resolve direct URL, using show URL",
			zap.String("show_url", uploadResp.ShowURL),
			zap.Error(err))
		directURL = uploadResp.ShowURL
	}

	return &UploadResult{
		ShowURL:   uploadResp.ShowURL,
		DirectURL: directURL,
		ThumbURL:  uploadResp.ThumbURL,
	}, nil
}

func (u *ImageHostUploader) resolveDirectURL(ctx context.Context, showURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", showURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64)")

	resp, err := u.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { httpclient.DrainBody(resp) }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return "", err
	}

	match := rePixhostDirectURL.FindString(string(body))
	if match == "" {
		return "", fmt.Errorf("direct image URL not found in page")
	}
	return match, nil
}
