package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/orphan"
	"github.com/ranfish/pt-forward/internal/setting"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type OrphanHandler struct {
	scanner  *orphan.Scanner
	recovery *orphan.Recovery
	logger   *zap.Logger
	db       *gorm.DB

	mu            sync.RWMutex
	lastResults   []orphan.Entry
	scannedAt     time.Time
	recoverStore  sync.Map
	recoverSeq    atomic.Int64
}

func NewOrphanHandler(scanner *orphan.Scanner, recovery *orphan.Recovery, db *gorm.DB, logger *zap.Logger) *OrphanHandler {
	return &OrphanHandler{
		scanner:  scanner,
		recovery: recovery,
		db:       db,
		logger:   logger,
	}
}

func (h *OrphanHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(r.URL.Path, "/")
	switch {
	case strings.HasSuffix(path, "/orphans/scan") && r.Method == http.MethodPost:
		h.handleScan(w, r)
	case strings.HasSuffix(path, "/orphans/recover") && r.Method == http.MethodPost:
		h.handleRecover(w, r)
	case strings.Contains(path, "/orphans/recover/") && r.Method == http.MethodGet:
		h.handlePollRecover(w, r)
	case strings.HasSuffix(path, "/orphans/ignore") && r.Method == http.MethodPost:
		h.handleIgnore(w, r)
	case strings.HasSuffix(path, "/orphans/delete") && r.Method == http.MethodPost:
		h.handleDelete(w, r)
	case strings.HasSuffix(path, "/orphans/ignored") && r.Method == http.MethodGet:
		h.handleListIgnored(w, r)
	case strings.HasSuffix(path, "/orphans/ignored") && r.Method == http.MethodDelete:
		h.handleUnignore(w, r)
	case strings.HasSuffix(path, "/orphans/scan-configs") && r.Method == http.MethodGet:
		h.handleListScanConfigs(w, r)
	case strings.HasSuffix(path, "/orphans/scan-configs") && r.Method == http.MethodPost:
		h.handleAddScanConfig(w, r)
	case strings.HasSuffix(path, "/orphans/scan-configs") && r.Method == http.MethodDelete:
		h.handleDeleteScanConfig(w, r)
	case strings.HasSuffix(path, "/orphans") && r.Method == http.MethodGet:
		h.handleList(w, r)
	default:
		Error(w, http.StatusNotFound, 40400, "接口不存在")
	}
}

func (h *OrphanHandler) handleScan(w http.ResponseWriter, r *http.Request) {
	if h.scanner == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "scanner not configured")
		return
	}

	var ignoredPaths []string
	if h.db != nil {
		var settings []setting.Setting
		h.db.Where("key = ?", "orphan_ignored_path").Find(&settings)
		for _, s := range settings {
			ignoredPaths = append(ignoredPaths, s.Value)
		}
	}
	h.scanner.SetIgnoredPaths(ignoredPaths)

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	orphans, err := h.scanner.Scan(ctx)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50001, "扫描失败: "+err.Error())
		return
	}

	h.mu.Lock()
	h.lastResults = orphans
	h.scannedAt = time.Now()
	h.mu.Unlock()

	Success(w, map[string]interface{}{
		"orphans":   orphans,
		"count":     len(orphans),
		"scanned_at": h.scannedAt,
	})
}

func (h *OrphanHandler) handleList(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	Success(w, map[string]interface{}{
		"orphans":   h.lastResults,
		"count":     len(h.lastResults),
		"scanned_at": h.scannedAt,
	})
}

func (h *OrphanHandler) handleRecover(w http.ResponseWriter, r *http.Request) {
	if h.recovery == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "recovery not configured")
		return
	}

	var req struct {
		Path     string `json:"path"`
		ClientID string `json:"client_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.Path == "" {
		Error(w, http.StatusBadRequest, 40001, "path 必填")
		return
	}

	h.mu.RLock()
	var target *orphan.Entry
	for i := range h.lastResults {
		if h.lastResults[i].Path == req.Path {
			target = &h.lastResults[i]
			break
		}
	}
	h.mu.RUnlock()

	if target == nil {
		Error(w, http.StatusNotFound, 40400, "未找到该孤儿记录，请先扫描")
		return
	}

	taskID := h.recoverSeq.Add(1)
	h.recoverStore.Store(taskID, &orphan.RecoverResult{Orphan: target, Message: "searching..."})

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		defer cancel()
	result := h.recovery.Recover(ctx, target, req.ClientID)
			h.recoverStore.Store(taskID, result)
			// 延迟清理任务结果（前端有 50×3s=150s 轮询窗口）
			time.AfterFunc(10*time.Minute, func() { h.recoverStore.Delete(taskID) })
		if result.Found {
			h.mu.Lock()
			var updated []orphan.Entry
			for _, e := range h.lastResults {
				if e.Path != target.Path {
					updated = append(updated, e)
				}
			}
			h.lastResults = updated
			h.mu.Unlock()
		}
	}()

	Success(w, map[string]interface{}{"task_id": taskID})
}

func (h *OrphanHandler) handlePollRecover(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimRight(r.URL.Path, "/"), "/")
	taskIDStr := parts[len(parts)-1]
	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的 task_id")
		return
	}

	val, ok := h.recoverStore.Load(taskID)
	if !ok {
		Error(w, http.StatusNotFound, 40400, "恢复任务不存在")
		return
	}
	result := val.(*orphan.RecoverResult)
	Success(w, result)
}

func (h *OrphanHandler) handleIgnore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.Path == "" {
		Error(w, http.StatusBadRequest, 40001, "path 必填")
		return
	}

	h.db.Where("key = ? AND value = ?", "orphan_ignored_path", req.Path).
		FirstOrCreate(&setting.Setting{Key: "orphan_ignored_path", Value: req.Path})

	h.mu.Lock()
	var updated []orphan.Entry
	for _, e := range h.lastResults {
		if e.Path != req.Path {
			updated = append(updated, e)
		}
	}
	h.lastResults = updated
	h.mu.Unlock()

	Success(w, map[string]interface{}{"ignored": true})
}

func (h *OrphanHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.Path == "" {
		Error(w, http.StatusBadRequest, 40001, "path 必填")
		return
	}

	// 安全校验：路径必须在最近扫描结果中
	h.mu.RLock()
	found := false
	for _, e := range h.lastResults {
		if e.Path == req.Path {
			found = true
			break
		}
	}
	h.mu.RUnlock()
	if !found {
		Error(w, http.StatusBadRequest, 40001, "该路径不在扫描结果中，拒绝删除")
		return
	}

	info, err := os.Stat(req.Path)
	if err != nil {
		Error(w, http.StatusNotFound, 40400, "路径不存在")
		return
	}

	var deleted int64
	if info.IsDir() {
		err = os.RemoveAll(req.Path)
	} else {
		err = os.Remove(req.Path)
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, 50001, "删除失败: "+err.Error())
		return
	}
	deleted = 1

	h.mu.Lock()
	var updated []orphan.Entry
	for _, e := range h.lastResults {
		if e.Path != req.Path {
			updated = append(updated, e)
		}
	}
	h.lastResults = updated
	h.mu.Unlock()

	h.logger.Info("orphan file deleted by user", zap.String("path", req.Path))
	Success(w, map[string]interface{}{"deleted": deleted})
}

func (h *OrphanHandler) handleListIgnored(w http.ResponseWriter, r *http.Request) {
	var settings []setting.Setting
	h.db.Where("key = ?", "orphan_ignored_path").Find(&settings)
	var paths []string
	for _, s := range settings {
		paths = append(paths, s.Value)
	}
	if paths == nil {
		paths = []string{}
	}
	Success(w, map[string]interface{}{"ignored": paths, "count": len(paths)})
}

func (h *OrphanHandler) handleUnignore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.Path == "" {
		Error(w, http.StatusBadRequest, 40001, "path 必填")
		return
	}

	h.db.Where("key = ? AND value = ?", "orphan_ignored_path", req.Path).Delete(&setting.Setting{})
	Success(w, map[string]interface{}{"unignored": true})
}

func (h *OrphanHandler) handleListScanConfigs(w http.ResponseWriter, r *http.Request) {
	var configs []model.OrphanScanConfig
	h.db.Find(&configs)
	Success(w, configs)
}

func (h *OrphanHandler) handleAddScanConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID string `json:"client_id"`
		ScanPath string `json:"scan_path"`
		Enabled  bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.ClientID == "" || req.ScanPath == "" {
		Error(w, http.StatusBadRequest, 40001, "client_id 和 scan_path 必填")
		return
	}

	cfg := model.OrphanScanConfig{
		ClientID: req.ClientID,
		ScanPath: req.ScanPath,
		Enabled:  req.Enabled,
	}
	if err := h.db.Where("client_id = ? AND scan_path = ?", req.ClientID, req.ScanPath).
		FirstOrCreate(&cfg).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50001, "保存失败")
		return
	}
	if !cfg.Enabled && req.Enabled {
		h.db.Model(&cfg).Update("enabled", true)
	}
	Success(w, cfg)
}

func (h *OrphanHandler) handleDeleteScanConfig(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		Error(w, http.StatusBadRequest, 40001, "id 必填")
		return
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的 id")
		return
	}
	h.db.Delete(&model.OrphanScanConfig{}, id)
	Success(w, map[string]interface{}{"deleted": true})
}
