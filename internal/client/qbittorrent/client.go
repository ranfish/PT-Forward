package qbittorrent

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type QBClient struct {
	cfg     *model.ClientConfig
	client  *http.Client
	baseURL string
	logger  *zap.Logger

	sid         string
	version     string
	apiVersion  string
	isV5        bool
	sharedPaths []model.SharedPathMapping

	mu sync.RWMutex
}

func NewQBClient(cfg *model.ClientConfig, sharedPaths []model.SharedPathMapping, logger *zap.Logger) (*QBClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, qbError(ErrQBNetwork, "create cookie jar", err)
	}

	baseURL := strings.TrimRight(cfg.URL, "/")
	return &QBClient{
		cfg:         cfg,
		client:      &http.Client{Jar: jar, Timeout: 30 * time.Second},
		baseURL:     baseURL,
		logger:      logger,
		sharedPaths: sharedPaths,
	}, nil
}

func (c *QBClient) Connect(ctx context.Context) error {
	if err := c.login(ctx); err != nil {
		return err
	}
	c.detectVersion(ctx)
	return nil
}

func (c *QBClient) GetName() string                           { return c.cfg.Name }
func (c *QBClient) GetRole() string                           { return c.cfg.Role }
func (c *QBClient) GetReseedTargetID() string                 { return c.cfg.ReseedTargetID }
func (c *QBClient) GetID() uint                               { return c.cfg.ID }
func (c *QBClient) GetSharedPaths() []model.SharedPathMapping { return c.sharedPaths }

func (c *QBClient) Version() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.version
}

func (c *QBClient) login(ctx context.Context) error {
	form := url.Values{}
	form.Set("username", c.cfg.Username)
	form.Set("password", c.cfg.Password)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/auth/login", strings.NewReader(form.Encode()))
	if err != nil {
		return c.wrapErr(11003, "build login request", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", c.baseURL)

	resp, err := c.client.Do(req)
	if err != nil {
		return c.wrapErr(11002, "login request", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return c.newErr(11003, fmt.Sprintf("login returned %d", resp.StatusCode))
	}

	body, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(body)) != "Ok." {
		return c.newErr(11003, "login failed: "+strings.TrimSpace(string(body)))
	}

	for _, cookie := range resp.Cookies() {
		if cookie.Name == "SID" {
			c.mu.Lock()
			c.sid = cookie.Value
			c.mu.Unlock()
			break
		}
	}

	c.logger.Debug("qBittorrent login success", zap.String("client", c.cfg.Name))
	return nil
}

func (c *QBClient) detectVersion(ctx context.Context) {
	if ver, err := c.getString(ctx, "/api/v2/app/version"); err == nil {
		c.mu.Lock()
		c.version = strings.TrimPrefix(ver, "v")
		major := 0
		if parts := strings.SplitN(c.version, ".", 2); len(parts) > 0 {
			major, _ = strconv.Atoi(parts[0])
		}
		c.isV5 = major >= 5
		c.mu.Unlock()
	}
	if apiVer, err := c.getString(ctx, "/api/v2/app/webapiVersion"); err == nil {
		c.mu.Lock()
		c.apiVersion = apiVer
		c.mu.Unlock()
	}
	c.logger.Debug("qBittorrent version detected",
		zap.String("client", c.cfg.Name),
		zap.String("version", c.version),
		zap.String("apiVersion", c.apiVersion),
		zap.Bool("isV5", c.isV5),
	)
}

func (c *QBClient) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", c.baseURL)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusForbidden {
		_ = resp.Body.Close()
		c.logger.Debug("session expired, re-login", zap.String("client", c.cfg.Name))
		if loginErr := c.login(ctx); loginErr != nil {
			return nil, loginErr
		}
		req2, _ := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
		req2.Header.Set("Referer", c.baseURL)
		return c.client.Do(req2)
	}

	return resp, nil
}

func (c *QBClient) get(ctx context.Context, path string) (*http.Response, error) {
	return c.doRequest(ctx, http.MethodGet, path, nil)
}

func (c *QBClient) postForm(ctx context.Context, path string, data url.Values) (*http.Response, error) {
	encoded := data.Encode()
	resp, err := c.doRequest(ctx, http.MethodPost, path, strings.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	resp.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return resp, nil
}

func (c *QBClient) getString(ctx context.Context, path string) (string, error) {
	resp, err := c.get(ctx, path)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return strings.TrimSpace(string(b)), nil
}

func (c *QBClient) pauseEndpoint() string {
	if c.isV5 {
		return "/api/v2/torrents/stop"
	}
	return "/api/v2/torrents/pause"
}

func (c *QBClient) resumeEndpoint() string {
	if c.isV5 {
		return "/api/v2/torrents/start"
	}
	return "/api/v2/torrents/resume"
}

func (c *QBClient) wrapErr(code int, msg string, err error) *model.AppError {
	return &model.AppError{Code: code, Message: fmt.Sprintf("qB %s: %s: %v", c.cfg.Name, msg, err)}
}

func (c *QBClient) newErr(code int, msg string) *model.AppError {
	return &model.AppError{Code: code, Message: fmt.Sprintf("qB %s: %s", c.cfg.Name, msg)}
}

func computeInfoHash(torrentData []byte) (string, error) {
	idx := bytes.Index(torrentData, []byte("4:info"))
	if idx == -1 {
		return "", qbError(ErrQBParse, "info dict not found in torrent data", nil)
	}
	infoStart := idx + 6
	end, err := findBencodeEnd(torrentData, infoStart)
	if err != nil {
		return "", qbError(ErrQBParse, "parse info dict", err)
	}
	h := sha1.Sum(torrentData[infoStart:end])
	return hex.EncodeToString(h[:]), nil
}

func findBencodeEnd(data []byte, pos int) (int, error) {
	if pos >= len(data) {
		return 0, qbError(ErrQBParse, "position out of bounds", nil)
	}
	switch data[pos] {
	case 'i':
		end := bytes.IndexByte(data[pos:], 'e')
		if end == -1 {
			return 0, qbError(ErrQBParse, "unterminated integer", nil)
		}
		return pos + end + 1, nil
	case 'l', 'd':
		cur := pos + 1
		for cur < len(data) && data[cur] != 'e' {
			end, err := findBencodeEnd(data, cur)
			if err != nil {
				return 0, err
			}
			cur = end
		}
		if cur >= len(data) {
			return 0, qbError(ErrQBParse, "unterminated container", nil)
		}
		return cur + 1, nil
	default:
		if data[pos] >= '0' && data[pos] <= '9' {
			colon := bytes.IndexByte(data[pos:], ':')
			if colon == -1 {
				return 0, qbError(ErrQBParse, "invalid string", nil)
			}
			length, err := strconv.Atoi(string(data[pos : pos+colon]))
			if err != nil {
				return 0, err
			}
			strEnd := pos + colon + 1 + length
			if strEnd > len(data) {
				return 0, qbError(ErrQBParse, "string exceeds data", nil)
			}
			return strEnd, nil
		}
		return 0, qbError(ErrQBParse, fmt.Sprintf("unexpected char %c at pos %d", data[pos], pos), nil)
	}
}

func buildMultipartBody(torrentData []byte, opts model.AddTorrentOptions) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	part, err := w.CreateFormFile("torrents", "torrent.torrent")
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(torrentData); err != nil {
		return nil, "", err
	}

	writeField := func(key, value string) {
		if value != "" {
			_ = w.WriteField(key, value)
		}
	}
	writeBool := func(key string, v bool) {
		if v {
			_ = w.WriteField(key, "true")
		}
	}

	writeField("savepath", opts.SavePath)
	writeField("category", opts.Category)
	if len(opts.Tags) > 0 {
		writeField("tags", strings.Join(opts.Tags, ","))
	}
	writeBool("paused", opts.Paused)
	if opts.UploadLimit > 0 {
		writeField("upLimit", strconv.FormatInt(opts.UploadLimit, 10))
	}
	if opts.DownloadLimit > 0 {
		writeField("dlLimit", strconv.FormatInt(opts.DownloadLimit, 10))
	}
	writeBool("skip_checking", opts.SkipChecking)
	if opts.AutoTMM {
		writeBool("autoTMM", true)
	}
	if opts.RatioLimit > 0 {
		writeField("ratioLimit", strconv.FormatFloat(opts.RatioLimit, 'f', -1, 64))
	}
	if opts.SeedingTimeLimit > 0 {
		writeField("seedingTimeLimit", strconv.Itoa(opts.SeedingTimeLimit))
	}

	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return &buf, w.FormDataContentType(), nil
}
