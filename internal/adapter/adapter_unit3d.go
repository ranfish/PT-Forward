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
	"sync"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
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
		return fmt.Errorf("构造 session 请求失败: %w", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return fmt.Errorf("访问首页建立 session 失败: %w", err)
	}
	func() { _ = resp.Body.Close() }()

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
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	setCommonHeaders(req, config.Cookie)
	if config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+config.APIKey)
	}

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("403 Forbidden: 权限不足或 cookie 过期")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
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
	u := baseURL + "/api/torrents/" + torrentID

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+config.APIKey)
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

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
		return nil, fmt.Errorf("解析 API 响应失败: %w", err)
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
	u := baseURL + "/torrents/" + torrentID

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
		parts := strings.SplitN(m[1], " :: ", 2)
		detail.Title = strings.TrimSpace(parts[0])
	}

	if m := regexp.MustCompile(`(?i)info_hash.*?([a-fA-F0-9]{40})`).FindStringSubmatch(html); len(m) > 1 {
		detail.InfoHash = strings.ToLower(m[1])
	}

	if m := regexp.MustCompile(`(?i)(?:size|大小)[^<]*<[^>]*>([^<]+)`).FindStringSubmatch(html); len(m) > 1 {
		detail.Size = parseSizeStr(m[1])
	}

	return detail, nil
}

func (a *Unit3DAdapter) DetectDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	if err := a.ensureSession(ctx, config); err != nil {
		return &model.DiscountResult{Level: model.DiscountNone}, nil
	}
	baseURL := resolveBase(config)
	u := baseURL + "/torrents/" + torrentID

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
		return &model.HRResult{HasHR: false}, nil
	}
	baseURL := resolveBase(config)
	u := baseURL + "/torrents/" + torrentID

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
		strings.Contains(html, "hr_rule") ||
		strings.Contains(html, "must seed")

	result := &model.HRResult{HasHR: hasHR}
	if hasHR {
		result.SeedTimeH = 72
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
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Data struct {
			Seeders  int `json:"seeders"`
			Leechers int `json:"leechers"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
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
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)
	sl := &model.SLData{}

	if m := regexp.MustCompile(`(?i)(?:seeders?|做种)[^<]*<[^>]*>(\d+)`).FindStringSubmatch(html); len(m) > 1 {
		sl.Seeders, _ = strconv.Atoi(m[1])
	}
	if m := regexp.MustCompile(`(?i)(?:leechers?|下载)[^<]*<[^>]*>(\d+)`).FindStringSubmatch(html); len(m) > 1 {
		sl.Leechers, _ = strconv.Atoi(m[1])
	}

	return sl, nil
}

func (a *Unit3DAdapter) UploadTorrent(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	if err := a.ensureSession(ctx, config); err != nil {
		return nil, err
	}
	if len(req.TorrentData) == 0 {
		return nil, fmt.Errorf("种子文件数据为空")
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

	fileWriter, err := writer.CreateFormFile("torrent", "upload.torrent")
	if err != nil {
		return nil, fmt.Errorf("创建表单文件字段失败: %w", err)
	}
	if _, err := fileWriter.Write(req.TorrentData); err != nil {
		return nil, fmt.Errorf("写入种子数据失败: %w", err)
	}

	if req.Title != "" {
		_ = writer.WriteField("name", req.Title)
	}
	if req.Subtitle != "" {
		_ = writer.WriteField("subhead", req.Subtitle)
	}
	if req.Description != "" {
		_ = writer.WriteField("description", req.Description)
	}
	if req.MediaInfo != "" {
		_ = writer.WriteField("mediainfo", req.MediaInfo)
	}
	if req.BDInfo != "" {
		_ = writer.WriteField("bdinfo", req.BDInfo)
	}
	if req.Anonymous {
		_ = writer.WriteField("anonymous", "on")
	}

	if csrfToken != "" {
		_ = writer.WriteField("_token", csrfToken)
	}

	fieldMapping := map[string]string{
		"cat":          "category_id",
		"source_sel":   "type_id",
		"standard_sel": "resolution_id",
	}
	for srcType, targetName := range fieldMapping {
		if v, ok := req.FormFields[srcType]; ok {
			_ = writer.WriteField(targetName, v)
		}
	}

	if imdbID := extractIMDbID(req.IMDbLink); imdbID != "" {
		_ = writer.WriteField("imdb_id", imdbID)
	}
	if tmdbID, ok := req.ExtraFields["tmdb_id"]; ok {
		_ = writer.WriteField("tmdb_id", tmdbID)
	}
	if malID, ok := req.ExtraFields["mal_id"]; ok {
		_ = writer.WriteField("mal_id", malID)
	}

	for k, v := range req.FormFields {
		switch k {
		case "cat", "source_sel", "standard_sel":
			continue
		}
		_ = writer.WriteField(k, v)
	}
	for k, v := range req.ExtraFields {
		switch k {
		case "tmdb_id", "mal_id":
			continue
		}
		_ = writer.WriteField(k, v)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("关闭 multipart writer 失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", uploadURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	setCommonHeaders(httpReq, config.Cookie)

	resp, err := a.doer.Client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("上传请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if resp.StatusCode == http.StatusForbidden {
		return &model.PublishResponse{Success: false, ErrorMessage: "403 Forbidden: 权限不足或 CSRF token 过期"}, nil
	}

	if idMatch := regexp.MustCompile(`/torrents/(\d+)`).FindStringSubmatch(html); len(idMatch) > 1 {
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
	if m := regexp.MustCompile(`class="error"[^>]*>([^<]+)`).FindStringSubmatch(html); len(m) > 1 {
		errMsg = strings.TrimSpace(m[1])
	} else if m := regexp.MustCompile(`<li[^>]*>([^<]*(?:error|fail|失败|错误|duplicate|already)[^<]*)</li>`).FindStringSubmatch(html); len(m) > 1 {
		errMsg = strings.TrimSpace(m[1])
	}

	return &model.PublishResponse{Success: false, ErrorMessage: errMsg}, nil
}

func (a *Unit3DAdapter) fetchCSRFToken(ctx context.Context, config *model.SiteConfig, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return "", err
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求页面失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	re := regexp.MustCompile(`<meta\s+name="csrf-token"\s+content="([^"]+)"`)
	if m := re.FindStringSubmatch(html); len(m) > 1 {
		return m[1], nil
	}

	re2 := regexp.MustCompile(`<input[^>]+name="_token"[^>]+value="([^"]+)"`)
	if m := re2.FindStringSubmatch(html); len(m) > 1 {
		return m[1], nil
	}

	return "", fmt.Errorf("未找到 CSRF token")
}

func extractIMDbID(link string) string {
	if link == "" {
		return ""
	}
	m := regexp.MustCompile(`tt\d+`).FindString(link)
	return m
}

func (a *Unit3DAdapter) GetTorrentInfoHash(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	detail, err := a.GetTorrentDetail(ctx, config, torrentID)
	if err != nil {
		return "", err
	}
	if detail.InfoHash == "" {
		return "", fmt.Errorf("未找到 info_hash")
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
