package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ranfish/pt-forward/internal/download"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type DownloadHandler struct {
	repo   *download.Repository
	db     *gorm.DB
	logger *zap.Logger
}

func NewDownloadHandler(db *gorm.DB, logger *zap.Logger) *DownloadHandler {
	return &DownloadHandler{
		repo:   download.NewRepository(db),
		db:     db,
		logger: logger,
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

var _ = fmt.Sprintf
