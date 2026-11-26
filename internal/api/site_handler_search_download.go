package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/metrics"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type searchRequest struct {
	Query      string `json:"query"`
	Category   string `json:"category,omitempty"`
	FreeOnly   bool   `json:"freeOnly,omitempty"`
	SortBy     string `json:"sortBy,omitempty"`
	MaxResults int    `json:"maxResults,omitempty"`
}

func (h *SiteHandler) handleSearch(w http.ResponseWriter, r *http.Request, idStr string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if h.provider == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "站点服务未就绪")
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40002, "无效的站点ID")
		return
	}

	var site model.Site
	if err := h.db.WithContext(r.Context()).First(&site, id).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "站点不存在")
		return
	}

	var req searchRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.Query == "" {
		Error(w, http.StatusBadRequest, 40004, "搜索关键词不能为空")
		return
	}

	adapter, err := h.provider.GetAdapter(r.Context(), site.Domain)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50002, "获取站点适配器失败")
		return
	}

	config, err := h.provider.GetSiteConfig(r.Context(), site.Domain)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50003, "failed to get site config")
		return
	}

	opts := &model.SearchOptions{
		Category:   req.Category,
		FreeOnly:   req.FreeOnly,
		SortBy:     req.SortBy,
		MaxResults: req.MaxResults,
	}
	if opts.MaxResults <= 0 {
		opts.MaxResults = 50
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	results, err := adapter.SearchTorrents(ctx, config, req.Query, opts)
	if err != nil {
		h.logger.Warn("search torrents failed",
			zap.String("site", site.Domain),
			zap.String("query", req.Query),
			zap.Error(err))
		metrics.SiteRequestErrors.WithLabelValues(site.Domain, "search").Inc()
		Error(w, http.StatusInternalServerError, 50004, "搜索失败")
		return
	}

	if results == nil {
		results = []*model.SeedingSearchResult{}
	}

	metrics.SiteRequestsTotal.WithLabelValues(site.Domain, "search").Inc()
	Success(w, results)
}

type discountRequest struct {
	TorrentID string `json:"torrentId"`
}

func (h *SiteHandler) handleDetectDiscount(w http.ResponseWriter, r *http.Request, idStr string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if h.provider == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "站点服务未就绪")
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40002, "无效的站点ID")
		return
	}

	var site model.Site
	if err := h.db.WithContext(r.Context()).First(&site, id).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "站点不存在")
		return
	}

	var req discountRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.TorrentID == "" {
		Error(w, http.StatusBadRequest, 40005, "种子ID不能为空")
		return
	}

	adapter, err := h.provider.GetAdapter(r.Context(), site.Domain)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50002, "获取站点适配器失败")
		return
	}

	config, err := h.provider.GetSiteConfig(r.Context(), site.Domain)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50003, "failed to get site config")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	result, err := adapter.DetectDiscount(ctx, config, req.TorrentID)
	if err != nil {
		h.logger.Warn("detect discount failed",
			zap.String("site", site.Domain),
			zap.String("torrentId", req.TorrentID),
			zap.Error(err))
		metrics.SiteRequestErrors.WithLabelValues(site.Domain, "discount").Inc()
		Error(w, http.StatusInternalServerError, 50005, "检测折扣失败")
		return
	}

	metrics.SiteRequestsTotal.WithLabelValues(site.Domain, "discount").Inc()
	Success(w, result)
}

func (h *SiteHandler) handleDownloadTest(w http.ResponseWriter, r *http.Request, idStr string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if h.provider == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "站点服务未就绪")
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40002, "无效的站点ID")
		return
	}

	var site model.Site
	if err := h.db.WithContext(r.Context()).First(&site, id).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "站点不存在")
		return
	}

	config, err := h.provider.GetSiteConfig(r.Context(), site.Domain)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50003, "failed to get site config")
		return
	}

	creds := map[string]bool{
		"cookie":  config.Cookie != "",
		"apiKey":  config.APIKey != "",
		"passkey": config.Passkey != "",
	}
	if !creds["cookie"] && !creds["apiKey"] && !creds["passkey"] {
		Error(w, http.StatusBadRequest, 14003, "站点无凭据")
		return
	}

	adapter, err := h.provider.GetAdapter(r.Context(), site.Domain)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50002, "获取站点适配器失败")
		return
	}

	var req struct {
		TorrentID string `json:"torrentId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req) // 空/坏 body → 空 torrentID 走 400

	torrentID := req.TorrentID
	var candidateIDs []string
	if torrentID == "" {
		var resolveErr error
		candidateIDs, resolveErr = h.resolveTorrentIDsForTest(r.Context(), adapter, config, site.Framework)
		if resolveErr != nil && len(config.AlternativeDomains) > 0 {
			origDomain := config.Domain
			origBase := config.BaseURL
			config.Domain = config.AlternativeDomains[0]
			config.BaseURL = config.AlternativeDomains[0]
			candidateIDs, resolveErr = h.resolveTorrentIDsForTest(r.Context(), adapter, config, site.Framework)
			if resolveErr != nil {
				config.Domain = origDomain
				config.BaseURL = origBase
			}
		}
		if resolveErr != nil {
			Error(w, http.StatusBadGateway, 50006, "获取测试种子ID失败: "+resolveErr.Error())
			return
		}
	} else {
		candidateIDs = []string{torrentID}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var data []byte
	var dlErr error
	for _, tid := range candidateIDs {
		data, dlErr = adapter.DownloadTorrent(ctx, config, tid)
		if dlErr == nil {
			torrentID = tid
			break
		}
		if len(config.AlternativeDomains) > 0 {
			origDomain := config.Domain
			config.Domain = config.AlternativeDomains[0]
			data, dlErr = adapter.DownloadTorrent(ctx, config, tid)
			config.Domain = origDomain
			if dlErr == nil {
				torrentID = tid
				break
			}
		}
	}
	if dlErr != nil {
		h.logger.Warn("download test failed",
			zap.String("site", site.Name),
			zap.String("torrentId", torrentID),
			zap.Error(dlErr))
		Error(w, http.StatusBadGateway, 50007, fmt.Sprintf("下载失败(id=%s): %s", torrentID, dlErr.Error()))
		return
	}

	isTorrent := len(data) >= 3 && data[0] == 'd' && data[1] >= '0' && data[1] <= '9' && data[2] == ':'
	if !isTorrent {
		preview := string(data)
		if len(preview) > 100 {
			preview = preview[:100]
		}
		Error(w, http.StatusBadGateway, 50008, fmt.Sprintf("返回了非种子文件(%dB): %s", len(data), preview))
		return
	}

	Success(w, map[string]interface{}{
		"ok":        true,
		"torrentId": torrentID,
		"size":      len(data),
		"siteName":  site.Name,
		"framework": site.Framework,
	})
}

func (h *SiteHandler) resolveTorrentIDsForTest(ctx context.Context, a model.SiteAdapter, config *model.SiteConfig, framework string) ([]string, error) {
	maxResults := 1
	if framework == "rousi" {
		maxResults = 5
	}
	results, err := a.SearchTorrents(ctx, config, "", &model.SearchOptions{MaxResults: maxResults})
	if err == nil && len(results) > 0 {
		ids := make([]string, 0, len(results))
		for _, r := range results {
			ids = append(ids, r.TorrentID)
		}
		return ids, nil
	}

	if framework == "tnode" {
		return h.resolveTnodeTorrentIDs(ctx, config)
	}
	if framework == "yemapt" {
		return h.resolveYemaptTorrentIDs(ctx, config)
	}
	if framework == "unit3d" {
		return h.resolveUnit3DTorrentIDs(ctx, config)
	}

	client := buildSiteHTTPClient(&model.Site{BaseURL: config.Domain, ProxyURL: config.ProxyURL, SkipSSLVerify: config.SkipSSLVerify}, 10*time.Second)
	browseURL := buildBrowseURLForTest(config, framework)
	if browseURL == "" {
		return nil, fmt.Errorf("无法自动获取种子ID，请手动提供 torrentId")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", browseURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", httpclient.BrowserUserAgent())
	if config.Cookie != "" {
		req.Header.Set("Cookie", config.Cookie)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { httpclient.DrainBody(resp) }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	bodyData, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	id := extractTorrentIDFromHTML(string(bodyData), framework, config.Domain)
	if id == "" {
		return nil, fmt.Errorf("未找到种子ID")
	}
	return []string{id}, nil
}

func buildBrowseURLForTest(config *model.SiteConfig, framework string) string {
	base := config.BaseURL
	if base == "" {
		base = "https://" + config.Domain
	}
	if config.Domain == "totheglory.im" {
		return base + "/browse.php"
	}
	if config.Domain == "star-space.net" {
		return base + "/p_torrent/video_list_g.php"
	}
	switch framework {
	case "nexusphp":
		return base + "/torrents.php"
	case "gazelle":
		return base + "/torrents.php"
	case "unit3d":
		return base + "/torrents"
	case "rousi":
		return base + "/torrents.php"
	case "generic":
		return base + "/browse.php"
	}
	return base + "/torrents.php"
}

func extractTorrentIDFromHTML(html, framework, domain string) string {
	var patterns []*regexp.Regexp
	switch {
	case domain == "totheglory.im":
		patterns = []*regexp.Regexp{
			regexp.MustCompile(`/dl/(\d+)/`),
			regexp.MustCompile(`details\.php\?id=(\d+)`),
		}
	case domain == "hdcity.city":
		patterns = []*regexp.Regexp{regexp.MustCompile(`download\?id=(\d+)`)}
	case domain == "star-space.net":
		patterns = []*regexp.Regexp{regexp.MustCompile(`download\.php\?tid=(\d+)`)}
	case framework == "gazelle":
		patterns = []*regexp.Regexp{regexp.MustCompile(`torrentid=(\d+)`)}
	case framework == "unit3d":
		patterns = []*regexp.Regexp{
			regexp.MustCompile(`/torrents/(\d+)`),
			regexp.MustCompile(`/torrent/(\d+)`),
		}
	default:
		patterns = []*regexp.Regexp{
			regexp.MustCompile(`download\.php\?id=(\d+)`),
			regexp.MustCompile(`download\?id=(\d+)`),
			regexp.MustCompile(`details\.php\?id=(\d+)`),
		}
	}
	for _, re := range patterns {
		if m := re.FindStringSubmatch(html); len(m) >= 2 {
			return m[1]
		}
	}
	return ""
}

func hasAlternativeDomains(s *model.Site) bool {
	return s.AlternativeDomains != ""
}

func firstAltDomain(s *model.Site) string {
	var alts []string
	if json.Unmarshal([]byte(s.AlternativeDomains), &alts) == nil && len(alts) > 0 {
		return alts[0]
	}
	return ""
}

func cloneSiteWithBaseURL(s *model.Site, baseURL string) *model.Site {
	clone := *s
	clone.BaseURL = baseURL
	return &clone
}

func (h *SiteHandler) testCookieAuthWithBaseURL(ctx context.Context, client *http.Client, s *model.Site, altURL string) (bool, string) {
	if altURL == "" {
		return false, "无备选域名"
	}
	return h.testCookieAuth(ctx, client, cloneSiteWithBaseURL(s, altURL))
}

func (h *SiteHandler) testAPIKeyAuthWithBaseURL(ctx context.Context, client *http.Client, s *model.Site, altURL string) (bool, string) {
	if altURL == "" {
		return false, "无备选域名"
	}
	return h.testAPIKeyAuth(ctx, client, cloneSiteWithBaseURL(s, altURL))
}

func (h *SiteHandler) testPasskeyAuthWithBaseURL(ctx context.Context, client *http.Client, s *model.Site, altURL string) (bool, string) {
	if altURL == "" {
		return false, "无备选域名"
	}
	return h.testPasskeyAuth(ctx, client, cloneSiteWithBaseURL(s, altURL))
}

func (h *SiteHandler) resolveTnodeTorrentIDs(ctx context.Context, config *model.SiteConfig) ([]string, error) {
	base := config.BaseURL
	if base == "" {
		base = "https://" + config.Domain
	}
	client := buildSiteHTTPClient(&model.Site{BaseURL: config.Domain, ProxyURL: config.ProxyURL, SkipSSLVerify: config.SkipSSLVerify}, 15*time.Second)

	csrfToken := ""
	csrfReq, err := http.NewRequestWithContext(ctx, "GET", base+"/index", nil)
	if err == nil {
		csrfReq.Header.Set("Cookie", config.Cookie)
		csrfReq.Header.Set("User-Agent", httpclient.BrowserUserAgent())
		if resp, err := client.Do(csrfReq); err == nil {
			defer func() { httpclient.DrainBody(resp) }()
			if body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024)); err == nil {
				re := regexp.MustCompile(`<meta\s+name="x-csrf-token"\s+content="([^"]+)"`)
				if m := re.FindSubmatch(body); len(m) >= 2 {
					csrfToken = string(m[1])
				}
			}
		}
	}

	for _, path := range []string{
		"/api/torrent/torrentList?page=1&pageSize=5",
		"/api/torrent/torrents?page=1&pageSize=5",
		"/api/torrent/list?page=1&size=5",
	} {
		req, err := http.NewRequestWithContext(ctx, "GET", base+path, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Cookie", config.Cookie)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", httpclient.BrowserUserAgent())
		if csrfToken != "" {
			req.Header.Set("x-csrf-token", csrfToken)
		}

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
		httpclient.DrainBody(resp)
		if resp.StatusCode != http.StatusOK {
			continue
		}

		var raw map[string]json.RawMessage
		if json.Unmarshal(body, &raw) != nil {
			continue
		}

		var dataField json.RawMessage
		if d, ok := raw["data"]; ok {
			dataField = d
		} else if d, ok := raw["records"]; ok {
			dataField = d
		}

		if dataField == nil {
			continue
		}

		var torrentList []struct {
			ID json.Number `json:"id"`
		}
		if json.Unmarshal(dataField, &torrentList) != nil || len(torrentList) == 0 {
			continue
		}

		ids := make([]string, 0, len(torrentList))
		for _, t := range torrentList {
			if s := t.ID.String(); s != "" && s != "0" {
				ids = append(ids, s)
			}
		}
		if len(ids) > 0 {
			return ids, nil
		}
	}

	return []string{"1"}, nil
}

func (h *SiteHandler) resolveYemaptTorrentIDs(ctx context.Context, config *model.SiteConfig) ([]string, error) {
	base := config.BaseURL
	if base == "" {
		base = "https://" + config.Domain
	}
	client := buildSiteHTTPClient(&model.Site{BaseURL: config.Domain, ProxyURL: config.ProxyURL, SkipSSLVerify: config.SkipSSLVerify}, 15*time.Second)

	for _, path := range []string{
		"/api/torrents?perPage=5&page=1",
		"/api/torrent/list?perPage=5&page=1",
	} {
		req, err := http.NewRequestWithContext(ctx, "GET", base+path, nil)
		if err != nil {
			continue
		}
		req.Header.Set("x-api-key", config.APIKey)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", httpclient.BrowserUserAgent())

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
		httpclient.DrainBody(resp)
		if resp.StatusCode != http.StatusOK {
			continue
		}

		var raw map[string]json.RawMessage
		if json.Unmarshal(body, &raw) != nil {
			continue
		}

		var dataField json.RawMessage
		for _, key := range []string{"data", "records", "torrents"} {
			if d, ok := raw[key]; ok {
				dataField = d
				break
			}
		}
		if dataField != nil {
			var nested map[string]json.RawMessage
			if json.Unmarshal(dataField, &nested) == nil {
				if t, ok := nested["torrents"]; ok {
					dataField = t
				} else if t, ok := nested["list"]; ok {
					dataField = t
				}
			}
		}

		if dataField == nil {
			continue
		}

		var torrentList []struct {
			ID json.Number `json:"id"`
		}
		if json.Unmarshal(dataField, &torrentList) != nil || len(torrentList) == 0 {
			continue
		}

		ids := make([]string, 0, len(torrentList))
		for _, t := range torrentList {
			if s := t.ID.String(); s != "" && s != "0" {
				ids = append(ids, s)
			}
		}
		if len(ids) > 0 {
			return ids, nil
		}
	}

	return []string{"10000"}, nil
}

func (h *SiteHandler) resolveUnit3DTorrentIDs(ctx context.Context, config *model.SiteConfig) ([]string, error) {
	base := config.BaseURL
	if base == "" {
		base = "https://" + config.Domain
	}
	client := buildSiteHTTPClient(&model.Site{BaseURL: config.Domain, ProxyURL: config.ProxyURL, SkipSSLVerify: config.SkipSSLVerify}, 15*time.Second)

	for _, path := range []string{
		"/api/torrents?perPage=5&page=1",
		"/api/torrent/filter?perPage=5&page=1",
	} {
		req, err := http.NewRequestWithContext(ctx, "GET", base+path, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Cookie", config.Cookie)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", httpclient.BrowserUserAgent())
		if config.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.APIKey)
		} else if config.AuthKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.AuthKey)
		}

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
		httpclient.DrainBody(resp)
		if resp.StatusCode != http.StatusOK {
			continue
		}

		var raw map[string]json.RawMessage
		if json.Unmarshal(body, &raw) != nil {
			continue
		}

		var dataField json.RawMessage
		for _, key := range []string{"data", "attributes", "records"} {
			if d, ok := raw[key]; ok {
				dataField = d
				break
			}
		}
		if dataField == nil {
			continue
		}

		var torrentList []struct {
			ID json.Number `json:"id"`
		}
		if json.Unmarshal(dataField, &torrentList) != nil || len(torrentList) == 0 {
			var nested struct {
				ID json.Number `json:"id"`
			}
			if json.Unmarshal(dataField, &nested) == nil && nested.ID.String() != "" {
				return []string{nested.ID.String()}, nil
			}
			continue
		}

		ids := make([]string, 0, len(torrentList))
		for _, t := range torrentList {
			if s := t.ID.String(); s != "" && s != "0" {
				ids = append(ids, s)
			}
		}
		if len(ids) > 0 {
			return ids, nil
		}
	}

	return []string{"1"}, nil
}

// hasFormConfig §59.318 D3: 发布配置存在性——非空+合法 JSON+含 form_fields
// （暂停态 enabled=false 的既有配置算"已配置"——开关语义=恢复发布）。
func hasFormConfig(cfg string) bool {
	if cfg == "" {
		return false
	}
	var m map[string]interface{}
	if json.Unmarshal([]byte(cfg), &m) != nil {
		return false
	}
	_, ok := m["form_fields"]
	return ok
}

// setFormConfigEnabled §59.318 D3: form_config.enabled 位写入——map 反序列化-
// 改-序列化保真（禁 struct 往返丢字段）。enabled 缺省视为 false 补齐。
func setFormConfigEnabled(s *model.Site, enabled bool) (string, error) {
	var m map[string]interface{}
	if s.PublishFormConfig == "" {
		m = map[string]interface{}{}
	} else if err := json.Unmarshal([]byte(s.PublishFormConfig), &m); err != nil {
		return "", fmt.Errorf("form_config JSON 解析失败: %w", err)
	}
	m["enabled"] = enabled
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
