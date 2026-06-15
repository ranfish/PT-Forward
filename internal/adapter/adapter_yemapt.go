package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type YemaptAdapter struct {
	*GenericAdapter
	doer   *HTTPDoer
	logger *zap.Logger
}

func NewYemaptAdapter(doer *HTTPDoer, logger *zap.Logger) *YemaptAdapter {
	return &YemaptAdapter{
		GenericAdapter: NewGenericAdapter("yemapt", doer, logger),
		doer:           doer,
		logger:         logger,
	}
}

func (a *YemaptAdapter) Framework() string { return "yemapt" }

func (a *YemaptAdapter) DownloadTorrent(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	baseURL := resolveBaseURL(config)

	dlKey, err := a.generateDownloadKey(ctx, config, baseURL, torrentID)
	if err != nil {
		return nil, downloadError("生成下载密钥失败", err)
	}

	dlURL := baseURL + "/api/torrent/download1?token=" + url.QueryEscape(dlKey)
	req, err := http.NewRequestWithContext(ctx, "GET", dlURL, nil)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, downloadError("下载失败", err)
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode == http.StatusForbidden {
		return nil, &model.AppError{Code: 14003, Message: "403 Forbidden: 认证失败"}
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, notFoundError("种子不存在或已被删除")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("HTTP %d", resp.StatusCode), nil)
	}

	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/html") || strings.Contains(ct, "application/json") {
		body, _ := readBody(resp)
		return nil, &model.AppError{Code: 15001, Message: fmtES("返回了非种子文件: %s", string(body[:min(len(body), 200)]))}
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return nil, downloadError("读取种子数据失败", err)
	}
	if len(data) == 0 {
		return nil, &model.AppError{Code: 15001, Message: "种子数据为空"}
	}
	return data, nil
}

func (a *YemaptAdapter) generateDownloadKey(ctx context.Context, config *model.SiteConfig, baseURL, torrentID string) (string, error) {
	u := baseURL + "/api/torrent/generateDownloadKey?id=" + url.QueryEscape(torrentID)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return "", err
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := readBody(resp)
	if err != nil {
		return "", err
	}

	var result struct {
		Success      bool        `json:"success"`
		ErrorMessage string      `json:"errorMessage"`
		ErrorCode    int         `json:"errorCode"`
		Data         interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if !result.Success {
		return "", fmt.Errorf("API error %d: %s", result.ErrorCode, result.ErrorMessage)
	}

	switch v := result.Data.(type) {
	case string:
		return v, nil
	case map[string]interface{}:
		if key, ok := v["key"].(string); ok {
			return key, nil
		}
		if token, ok := v["token"].(string); ok {
			return token, nil
		}
	}
	return "", fmt.Errorf("unexpected download key format: %T", result.Data)
}

func (a *YemaptAdapter) FetchUserStats(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	stats := &model.UserStatsResult{}

	if config.APIKey != "" {
		if err := a.fetchUserStatsOpenAPI(ctx, config, stats); err != nil {
			a.logger.Debug("openApi stats failed, trying web api", zap.Error(err))
		} else {
			a.enrichSeedingInfo(ctx, config, stats)
			return stats, nil
		}
	}

	if config.Cookie != "" {
		if err := a.fetchUserStatsWebAPI(ctx, config, stats); err != nil {
			return nil, err
		}
		a.enrichSeedingInfo(ctx, config, stats)
		return stats, nil
	}

	return nil, &model.AppError{Code: 14003, Message: "野马需要 API Key 或 Cookie"}
}

func (a *YemaptAdapter) fetchUserStatsOpenAPI(ctx context.Context, config *model.SiteConfig, stats *model.UserStatsResult) error {
	baseURL := resolveBaseURL(config)
	u := baseURL + "/openApi/user/fetchBasicInfo.json"

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", config.APIKey)
	setCommonHeaders(req, "")

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := readBody(resp)
	if err != nil {
		return err
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			ID                    int    `json:"id"`
			Name                  string `json:"name"`
			Level                 int    `json:"level"`
			Bonus                 int64  `json:"bonus"`
			PromotionUploadSize   int64  `json:"promotionUploadSize"`
			PromotionDownloadSize int64  `json:"promotionDownloadSize"`
			Status                string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析失败: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("API 返回失败")
	}

	stats.Username = result.Data.Name
	stats.UploadBytes = result.Data.PromotionUploadSize
	stats.DownloadBytes = result.Data.PromotionDownloadSize
	stats.BonusPoints = float64(result.Data.Bonus)
	if result.Data.PromotionDownloadSize > 0 {
		stats.Ratio = float64(result.Data.PromotionUploadSize) / float64(result.Data.PromotionDownloadSize)
	}

	levelNames := map[int]string{
		0: "乱民", 1: "小卒", 2: "教谕", 3: "登仕郎", 4: "修职郎",
		5: "文林郎", 6: "忠武校尉", 7: "承信将军", 8: "武毅将军",
		9: "武节将军", 10: "显威将军", 11: "宣武将军",
	}
	if name, ok := levelNames[result.Data.Level]; ok {
		stats.UserClass = name
	}

	return nil
}

func (a *YemaptAdapter) fetchUserStatsWebAPI(ctx context.Context, config *model.SiteConfig, stats *model.UserStatsResult) error {
	baseURL := resolveBaseURL(config)
	u := baseURL + "/api/consumer/fetchSelfDetail"

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return err
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := readBody(resp)
	if err != nil {
		return err
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			ID                    int    `json:"id"`
			Name                  string `json:"name"`
			Level                 int    `json:"level"`
			Bonus                 int64  `json:"bonus"`
			UploadSize            int64  `json:"uploadSize"`
			DownloadSize          int64  `json:"downloadSize"`
			PromotionUploadSize   int64  `json:"promotionUploadSize"`
			PromotionDownloadSize int64  `json:"promotionDownloadSize"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析失败: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("API 返回失败")
	}

	stats.Username = result.Data.Name
	stats.UploadBytes = result.Data.PromotionUploadSize
	stats.DownloadBytes = result.Data.PromotionDownloadSize
	stats.BonusPoints = float64(result.Data.Bonus)
	if result.Data.PromotionDownloadSize > 0 {
		stats.Ratio = float64(result.Data.PromotionUploadSize) / float64(result.Data.PromotionDownloadSize)
	}

	levelNames := map[int]string{
		0: "乱民", 1: "小卒", 2: "教谕", 3: "登仕郎", 4: "修职郎",
		5: "文林郎", 6: "忠武校尉", 7: "承信将军", 8: "武毅将军",
		9: "武节将军", 10: "显威将军", 11: "宣武将军",
	}
	if name, ok := levelNames[result.Data.Level]; ok {
		stats.UserClass = name
	}

	return nil
}

func (a *YemaptAdapter) enrichSeedingInfo(ctx context.Context, config *model.SiteConfig, stats *model.UserStatsResult) {
	if config.Cookie == "" {
		return
	}

	baseURL := resolveBaseURL(config)
	u := baseURL + "/api/userTorrent/fetchSeedTorrentInfo"

	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader("{}"))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode != http.StatusOK {
		return
	}

	body, err := readBody(resp)
	if err != nil {
		return
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Num      int   `json:"num"`
			FileSize int64 `json:"fileSize"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return
	}
	if result.Success {
		stats.SeedingCount = result.Data.Num
		stats.SeedingSize = result.Data.FileSize
	}
}

func (a *YemaptAdapter) GetTorrentDetail(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	return a.GenericAdapter.GetTorrentDetail(ctx, config, torrentID)
}

func (a *YemaptAdapter) DetectDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	return a.GenericAdapter.DetectDiscount(ctx, config, torrentID)
}

func (a *YemaptAdapter) DetectHR(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
	return a.GenericAdapter.DetectHR(ctx, config, torrentID)
}

func (a *YemaptAdapter) DetectHRAndDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, *model.DiscountResult, error) {
	return a.GenericAdapter.DetectHRAndDiscount(ctx, config, torrentID)
}

func (a *YemaptAdapter) GetBatchSLData(ctx context.Context, config *model.SiteConfig, torrentIDs []string) (map[string]*model.SLData, error) {
	return a.GenericAdapter.GetBatchSLData(ctx, config, torrentIDs)
}

func (a *YemaptAdapter) GetPreciseSLData(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	return a.GenericAdapter.GetPreciseSLData(ctx, config, torrentID)
}

func (a *YemaptAdapter) GetTorrentInfoHash(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	return a.GenericAdapter.GetTorrentInfoHash(ctx, config, torrentID)
}

func (a *YemaptAdapter) UploadTorrent(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	return a.GenericAdapter.UploadTorrent(ctx, config, req)
}

func (a *YemaptAdapter) VerifyExists(ctx context.Context, config *model.SiteConfig, torrentID string) (bool, error) {
	return a.GenericAdapter.VerifyExists(ctx, config, torrentID)
}
