package adapter

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type RousiAdapter struct {
	*GenericAdapter
	doer   *HTTPDoer
	logger *zap.Logger
}

func NewRousiAdapter(doer *HTTPDoer, logger *zap.Logger) *RousiAdapter {
	return &RousiAdapter{
		GenericAdapter: NewGenericAdapter("rousi", doer, logger),
		doer:           doer,
		logger:         logger,
	}
}

func (a *RousiAdapter) Framework() string { return "rousi" }

func (a *RousiAdapter) SearchTorrents(ctx context.Context, config *model.SiteConfig, keyword string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
	u := resolveBaseURL(config) + "/api/v1/torrents?perPage=20&page=1"
	if keyword != "" {
		u += "&search=" + keyword
	}
	if opts != nil && opts.Category != "" {
		u += "&category=" + opts.Category
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, searchError("构造搜索请求失败", err)
	}
	a.setAuthHeaders(req, config.Passkey)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, searchError("搜索请求失败", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	var result struct {
		Code int `json:"code"`
		Data struct {
			Torrents []struct {
				ID       int    `json:"id"`
				UUID     string `json:"uuid"`
				Title    string `json:"title"`
				Size     int64  `json:"size"`
				Seeders  int    `json:"seeders"`
				Leechers int    `json:"leechers"`
				Category string `json:"category"`
			} `json:"torrents"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, parseError("解析搜索结果失败", err)
	}

	var results []*model.SeedingSearchResult
	for _, t := range result.Data.Torrents {
		tid := t.UUID
		if tid == "" {
			tid = fmt.Sprintf("%d", t.ID)
		}
		results = append(results, &model.SeedingSearchResult{
			TorrentID: tid,
			Title:     t.Title,
			Size:      t.Size,
			Seeders:   t.Seeders,
			Leechers:  t.Leechers,
		})
	}
	return results, nil
}

func (a *RousiAdapter) setAuthHeaders(req *http.Request, passkey string) {
	req.Header.Set("Authorization", "Bearer "+passkey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://rousi.pro")
	req.Header.Set("Referer", "https://rousi.pro/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
}

func (a *RousiAdapter) DownloadTorrent(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	u := resolveBaseURL(config) + "/api/torrent/" + torrentID + "/download/" + config.Passkey
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	a.setAuthHeaders(req, config.Passkey)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, downloadError("下载失败", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("HTTP %d", resp.StatusCode), nil)
	}

	return io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
}

func (a *RousiAdapter) GetTorrentDetail(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	u := resolveBaseURL(config) + "/api/v1/torrents/" + torrentID
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	a.setAuthHeaders(req, config.Passkey)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("请求详情失败", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("HTTP %d", resp.StatusCode), nil)
	}

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	var result struct {
		Code int `json:"code"`
		Data struct {
			UUID      string `json:"uuid"`
			Title     string `json:"title"`
			Subtitle  string `json:"subtitle"`
			Size      int64  `json:"size"`
			InfoHash  string `json:"info_hash"`
			Category  string `json:"category"`
			Seeders   int    `json:"seeders"`
			Leechers  int    `json:"leechers"`
			MediaInfo string `json:"media_info"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, parseError("解析响应失败", err)
	}

	d := result.Data
	return &model.TorrentDetail{
		Title:     d.Title,
		Size:      d.Size,
		InfoHash:  strings.ToLower(d.InfoHash),
		Category:  d.Category,
		MediaInfo: d.MediaInfo,
	}, nil
}

func (a *RousiAdapter) DetectDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	u := resolveBaseURL(config) + "/api/v1/torrents/" + torrentID
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	a.setAuthHeaders(req, config.Passkey)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("请求优惠信息失败", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	var result struct {
		Data struct {
			Promotion *struct {
				IsActive       bool    `json:"is_active"`
				Type           int     `json:"type"`
				DownMultiplier float64 `json:"down_multiplier"`
				UpMultiplier   float64 `json:"up_multiplier"`
			} `json:"promotion"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, parseError("解析优惠信息失败", err)
	}

	promo := result.Data.Promotion
	if promo == nil || !promo.IsActive {
		return &model.DiscountResult{Level: model.DiscountNone}, nil
	}

	down := promo.DownMultiplier
	up := promo.UpMultiplier

	if down == 0 && up >= 2 {
		return &model.DiscountResult{Level: model.Discount2xFree, Multiplier: up}, nil
	}
	if down == 0 {
		return &model.DiscountResult{Level: model.DiscountFree}, nil
	}
	if down == 0.5 && up >= 2 {
		return &model.DiscountResult{Level: model.Discount2x50}, nil
	}
	if down == 0.5 {
		return &model.DiscountResult{Level: model.DiscountPercent50}, nil
	}
	if down == 0.3 {
		return &model.DiscountResult{Level: model.DiscountPercent30}, nil
	}
	if up >= 2 {
		return &model.DiscountResult{Level: model.Discount2xUp, Multiplier: up}, nil
	}
	return &model.DiscountResult{Level: model.DiscountNone}, nil
}

func (a *RousiAdapter) DetectHR(_ context.Context, _ *model.SiteConfig, _ string) (*model.HRResult, error) {
	return &model.HRResult{HasHR: false}, nil
}

func (a *RousiAdapter) GetPreciseSLData(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	u := resolveBaseURL(config) + "/api/v1/torrents/" + torrentID
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return &model.SLData{}, nil
	}
	a.setAuthHeaders(req, config.Passkey)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return &model.SLData{}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	var result struct {
		Data struct {
			Seeders  int `json:"seeders"`
			Leechers int `json:"leechers"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return &model.SLData{}, nil
	}

	return &model.SLData{Seeders: result.Data.Seeders, Leechers: result.Data.Leechers}, nil
}

func (a *RousiAdapter) UploadTorrent(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	if len(req.TorrentData) == 0 {
		return nil, &model.AppError{Code: 40001, Message: "种子文件数据为空"}
	}

	payload := map[string]interface{}{
		"torrent":     base64.StdEncoding.EncodeToString(req.TorrentData),
		"title":       req.Title,
		"category":    resolveField(req.FormFields, "cat", "category"),
		"description": req.Description,
		"subtitle":    req.Subtitle,
	}
	if req.Anonymous {
		payload["anonymous"] = true
	}
	if req.MediaInfo != "" {
		payload["media_info"] = req.MediaInfo
	}

	attributes := map[string]interface{}{}
	if v, ok := req.FormFields["resolution"]; ok && v != "" {
		attributes["resolution"] = v
	}
	if v, ok := req.FormFields["source"]; ok && v != "" {
		attributes["source"] = v
	}
	if v, ok := req.FormFields["region"]; ok && v != "" {
		attributes["region"] = v
	}
	if v, ok := req.FormFields["genre"]; ok && v != "" {
		attributes["genre"] = strings.Split(v, ",")
	}
	if v, ok := req.FormFields["format"]; ok && v != "" {
		attributes["format"] = v
	}
	if v, ok := req.FormFields["platform"]; ok && v != "" {
		attributes["platform"] = v
	}
	if req.IMDbLink != "" {
		attributes["imdb"] = req.IMDbLink
	}
	if req.DoubanLink != "" {
		attributes["douban"] = req.DoubanLink
	}
	if tmdb, ok := req.ExtraFields["tmdb_id"]; ok && tmdb != "" {
		attributes["tmdb"] = tmdb
	}
	if len(attributes) > 0 {
		payload["attributes"] = attributes
	}

	if len(req.Screenshots) > 0 {
		images := make([]string, 0, len(req.Screenshots))
		images = append(images, req.Screenshots...)
		payload["images"] = images
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, networkError("序列化请求失败", err)
	}

	u := resolveBaseURL(config) + "/api/v1/torrents"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	a.setAuthHeaders(httpReq, config.Passkey)

	resp, err := a.doer.Client.Do(httpReq)
	if err != nil {
		return nil, uploadError("上传请求失败", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			UUID string `json:"UUID"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, &model.AppError{Code: 15001, Message: "解析响应失败", Cause: err}
	}

	if result.Code != 0 {
		return nil, &model.AppError{Code: 15001, Message: fmt.Sprintf("API 错误(code=%d): %s", result.Code, result.Message)}
	}

	uuid := result.Data.UUID
	return &model.PublishResponse{
		Success:    true,
		TorrentID:  uuid,
		DetailURL:  resolveBaseURL(config) + "/torrent/" + uuid,
		TargetSite: config.Domain,
	}, nil
}

func (a *RousiAdapter) GetTorrentInfoHash(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	detail, err := a.GetTorrentDetail(ctx, config, torrentID)
	if err != nil {
		return "", err
	}
	if detail == nil || detail.InfoHash == "" {
		return "", notFoundError("未找到 info_hash")
	}
	return detail.InfoHash, nil
}

func resolveField(fields map[string]string, keys ...string) string {
	for _, k := range keys {
		if v, ok := fields[k]; ok && v != "" {
			return v
		}
	}
	return ""
}

func (a *RousiAdapter) VerifyExists(ctx context.Context, config *model.SiteConfig, torrentID string) (bool, error) {
	results, err := a.SearchTorrents(ctx, config, torrentID, nil)
	if err != nil {
		return false, nil
	}
	for _, r := range results {
		if r.TorrentID == torrentID {
			return true, nil
		}
	}
	return false, nil
}
