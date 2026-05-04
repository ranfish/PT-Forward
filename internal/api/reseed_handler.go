package api

import (
	"encoding/json"
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

	switch {
	case trimmed == "/api/v1/reseed/tasks" || trimmed == "/api/v1/reseed/tasks/":
		if r.Method == http.MethodGet {
			h.handleList(w, r)
		} else if r.Method == http.MethodPost {
			h.handleCreate(w, r)
		} else {
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
			h.handleListMatches(w, r, idStr)
		}
	case "negative-cache":
		h.handleNegativeCache(w, r)
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
		Error(w, http.StatusNotFound, 40400, "辅种任务不存在")
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
	if task.MaxInjectionsPerRun == 0 {
		task.MaxInjectionsPerRun = 100
	}
	if task.ReseedCategory == "" {
		task.ReseedCategory = "cross-seed"
	}

	if err := h.engine.CreateTask(r.Context(), task); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "创建辅种任务失败")
		return
	}
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

	if err := h.engine.UpdateTask(r.Context(), task); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "更新辅种任务失败")
		return
	}
	Success(w, task)
}

func (h *ReseedHandler) handleDelete(w http.ResponseWriter, r *http.Request, id uint) {
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
		Error(w, http.StatusNotFound, 40400, "辅种任务不存在")
		return
	}

	result, err := h.engine.RunTask(r.Context(), task)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "执行辅种任务失败")
		return
	}

	h.logger.Info("reseed task triggered", zap.Uint("id", id))
	Success(w, result)
}

func (h *ReseedHandler) handleCancel(w http.ResponseWriter, r *http.Request, id uint) {
	if r.Method != http.MethodPost {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	_, err := h.engine.GetTask(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, 40400, "辅种任务不存在")
		return
	}

	h.engine.CancelTask(id)
	h.logger.Info("reseed task cancelled", zap.Uint("id", id))
	Success(w, map[string]interface{}{"message": "任务已取消"})
}

func (h *ReseedHandler) handleListMatches(w http.ResponseWriter, r *http.Request, taskIDStr string) {
	if r.Method != http.MethodGet {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	infoHash := r.URL.Query().Get("infoHash")
	if infoHash == "" {
		Success(w, map[string]interface{}{"items": []model.ReseedMatch{}, "total": 0})
		return
	}

	matches, err := h.engine.FindMatchesByInfoHash(r.Context(), infoHash)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询匹配记录失败")
		return
	}

	Success(w, map[string]interface{}{
		"items": matches,
		"total": len(matches),
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
		} else {
			Error(w, http.StatusNotFound, 40400, "匹配记录不存在")
		}
		return
	}

	h.logger.Info("reseed match retry triggered", zap.Uint("id", id))
	Success(w, match)
}

func (h *ReseedHandler) handleNegativeCache(w http.ResponseWriter, r *http.Request) {
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
