package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type PublishHandler struct {
	pipeline *publish.Pipeline
	db       *gorm.DB
	logger   *zap.Logger
}

func NewPublishHandler(pipeline *publish.Pipeline, logger *zap.Logger, db *gorm.DB) *PublishHandler {
	return &PublishHandler{pipeline: pipeline, logger: logger, db: db}
}

func (h *PublishHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimRight(path, "/")

	switch {
	case trimmed == "/api/v1/publish/tasks" || trimmed == "/api/v1/publish/tasks/":
		switch r.Method {
		case http.MethodGet:
			h.handleListTasks(w, r)
		case http.MethodPost:
			h.handleCreateTask(w, r)
		default:
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return

	case trimmed == "/api/v1/publish/candidates" || trimmed == "/api/v1/publish/candidates/":
		if r.Method == http.MethodGet {
			h.handleListCandidates(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return

	case trimmed == "/api/v1/publish/results" || trimmed == "/api/v1/publish/results/":
		if r.Method == http.MethodGet {
			h.handleListResults(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	if strings.Contains(trimmed, "/publish/tasks/") {
		remaining := extractLastSegment(trimmed, "/api/v1/publish/tasks/")
		parts := strings.SplitN(remaining, "/", 2)
		id, err := strconv.ParseUint(parts[0], 10, 64)
		if err != nil {
			Error(w, http.StatusBadRequest, 40001, "无效的任务 ID")
			return
		}

		if len(parts) == 2 && parts[1] == "cancel" && r.Method == http.MethodPost {
			h.handleCancelTask(w, r, uint(id))
			return
		}

		switch r.Method {
		case http.MethodGet:
			h.handleGetTask(w, r, uint(id))
		case http.MethodDelete:
			h.handleDeleteTask(w, r, uint(id))
		default:
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	if strings.Contains(trimmed, "/publish/candidates/") {
		remaining := strings.TrimPrefix(trimmed, "/api/v1/publish/candidates/")
		remaining = strings.TrimRight(remaining, "/")

		parts := strings.SplitN(remaining, "/", 2)
		id, err := strconv.ParseUint(parts[0], 10, 64)
		if err != nil {
			Error(w, http.StatusBadRequest, 40001, "无效的候选 ID")
			return
		}

		if len(parts) == 2 && parts[1] == "publish" && r.Method == http.MethodPost {
			h.handleManualPublish(w, r, uint(id))
			return
		}

		switch r.Method {
		case http.MethodGet:
			h.handleGetCandidate(w, r, uint(id))
		case http.MethodDelete:
			h.handleDeleteCandidate(w, r, uint(id))
		default:
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	if strings.Contains(trimmed, "/publish/groups") {
		remaining := strings.TrimPrefix(trimmed, "/api/v1/publish/groups")
		remaining = strings.TrimRight(remaining, "/")
		remaining = strings.TrimPrefix(remaining, "/")

		if remaining == "" {
			h.handleListGroups(w, r)
			return
		}

		parts := strings.SplitN(remaining, "/", 3)
		groupID, err := strconv.ParseUint(parts[0], 10, 64)
		if err != nil {
			Error(w, http.StatusBadRequest, 40001, "无效的组 ID")
			return
		}

		if len(parts) == 1 {
			switch r.Method {
			case http.MethodGet:
				h.handleGetGroup(w, r, uint(groupID))
			case http.MethodDelete:
				h.handleDeleteGroup(w, r, uint(groupID))
			default:
				Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
			}
			return
		}

		if len(parts) >= 2 && parts[1] == "lifecycle" {
			if len(parts) == 3 {
				switch parts[2] {
				case "pause":
					h.handleLifecyclePause(w, r, uint(groupID))
				case "resume":
					h.handleLifecycleResume(w, r, uint(groupID))
				case "delete":
					h.handleLifecycleDelete(w, r, uint(groupID))
				default:
					Error(w, http.StatusNotFound, 40400, "路径不存在")
				}
			} else {
				Error(w, http.StatusNotFound, 40400, "路径不存在")
			}
			return
		}

		Error(w, http.StatusNotFound, 40400, "路径不存在")
		return
	}

	Error(w, http.StatusNotFound, 40400, "接口不存在")
}

func (h *PublishHandler) handleListTasks(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if page < 1 {
		page = 1
	} else if page > 10000 {
		page = 10000
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	tasks, total, err := h.pipeline.ListTasks(r.Context(), offset, pageSize)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询发布任务失败")
		return
	}
	Success(w, map[string]interface{}{
		"items": tasks,
		"total": total,
		"page":  page,
	})
}

func (h *PublishHandler) handleGetTask(w http.ResponseWriter, r *http.Request, id uint) {
	task, err := h.pipeline.GetTask(r.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Error(w, http.StatusNotFound, 40400, "发布任务不存在")
		} else {
			Error(w, http.StatusInternalServerError, 50000, "查询发布任务失败")
		}
		return
	}
	Success(w, task)
}

func (h *PublishHandler) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type         string   `json:"type"`
		SourceSiteID uint     `json:"sourceSiteId"`
		TargetSites  []string `json:"targetSites"`
		ManualCheck  bool     `json:"manualCheck"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.SourceSiteID == 0 {
		Error(w, http.StatusBadRequest, 40001, "sourceSiteId 为必填项")
		return
	}
	if len(req.TargetSites) == 0 {
		Error(w, http.StatusBadRequest, 40001, "至少需要一个目标站点")
		return
	}

	task := &model.PublishTask{
		SourceSiteID: req.SourceSiteID,
		TargetSites:  req.TargetSites,
		ManualCheck:  req.ManualCheck,
		Status:       model.PublishTaskPending,
	}
	if req.Type != "" {
		task.Type = model.PublishTaskType(req.Type)
	}

	if err := h.pipeline.CreateTask(r.Context(), task); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "创建发布任务失败")
		return
	}

	h.logger.Info("publish task created", zap.Uint("id", task.ID))
	Success(w, task)
}

func (h *PublishHandler) handleDeleteTask(w http.ResponseWriter, r *http.Request, id uint) {
	task, err := h.pipeline.GetTask(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, 40400, "发布任务不存在")
		return
	}
	if err := h.pipeline.UpdateTaskStatus(r.Context(), task.ID, model.PublishTaskFailed); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "删除发布任务失败")
		return
	}
	h.logger.Info("publish task deleted", zap.Uint("id", id))
	Success(w, map[string]interface{}{
		"message": "发布任务已删除",
		"id":      id,
	})
}

func (h *PublishHandler) handleCancelTask(w http.ResponseWriter, r *http.Request, id uint) {
	task, err := h.pipeline.GetTask(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, 40400, "发布任务不存在")
		return
	}
	if task.Status != model.PublishTaskPending && task.Status != model.PublishTaskChecked && task.Status != model.PublishTaskPublishing {
		Error(w, http.StatusBadRequest, 40001, "只能取消进行中的任务")
		return
	}
	if err := h.pipeline.UpdateTaskStatus(r.Context(), task.ID, model.PublishTaskFailed); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "取消发布任务失败")
		return
	}
	h.logger.Info("publish task cancelled", zap.Uint("id", id))
	Success(w, map[string]interface{}{
		"message": "发布任务已取消",
		"id":      id,
	})
}

func (h *PublishHandler) handleListCandidates(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	candidates, err := h.pipeline.ListPendingCandidates(r.Context(), limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询发布候选失败")
		return
	}
	Success(w, map[string]interface{}{
		"items": candidates,
		"total": len(candidates),
	})
}

func (h *PublishHandler) handleGetCandidate(w http.ResponseWriter, r *http.Request, id uint) {
	candidate, err := h.pipeline.GetCandidate(r.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Error(w, http.StatusNotFound, 40400, "发布候选不存在")
		} else {
			Error(w, http.StatusInternalServerError, 50000, "查询发布候选失败")
		}
		return
	}
	Success(w, candidate)
}

func (h *PublishHandler) handleListResults(w http.ResponseWriter, r *http.Request) {
	candidateIDStr := r.URL.Query().Get("candidateId")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	var candidateID uint
	if candidateIDStr != "" {
		n, err := strconv.ParseUint(candidateIDStr, 10, 64)
		if err != nil {
			Error(w, http.StatusBadRequest, 40001, "无效的 candidateId")
			return
		}
		candidateID = uint(n)
	}

	results, err := h.pipeline.ListResults(r.Context(), candidateID, limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询发布结果失败")
		return
	}
	Success(w, map[string]interface{}{
		"items": results,
		"total": len(results),
	})
}

func (h *PublishHandler) handleDeleteCandidate(w http.ResponseWriter, r *http.Request, id uint) {
	if err := h.pipeline.DeleteCandidate(r.Context(), id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Error(w, http.StatusNotFound, 40400, "发布候选不存在")
		} else {
			Error(w, http.StatusInternalServerError, 50000, "删除发布候选失败")
		}
		return
	}
	h.logger.Info("publish candidate deleted", zap.Uint("id", id))
	Success(w, map[string]interface{}{
		"message": "发布候选已删除",
		"id":      id,
	})
}

func (h *PublishHandler) handleManualPublish(w http.ResponseWriter, r *http.Request, id uint) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	candidate, err := h.pipeline.PublishCandidate(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "合规检查") {
			Error(w, http.StatusBadRequest, 40001, err.Error())
		} else {
			Error(w, http.StatusInternalServerError, 50000, err.Error())
		}
		return
	}
	h.logger.Info("publish candidate manually triggered", zap.Uint("id", id))
	Success(w, candidate)
}

func (h *PublishHandler) handleListGroups(w http.ResponseWriter, r *http.Request) {
	q := h.db.Model(&model.PublishGroup{})
	if status := r.URL.Query().Get("status"); status != "" {
		q = q.Where("status = ?", status)
	}

	var total int64
	q.Count(&total)

	var groups []model.PublishGroup
	if err := q.Session(&gorm.Session{}).Order("created_at DESC").Limit(100).Find(&groups).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询发布组失败")
		return
	}

	Success(w, map[string]interface{}{
		"items": groups,
		"total": total,
	})
}

func (h *PublishHandler) handleGetGroup(w http.ResponseWriter, r *http.Request, id uint) {
	var group model.PublishGroup
	if err := h.db.First(&group, id).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "发布组不存在")
		return
	}

	var members []model.PublishGroupMember
	if err := h.db.Where("publish_group_id = ?", id).Find(&members).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询发布成员失败")
		return
	}

	Success(w, map[string]interface{}{
		"id":          group.ID,
		"candidateId": group.CandidateID,
		"status":      group.Status,
		"sourceHash":  group.SourceHash,
		"sourceSite":  group.SourceSite,
		"createdAt":   group.CreatedAt,
		"updatedAt":   group.UpdatedAt,
		"members":     members,
	})
}

func (h *PublishHandler) handleDeleteGroup(w http.ResponseWriter, r *http.Request, id uint) {
	if err := h.db.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("publish_group_id = ?", id).Delete(&model.PublishGroupMember{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.PublishGroup{}, id).Error
	}); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "删除发布组失败")
		return
	}
	Success(w, nil)
}

func (h *PublishHandler) handleLifecyclePause(w http.ResponseWriter, r *http.Request, id uint) {
	now := time.Now()
	if err := h.db.WithContext(r.Context()).Model(&model.PublishGroupMember{}).
		Where("publish_group_id = ? AND paused = ?", id, false).
		Updates(map[string]interface{}{"paused": true, "status_at": now}).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "暂停失败")
		return
	}
	h.logger.Info("publish group paused", zap.Uint("id", id))
	Success(w, map[string]interface{}{"message": "已暂停"})
}

func (h *PublishHandler) handleLifecycleResume(w http.ResponseWriter, r *http.Request, id uint) {
	now := time.Now()
	if err := h.db.WithContext(r.Context()).Model(&model.PublishGroupMember{}).
		Where("publish_group_id = ? AND paused = ?", id, true).
		Updates(map[string]interface{}{"paused": false, "status_at": now}).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "恢复失败")
		return
	}
	h.logger.Info("publish group resumed", zap.Uint("id", id))
	Success(w, map[string]interface{}{"message": "已恢复"})
}

func (h *PublishHandler) handleLifecycleDelete(w http.ResponseWriter, r *http.Request, id uint) {
	if err := h.db.Model(&model.PublishGroup{}).Where("id = ?", id).
		Updates(map[string]interface{}{"status": "deleting", "updated_at": time.Now()}).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "触发删除失败")
		return
	}
	h.logger.Info("publish group deleting", zap.Uint("id", id))
	Success(w, map[string]interface{}{"message": "删除已触发"})
}
