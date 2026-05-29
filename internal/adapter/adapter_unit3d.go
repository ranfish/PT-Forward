package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

var (
	reUnit3DTitle      = regexp.MustCompile(`<title>([^<]+)</title>`)
	reUnit3DInfoHash   = regexp.MustCompile(`(?i)info_hash.*?([a-fA-F0-9]{40})`)
	reUnit3DSize       = regexp.MustCompile(`(?i)(?:size|大小)[^<]*<[^>]*>([^<]+)`)
	reUnit3DSeeders    = regexp.MustCompile(`(?i)(?:seeders?|做种)[^<]*<[^>]*>(\d+)`)
	reUnit3DLeechers   = regexp.MustCompile(`(?i)(?:leechers?|下载)[^<]*<[^>]*>(\d+)`)
	reUnit3DTorrentID  = regexp.MustCompile(`/torrents/(\d+)`)
	reUnit3DErrorClass = regexp.MustCompile(`class="error"[^>]*>([^<]+)`)
	reUnit3DErrorLI    = regexp.MustCompile(`<li[^>]*>([^<]*(?:error|fail|失败|错误|duplicate|already)[^<]*)</li>`)
	reUnit3DCSRFMeta   = regexp.MustCompile(`<meta\s+name="csrf-token"\s+content="([^"]+)"`)
	reUnit3DCSRFInput  = regexp.MustCompile(`<input[^>]+name="_token"[^>]+value="([^"]+)"`)
	reUnit3DIMDbID     = regexp.MustCompile(`tt\d+`)
)

type Unit3DAdapter struct {
	*GenericAdapter
	doer         *HTTPDoer
	logger       *zap.Logger
	sessionMu    sync.Mutex
	sessionReady map[string]bool
}

func NewUnit3DAdapter(doer *HTTPDoer, logger *zap.Logger) *Unit3DAdapter {
	return &Unit3DAdapter{
		GenericAdapter: NewGenericAdapter("unit3d", doer, logger),
		doer:           doer,
		logger:         logger,
		sessionReady:   make(map[string]bool),
	}
}

func (a *Unit3DAdapter) Framework() string { return "unit3d" }

func (a *Unit3DAdapter) ensureSession(ctx context.Context, config *model.SiteConfig) error {
	a.sessionMu.Lock()
	defer a.sessionMu.Unlock()

	domain := config.Domain
	if a.sessionReady[domain] {
		return nil
	}

	hasSession := false
	cookieMap := parseCookieString(config.Cookie)
	for name := range cookieMap {
		if strings.HasSuffix(name, "_session") || strings.HasPrefix(name, "XSRF-TOKEN") {
			hasSession = true
			break
		}
	}
	if hasSession {
		a.sessionReady[domain] = true
		return nil
	}

	baseURL := resolveBase(config)
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/", nil)
	if err != nil {
		return networkError("构造 session 请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return networkError("访问首页建立 session 失败", err)
	}
	drainBody(resp)

	newCookies := resp.Cookies()
	if len(newCookies) == 0 {
		a.sessionReady[domain] = true
		return nil
	}

	for _, c := range newCookies {
		cookieMap[c.Name] = c.Value
	}

	var parts []string
	for k, v := range cookieMap {
		parts = append(parts, k+"="+v)
	}
	config.Cookie = strings.Join(parts, "; ")
	a.sessionReady[domain] = true

	a.logger.Debug("Unit3D session 已建立",
		zap.String("domain", domain),
		zap.Int("new_cookies", len(newCookies)),
	)
	return nil
}

func (a *Unit3DAdapter) resetSession(domain string) {
	a.sessionMu.Lock()
	defer a.sessionMu.Unlock()
	delete(a.sessionReady, domain)
}

func parseCookieString(raw string) map[string]string {
	m := make(map[string]string)
	if raw == "" {
		return m
	}
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx := strings.Index(part, "=")
		if idx < 0 {
			continue
		}
		m[part[:idx]] = part[idx+1:]
	}
	return m
}

func (a *Unit3DAdapter) DownloadTorrent(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	if err := a.ensureSession(ctx, config); err != nil {
		return nil, err
	}
	baseURL := resolveBase(config)
	downloadPath := "/torrents/download/" + torrentID
	u := baseURL + downloadPath

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)
	if config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+config.APIKey)
	}

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, downloadError("下载失败", err)
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode == http.StatusForbidden {
		a.resetSession(config.Domain)
		return nil, &model.AppError{Code: 14003, Message: "403 Forbidden: 权限不足或 cookie 过期"}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("HTTP %d", resp.StatusCode), nil)
	}

	return io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
}

func (a *Unit3DAdapter) GetTorrentDetail(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	if err := a.ensureSession(ctx, config); err != nil {
		return nil, err
	}
	if config.APIKey != "" {
		return a.detailViaAPI(ctx, config, torrentID)
	}
	return a.detailViaWeb(ctx, config, torrentID)
}

func (a *Unit3DAdapter) detailViaAPI(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	baseURL := resolveBase(config)
	u := baseURL + "/torrents/" + url.PathEscape(torrentID)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
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

	var result struct {
		Data struct {
			Name       string `json:"name"`
			Size       int64  `json:"size"`
			InfoHash   string `json:"info_hash"`
			Category   string `json:"category"`
			Seeders    int    `json:"seeders"`
			Leechers   int    `json:"leechers"`
			Type       string `json:"type"`
			Resolution string `json:"resolution"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, parseError("解析 API 响应失败", err)
	}

	d := result.Data
	return &model.TorrentDetail{
		Title:      d.Name,
		Size:       d.Size,
		InfoHash:   strings.ToLower(d.InfoHash),
		Category:   d.Category,
		Source:     d.Type,
		Resolution: d.Resolution,
	}, nil
}

func (a *Unit3DAdapter) detailViaWeb(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	baseURL := resolveBase(config)
	u := baseURL + "/torrents/" + url.PathEscape(torrentID)

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
	if m := reUnit3DTitle.FindStringSubmatch(html); len(m) > 1 {
		parts := strings.SplitN(m[1], " :: ", 2)
		detail.Title = strings.TrimSpace(parts[0])
	}

	if m := reUnit3DInfoHash.FindStringSubmatch(html); len(m) > 1 {
		detail.InfoHash = strings.ToLower(m[1])
	}

	if m := reUnit3DSize.FindStringSubmatch(html); len(m) > 1 {
		detail.Size = parseSizeStr(m[1])
	}

	return detail, nil
}

func (a *Unit3DAdapter) DetectDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	if err := a.ensureSession(ctx, config); err != nil {
		return nil, err
	}
	baseURL := resolveBase(config)
	u := baseURL + "/torrents/" + torrentID

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("请求种子页面失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := strings.ToLower(string(body))

	if strings.Contains(html, "freeleech") || strings.Contains(html, "free") {
		if strings.Contains(html, "double upload") || strings.Contains(html, "2x") {
			return &model.DiscountResult{Level: model.Discount2xFree, Multiplier: 2.0}, nil
		}
		return &model.DiscountResult{Level: model.DiscountFree}, nil
	}
	if strings.Contains(html, "double upload") || strings.Contains(html, "2x upload") {
		return &model.DiscountResult{Level: model.Discount2xUp, Multiplier: 2.0}, nil
	}
	if strings.Contains(html, "50% free") || strings.Contains(html, "half download") {
		return &model.DiscountResult{Level: model.DiscountPercent50}, nil
	}

	return &model.DiscountResult{Level: model.DiscountNone}, nil
}

func (a *Unit3DAdapter) DetectHR(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
	if err := a.ensureSession(ctx, config); err != nil {
		return nil, err
	}
	baseURL := resolveBase(config)
	u := baseURL + "/torrents/" + torrentID

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("请求种子页面失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := strings.ToLower(string(body))

	hasHR := detectHRFromHTML(html) ||
		strings.Contains(html, "hr_rule") ||
		strings.Contains(html, "must seed")

	result := &model.HRResult{HasHR: hasHR}
	if hasHR {
		result.SeedTimeH = config.HR.SeedTimeH()
	}
	return result, nil
}

func (a *Unit3DAdapter) DetectHRAndDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, *model.DiscountResult, error) {
	if err := a.ensureSession(ctx, config); err != nil {
		return nil, nil, err
	}
	baseURL := resolveBase(config)
	u := baseURL + "/torrents/" + torrentID

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, networkError("构造请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, nil, networkError("请求种子页面失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, nil, err
	}
	html := strings.ToLower(string(body))

	hrResult := &model.HRResult{HasHR: detectHRFromHTML(html) ||
		strings.Contains(html, "hr_rule") ||
		strings.Contains(html, "must seed")}
	if hrResult.HasHR {
		hrResult.SeedTimeH = config.HR.SeedTimeH()
	}

	var discResult *model.DiscountResult
	if strings.Contains(html, "freeleech") || strings.Contains(html, "free") {
		if strings.Contains(html, "double upload") || strings.Contains(html, "2x") {
			discResult = &model.DiscountResult{Level: model.Discount2xFree, Multiplier: 2.0}
		} else {
			discResult = &model.DiscountResult{Level: model.DiscountFree}
		}
	} else if strings.Contains(html, "double upload") || strings.Contains(html, "2x upload") {
		discResult = &model.DiscountResult{Level: model.Discount2xUp, Multiplier: 2.0}
	} else if strings.Contains(html, "50% free") || strings.Contains(html, "half download") {
		discResult = &model.DiscountResult{Level: model.DiscountPercent50}
	} else {
		discResult = &model.DiscountResult{Level: model.DiscountNone}
	}

	return hrResult, discResult, nil
}

func (a *Unit3DAdapter) GetBatchSLData(ctx context.Context, config *model.SiteConfig, torrentIDs []string) (map[string]*model.SLData, error) {
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

func (a *Unit3DAdapter) GetPreciseSLData(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	if err := a.ensureSession(ctx, config); err != nil {
		return nil, err
	}
	if config.APIKey != "" {
		return a.slViaAPI(ctx, config, torrentID)
	}
	return a.slViaWeb(ctx, config, torrentID)
}

func (a *Unit3DAdapter) slViaAPI(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	baseURL := resolveBase(config)
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/torrents/"+torrentID, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
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
	var result struct {
		Data struct {
			Seeders  int `json:"seeders"`
			Leechers int `json:"leechers"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, parseError("解析响应失败", err)
	}
	return &model.SLData{Seeders: result.Data.Seeders, Leechers: result.Data.Leechers}, nil
}

func (a *Unit3DAdapter) slViaWeb(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	baseURL := resolveBase(config)
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/torrents/"+torrentID, nil)
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

	if m := reUnit3DSeeders.FindStringSubmatch(html); len(m) > 1 {
		sl.Seeders, _ = strconv.Atoi(m[1])
	}
	if m := reUnit3DLeechers.FindStringSubmatch(html); len(m) > 1 {
		sl.Leechers, _ = strconv.Atoi(m[1])
	}

	return sl, nil
}

func (a *Unit3DAdapter) UploadTorrent(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	if err := a.ensureSession(ctx, config); err != nil {
		return nil, err
	}
	if len(req.TorrentData) == 0 {
		return nil, &model.AppError{Code: 40001, Message: "种子文件数据为空"}
	}

	baseURL := resolveBase(config)
	uploadURL := baseURL + "/upload"
	if config.Paths.Upload != "" {
		uploadURL = baseURL + config.Paths.Upload
	}

	csrfToken, err := a.fetchCSRFToken(ctx, config, uploadURL)
	if err != nil {
		a.logger.Warn("获取 CSRF token 失败，尝试无 token 上传", zap.Error(err))
		csrfToken = ""
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	fw := newFieldWriter(writer)

	fileWriter, err := writer.CreateFormFile("torrent", "upload.torrent")
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
		fw.writeField("subhead", req.Subtitle)
	}
	if req.Description != "" {
		fw.writeField("description", req.Description)
	}
	if req.MediaInfo != "" {
		fw.writeField("mediainfo", req.MediaInfo)
	}
	if req.BDInfo != "" {
		fw.writeField("bdinfo", req.BDInfo)
	}
	if req.Anonymous {
		fw.writeField("anonymous", "on")
	}

	if csrfToken != "" {
		fw.writeField("_token", csrfToken)
	}

	fieldMapping := map[string]string{
		"cat":          "category_id",
		"source_sel":   "type_id",
		"standard_sel": "resolution_id",
		"category":     "category_id",
		"source":       "type_id",
		"resolution":   "resolution_id",
		"codec":        "codec_id",
	}
	for srcType, targetName := range fieldMapping {
		if v, ok := req.FormFields[srcType]; ok {
			fw.writeField(targetName, v)
		}
	}

	if imdbID := extractIMDbID(req.IMDbLink); imdbID != "" {
		fw.writeField("imdb_id", imdbID)
	}
	if tmdbID, ok := req.ExtraFields["tmdb_id"]; ok {
		fw.writeField("tmdb_id", tmdbID)
	}
	if malID, ok := req.ExtraFields["mal_id"]; ok {
		fw.writeField("mal_id", malID)
	}
	if tvdbID, ok := req.ExtraFields["tvdb_id"]; ok {
		fw.writeField("tvdb_id", tvdbID)
	}
	if bgmID, ok := req.ExtraFields["bgm_id"]; ok {
		fw.writeField("bgm_id", bgmID)
	}

	for k, v := range req.FormFields {
		switch k {
		case "cat", "source_sel", "standard_sel", "category", "source", "resolution", "codec":
			continue
		}
		fw.writeField(k, v)
	}
	for k, v := range req.ExtraFields {
		switch k {
		case "tmdb_id", "mal_id", "tvdb_id", "bgm_id":
			continue
		}
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
		a.resetSession(config.Domain)
		return nil, &model.AppError{Code: 14003, Message: "403 Forbidden: 权限不足或 CSRF token 过期"}
	}

	if resp.StatusCode == 419 {
		a.resetSession(config.Domain)
		return nil, &model.AppError{Code: 14003, Message: "419 CSRF token 过期，需要重建 session"}
	}

	if idMatch := reUnit3DTorrentID.FindStringSubmatch(html); len(idMatch) > 1 {
		torrentID := idMatch[1]
		return &model.PublishResponse{
			Success:    true,
			TorrentID:  torrentID,
			DetailURL:  baseURL + "/torrents/" + torrentID,
			TargetSite: config.Domain,
		}, nil
	}

	if strings.Contains(html, "success") || strings.Contains(html, "成功") {
		return &model.PublishResponse{Success: true, TargetSite: config.Domain}, nil
	}

	errMsg := "上传失败"
	if m := reUnit3DErrorClass.FindStringSubmatch(html); len(m) > 1 {
		errMsg = strings.TrimSpace(m[1])
	} else if m := reUnit3DErrorLI.FindStringSubmatch(html); len(m) > 1 {
		errMsg = strings.TrimSpace(m[1])
	}

	return nil, &model.AppError{Code: 15001, Message: errMsg}
}

func (a *Unit3DAdapter) fetchCSRFToken(ctx context.Context, config *model.SiteConfig, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return "", err
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return "", networkError("请求页面失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return "", err
	}
	html := string(body)

	if m := reUnit3DCSRFMeta.FindStringSubmatch(html); len(m) > 1 {
		return m[1], nil
	}

	if m := reUnit3DCSRFInput.FindStringSubmatch(html); len(m) > 1 {
		return m[1], nil
	}

	return "", notFoundError("未找到 CSRF token")
}

func extractIMDbID(link string) string {
	if link == "" {
		return ""
	}
	m := reUnit3DIMDbID.FindString(link)
	return m
}

func (a *Unit3DAdapter) GetTorrentInfoHash(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	detail, err := a.GetTorrentDetail(ctx, config, torrentID)
	if err != nil {
		return "", err
	}
	if detail.InfoHash == "" {
		return "", notFoundError("未找到 info_hash")
	}
	return detail.InfoHash, nil
}

func resolveBase(config *model.SiteConfig) string {
	u := config.Domain
	if !strings.HasPrefix(u, "http") {
		u = "https://" + u
	}
	return strings.TrimRight(u, "/")
}

func (a *Unit3DAdapter) VerifyExists(ctx context.Context, config *model.SiteConfig, torrentID string) (bool, error) {
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

func (a *Unit3DAdapter) FetchUserStats(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	if config.APIKey != "" || config.BearerToken != "" {
		return a.fetchUserStatsAPI(ctx, config)
	}
	return a.fetchUserStatsHTML(ctx, config)
}

func (a *Unit3DAdapter) fetchUserStatsAPI(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	u := resolveBase(config) + "/api/user"
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+config.APIKey)
	} else if config.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+config.BearerToken)
	}
	if config.Cookie != "" {
		req.Header.Set("Cookie", config.Cookie)
	}

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer httpclient.DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var raw struct {
		ID          flexInt `json:"id"`
		Username    string  `json:"username"`
		Uploaded    flexInt `json:"uploaded"`
		Downloaded  flexInt `json:"downloaded"`
		Ratio       float64 `json:"ratio"`
		Seedbonus   float64 `json:"seedbonus"`
		Seeding     int     `json:"seeding"`
		Group       struct {
			Name string `json:"name"`
		} `json:"group"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("解析用户信息失败: %w", err)
	}

	return &model.UserStatsResult{
		Username:     raw.Username,
		UserClass:    raw.Group.Name,
		UploadBytes:  int64(raw.Uploaded),
		DownloadBytes: int64(raw.Downloaded),
		Ratio:        raw.Ratio,
		BonusPoints:  raw.Seedbonus,
		SeedingCount: raw.Seeding,
	}, nil
}

func (a *Unit3DAdapter) fetchUserStatsHTML(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	pageURL := config.Domain + "/"
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
	if result.DownloadBytes > 0 && result.UploadBytes > 0 {
		result.Ratio = float64(result.UploadBytes) / float64(result.DownloadBytes)
	}
	return result, nil
}
