package api

import (
	"bufio"
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
	version      string
	db           *gorm.DB
	clientMgr    *client.Manager
	logger       *zap.Logger
	seedingEngine torrentCounter
}

func NewSystemHandler(version string, db *gorm.DB, clientMgr *client.Manager, logger *zap.Logger) *SystemHandler {
	return &SystemHandler{version: version, db: db, clientMgr: clientMgr, logger: logger}
}

func (h *SystemHandler) SetSeedingEngine(engine torrentCounter) {
	h.seedingEngine = engine
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
		switch r.Method {
		case http.MethodGet:
			h.handleListLogs(w, r)
		case http.MethodDelete:
			h.handleClearLogs(w, r)
		default:
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
	dbOK := true
	sqlDB, err := h.db.DB()
	if err != nil {
		dbOK = false
	} else if err := sqlDB.Ping(); err != nil {
		dbOK = false
	}

	status := "healthy"
	if !dbOK {
		status = "unhealthy"
	}

	Success(w, map[string]interface{}{
		"status":  status,
		"version": h.version,
		"database": map[string]interface{}{
			"ok": dbOK,
		},
	})
}

func (h *SystemHandler) handleInfo(w http.ResponseWriter, _ *http.Request) {
	connectedClients := 0
	if h.clientMgr != nil {
		connectedClients = h.clientMgr.ConnectedCount()
	}

	var rssCount int64
	if err := h.db.Model(&model.RSSSubscription{}).Where("enabled = ?", true).Count(&rssCount).Error; err != nil {
		h.logger.Warn("query rss subscription count failed", zap.Error(err))
	}

	seedingActive := 0
	if h.seedingEngine != nil {
		for _, rc := range h.seedingEngine.GetRealTorrentCounts() {
			seedingActive += rc.Seeding
		}
	}

	uptime := time.Since(startTime)

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	Success(w, map[string]interface{}{
		"version":          h.version,
		"uptime":           uptime.String(),
		"goVersion":        runtime.Version(),
		"os":               runtime.GOOS,
		"arch":             runtime.GOARCH,
		"cpuCount":         runtime.NumCPU(),
		"goroutines":       runtime.NumGoroutine(),
		"memAlloc":         memStats.Alloc,
		"heapAlloc":        memStats.HeapAlloc,
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
	if level != "" {
		validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true, "dpanic": true, "panic": true, "fatal": true}
		if !validLevels[level] {
			level = ""
		}
	}

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
	for i := len(matches) - 1; i >= 0 && count < limit; i++ {
		f, err := os.Open(matches[i])
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		var allLines []string
		for scanner.Scan() {
			allLines = append(allLines, scanner.Text())
		}
		f.Close()

		for j := len(allLines) - 1; j >= 0 && count < limit; j-- {
			line := strings.TrimSpace(allLines[j])
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
	deleted := 0
	for _, f := range matches {
		if err := os.Truncate(f, 0); err != nil {
			h.logger.Warn("truncate log failed", zap.String("file", f), zap.Error(err))
		} else {
			deleted++
		}
	}
	Success(w, map[string]interface{}{"deleted": deleted})
}

var startTime = time.Now()
