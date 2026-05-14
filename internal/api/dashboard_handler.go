package api

import (
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type DashboardHandler struct {
	db            *gorm.DB
	logger        *zap.Logger
	version       string
	clientChecker clientOnlineChecker
}

type clientOnlineChecker interface {
	ConnectedCount() int
}

func NewDashboardHandler(db *gorm.DB, logger *zap.Logger, version string, checker clientOnlineChecker) *DashboardHandler {
	return &DashboardHandler{db: db, logger: logger, version: version, clientChecker: checker}
}

func (h *DashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimRight(path, "/")

	switch {
	case strings.HasSuffix(trimmed, "/dashboard/overview"):
		h.handleOverview(w, r)
	case strings.HasSuffix(trimmed, "/dashboard/activities"):
		h.handleActivities(w, r)
	case strings.HasSuffix(trimmed, "/dashboard/trends"):
		h.handleTrends(w, r)
	case strings.HasPrefix(trimmed, "/api/v1/torrent-events"):
		h.handleTorrentEvents(w, r, trimmed)
	default:
		Error(w, http.StatusNotFound, 40400, "接口不存在")
	}
}

func (h *DashboardHandler) handleOverview(w http.ResponseWriter, _ *http.Request) {
	var rssCount int64
	h.db.Model(&model.RSSSubscription{}).Where("enabled = ? AND deleted_at = ?", true, time.Time{}).Count(&rssCount)

	var seedingActive int64
	h.db.Model(&model.SeedingTorrentRecord{}).Where("status = ?", "seeding").Count(&seedingActive)

	var seedingPaused int64
	h.db.Model(&model.SeedingTorrentRecord{}).Where("status IN ?", []string{"paused_free_end", "paused_rule"}).Count(&seedingPaused)

	var reseedActive int64
	h.db.Model(&model.ReseedTask{}).Where("enabled = ?", true).Count(&reseedActive)

	var publishPending int64
	h.db.Model(&model.PublishCandidate{}).Where("publish_status = ?", "pending").Count(&publishPending)

	var publishToday int64
	today := time.Now().Truncate(24 * time.Hour)
	h.db.Model(&model.PublishCandidate{}).Where("publish_status = ? AND updated_at >= ?", "done", today).Count(&publishToday)

	var publishTotal int64
	h.db.Model(&model.PublishCandidate{}).Where("publish_status = ?", "done").Count(&publishTotal)

	var reseedToday int64
	h.db.Model(&model.ReseedMatch{}).Where("status = ? AND injected_at >= ?", "injected", today).Count(&reseedToday)

	var reseedTotal int64
	h.db.Model(&model.ReseedMatch{}).Where("status = ?", "injected").Count(&reseedTotal)

	var siteTotal int64
	h.db.Model(&model.Site{}).Count(&siteTotal)

	var siteOnline int64
	h.db.Model(&model.Site{}).Where("enabled = ?", true).Count(&siteOnline)

	var clientConfigs []model.ClientConfig
	if err := h.db.Where("deleted_at = ? AND enabled = ?", time.Time{}, true).Find(&clientConfigs).Error; err != nil {
		h.logger.Warn("dashboard: query client configs failed", zap.Error(err))
	}

	onlineCount := len(clientConfigs)
	if h.clientChecker != nil {
		if count := h.clientChecker.ConnectedCount(); count >= 0 {
			onlineCount = count
		}
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	var downloadingCount int64
	h.db.Model(&model.SeedingTorrentRecord{}).Where("status = ?", "downloading").Count(&downloadingCount)

	var totalSizeResult []struct {
		Total int64
	}
	h.db.Model(&model.SeedingTorrentRecord{}).Select("COALESCE(SUM(size), 0) as total").Scan(&totalSizeResult)
	totalSize := int64(0)
	if len(totalSizeResult) > 0 {
		totalSize = totalSizeResult[0].Total
	}

	Success(w, map[string]interface{}{
		"sites": map[string]interface{}{
			"total":  siteTotal,
			"online": siteOnline,
		},
		"downloaders": map[string]interface{}{
			"total":  len(clientConfigs),
			"online": onlineCount,
		},
		"torrents": map[string]interface{}{
			"seeding":     seedingActive,
			"paused":      seedingPaused,
			"downloading": downloadingCount,
			"totalSize":   totalSize,
		},
		"publish": map[string]interface{}{
			"todayCount":   publishToday,
			"totalCount":   publishTotal,
			"pendingCount": publishPending,
		},
		"reseed": map[string]interface{}{
			"todayCount":   reseedToday,
			"totalCount":   reseedTotal,
			"runningTasks": reseedActive,
		},
		"system": map[string]interface{}{
			"uptime":      time.Since(startTime).Seconds(),
			"version":     h.version,
			"goroutines":  runtime.NumGoroutine(),
			"memoryUsage": memStats.Alloc,
		},
		"rssSubscriptions": rssCount,
	})
}

func (h *DashboardHandler) handleActivities(w http.ResponseWriter, r *http.Request) {
	limit := int64(50)
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := parseInt64(v)
		if err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	var seen []model.RSSTorrentSeen
	if err := h.db.Order("created_at DESC").Limit(int(limit)).Find(&seen).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询最近种子失败")
		return
	}

	Success(w, map[string]interface{}{
		"items": seen,
		"total": len(seen),
	})
}

func (h *DashboardHandler) handleTrends(w http.ResponseWriter, r *http.Request) {
	days := 7
	if v := r.URL.Query().Get("days"); v != "" {
		if n, err := parseInt64(v); err == nil && n > 0 && n <= 30 {
			days = int(n)
		}
	}

	now := time.Now()
	points := make([]map[string]interface{}, 0, days)

	for i := days - 1; i >= 0; i-- {
		day := now.AddDate(0, 0, -i)
		dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
		dayEnd := dayStart.AddDate(0, 0, 1)

		var events int64
		h.db.Model(&model.TorrentEvent{}).Where("created_at >= ? AND created_at < ?", dayStart, dayEnd).Count(&events)

		var seen int64
		h.db.Model(&model.RSSTorrentSeen{}).Where("created_at >= ? AND created_at < ?", dayStart, dayEnd).Count(&seen)

		var publishDone int64
		h.db.Model(&model.PublishCandidate{}).Where("updated_at >= ? AND updated_at < ? AND publish_status = ?", dayStart, dayEnd, "done").Count(&publishDone)

		var reseedDone int64
		h.db.Model(&model.ReseedMatch{}).Where("injected_at >= ? AND injected_at < ?", dayStart, dayEnd).Count(&reseedDone)

		points = append(points, map[string]interface{}{
			"date":    dayStart.Format("2006-01-02"),
			"events":  events,
			"rss":     seen,
			"publish": publishDone,
			"reseed":  reseedDone,
		})
	}

	Success(w, map[string]interface{}{
		"trends": points,
		"days":   days,
	})
}

func (h *DashboardHandler) handleTorrentEvents(w http.ResponseWriter, r *http.Request, trimmed string) {
	if r.Method != http.MethodGet {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	if trimmed == "/api/v1/torrent-events" || trimmed == "/api/v1/torrent-events/" {
		var events []model.TorrentEvent
		var total int64

		q := h.db.Model(&model.TorrentEvent{})
		if site := r.URL.Query().Get("site"); site != "" {
			q = q.Where("site_name = ?", site)
		}
		q.Count(&total)
		if err := q.Order("created_at DESC").Limit(100).Find(&events).Error; err != nil {
			Error(w, http.StatusInternalServerError, 50000, "查询种子事件失败")
			return
		}

		Success(w, map[string]interface{}{
			"items": events,
			"total": total,
		})
		return
	}

	remaining := strings.TrimPrefix(trimmed, "/api/v1/torrent-events/")
	remaining = strings.TrimRight(remaining, "/")

	if remaining == "cleanup" && r.Method == http.MethodPost {
		before := time.Now().AddDate(0, 0, -30)
		result := h.db.Where("created_at < ?", before).Delete(&model.TorrentEvent{})
		if result.Error != nil {
			Error(w, http.StatusInternalServerError, 50000, "清理事件失败")
			return
		}
		Success(w, map[string]interface{}{
			"deleted": result.RowsAffected,
		})
		return
	}

	var event model.TorrentEvent
	if err := h.db.First(&event, remaining).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "事件不存在")
		return
	}
	Success(w, event)
}

func parseInt64(s string) (int64, error) {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, nil
		}
		n = n*10 + int64(c-'0')
	}
	return n, nil
}
