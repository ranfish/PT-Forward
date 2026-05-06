package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/rss"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type RSSHandler struct {
	repo   *rss.Repository
	engine *rss.Engine
	db     *gorm.DB
	logger *zap.Logger
}

func NewRSSHandler(repo *rss.Repository, engine *rss.Engine, db *gorm.DB, logger *zap.Logger) *RSSHandler {
	return &RSSHandler{repo: repo, engine: engine, db: db, logger: logger}
}

type createRSSRequest struct {
	Name     string   `json:"name"`
	Enabled  bool     `json:"enabled"`
	URLs     []string `json:"urls"`
	SiteName string   `json:"siteName"`
	Cron     string   `json:"cron,omitempty"`

	ClientID  string `json:"clientId,omitempty"`
	SavePath  string `json:"savePath,omitempty"`
	Category  string `json:"category,omitempty"`
	AddPaused bool   `json:"addPaused"`
	AutoTMM   bool   `json:"autoTmm"`

	UploadLimitKB   int64 `json:"uploadLimitKb,omitempty"`
	DownloadLimitKB int64 `json:"downloadLimitKb,omitempty"`

	Tags     []string `json:"tags,omitempty"`
	IsSource bool     `json:"isSource"`
	IsTarget bool     `json:"isTarget"`

	ScrapeFree bool `json:"scrapeFree"`
	ScrapeHR   bool `json:"scrapeHr"`

	PushNotify bool   `json:"pushNotify"`
	NotifyID   string `json:"notifyId,omitempty"`

	PublishEnabled bool     `json:"publishEnabled"`
	PublishTargets []string `json:"publishTargets,omitempty"`

	AutoReseed      bool     `json:"autoReseed"`
	ReseedClientIDs []string `json:"reseedClientIds,omitempty"`
}

type updateRSSRequest struct {
	Name     *string   `json:"name,omitempty"`
	Enabled  *bool     `json:"enabled,omitempty"`
	URLs     *[]string `json:"urls,omitempty"`
	SiteName *string   `json:"siteName,omitempty"`
	Cron     *string   `json:"cron,omitempty"`

	ClientID  *string `json:"clientId,omitempty"`
	SavePath  *string `json:"savePath,omitempty"`
	Category  *string `json:"category,omitempty"`
	AddPaused *bool   `json:"addPaused,omitempty"`
	AutoTMM   *bool   `json:"autoTmm,omitempty"`

	UploadLimitKB   *int64 `json:"uploadLimitKb,omitempty"`
	DownloadLimitKB *int64 `json:"downloadLimitKb,omitempty"`

	Tags *[]string `json:"tags,omitempty"`

	ScrapeFree *bool `json:"scrapeFree,omitempty"`
	ScrapeHR   *bool `json:"scrapeHr,omitempty"`

	PushNotify *bool   `json:"pushNotify,omitempty"`
	NotifyID   *string `json:"notifyId,omitempty"`

	PublishEnabled *bool     `json:"publishEnabled,omitempty"`
	PublishTargets *[]string `json:"publishTargets,omitempty"`

	AutoReseed      *bool     `json:"autoReseed,omitempty"`
	ReseedClientIDs *[]string `json:"reseedClientIds,omitempty"`
}

type rssResponse struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	URLs     []string `json:"urls"`
	SiteName string   `json:"siteName"`
	Cron     string   `json:"cron"`

	ClientID  string `json:"clientId,omitempty"`
	SavePath  string `json:"savePath,omitempty"`
	Category  string `json:"category,omitempty"`
	AddPaused bool   `json:"addPaused"`
	AutoTMM   bool   `json:"autoTmm"`

	UploadLimitKB   int64 `json:"uploadLimitKb,omitempty"`
	DownloadLimitKB int64 `json:"downloadLimitKb,omitempty"`

	Tags []string `json:"tags,omitempty"`

	ScrapeFree bool `json:"scrapeFree"`
	ScrapeHR   bool `json:"scrapeHr"`

	PushNotify bool   `json:"pushNotify"`
	NotifyID   string `json:"notifyId,omitempty"`

	PublishEnabled bool     `json:"publishEnabled"`
	PublishTargets []string `json:"publishTargets,omitempty"`

	AutoReseed      bool     `json:"autoReseed"`
	ReseedClientIDs []string `json:"reseedClientIds,omitempty"`

	SkipSameSize    bool `json:"skipSameSize"`
	AddCountPerHour int  `json:"addCountPerHour"`
}

func (h *RSSHandler) toResponse(s *model.RSSSubscription) rssResponse {
	return rssResponse{
		ID:        s.ID,
		Name:      s.Name,
		Enabled:   s.Enabled,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,

		URLs:     s.URLs,
		SiteName: s.SiteName,
		Cron:     s.Cron,

		ClientID:  s.ClientID,
		SavePath:  s.SavePath,
		Category:  s.Category,
		AddPaused: s.AddPaused,
		AutoTMM:   s.AutoTMM,

		UploadLimitKB:   s.UploadLimitKB,
		DownloadLimitKB: s.DownloadLimitKB,

		Tags: s.Tags,

		ScrapeFree: s.ScrapeFree,
		ScrapeHR:   s.ScrapeHR,

		PushNotify: s.PushNotify,
		NotifyID:   s.NotifyID,

		PublishEnabled: s.PublishEnabled,
		PublishTargets: s.PublishTargets,

		AutoReseed:      s.AutoReseed,
		ReseedClientIDs: s.ReseedClientIDs,

		SkipSameSize:    s.SkipSameSize,
		AddCountPerHour: s.AddCountPerHour,
	}
}

func (h *RSSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handleRouteByPath(w, r)
}

func (h *RSSHandler) handleRouteByPath(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimRight(path, "/")

	if trimmed == "/api/v1/rss/subscriptions" {
		if r.Method == http.MethodGet {
			h.handleList(w, r)
			return
		}
		if r.Method == http.MethodPost {
			h.handleCreate(w, r)
			return
		}
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	remaining := strings.TrimPrefix(trimmed, "/api/v1/rss/subscriptions/")
	if remaining == "" || remaining == "/" {
		if r.Method == http.MethodGet {
			h.handleList(w, r)
		} else if r.Method == http.MethodPost {
			h.handleCreate(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	remaining = strings.TrimPrefix(path, "/api/v1/rss/subscriptions/")
	remaining = strings.TrimRight(remaining, "/")
	parts := strings.SplitN(remaining, "/", 3)

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			h.handleGet(w, r)
		case http.MethodPut:
			h.handleUpdate(w, r)
		case http.MethodDelete:
			h.handleDelete(w, r)
		default:
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	idStr := parts[0]
	subResource := parts[1]

	switch subResource {
	case "trigger":
		if r.Method == http.MethodPost {
			h.handleTrigger(w, r, idStr)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
	case "pause":
		if r.Method == http.MethodPost {
			h.handleSetPause(w, r, idStr, true)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
	case "resume":
		if r.Method == http.MethodPost {
			h.handleSetPause(w, r, idStr, false)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
	case "dryrun":
		if r.Method == http.MethodPost {
			h.handleDryrun(w, r, idStr)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
	case "rules":
		if r.Method == http.MethodPut {
			h.handleUpdateRules(w, r, idStr)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
	default:
		Error(w, http.StatusNotFound, 40400, "路径不存在")
	}
}

func (h *RSSHandler) handleList(w http.ResponseWriter, r *http.Request) {
	page, size := parsePagination(r)

	var total int64
	h.db.Model(&model.RSSSubscription{}).
		Where("deleted_at = ?", time.Time{}).
		Count(&total)

	var subs []model.RSSSubscription
	h.db.Where("deleted_at = ?", time.Time{}).
		Order("name ASC").
		Offset(offset(page, size)).Limit(size).
		Find(&subs)

	items := make([]rssResponse, 0, len(subs))
	for i := range subs {
		items = append(items, h.toResponse(&subs[i]))
	}

	Success(w, PaginatedResult{
		Items: items,
		Total: total,
		Page:  page,
		Size:  size,
	})
}

func (h *RSSHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createRSSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	if req.Name == "" || len(req.URLs) == 0 || req.SiteName == "" {
		Error(w, http.StatusBadRequest, 40001, "name, urls, siteName 为必填项")
		return
	}

	exists, _ := h.repo.ExistsByName(r.Context(), req.Name, 0)
	if exists {
		Error(w, http.StatusConflict, 40900, "订阅名称已存在")
		return
	}

	var site model.Site
	if err := h.db.Where("name = ? OR domain = ?", req.SiteName, req.SiteName).First(&site).Error; err != nil {
		Error(w, http.StatusBadRequest, 13001, "关联站点不存在，请先创建站点")
		return
	}

	cron := req.Cron
	if cron == "" {
		cron = "*/5 * * * *"
	}

	sub := model.RSSSubscription{
		Name:     req.Name,
		Enabled:  req.Enabled,
		URLs:     req.URLs,
		SiteName: req.SiteName,
		Cron:     cron,

		ClientID:  req.ClientID,
		SavePath:  req.SavePath,
		Category:  req.Category,
		AddPaused: req.AddPaused,
		AutoTMM:   req.AutoTMM,

		UploadLimitKB:   req.UploadLimitKB,
		DownloadLimitKB: req.DownloadLimitKB,

		Tags: req.Tags,

		ScrapeFree: req.ScrapeFree,
		ScrapeHR:   req.ScrapeHR,

		PushNotify: req.PushNotify,
		NotifyID:   req.NotifyID,

		PublishEnabled: req.PublishEnabled,
		PublishTargets: req.PublishTargets,

		AutoReseed:      req.AutoReseed,
		ReseedClientIDs: req.ReseedClientIDs,
	}

	if err := h.repo.Create(r.Context(), &sub); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "创建订阅失败")
		return
	}

	h.logger.Info("rss subscription created", zap.String("name", sub.Name), zap.String("site", sub.SiteName))
	Success(w, h.toResponse(&sub))
}

func (h *RSSHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := h.extractID(r.URL.Path, "/api/v1/rss/subscriptions/")
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的订阅 ID")
		return
	}

	sub, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, 13002, "订阅不存在")
		return
	}

	Success(w, h.toResponse(sub))
}

func (h *RSSHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := h.extractID(r.URL.Path, "/api/v1/rss/subscriptions/")
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的订阅 ID")
		return
	}

	sub, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, 13002, "订阅不存在")
		return
	}

	var req updateRSSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	if req.Name != nil {
		exists, _ := h.repo.ExistsByName(r.Context(), *req.Name, id)
		if exists {
			Error(w, http.StatusConflict, 40900, "订阅名称已存在")
			return
		}
		sub.Name = *req.Name
	}
	if req.Enabled != nil {
		sub.Enabled = *req.Enabled
	}
	if req.URLs != nil {
		sub.URLs = *req.URLs
	}
	if req.SiteName != nil {
		sub.SiteName = *req.SiteName
	}
	if req.Cron != nil {
		sub.Cron = *req.Cron
	}
	if req.ClientID != nil {
		sub.ClientID = *req.ClientID
	}
	if req.SavePath != nil {
		sub.SavePath = *req.SavePath
	}
	if req.Category != nil {
		sub.Category = *req.Category
	}
	if req.AddPaused != nil {
		sub.AddPaused = *req.AddPaused
	}
	if req.AutoTMM != nil {
		sub.AutoTMM = *req.AutoTMM
	}
	if req.UploadLimitKB != nil {
		sub.UploadLimitKB = *req.UploadLimitKB
	}
	if req.DownloadLimitKB != nil {
		sub.DownloadLimitKB = *req.DownloadLimitKB
	}
	if req.Tags != nil {
		sub.Tags = *req.Tags
	}
	if req.ScrapeFree != nil {
		sub.ScrapeFree = *req.ScrapeFree
	}
	if req.ScrapeHR != nil {
		sub.ScrapeHR = *req.ScrapeHR
	}
	if req.PushNotify != nil {
		sub.PushNotify = *req.PushNotify
	}
	if req.NotifyID != nil {
		sub.NotifyID = *req.NotifyID
	}
	if req.PublishEnabled != nil {
		sub.PublishEnabled = *req.PublishEnabled
	}
	if req.PublishTargets != nil {
		sub.PublishTargets = *req.PublishTargets
	}
	if req.AutoReseed != nil {
		sub.AutoReseed = *req.AutoReseed
	}
	if req.ReseedClientIDs != nil {
		sub.ReseedClientIDs = *req.ReseedClientIDs
	}

	if err := h.repo.Update(r.Context(), sub); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "更新订阅失败")
		return
	}

	h.logger.Info("rss subscription updated", zap.String("name", sub.Name))
	Success(w, h.toResponse(sub))
}

func (h *RSSHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := h.extractID(r.URL.Path, "/api/v1/rss/subscriptions/")
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的订阅 ID")
		return
	}

	_, err = h.repo.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, 13002, "订阅不存在")
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "删除订阅失败")
		return
	}

	h.logger.Info("rss subscription deleted", zap.Uint("id", id))
	Success(w, nil)
}

func (h *RSSHandler) handleTrigger(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的订阅 ID")
		return
	}

	sub, err := h.repo.GetByID(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, 13002, "订阅不存在")
		return
	}

	if !sub.Enabled {
		Error(w, http.StatusBadRequest, 13003, "订阅已禁用，无法触发")
		return
	}

	if e := h.engine.Trigger(r.Context(), uint(id)); e != nil {
		writeAppError(w, e)
		return
	}

	Success(w, map[string]interface{}{
		"triggered": true,
		"message":   "RSS 抓取已触发（后台执行中）",
	})
}

func (h *RSSHandler) handleSetPause(w http.ResponseWriter, r *http.Request, idStr string, paused bool) {
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的订阅 ID")
		return
	}

	sub, err := h.repo.GetByID(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, 13002, "订阅不存在")
		return
	}

	sub.Paused = paused
	if paused {
		now := time.Now()
		sub.PausedAt = &now
		sub.PauseReason = "manual"
	} else {
		sub.PausedAt = nil
		sub.PauseReason = ""
	}

	if err := h.repo.Update(r.Context(), sub); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "更新订阅状态失败")
		return
	}

	action := "已恢复"
	if paused {
		action = "已暂停"
	}
	h.logger.Info("rss subscription "+action, zap.String("name", sub.Name))
	Success(w, map[string]interface{}{
		"paused": paused,
	})
}

func (h *RSSHandler) extractID(path string, prefix string) (uint, error) {
	p := strings.TrimPrefix(path, prefix)
	p = strings.TrimRight(p, "/")
	n, err := strconv.ParseUint(p, 10, 32)
	if err != nil {
		return 0, apiError(ErrAPIInvalidID, "invalid id", nil)
	}
	return uint(n), nil
}

func (h *RSSHandler) handleDryrun(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的订阅 ID")
		return
	}

	sub, err := h.repo.GetByID(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, 13002, "订阅不存在")
		return
	}

	seen, err := h.repo.ListSeenBySite(r.Context(), sub.SiteName, time.Time{})
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询种子记录失败")
		return
	}

	Success(w, map[string]interface{}{
		"subscription":   h.toResponse(sub),
		"recentTorrents": seen,
		"total":          len(seen),
	})
}

func (h *RSSHandler) handleUpdateRules(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的订阅 ID")
		return
	}

	var req struct {
		AcceptRuleIDs []uint `json:"acceptRuleIds"`
		RejectRuleIDs []uint `json:"rejectRuleIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	sub, err := h.repo.GetByID(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, 13002, "订阅不存在")
		return
	}

	sub.AcceptRuleIDs = req.AcceptRuleIDs
	sub.RejectRuleIDs = req.RejectRuleIDs
	if err := h.repo.Update(r.Context(), sub); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "更新规则失败")
		return
	}

	h.logger.Info("subscription rules updated", zap.String("id", idStr))
	Success(w, map[string]interface{}{
		"acceptRuleIds": req.AcceptRuleIDs,
		"rejectRuleIds": req.RejectRuleIDs,
	})
}
