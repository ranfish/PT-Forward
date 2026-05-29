package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

var (
	reMTeamTitle      = regexp.MustCompile(`<title>([^<]+)</title>`)
	reMTeamInfoHash   = regexp.MustCompile(`(?i)info_hash.*?<td[^>]*>([a-fA-F0-9]{40})`)
	reMTeamSize       = regexp.MustCompile(`(?i)(?:大小|Size)[^<]*<[^>]*>([^<]+)`)
	reMTeamCategory   = regexp.MustCompile(`(?i)(?:分类|Category)[^<]*<[^>]*>([^<]+)`)
	reMTeamSeeders    = regexp.MustCompile(`(?i)(?:做种|Seeders?)[^<]*<[^>]*>(\d+)`)
	reMTeamLeechers   = regexp.MustCompile(`(?i)(?:下载|Leechers?)[^<]*<[^>]*>(\d+)`)
	reMTeamDetailID   = regexp.MustCompile(`(?:details|detail)\.php\?id=(\d+)`)
	reMTeamErrorClass = regexp.MustCompile(`class="error"[^>]*>([^<]+)`)
)

const mteamUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

type flexInt int64

func (f *flexInt) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		*f = 0
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			*f = 0
			return nil
		}
		*f = flexInt(n)
		return nil
	}
	var n int64
	if err := json.Unmarshal(data, &n); err != nil {
		*f = 0
		return nil
	}
	*f = flexInt(n)
	return nil
}

type MTeamAdapter struct {
	*GenericAdapter
	doer   *HTTPDoer
	logger *zap.Logger
}

func NewMTeamAdapter(doer *HTTPDoer, logger *zap.Logger) *MTeamAdapter {
	return &MTeamAdapter{
		GenericAdapter: NewGenericAdapter("mteam", doer, logger),
		doer:           doer,
		logger:         logger,
	}
}

func (a *MTeamAdapter) Framework() string { return "mteam" }

func (a *MTeamAdapter) SearchTorrents(ctx context.Context, config *model.SiteConfig, keyword string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
	if config.APIKey != "" {
		return a.searchViaAPI(ctx, config, keyword)
	}
	return a.GenericAdapter.SearchTorrents(ctx, config, keyword, opts)
}

func (a *MTeamAdapter) searchViaAPI(ctx context.Context, config *model.SiteConfig, keyword string) ([]*model.SeedingSearchResult, error) {
	u := resolveBaseURL(config) + "/api/torrent/search"
	payload, _ := json.Marshal(map[string]interface{}{
		"keyword":    keyword,
		"mode":       "normal",
		"pageNumber": 1,
		"pageSize":   20,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(payload))
	if err != nil {
		return nil, searchError("构造搜索请求失败", err)
	}
	req.Header.Set("Content-Type", "application/json")
	a.setAPIHeaders(req, config.APIKey)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, searchError("搜索请求失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code json.Number `json:"code"`
		Data struct {
			Data []struct {
				ID     string  `json:"id"`
				Name   string  `json:"name"`
				Size   flexInt `json:"size"`
				Status struct {
					Seeders  flexInt `json:"seeders"`
					Leechers flexInt `json:"leechers"`
					Discount string  `json:"discount"`
				} `json:"status"`
			} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, parseError("解析搜索结果失败", err)
	}

	var results []*model.SeedingSearchResult
	for _, item := range result.Data.Data {
		results = append(results, &model.SeedingSearchResult{
			TorrentID: item.ID,
			Title:     item.Name,
			Size:      int64(item.Size),
			Seeders:   int(item.Status.Seeders),
			Leechers:  int(item.Status.Leechers),
		})
	}
	return results, nil
}

func (a *MTeamAdapter) setAPIHeaders(req *http.Request, apiKey string) {
	req.Header.Set("User-Agent", mteamUserAgent)
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Accept", "application/json")
}

func (a *MTeamAdapter) DownloadTorrent(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	if config.APIKey != "" {
		return a.downloadViaAPI(ctx, config, torrentID)
	}
	return a.downloadViaWeb(ctx, config, torrentID)
}

func (a *MTeamAdapter) downloadViaAPI(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	tokenURL := resolveBaseURL(config) + "/api/torrent/genDlToken"
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader("id="+torrentID))
	if err != nil {
		return nil, networkError("构造 genDlToken 请求失败", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	a.setAPIHeaders(req, config.APIKey)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("genDlToken 请求失败", err)
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("genDlToken HTTP %d", resp.StatusCode), nil)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return nil, networkError("读取 genDlToken 响应失败", err)
	}
	var tokenResp struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, parseError("解析 genDlToken 响应失败", err)
	}
	if tokenResp.Data == "" {
		return nil, downloadError("genDlToken 返回空下载链接", nil)
	}

	dlReq, err := http.NewRequestWithContext(ctx, "GET", tokenResp.Data, nil)
	if err != nil {
		return nil, networkError("构造下载请求失败", err)
	}
	a.setAPIHeaders(dlReq, config.APIKey)

	dlResp, err := a.doer.Client.Do(dlReq)
	if err != nil {
		return nil, downloadError("下载种子失败", err)
	}
	defer func() { drainBody(dlResp) }()

	if dlResp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("下载种子 HTTP %d", dlResp.StatusCode), nil)
	}

	return io.ReadAll(io.LimitReader(dlResp.Body, 50*1024*1024))
}

func (a *MTeamAdapter) downloadViaWeb(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	u := resolveBaseURL(config) + "/download.php?id=" + torrentID
	if config.Passkey != "" {
		u += "&passkey=" + config.Passkey
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, downloadError("下载失败", err)
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("HTTP %d", resp.StatusCode), nil)
	}

	return io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
}

func (a *MTeamAdapter) GetTorrentDetail(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	if config.APIKey != "" {
		return a.detailViaAPI(ctx, config, torrentID)
	}
	return a.detailViaWeb(ctx, config, torrentID)
}

func (a *MTeamAdapter) detailViaAPI(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	u := resolveBaseURL(config) + "/api/torrent/detail"
	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader("id="+torrentID))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	a.setAPIHeaders(req, config.APIKey)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code json.Number `json:"code"`
		Data struct {
			Name        string   `json:"name"`
			SmallDescr  string   `json:"smallDescr"`
			Description string   `json:"description"`
			Size        flexInt  `json:"size"`
			InfoHash    string   `json:"infoHash"`
			Category    string   `json:"category"`
			Source      string   `json:"source"`
			Standard    string   `json:"standard"`
			VideoCodec  string   `json:"videoCodec"`
			AudioCodec  string   `json:"audioCodec"`
			Team        string   `json:"team"`
			Imdb        string   `json:"imdb"`
			Douban      string   `json:"douban"`
			MediaInfo   string   `json:"mediaInfo"`
			Tags        []string `json:"tags"`
			Status      struct {
				Discount        string  `json:"discount"`
				DiscountEndTime string  `json:"discountEndTime"`
				Seeders         flexInt `json:"seeders"`
				Leechers        flexInt `json:"leechers"`
				HR              bool    `json:"hr"`
			} `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, parseError("解析 API 响应失败", err)
	}

	d := result.Data
	return &model.TorrentDetail{
		Title:        d.Name,
		Description:  d.Description,
		Size:         int64(d.Size),
		InfoHash:     strings.ToLower(d.InfoHash),
		Category:     d.Category,
		Source:       d.Source,
		Resolution:   d.Standard,
		Codec:        d.VideoCodec,
		ReleaseGroup: d.Team,
		MediaInfo:    d.MediaInfo,
		IMDbID:       d.Imdb,
		Tags:         d.Tags,
	}, nil
}

func (a *MTeamAdapter) detailViaWeb(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	u := resolveBaseURL(config) + "/details.php?id=" + torrentID
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := string(body)

	detail := &model.TorrentDetail{}
	if m := reMTeamTitle.FindStringSubmatch(html); len(m) > 1 {
		detail.Title = strings.TrimSpace(m[1])
	}
	if m := reMTeamInfoHash.FindStringSubmatch(html); len(m) > 1 {
		detail.InfoHash = strings.ToLower(m[1])
	}
	if m := reMTeamSize.FindStringSubmatch(html); len(m) > 1 {
		detail.Size = parseSizeStr(m[1])
	}
	if m := reMTeamCategory.FindStringSubmatch(html); len(m) > 1 {
		detail.Category = strings.TrimSpace(m[1])
	}
	detail.Tags = extractTags(html)

	return detail, nil
}

func (a *MTeamAdapter) DetectDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	if config.APIKey != "" {
		return a.discountViaAPI(ctx, config, torrentID)
	}
	return a.discountViaWeb(ctx, config, torrentID)
}

func (a *MTeamAdapter) discountViaAPI(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	u := resolveBaseURL(config) + "/api/torrent/detail"
	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader("id="+torrentID))
	if err != nil {
		return nil, networkError("构造优惠检测请求失败", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	a.setAPIHeaders(req, config.APIKey)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("优惠检测请求失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	var rawResult struct {
		Data struct {
			Status struct {
				Discount        string `json:"discount"`
				DiscountEndTime string `json:"discountEndTime"`
			} `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &rawResult); err != nil {
		return nil, parseError("解析优惠检测结果失败", err)
	}

	var dr *model.DiscountResult
	switch strings.ToUpper(rawResult.Data.Status.Discount) {
	case "FREE":
		dr = &model.DiscountResult{Level: model.DiscountFree}
	case "_2X_FREE", "FREE_2XUP", "TWOFREE":
		dr = &model.DiscountResult{Level: model.Discount2xFree, Multiplier: 2.0}
	case "_2X", "2XUP":
		dr = &model.DiscountResult{Level: model.Discount2xUp, Multiplier: 2.0}
	case "_2X_PERCENT_50":
		dr = &model.DiscountResult{Level: model.Discount2x50, Multiplier: 2.0}
	case "PERCENT_50":
		dr = &model.DiscountResult{Level: model.DiscountPercent50}
	case "PERCENT_70", "_2X_PERCENT_70":
		dr = &model.DiscountResult{Level: model.DiscountPercent70}
	case "PERCENT_30":
		dr = &model.DiscountResult{Level: model.DiscountPercent30}
	case "NORMAL":
		dr = &model.DiscountResult{Level: model.DiscountNone}
	default:
		dr = &model.DiscountResult{Level: model.DiscountNone}
	}
	if dr.Level != model.DiscountNone && rawResult.Data.Status.DiscountEndTime != "" {
		if t, err := time.Parse(time.RFC3339, rawResult.Data.Status.DiscountEndTime); err == nil {
			dr.FreeEndAt = &t
		}
	}
	return dr, nil
}

func (a *MTeamAdapter) discountViaWeb(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	u := resolveBaseURL(config) + "/details.php?id=" + torrentID
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造优惠检测请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("优惠检测请求失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := strings.ToLower(string(body))

	if strings.Contains(html, "pro_free2up") {
		return &model.DiscountResult{Level: model.Discount2xFree, Multiplier: 2.0}, nil
	}
	if strings.Contains(html, "pro_2up") {
		return &model.DiscountResult{Level: model.Discount2xUp, Multiplier: 2.0}, nil
	}
	if strings.Contains(html, "pro_free") || strings.Contains(html, "免费") {
		return &model.DiscountResult{Level: model.DiscountFree}, nil
	}
	if strings.Contains(html, "pro_50p") {
		return &model.DiscountResult{Level: model.DiscountPercent50}, nil
	}

	return &model.DiscountResult{Level: model.DiscountNone}, nil
}

func (a *MTeamAdapter) DetectHRAndDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, *model.DiscountResult, error) {
	if config.APIKey != "" {
		return a.detectHRAndDiscountViaAPI(ctx, config, torrentID)
	}
	hr, err := a.DetectHR(ctx, config, torrentID)
	if err != nil {
		return nil, nil, err
	}
	disc, err := a.DetectDiscount(ctx, config, torrentID)
	if err != nil {
		return hr, nil, err
	}
	return hr, disc, nil
}

func (a *MTeamAdapter) detectHRAndDiscountViaAPI(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, *model.DiscountResult, error) {
	u := resolveBaseURL(config) + "/api/torrent/detail"
	a.logger.Debug("mteam combined detect: sending request",
		zap.String("url", u),
		zap.String("torrent", torrentID),
		zap.Bool("hasAPIKey", config.APIKey != ""))
	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader("id="+torrentID))
	if err != nil {
		return nil, nil, networkError("构造合并检测请求失败", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	a.setAPIHeaders(req, config.APIKey)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, nil, networkError("合并检测请求失败", err)
	}
	defer func() { drainBody(resp) }()
	a.logger.Debug("mteam combined detect: got response",
		zap.String("torrent", torrentID),
		zap.Int("status", resp.StatusCode))

	body, err := readBody(resp)
	if err != nil {
		return nil, nil, err
	}
	var rawResult struct {
		Data struct {
			Status struct {
				Discount        string `json:"discount"`
				DiscountEndTime string `json:"discountEndTime"`
				HR              bool   `json:"hr"`
			} `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &rawResult); err != nil {
		return nil, nil, parseError("解析合并检测结果失败", err)
	}

	hrResult := &model.HRResult{HasHR: rawResult.Data.Status.HR}
	if hrResult.HasHR {
		hrResult.SeedTimeH = config.HR.SeedTimeH()
	}

	var dr *model.DiscountResult
	switch strings.ToUpper(rawResult.Data.Status.Discount) {
	case "FREE":
		dr = &model.DiscountResult{Level: model.DiscountFree}
	case "_2X_FREE", "FREE_2XUP", "TWOFREE":
		dr = &model.DiscountResult{Level: model.Discount2xFree, Multiplier: 2.0}
	case "_2X", "2XUP":
		dr = &model.DiscountResult{Level: model.Discount2xUp, Multiplier: 2.0}
	case "_2X_PERCENT_50":
		dr = &model.DiscountResult{Level: model.Discount2x50, Multiplier: 2.0}
	case "PERCENT_50":
		dr = &model.DiscountResult{Level: model.DiscountPercent50}
	case "PERCENT_70", "_2X_PERCENT_70":
		dr = &model.DiscountResult{Level: model.DiscountPercent70}
	case "PERCENT_30":
		dr = &model.DiscountResult{Level: model.DiscountPercent30}
	case "NORMAL":
		dr = &model.DiscountResult{Level: model.DiscountNone}
	default:
		dr = &model.DiscountResult{Level: model.DiscountNone}
	}
	if dr.Level != model.DiscountNone && rawResult.Data.Status.DiscountEndTime != "" {
		if t, err := time.Parse(time.RFC3339, rawResult.Data.Status.DiscountEndTime); err == nil {
			dr.FreeEndAt = &t
		}
	}
	return hrResult, dr, nil
}

func (a *MTeamAdapter) DetectHR(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
	if config.APIKey != "" {
		u := resolveBaseURL(config) + "/api/torrent/detail"
		req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader("id="+torrentID))
		if err != nil {
			return nil, networkError("构造 HR 检测请求失败", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		a.setAPIHeaders(req, config.APIKey)

		resp, err := a.doer.Client.Do(req)
		if err != nil {
			return nil, networkError("HR 检测请求失败", err)
		}
		defer func() { drainBody(resp) }()

		body, err := readBody(resp)
		if err != nil {
			return nil, err
		}
		var rawResult struct {
			Data struct {
				Status struct {
					HR bool `json:"hr"`
				} `json:"status"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &rawResult); err != nil {
			return nil, parseError("解析 HR 检测结果失败", err)
		}

		if rawResult.Data.Status.HR {
			return &model.HRResult{HasHR: true, SeedTimeH: config.HR.SeedTimeH()}, nil
		}
		return &model.HRResult{HasHR: false}, nil
	}

	u := resolveBaseURL(config) + "/details.php?id=" + torrentID
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造 HR 检测请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("HR 检测请求失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := strings.ToLower(string(body))

	hasHR := detectHRFromHTML(html)

	result := &model.HRResult{HasHR: hasHR}
	if hasHR {
		result.SeedTimeH = config.HR.SeedTimeH()
	}
	return result, nil
}

func (a *MTeamAdapter) GetBatchSLData(ctx context.Context, config *model.SiteConfig, torrentIDs []string) (map[string]*model.SLData, error) {
	result := make(map[string]*model.SLData, len(torrentIDs))
	for _, id := range torrentIDs {
		sl, err := a.GetPreciseSLData(ctx, config, id)
		if err != nil {
			a.logger.Warn("获取SL数据失败", zap.String("torrentID", id), zap.Error(err))
			continue
		}
		result[id] = sl
	}
	return result, nil
}

func (a *MTeamAdapter) GetPreciseSLData(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	if config.APIKey != "" {
		return a.slViaAPI(ctx, config, torrentID)
	}
	return a.slViaWeb(ctx, config, torrentID)
}

func (a *MTeamAdapter) slViaAPI(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	u := resolveBaseURL(config) + "/api/torrent/detail"
	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader("id="+torrentID))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	a.setAPIHeaders(req, config.APIKey)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	var result struct {
		Data struct {
			Status struct {
				Seeders  flexInt `json:"seeders"`
				Leechers flexInt `json:"leechers"`
			} `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, parseError("解析响应失败", err)
	}

	return &model.SLData{Seeders: int(result.Data.Status.Seeders), Leechers: int(result.Data.Status.Leechers)}, nil
}

func (a *MTeamAdapter) slViaWeb(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	u := resolveBaseURL(config) + "/details.php?id=" + torrentID
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := string(body)
	sl := &model.SLData{}

	if m := reMTeamSeeders.FindStringSubmatch(html); len(m) > 1 {
		sl.Seeders, _ = strconv.Atoi(m[1])
	}
	if m := reMTeamLeechers.FindStringSubmatch(html); len(m) > 1 {
		sl.Leechers, _ = strconv.Atoi(m[1])
	}

	return sl, nil
}

func (a *MTeamAdapter) UploadTorrent(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	if len(req.TorrentData) == 0 {
		return nil, &model.AppError{Code: 40001, Message: "种子文件数据为空"}
	}

	if config.APIKey != "" {
		return a.uploadViaAPI(ctx, config, req)
	}
	return a.uploadViaWeb(ctx, config, req)
}

func (a *MTeamAdapter) uploadViaAPI(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	baseURL := resolveBaseURL(config)
	uploadURL := baseURL + "/api/torrent/createOredit"

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	fw := newFieldWriter(writer)

	fileWriter, err := writer.CreateFormFile("file", "upload.torrent")
	if err != nil {
		return nil, uploadError("创建表单文件字段失败", err)
	}
	if _, err := fileWriter.Write(req.TorrentData); err != nil {
		return nil, networkError("写入种子数据失败", err)
	}

	if req.Title != "" {
		fw.writeField("name", req.Title)
	}
	if req.Subtitle != "" {
		fw.writeField("smallDescr", req.Subtitle)
	}
	if req.Description != "" {
		fw.writeField("descr", req.Description)
	}
	if req.Anonymous {
		fw.writeField("anonymous", "true")
	}
	if req.IMDbLink != "" {
		fw.writeField("imdb", req.IMDbLink)
	}
	if req.DoubanLink != "" {
		fw.writeField("douban", req.DoubanLink)
	}
	if req.MediaInfo != "" {
		fw.writeField("mediainfo", req.MediaInfo)
	}

	if v, ok := req.FormFields["category"]; ok {
		fw.writeField("category", v)
	}
	if v, ok := req.FormFields["source"]; ok {
		fw.writeField("source", v)
	}
	if v, ok := req.FormFields["medium"]; ok {
		fw.writeField("medium", v)
	}
	if v, ok := req.FormFields["standard"]; ok {
		fw.writeField("standard", v)
	}
	if v, ok := req.FormFields["videoCodec"]; ok {
		fw.writeField("videoCodec", v)
	}
	if v, ok := req.FormFields["audioCodec"]; ok {
		fw.writeField("audioCodec", v)
	}
	if v, ok := req.FormFields["team"]; ok {
		fw.writeField("team", v)
	}
	if v, ok := req.FormFields["processing"]; ok {
		fw.writeField("processing", v)
	}

	for k, v := range req.ExtraFields {
		fw.writeField(k, v)
	}

	if err := fw.hasError(); err != nil {
		return nil, fmt.Errorf("write form field: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, networkError("关闭 multipart writer 失败", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", uploadURL, &buf)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	a.setAPIHeaders(httpReq, config.APIKey)

	resp, err := a.doer.Client.Do(httpReq)
	if err != nil {
		return nil, uploadError("上传请求失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusForbidden {
		return nil, &model.AppError{Code: 14003, Message: "403 Forbidden: API Key 无效或权限不足"}
	}

	var apiResp struct {
		Data struct {
			ID interface{} `json:"id"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &apiResp); err == nil {
		if apiResp.Data.ID != nil {
			torrentID := fmt.Sprintf("%v", apiResp.Data.ID)
			return &model.PublishResponse{
				Success:    true,
				TorrentID:  torrentID,
				DetailURL:  baseURL + "/detail/" + torrentID,
				TargetSite: config.Domain,
			}, nil
		}
		if apiResp.Message != "" {
			return &model.PublishResponse{Success: false, ErrorMessage: apiResp.Message}, nil
		}
	}

	html := string(body)
	if idMatch := reMTeamDetailID.FindStringSubmatch(html); len(idMatch) > 1 {
		torrentID := idMatch[1]
		return &model.PublishResponse{
			Success:    true,
			TorrentID:  torrentID,
			DetailURL:  baseURL + "/details.php?id=" + torrentID,
			TargetSite: config.Domain,
		}, nil
	}

	if strings.Contains(html, "成功") || strings.Contains(html, "succeeded") {
		return &model.PublishResponse{Success: true, TargetSite: config.Domain}, nil
	}

	return nil, &model.AppError{Code: 15001, Message: fmt.Sprintf("上传失败: HTTP %d", resp.StatusCode)}
}

func (a *MTeamAdapter) uploadViaWeb(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	baseURL := resolveBaseURL(config)
	uploadPath := "/upload.php"
	if config.Paths.Upload != "" {
		uploadPath = config.Paths.Upload
	}
	uploadURL := baseURL + uploadPath

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	fw := newFieldWriter(writer)

	fileWriter, err := writer.CreateFormFile("file", "upload.torrent")
	if err != nil {
		return nil, uploadError("创建表单文件字段失败", err)
	}
	if _, err := fileWriter.Write(req.TorrentData); err != nil {
		return nil, networkError("写入种子数据失败", err)
	}

	if req.Title != "" {
		fw.writeField("name", req.Title)
	}
	if req.Subtitle != "" {
		fw.writeField("small_descr", req.Subtitle)
	}
	if req.Description != "" {
		fw.writeField("descr", req.Description)
	}
	if req.IMDbLink != "" {
		fw.writeField("url", req.IMDbLink)
	}
	if req.Anonymous {
		fw.writeField("uplver", "1")
	}
	if req.DoubanLink != "" {
		fw.writeField("douban", req.DoubanLink)
	}
	if req.MediaInfo != "" {
		fw.writeField("mediainfo", req.MediaInfo)
	}
	for k, v := range req.FormFields {
		fw.writeField(k, v)
	}

	if err := fw.hasError(); err != nil {
		return nil, fmt.Errorf("write form field: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, networkError("关闭 multipart writer 失败", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", uploadURL, &buf)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	setCommonHeaders(httpReq, config.Cookie)

	resp, err := a.doer.Client.Do(httpReq)
	if err != nil {
		return nil, uploadError("上传请求失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := string(body)

	if resp.StatusCode == http.StatusForbidden {
		return nil, &model.AppError{Code: 14003, Message: "403 Forbidden: 权限不足或 cookie 过期"}
	}

	if idMatch := reMTeamDetailID.FindStringSubmatch(html); len(idMatch) > 1 {
		torrentID := idMatch[1]
		return &model.PublishResponse{
			Success:    true,
			TorrentID:  torrentID,
			DetailURL:  baseURL + "/details.php?id=" + torrentID,
			TargetSite: config.Domain,
		}, nil
	}

	if strings.Contains(html, "成功") || strings.Contains(html, "succeeded") {
		return &model.PublishResponse{Success: true, TargetSite: config.Domain}, nil
	}

	errMsg := "上传失败"
	if m := reMTeamErrorClass.FindStringSubmatch(html); len(m) > 1 {
		errMsg = strings.TrimSpace(m[1])
	}

	return nil, &model.AppError{Code: 15001, Message: errMsg}
}

func (a *MTeamAdapter) GetTorrentInfoHash(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	detail, err := a.GetTorrentDetail(ctx, config, torrentID)
	if err != nil {
		return "", err
	}
	if detail.InfoHash == "" {
		return "", notFoundError("未找到 info_hash")
	}
	return detail.InfoHash, nil
}

func resolveBaseURL(config *model.SiteConfig) string {
	u := config.Domain
	if !strings.HasPrefix(u, "http") {
		u = "https://" + u
	}
	return strings.TrimRight(u, "/")
}

func (a *MTeamAdapter) VerifyExists(ctx context.Context, config *model.SiteConfig, torrentID string) (bool, error) {
	results, err := a.SearchTorrents(ctx, config, torrentID, nil)
	if err != nil {
		return false, fmt.Errorf("search for verify exists %q: %w", torrentID, err)
	}
	for _, r := range results {
		if r.TorrentID == torrentID {
			return true, nil
		}
	}
	return false, nil
}

func (a *MTeamAdapter) FetchUserStats(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	if config.APIKey == "" {
		return a.fetchUserStatsHTML(ctx, config)
	}
	return a.fetchUserStatsAPI(ctx, config)
}

func (a *MTeamAdapter) fetchUserStatsAPI(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	u := resolveBaseURL(config) + "/api/member/profile"
	req, err := http.NewRequestWithContext(ctx, "POST", u, nil)
	if err != nil {
		return nil, err
	}
	a.setAPIHeaders(req, config.APIKey)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	var result struct {
		Message string `json:"message"`
		Data    struct {
			ID           flexInt `json:"id"`
			Username     string  `json:"username"`
			MemberCount  struct {
				Uploaded   string `json:"uploaded"`
				Downloaded string `json:"downloaded"`
				Seeding    string `json:"seeding"`
				Bonus      string `json:"bonus"`
				ShareRate  string `json:"shareRate"`
			} `json:"memberCount"`
			MemberStatus struct {
				LastLogin string `json:"lastLogin"`
			} `json:"memberStatus"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析用户信息失败: %w", err)
	}

	stats := &model.UserStatsResult{
		Username: result.Data.Username,
	}
	stats.UploadBytes = parseSizeString(result.Data.MemberCount.Uploaded)
	stats.DownloadBytes = parseSizeString(result.Data.MemberCount.Downloaded)
	stats.SeedingCount, _ = strconv.Atoi(result.Data.MemberCount.Seeding)
	stats.BonusPoints, _ = strconv.ParseFloat(result.Data.MemberCount.Bonus, 64)
	stats.Ratio, _ = strconv.ParseFloat(result.Data.MemberCount.ShareRate, 64)
	if stats.Ratio == 0 && stats.DownloadBytes > 0 {
		stats.Ratio = float64(stats.UploadBytes) / float64(stats.DownloadBytes)
	}
	return stats, nil
}

func (a *MTeamAdapter) fetchUserStatsHTML(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	pageURL := config.Domain + "/index.php"
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return nil, err
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer httpclient.DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	htmlBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	html := string(htmlBytes)

	result := &model.UserStatsResult{}
	if m := reNexusUsername.FindStringSubmatch(html); len(m) > 2 {
		result.Username = strings.TrimSpace(m[2])
	} else if m := reNexusUsernameAlt.FindStringSubmatch(html); len(m) > 2 {
		result.Username = strings.TrimSpace(m[2])
	}
	if m := reNexusFontUploaded.FindStringSubmatch(html); len(m) > 1 {
		result.UploadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reNexusLabelUpload.FindStringSubmatch(html); len(m) > 1 {
		result.UploadBytes = parseSizeString(cleanText(m[1]))
	}
	if m := reNexusFontDownloaded.FindStringSubmatch(html); len(m) > 1 {
		result.DownloadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reNexusLabelDownload.FindStringSubmatch(html); len(m) > 1 {
		result.DownloadBytes = parseSizeString(cleanText(m[1]))
	}
	return result, nil
}
