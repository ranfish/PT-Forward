package transmission

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type TRClient struct {
	cfg     *model.ClientConfig
	client  *http.Client
	baseURL string
	logger  *zap.Logger

	sessionID   string
	sharedPaths []model.SharedPathMapping
	mu          sync.Mutex
}

type rpcRequest struct {
	Method    string      `json:"method"`
	Arguments interface{} `json:"arguments,omitempty"`
}

type rpcResponse struct {
	Result    string          `json:"result"`
	Arguments json.RawMessage `json:"arguments"`
}

func NewTRClient(cfg *model.ClientConfig, sharedPaths []model.SharedPathMapping, logger *zap.Logger) (*TRClient, error) {
	baseURL := strings.TrimRight(cfg.URL, "/")
	if !strings.HasSuffix(baseURL, "/rpc") {
		baseURL += "/transmission/rpc"
	}
	return &TRClient{
		cfg:         cfg,
		client:      &http.Client{Timeout: 30 * time.Second},
		baseURL:     baseURL,
		logger:      logger,
		sharedPaths: sharedPaths,
	}, nil
}

func (c *TRClient) Connect(ctx context.Context) error {
	_, err := c.rpcCall(ctx, "session-get", map[string]interface{}{"fields": []string{"version"}})
	if err != nil {
		return err
	}
	c.logger.Debug("Transmission connected", zap.String("client", c.cfg.Name))
	return nil
}

func (c *TRClient) GetName() string                           { return c.cfg.Name }
func (c *TRClient) GetRole() string                           { return c.cfg.Role }
func (c *TRClient) GetReseedTargetID() string                 { return c.cfg.ReseedTargetID }
func (c *TRClient) GetID() uint                               { return c.cfg.ID }
func (c *TRClient) GetSharedPaths() []model.SharedPathMapping { return c.sharedPaths }

func (c *TRClient) rpcCall(ctx context.Context, method string, args interface{}) (*rpcResponse, error) {
	body, err := json.Marshal(rpcRequest{Method: method, Arguments: args})
	if err != nil {
		return nil, trError(ErrTRParse, "marshal rpc request", err)
	}

	resp, err := c.doRPCCall(ctx, body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusConflict {
		sid := resp.Header.Get("X-Transmission-Session-Id")
		_ = resp.Body.Close()
		if sid != "" {
			c.mu.Lock()
			c.sessionID = sid
			c.mu.Unlock()
		}
		resp, err = c.doRPCCall(ctx, body)
		if err != nil {
			return nil, err
		}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, c.newErr(11003, "unauthorized: wrong username or password")
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, c.newErr(11002, fmt.Sprintf("rpc returned %d: %s", resp.StatusCode, string(b)))
	}

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, c.wrapErr(11002, "decode rpc response", err)
	}
	if rpcResp.Result != "success" {
		return nil, c.newErr(11002, fmt.Sprintf("rpc error: %s", rpcResp.Result))
	}
	return &rpcResp, nil
}

func (c *TRClient) doRPCCall(ctx context.Context, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	c.mu.Lock()
	sid := c.sessionID
	c.mu.Unlock()
	if sid != "" {
		req.Header.Set("X-Transmission-Session-Id", sid)
	}

	if c.cfg.Username != "" {
		req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	}

	return c.client.Do(req)
}

func (c *TRClient) getTorrents(ctx context.Context, ids interface{}, fields []string) ([]trTorrent, error) {
	args := map[string]interface{}{
		"fields": fields,
	}
	if ids != nil {
		args["ids"] = ids
	}

	resp, err := c.rpcCall(ctx, "torrent-get", args)
	if err != nil {
		return nil, err
	}

	var result struct {
		Torrents []trTorrent `json:"torrents"`
	}
	if err := json.Unmarshal(resp.Arguments, &result); err != nil {
		return nil, c.wrapErr(11002, "decode torrents", err)
	}
	return result.Torrents, nil
}

func (c *TRClient) torrentAction(ctx context.Context, method string, hashes []string) error {
	ids := make([]string, len(hashes))
	copy(ids, hashes)
	_, err := c.rpcCall(ctx, method, map[string]interface{}{"ids": ids})
	return err
}

func (c *TRClient) getLabels(ctx context.Context, hash string) ([]string, error) {
	torrents, err := c.getTorrents(ctx, []string{hash}, []string{"labels"})
	if err != nil || len(torrents) == 0 {
		return nil, err
	}
	return torrents[0].Labels, nil
}

func (c *TRClient) setLabels(ctx context.Context, hash string, labels []string) error {
	_, err := c.rpcCall(ctx, "torrent-set", map[string]interface{}{
		"ids":    []string{hash},
		"labels": labels,
	})
	return err
}

func (c *TRClient) wrapErr(code int, msg string, err error) *model.AppError {
	return &model.AppError{Code: code, Message: fmt.Sprintf("TR %s: %s: %v", c.cfg.Name, msg, err)}
}

func (c *TRClient) newErr(code int, msg string) *model.AppError {
	return &model.AppError{Code: code, Message: fmt.Sprintf("TR %s: %s", c.cfg.Name, msg)}
}

func buildTRLabels(category string, tags []string) []string {
	var labels []string
	if category != "" {
		labels = append(labels, category)
	}
	labels = append(labels, tags...)
	if len(labels) == 0 {
		return []string{}
	}
	return labels
}
