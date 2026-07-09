package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/reseed"
	"go.uber.org/zap"
)

type ReseedHandler struct {
	engine *reseed.Engine
	logger *zap.Logger
}

func NewReseedHandler(engine *reseed.Engine, logger *zap.Logger) *ReseedHandler {
	return &ReseedHandler{engine: engine, logger: logger}
}

func (h *ReseedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimRight(path, "/")

	if trimmed == "/api/v1/reseed/tasks" || trimmed == "/api/v1/reseed/tasks/" {
		switch r.Method {
		case http.MethodGet:
			h.handleList(w, r)
		case http.MethodPost:
			h.handleCreate(w, r)
		default:
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	remaining := strings.TrimPrefix(trimmed, "/api/v1/reseed/tasks/")
	if remaining == "" {
		h.handleList(w, r)
		return
	}

	parts := strings.SplitN(remaining, "/", 3)
	idStr := parts[0]

	id, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的任务 ID")
		return
	}
	taskID := uint(id)

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			h.handleGet(w, r, taskID)
		case http.MethodPut:
			h.handleUpdate(w, r, taskID)
		case http.MethodDelete:
			h.handleDelete(w, r, taskID)
		default:
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	subResource := parts[1]
	switch subResource {
	case "trigger":
		h.handleTrigger(w, r, taskID)
	case "cancel":
		h.handleCancel(w, r, taskID)
	case "matches":
		if len(parts) == 3 && parts[2] != "" {
			if parts[2] == "batch-retry" {
				h.handleBatchRetryMatches(w, r, taskID)
				return
			}
			if parts[2] == "batch-delete" {
				h.handleBatchDeleteMatches(w, r, taskID)
				return
			}
			if parts[2] == "clear" {
				h.handleClearMatches(w, r, taskID)
				return
			}
			subParts := strings.SplitN(parts[2], "/", 2)
			if len(subParts) == 2 && subParts[1] == "retry" {
				matchID, retryErr := parseMatchID(subParts[0])
				if retryErr != nil {
					Error(w, http.StatusBadRequest, 40001, "无效的匹配 ID")
					return
				}
				h.handleRetryMatch(w, r, matchID)
			} else {
				h.handleGetMatch(w, r, idStr, parts[2])
			}
		} else {
			h.handleListMatches(w, r, taskID)
		}
	case "negative-cache":
		h.handleNegativeCache(w, r, taskID)
	case "feature-logs":
		h.handleListFeatureLogs(w, r, taskID)
	case "iyuu-logs":
		h.handleListIYUULogs(w, r, taskID)
	default:
		Error(w, http.StatusNotFound, 40400, "路径不存在")
	}
}

func (h *ReseedHandler) handleList(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.engine.ListTasks(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询辅种任务失败")
		return
	}
	Success(w, map[string]interface{}{
		"items": tasks,
		"total": len(tasks),
	})
}

func (h *ReseedHandler) handleGet(w http.ResponseWriter, r *http.Request, id uint) {
	task, err := h.engine.GetTask(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			Error(w, http.StatusNotFound, 40400, "辅种任务不存在")
		} else {
			Error(w, http.StatusInternalServerError, 50000, "查询辅种任务失败")
		}
		return
	}
	Success(w, task)
}

func (h *ReseedHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name                 string  `json:"name"`
		Enabled              bool    `json:"enabled"`
		ClientIDs            string  `json:"clientIds"`
		SourceSiteIDs        string  `json:"sourceSiteIds"`
		TargetSiteIDs        string  `json:"targetSiteIds"`
		SizeTolerancePercent float64 `json:"sizeTolerancePercent"`
		ConfidenceThreshold  float64 `json:"confidenceThreshold"`
		Schedule             string  `json:"schedule"`
		MaxInjectionsPerRun  int     `json:"maxInjectionsPerRun"`
		ReseedCategory       string  `json:"reseedCategory"`
		ReseedTags           string  `json:"reseedTags"`
		TargetSiteExcludes   string  `json:"targetSiteExcludes"`
		ReleaseGroupExcludes string  `json:"releaseGroupExcludes"`
		CategoryExcludes     string  `json:"categoryExcludes"`
		TitleKeywordExcludes string  `json:"titleKeywordExcludes"`
		MatchMethods         string  `json:"matchMethods"`
		FallbackEnabled      bool    `json:"fallbackEnabled"`
		MaxFallbacks         int     `json:"maxFallbacks"`
		EngineMode           string  `json:"engineMode"`
		InjectionIntervalS   int     `json:"injectionIntervalS"`
		InjectionJitterS     int     `json:"injectionJitterS"`
		InjectionConcurrency int     `json:"injectionConcurrency"`
		ScanConcurrency      int     `json:"scanConcurrency"`
		MaxRetries           int     `json:"maxRetries"`
		RetryIntervalH       int     `json:"retryIntervalH"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.Name == "" {
		Error(w, http.StatusBadRequest, 40001, "name 为必填项")
		return
	}

	task := &model.ReseedTask{
		Name:                 req.Name,
		Enabled:              req.Enabled,
		ClientIDs:            req.ClientIDs,
		SourceSiteIDs:        req.SourceSiteIDs,
		TargetSiteIDs:        req.TargetSiteIDs,
		SizeTolerancePercent: req.SizeTolerancePercent,
		ConfidenceThreshold:  req.ConfidenceThreshold,
		Schedule:             req.Schedule,
		MaxInjectionsPerRun:  req.MaxInjectionsPerRun,
		ReseedCategory:       req.ReseedCategory,
		ReseedTags:           req.ReseedTags,
		TargetSiteExcludes:   req.TargetSiteExcludes,
		ReleaseGroupExcludes: req.ReleaseGroupExcludes,
		CategoryExcludes:     req.CategoryExcludes,
		TitleKeywordExcludes: req.TitleKeywordExcludes,
		MatchMethods:         req.MatchMethods,
		FallbackEnabled:      req.FallbackEnabled,
		MaxFallbacks:         req.MaxFallbacks,
		EngineMode:           req.EngineMode,
		InjectionIntervalS:   req.InjectionIntervalS,
		InjectionJitterS:     req.InjectionJitterS,
		InjectionConcurrency: req.InjectionConcurrency,
		ScanConcurrency:      req.ScanConcurrency,
		MaxRetries:           req.MaxRetries,
		RetryIntervalH:       req.RetryIntervalH,
	}
	if task.Schedule == "" {
		task.Schedule = "0 */6 * * *"
	}
	if task.SizeTolerancePercent == 0 {
		task.SizeTolerancePercent = 1.0
	}
	if task.ConfidenceThreshold == 0 {
		task.ConfidenceThreshold = 0.7
	}
	if task.ReseedCategory == "" {
		task.ReseedCategory = "cross-seed"
	}
	if task.ReseedTags == "" {
		task.ReseedTags = "reseed,pt-forward"
	}
	if task.EngineMode == "" {
		task.EngineMode = model.ReseedModeSeedFeature
	}
	if !model.ValidReseedMode(task.EngineMode) {
		Error(w, http.StatusBadRequest, 40001, "engineMode 必须为 seed_feature 或 iyuu_cloud")
		return
	}
	if task.MatchMethods == "" {
		task.MatchMethods = model.ReseedModeDefaults[task.EngineMode]
	}
	if task.EngineMode == model.ReseedModeSeedFeature && !strings.Contains(task.MatchMethods, "pieces_hash") {
		task.MatchMethods = "pieces_hash," + task.MatchMethods
	}

	if err := h.engine.ValidateClientRoles(r.Context(), req.ClientIDs); err != nil {
		Error(w, http.StatusBadRequest, 40001, err.Error())
		return
	}

	if err := h.engine.CreateTask(r.Context(), task); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "创建辅种任务失败")
		return
	}
	h.engine.SyncTaskSchedule(r.Context(), task)
	Success(w, task)
}

func (h *ReseedHandler) handleUpdate(w http.ResponseWriter, r *http.Request, id uint) {
	task, err := h.engine.GetTask(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, 40400, "辅种任务不存在")
		return
	}

	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	if v, ok := req["name"].(string); ok {
		task.Name = v
	}
	if v, ok := req["enabled"].(bool); ok {
		task.Enabled = v
	}
	if v, ok := req["clientIds"].(string); ok {
		task.ClientIDs = v
	}
	if v, ok := req["sourceSiteIds"].(string); ok {
		task.SourceSiteIDs = v
	}
	if v, ok := req["targetSiteIds"].(string); ok {
		task.TargetSiteIDs = v
	}
	if v, ok := req["sizeTolerancePercent"].(float64); ok {
		task.SizeTolerancePercent = v
	}
	if v, ok := req["confidenceThreshold"].(float64); ok {
		task.ConfidenceThreshold = v
	}
	if v, ok := req["schedule"].(string); ok {
		task.Schedule = v
	}
	if v, ok := req["maxInjectionsPerRun"].(float64); ok {
		task.MaxInjectionsPerRun = int(v)
	}
	if v, ok := req["reseedCategory"].(string); ok {
		task.ReseedCategory = v
	}
	if v, ok := req["reseedTags"].(string); ok {
		task.ReseedTags = v
	}
	if v, ok := req["targetSiteExcludes"].(string); ok {
		task.TargetSiteExcludes = v
	}
	if v, ok := req["releaseGroupExcludes"].(string); ok {
		task.ReleaseGroupExcludes = v
	}
	if v, ok := req["categoryExcludes"].(string); ok {
		task.CategoryExcludes = v
	}
	if v, ok := req["titleKeywordExcludes"].(string); ok {
		task.TitleKeywordExcludes = v
	}
	if v, ok := req["matchMethods"].(string); ok {
		task.MatchMethods = v
	}
	if v, ok := req["fallbackEnabled"].(bool); ok {
		task.FallbackEnabled = v
	}
	if v, ok := req["maxFallbacks"].(float64); ok {
		task.MaxFallbacks = int(v)
	}
	if v, ok := req["engineMode"].(string); ok {
		if v != "" && !model.ValidReseedMode(v) {
			Error(w, http.StatusBadRequest, 40001, "engineMode 必须为 seed_feature 或 iyuu_cloud")
			return
		}
		task.EngineMode = v
		if task.EngineMode != "" && task.MatchMethods == "" {
			task.MatchMethods = model.ReseedModeDefaults[task.EngineMode]
		}
	}
	if task.EngineMode == model.ReseedModeSeedFeature && task.MatchMethods != "" && !strings.Contains(task.MatchMethods, "pieces_hash") {
		task.MatchMethods = "pieces_hash," + task.MatchMethods
	}
	if v, ok := req["injectionIntervalS"].(float64); ok {
		task.InjectionIntervalS = int(v)
	}
	if v, ok := req["injectionJitterS"].(float64); ok {
		task.InjectionJitterS = int(v)
	}
	if v, ok := req["injectionConcurrency"].(float64); ok {
		task.InjectionConcurrency = int(v)
	}
	if v, ok := req["scanConcurrency"].(float64); ok {
		task.ScanConcurrency = int(v)
	}
	if v, ok := req["maxRetries"].(float64); ok {
		task.MaxRetries = int(v)
	}
	if v, ok := req["retryIntervalH"].(float64); ok {
		task.RetryIntervalH = int(v)
	}

	if err := h.engine.ValidateClientRoles(r.Context(), task.ClientIDs); err != nil {
		Error(w, http.StatusBadRequest, 40001, err.Error())
		return
	}

	if err := h.engine.UpdateTask(r.Context(), task); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "更新辅种任务失败")
		return
	}
	h.engine.SyncTaskSchedule(r.Context(), task)
	Success(w, task)
}

func (h *ReseedHandler) handleDelete(w http.ResponseWriter, r *http.Request, id uint) {
	h.engine.RemoveTaskSchedule(id)
	if err := h.engine.DeleteTask(r.Context(), id); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "删除辅种任务失败")
		return
	}
	h.logger.Info("reseed task deleted", zap.Uint("id", id))
	Success(w, nil)
}

func (h *ReseedHandler) handleTrigger(w http.ResponseWriter, r *http.Request, id uint) {
	if r.Method != http.MethodPost {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	task, err := h.engine.GetTask(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			Error(w, http.StatusNotFound, 40400, "辅种任务不存在")
		} else {
			Error(w, http.StatusInternalServerError, 50000, "查询辅种任务失败")
		}
		return
	}

	if task.Status == model.ReseedTaskRunning {
		Error(w, http.StatusConflict, 40900, "任务正在执行中")
		return
	}

	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		result, err := h.engine.RunTask(ctx, task)
		if err != nil {
			h.logger.Warn("reseed task async execution failed",
				zap.Uint("id", id), zap.Error(err))
			return
		}
		h.logger.Info("reseed task async completed",
			zap.Uint("id", id),
			zap.Int("matched", result.Matched),
			zap.Int("injected", result.Injected),
			zap.Int("failed", result.Failed),
			zap.Int("blocked", result.Blocked))
	}()

	h.logger.Info("reseed task triggered async", zap.Uint("id", id))
	Success(w, map[string]interface{}{"message": "任务已触发", "status": "running"})
}

func (h *ReseedHandler) handleCancel(w http.ResponseWriter, r *http.Request, id uint) {
	if r.Method != http.MethodPost {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	_, err := h.engine.GetTask(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			Error(w, http.StatusNotFound, 40400, "辅种任务不存在")
		} else {
			Error(w, http.StatusInternalServerError, 50000, "查询辅种任务失败")
		}
		return
	}

	h.engine.CancelTask(id)
	h.logger.Info("reseed task cancelled", zap.Uint("id", id))
	Success(w, map[string]interface{}{"message": "任务已取消"})
}

func (h *ReseedHandler) handleListMatches(w http.ResponseWriter, r *http.Request, taskID uint) {
	if r.Method != http.MethodGet {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	clientID := r.URL.Query().Get("clientId")
	site := r.URL.Query().Get("site")
	torrentID := r.URL.Query().Get("torrentId")
	status := r.URL.Query().Get("status")
	orderField := r.URL.Query().Get("orderField")
	order := r.URL.Query().Get("order")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}

	query := h.engine.DB().Model(&model.ReseedMatch{}).Where("task_id = ? OR task_id = 0", taskID)

	if clientID != "" {
		query = query.Where("client_id = ?", clientID)
	}
	if site != "" {
		query = query.Where("target_site = ? OR source_site = ?", site, site)
	}
	if torrentID != "" {
		query = query.Where("source_torrent_id LIKE ? OR target_torrent_id LIKE ?", "%"+torrentID+"%", "%"+torrentID+"%")
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	allowedOrders := map[string]bool{
		"created_at": true, "status": true, "target_site": true, "source_site": true,
		"client_id": true, "confidence": true, "source_torrent_id": true,
		"target_torrent_id": true, "directory": true,
	}
	orderClause := "created_at DESC"
	if allowedOrders[orderField] {
		dir := "DESC"
		if strings.ToLower(order) == "asc" {
			dir = "ASC"
		}
		orderClause = orderField + " " + dir
	}

	var matches []model.ReseedMatch
	query.Order(orderClause).Offset((page - 1) * pageSize).Limit(pageSize).Find(&matches)

	type siteBrief struct {
		BaseURL   string
		Framework string
	}
	siteInfos := make(map[string]siteBrief)
	var sites []model.Site
	h.engine.DB().Select("name, base_url, framework").Find(&sites)
	for _, s := range sites {
		siteInfos[s.Name] = siteBrief{BaseURL: s.BaseURL, Framework: s.Framework}
	}

	type matchWithURL struct {
		model.ReseedMatch
		TargetDetailURL string `json:"target_detail_url"`
	}
	items := make([]matchWithURL, 0, len(matches))
	for _, m := range matches {
		item := matchWithURL{ReseedMatch: m}
		if si, ok := siteInfos[m.TargetSite]; ok && m.TargetTorrentID != "" {
			item.TargetDetailURL = buildDetailURL(si.BaseURL, si.Framework, m.TargetTorrentID)
		}
		items = append(items, item)
	}

	Success(w, map[string]interface{}{
		"items":    items,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (h *ReseedHandler) handleListFeatureLogs(w http.ResponseWriter, r *http.Request, taskID uint) {
	if r.Method != http.MethodGet {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	query := h.engine.DB().Model(&model.ReseedFeatureLog{}).Where("task_id = ?", taskID)
	var total int64
	query.Count(&total)
	var logs []model.ReseedFeatureLog
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs)
	type featureStats struct {
		TotalCalls   int64
		TotalQueried int64
		TotalMatched int64
	}
	var stats featureStats
	h.engine.DB().Model(&model.ReseedFeatureLog{}).Where("task_id = ?", taskID).
		Select("COUNT(*) as total_calls, COALESCE(SUM(queried),0) as total_queried, COALESCE(SUM(matched),0) as total_matched").
		Scan(&stats)
	Success(w, map[string]interface{}{
		"items":    logs,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
		"stats":    stats,
	})
}

func (h *ReseedHandler) handleListIYUULogs(w http.ResponseWriter, r *http.Request, taskID uint) {
	if r.Method != http.MethodGet {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}

	query := h.engine.DB().Model(&model.ReseedIYUULog{}).Where("task_id = ?", taskID)

	var total int64
	query.Count(&total)

	var logs []model.ReseedIYUULog
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs)

	type iyuuLogStats struct {
		TotalCalls    int64
		SuccessCalls  int64
		ErrorCalls    int64
		TotalRequests int64
		TotalMatched  int64
		TotalTargets  int64
	}
	var stats iyuuLogStats
	h.engine.DB().Model(&model.ReseedIYUULog{}).Where("task_id = ?", taskID).
		Select("COUNT(*) as total_calls, COALESCE(SUM(CASE WHEN status='success' THEN 1 ELSE 0 END),0) as success_calls, COALESCE(SUM(CASE WHEN status='error' THEN 1 ELSE 0 END),0) as error_calls, COALESCE(SUM(request_hashes),0) as total_requests, COALESCE(SUM(matched_hashes),0) as total_matched, COALESCE(SUM(response_targets),0) as total_targets").
		Scan(&stats)

	Success(w, map[string]interface{}{
		"items":    logs,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
		"stats":    stats,
	})
}

func (h *ReseedHandler) handleGetMatch(w http.ResponseWriter, r *http.Request, taskIDStr string, matchID string) {
	if r.Method != http.MethodGet {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	id, err := strconv.ParseUint(matchID, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的匹配 ID")
		return
	}

	match, err := h.engine.FindMatchByID(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, 40400, "匹配记录不存在")
		return
	}

	Success(w, match)
}

func (h *ReseedHandler) handleRetryMatch(w http.ResponseWriter, r *http.Request, id uint) {
	if r.Method != http.MethodPost {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	match, err := h.engine.RetryMatch(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "只能重试") {
			Error(w, http.StatusBadRequest, 40001, err.Error())
		} else if strings.Contains(err.Error(), "not found") {
			Error(w, http.StatusNotFound, 40400, "匹配记录不存在")
		} else {
			Error(w, http.StatusInternalServerError, 50000, "重试匹配失败")
		}
		return
	}

	h.logger.Info("reseed match retry triggered", zap.Uint("id", id))
	Success(w, match)
}

func (h *ReseedHandler) handleBatchRetryMatches(w http.ResponseWriter, r *http.Request, taskID uint) {
	if r.Method != http.MethodPost {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}
	var req struct {
		MatchIDs []uint `json:"match_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if len(req.MatchIDs) == 0 {
		Error(w, http.StatusBadRequest, 40001, "match_ids 不能为空")
		return
	}
	if len(req.MatchIDs) > 100 {
		Error(w, http.StatusBadRequest, 40001, "单次最多重试 100 条")
		return
	}

	_ = taskID

	var succeeded, failed uint
	var failMsgs []string
	for _, id := range req.MatchIDs {
		if _, err := h.engine.RetryMatch(r.Context(), id); err != nil {
			failed++
			failMsgs = append(failMsgs, fmt.Sprintf("id=%d: %v", id, err))
		} else {
			succeeded++
		}
	}

	h.logger.Info("reseed matches batch retry",
		zap.Uint("task_id", taskID),
		zap.Int("count", len(req.MatchIDs)),
		zap.Uint("succeeded", succeeded),
		zap.Uint("failed", failed))

	Success(w, map[string]interface{}{
		"succeeded": succeeded,
		"failed":    failed,
		"messages":  failMsgs,
	})
}

func (h *ReseedHandler) handleBatchDeleteMatches(w http.ResponseWriter, r *http.Request, taskID uint) {
	if r.Method != http.MethodPost {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}
	var req struct {
		MatchIDs []uint `json:"match_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if len(req.MatchIDs) == 0 {
		Error(w, http.StatusBadRequest, 40001, "match_ids 不能为空")
		return
	}

	_ = taskID

	result := h.engine.DB().Where("id IN ?", req.MatchIDs).Delete(&model.ReseedMatch{})
	if result.Error != nil {
		Error(w, http.StatusInternalServerError, 50000, "删除失败")
		return
	}

	h.logger.Info("reseed matches batch delete",
		zap.Uint("task_id", taskID),
		zap.Int("count", len(req.MatchIDs)),
		zap.Int64("deleted", result.RowsAffected))

	Success(w, map[string]interface{}{
		"deleted": result.RowsAffected,
	})
}

func (h *ReseedHandler) handleClearMatches(w http.ResponseWriter, r *http.Request, taskID uint) {
	if r.Method != http.MethodPost {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}
	result := h.engine.DB().Where("task_id = ? OR task_id = 0", taskID).Delete(&model.ReseedMatch{})
	if result.Error != nil {
		Error(w, http.StatusInternalServerError, 50000, "清除失败")
		return
	}
	h.logger.Info("reseed matches cleared", zap.Uint("task_id", taskID), zap.Int64("deleted", result.RowsAffected))
	Success(w, map[string]interface{}{
		"deleted": result.RowsAffected,
	})
}

func (h *ReseedHandler) handleNegativeCache(w http.ResponseWriter, r *http.Request, taskID uint) {
	if r.Method == http.MethodGet {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
		if pageSize < 1 || pageSize > 200 {
			pageSize = 20
		}
		query := h.engine.DB().Model(&model.ReseedNegativeCache{})
		var total int64
		query.Count(&total)
		var items []model.ReseedNegativeCache
		query.Order("expires_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items)
		Success(w, map[string]interface{}{
			"items":    items,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		})
		return
	}
	if r.Method == http.MethodDelete {
		infoHash := r.URL.Query().Get("infoHash")
		site := r.URL.Query().Get("site")
		if infoHash == "" {
			Error(w, http.StatusBadRequest, 40001, "infoHash 为必填项")
			return
		}

		deleted, err := h.engine.DeleteNegativeCache(r.Context(), infoHash, site)
		if err != nil {
			Error(w, http.StatusInternalServerError, 50000, "删除负面缓存失败")
			return
		}

		Success(w, map[string]interface{}{
			"deleted": deleted,
		})
		return
	}

	Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
}

func parseMatchID(s string) (uint, error) {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(n), nil
}
