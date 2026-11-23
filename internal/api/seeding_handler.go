package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/seeding"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type SeedingHandler struct {
	db     *gorm.DB
	logger *zap.Logger
	engine *seeding.Engine
}

func NewSeedingHandler(db *gorm.DB, logger *zap.Logger, engine *seeding.Engine) *SeedingHandler {
	return &SeedingHandler{db: db, logger: logger, engine: engine}
}

func (h *SeedingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimRight(path, "/")

	switch {
	case trimmed == "/api/v1/seeding/configs" || trimmed == "/api/v1/seeding/configs/":
		switch r.Method {
		case http.MethodGet:
			h.handleListConfigs(w, r)
		case http.MethodPost:
			h.handleCreateConfig(w, r)
		default:
			methodNotAllowed(w)
		}
		return

	case trimmed == "/api/v1/seeding/records" || trimmed == "/api/v1/seeding/records/":
		if r.Method == http.MethodGet {
			h.handleListRecords(w, r)
		} else {
			methodNotAllowed(w)
		}
		return

	case trimmed == "/api/v1/seeding/stats" || trimmed == "/api/v1/seeding/stats/":
		h.handleStats(w, r)
		return

	case trimmed == "/api/v1/seeding/scoring-dryrun" || trimmed == "/api/v1/seeding/scoring-dryrun/":
		if r.Method == http.MethodPost {
			h.handleScoringDryrun(w, r)
		} else {
			methodNotAllowed(w)
		}
		return

	case trimmed == "/api/v1/seeding/status" || trimmed == "/api/v1/seeding/status/":
		h.handleEngineStatus(w, r)
		return

	case trimmed == "/api/v1/seeding/free-wait-queue" || trimmed == "/api/v1/seeding/free-wait-queue/":
		h.handleFreeWaitQueue(w, r)
		return

	case trimmed == "/api/v1/seeding/torrents" || trimmed == "/api/v1/seeding/torrents/":
		h.handleListTorrents(w, r)
		return

	case trimmed == "/api/v1/seeding/clients" || trimmed == "/api/v1/seeding/clients/":
		h.handleListClients(w, r)
		return

	case trimmed == "/api/v1/seeding/scoring-config" || trimmed == "/api/v1/seeding/scoring-config/":
		h.handleScoringConfig(w, r)
		return

	case trimmed == "/api/v1/seeding/scoring-logs" || trimmed == "/api/v1/seeding/scoring-logs/":
		h.handleScoringLogs(w, r)
		return

	case trimmed == "/api/v1/seeding/unregistered-keywords" || trimmed == "/api/v1/seeding/unregistered-keywords/":
		h.handleUnregisteredKeywords(w, r)
		return

	case trimmed == "/api/v1/seeding/history" || trimmed == "/api/v1/seeding/history/":
		if r.Method == http.MethodGet {
			h.handleListHistory(w, r)
		} else {
			methodNotAllowed(w)
		}
		return

	case strings.HasPrefix(trimmed, "/api/v1/seeding/stats/"):
		h.handleStatsSubroute(w, r, trimmed)
		return
	}

	if strings.Contains(trimmed, "/seeding/records/") {
		remaining := extractLastSegment(trimmed, "/api/v1/seeding/records/")
		parts := strings.SplitN(remaining, "/", 2)
		_, err := strconv.ParseUint(parts[0], 10, 64)
		if err != nil {
			Error(w, http.StatusBadRequest, 40001, "无效的记录 ID")
			return
		}
		if len(parts) == 2 && parts[1] == "resume" && r.Method == http.MethodPost {
			h.handleResumeRecord(w, r, parts[0])
			return
		}
		if len(parts) == 2 && parts[1] == "pause" && r.Method == http.MethodPost {
			h.handlePauseRecord(w, r, parts[0])
			return
		}
		Error(w, http.StatusNotFound, 40400, "路径不存在")
		return
	}

	if strings.Contains(trimmed, "/seeding/configs/") {
		remaining := extractLastSegment(trimmed, "/api/v1/seeding/configs/")
		if remaining == "" {
			h.handleListConfigs(w, r)
			return
		}
		id, err := strconv.ParseUint(remaining, 10, 64)
		if err != nil {
			Error(w, http.StatusBadRequest, 40001, "无效的配置 ID")
			return
		}
		switch r.Method {
		case http.MethodGet:
			h.handleGetConfig(w, r, uint(id))
		case http.MethodPut:
			h.handleUpdateConfig(w, r, uint(id))
		case http.MethodDelete:
			h.handleDeleteConfig(w, r, uint(id))
		default:
			methodNotAllowed(w)
		}
		return
	}

	if strings.Contains(trimmed, "/seeding/torrents/") {
		remaining := extractLastSegment(trimmed, "/api/v1/seeding/torrents/")
		parts := strings.SplitN(remaining, "/", 2)
		_, err := strconv.ParseUint(parts[0], 10, 64)
		if err != nil {
			Error(w, http.StatusBadRequest, 40001, "无效的种子 ID")
			return
		}
		if len(parts) == 2 && parts[1] == "resume" && r.Method == http.MethodPost {
			h.handleResumeRecord(w, r, parts[0])
			return
		}
		Error(w, http.StatusNotFound, 40400, "路径不存在")
		return
	}

	if strings.Contains(trimmed, "/seeding/clients/") {
		remaining := extractLastSegment(trimmed, "/api/v1/seeding/clients/")
		parts := strings.SplitN(remaining, "/", 2)
		if len(parts) == 2 && parts[1] == "trigger" && r.Method == http.MethodPost {
			uidNum, _ := strconv.ParseUint(parts[0], 10, 64)
			h.handleTriggerClient(w, r, uint(uidNum))
			return
		}
		Error(w, http.StatusNotFound, 40400, "路径不存在")
		return
	}

	if strings.Contains(trimmed, "/seeding/scoring-config/") {
		remaining := extractLastSegment(trimmed, "/api/v1/seeding/scoring-config/")
		switch r.Method {
		case http.MethodGet:
			h.handleScoringConfigByID(w, r, remaining)
		case http.MethodPut:
			h.handleScoringConfigByID(w, r, remaining)
		default:
			methodNotAllowed(w)
		}
		return
	}

	if strings.Contains(trimmed, "/seeding/scoring-logs/") {
		remaining := extractLastSegment(trimmed, "/api/v1/seeding/scoring-logs/")
		if strings.HasPrefix(remaining, "cycles/") {
			h.handleScoringCycle(w, r, strings.TrimPrefix(remaining, "cycles/"))
			return
		}
		Error(w, http.StatusNotFound, 40400, "路径不存在")
		return
	}

	if strings.Contains(trimmed, "/seeding/dryrun/") {
		remaining := extractLastSegment(trimmed, "/api/v1/seeding/dryrun/")
		if remaining != "" && r.Method == http.MethodPost {
			h.handleDryrunBySub(w, r, remaining)
			return
		}
		Error(w, http.StatusNotFound, 40400, "路径不存在")
		return
	}

	if trimmed == "/api/v1/seeding/dryrun" {
		if r.Method == http.MethodPost {
			h.handleDryrunAll(w, r)
			return
		}
		methodNotAllowed(w)
		return
	}

	Error(w, http.StatusNotFound, 40400, "接口不存在")
}

func (h *SeedingHandler) handleStatsSubroute(w http.ResponseWriter, r *http.Request, trimmed string) {
	remaining := strings.TrimPrefix(trimmed, "/api/v1/seeding/stats/")
	remaining = strings.TrimRight(remaining, "/")

	switch {
	case remaining == "overview":
		h.handleStatsOverview(w, r)
	case remaining == "by-site":
		h.handleStatsBySite(w, r)
	case remaining == "torrents":
		h.handleStatsTorrents(w, r)
	case strings.HasPrefix(remaining, "by-site/"):
		siteName := strings.TrimPrefix(remaining, "by-site/")
		siteName = strings.TrimRight(siteName, "/")
		if strings.HasSuffix(siteName, "/trend") {
			site := strings.TrimSuffix(siteName, "/trend")
			h.handleSiteTrend(w, r, site)
		} else {
			Error(w, http.StatusNotFound, 40400, "路径不存在")
		}
	case strings.HasPrefix(remaining, "downloader/"):
		clientIDPath := strings.TrimRight(strings.TrimPrefix(remaining, "downloader/"), "/")
		if strings.HasSuffix(clientIDPath, "/speed-trend") {
			idStr := strings.TrimSuffix(clientIDPath, "/speed-trend")
			idNum, _ := strconv.ParseUint(idStr, 10, 64)
			h.handleDownloaderSpeedTrend(w, r, uint(idNum))
		} else {
			Error(w, http.StatusNotFound, 40400, "路径不存在")
		}
	default:
		Error(w, http.StatusNotFound, 40400, "路径不存在")
	}
}
