package imagehost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"go.uber.org/zap"
)

const agsvptBaseURL = "https://img.seedvault.cn/api/v1"

type AGSVPTHost struct {
	email    string
	password string
	client   *http.Client
	logger   *zap.Logger

	mu    sync.Mutex
	token string
}

func NewAGSVPTHost(email, password string, logger *zap.Logger) *AGSVPTHost {
	return &AGSVPTHost{
		email:    email,
		password: password,
		client:   &http.Client{Timeout: 60 * time.Second},
		logger:   logger,
	}
}

func (a *AGSVPTHost) Name() string { return "agsvpt" }


func (a *AGSVPTHost) SetCredentials(email, password string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.email = email
	a.password = password
	a.token = ""
}

func (a *AGSVPTHost) Upload(ctx context.Context, data []byte, filename string) (*UploadResult, error) {
	if err := a.ensureToken(ctx); err != nil {
		return nil, fmt.Errorf("agsvpt auth: %w", err)
	}

	result, err := a.doUpload(ctx, data, filename)
	if err != nil && a.isAuthError(err) {
		a.logger.Debug("agsvpt token expired, refreshing")
		a.mu.Lock()
		a.token = ""
		a.mu.Unlock()
		if refreshErr := a.ensureToken(ctx); refreshErr != nil {
			return nil, fmt.Errorf("agsvpt token refresh: %w", refreshErr)
		}
		result, err = a.doUpload(ctx, data, filename)
	}
	return result, err
}

func (a *AGSVPTHost) doUpload(ctx context.Context, data []byte, filename string) (*UploadResult, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return nil, fmt.Errorf("write data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", agsvptBaseURL+"/upload", &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.getToken())

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload: %w", err)
	}
	defer func() { httpclient.DrainBody(resp) }()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("agsvpt auth error: HTTP 401")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("agsvpt rate limited: HTTP 429")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("agsvpt upload status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Status  bool `json:"status"`
		Message string `json:"message"`
		Data    struct {
			Key     string `json:"key"`
			Name    string `json:"name"`
			Links   struct {
				URL       string `json:"url"`
				BBCode    string `json:"bbcode"`
				Thumbnail string `json:"thumbnail_url"`
			} `json:"links"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if !apiResp.Status {
		return nil, fmt.Errorf("agsvpt api error: %s", apiResp.Message)
	}

	return &UploadResult{
		URL:       apiResp.Data.Links.URL,
		BBCode:    apiResp.Data.Links.BBCode,
		DeleteKey: apiResp.Data.Key,
		HostName:  "agsvpt",
	}, nil
}

func (a *AGSVPTHost) Rehost(ctx context.Context, sourceURL string) (*UploadResult, error) {
	data, filename, err := downloadURL(ctx, sourceURL)
	if err != nil {
		return nil, fmt.Errorf("agsvpt rehost download: %w", err)
	}
	return a.Upload(ctx, data, filename)
}

func (a *AGSVPTHost) HealthCheck(ctx context.Context) error {
	if err := a.ensureToken(ctx); err != nil {
		return fmt.Errorf("agsvpt auth: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", agsvptBaseURL+"/profile", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.getToken())
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { httpclient.DrainBody(resp) }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("agsvpt health check: HTTP %d", resp.StatusCode)
	}
	return nil
}

func (a *AGSVPTHost) ensureToken(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.token != "" {
		return nil
	}
	if a.email == "" || a.password == "" {
		return fmt.Errorf("agsvpt credentials not set")
	}

	payload, _ := json.Marshal(map[string]string{
		"email":    a.email,
		"password": a.password,
	})
	req, err := http.NewRequestWithContext(ctx, "POST", agsvptBaseURL+"/tokens", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("token request: %w", err)
	}
	defer func() { httpclient.DrainBody(resp) }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token request status %d", resp.StatusCode)
	}

	var tokenResp struct {
		Status bool `json:"status"`
		Data   struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("parse token response: %w", err)
	}
	if !tokenResp.Status || tokenResp.Data.Token == "" {
		return fmt.Errorf("empty token in response")
	}
	a.token = tokenResp.Data.Token
	return nil
}

func (a *AGSVPTHost) getToken() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.token
}

func (a *AGSVPTHost) isAuthError(err error) bool {
	return err != nil && (contains(err.Error(), "HTTP 401") || contains(err.Error(), "auth error"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsInner(s, substr))
}

func containsInner(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
