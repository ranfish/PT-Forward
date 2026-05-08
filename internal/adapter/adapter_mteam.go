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

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
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
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("genDlToken HTTP %d", resp.StatusCode), nil)
	}

	body, _ := io.ReadAll(resp.Body)
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
	defer func() { _ = dlResp.Body.Close() }()

	if dlResp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("下载种子 HTTP %d", dlResp.StatusCode), nil)
	}

	return io.ReadAll(dlResp.Body)
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("HTTP %d", resp.StatusCode), nil)
	}

	return io.ReadAll(resp.Body)
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
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

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
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	detail := &model.TorrentDetail{}
	if m := regexp.MustCompile(`<title>([^<]+)</title>`).FindStringSubmatch(html); len(m) > 1 {
		detail.Title = strings.TrimSpace(m[1])
	}
	if m := regexp.MustCompile(`(?i)info_hash.*?<td[^>]*>([a-fA-F0-9]{40})`).FindStringSubmatch(html); len(m) > 1 {
		detail.InfoHash = strings.ToLower(m[1])
	}
	if m := regexp.MustCompile(`(?i)(?:大小|Size)[^<]*<[^>]*>([^<]+)`).FindStringSubmatch(html); len(m) > 1 {
		detail.Size = parseSizeStr(m[1])
	}
	if m := regexp.MustCompile(`(?i)(?:分类|Category)[^<]*<[^>]*>([^<]+)`).FindStringSubmatch(html); len(m) > 1 {
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
		return &model.DiscountResult{Level: model.DiscountNone}, nil
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	a.setAPIHeaders(req, config.APIKey)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return &model.DiscountResult{Level: model.DiscountNone}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	var rawResult struct {
		Data struct {
			Status struct {
				Discount        string `json:"discount"`
				DiscountEndTime string `json:"discountEndTime"`
			} `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &rawResult); err != nil {
		return &model.DiscountResult{Level: model.DiscountNone}, nil
	}

	var dr *model.DiscountResult
	switch strings.ToUpper(rawResult.Data.Status.Discount) {
	case "FREE":
		dr = &model.DiscountResult{Level: model.DiscountFree}
	case "_2X_FREE", "FREE_2XUP", "TWOFREE":
		dr = &model.DiscountResult{Level: model.Discount2xFree, Multiplier: 2.0}
	case "_2X", "2XUP":
		dr = &model.DiscountResult{Level: model.Discount2xUp, Multiplier: 2.0}
	case "PERCENT_50", "_2X_PERCENT_50":
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
		return &model.DiscountResult{Level: model.DiscountNone}, nil
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return &model.DiscountResult{Level: model.DiscountNone}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
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

func (a *MTeamAdapter) DetectHR(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
	if config.APIKey != "" {
		u := resolveBaseURL(config) + "/api/torrent/detail"
		req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader("id="+torrentID))
		if err != nil {
			return &model.HRResult{HasHR: false}, nil
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		a.setAPIHeaders(req, config.APIKey)

		resp, err := a.doer.Client.Do(req)
		if err != nil {
			return &model.HRResult{HasHR: false}, nil
		}
		defer func() { _ = resp.Body.Close() }()

		body, _ := io.ReadAll(resp.Body)
		var rawResult struct {
			Data struct {
				Status struct {
					HR bool `json:"hr"`
				} `json:"status"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &rawResult); err != nil {
			return &model.HRResult{HasHR: false}, nil
		}

		if rawResult.Data.Status.HR {
			return &model.HRResult{HasHR: true, SeedTimeH: config.HR.SeedTimeH()}, nil
		}
		return &model.HRResult{HasHR: false}, nil
	}

	u := resolveBaseURL(config) + "/details.php?id=" + torrentID
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return &model.HRResult{HasHR: false}, nil
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return &model.HRResult{HasHR: false}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	html := strings.ToLower(string(body))

	hasHR := strings.Contains(html, "hit and run") ||
		strings.Contains(html, "hit&run") ||
		strings.Contains(html, "h&r") ||
		strings.Contains(html, "考核")

	result := &model.HRResult{HasHR: hasHR}
	if hasHR {
		result.SeedTimeH = config.HR.SeedTimeH()
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
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
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
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)
	sl := &model.SLData{}

	if m := regexp.MustCompile(`(?i)(?:做种|Seeders?)[^<]*<[^>]*>(\d+)`).FindStringSubmatch(html); len(m) > 1 {
		sl.Seeders, _ = strconv.Atoi(m[1])
	}
	if m := regexp.MustCompile(`(?i)(?:下载|Leechers?)[^<]*<[^>]*>(\d+)`).FindStringSubmatch(html); len(m) > 1 {
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

	fileWriter, err := writer.CreateFormFile("file", "upload.torrent")
	if err != nil {
		return nil, uploadError("创建表单文件字段失败", err)
	}
	if _, err := fileWriter.Write(req.TorrentData); err != nil {
		return nil, networkError("写入种子数据失败", err)
	}

	if req.Title != "" {
		_ = writer.WriteField("name", req.Title)
	}
	if req.Subtitle != "" {
		_ = writer.WriteField("smallDescr", req.Subtitle)
	}
	if req.Description != "" {
		_ = writer.WriteField("descr", req.Description)
	}
	if req.Anonymous {
		_ = writer.WriteField("anonymous", "true")
	}
	if req.IMDbLink != "" {
		_ = writer.WriteField("imdb", req.IMDbLink)
	}
	if req.DoubanLink != "" {
		_ = writer.WriteField("douban", req.DoubanLink)
	}
	if req.MediaInfo != "" {
		_ = writer.WriteField("mediainfo", req.MediaInfo)
	}

	if v, ok := req.FormFields["category"]; ok {
		_ = writer.WriteField("category", v)
	}
	if v, ok := req.FormFields["source"]; ok {
		_ = writer.WriteField("source", v)
	}
	if v, ok := req.FormFields["medium"]; ok {
		_ = writer.WriteField("medium", v)
	}
	if v, ok := req.FormFields["standard"]; ok {
		_ = writer.WriteField("standard", v)
	}
	if v, ok := req.FormFields["videoCodec"]; ok {
		_ = writer.WriteField("videoCodec", v)
	}
	if v, ok := req.FormFields["audioCodec"]; ok {
		_ = writer.WriteField("audioCodec", v)
	}
	if v, ok := req.FormFields["team"]; ok {
		_ = writer.WriteField("team", v)
	}
	if v, ok := req.FormFields["processing"]; ok {
		_ = writer.WriteField("processing", v)
	}

	for k, v := range req.ExtraFields {
		_ = writer.WriteField(k, v)
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
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

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
	if idMatch := regexp.MustCompile(`(?:details|detail)\.php\?id=(\d+)`).FindStringSubmatch(html); len(idMatch) > 1 {
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

	fileWriter, err := writer.CreateFormFile("file", "upload.torrent")
	if err != nil {
		return nil, uploadError("创建表单文件字段失败", err)
	}
	if _, err := fileWriter.Write(req.TorrentData); err != nil {
		return nil, networkError("写入种子数据失败", err)
	}

	if req.Title != "" {
		_ = writer.WriteField("name", req.Title)
	}
	if req.Subtitle != "" {
		_ = writer.WriteField("small_descr", req.Subtitle)
	}
	if req.Description != "" {
		_ = writer.WriteField("descr", req.Description)
	}
	if req.IMDbLink != "" {
		_ = writer.WriteField("url", req.IMDbLink)
	}
	if req.Anonymous {
		_ = writer.WriteField("uplver", "1")
	}
	if req.DoubanLink != "" {
		_ = writer.WriteField("douban", req.DoubanLink)
	}
	if req.MediaInfo != "" {
		_ = writer.WriteField("mediainfo", req.MediaInfo)
	}
	for k, v := range req.FormFields {
		_ = writer.WriteField(k, v)
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
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if resp.StatusCode == http.StatusForbidden {
		return nil, &model.AppError{Code: 14003, Message: "403 Forbidden: 权限不足或 cookie 过期"}
	}

	if idMatch := regexp.MustCompile(`(?:details|detail)\.php\?id=(\d+)`).FindStringSubmatch(html); len(idMatch) > 1 {
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
	if m := regexp.MustCompile(`class="error"[^>]*>([^<]+)`).FindStringSubmatch(html); len(m) > 1 {
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
		return false, nil
	}
	for _, r := range results {
		if r.TorrentID == torrentID {
			return true, nil
		}
	}
	return false, nil
}
