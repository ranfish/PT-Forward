package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

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
		case http.MethodPost:
			h.handleAdd(w, r)
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

	if strings.HasPrefix(path, "/api/v1/downloads/configs") {
		h.handleConfigs(w, r, strings.TrimPrefix(path, "/api/v1/downloads/configs"))
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
	case rest == "/retry-transfer" || rest == "/retry-transfer/":
		if r.Method == http.MethodPost {
			h.handleRetryTransfer(w, r, uint(id))
		} else {
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

type addTaskRequest struct {
	ClientID string `json:"client_id"`
	URL      string `json:"url"`
	Category string `json:"category"`
	Paused   bool   `json:"paused"`
}

func (h *DownloadHandler) handleAdd(w http.ResponseWriter, r *http.Request) {
	var torrentData []byte
	var clientID, category string
	var paused bool

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			Error(w, http.StatusBadRequest, 40001, "解析表单失败")
			return
		}
		file, _, err := r.FormFile("torrent")
		if err != nil {
			Error(w, http.StatusBadRequest, 40001, "请上传 .torrent 文件")
			return
		}
		defer file.Close()
		torrentData, err = io.ReadAll(file)
		if err != nil {
			Error(w, http.StatusBadRequest, 40001, "读取文件失败")
			return
		}
		clientID = r.FormValue("client_id")
		category = r.FormValue("category")
		paused = r.FormValue("paused") == "true"
	} else {
		var req addTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, 40001, "请求格式错误")
			return
		}
		clientID = req.ClientID
		category = req.Category
		paused = req.Paused
		if req.URL != "" {
			httpClient := &http.Client{Timeout: 30 * time.Second}
			resp, err := httpClient.Get(req.URL)
			if err != nil {
				Error(w, http.StatusBadRequest, 40001, "下载 .torrent 失败: "+err.Error())
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				Error(w, http.StatusBadRequest, 40001, fmt.Sprintf("下载 .torrent 失败: HTTP %d", resp.StatusCode))
				return
			}
			torrentData, err = io.ReadAll(resp.Body)
			if err != nil {
				Error(w, http.StatusBadRequest, 40001, "读取 .torrent 数据失败")
				return
			}
		}
	}

	if clientID == "" {
		Error(w, http.StatusBadRequest, 40001, "client_id 为必填项")
		return
	}
	if len(torrentData) == 0 {
		Error(w, http.StatusBadRequest, 40001, "请提供 .torrent 文件或下载链接")
		return
	}

	c, err := h.clientMgr.Get(clientID)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "下载器不可用: "+err.Error())
		return
	}

	opts := model.AddTorrentOptions{
		Category: category,
		Paused:   paused,
	}
	result, err := c.AddFromFile(r.Context(), torrentData, opts)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "添加种子失败: "+err.Error())
		return
	}

	task := &model.DownloadTask{
		Source:      model.DownloadSourceManual,
		ClientID:    clientID,
		InfoHash:    result.InfoHash,
		TorrentName: result.Name,
		Category:    category,
		Status:      model.DownloadStatusDownloading,
	}
	if paused {
		task.Status = model.DownloadStatusPaused
	}
	if err := h.repo.Create(r.Context(), task); err != nil {
		h.logger.Warn("failed to create download task", zap.Error(err))
	}

	Success(w, task)
}

func (h *DownloadHandler) handleRetryTransfer(w http.ResponseWriter, r *http.Request, id uint) {
	task, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, 40400, "任务不存在")
		return
	}
	if task.TransferStatus != model.TransferStatusFailed && task.TransferStatus != model.TransferStatusPartial {
		Error(w, http.StatusBadRequest, 40001, "仅转移失败或部分转移的任务可重试")
		return
	}

	h.repo.UpdateTransfer(r.Context(), id, model.TransferStatusPending, "", "")
	Success(w, map[string]interface{}{"id": id, "transfer_status": model.TransferStatusPending})
}

var _ = fmt.Sprintf

func (h *DownloadHandler) handleConfigs(w http.ResponseWriter, r *http.Request, rest string) {
	rest = strings.TrimPrefix(rest, "/")
	switch {
	case rest == "" && r.Method == http.MethodGet:
		var configs []model.DownloadClientConfig
		h.db.WithContext(r.Context()).Order("client_id ASC").Find(&configs)
		Success(w, configs)
	case rest == "" && r.Method == http.MethodPost:
		var req model.DownloadClientConfig
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, 40001, "请求格式错误")
			return
		}
		if req.ClientID == "" {
			Error(w, http.StatusBadRequest, 40001, "client_id 为必填项")
			return
		}
		if req.AutoDeleteCron == "" {
			req.AutoDeleteCron = "*/30 * * * *"
		}
		if req.MainDataCron == "" {
			req.MainDataCron = "*/20 * * * *"
		}
		if req.Scope == "" {
			req.Scope = "managed"
		}
		if err := h.db.WithContext(r.Context()).Create(&req).Error; err != nil {
			Error(w, http.StatusInternalServerError, 50000, "创建配置失败")
			return
		}
		Success(w, req)
	default:
		idStr := rest
		if idx := strings.Index(idStr, "/"); idx >= 0 {
			idStr = idStr[:idx]
		}
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			Error(w, http.StatusBadRequest, 40001, "无效的 ID")
			return
		}
		switch r.Method {
		case http.MethodPut:
			var req map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				Error(w, http.StatusBadRequest, 40001, "请求格式错误")
				return
			}
			delete(req, "id")
			delete(req, "created_at")
			req["updated_at"] = time.Now()
			if err := h.db.WithContext(r.Context()).Model(&model.DownloadClientConfig{}).Where("id = ?", id).Updates(req).Error; err != nil {
				Error(w, http.StatusInternalServerError, 50000, "更新配置失败")
				return
			}
			var updated model.DownloadClientConfig
			h.db.WithContext(r.Context()).First(&updated, id)
			Success(w, updated)
		case http.MethodDelete:
			if err := h.db.WithContext(r.Context()).Delete(&model.DownloadClientConfig{}, id).Error; err != nil {
				Error(w, http.StatusInternalServerError, 50000, "删除配置失败")
				return
			}
			Success(w, map[string]interface{}{"id": id})
		default:
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
	}
}
