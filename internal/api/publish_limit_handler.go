package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type PublishLimitHandler struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewPublishLimitHandler(db *gorm.DB, logger *zap.Logger) *PublishLimitHandler {
	return &PublishLimitHandler{db: db, logger: logger}
}

func (h *PublishLimitHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(r.URL.Path, "/")
	switch {
	case strings.HasSuffix(path, "/publish/limits") && r.Method == http.MethodGet:
		h.handleList(w, r)
	case strings.HasSuffix(path, "/publish/limits") && r.Method == http.MethodPut:
		h.handleUpsert(w, r)
	case strings.Contains(path, "/publish/limits/") && r.Method == http.MethodDelete:
		h.handleDelete(w, r)
	case strings.Contains(path, "/publish/limits/") && r.Method == http.MethodGet:
		h.handleGet(w, r)
	default:
		Error(w, http.StatusNotFound, 40400, "接口不存在")
	}
}

func (h *PublishLimitHandler) handleList(w http.ResponseWriter, r *http.Request) {
	var limits []model.SitePublishLimit
	if err := h.db.WithContext(r.Context()).Find(&limits).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询发布限制失败")
		return
	}
	type limitWithCount struct {
		model.SitePublishLimit
		CurrentCount int64 `json:"current_count"`
	}
	items := make([]limitWithCount, 0, len(limits))
	for _, l := range limits {
		item := limitWithCount{SitePublishLimit: l}
		if l.Enabled && l.WindowHours > 0 {
			cutoff := time.Now().Add(time.Duration(-l.WindowHours) * time.Hour)
			h.db.WithContext(r.Context()).Model(&model.PublishResultRecord{}).
				Where("target_site = ? AND created_at >= ? AND status IN ?",
					l.SiteName, cutoff,
					[]string{"completed", "exists", "edited"}).
				Count(&item.CurrentCount)
		}
		items = append(items, item)
	}
	Success(w, map[string]interface{}{"items": items, "total": len(items)})
}

func (h *PublishLimitHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) == 0 {
		Error(w, http.StatusBadRequest, 40001, "缺少站点名")
		return
	}
	siteName := parts[len(parts)-1]
	var limit model.SitePublishLimit
	if err := h.db.WithContext(r.Context()).
		Where("site_name = ?", siteName).
		First(&limit).Error; err != nil {
		Success(w, map[string]interface{}{"exists": false})
		return
	}
	Success(w, map[string]interface{}{"exists": true, "limit": limit})
}

func (h *PublishLimitHandler) handleUpsert(w http.ResponseWriter, r *http.Request) {
	var req model.SitePublishLimit
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "参数解析失败")
		return
	}
	if req.SiteName == "" {
		Error(w, http.StatusBadRequest, 40001, "site_name 不能为空")
		return
	}
	if err := h.db.WithContext(r.Context()).Save(&req).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "保存发布限制失败")
		return
	}
	Success(w, map[string]interface{}{"success": true, "limit": req})
}

func (h *PublishLimitHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) == 0 {
		Error(w, http.StatusBadRequest, 40001, "缺少站点名")
		return
	}
	siteName := parts[len(parts)-1]
	if err := h.db.WithContext(r.Context()).
		Where("site_name = ?", siteName).
		Delete(&model.SitePublishLimit{}).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "删除发布限制失败")
		return
	}
	Success(w, map[string]interface{}{"success": true})
}

func (h *PublishLimitHandler) CheckLimit(ctx context.Context, siteName string) (bool, string) {
	var limit model.SitePublishLimit
	if err := h.db.WithContext(ctx).
		Where("site_name = ? AND enabled = ?", siteName, true).
		First(&limit).Error; err != nil {
		return true, ""
	}
	if limit.MaxCount <= 0 || limit.WindowHours <= 0 {
		return true, ""
	}
	cutoff := time.Now().Add(time.Duration(-limit.WindowHours) * time.Hour)
	var count int64
	h.db.WithContext(ctx).Model(&model.PublishResultRecord{}).
		Where("target_site = ? AND created_at >= ? AND status IN ?",
			siteName, cutoff,
			[]string{"completed", "exists", "edited"}).
		Count(&count)
	if count >= int64(limit.MaxCount) {
		return false, "publish_limit_exceeded"
	}
	return true, ""
}
