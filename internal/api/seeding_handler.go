package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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
		if r.Method == http.MethodGet {
			h.handleListConfigs(w, r)
		} else if r.Method == http.MethodPost {
			h.handleCreateConfig(w, r)
		} else {
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

	case trimmed == "/api/v1/seeding/rules" || trimmed == "/api/v1/seeding/rules/":
		h.handleRulesRoot(w, r)
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

	if strings.Contains(trimmed, "/seeding/rules/") {
		remaining := extractLastSegment(trimmed, "/api/v1/seeding/rules/")
		parts := strings.SplitN(remaining, "/", 2)
		id, err := strconv.ParseUint(parts[0], 10, 64)
		if err != nil {
			Error(w, http.StatusBadRequest, 40001, "无效的规则 ID")
			return
		}
		if len(parts) == 2 && parts[1] == "test" && r.Method == http.MethodPost {
			h.handleTestRule(w, r, uint(id))
			return
		}
		switch r.Method {
		case http.MethodGet:
			h.handleGetRule(w, r, uint(id))
		case http.MethodPut:
			h.handleUpdateRule(w, r, uint(id))
		case http.MethodDelete:
			h.handleDeleteRule(w, r, uint(id))
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
		if r.Method == http.MethodGet {
			h.handleScoringConfigByID(w, r, remaining)
		} else if r.Method == http.MethodPut {
			h.handleScoringConfigByID(w, r, remaining)
		} else {
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
		ClientID            string  `json:"clientId"`
		Enabled             bool    `json:"enabled"`
		DeleteRuleIDs       string  `json:"deleteRuleIds"`
		AutoDeleteCron      string  `json:"autoDeleteCron"`
		MainDataCron        string  `json:"mainDataCron"`
		DiskProtectEnabled  bool    `json:"diskProtectEnabled"`
		MinDiskSpaceGB      float64 `json:"minDiskSpaceGB"`
		MaxActiveUploads    int     `json:"maxActiveUploads"`
		MaxActiveDownloads  int     `json:"maxActiveDownloads"`
		SuperSeedingDefault bool    `json:"superSeedingDefault"`
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
	h.db.Model(&model.SeedingClientConfig{}).Where("client_id = ?", req.ClientID).Count(&count)
	if count > 0 {
		Error(w, http.StatusConflict, 40900, "该下载器已有刷流配置")
		return
	}

	config := model.SeedingClientConfig{
		ClientID:            req.ClientID,
		Enabled:             req.Enabled,
		DeleteRuleIDs:       req.DeleteRuleIDs,
		AutoDeleteCron:      req.AutoDeleteCron,
		MainDataCron:        req.MainDataCron,
		DiskProtectEnabled:  req.DiskProtectEnabled,
		MinDiskSpaceGB:      req.MinDiskSpaceGB,
		MaxActiveUploads:    req.MaxActiveUploads,
		MaxActiveDownloads:  req.MaxActiveDownloads,
		SuperSeedingDefault: req.SuperSeedingDefault,
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

	if err := h.db.Create(&config).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "创建刷流配置失败")
		return
	}
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
		updates["maindata_cron"] = v
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
	if v, ok := req["superSeedingDefault"]; ok {
		updates["super_seeding_default"] = v
	}
	updates["updated_at"] = time.Now()

	if err := h.db.Model(&config).Updates(updates).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "更新刷流配置失败")
		return
	}
	h.db.First(&config, id)
	Success(w, config)
}

func (h *SeedingHandler) handleDeleteConfig(w http.ResponseWriter, _ *http.Request, id uint) {
	if err := h.db.Delete(&model.SeedingClientConfig{}, id).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "删除刷流配置失败")
		return
	}
	Success(w, nil)
}

func (h *SeedingHandler) handleListRecords(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("clientId")

	if h.engine != nil && clientID != "" {
		records, err := h.engine.ListByClient(r.Context(), clientID)
		if err != nil {
			Error(w, http.StatusInternalServerError, 50000, "查询刷流记录失败")
			return
		}
		Success(w, map[string]interface{}{
			"items": records,
			"total": len(records),
		})
		return
	}

	q := h.db.Model(&model.SeedingTorrentRecord{}).
		Where("status IN ?", []string{"seeding", "paused_free_end", "paused_rule"})
	if clientID != "" {
		q = q.Where("client_id = ?", clientID)
	}

	var records []model.SeedingTorrentRecord
	var total int64
	q.Count(&total)
	q.Order("updated_at DESC").Find(&records)

	Success(w, map[string]interface{}{
		"items": records,
		"total": total,
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
	var totalActive int64
	h.db.Model(&model.SeedingTorrentRecord{}).Where("status = ?", "seeding").Count(&totalActive)

	var totalPaused int64
	h.db.Model(&model.SeedingTorrentRecord{}).Where("status IN ?", []string{"paused_free_end", "paused_rule"}).Count(&totalPaused)

	activeCount := 0
	if h.engine != nil {
		activeCount = h.engine.TotalActiveCount()
	}

	Success(w, map[string]interface{}{
		"activeRecords": activeCount,
		"dbActiveCount": totalActive,
		"dbPausedCount": totalPaused,
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

	if h.engine != nil {
		if err := h.engine.UpdateStatus(r.Context(), uint(id), model.SeedingStatusSeeding, "manual_resume"); err != nil {
			Error(w, http.StatusInternalServerError, 50000, "恢复记录失败")
			return
		}
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

	if h.engine != nil {
		if err := h.engine.UpdateStatus(r.Context(), uint(id), model.SeedingStatusPausedFreeEnd, "manual_pause"); err != nil {
			Error(w, http.StatusInternalServerError, 50000, "暂停记录失败")
			return
		}
	}

	h.logger.Info("seeding record paused", zap.String("id", idStr))
	Success(w, map[string]interface{}{"message": "已暂停"})
}

func (h *SeedingHandler) handleEngineStatus(w http.ResponseWriter, _ *http.Request) {
	activeCount := 0
	if h.engine != nil {
		activeCount = h.engine.TotalActiveCount()
	}

	var totalActive int64
	h.db.Model(&model.SeedingTorrentRecord{}).Where("status = ?", "seeding").Count(&totalActive)

	var totalPaused int64
	h.db.Model(&model.SeedingTorrentRecord{}).Where("status IN ?", []string{"paused_free_end", "paused_rule"}).Count(&totalPaused)

	Success(w, map[string]interface{}{
		"running":       true,
		"uptimeSeconds": time.Since(startTime).Seconds(),
		"clients": map[string]interface{}{
			"online": activeCount,
			"total":  activeCount,
		},
		"overview": map[string]interface{}{
			"totalTorrents":  totalActive + totalPaused,
			"activeTorrents": totalActive,
			"pausedTorrents": totalPaused,
		},
	})
}

func (h *SeedingHandler) handleRulesRoot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleListRules(w, r)
	case http.MethodPost:
		h.handleCreateRule(w, r)
	default:
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
	}
}

func (h *SeedingHandler) handleListRules(w http.ResponseWriter, _ *http.Request) {
	var rules []model.DeleteRule
	h.db.Order("priority ASC").Find(&rules)
	Success(w, map[string]interface{}{
		"items": rules,
		"total": len(rules),
	})
}

func (h *SeedingHandler) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Alias           string `json:"alias"`
		Priority        int    `json:"priority"`
		Enabled         bool   `json:"enabled"`
		Type            string `json:"type"`
		Conditions      string `json:"conditions"`
		Expr            string `json:"expr"`
		Action          string `json:"action"`
		CascadeDelete   bool   `json:"cascade_delete"`
		CascadeMaxDepth int    `json:"cascade_max_depth"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.Alias == "" {
		Error(w, http.StatusBadRequest, 40001, "alias 为必填项")
		return
	}
	if req.Type == "" {
		req.Type = "normal"
	}
	if req.Action == "" {
		req.Action = "delete"
	}

	if req.Type == "expr" && req.Expr != "" {
		if err := seeding.ValidateExpr(req.Expr); err != nil {
			Error(w, http.StatusBadRequest, 14003, "表达式语法错误: "+err.Error())
			return
		}
	}

	rule := model.DeleteRule{
		Alias:           req.Alias,
		Priority:        req.Priority,
		Enabled:         req.Enabled,
		Type:            req.Type,
		Conditions:      req.Conditions,
		Expr:            req.Expr,
		Action:          req.Action,
		CascadeDelete:   req.CascadeDelete,
		CascadeMaxDepth: req.CascadeMaxDepth,
	}
	if err := h.db.Create(&rule).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "创建规则失败")
		return
	}
	Success(w, rule)
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
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int64
	q.Count(&total)

	var records []model.SeedingTorrentRecord
	q.Order("updated_at DESC").Offset(offset).Limit(pageSize).Find(&records)

	Success(w, map[string]interface{}{
		"items": records,
		"total": total,
		"page":  page,
	})
}

func (h *SeedingHandler) handleListClients(w http.ResponseWriter, _ *http.Request) {
	var configs []model.SeedingClientConfig
	h.db.Find(&configs)
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
		if v, ok := req["enabled"].(bool); ok {
			sub.ScoringConfig.Enabled = v
		}
		if v, ok := req["halfLifeHours"].(float64); ok {
			sub.ScoringConfig.HalfLifeHours = v
		}
		if v, ok := req["minScore"].(float64); ok {
			sub.ScoringConfig.MinScore = v
		}
		if err := h.db.Save(&sub).Error; err != nil {
			Error(w, http.StatusInternalServerError, 50000, "保存评分配置失败")
			return
		}
	}

	Success(w, sub.ScoringConfig)
}

func (h *SeedingHandler) handleScoringLogs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 20
	}

	var logs []model.ScoringLog
	h.db.Order("created_at DESC").Limit(limit).Find(&logs)

	Success(w, map[string]interface{}{
		"items": logs,
		"total": len(logs),
	})
}

func (h *SeedingHandler) handleGetRule(w http.ResponseWriter, _ *http.Request, id uint) {
	var rule model.DeleteRule
	if err := h.db.First(&rule, id).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "规则不存在")
		return
	}
	Success(w, rule)
}

func (h *SeedingHandler) handleUpdateRule(w http.ResponseWriter, r *http.Request, id uint) {
	var rule model.DeleteRule
	if err := h.db.First(&rule, id).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "规则不存在")
		return
	}

	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	updates := make(map[string]interface{})
	for _, field := range []string{"alias", "priority", "enabled", "type", "conditions", "action", "expr", "cascade_delete", "cascade_max_depth"} {
		if v, ok := req[field]; ok {
			updates[field] = v
		}
	}

	if exprVal, ok := req["expr"].(string); ok && exprVal != "" {
		if typeVal, _ := req["type"].(string); typeVal == "expr" || rule.Type == "expr" {
			if err := seeding.ValidateExpr(exprVal); err != nil {
				Error(w, http.StatusBadRequest, 14003, "表达式语法错误: "+err.Error())
				return
			}
		}
	}

	updates["updated_at"] = time.Now()

	if len(updates) > 1 {
		h.db.Model(&rule).Updates(updates)
	}
	h.db.First(&rule, id)
	Success(w, rule)
}

func (h *SeedingHandler) handleDeleteRule(w http.ResponseWriter, _ *http.Request, id uint) {
	if err := h.db.Delete(&model.DeleteRule{}, id).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "删除规则失败")
		return
	}
	Success(w, nil)
}

func (h *SeedingHandler) handleTestRule(w http.ResponseWriter, r *http.Request, id uint) {
	var rule model.DeleteRule
	if err := h.db.First(&rule, id).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "规则不存在")
		return
	}

	var req struct {
		TorrentName string `json:"torrentName"`
		Size        int64  `json:"size"`
		Seeders     int    `json:"seeders"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	matched := false
	reason := "未匹配"

	if rule.Type == "expr" && rule.Expr != "" {
		if err := seeding.ValidateExpr(rule.Expr); err != nil {
			reason = "表达式语法错误: " + err.Error()
		} else {
			rc := &seeding.RuleContext{
				Record: &model.SeedingTorrentRecord{
					SiteName: "test",
					Status:   model.SeedingStatusSeeding,
					Discount: model.DiscountNone,
					HasHR:    false,
					IsFree:   false,
				},
				Torrent: &model.TorrentInfo{
					Name:        req.TorrentName,
					TotalSize:   req.Size,
					NumComplete: req.Seeders,
				},
				FreeSpace: -1,
				Now:       time.Now(),
			}
			ok, err := seeding.EvalExprForTest(rule.Expr, rc)
			if err != nil {
				reason = "表达式求值错误: " + err.Error()
			} else if ok {
				matched = true
				reason = "表达式匹配: " + rule.Expr
			}
		}
	}

	if !matched && rule.Conditions != "" {
		rc := &seeding.RuleContext{
			Record: &model.SeedingTorrentRecord{
				SiteName:  "test",
				Status:    model.SeedingStatusSeeding,
				Discount:  model.DiscountNone,
				IsFree:    false,
				HasHR:     false,
				ClientID:  "test",
				TorrentID: "test",
			},
			Torrent: &model.TorrentInfo{
				Name:        req.TorrentName,
				TotalSize:   req.Size,
				NumComplete: req.Seeders,
			},
			FreeSpace: -1,
			Now:       time.Now(),
		}
		conditions := seeding.ParseConditions(rule.Conditions)
		if seeding.MatchContext(rc, conditions) {
			matched = true
			reason = fmt.Sprintf("条件匹配: %d 个条件", len(conditions))
		}
	}

	Success(w, map[string]interface{}{
		"matched": matched,
		"reason":  reason,
		"ruleId":  id,
	})
}

func (h *SeedingHandler) handleTriggerClient(w http.ResponseWriter, _ *http.Request, clientID string) {
	var count int64
	h.db.Model(&model.SeedingTorrentRecord{}).Where("client_id = ? AND status = ?", clientID, "seeding").Count(&count)
	h.logger.Info("seeding client triggered", zap.String("clientId", clientID), zap.Int64("activeCount", count))
	Success(w, map[string]interface{}{
		"processedCount": count,
		"clientId":       clientID,
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
		if v, ok := req["enabled"].(bool); ok {
			sub.ScoringConfig.Enabled = v
		}
		if v, ok := req["halfLifeHours"].(float64); ok {
			sub.ScoringConfig.HalfLifeHours = v
		}
		if v, ok := req["minScore"].(float64); ok {
			sub.ScoringConfig.MinScore = v
		}
		h.db.Save(&sub)
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
	var totalActive int64
	h.db.Model(&model.SeedingTorrentRecord{}).Where("status = ?", "seeding").Count(&totalActive)

	var totalPaused int64
	h.db.Model(&model.SeedingTorrentRecord{}).Where("status IN ?", []string{"paused_free_end", "paused_rule"}).Count(&totalPaused)

	var total int64
	h.db.Model(&model.SeedingTorrentRecord{}).Count(&total)

	today := time.Now().Truncate(24 * time.Hour)
	var todayAdded int64
	h.db.Model(&model.SeedingTorrentRecord{}).Where("created_at >= ?", today).Count(&todayAdded)

	var todayDeleted int64
	h.db.Model(&model.SeedingTorrentRecord{}).Where("status = ? AND updated_at >= ?", "deleted", today).Count(&todayDeleted)

	var trafficResult []struct {
		TotalUpload   int64
		TotalDownload int64
	}
	h.db.Model(&model.SiteTrafficDaily{}).
		Select("COALESCE(SUM(upload_delta), 0) as total_upload, COALESCE(SUM(download_delta), 0) as total_download").
		Scan(&trafficResult)

	totalUpload := int64(0)
	totalDownload := int64(0)
	if len(trafficResult) > 0 {
		totalUpload = trafficResult[0].TotalUpload
		totalDownload = trafficResult[0].TotalDownload
	}

	var globalRatio float64
	if totalDownload > 0 {
		globalRatio = float64(totalUpload) / float64(totalDownload)
	} else if totalUpload > 0 {
		globalRatio = -1
	}

	Success(w, map[string]interface{}{
		"totalTorrents":      total,
		"activeTorrents":     totalActive,
		"pausedTorrents":     totalPaused,
		"totalUploadBytes":   totalUpload,
		"totalDownloadBytes": totalDownload,
		"globalRatio":        globalRatio,
		"todayDeleted":       todayDeleted,
		"todayAdded":         todayAdded,
	})
}

func (h *SeedingHandler) handleStatsBySite(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 20
	}

	type siteStat struct {
		SiteName string `json:"siteName"`
		Count    int64  `json:"count"`
	}

	var stats []siteStat
	h.db.Model(&model.SeedingTorrentRecord{}).
		Select("site_name, count(*) as count").
		Where("status = ?", "seeding").
		Group("site_name").
		Order("count DESC").
		Limit(limit).
		Find(&stats)

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
		h.db.Model(&model.SeedingTorrentRecord{}).
			Where("site_name = ? AND created_at >= ? AND created_at < ?", site, dayStart, dayEnd).
			Count(&count)

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
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	q := h.db.Model(&model.SeedingTorrentRecord{}).
		Where("status = ?", "seeding")

	var total int64
	q.Count(&total)

	var records []model.SeedingTorrentRecord
	q.Order("created_at ASC").Offset(offset).Limit(pageSize).Find(&records)

	Success(w, map[string]interface{}{
		"items": records,
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
	points := make([]map[string]interface{}, 0)

	step := hours / 24
	if step < 1 {
		step = 1
	}

	for i := hours; i >= 0; i -= step {
		t := now.Add(time.Duration(-i) * time.Hour)
		tStart := t.Truncate(time.Hour)

		var snapshot model.DownloaderSpeedSnapshot
		err := h.db.Where("client_id = ? AND recorded_at >= ? AND recorded_at < ?", clientID, tStart, tStart.Add(time.Hour)).
			Order("recorded_at DESC").First(&snapshot).Error

		point := map[string]interface{}{
			"timestamp":     tStart.Format(time.RFC3339),
			"uploadSpeed":   0,
			"downloadSpeed": 0,
		}
		if err == nil {
			point["uploadSpeed"] = snapshot.UploadSpeed
			point["downloadSpeed"] = snapshot.DownloadSpeed
		}
		points = append(points, point)
	}

	Success(w, map[string]interface{}{
		"clientId": clientID,
		"points":   points,
	})
}

func (h *SeedingHandler) handleDryrunAll(w http.ResponseWriter, r *http.Request) {
	Success(w, map[string]interface{}{
		"candidates": []interface{}{},
		"total":      0,
	})
}

func (h *SeedingHandler) handleDryrunBySub(w http.ResponseWriter, r *http.Request, subID string) {
	Success(w, map[string]interface{}{
		"subscriptionId": subID,
		"candidates":     []interface{}{},
		"total":          0,
	})
}
