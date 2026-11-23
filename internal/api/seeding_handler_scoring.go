package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/seeding"
	"go.uber.org/zap"
)

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
	if !decodeJSONBody(w, r, &req) {
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
	if err := h.db.Model(&model.SeedingTorrentRecord{}).
		Where("client_uid IN (SELECT id FROM clients WHERE role = 'seeding')").
		Count(&total).Error; err != nil {
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

func (h *SeedingHandler) handleTriggerClient(w http.ResponseWriter, r *http.Request, clientUID uint) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	if h.engine == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "刷流引擎未初始化")
		return
	}

	// §55.15：统一配置加载入口（查 seeding_client_configs 或 download_client_configs），
	// 避免仅查单表导致 role≠seeding 下载器手动触发评估时被误报 "no enabled config"。
	cfg, ok := h.engine.LoadActiveClientConfig(ctx, clientUID)
	if !ok {
		count := h.engine.GetActiveCount(clientUID)
		h.logger.Warn("seeding trigger: no enabled config, returning count only", zap.Uint("clientUid", clientUID))
		Success(w, map[string]interface{}{
			"processedCount": count,
			"clientId":       clientUID,
			"evaluated":      false,
			"reason":         "no enabled config",
		})
		return
	}

	result, err := h.engine.Evaluate(ctx, clientUID, &cfg)
	if err != nil {
		h.logger.Error("seeding evaluate failed", zap.Uint("clientUid", clientUID), zap.Error(err))
		Error(w, http.StatusInternalServerError, 50000, "评估执行失败: "+err.Error())
		return
	}

	h.logger.Info("seeding client evaluated",
		zap.Uint("clientUid", clientUID),
		zap.Int("evaluated", result.Evaluated),
		zap.Int("paused", result.Paused),
		zap.Int("deleted", result.Deleted),
		zap.Int("limited", result.Limited),
		zap.Int("errors", result.Errors),
	)
	auditLog(r, "seeding", "trigger", "client", fmt.Sprintf("%d", clientUID), fmt.Sprintf("evaluated=%d,paused=%d,deleted=%d", result.Evaluated, result.Paused, result.Deleted), "success")
	Success(w, map[string]interface{}{
		"clientId":       clientUID,
		"evaluated":      true,
		"processedCount": result.Evaluated,
		"pausedCount":    result.Paused,
		"deletedCount":   result.Deleted,
		"limitedCount":   result.Limited,
		"errorCount":     result.Errors,
	})
}

func (h *SeedingHandler) handleScoringCycle(w http.ResponseWriter, r *http.Request, cycleID string) {
	Success(w, map[string]interface{}{
		"cycleId": cycleID,
		"items":   []interface{}{},
		"total":   0,
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
		count, evalErr := h.engine.DryRunEvaluate(ctx, cfg.ClientUID, cfg)
		if evalErr != nil {
			h.logger.Warn("dryrun evaluate failed", zap.Uint("client_uid", cfg.ClientUID), zap.Error(evalErr))
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
		count, err := h.engine.DryRunEvaluate(ctx, cfg.ClientUID, cfg)
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
