package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/coverage"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type PublishTorrentsHandler struct {
	db        *gorm.DB
	coverage  *coverage.Service
	clientMgr MFClientProvider
	logger    *zap.Logger
}

func NewPublishTorrentsHandler(db *gorm.DB, logger *zap.Logger) *PublishTorrentsHandler {
	return &PublishTorrentsHandler{db: db, logger: logger}
}

func (h *PublishTorrentsHandler) SetCoverageService(s *coverage.Service) { h.coverage = s }
func (h *PublishTorrentsHandler) SetClientProvider(c MFClientProvider)  { h.clientMgr = c }

func (h *PublishTorrentsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(r.URL.Path, "/")

	switch {
	case strings.HasSuffix(path, "/publish/torrents") && r.Method == http.MethodGet:
		h.handleListTorrents(w, r)
	case strings.HasSuffix(path, "/publish/torrents/coverage") && r.Method == http.MethodPost:
		h.handleQueryCoverage(w, r)
	default:
		Error(w, http.StatusNotFound, 40400, "接口不存在")
	}
}

func (h *PublishTorrentsHandler) handleListTorrents(w http.ResponseWriter, r *http.Request) {
	if h.clientMgr == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "客户端管理器未初始化")
		return
	}

	clientIDStr := r.URL.Query().Get("client_id")
	if clientIDStr == "" {
		var clients []model.ClientConfig
		h.db.Where("enabled = ?", true).Find(&clients)
		if len(clients) == 0 {
			Success(w, map[string]interface{}{"items": []interface{}{}, "total": 0})
			return
		}
		clientIDStr = fmt.Sprintf("%d", clients[0].ID)
	}

	clientID, err := strconv.ParseUint(clientIDStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的 client_id")
		return
	}

	var cfg model.ClientConfig
	if err := h.db.First(&cfg, clientID).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "下载器不存在")
		return
	}

	client, err := h.clientMgr.Get(cfg.Name)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("连接下载器失败: %v", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	torrents, err := client.GetSeedingTorrents(ctx)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("获取种子列表失败: %v", err))
		return
	}

	var totalSites int64
	h.db.Model(&model.Site{}).Where("enabled = ? AND is_target = ?", true, true).Count(&totalSites)

	infoHashes := make([]string, 0, len(torrents))
	for _, t := range torrents {
		infoHashes = append(infoHashes, t.Hash)
	}

	var coverageMap map[string][]model.SiteCoverageCache
	if h.coverage != nil && len(infoHashes) > 0 {
		coverageMap, err = h.coverage.GetBatchCachedCoverage(ctx, infoHashes)
		if err != nil {
			h.logger.Warn("list torrents: coverage cache read failed", zap.Error(err))
		}
	}
	if coverageMap == nil {
		coverageMap = map[string][]model.SiteCoverageCache{}
	}

	items := make([]map[string]interface{}, 0, len(torrents))
	for _, t := range torrents {
		cov := coverageMap[t.Hash]
		hasCount := 0
		for _, c := range cov {
			if c.Status == model.CoverageConfirmedHas || c.Status == model.CoverageProbablyHas {
				hasCount++
			}
		}
		items = append(items, map[string]interface{}{
			"info_hash": t.Hash,
			"name":      t.Name,
			"size":      t.TotalSize,
			"save_path": t.SavePath,
			"state":     t.State,
			"uploaded":  t.Uploaded,
			"coverage": map[string]interface{}{
				"has_count":    hasCount,
				"total_sites":  totalSites,
				"target_count": int(totalSites) - hasCount,
				"sites":        cov,
			},
		})
	}

	Success(w, map[string]interface{}{
		"items":       items,
		"total":       len(items),
		"total_sites": totalSites,
	})
}

type coverageQueryRequest struct {
	ClientID uint   `json:"client_id"`
	InfoHash string `json:"info_hash"`
}

func (h *PublishTorrentsHandler) handleQueryCoverage(w http.ResponseWriter, r *http.Request) {
	if h.coverage == nil || h.clientMgr == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "覆盖服务未初始化")
		return
	}

	var req coverageQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.InfoHash == "" || req.ClientID == 0 {
		Error(w, http.StatusBadRequest, 40001, "info_hash 和 client_id 必填")
		return
	}

	var cfg model.ClientConfig
	if err := h.db.First(&cfg, req.ClientID).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "下载器不存在")
		return
	}

	client, err := h.clientMgr.Get(cfg.Name)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("连接下载器失败: %v", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// L0: 获取 tracker 列表
	trackers, err := client.GetTrackers(ctx, req.InfoHash)
	if err != nil {
		h.logger.Warn("query coverage: get trackers failed", zap.Error(err))
		trackers = nil
	}

	// 执行四层查询
	results, err := h.coverage.QueryCoverage(ctx, req.InfoHash, trackers)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("覆盖查询失败: %v", err))
		return
	}

	// 统计
	var totalSites int64
	h.db.Model(&model.Site{}).Where("enabled = ? AND is_target = ?", true, true).Count(&totalSites)

	hasCount := 0
	for _, r := range results {
		if r.Status == model.CoverageConfirmedHas || r.Status == model.CoverageProbablyHas {
			hasCount++
		}
	}

	Success(w, map[string]interface{}{
		"info_hash":    req.InfoHash,
		"sites":        results,
		"has_count":    hasCount,
		"total_sites":  totalSites,
		"target_count": int(totalSites) - hasCount,
	})
}
