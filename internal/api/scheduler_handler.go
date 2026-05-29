package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/scheduler"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type SchedulerHandler struct {
	registry *scheduler.Registry
	db       *gorm.DB
	logger   *zap.Logger
}

func NewSchedulerHandler(registry *scheduler.Registry, db *gorm.DB, logger *zap.Logger) *SchedulerHandler {
	return &SchedulerHandler{registry: registry, db: db, logger: logger}
}

func (h *SchedulerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimRight(path, "/")

	switch {
	case r.Method == http.MethodGet && strings.HasSuffix(trimmed, "/scheduler/tasks"):
		h.handleList(w, r)
	case r.Method == http.MethodGet && strings.Contains(trimmed, "/scheduler/tasks/"):
		name := h.extractTaskName(trimmed, "/scheduler/tasks/")
		h.handleGet(w, r, name)
	case r.Method == http.MethodPost && strings.Contains(trimmed, "/pause"):
		name := h.extractTaskName(trimmed, "/scheduler/tasks/")
		name = strings.TrimSuffix(name, "/pause")
		h.handlePause(w, r, name)
	case r.Method == http.MethodPost && strings.Contains(trimmed, "/resume"):
		name := h.extractTaskName(trimmed, "/scheduler/tasks/")
		name = strings.TrimSuffix(name, "/resume")
		h.handleResume(w, r, name)
	case r.Method == http.MethodPost && strings.Contains(trimmed, "/trigger"):
		name := h.extractTaskName(trimmed, "/scheduler/tasks/")
		name = strings.TrimSuffix(name, "/trigger")
		h.handleTrigger(w, r, name)
	case r.Method == http.MethodPut && strings.Contains(trimmed, "/schedule"):
		name := h.extractTaskName(trimmed, "/scheduler/tasks/")
		name = strings.TrimSuffix(name, "/schedule")
		h.handleReschedule(w, r, name)
	default:
		Error(w, http.StatusNotFound, 40400, "接口不存在")
	}
}

func (h *SchedulerHandler) handleList(w http.ResponseWriter, _ *http.Request) {
	tasks := h.registry.List()
	Success(w, map[string]interface{}{
		"items": tasks,
		"total": len(tasks),
	})
}

func (h *SchedulerHandler) handleGet(w http.ResponseWriter, _ *http.Request, name string) {
	if name == "" {
		Error(w, http.StatusBadRequest, 40001, "任务名称不能为空")
		return
	}
	task, err := h.registry.Get(name)
	if err != nil {
		Error(w, http.StatusNotFound, 40400, err.Error())
		return
	}
	Success(w, task)
}

func (h *SchedulerHandler) handlePause(w http.ResponseWriter, _ *http.Request, name string) {
	if name == "" {
		Error(w, http.StatusBadRequest, 40001, "任务名称不能为空")
		return
	}
	if err := h.registry.Pause(name); err != nil {
		Error(w, http.StatusNotFound, 40400, err.Error())
		return
	}
	Success(w, map[string]interface{}{"paused": true, "name": name})
}

func (h *SchedulerHandler) handleResume(w http.ResponseWriter, _ *http.Request, name string) {
	if name == "" {
		Error(w, http.StatusBadRequest, 40001, "任务名称不能为空")
		return
	}
	if err := h.registry.Resume(name); err != nil {
		Error(w, http.StatusNotFound, 40400, err.Error())
		return
	}
	Success(w, map[string]interface{}{"resumed": true, "name": name})
}

func (h *SchedulerHandler) handleTrigger(w http.ResponseWriter, r *http.Request, name string) {
	if name == "" {
		Error(w, http.StatusBadRequest, 40001, "任务名称不能为空")
		return
	}
	if err := h.registry.Trigger(r.Context(), name); err != nil {
		Error(w, http.StatusBadRequest, 40002, err.Error())
		return
	}
	Success(w, map[string]interface{}{"triggered": true, "name": name})
}

func (h *SchedulerHandler) handleReschedule(w http.ResponseWriter, r *http.Request, name string) {
	if name == "" {
		Error(w, http.StatusBadRequest, 40001, "任务名称不能为空")
		return
	}
	var req struct {
		Schedule string `json:"schedule"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.Schedule == "" {
		Error(w, http.StatusBadRequest, 40001, "cron 表达式不能为空")
		return
	}
	if err := h.registry.Reschedule(name, req.Schedule); err != nil {
		code := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			code = http.StatusNotFound
		}
		Error(w, code, 40003, err.Error())
		return
	}
	if h.db != nil {
		if err := h.db.WithContext(r.Context()).Save(&model.SchedulerTaskOverride{
			Name:     name,
			Schedule: req.Schedule,
		}).Error; err != nil {
			h.logger.Warn("failed to persist schedule override", zap.String("name", name), zap.Error(err))
		}
	}
	Success(w, map[string]interface{}{"rescheduled": true, "name": name, "schedule": req.Schedule})
}

func ApplySchedulerOverrides(ctx context.Context, db *gorm.DB, registry *scheduler.Registry, logger *zap.Logger) {
	var overrides []model.SchedulerTaskOverride
	if err := db.WithContext(ctx).Find(&overrides).Error; err != nil {
		logger.Warn("failed to load scheduler overrides", zap.Error(err))
		return
	}
	for _, o := range overrides {
		if err := registry.Reschedule(o.Name, o.Schedule); err != nil {
			logger.Warn("failed to apply schedule override",
				zap.String("name", o.Name),
				zap.String("schedule", o.Schedule),
				zap.Error(err))
			continue
		}
		logger.Info("applied schedule override",
			zap.String("name", o.Name),
			zap.String("schedule", o.Schedule))
	}
}

func (h *SchedulerHandler) extractTaskName(trimmed, prefix string) string {
	idx := strings.Index(trimmed, prefix)
	if idx < 0 {
		return ""
	}
	return trimmed[idx+len(prefix):]
}
