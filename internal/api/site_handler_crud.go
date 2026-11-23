package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/middleware"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/site"
	"go.uber.org/zap"
)

func (h *SiteHandler) handleList(w http.ResponseWriter, r *http.Request) {
	page, size := parsePagination(r)
	q := r.URL.Query()
	search := q.Get("search")
	framework := q.Get("framework")
	enabled := q.Get("enabled")
	isSource := q.Get("isSource")
	isTarget := q.Get("isTarget")

	query := h.db.Model(&model.Site{})
	if search != "" {
		escaped := strings.NewReplacer("%", "\\%", "_", "\\_").Replace(search)
		like := "%" + escaped + "%"
		query = query.Where("name LIKE ? OR domain LIKE ? ESCAPE '\\'", like, like)
	}
	if framework != "" {
		query = query.Where("framework = ?", framework)
	}
	if enabled == "true" {
		query = query.Where("enabled = ?", true)
	} else if enabled == "false" {
		query = query.Where("enabled = ?", false)
	}
	if isSource == "true" {
		query = query.Where("is_source = ?", true)
	} else if isSource == "false" {
		query = query.Where("is_source = ?", false)
	}
	if isTarget == "true" {
		query = query.Where("is_target = ?", true)
	} else if isTarget == "false" {
		query = query.Where("is_target = ?", false)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		h.logger.Warn("query failed", zap.Error(err))
	}

	var sites []model.Site
	query.Order("name ASC").
		Offset(offset(page, size)).Limit(size).
		Find(&sites)

	items := make([]siteResponse, 0, len(sites))
	for i := range sites {
		items = append(items, h.toResponse(&sites[i]))
	}

	Success(w, PaginatedResult{
		Items: items,
		Total: total,
		Page:  page,
		Size:  size,
	})
}

func (h *SiteHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := h.extractID(r.URL.Path, "/api/v1/sites/")
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的站点 ID")
		return
	}

	s, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, 12001, "站点不存在")
		return
	}

	Success(w, h.toResponse(s))
}

func (h *SiteHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := h.extractID(r.URL.Path, "/api/v1/sites/")
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的站点 ID")
		return
	}

	s, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, 12001, "站点不存在")
		return
	}

	var req updateSiteRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	if req.Name != nil && *req.Name != "" {
		exists, _ := h.repo.ExistsByName(r.Context(), *req.Name, id)
		if exists {
			Error(w, http.StatusConflict, 40900, "站点名称已存在")
			return
		}
		s.Name = *req.Name
	}
	if req.Domain != nil && *req.Domain != "" {
		exists, _ := h.repo.ExistsByDomain(r.Context(), *req.Domain, id)
		if exists {
			Error(w, http.StatusConflict, 40900, "站点域名已存在")
			return
		}
		s.Domain = *req.Domain
	}
	if req.BaseURL != nil && *req.BaseURL != "" {
		if err := middleware.ValidatePublicURL(*req.BaseURL); err != nil {
			Error(w, http.StatusBadRequest, 40001, "baseUrl 不合法: "+err.Error())
			return
		}
		s.BaseURL = *req.BaseURL
	}
	// framework 不可由用户修改：内置站点的 framework 由 supported_sites.json 唯一确定
	if req.AuthType != nil {
		if !model.ValidAuthType(*req.AuthType) {
			Error(w, http.StatusBadRequest, 40001, "不支持的 authType")
			return
		}
		s.AuthType = *req.AuthType
	}
	if req.Enabled != nil {
		s.Enabled = *req.Enabled
	}
	// §59.166: clamp 1-60（防脏数据）
	if req.PublishIntervalSeconds != nil {
		v := *req.PublishIntervalSeconds
		if v < 1 {
			v = 1
		}
		if v > 60 {
			v = 60
		}
		s.PublishIntervalSeconds = v
	}
	if req.IsSource != nil {
		s.IsSource = *req.IsSource
	}
	if req.IsTarget != nil {
		// §59.318 D3+D7: is_target=发布目标开关——单真相位 form_config.enabled。
		// 开启三重门槛：①publish_supported（开发者适配权威位）②配置已存在
		// （空配置+enabled=true 为非法态）③写 form_config.enabled 并同步 is_target
		// 冗余列（写点之一；另一写点 form_config handleApply）。
		if *req.IsTarget {
			if !site.IsPublishSupported(s.Domain) {
				Error(w, http.StatusForbidden, 40021, "站点未适配发布，无法启用为目标站")
				return
			}
			if !hasFormConfig(s.PublishFormConfig) {
				Error(w, http.StatusBadRequest, 40022, "请先完成发布配置再启用目标站")
				return
			}
			updated, err := setFormConfigEnabled(s, true)
			if err != nil {
				Error(w, http.StatusInternalServerError, 50000, "更新发布配置失败: "+err.Error())
				return
			}
			s.PublishFormConfig = updated
			s.IsTarget = true
		} else {
			if s.PublishFormConfig != "" {
				updated, err := setFormConfigEnabled(s, false)
				if err != nil {
					Error(w, http.StatusInternalServerError, 50000, "更新发布配置失败: "+err.Error())
					return
				}
				s.PublishFormConfig = updated
			}
			s.IsTarget = false
		}
	}
	if req.IsReseedTarget != nil {
		// §59.318 D1: 辅种探测范围开关——用户自由切换（能力层由 enabled+凭证隐含）
		s.IsReseedTarget = *req.IsReseedTarget
	}
	if req.ParticipateAutoPublish != nil {
		s.ParticipateAutoPublish = *req.ParticipateAutoPublish
	}
	if req.AssumeFree != nil {
		s.AssumeFree = *req.AssumeFree
	}
	if req.CookieCloudSync != nil {
		s.CookieCloudSync = *req.CookieCloudSync
	}
	if req.CookieCloudDomain != nil {
		s.CookieCloudDomain = *req.CookieCloudDomain
	}
	if req.AlternativeDomains != nil {
		s.AlternativeDomains = *req.AlternativeDomains
	}
	if req.Passkey != nil && *req.Passkey != "" {
		s.Passkey = *req.Passkey
	}
	if req.Cookie != nil && *req.Cookie != "" {
		s.Cookie = *req.Cookie
	}
	if req.APIKey != nil && *req.APIKey != "" {
		s.APIKey = *req.APIKey
	}
	if req.HashStrategy != nil {
		s.HashStrategy = *req.HashStrategy
	}
	if req.SizeStrategy != nil {
		s.SizeStrategy = *req.SizeStrategy
	}
	if req.IDStrategy != nil {
		s.IDStrategy = *req.IDStrategy
	}
	if req.IDPattern != nil {
		s.IDPattern = *req.IDPattern
	}
	if req.HashXMLTagName != nil {
		s.HashXMLTagName = *req.HashXMLTagName
	}
	if req.SizeXMLTagName != nil {
		s.SizeXMLTagName = *req.SizeXMLTagName
	}
	if req.HashURLParamName != nil {
		s.HashURLParamName = *req.HashURLParamName
	}
	if req.SizeDescRegex != nil {
		s.SizeDescRegex = *req.SizeDescRegex
	}
	if req.SizeTitleRegex != nil {
		s.SizeTitleRegex = *req.SizeTitleRegex
	}
	if req.SizeBaseUnit != nil {
		s.SizeBaseUnit = *req.SizeBaseUnit
	}
	if req.DownloadMode != nil {
		s.DownloadMode = *req.DownloadMode
	}
	if req.DownloadURLTemplate != nil {
		s.DownloadURLTemplate = *req.DownloadURLTemplate
	}
	if req.DetailsURLTemplate != nil {
		s.DetailsURLTemplate = *req.DetailsURLTemplate
	}
	if req.DownloadPagePattern != nil {
		s.DownloadPagePattern = *req.DownloadPagePattern
	}
	if req.RequiresSideLoading != nil {
		s.RequiresSideLoading = *req.RequiresSideLoading
	}
	if req.OverrideRSSURL != nil {
		if *req.OverrideRSSURL != "" {
			if err := middleware.ValidatePublicURL(*req.OverrideRSSURL); err != nil {
				Error(w, http.StatusBadRequest, 40001, "overrideRssUrl 不合法: "+err.Error())
				return
			}
		}
		s.OverrideRSSURL = *req.OverrideRSSURL
	}
	if req.OverrideSavePath != nil {
		s.OverrideSavePath = *req.OverrideSavePath
	}
	if req.ProxyURL != nil {
		if *req.ProxyURL != "" {
			if err := middleware.ValidateProxyURL(*req.ProxyURL); err != nil {
				Error(w, http.StatusBadRequest, 40001, "proxyUrl 不合法: "+err.Error())
				return
			}
		}
		s.ProxyURL = *req.ProxyURL
	}
	if req.UseGlobalProxy != nil {
		s.UseGlobalProxy = *req.UseGlobalProxy
	}
	if req.SkipSSLVerify != nil {
		s.SkipSSLVerify = *req.SkipSSLVerify
	}
	if req.BrowserFallback != nil { // §59.336
		s.BrowserFallback = *req.BrowserFallback
	}
	if req.MaxConcurrent != nil {
		s.MaxConcurrent = *req.MaxConcurrent
	}
	if req.DownloadHourlyLimit != nil {
		s.DownloadHourlyLimit = *req.DownloadHourlyLimit
	}
	if req.HRStrategy != nil {
		s.HRStrategy = *req.HRStrategy
	}
	if req.ReseedLimitCount != nil {
		s.ReseedLimitCount = *req.ReseedLimitCount
	}
	if req.ReseedLimitInterval != nil {
		s.ReseedLimitInterval = *req.ReseedLimitInterval
	}
	if req.IYUULimitCount != nil {
		s.IYUULimitCount = *req.IYUULimitCount
	}
	if req.IYUULimitInterval != nil {
		s.IYUULimitInterval = *req.IYUULimitInterval
	}

	// 有官组映射才允许 is_source=true
	if s.IsSource && h.sourceDetector != nil && !h.sourceDetector.HasGroupMappings(r.Context(), s) {
		s.IsSource = false
	}
	if req.SupportsPiecesHashAPI != nil {
		s.SupportsPiecesHashAPI = *req.SupportsPiecesHashAPI
	}

	if err := h.repo.Update(r.Context(), s); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "更新站点失败")
		return
	}

	applySiteMaxConcurrent(s.Domain, s.MaxConcurrent)

	// §59.367 附二: 站点配置变更后失效 adapter 缓存——proxy/skip_ssl/
	// tls_pretend 等 doer 构造参数在缓存 adapter 里持有旧值，不失效则
	// 配置变更需重启才生效（§59.333 附同款）。双键失效：GetAdapter 的
	// 缓存键可能是站名（RSS detect 链）或域名（其它链）——同站双副本。
	if h.provider != nil {
		h.provider.InvalidateAdapter(s.Name, s.Domain)
	}

	h.logger.Info("site updated", zap.String("name", s.Name))
	auditLog(r, "site", "update", "site", fmt.Sprintf("%d", id), s.Name, "success")

	Success(w, h.toResponse(s))
}

func (h *SiteHandler) handleTest(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的站点 ID")
		return
	}

	s, err := h.repo.GetByID(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, 12001, "站点不存在")
		return
	}

	ok, message := h.testSiteConnection(s)

	if !ok {
		Error(w, http.StatusBadGateway, 30001, message)
		return
	}

	if h.statsSync != nil {
		go func() { //nolint:gosec // G118: intentional — must outlive request context
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := h.statsSync.SyncSiteStats(ctx, s.ID); err != nil {
				h.logger.Debug("test-triggered stats sync failed", zap.String("site", s.Name), zap.Error(err))
			}
		}()
	}

	Success(w, map[string]interface{}{
		"ok":      ok,
		"message": message,
	})
}

func (h *SiteHandler) handleDetect(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的站点 ID")
		return
	}

	s, err := h.repo.GetByID(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, 12001, "站点不存在")
		return
	}

	result := h.detectFramework(s)
	Success(w, result)
}

func (h *SiteHandler) handleUpdateCredentials(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的站点 ID")
		return
	}

	_, err = h.repo.GetByID(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, 12001, "站点不存在")
		return
	}

	var req updateCredentialsRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	creds := map[string]interface{}{}
	if req.Passkey != nil {
		creds["passkey"] = *req.Passkey
	}
	if req.Cookie != nil {
		creds["cookie"] = *req.Cookie
	}
	if req.APIKey != nil {
		creds["api_key"] = *req.APIKey
	}
	if req.BearerToken != nil {
		creds["bearer_token"] = *req.BearerToken
	}
	if req.AuthKey != nil {
		creds["auth_key"] = *req.AuthKey
	}
	if req.AuthHash != nil {
		creds["auth_hash"] = *req.AuthHash
	}
	if req.UploadAuth != nil {
		creds["upload_auth"] = *req.UploadAuth
	}
	if req.UserID != nil {
		creds["user_id"] = *req.UserID
	}
	if req.RSSKey != nil {
		creds["rss_key"] = *req.RSSKey
	}

	if len(creds) == 0 {
		Error(w, http.StatusBadRequest, 40001, "未提供任何凭据字段")
		return
	}

	if err := h.repo.UpdateCredentials(r.Context(), uint(id), creds); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "更新凭据失败")
		return
	}

	updated, err := h.repo.GetByID(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "重新获取站点失败")
		return
	}
	h.logger.Info("site credentials updated", zap.String("domain", updated.Domain))
	auditLog(r, "site", "update_credentials", "site", idStr, updated.Domain, "success")
	Success(w, h.toResponse(updated))
}

func (h *SiteHandler) handleGetStats(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的站点 ID")
		return
	}

	s, err := h.repo.GetByID(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, 12001, "站点不存在")
		return
	}

	Success(w, map[string]interface{}{
		"username":      s.Username,
		"uploadBytes":   s.UploadBytes,
		"downloadBytes": s.DownloadBytes,
		"seedingPoints": s.SeedingPoints,
		"seedingSize":   s.SeedingSize,
		"seedingCount":  s.SeedingCount,
		"userClass":     s.UserClass,
		"ratio":         s.Ratio,
		"bonusPoints":   s.BonusPoints,
		"statsSyncedAt": s.StatsSyncedAt,
	})
}

func (h *SiteHandler) handleBatchUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs    []uint                 `json:"ids"`
		Fields map[string]interface{} `json:"fields"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if len(req.IDs) == 0 {
		Error(w, http.StatusBadRequest, 40001, "ids 不能为空")
		return
	}
	if len(req.Fields) == 0 {
		Error(w, http.StatusBadRequest, 40001, "fields 不能为空")
		return
	}

	updates := make(map[string]interface{})
	boolFields := map[string]bool{
		"enabled": true, "is_source": true,
		"participate_auto_publish": true, "cookie_cloud_sync": true,
		"assume_free": true,
	}
	for k, v := range req.Fields {
		if !boolFields[k] {
			Error(w, http.StatusBadRequest, 40001, "不允许批量更新字段: "+k)
			return
		}
		bVal, ok := v.(bool)
		if !ok {
			Error(w, http.StatusBadRequest, 40001, "字段 "+k+" 必须为布尔值")
			return
		}
		updates[k] = bVal
	}
	if len(updates) == 0 {
		Error(w, http.StatusBadRequest, 40001, "没有有效的更新字段")
		return
	}

	result := h.db.WithContext(r.Context()).Model(&model.Site{}).Where("id IN ?", req.IDs).Updates(updates)
	if result.Error != nil {
		Error(w, http.StatusInternalServerError, 50000, "批量更新失败: "+result.Error.Error())
		return
	}

	auditLog(r, "site", "batch_update", "site", "", fmt.Sprintf("ids=%v, affected=%d", req.IDs, result.RowsAffected), "success")
	Success(w, map[string]interface{}{
		"updated": result.RowsAffected,
	})
}

func (h *SiteHandler) handleSyncAllStats(w http.ResponseWriter, r *http.Request) {
	if h.statsSync == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "Stats 同步服务未初始化")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	synced, failedSites := h.statsSync.SyncAllSites(ctx)
	Success(w, map[string]interface{}{
		"synced":      synced,
		"failed":      len(failedSites),
		"failedSites": failedSites,
	})
}

func (h *SiteHandler) handleBatchSyncSiteStats(w http.ResponseWriter, r *http.Request) {
	if h.statsSync == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "Stats 同步服务未初始化")
		return
	}
	var req struct {
		IDs []uint `json:"ids"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if len(req.IDs) == 0 {
		Error(w, http.StatusBadRequest, 40001, "ids 不能为空")
		return
	}

	timeout := time.Duration(len(req.IDs)*30) * time.Second
	if timeout < 2*time.Minute {
		timeout = 2 * time.Minute
	}
	if timeout > 5*time.Minute {
		timeout = 5 * time.Minute
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	synced, failedSites := h.statsSync.SyncSelectedSites(ctx, req.IDs)
	auditLog(r, "site", "batch_sync", "site", "", fmt.Sprintf("ids=%v, synced=%d, failed=%d", req.IDs, synced, len(failedSites)), "success")
	Success(w, map[string]interface{}{
		"synced":      synced,
		"failed":      len(failedSites),
		"failedSites": failedSites,
	})
}

func (h *SiteHandler) handleSyncSiteStats(w http.ResponseWriter, r *http.Request, idStr string) {
	if h.statsSync == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "Stats 同步服务未初始化")
		return
	}
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的站点 ID")
		return
	}

	if err := h.statsSync.SyncSiteStats(r.Context(), uint(id)); err != nil {
		updated, _ := h.repo.GetByID(r.Context(), uint(id))
		if updated != nil {
			resp := h.toResponse(updated)
			Success(w, map[string]interface{}{
				"site":        resp,
				"syncWarning": err.Error(),
			})
			return
		}
		Error(w, http.StatusBadGateway, 50000, "同步站点统计失败: "+err.Error())
		return
	}

	updated, err := h.repo.GetByID(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "重新获取站点失败")
		return
	}
	Success(w, h.toResponse(updated))
}

func (h *SiteHandler) testSiteConnection(s *model.Site) (bool, string) {
	switch s.AuthType {
	case "cookie":
		if s.Cookie == "" {
			return false, "未配置 Cookie"
		}
	case "apikey":
		if s.APIKey == "" {
			return false, "未配置 API Key"
		}
	case "passkey":
		if s.Passkey == "" {
			return false, "未配置 Passkey"
		}
	}

	client := buildSiteHTTPClient(s, 15*time.Second)

	switch s.AuthType {
	case "cookie":
		ok, msg := h.testCookieAuth(client, s)
		if ok || !hasAlternativeDomains(s) {
			return ok, msg
		}
		return h.testCookieAuthWithBaseURL(client, s, firstAltDomain(s))
	case "apikey":
		ok, msg := h.testAPIKeyAuth(client, s)
		if ok || !hasAlternativeDomains(s) {
			return ok, msg
		}
		return h.testAPIKeyAuthWithBaseURL(client, s, firstAltDomain(s))
	case "passkey":
		ok, msg := h.testPasskeyAuth(client, s)
		if ok || !hasAlternativeDomains(s) {
			return ok, msg
		}
		return h.testPasskeyAuthWithBaseURL(client, s, firstAltDomain(s))
	}
	return h.testCookieAuth(client, s)
}

func (h *SiteHandler) testCookieAuth(client *http.Client, s *model.Site) (bool, string) {
	// v0.0.254: 先用不跟随重定向的方式检测 cookie 有效性（精准检测 SPA 站点）
	// 对朱雀（302→/entry/login）/ 野马（200 SPA 壳）等非标准 NexusPHP 站点，
	// 传统 HTML 关键字检测会误判。先用重定向检测，再退化到 HTML 检测。
	noRedirectClient := &http.Client{
		Timeout:   client.Timeout,
		Transport: client.Transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse // 不跟随重定向
		},
	}

	req, err := http.NewRequest("GET", s.BaseURL, nil) //nolint:gosec // admin test endpoint, URL from site config
	if err != nil {
		return false, "构造请求失败: " + err.Error()
	}
	req.Header.Set("Cookie", s.Cookie)

	noRedirectResp, err := noRedirectClient.Do(req) //nolint:gosec // admin test endpoint
	if err != nil {
		return false, "连接失败: " + err.Error()
	}

	// v0.0.254: 3xx 重定向 → 检查是否到登录页
	if noRedirectResp.StatusCode == 301 || noRedirectResp.StatusCode == 302 ||
		noRedirectResp.StatusCode == 303 || noRedirectResp.StatusCode == 307 || noRedirectResp.StatusCode == 308 {
		location := noRedirectResp.Header.Get("Location")
		httpclient.DrainBody(noRedirectResp)
		locLower := strings.ToLower(location)
		// 重定向到 login 页 = cookie 明确无效
		if strings.Contains(locLower, "login") || strings.Contains(locLower, "entry") {
			return false, "Cookie 无效或已过期（重定向到登录页: " + location + "）"
		}
		// 重定向到其他页（如首页/种子列表）= cookie 可能有效，继续验证
	} else {
		httpclient.DrainBody(noRedirectResp)
	}

	// 用跟随重定向的 client 做完整检测（现有逻辑）
	resp, err := client.Do(req) //nolint:gosec // admin test endpoint
	if err != nil {
		return false, "连接失败: " + err.Error()
	}
	defer func() { httpclient.DrainBody(resp) }()
	if resp.StatusCode >= 400 {
		return false, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	bodyData, _ := io.ReadAll(io.LimitReader(resp.Body, 128*1024))
	bodyStr := string(bodyData)
	lower := strings.ToLower(bodyStr)

	// v0.0.254: SPA 壳页面检测（野马/朱雀等）
	// SPA 站点首页只有 JS 壳（<div id="root">），登录态由 JS 渲染，HTML 不含关键字
	if isSPAShell(bodyStr) {
		// SPA 壳 + 无登录页重定向（前面已检查）= cookie 有效
		return true, "连接成功（SPA 站点）"
	}

	loginIndicators := []string{
		`type="password"`,
		`name="password"`,
		`id="password"`,
		`name="login"`,
		`action="login"`,
		`action="/login"`,
		`action="/takelogin"`,
		`action="takelogin"`,
		`please login`,
		`please log in`,
		`you must login`,
		`you must log in`,
		`请先登录`,
		`login_return`,
		`login_form`,
	}
	for _, indicator := range loginIndicators {
		if strings.Contains(lower, indicator) {
			return false, "Cookie 无效或已过期（页面返回登录表单）"
		}
	}

	finalURL := resp.Request.URL.String()
	if strings.Contains(finalURL, "login") && !strings.Contains(finalURL, "userdetails") && !strings.Contains(finalURL, "userdetail.") {
		return false, "Cookie 无效或已过期（页面重定向到登录页）"
	}

	if !strings.Contains(bodyStr, "userdetails") && !strings.Contains(bodyStr, "userdetail.") && !strings.Contains(bodyStr, "logout") {
		return false, "Cookie 无效或已过期（页面缺少登录态标识）"
	}

	return true, "连接成功"
}

// isSPAShell 检测是否为 SPA 壳页面（v0.0.254）。
// SPA 站点（野马/朱雀等）首页只有 JS 壳 + <div id="root">，
// 登录态由 JS 渲染，服务端 HTML 不含 userdetails/logout 等关键字。
func isSPAShell(html string) bool {
	// 特征 1: 含 id="root" 或 id="app"
	hasRootContainer := strings.Contains(html, `id="root"`) || strings.Contains(html, `id="app"`)
	// 特征 2: HTML 较小（< 10KB，SPA 壳通常 < 5KB）
	isSmallHTML := len(html) < 10240
	// 特征 3: 不含 NexusPHP 标准标识
	noNexusPHP := !strings.Contains(html, "userdetails") && !strings.Contains(html, "logout")
	return hasRootContainer && isSmallHTML && noNexusPHP
}

func (h *SiteHandler) testAPIKeyAuth(client *http.Client, s *model.Site) (bool, string) {
	// §59.457 附五: 肉丝（PeerGo 兼容层）——Bearer pgk + GET torrents
	// （POST BaseURL+x-api-key 对该站恒失败——auth_type=apikey 后需要真分支）
	if s.Framework == "rousi" {
		testURL := strings.TrimRight(s.BaseURL, "/") + "/api/v1/torrents?page_size=1"
		req, err := http.NewRequest("GET", testURL, nil) //nolint:gosec // admin test endpoint, URL from site config
		if err != nil {
			return false, "构造请求失败: " + err.Error()
		}
		req.Header.Set("Authorization", "Bearer "+s.APIKey)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", httpclient.BrowserUserAgent())
		resp, err := client.Do(req)
		if err != nil {
			return false, "连接失败: " + err.Error()
		}
		defer func() { httpclient.DrainBody(resp) }()
		if resp.StatusCode == http.StatusOK {
			return true, "API Key 认证成功"
		}
		if resp.StatusCode == http.StatusUnauthorized {
			return false, "API Key 无效（401）"
		}
		return false, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	testURL := s.BaseURL
	if s.Framework == "mteam" || s.Framework == "cola" {
		testURL = strings.TrimRight(s.BaseURL, "/") + "/api/member/profile"
	}
	// §59.540: 猪猪（piggo 令牌 API）——POST /v1/api/me 探活（Bearer·
	// 查询类 JSON 正文契约·§59.1449 hdarea 同族：通用分支 POST 根 URL
	// 对 API 站不适配）。
	if s.Framework == "piggo" {
		meURL := strings.TrimRight(s.BaseURL, "/") + "/v1/api/me"
		req, err := http.NewRequest("POST", meURL, strings.NewReader("{}"))
		if err != nil {
			return false, "构造请求失败: " + err.Error()
		}
		req.Header.Set("Authorization", "Bearer "+s.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return false, "连接失败: " + err.Error()
		}
		defer func() { httpclient.DrainBody(resp) }()
		switch {
		case resp.StatusCode == http.StatusOK:
			return true, "存取令牌认证成功（/v1/api/me）"
		case resp.StatusCode == http.StatusUnauthorized:
			return false, "存取令牌无效（401）"
		case resp.StatusCode == http.StatusForbidden:
			return false, "令牌有效但被拦（403——可能未读必读公告，请到猪猪前台读公告）"
		default:
			return false, fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
	}
	// §59.515 刀1 生产首验：API 网关站（好大）探活走 /api/v1/status——
	// 通用分支 POST 站点根 URL 会 302（→index/login）误报「被重定向」。
	// status 为 GET 探活端点（任意有效 Key·返回身份与协议入口）。
	if s.Framework == "hdarea" {
		statusURL := strings.TrimRight(s.BaseURL, "/") + "/api/v1/status"
		req, err := http.NewRequest("GET", statusURL, nil) //nolint:gosec // admin test endpoint, URL from site config
		if err != nil {
			return false, "构造请求失败: " + err.Error()
		}
		req.Header.Set("X-API-Key", s.APIKey)
		req.Header.Set("User-Agent", httpclient.BrowserUserAgent())
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return false, "连接失败: " + err.Error()
		}
		defer func() { httpclient.DrainBody(resp) }()
		switch {
		case resp.StatusCode == http.StatusOK:
			return true, "API Key 认证成功（/api/v1/status）"
		case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
			return false, "API Key 无效或权限不足（" + strconv.Itoa(resp.StatusCode) + "）"
		default:
			return false, fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
	}
	req, err := http.NewRequest("POST", testURL, nil) //nolint:gosec // admin test endpoint, URL from site config
	if err != nil {
		return false, "构造请求失败: " + err.Error()
	}
	req.Header.Set("x-api-key", s.APIKey)
	req.Header.Set("User-Agent", httpclient.BrowserUserAgent())
	req.Header.Set("Accept", "application/json")

	noRedirectClient := &http.Client{
		Timeout:   client.Timeout,
		Transport: client.Transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := noRedirectClient.Do(req) //nolint:gosec // admin test endpoint
	if err != nil {
		return false, "连接失败: " + err.Error()
	}
	defer func() { httpclient.DrainBody(resp) }()

	bodyData, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	bodyStr := string(bodyData)

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return false, "API Key 无效或已过期"
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return false, "API Key 无效或已过期（被重定向）"
	}
	if resp.StatusCode >= 400 {
		return false, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	lower := strings.ToLower(bodyStr)

	if strings.Contains(lower, `"code":1`) || strings.Contains(lower, `"code":401`) || strings.Contains(lower, `"code":403`) {
		return false, "API Key 无效或已过期"
	}
	if strings.Contains(lower, `"error"`) || strings.Contains(lower, "無效") || strings.Contains(lower, "无效") {
		return false, "API Key 无效或已过期"
	}

	return true, "连接成功"
}

func (h *SiteHandler) testPasskeyAuth(client *http.Client, s *model.Site) (bool, string) {
	req, err := http.NewRequest("GET", strings.TrimRight(s.BaseURL, "/")+"/api/v1/torrents", nil) //nolint:gosec // admin test endpoint
	if err != nil {
		return false, "构造请求失败: " + err.Error()
	}
	req.Header.Set("Authorization", "Bearer "+s.Passkey)

	resp, err := client.Do(req) //nolint:gosec // admin test endpoint
	if err != nil {
		return false, "连接失败: " + err.Error()
	}
	defer func() { httpclient.DrainBody(resp) }()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return false, "Passkey 无效"
	}
	if resp.StatusCode >= 400 {
		return false, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	bodyData, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	bodyStr := string(bodyData)
	lower := strings.ToLower(bodyStr)

	if strings.Contains(lower, `"code":401`) || strings.Contains(lower, `"code":403`) {
		return false, "Passkey 无效"
	}
	if strings.Contains(lower, "缺少认证") || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "invalid") {
		return false, "Passkey 无效"
	}

	return true, "连接成功"
}

func (h *SiteHandler) detectFramework(s *model.Site) *model.DetectResult {
	client := buildSiteHTTPClient(s, 15*time.Second)
	req, err := http.NewRequest("GET", s.BaseURL, nil) //nolint:gosec // admin detect endpoint, URL from site config
	if err != nil {
		return &model.DetectResult{
			Framework:       s.Framework,
			Confidence:      0,
			DetectionDetail: "无法访问站点: " + err.Error(),
		}
	}
	if s.Cookie != "" {
		req.Header.Set("Cookie", s.Cookie)
	}
	resp, err := client.Do(req) //nolint:gosec // admin detect endpoint
	if err != nil {
		return &model.DetectResult{
			Framework:       s.Framework,
			Confidence:      0,
			DetectionDetail: "无法访问站点: " + err.Error(),
		}
	}
	defer func() { httpclient.DrainBody(resp) }()

	bodyData, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	bodyStr := string(bodyData)

	framework := "generic"
	confidence := 0.3
	detail := ""

	switch {
	case strings.Contains(bodyStr, "NexusPHP") || strings.Contains(bodyStr, "nexusphp"):
		framework = "nexusphp"
		confidence = 0.9
		detail = "检测到 NexusPHP 标识"
	case strings.Contains(bodyStr, "UNIT3D") || strings.Contains(bodyStr, "unit3d"):
		framework = "unit3d"
		confidence = 0.9
		detail = "检测到 Unit3D 标识"
	case strings.Contains(bodyStr, "Gazelle") || strings.Contains(bodyStr, "gazelle"):
		framework = "gazelle"
		confidence = 0.9
		detail = "检测到 Gazelle 标识"
	case strings.Contains(bodyStr, "M-Team") || strings.Contains(bodyStr, "mteam"):
		framework = "mteam"
		confidence = 0.9
		detail = "检测到 M-Team 标识"
	case strings.Contains(bodyStr, "TNode") || strings.Contains(bodyStr, "tnode") || strings.Contains(bodyStr, "朱雀"):
		framework = "tnode"
		confidence = 0.9
		detail = "检测到 TNode 标识"
	case strings.Contains(bodyStr, "Luminance") || strings.Contains(bodyStr, "luminance"):
		framework = "luminance"
		confidence = 0.9
		detail = "检测到 Luminance 标识"
	case strings.Contains(bodyStr, "Rousi") || strings.Contains(bodyStr, "rousi"):
		framework = "rousi"
		confidence = 0.9
		detail = "检测到 Rousi 标识"
	case strings.Contains(bodyStr, "reelcore") || strings.Contains(bodyStr, "ReelCore"):
		framework = "cola"
		confidence = 0.9
		detail = "检测到 ReelCore 标识（可乐）"
	case strings.Contains(bodyStr, "Nexus"):
		framework = "nexusphp"
		confidence = 0.7
		detail = "检测到 Nexus 字样（可能是 NexusPHP）"
	default:
		detail = "未能识别框架，使用 generic"
	}

	defaults := model.FrameworkDefaults{
		HashStrategy: frameworkDefaultHash(framework),
		SizeStrategy: frameworkDefaultSize(framework),
		IDStrategy:   frameworkDefaultID(framework),
	}
	switch framework {
	case "nexusphp", "mteam", "cola":
		defaults.DownloadURLTemplate = s.BaseURL + "/download.php?id={id}&passkey={passkey}"
	case "unit3d":
		defaults.IDPattern = `\/torrent\/(\d+)`
		defaults.DownloadURLTemplate = s.BaseURL + "/torrent/download/{id}"
	case "gazelle":
		defaults.DownloadURLTemplate = s.BaseURL + "/torrents.php?action=download&id={id}&authkey={authkey}&torrent_pass={passkey}"
	case "luminance":
		defaults.DownloadURLTemplate = s.BaseURL + "/torrents.php?action=download&id={id}&authkey={authkey}&torrent_pass={passkey}"
	case "tnode":
		defaults.DownloadURLTemplate = s.BaseURL + "/download.php?id={id}&passkey={passkey}"
	case "rousi":
		defaults.IDPattern = "uuid"
		defaults.DownloadURLTemplate = s.BaseURL + "/api/torrent/{id}/download/{passkey}"
	}

	return &model.DetectResult{
		Framework:       framework,
		Confidence:      confidence,
		DetectionDetail: detail,
		Defaults:        defaults,
	}
}

func (h *SiteHandler) extractID(path string, prefix string) (uint, error) {
	p := strings.TrimPrefix(path, prefix)
	p = strings.TrimRight(p, "/")
	n, err := strconv.ParseUint(p, 10, 32)
	if err != nil {
		return 0, apiError(ErrAPIInvalidID, "invalid id", nil)
	}
	return uint(n), nil
}

func defaultStr(val, def string) string {
	if val == "" {
		return def
	}
	return val
}

func buildSiteHTTPClient(s *model.Site, timeout time.Duration) *http.Client {
	return httpclient.NewSiteHTTPClient(httpclient.SiteHTTPConfig{
		Domain:        s.Domain,
		Timeout:       timeout,
		ProxyURL:      s.ProxyURL,
		SkipSSLVerify: s.SkipSSLVerify,
		TLSPretend:    s.TLSPretend, // §59.333
	})
}

func frameworkDefaultHash(fw string) string {
	switch fw {
	case "gazelle", "luminance":
		return "xml_tag"
	case "unit3d":
		return "fake_from_id"
	default:
		return "guid"
	}
}

func frameworkDefaultSize(fw string) string {
	switch fw {
	case "unit3d":
		return "desc_regex"
	case "gazelle", "luminance":
		return "xml_tag"
	default:
		return "enclosure"
	}
}

func frameworkDefaultID(fw string) string {
	switch fw {
	case "unit3d", "gazelle":
		return "link_regex"
	default:
		return "query_param"
	}
}
