package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	dbimpl "github.com/ranfish/pt-forward/internal/db"
	"github.com/ranfish/pt-forward/internal/model"
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
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return

	case trimmed == "/api/v1/seeding/records" || trimmed == "/api/v1/seeding/records/":
		if r.Method == http.MethodGet {
			h.handleListRecords(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return

	case trimmed == "/api/v1/seeding/stats" || trimmed == "/api/v1/seeding/stats/":
		h.handleStats(w, r)
		return

	case trimmed == "/api/v1/seeding/scoring-dryrun" || trimmed == "/api/v1/seeding/scoring-dryrun/":
		if r.Method == http.MethodPost {
			h.handleScoringDryrun(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return

	case trimmed == "/api/v1/seeding/status" || trimmed == "/api/v1/seeding/status/":
		h.handleEngineStatus(w, r)
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
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
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
			h.handleTriggerClient(w, r, parts[0])
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
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
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
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	Error(w, http.StatusNotFound, 40400, "接口不存在")
}

func (h *SeedingHandler) handleListConfigs(w http.ResponseWriter, r *http.Request) {
	var configs []model.SeedingClientConfig
	if err := h.db.Find(&configs).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询刷流配置失败")
		return
	}
	Success(w, map[string]interface{}{
		"items": configs,
		"total": len(configs),
	})
}

func (h *SeedingHandler) handleGetConfig(w http.ResponseWriter, _ *http.Request, id uint) {
	var config model.SeedingClientConfig
	if err := h.db.First(&config, id).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "配置不存在")
		return
	}
	Success(w, config)
}

func (h *SeedingHandler) handleCreateConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID                 string  `json:"clientId"`
		Enabled                  bool    `json:"enabled"`
		DeleteRuleIDs            string  `json:"deleteRuleIds"`
		AutoDeleteCron           string  `json:"autoDeleteCron"`
		MainDataCron             string  `json:"mainDataCron"`
		DiskProtectEnabled       bool    `json:"diskProtectEnabled"`
		MinDiskSpaceGB           float64 `json:"minDiskSpaceGB"`
		MaxActiveUploads         int     `json:"maxActiveUploads"`
		MaxActiveDownloads       int     `json:"maxActiveDownloads"`
		SuperSeedingDefault      bool    `json:"superSeedingDefault"`
		FitTimeCheckMs           int     `json:"fitTimeCheckMs"`
		EmergencyBuffer          float64 `json:"emergencyBuffer"`
		SpaceAlarmEnabled        bool    `json:"spaceAlarmEnabled"`
		SpaceAlarmGB             float64 `json:"spaceAlarmGb"`
		MinDiskSpacePercent      float64 `json:"minDiskSpacePercent"`
		Scope                    string  `json:"scope"`
		PreFilterEnabled         bool    `json:"preFilterEnabled"`
		EnhancementBatchSize     int     `json:"enhancementBatchSize"`
		EnhancementCacheTTL      int     `json:"enhancementCacheTtl"`
		ActiveTimeWindows        string  `json:"activeTimeWindows"`
		EmaAlpha                 float64 `json:"emaAlpha"`
		CleanupScoreWeights      string  `json:"cleanupScoreWeights"`
		ArchiveGranularity       string  `json:"archiveGranularity"`
		RejectRuleIDs            string  `json:"rejectRuleIds"`
		ReannounceBefore         bool    `json:"reannounceBefore"`
		ReannounceRetries        int     `json:"reannounceRetries"`
		ReannounceIntervalMs     int     `json:"reannounceIntervalMs"`
		ReannounceWaitMs         int     `json:"reannounceWaitMs"`
		MinSeedHoursBeforeDelete float64 `json:"minSeedHoursBeforeDelete"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.ClientID == "" {
		Error(w, http.StatusBadRequest, 40001, "clientId 为必填项")
		return
	}

	var count int64
	if err := h.db.Model(&model.SeedingClientConfig{}).Where("client_id = ?", req.ClientID).Count(&count).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询刷流配置失败")
		return
	}
	if count > 0 {
		Error(w, http.StatusConflict, 40900, "该下载器已有刷流配置")
		return
	}

	config := model.SeedingClientConfig{
		ClientID:                 req.ClientID,
		Enabled:                  req.Enabled,
		DeleteRuleIDs:            req.DeleteRuleIDs,
		AutoDeleteCron:           req.AutoDeleteCron,
		MainDataCron:             req.MainDataCron,
		DiskProtectEnabled:       req.DiskProtectEnabled,
		MinDiskSpaceGB:           req.MinDiskSpaceGB,
		MaxActiveUploads:         req.MaxActiveUploads,
		MaxActiveDownloads:       req.MaxActiveDownloads,
		SuperSeedingDefault:      req.SuperSeedingDefault,
		FitTimeCheckMs:           req.FitTimeCheckMs,
		EmergencyBuffer:          req.EmergencyBuffer,
		SpaceAlarmEnabled:        req.SpaceAlarmEnabled,
		SpaceAlarmGB:             req.SpaceAlarmGB,
		MinDiskSpacePercent:      req.MinDiskSpacePercent,
		Scope:                    req.Scope,
		PreFilterEnabled:         req.PreFilterEnabled,
		EnhancementBatchSize:     req.EnhancementBatchSize,
		EnhancementCacheTTL:      req.EnhancementCacheTTL,
		ActiveTimeWindows:        req.ActiveTimeWindows,
		EmaAlpha:                 req.EmaAlpha,
		CleanupScoreWeights:      req.CleanupScoreWeights,
		ArchiveGranularity:       req.ArchiveGranularity,
		RejectRuleIDs:            req.RejectRuleIDs,
		ReannounceBefore:         req.ReannounceBefore,
		ReannounceRetries:        req.ReannounceRetries,
		ReannounceIntervalMs:     req.ReannounceIntervalMs,
		ReannounceWaitMs:         req.ReannounceWaitMs,
		MinSeedHoursBeforeDelete: req.MinSeedHoursBeforeDelete,
	}
	if config.AutoDeleteCron == "" {
		config.AutoDeleteCron = "*/30 * * * *"
	}
	if config.MainDataCron == "" {
		config.MainDataCron = "*/10 * * * *"
	}
	if config.MinDiskSpaceGB == 0 {
		config.MinDiskSpaceGB = 50
	}

	if err := dbimpl.ForceCreate(h.db, &config); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "创建刷流配置失败")
		return
	}
	auditLog(r, "seeding", "create", "config", fmt.Sprintf("%d", config.ID), config.ClientID, "success")
	Success(w, config)
}

func (h *SeedingHandler) handleUpdateConfig(w http.ResponseWriter, r *http.Request, id uint) {
	var config model.SeedingClientConfig
	if err := h.db.First(&config, id).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "配置不存在")
		return
	}

	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	updates := make(map[string]interface{})
	if v, ok := req["enabled"]; ok {
		updates["enabled"] = v
	}
	if v, ok := req["deleteRuleIds"]; ok {
		updates["delete_rule_ids"] = v
	}
	if v, ok := req["autoDeleteCron"]; ok {
		updates["auto_delete_cron"] = v
	}
	if v, ok := req["mainDataCron"]; ok {
		updates["main_data_cron"] = v
	}
	if v, ok := req["diskProtectEnabled"]; ok {
		updates["disk_protect_enabled"] = v
	}
	if v, ok := req["minDiskSpaceGB"]; ok {
		updates["min_disk_space_gb"] = v
	}
	if v, ok := req["maxActiveUploads"]; ok {
		updates["max_active_uploads"] = v
	}
	if v, ok := req["maxActiveDownloads"]; ok {
		updates["max_active_downloads"] = v
	}
	if v, ok := req["maxActiveSeeding"]; ok {
		updates["max_active_seeding"] = v
	}
	if v, ok := req["superSeedingDefault"]; ok {
		updates["super_seeding_default"] = v
	}
	if v, ok := req["fitTimeCheckMs"]; ok {
		updates["fit_time_check_ms"] = v
	}
	if v, ok := req["emergencyBuffer"]; ok {
		updates["emergency_buffer"] = v
	}
	if v, ok := req["spaceAlarmEnabled"]; ok {
		updates["space_alarm_enabled"] = v
	}
	if v, ok := req["spaceAlarmGb"]; ok {
		updates["space_alarm_gb"] = v
	}
	if v, ok := req["minDiskSpacePercent"]; ok {
		updates["min_disk_space_percent"] = v
	}
	if v, ok := req["scope"]; ok {
		updates["scope"] = v
	}
	if v, ok := req["preFilterEnabled"]; ok {
		updates["pre_filter_enabled"] = v
	}
	if v, ok := req["enhancementBatchSize"]; ok {
		updates["enhancement_batch_size"] = v
	}
	if v, ok := req["enhancementCacheTtl"]; ok {
		updates["enhancement_cache_ttl"] = v
	}
	if v, ok := req["activeTimeWindows"]; ok {
		updates["active_time_windows"] = v
	}
	if v, ok := req["emaAlpha"]; ok {
		updates["ema_alpha"] = v
	}
	if v, ok := req["cleanupScoreWeights"]; ok {
		updates["cleanup_score_weights"] = v
	}
	if v, ok := req["archiveGranularity"]; ok {
		updates["archive_granularity"] = v
	}
	if v, ok := req["rejectRuleIds"]; ok {
		updates["reject_rule_ids"] = v
	}
	if v, ok := req["reannounceBefore"]; ok {
		updates["reannounce_before"] = v
	}
	if v, ok := req["reannounceRetries"]; ok {
		updates["reannounce_retries"] = v
	}
	if v, ok := req["reannounceIntervalMs"]; ok {
		updates["reannounce_interval_ms"] = v
	}
	if v, ok := req["reannounceWaitMs"]; ok {
		updates["reannounce_wait_ms"] = v
	}
	if v, ok := req["minSeedHoursBeforeDelete"]; ok {
		updates["min_seed_hours_before_delete"] = v
	}
	updates["updated_at"] = time.Now()

	if err := h.db.Model(&config).Updates(updates).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "更新刷流配置失败")
		return
	}
	if err := h.db.First(&config, id).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询刷流配置失败")
		return
	}
	auditLog(r, "seeding", "update", "config", fmt.Sprintf("%d", id), config.ClientID, "success")
	Success(w, config)
}

func (h *SeedingHandler) handleDeleteConfig(w http.ResponseWriter, r *http.Request, id uint) {
	if err := h.db.Delete(&model.SeedingClientConfig{}, id).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "删除刷流配置失败")
		return
	}
	auditLog(r, "seeding", "delete", "config", fmt.Sprintf("%d", id), "", "success")
	Success(w, nil)
}

func (h *SeedingHandler) handleListRecords(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	clientID := query.Get("client_id")
	siteName := query.Get("site")
	status := query.Get("status")
	search := query.Get("search")
	page, _ := strconv.Atoi(query.Get("page"))
	size, _ := strconv.Atoi(query.Get("size"))
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 50
	}

	if h.engine != nil && clientID != "" {
		records, err := h.engine.ListByClient(r.Context(), clientID)
		if err != nil {
			Error(w, http.StatusInternalServerError, 50000, "查询刷流记录失败")
			return
		}

		filtered := records
		if siteName != "" {
			var tmp []model.SeedingTorrentRecord
			for _, r := range filtered {
				if r.SiteName == siteName {
					tmp = append(tmp, r)
				}
			}
			filtered = tmp
		}
		if status != "" {
			var tmp []model.SeedingTorrentRecord
			for _, r := range filtered {
				if string(r.Status) == status {
					tmp = append(tmp, r)
				}
			}
			filtered = tmp
		}
		if search != "" {
			var tmp []model.SeedingTorrentRecord
			for _, r := range filtered {
				if strings.Contains(r.InfoHash, search) || strings.Contains(r.TorrentID, search) || strings.Contains(r.SiteName, search) {
					tmp = append(tmp, r)
				}
			}
			filtered = tmp
		}

		total := len(filtered)
		start := (page - 1) * size
		if start > total {
			start = total
		}
		end := start + size
		if end > total {
			end = total
		}

		Success(w, map[string]interface{}{
			"items": filtered[start:end],
			"total": total,
			"page":  page,
			"size":  size,
		})
		return
	}

	q := h.db.Model(&model.SeedingTorrentRecord{}).
		Where("status IN ?", []string{"seeding", "paused_free_end", "paused_rule"})
	if clientID != "" {
		q = q.Where("client_id = ?", clientID)
	}
	if siteName != "" {
		q = q.Where("site_name = ?", siteName)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if search != "" {
		q = q.Where("info_hash LIKE ? OR torrent_id LIKE ? OR site_name LIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询种子记录总数失败")
		return
	}

	var records []model.SeedingTorrentRecord
	offset := (page - 1) * size
	if err := q.Session(&gorm.Session{}).Order("updated_at DESC").Offset(offset).Limit(size).Find(&records).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询种子记录失败")
		return
	}

	type seenTitle struct {
		SiteName  string
		TorrentID string
		Title     string
	}
	var titles []seenTitle
	h.db.Model(&model.RSSTorrentSeen{}).
		Select("site_name, torrent_id, title").
		Find(&titles)
	titleMap := make(map[string]string, len(titles))
	for _, t := range titles {
		titleMap[t.SiteName+"|"+t.TorrentID] = t.Title
	}

	type siteInfo struct {
		BaseURL   string
		Framework string
	}
	var sites []model.Site
	h.db.Select("name, base_url, framework").Find(&sites)
	siteMap := make(map[string]siteInfo, len(sites))
	for _, s := range sites {
		siteMap[s.Name] = siteInfo{BaseURL: s.BaseURL, Framework: s.Framework}
	}

	type recordItem struct {
		model.SeedingTorrentRecord
		Title     string `json:"title"`
		DetailURL string `json:"detail_url"`
	}

	items := make([]recordItem, len(records))
	for i := range records {
		items[i] = recordItem{SeedingTorrentRecord: records[i]}
		if title, ok := titleMap[records[i].SiteName+"|"+records[i].TorrentID]; ok {
			items[i].Title = title
		}
		if si, ok := siteMap[records[i].SiteName]; ok && si.BaseURL != "" && records[i].TorrentID != "" {
			items[i].DetailURL = buildDetailURL(si.BaseURL, si.Framework, records[i].TorrentID)
		}
	}

	Success(w, map[string]interface{}{
		"items": items,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

func extractLastSegment(path, prefix string) string {
	remaining := strings.TrimPrefix(path, prefix)
	remaining = strings.TrimRight(remaining, "/")
	return remaining
}

func parseUintParam(path, prefix string) (uint, error) {
	remaining := strings.TrimPrefix(path, prefix)
	remaining = strings.TrimRight(remaining, "/")
	parts := strings.Split(remaining, "/")
	last := parts[len(parts)-1]
	n, err := strconv.ParseUint(last, 10, 64)
	if err != nil {
		return 0, apiError(ErrAPIInvalidID, fmt.Sprintf("invalid ID: %s", last), nil)
	}
	return uint(n), nil
}

func (h *SeedingHandler) handleStats(w http.ResponseWriter, _ *http.Request) {
	activeCount := 0
	var managedCounts *seeding.ManagedCounts
	if h.engine != nil {
		managedCounts = h.engine.GetManagedCounts()
		activeCount = managedCounts.Active
	}

	dbPausedCount := 0
	if managedCounts != nil {
		dbPausedCount = managedCounts.Paused
	}

	Success(w, map[string]interface{}{
		"activeRecords": activeCount,
		"dbPausedCount": dbPausedCount,
	})
}

func (h *SeedingHandler) handleScoringDryrun(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Seeders       int     `json:"seeders"`
		Leechers      int     `json:"leechers"`
		AgeHours      float64 `json:"ageHours"`
		Size          int64   `json:"size"`
		Discount      string  `json:"discount"`
		HalfLifeHours float64 `json:"halfLifeHours"`
		SiteWeight    float64 `json:"siteWeight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	discount := model.DiscountLevel(req.Discount)
	if discount == "" {
		discount = model.DiscountNone
	}

	result := seeding.CalculateScore(seeding.ScoreInput{
		Seeders:       req.Seeders,
		Leechers:      req.Leechers,
		AgeHours:      req.AgeHours,
		Size:          req.Size,
		Discount:      discount,
		HalfLifeHours: req.HalfLifeHours,
		SiteWeight:    req.SiteWeight,
	})

	isFree := discount == model.DiscountFree || discount == model.Discount2xFree
	shouldCleanup := seeding.ShouldCleanup(seeding.CleanupCandidate{
		Score:    result.EffectiveScore,
		AgeHours: req.AgeHours,
		IsFree:   isFree,
		Discount: discount,
	}, 0.3, 72)

	Success(w, map[string]interface{}{
		"score":          result.Score,
		"effectiveScore": result.EffectiveScore,
		"demandScore":    result.DemandScore,
		"uploadValue":    result.UploadValue,
		"recencyFactor":  result.RecencyFactor,
		"shouldCleanup":  shouldCleanup,
	})
}

func (h *SeedingHandler) handleResumeRecord(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的记录 ID")
		return
	}

	if h.engine == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "刷流引擎未初始化")
		return
	}

	if err := h.engine.UpdateStatus(r.Context(), uint(id), model.SeedingStatusSeeding, "manual_resume"); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "恢复记录失败")
		return
	}

	h.logger.Info("seeding record resumed", zap.String("id", idStr))
	Success(w, map[string]interface{}{"message": "已恢复"})
}

func (h *SeedingHandler) handlePauseRecord(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的记录 ID")
		return
	}

	if h.engine == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "刷流引擎未初始化")
		return
	}

	if err := h.engine.UpdateStatus(r.Context(), uint(id), model.SeedingStatusPausedFreeEnd, "manual_pause"); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "暂停记录失败")
		return
	}

	h.logger.Info("seeding record paused", zap.String("id", idStr))
	Success(w, map[string]interface{}{"message": "已暂停"})
}

func (h *SeedingHandler) handleEngineStatus(w http.ResponseWriter, _ *http.Request) {
	activeCount := 0
	pausedCount := 0
	realSeeding := 0
	realDownloading := 0
	if h.engine != nil {
		mc := h.engine.GetManagedCounts()
		activeCount = mc.Active
		pausedCount = mc.Paused
		for _, rc := range h.engine.GetRealTorrentCounts() {
			realSeeding += rc.Seeding
			realDownloading += rc.Downloading
		}
	}

	var total int64
	if err := h.db.Model(&model.SeedingTorrentRecord{}).Count(&total).Error; err != nil {
		h.logger.Warn("engine status: query total count failed", zap.Error(err))
	}

	Success(w, map[string]interface{}{
		"running":       true,
		"uptimeSeconds": time.Since(startTime).Seconds(),
		"overview": map[string]interface{}{
			"totalTorrents":   total,
			"activeTorrents":  activeCount,
			"pausedTorrents":  pausedCount,
			"realSeeding":     realSeeding,
			"realDownloading": realDownloading,
		},
	})
}

func (h *SeedingHandler) handleListTorrents(w http.ResponseWriter, r *http.Request) {
	q := h.db.Model(&model.SeedingTorrentRecord{}).
		Where("status IN ?", []string{"seeding", "paused_free_end", "paused_rule"})

	if clientID := r.URL.Query().Get("clientId"); clientID != "" {
		q = q.Where("client_id = ?", clientID)
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if page < 1 {
		page = 1
	} else if page > 10000 {
		page = 10000
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int64
	if err := q.Count(&total).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询种子列表总数失败")
		return
	}

	var records []model.SeedingTorrentRecord
	if err := q.Session(&gorm.Session{}).Order("updated_at DESC").Offset(offset).Limit(pageSize).Find(&records).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询种子列表失败")
		return
	}

	type seenTitle struct {
		SiteName  string
		TorrentID string
		Title     string
	}
	var titles []seenTitle
	h.db.Model(&model.RSSTorrentSeen{}).
		Select("site_name, torrent_id, title").
		Find(&titles)
	titleMap := make(map[string]string, len(titles))
	for _, t := range titles {
		titleMap[t.SiteName+":"+t.TorrentID] = t.Title
	}

	type siteInfo struct {
		BaseURL   string
		Framework string
	}
	var sites []model.Site
	h.db.Select("name, base_url, framework").Find(&sites)
	siteMap := make(map[string]siteInfo, len(sites))
	for _, s := range sites {
		siteMap[s.Name] = siteInfo{BaseURL: s.BaseURL, Framework: s.Framework}
	}

	type torrentItem struct {
		model.SeedingTorrentRecord
		Title     string `json:"title"`
		DetailURL string `json:"detail_url"`
	}

	items := make([]torrentItem, len(records))
	for i := range records {
		items[i] = torrentItem{SeedingTorrentRecord: records[i]}
		if title, ok := titleMap[records[i].SiteName+":"+records[i].TorrentID]; ok {
			items[i].Title = title
		}
		if si, ok := siteMap[records[i].SiteName]; ok && si.BaseURL != "" && records[i].TorrentID != "" {
			items[i].DetailURL = buildDetailURL(si.BaseURL, si.Framework, records[i].TorrentID)
		}
	}

	Success(w, map[string]interface{}{
		"items": items,
		"total": total,
		"page":  page,
	})
}

func (h *SeedingHandler) handleListClients(w http.ResponseWriter, _ *http.Request) {
	var configs []model.SeedingClientConfig
	if err := h.db.Find(&configs).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询做种客户端配置失败")
		return
	}
	Success(w, map[string]interface{}{
		"items": configs,
		"total": len(configs),
	})
}

func (h *SeedingHandler) handleScoringConfig(w http.ResponseWriter, r *http.Request) {
	subID := r.URL.Query().Get("subscriptionId")
	if subID == "" {
		if r.Method == http.MethodGet {
			Success(w, map[string]interface{}{"enabled": false, "halfLifeHours": 2.0})
		} else {
			Error(w, http.StatusBadRequest, 40001, "subscriptionId 为必填项")
		}
		return
	}

	var sub model.RSSSubscription
	if err := h.db.Where("id = ?", subID).First(&sub).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "订阅不存在")
		return
	}

	if r.Method == http.MethodPut {
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, 40001, "请求格式错误")
			return
		}
		updates := make(map[string]interface{})
		if v, ok := req["enabled"].(bool); ok {
			sub.ScoringConfig.Enabled = v
			updates["scoring_enabled"] = v
		}
		if v, ok := req["halfLifeHours"].(float64); ok {
			sub.ScoringConfig.HalfLifeHours = v
			updates["half_life_hours"] = v
		}
		if v, ok := req["minScore"].(float64); ok {
			sub.ScoringConfig.MinScore = v
			updates["min_score"] = v
		}
		if v, ok := req["maxCandidates"].(float64); ok && v >= 0 && v <= 10000 {
			sub.ScoringConfig.MaxCandidates = int(v)
			updates["max_candidates"] = int(v)
		}
		if v, ok := req["maxActiveSeeding"].(float64); ok && v >= 0 && v <= 10000 {
			sub.ScoringConfig.MaxActiveSeeding = int(v)
			updates["max_active_seeding"] = int(v)
		}
		if v, ok := req["topNConfirm"].(float64); ok && v >= 0 && v <= 100 {
			sub.ScoringConfig.TopNConfirm = int(v)
			updates["top_n_confirm"] = int(v)
		}
		if v, ok := req["include2xUp"].(bool); ok {
			sub.ScoringConfig.Include2xUp = v
			updates["include2x_up"] = v
		}
		if v, ok := req["siteWeightsJSON"].(string); ok {
			sub.ScoringConfig.SiteWeightsJSON = v
			updates["site_weights_json"] = v
		}
		if v, ok := req["batchLimit"].(float64); ok && v >= 0 && v <= 10000 {
			sub.ScoringConfig.BatchLimit = int(v)
			updates["batch_limit"] = int(v)
		}
		if v, ok := req["pushIntervalMs"].(float64); ok && v >= 0 && v <= 3600000 {
			sub.ScoringConfig.PushIntervalMs = int(v)
			updates["push_interval_ms"] = int(v)
		}
		if len(updates) > 0 {
			if err := h.db.Model(&sub).Updates(updates).Error; err != nil {
				Error(w, http.StatusInternalServerError, 50000, "保存评分配置失败")
				return
			}
		}
	}

	Success(w, sub.ScoringConfig)
}

func (h *SeedingHandler) handleUnregisteredKeywords(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		var val string
		h.db.Raw("SELECT value FROM system_settings WHERE key = 'seeding.unregistered_keywords' LIMIT 1").Row().Scan(&val)
		var keywords []string
		if val != "" {
			json.Unmarshal([]byte(val), &keywords)
		}
		if len(keywords) == 0 {
			keywords = []string{
				"unregistered torrent", "unregistered", "torrent not found",
				"torrent not exist", "not registered", "unknown torrent",
				"invalid torrent", "torrent has been deleted",
			}
		}
		Success(w, map[string]interface{}{"keywords": keywords})
		return
	}
	if r.Method == http.MethodPut || r.Method == http.MethodPost {
		var body struct {
			Keywords []string `json:"keywords"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			Error(w, http.StatusBadRequest, 40000, "invalid body")
			return
		}
		data, _ := json.Marshal(body.Keywords)
		h.db.Exec("INSERT INTO system_settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
			"seeding.unregistered_keywords", string(data))
		Success(w, map[string]interface{}{"keywords": body.Keywords})
		return
	}
	Error(w, http.StatusMethodNotAllowed, 40500, "method not allowed")
}

func (h *SeedingHandler) handleScoringLogs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	var logs []model.ScoringLog
	if err := h.db.Order("created_at DESC").Limit(limit).Find(&logs).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询评分日志失败")
		return
	}

	type titleRow struct {
		SiteName  string `json:"site_name"`
		TorrentID string `json:"torrent_id"`
		Title     string `json:"title"`
	}
	var titles []titleRow
	if len(logs) > 0 {
		keys := make([]string, 0, len(logs))
		for _, l := range logs {
			keys = append(keys, l.SiteName+"|"+l.TorrentID)
		}
		h.db.Raw(`SELECT site_name, torrent_id, title FROM rss_torrent_seen WHERE site_name || '|' || torrent_id IN (?)`, keys).Scan(&titles)
	}
	titleMap := make(map[string]string, len(titles))
	for _, t := range titles {
		titleMap[t.SiteName+"|"+t.TorrentID] = t.Title
	}

	type siteInfo struct {
		BaseURL   string
		Framework string
	}
	var siteList []model.Site
	h.db.Select("name, base_url, framework").Find(&siteList)
	siteMap := make(map[string]siteInfo, len(siteList))
	for _, s := range siteList {
		siteMap[s.Name] = siteInfo{BaseURL: s.BaseURL, Framework: s.Framework}
	}

	type logItem struct {
		model.ScoringLog
		Title     string `json:"title"`
		DetailURL string `json:"detail_url"`
	}

	items := make([]logItem, len(logs))
	for i := range logs {
		items[i] = logItem{ScoringLog: logs[i]}
		if t, ok := titleMap[logs[i].SiteName+"|"+logs[i].TorrentID]; ok {
			items[i].Title = t
		}
		if si, ok := siteMap[logs[i].SiteName]; ok && si.BaseURL != "" && logs[i].TorrentID != "" {
			items[i].DetailURL = buildDetailURL(si.BaseURL, si.Framework, logs[i].TorrentID)
		}
	}

	Success(w, map[string]interface{}{
		"items": items,
		"total": len(items),
	})
}

func (h *SeedingHandler) handleTriggerClient(w http.ResponseWriter, r *http.Request, clientID string) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	var cfg model.SeedingClientConfig
	cfgErr := h.db.WithContext(ctx).Where("client_id = ? AND enabled = ?", clientID, true).First(&cfg).Error

	if cfgErr != nil {
		count := 0
		if h.engine != nil {
			count = h.engine.GetActiveCount(clientID)
		}
		h.logger.Warn("seeding trigger: no enabled config, returning count only", zap.String("clientId", clientID))
		Success(w, map[string]interface{}{
			"processedCount": count,
			"clientId":       clientID,
			"evaluated":      false,
			"reason":         "no enabled config",
		})
		return
	}

	if h.engine == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "刷流引擎未初始化")
		return
	}

	result, err := h.engine.Evaluate(ctx, clientID, &cfg)
	if err != nil {
		h.logger.Error("seeding evaluate failed", zap.String("clientId", clientID), zap.Error(err))
		Error(w, http.StatusInternalServerError, 50000, "评估执行失败: "+err.Error())
		return
	}

	h.logger.Info("seeding client evaluated",
		zap.String("clientId", clientID),
		zap.Int("evaluated", result.Evaluated),
		zap.Int("paused", result.Paused),
		zap.Int("deleted", result.Deleted),
		zap.Int("limited", result.Limited),
		zap.Int("errors", result.Errors),
	)
	auditLog(r, "seeding", "trigger", "client", clientID, fmt.Sprintf("evaluated=%d,paused=%d,deleted=%d", result.Evaluated, result.Paused, result.Deleted), "success")
	Success(w, map[string]interface{}{
		"clientId":       clientID,
		"evaluated":      true,
		"processedCount": result.Evaluated,
		"pausedCount":    result.Paused,
		"deletedCount":   result.Deleted,
		"limitedCount":   result.Limited,
		"errorCount":     result.Errors,
	})
}

func (h *SeedingHandler) handleScoringConfigByID(w http.ResponseWriter, r *http.Request, subID string) {
	var sub model.RSSSubscription
	if err := h.db.Where("id = ?", subID).First(&sub).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "订阅不存在")
		return
	}

	if r.Method == http.MethodPut {
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, 40001, "请求格式错误")
			return
		}
		updates := make(map[string]interface{})
		if v, ok := req["enabled"].(bool); ok {
			sub.ScoringConfig.Enabled = v
			updates["enabled"] = v
		}
		if v, ok := req["halfLifeHours"].(float64); ok {
			sub.ScoringConfig.HalfLifeHours = v
			updates["half_life_hours"] = v
		}
		if v, ok := req["minScore"].(float64); ok {
			sub.ScoringConfig.MinScore = v
			updates["min_score"] = v
		}
		if v, ok := req["maxCandidates"].(float64); ok && v >= 0 && v <= 10000 {
			sub.ScoringConfig.MaxCandidates = int(v)
			updates["max_candidates"] = int(v)
		}
		if v, ok := req["maxActiveSeeding"].(float64); ok && v >= 0 && v <= 10000 {
			sub.ScoringConfig.MaxActiveSeeding = int(v)
			updates["max_active_seeding"] = int(v)
		}
		if v, ok := req["topNConfirm"].(float64); ok && v >= 0 && v <= 100 {
			sub.ScoringConfig.TopNConfirm = int(v)
			updates["top_n_confirm"] = int(v)
		}
		if v, ok := req["include2xUp"].(bool); ok {
			sub.ScoringConfig.Include2xUp = v
			updates["include2x_up"] = v
		}
		if v, ok := req["siteWeightsJSON"].(string); ok {
			sub.ScoringConfig.SiteWeightsJSON = v
			updates["site_weights_json"] = v
		}
		if v, ok := req["batchLimit"].(float64); ok && v >= 0 && v <= 10000 {
			sub.ScoringConfig.BatchLimit = int(v)
			updates["batch_limit"] = int(v)
		}
		if v, ok := req["pushIntervalMs"].(float64); ok && v >= 0 && v <= 3600000 {
			sub.ScoringConfig.PushIntervalMs = int(v)
			updates["push_interval_ms"] = int(v)
		}
		if len(updates) > 0 {
			if err := h.db.Model(&sub).Updates(updates).Error; err != nil {
				Error(w, http.StatusInternalServerError, 50000, "保存评分配置失败")
				return
			}
		}
	}

	Success(w, sub.ScoringConfig)
}

func (h *SeedingHandler) handleScoringCycle(w http.ResponseWriter, r *http.Request, cycleID string) {
	Success(w, map[string]interface{}{
		"cycleId": cycleID,
		"items":   []interface{}{},
		"total":   0,
	})
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
		clientID := strings.TrimPrefix(remaining, "downloader/")
		clientID = strings.TrimRight(clientID, "/")
		if strings.HasSuffix(clientID, "/speed-trend") {
			id := strings.TrimSuffix(clientID, "/speed-trend")
			h.handleDownloaderSpeedTrend(w, r, id)
		} else {
			Error(w, http.StatusNotFound, 40400, "路径不存在")
		}
	default:
		Error(w, http.StatusNotFound, 40400, "路径不存在")
	}
}

func (h *SeedingHandler) handleStatsOverview(w http.ResponseWriter, _ *http.Request) {
	activeTorrents := 0
	pausedTorrents := 0
	realSeeding := 0
	realDownloading := 0
	if h.engine != nil {
		mc := h.engine.GetManagedCounts()
		activeTorrents = mc.Active
		pausedTorrents = mc.Paused
		for _, rc := range h.engine.GetRealTorrentCounts() {
			realSeeding += rc.Seeding
			realDownloading += rc.Downloading
		}
	}

	var total int64
	if err := h.db.Model(&model.SeedingTorrentRecord{}).Count(&total).Error; err != nil {
		h.logger.Warn("stats overview: query total count failed", zap.Error(err))
	}

	today := time.Now().Truncate(24 * time.Hour)
	var todayAdded int64
	if err := h.db.Model(&model.SeedingTorrentRecord{}).Where("created_at >= ?", today).Count(&todayAdded).Error; err != nil {
		h.logger.Warn("stats overview: query today added count failed", zap.Error(err))
	}

	var todayDeleted int64
	if err := h.db.Model(&model.SeedingTorrentRecord{}).Where("status = ? AND updated_at >= ?", "deleted", today).Count(&todayDeleted).Error; err != nil {
		h.logger.Warn("stats overview: query today deleted count failed", zap.Error(err))
	}

	var globalStats *model.GlobalTransferStats
	if h.engine != nil {
		globalStats = h.engine.GetGlobalTransferStats(context.Background())
	}
	if globalStats == nil {
		globalStats = &model.GlobalTransferStats{}
	}

	todayUpload := int64(0)
	todayDownload := int64(0)
	if h.engine != nil {
		todayStats := h.engine.GetTodayTransferDelta(context.Background())
		todayUpload = todayStats.AllTimeUpload
		todayDownload = todayStats.AllTimeDownload
	}

	var globalRatio float64
	if globalStats.AllTimeDownload > 0 {
		globalRatio = float64(globalStats.AllTimeUpload) / float64(globalStats.AllTimeDownload)
	} else if globalStats.AllTimeUpload > 0 {
		globalRatio = -1
	}

	Success(w, map[string]interface{}{
		"totalTorrents":      total,
		"activeTorrents":     activeTorrents,
		"pausedTorrents":     pausedTorrents,
		"realSeeding":        realSeeding,
		"realDownloading":    realDownloading,
		"totalUploadBytes":   globalStats.AllTimeUpload,
		"totalDownloadBytes": globalStats.AllTimeDownload,
		"todayUploadBytes":   todayUpload,
		"todayDownloadBytes": todayDownload,
		"globalRatio":        globalRatio,
		"todayDeleted":       todayDeleted,
		"todayAdded":         todayAdded,
		"statsScope":         "seeding",
		"statsScopeNote":     "仅统计刷流绑定且启用的下载器全局传输量（含该下载器内所有种子，含非刷流种子）",
	})
}

func (h *SeedingHandler) handleStatsBySite(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	today := time.Now().Truncate(24 * time.Hour)

	type siteStat struct {
		SiteName      string `json:"siteName"`
		SeedingCount  int64  `json:"seedingCount"`
		TotalCount    int64  `json:"totalCount"`
		TodayAdded    int64  `json:"todayAdded"`
		TodayDeleted  int64  `json:"todayDeleted"`
		ActiveFree    int64  `json:"activeFree"`
		ActiveNonFree int64  `json:"activeNonFree"`
		TodayUpload   int64  `json:"todayUploadBytes"`
		HistoryUpload int64  `json:"historyUploadBytes"`
	}

	var stats []siteStat
	if err := h.db.Model(&model.SeedingTorrentRecord{}).
		Select(`site_name,
			SUM(CASE WHEN status IN ('seeding','pending','paused_free_end','paused_rule') THEN 1 ELSE 0 END) as seeding_count,
			COUNT(*) as total_count,
			SUM(CASE WHEN created_at >= ? THEN 1 ELSE 0 END) as today_added,
			SUM(CASE WHEN status = 'deleted' AND updated_at >= ? THEN 1 ELSE 0 END) as today_deleted,
			SUM(CASE WHEN is_free = 1 AND status IN ('seeding','pending','paused_free_end','paused_rule') THEN 1 ELSE 0 END) as active_free,
			SUM(CASE WHEN is_free = 0 AND status IN ('seeding','pending','paused_free_end','paused_rule') THEN 1 ELSE 0 END) as active_non_free,
			0 as today_upload_bytes,
			SUM(final_uploaded) as history_upload_bytes`,
			today, today).
		Group("site_name").
		Order("seeding_count DESC").
		Limit(limit).
		Find(&stats).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询站点统计失败")
		return
	}

	siteUploadMap := map[string]int64{}
	type uploadRow struct {
		SiteName    string `json:"site_name"`
		UploadDelta int64  `json:"upload_delta"`
	}
	var uploadRows []uploadRow
	h.db.Raw(`SELECT site_name, SUM(delta_upload) as upload_delta
		FROM (
			SELECT site_name, info_hash,
				MAX(uploaded) - MIN(uploaded) AS delta_upload
			FROM torrent_traffic
			WHERE recorded_at >= ?
			GROUP BY site_name, info_hash
		) sub
		GROUP BY site_name`, today).Scan(&uploadRows)
	for _, row := range uploadRows {
		siteUploadMap[row.SiteName] = row.UploadDelta
	}

	for i := range stats {
		if v, ok := siteUploadMap[stats[i].SiteName]; ok {
			stats[i].TodayUpload = v
		}
	}

	historyUploadMap := map[string]int64{}
	type historyRow struct {
		SiteName string `json:"site_name"`
		Uploaded int64  `json:"uploaded"`
	}
	var historyRows []historyRow
	h.db.Raw(`SELECT site_name, SUM(uploaded) as uploaded
		FROM (
			SELECT str.site_name, str.info_hash,
				CASE WHEN str.final_uploaded > 0 THEN str.final_uploaded
					WHEN tt.uploaded > 0 THEN tt.uploaded
					ELSE 0 END as uploaded
			FROM seeding_torrent_records str
			LEFT JOIN (
				SELECT info_hash, MAX(uploaded) as uploaded
				FROM torrent_traffic
				GROUP BY info_hash
			) tt ON tt.info_hash = str.info_hash
			WHERE str.status IN ('seeding','pending','paused_free_end','paused_rule')
		) sub
		GROUP BY site_name`).Scan(&historyRows)
	for _, row := range historyRows {
		historyUploadMap[row.SiteName] = row.Uploaded
	}

	for i := range stats {
		if v, ok := historyUploadMap[stats[i].SiteName]; ok {
			stats[i].HistoryUpload += v
		}
	}

	Success(w, map[string]interface{}{
		"items": stats,
		"total": len(stats),
	})
}

func (h *SeedingHandler) handleSiteTrend(w http.ResponseWriter, r *http.Request, site string) {
	days := 7
	if v := r.URL.Query().Get("range"); v == "30d" {
		days = 30
	} else if v == "24h" {
		days = 1
	}

	now := time.Now()
	points := make([]map[string]interface{}, 0, days)

	for i := days; i >= 0; i-- {
		day := now.AddDate(0, 0, -i)
		dayStart := day.Truncate(24 * time.Hour)
		dayEnd := dayStart.AddDate(0, 0, 1)

		var count int64
		if err := h.db.Model(&model.SeedingTorrentRecord{}).
			Where("site_name = ? AND created_at >= ? AND created_at < ?", site, dayStart, dayEnd).
			Count(&count).Error; err != nil {
			h.logger.Warn("site trend: query count failed", zap.Error(err))
		}

		points = append(points, map[string]interface{}{
			"date":  dayStart.Format("2006-01-02"),
			"count": count,
		})
	}

	Success(w, map[string]interface{}{
		"site":   site,
		"trends": points,
	})
}

func (h *SeedingHandler) handleStatsTorrents(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("size"))
	sortBy := r.URL.Query().Get("sort")
	if page < 1 {
		page = 1
	} else if page > 10000 {
		page = 10000
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	status := r.URL.Query().Get("status")
	q := h.db.Model(&model.SeedingTorrentRecord{}).
		Where("status IN ?", []string{"seeding", "paused_free_end", "paused_rule"})
	if status != "" {
		q = q.Where("status = ?", status)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询种子统计总数失败")
		return
	}

	orderClause := "final_uploaded DESC"
	switch sortBy {
	case "size":
		orderClause = "torrent_size DESC"
	case "uploaded":
		orderClause = "final_uploaded DESC"
	case "time":
		orderClause = "flushed_at DESC"
	default:
		orderClause = "final_uploaded DESC"
	}

	var records []model.SeedingTorrentRecord
	if err := q.Order(orderClause).Offset(offset).Limit(pageSize).Find(&records).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询种子统计失败")
		return
	}

	type torrentWithTraffic struct {
		model.SeedingTorrentRecord
		LatestUpload int64  `json:"latest_upload"`
		Title        string `json:"title"`
		DetailURL    string `json:"detail_url"`
	}

	type siteInfo struct {
		BaseURL   string
		Framework string
	}
	siteMap := map[string]siteInfo{}
	var siteList []model.Site
	h.db.Select("name, base_url, framework").Find(&siteList)
	for _, s := range siteList {
		siteMap[s.Name] = siteInfo{BaseURL: s.BaseURL, Framework: s.Framework}
	}

	hashes := make([]string, 0, len(records))
	for _, rec := range records {
		hyield := rec.InfoHash
		if len(hyield) > 0 {
			hashes = append(hashes, hyield)
		}
	}

	latestMap := map[string]int64{}
	titleKeyMap := map[string]string{}
	if len(hashes) > 0 {
		type trafficRow struct {
			InfoHash string `json:"info_hash"`
			Uploaded int64  `json:"uploaded"`
		}
		var rows []trafficRow
		h.db.Raw(`SELECT info_hash, MAX(uploaded) as uploaded
			FROM torrent_traffic
			WHERE info_hash IN (?) AND recorded_at >= ?
			GROUP BY info_hash`, hashes, time.Now().Truncate(24*time.Hour)).Scan(&rows)
		for _, row := range rows {
			latestMap[row.InfoHash] = row.Uploaded
		}

		type titleRow struct {
			SiteName  string `json:"site_name"`
			TorrentID string `json:"torrent_id"`
			Title     string `json:"title"`
		}
		var titles []titleRow
		siteTorrents := make([]string, 0, len(records))
		for _, rec := range records {
			siteTorrents = append(siteTorrents, rec.SiteName+"|"+rec.TorrentID)
		}
		h.db.Raw(`SELECT site_name, torrent_id, title FROM rss_torrent_seen WHERE site_name || '|' || torrent_id IN (?)`, siteTorrents).Scan(&titles)
		for _, t := range titles {
			titleKeyMap[t.SiteName+"|"+t.TorrentID] = t.Title
		}
	}

	result := make([]torrentWithTraffic, 0, len(records))
	for _, rec := range records {
		r := torrentWithTraffic{SeedingTorrentRecord: rec}
		if v, ok := latestMap[rec.InfoHash]; ok {
			r.LatestUpload = v
		}
		if t, ok := titleKeyMap[rec.SiteName+"|"+rec.TorrentID]; ok {
			r.Title = t
		}
		if si, ok := siteMap[rec.SiteName]; ok && si.BaseURL != "" && rec.TorrentID != "" {
			r.DetailURL = buildDetailURL(si.BaseURL, si.Framework, rec.TorrentID)
		}
		result = append(result, r)
	}

	Success(w, map[string]interface{}{
		"items": result,
		"total": total,
		"page":  page,
	})
}

func (h *SeedingHandler) handleDownloaderSpeedTrend(w http.ResponseWriter, r *http.Request, clientID string) {
	hours := 24
	if v := r.URL.Query().Get("range"); v == "7d" {
		hours = 168
	} else if v == "30d" {
		hours = 720
	}

	now := time.Now()
	step := hours / 24
	if step < 1 {
		step = 1
	}

	rangeStart := now.Add(time.Duration(-hours) * time.Hour).Truncate(time.Hour)
	rangeEnd := now.Truncate(time.Hour).Add(time.Hour)

	var snapshots []model.DownloaderSpeedSnapshot
	if err := h.db.Where("client_id = ? AND recorded_at >= ? AND recorded_at < ?", clientID, rangeStart, rangeEnd).
		Order("recorded_at ASC").Find(&snapshots).Error; err != nil {
		h.logger.Warn("query speed snapshots failed",
			zap.String("clientID", clientID),
			zap.Error(err))
	}

	bucketMap := make(map[string]model.DownloaderSpeedSnapshot)
	for _, s := range snapshots {
		key := s.RecordedAt.Truncate(time.Duration(step) * time.Hour).Format(time.RFC3339)
		bucketMap[key] = s
	}

	points := make([]map[string]interface{}, 0)
	for i := hours; i >= 0; i -= step {
		t := now.Add(time.Duration(-i) * time.Hour)
		tKey := t.Truncate(time.Duration(step) * time.Hour).Format(time.RFC3339)
		tDisplay := t.Truncate(time.Hour).Format(time.RFC3339)

		point := map[string]interface{}{
			"timestamp":     tDisplay,
			"uploadSpeed":   0,
			"downloadSpeed": 0,
		}
		if s, ok := bucketMap[tKey]; ok {
			point["uploadSpeed"] = s.UploadSpeed
			point["downloadSpeed"] = s.DownloadSpeed
		}
		points = append(points, point)
	}

	Success(w, map[string]interface{}{
		"clientId": clientID,
		"points":   points,
	})
}

func (h *SeedingHandler) handleDryrunAll(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "刷流引擎未初始化")
		return
	}
	ctx := r.Context()
	configs, err := h.engine.ListConfigs(ctx)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询刷流配置失败")
		return
	}

	totalEvaluated := 0
	for _, cfg := range configs {
		count, evalErr := h.engine.DryRunEvaluate(ctx, cfg.ClientID, cfg)
		if evalErr != nil {
			h.logger.Warn("dryrun evaluate failed", zap.String("clientId", cfg.ClientID), zap.Error(evalErr))
			continue
		}
		totalEvaluated += count
	}

	Success(w, map[string]interface{}{
		"candidates":  []interface{}{},
		"total":       totalEvaluated,
		"configCount": len(configs),
	})
}

func (h *SeedingHandler) handleDryrunBySub(w http.ResponseWriter, r *http.Request, subID string) {
	if h.engine == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "刷流引擎未初始化")
		return
	}
	ctx := r.Context()
	var configs []*model.SeedingClientConfig
	if err := h.db.WithContext(ctx).Where("enabled = ?", true).Find(&configs).Error; err != nil {
		h.logger.Warn("query seeding configs for dryrun",
			zap.String("subID", subID),
			zap.Error(err))
	}

	totalEvaluated := 0
	for _, cfg := range configs {
		count, err := h.engine.DryRunEvaluate(ctx, cfg.ClientID, cfg)
		if err != nil {
			continue
		}
		totalEvaluated += count
	}

	Success(w, map[string]interface{}{
		"subscriptionId": subID,
		"candidates":     []interface{}{},
		"total":          totalEvaluated,
	})
}
