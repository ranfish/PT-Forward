package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ranfish/pt-forward/internal/client"
	"github.com/ranfish/pt-forward/internal/download"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type DownloadHandler struct {
	repo      *download.Repository
	db        *gorm.DB
	clientMgr *client.Manager
	logger    *zap.Logger
}

func NewDownloadHandler(db *gorm.DB, clientMgr *client.Manager, logger *zap.Logger) *DownloadHandler {
	return &DownloadHandler{
		repo:      download.NewRepository(db),
		db:        db,
		clientMgr: clientMgr,
		logger:    logger,
	}
}

func (h *DownloadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/api/v1/downloads" || path == "/api/v1/downloads/" {
		switch r.Method {
		case http.MethodGet:
			h.handleList(w, r)
		default:
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	if path == "/api/v1/downloads/bulk-action" || path == "/api/v1/downloads/bulk-action/" {
		if r.Method == http.MethodPost {
			h.handleBulkAction(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	idStr := ""
	for _, seg := range splitPath(path) {
		if _, err := strconv.ParseUint(seg, 10, 32); err == nil {
			idStr = seg
			break
		}
	}
	if idStr == "" {
		Error(w, http.StatusBadRequest, 40001, "无效的路径")
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的 ID")
		return
	}

	rest := path[len("/api/v1/downloads/")+len(idStr):]
	switch {
	case rest == "" || rest == "/":
		switch r.Method {
		case http.MethodGet:
			h.handleGet(w, r, uint(id))
		case http.MethodDelete:
			h.handleDelete(w, r, uint(id))
		default:
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
	default:
		Error(w, http.StatusNotFound, 40400, "路径不存在")
	}
}

func (h *DownloadHandler) handleList(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size <= 0 || size > 200 {
		size = 20
	}
	clientID := r.URL.Query().Get("client_id")
	status := r.URL.Query().Get("status")

	tasks, total, err := h.repo.List(r.Context(), page, size, clientID, status)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询失败")
		return
	}
	Success(w, map[string]interface{}{
		"items": tasks,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

func (h *DownloadHandler) handleGet(w http.ResponseWriter, r *http.Request, id uint) {
	task, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, 40400, "任务不存在")
		return
	}
	Success(w, task)
}

type deleteTaskRequest struct {
	DeleteCompanions bool `json:"delete_companions"`
}

func (h *DownloadHandler) handleDelete(w http.ResponseWriter, r *http.Request, id uint) {
	var req deleteTaskRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	task, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, 40400, "任务不存在")
		return
	}

	if task.Status == model.DownloadStatusDeleted {
		Error(w, http.StatusBadRequest, 40001, "任务已删除")
		return
	}

	action := model.DeleteActionWithCompanions
	if !req.DeleteCompanions {
		action = model.DeleteActionSiteOnly
	}

	if err := h.repo.MarkDeleted(r.Context(), id, action); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "更新任务状态失败")
		return
	}

	Success(w, map[string]interface{}{"id": id, "status": "deleted"})
}

func splitPath(path string) []string {
	var segs []string
	current := ""
	for _, c := range path {
		if c == '/' {
			if current != "" {
				segs = append(segs, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		segs = append(segs, current)
	}
	return segs
}

type bulkActionRequest struct {
	IDs    []uint `json:"ids"`
	Action string `json:"action"` // pause|resume|recheck|delete
	DeleteCompanions *bool `json:"delete_companions,omitempty"`
}

func (h *DownloadHandler) handleBulkAction(w http.ResponseWriter, r *http.Request) {
	var req bulkActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if len(req.IDs) == 0 {
		Error(w, http.StatusBadRequest, 40001, "ids 为必填项")
		return
	}

	succeeded := 0
	failed := 0
	for _, id := range req.IDs {
		task, err := h.repo.GetByID(r.Context(), id)
		if err != nil || task == nil {
			failed++
			continue
		}
		if task.Status == model.DownloadStatusDeleted && req.Action != "delete" {
			continue
		}

		switch req.Action {
		case "pause":
			err = h.doPause(r.Context(), task)
		case "resume":
			err = h.doResume(r.Context(), task)
		case "recheck":
			err = h.doRecheck(r.Context(), task)
		case "delete":
			delComp := true
			if req.DeleteCompanions != nil {
				delComp = *req.DeleteCompanions
			}
			action := model.DeleteActionWithCompanions
			if !delComp {
				action = model.DeleteActionSiteOnly
			}
			err = h.repo.MarkDeleted(r.Context(), id, action)
		default:
			Error(w, http.StatusBadRequest, 40001, "不支持的操作: "+req.Action)
			return
		}

		if err != nil {
			h.logger.Warn("bulk action failed",
				zap.Uint("id", id), zap.String("action", req.Action), zap.Error(err))
			failed++
		} else {
			succeeded++
		}
	}

	Success(w, map[string]interface{}{
		"succeeded": succeeded,
		"failed":    failed,
	})
}

func (h *DownloadHandler) doPause(ctx context.Context, task *model.DownloadTask) error {
	c, err := h.clientMgr.Get(task.ClientID)
	if err != nil {
		return err
	}
	return c.PauseTorrent(ctx, task.InfoHash)
}

func (h *DownloadHandler) doResume(ctx context.Context, task *model.DownloadTask) error {
	c, err := h.clientMgr.Get(task.ClientID)
	if err != nil {
		return err
	}
	return c.ResumeTorrent(ctx, task.InfoHash)
}

func (h *DownloadHandler) doRecheck(ctx context.Context, task *model.DownloadTask) error {
	c, err := h.clientMgr.Get(task.ClientID)
	if err != nil {
		return err
	}
	return c.Recheck(ctx, task.InfoHash)
}

var _ = fmt.Sprintf
