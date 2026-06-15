package cloudfp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Service struct {
	db     *gorm.DB
	logger *zap.Logger
	client *http.Client

	mu         sync.RWMutex
	config     *model.CloudFPConfig
	breaker    *circuitBreaker
	stopCh     chan struct{}
	stopped    bool
}

func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	s := &Service{
		db:     db,
		logger: logger.With(zap.String("component", "cloudfp")),
		client: &http.Client{Timeout: 10 * time.Second},
		breaker: &circuitBreaker{
			maxFailures:  5,
			recoveryTime: 30 * time.Second,
		},
		stopCh: make(chan struct{}),
	}
	s.reloadConfig()
	return s
}

func (s *Service) reloadConfig() {
	var cfg model.CloudFPConfig
	if err := s.db.First(&cfg, 1).Error; err != nil {
		s.mu.Lock()
		s.config = nil
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	s.config = &cfg
	if cfg.RequestTimeoutSec > 0 {
		s.client.Timeout = time.Duration(cfg.RequestTimeoutSec) * time.Second
	}
	s.mu.Unlock()
}

func (s *Service) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config != nil && s.config.Enabled
}

func (s *Service) getBaseURL() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.config == nil {
		return ""
	}
	return s.config.BaseURL
}

func (s *Service) getToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.config == nil {
		return ""
	}
	return s.config.APIToken
}

func (s *Service) BatchLookup(ctx context.Context, piecesHashes []string, targetSites []string) (map[string][]model.CloudFPMatch, error) {
	if !s.isEnabled() {
		return nil, nil
	}
	if !s.breaker.allow() {
		return nil, fmt.Errorf("circuit breaker open")
	}

	body, _ := json.Marshal(map[string]interface{}{
		"pieces_hashes": piecesHashes,
		"target_sites":  targetSites,
	})

	respBody, err := s.doRequest(ctx, "POST", "/api/v1/fingerprints/lookup", body)
	if err != nil {
		s.breaker.recordFailure()
		return nil, err
	}
	s.breaker.recordSuccess()

	var resp struct {
		Matches map[string][]model.CloudFPMatch `json:"matches"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse lookup response: %w", err)
	}
	return resp.Matches, nil
}

func (s *Service) ReportDeleted(ctx context.Context, reports []model.CloudFPDeleteReport) error {
	if !s.isEnabled() {
		return nil
	}
	body, _ := json.Marshal(map[string]interface{}{"reports": reports})
	_, err := s.doRequest(ctx, "POST", "/api/v1/fingerprints/report-deleted", body)
	return err
}

func (s *Service) UploadRecords(ctx context.Context, records []model.CloudFPContribute) error {
	if !s.isEnabled() {
		return nil
	}
	body, _ := json.Marshal(map[string]interface{}{"records": records})
	_, err := s.doRequest(ctx, "POST", "/api/v1/fingerprints/contribute", body)
	return err
}

func (s *Service) TestConnection(ctx context.Context) error {
	baseURL := s.getBaseURL()
	if baseURL == "" {
		return fmt.Errorf("cloud FP service not configured")
	}
	_, err := s.doRequest(ctx, "GET", "/api/v1/health", nil)
	return err
}

func (s *Service) isEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config != nil && s.config.Enabled
}

func (s *Service) doRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	baseURL := s.getBaseURL()
	if baseURL == "" {
		return nil, fmt.Errorf("cloud FP service not configured")
	}

	url := baseURL + path
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	token := s.getToken()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			if attempt == 0 {
				req.Body = io.NopCloser(bytes.NewReader(body))
				continue
			}
			return nil, lastErr
		}

		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()

		if err != nil {
			return nil, fmt.Errorf("read response: %w", err)
		}

		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("server error: HTTP %d", resp.StatusCode)
			if attempt == 0 {
				req.Body = io.NopCloser(bytes.NewReader(body))
				continue
			}
			return nil, lastErr
		}
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("client error: HTTP %d: %s", resp.StatusCode, string(respBody))
		}
		return respBody, nil
	}
	return nil, lastErr
}

func (s *Service) IsBreakerOpen() bool {
	s.breaker.mu.Lock()
	defer s.breaker.mu.Unlock()
	return s.breaker.failureCount >= s.breaker.maxFailures &&
		time.Since(s.breaker.lastFailureTime) <= s.breaker.recoveryTime
}

func (s *Service) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.stopped {
		close(s.stopCh)
		s.stopped = true
	}
}

type circuitBreaker struct {
	mu              sync.Mutex
	failureCount    int
	lastFailureTime time.Time
	maxFailures     int
	recoveryTime    time.Duration
}

func (cb *circuitBreaker) allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.failureCount >= cb.maxFailures {
		if time.Since(cb.lastFailureTime) > cb.recoveryTime {
			cb.failureCount = 0
			return true
		}
		return false
	}
	return true
}

func (cb *circuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount = 0
}

func (cb *circuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount++
	cb.lastFailureTime = time.Now()
}
