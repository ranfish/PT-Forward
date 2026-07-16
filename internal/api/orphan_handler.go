package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ranfish/pt-forward/internal/orphan"
	"go.uber.org/zap"
)

type OrphanHandler struct {
	scanner  *orphan.Scanner
	recovery *orphan.Recovery
	logger   *zap.Logger

	mu            sync.RWMutex
	lastResults   []orphan.Entry
	scannedAt     time.Time
	recoverStore  sync.Map
	recoverSeq    atomic.Int64
}

func NewOrphanHandler(scanner *orphan.Scanner, recovery *orphan.Recovery, logger *zap.Logger) *OrphanHandler {
	return &OrphanHandler{
		scanner:  scanner,
		recovery: recovery,
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
		result := h.recovery.Recover(ctx, target)
		h.recoverStore.Store(taskID, result)
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
