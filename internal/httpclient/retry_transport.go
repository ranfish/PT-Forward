package httpclient

import (
	"bytes"
	"io"
	"math/rand/v2"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type RetryConfig struct {
	MaxRetries     int
	BaseDelay      time.Duration
	MaxDelay       time.Duration
	RetryableCheck func(*http.Response, error) bool
	OnRetry        func(attempt int, resp *http.Response, err error)
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		BaseDelay:      500 * time.Millisecond,
		MaxDelay:       30 * time.Second,
		RetryableCheck: DefaultRetryableCheck,
	}
}

func DefaultRetryableCheck(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	if resp == nil {
		return true
	}

	if resp.StatusCode == 429 {
		return true
	}
	if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
		return true
	}
	return false
}

type retryTransport struct {
	base   http.RoundTripper
	config RetryConfig
	logger *zap.Logger
}

func NewRetryTransport(base http.RoundTripper, cfg RetryConfig, logger *zap.Logger) *retryTransport {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.BaseDelay <= 0 {
		cfg.BaseDelay = 500 * time.Millisecond
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 30 * time.Second
	}
	if cfg.RetryableCheck == nil {
		cfg.RetryableCheck = DefaultRetryableCheck
	}
	return &retryTransport{
		base:   base,
		config: cfg,
		logger: logger,
	}
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var (
		resp *http.Response
		err  error
	)

	bodyBytes, bodyReadErr := readAndCacheBody(req)
	if bodyReadErr != nil {
		bodyBytes = nil
	}

	for attempt := 0; attempt <= t.config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := t.backoffDelay(attempt)
			if t.logger != nil {
				t.logger.Debug("retrying request",
					zap.String("url", req.URL.String()),
					zap.Int("attempt", attempt),
					zap.Duration("delay", delay),
				)
			}

			select {
			case <-time.After(delay):
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}

			if bodyBytes != nil {
				req.Body = io.NopCloser(bodyBytes.Reader())
			}
			if req.GetBody != nil && bodyBytes == nil {
				newBody, bodyErr := req.GetBody()
				if bodyErr == nil {
					req.Body = newBody
				}
			}
		}

		resp, err = t.base.RoundTrip(req)

		if !t.config.RetryableCheck(resp, err) {
			return resp, err
		}

		if resp != nil && resp.Body != nil {
			drainAndClose(resp.Body)
		}

		if t.config.OnRetry != nil {
			t.config.OnRetry(attempt+1, resp, err)
		}
	}

	return resp, err
}

func (t *retryTransport) backoffDelay(attempt int) time.Duration {
	delay := t.config.BaseDelay * time.Duration(1<<(attempt-1))

	jitter := time.Duration(rand.Int64N(int64(t.config.BaseDelay))) //nolint:gosec // retry jitter does not need crypto/rand
	delay += jitter

	if delay > t.config.MaxDelay {
		delay = t.config.MaxDelay
	}
	return delay
}

func (t *retryTransport) CloseIdleConnections() {
	if ci, ok := t.base.(interface{ CloseIdleConnections() }); ok {
		ci.CloseIdleConnections()
	}
}

type cachedBody struct {
	data []byte
}

func (cb *cachedBody) Reader() *cachedReader {
	return &cachedReader{data: cb.data, pos: 0}
}

type cachedReader struct {
	data []byte
	pos  int
}

func (cr *cachedReader) Read(p []byte) (int, error) {
	if cr.pos >= len(cr.data) {
		return 0, io.EOF
	}
	n := copy(p, cr.data[cr.pos:])
	cr.pos += n
	return n, nil
}

func readAndCacheBody(req *http.Request) (*cachedBody, error) {
	if req.Body == nil || req.GetBody != nil {
		return nil, nil
	}

	data, err := io.ReadAll(io.LimitReader(req.Body, 100<<20))
	if closeErr := req.Body.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewReader(data))
	return &cachedBody{data: data}, nil
}

func drainAndClose(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, io.LimitReader(body, 1<<20))
	_ = body.Close()
}
