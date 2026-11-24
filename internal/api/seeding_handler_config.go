package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	dbimpl "github.com/ranfish/pt-forward/internal/db"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

func (h *SeedingHandler) handleListConfigs(w http.ResponseWriter, r *http.Request) {
	var configs []model.SeedingClientConfig
	// §59.13: 统一表后必须过滤 role=seeding，否则 download 配置也会出现在 /seeding 页面
	if err := h.db.WithContext(r.Context()).Where("role = ? OR role = ?", "seeding", "").Find(&configs).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询刷流配置失败")
		return
	}
	Success(w, map[string]interface{}{
		"items": configs,
		"total": len(configs),
	})
}

func (h *SeedingHandler) handleGetConfig(w http.ResponseWriter, r *http.Request, id uint) {
	var config model.SeedingClientConfig
	if err := h.db.WithContext(r.Context()).First(&config, id).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "配置不存在")
		return
	}
	Success(w, config)
}

func (h *SeedingHandler) handleCreateConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientUID                uint    `json:"clientId"`
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
		CleanupMinScore          float64 `json:"cleanupMinScore"`    // §59.122: 评分清理阈值(0=默认0.3)
		CleanupMinAgeHours       float64 `json:"cleanupMinAgeHours"` // §59.122: (0=默认48h)
		ArchiveGranularity       string  `json:"archiveGranularity"`
		RejectRuleIDs            string  `json:"rejectRuleIds"`
		ReannounceBefore         bool    `json:"reannounceBefore"`
		ReannounceRetries        int     `json:"reannounceRetries"`
		ReannounceIntervalMs     int     `json:"reannounceIntervalMs"`
		ReannounceWaitMs         int     `json:"reannounceWaitMs"`
		MinSeedHoursBeforeDelete float64 `json:"minSeedHoursBeforeDelete"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.ClientUID == 0 {
		Error(w, http.StatusBadRequest, 40001, "clientId 为必填项")
		return
	}
	// §55.14 role 校验：/seeding 只接 role=seeding
	var seedClient model.ClientConfig
	if h.db.WithContext(r.Context()).Where("id = ?", req.ClientUID).First(&seedClient).Error == nil {
		if seedClient.Role != "seeding" {
			Error(w, http.StatusBadRequest, 40001, "刷流管理只接受刷流专用(role=seeding)下载器，普通下载器请到下载管理添加")
			return
		}
	}

	var count int64
	if err := h.db.WithContext(r.Context()).Model(&model.SeedingClientConfig{}).Where("client_uid = ?", req.ClientUID).Count(&count).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询刷流配置失败")
		return
	}
	if count > 0 {
		Error(w, http.StatusConflict, 40900, "该下载器已有刷流配置")
		return
	}

	config := model.SeedingClientConfig{
		ClientUID:                req.ClientUID,
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
		CleanupMinScore:          req.CleanupMinScore,
		CleanupMinAgeHours:       req.CleanupMinAgeHours,
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
	auditLog(r, "seeding", "create", "config", fmt.Sprintf("%d", config.ID), fmt.Sprintf("client_uid=%d", config.ClientUID), "success")
	Success(w, config)
}

// normalizeRuleIDs 规则 ID 列表归一：JSON 数组（[1,2]）→ 逗号串（"1,2"）——
// 数组直写 TEXT 列会 500/产生 "1.0" float 形态脏数据；字符串原样。
func normalizeRuleIDs(v interface{}) interface{} {
	if arr, ok := v.([]interface{}); ok {
		parts := make([]string, 0, len(arr))
		for _, item := range arr {
			if f, ok := item.(float64); ok {
				parts = append(parts, fmt.Sprintf("%d", int(f)))
			}
		}
		return strings.Join(parts, ",")
	}
	return v
}

func (h *SeedingHandler) handleUpdateConfig(w http.ResponseWriter, r *http.Request, id uint) {
	var config model.SeedingClientConfig
	if err := h.db.WithContext(r.Context()).First(&config, id).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "配置不存在")
		return
	}

	var req map[string]interface{}
	if !decodeJSONBody(w, r, &req) {
		return
	}

	updates := make(map[string]interface{})
	if v, ok := req["enabled"]; ok {
		updates["enabled"] = v
	}
	if v, ok := req["deleteRuleIds"]; ok {
		updates["delete_rule_ids"] = normalizeRuleIDs(v)
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
	if v, ok := req["cleanupMinScore"]; ok {
		updates["cleanup_min_score"] = v
	}
	if v, ok := req["cleanupMinAgeHours"]; ok {
		updates["cleanup_min_age_hours"] = v
	}
	if v, ok := req["archiveGranularity"]; ok {
		updates["archive_granularity"] = v
	}
	if v, ok := req["rejectRuleIds"]; ok {
		updates["reject_rule_ids"] = normalizeRuleIDs(v)
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

	if err := h.db.WithContext(r.Context()).Model(&config).Updates(updates).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "更新刷流配置失败")
		return
	}
	if err := h.db.WithContext(r.Context()).First(&config, id).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询刷流配置失败")
		return
	}
	auditLog(r, "seeding", "update", "config", fmt.Sprintf("%d", id), fmt.Sprintf("client_uid=%d", config.ClientUID), "success")
	Success(w, config)
}

func (h *SeedingHandler) handleDeleteConfig(w http.ResponseWriter, r *http.Request, id uint) {
	if err := h.db.WithContext(r.Context()).Delete(&model.SeedingClientConfig{}, id).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "删除刷流配置失败")
		return
	}
	auditLog(r, "seeding", "delete", "config", fmt.Sprintf("%d", id), "", "success")
	Success(w, nil)
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
	if err := h.db.WithContext(r.Context()).Where("id = ?", subID).First(&sub).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "订阅不存在")
		return
	}

	if r.Method == http.MethodPut {
		var req map[string]interface{}
		if !decodeJSONBody(w, r, &req) {
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
			if err := h.db.WithContext(r.Context()).Model(&sub).Updates(updates).Error; err != nil {
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
		_ = h.db.Raw("SELECT value FROM system_settings WHERE key = 'seeding.unregistered_keywords' LIMIT 1").Row().Scan(&val) // 空表 → 空串
		var keywords []string
		if val != "" {
			_ = json.Unmarshal([]byte(val), &keywords) // 非法 JSON → 空表回退
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
		if !decodeJSONBody(w, r, &body) {
			return
		}
		data, _ := json.Marshal(body.Keywords)
		h.db.WithContext(r.Context()).Exec("INSERT INTO system_settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
			"seeding.unregistered_keywords", string(data))
		Success(w, map[string]interface{}{"keywords": body.Keywords})
		return
	}
	Error(w, http.StatusMethodNotAllowed, 40500, "method not allowed")
}

func (h *SeedingHandler) handleScoringLogs(w http.ResponseWriter, r *http.Request) {
	// §59.565 D2: limit 补上限（parseLimitQuery 单点·原 <1 回落 20 无上限）
	limit, clamped := parseLimitQuery(r)
	if clamped {
		h.logger.Warn("limit clamped to 200", zap.String("op", "scoring_logs"))
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
	if err := h.db.Select("name, base_url, framework").Find(&siteList).Error; err != nil {
		h.logger.Warn("query failed", zap.Error(err))
	}

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

func (h *SeedingHandler) handleScoringConfigByID(w http.ResponseWriter, r *http.Request, subID string) {
	var sub model.RSSSubscription
	if err := h.db.WithContext(r.Context()).Where("id = ?", subID).First(&sub).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "订阅不存在")
		return
	}

	if r.Method == http.MethodPut {
		var req map[string]interface{}
		if !decodeJSONBody(w, r, &req) {
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
			if err := h.db.WithContext(r.Context()).Model(&sub).Updates(updates).Error; err != nil {
				Error(w, http.StatusInternalServerError, 50000, "保存评分配置失败")
				return
			}
		}
	}

	Success(w, sub.ScoringConfig)
}
