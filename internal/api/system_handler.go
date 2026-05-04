package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/client"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type SystemHandler struct {
	version   string
	db        *gorm.DB
	clientMgr *client.Manager
	logger    *zap.Logger
}

func NewSystemHandler(version string, db *gorm.DB, clientMgr *client.Manager, logger *zap.Logger) *SystemHandler {
	return &SystemHandler{version: version, db: db, clientMgr: clientMgr, logger: logger}
}

func (h *SystemHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimRight(path, "/")

	switch {
	case strings.HasSuffix(trimmed, "/system/ping"):
		h.handlePing(w, r)
	case strings.HasSuffix(trimmed, "/system/health"):
		h.handleHealth(w, r)
	case strings.HasSuffix(trimmed, "/system/info"):
		h.handleInfo(w, r)
	case strings.HasSuffix(trimmed, "/system/logs"):
		if r.Method == http.MethodGet {
			h.handleListLogs(w, r)
		} else if r.Method == http.MethodDelete {
			h.handleClearLogs(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
	default:
		Error(w, http.StatusNotFound, 40400, "接口不存在")
	}
}

func (h *SystemHandler) HandlePing(w http.ResponseWriter, r *http.Request) {
	h.handlePing(w, r)
}

func (h *SystemHandler) handlePing(w http.ResponseWriter, _ *http.Request) {
	Success(w, map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
	})
}

func (h *SystemHandler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	dbStatus := "ok"
	dbMsg := "connected"
	sqlDB, err := h.db.DB()
	if err != nil {
		dbStatus = "error"
		dbMsg = err.Error()
	} else if err := sqlDB.Ping(); err != nil {
		dbStatus = "error"
		dbMsg = "ping failed"
	}

	connectedClients := 0
	if h.clientMgr != nil {
		connectedClients = h.clientMgr.ConnectedCount()
	}

	status := "healthy"
	if dbStatus != "ok" {
		status = "unhealthy"
	}

	Success(w, map[string]interface{}{
		"status":  status,
		"version": h.version,
		"uptime":  time.Since(startTime).String(),
		"database": map[string]interface{}{
			"ok":      dbStatus == "ok",
			"message": dbMsg,
		},
		"downloaders": map[string]interface{}{
			"connected": connectedClients,
			"total":     connectedClients,
		},
		"system": map[string]interface{}{
			"goroutines": runtime.NumGoroutine(),
			"memoryMB":   memStats.Alloc / 1024 / 1024,
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
		},
	})
}

func (h *SystemHandler) handleInfo(w http.ResponseWriter, _ *http.Request) {
	connectedClients := 0
	if h.clientMgr != nil {
		connectedClients = h.clientMgr.ConnectedCount()
	}

	var rssCount int64
	h.db.Model(&model.RSSSubscription{}).Where("enabled = ? AND deleted_at = ?", true, time.Time{}).Count(&rssCount)

	var seedingActive int64
	h.db.Model(&model.SeedingTorrentRecord{}).Where("status = ?", "seeding").Count(&seedingActive)

	uptime := time.Since(startTime)

	Success(w, map[string]interface{}{
		"version":          h.version,
		"uptime":           uptime.String(),
		"goVersion":        runtime.Version(),
		"connectedClients": connectedClients,
		"rssSubscriptions": rssCount,
		"seedingActive":    seedingActive,
		"status":           "running",
	})
}

func (h *SystemHandler) handleListLogs(w http.ResponseWriter, r *http.Request) {
	limit := int64(100)
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := parseInt64(v)
		if err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	level := r.URL.Query().Get("level")

	logDir := "logs"
	entries := []map[string]interface{}{}

	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		Success(w, map[string]interface{}{"items": entries, "total": 0})
		return
	}

	matches, _ := filepath.Glob(filepath.Join(logDir, "*.log"))
	if len(matches) == 0 {
		Success(w, map[string]interface{}{"items": entries, "total": 0})
		return
	}

	count := int64(0)
	for i := len(matches) - 1; i >= 0 && count < limit; i-- {
		data, err := os.ReadFile(matches[i])
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for j := len(lines) - 1; j >= 0 && count < limit; j-- {
			line := strings.TrimSpace(lines[j])
			if line == "" {
				continue
			}
			if level != "" && !strings.Contains(line, `"level":"`+level+`"`) {
				continue
			}
			var logEntry map[string]interface{}
			if json.Unmarshal([]byte(line), &logEntry) == nil {
				entries = append(entries, logEntry)
			} else {
				entries = append(entries, map[string]interface{}{"message": line})
			}
			count++
		}
	}

	Success(w, map[string]interface{}{
		"items": entries,
		"total": len(entries),
	})
}

func (h *SystemHandler) handleClearLogs(w http.ResponseWriter, _ *http.Request) {
	logDir := "logs"
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		Success(w, map[string]interface{}{"deleted": 0})
		return
	}

	matches, _ := filepath.Glob(filepath.Join(logDir, "*.log"))
	for _, f := range matches {
		_ = os.Truncate(f, 0)
	}
	Success(w, map[string]interface{}{"deleted": len(matches)})
}

var startTime = time.Now()
