package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type MetadataHandler struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewMetadataHandler(db *gorm.DB, logger *zap.Logger) *MetadataHandler {
	return &MetadataHandler{db: db, logger: logger}
}

func (h *MetadataHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(r.URL.Path, "/")
	switch {
	case strings.HasSuffix(path, "/metadata/type") && r.Method == http.MethodPut:
		h.handleUpdateType(w, r)
	case strings.HasSuffix(path, "/metadata") && r.Method == http.MethodGet:
		h.handleGet(w, r)
	case strings.Contains(path, "/metadata/") && r.Method == http.MethodGet:
		h.handleGetByHash(w, r)
	case strings.HasSuffix(path, "/metadata") && r.Method == http.MethodDelete:
		h.handleDelete(w, r)
	default:
		Error(w, http.StatusNotFound, 40400, "接口不存在")
	}
}

func (h *MetadataHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	infoHash := r.URL.Query().Get("info_hash")
	siteName := r.URL.Query().Get("site_name")
	if infoHash == "" {
		Error(w, http.StatusBadRequest, 40001, "info_hash is required")
		return
	}

	query := h.db.WithContext(r.Context()).Model(&model.TorrentMetadata{}).
		Where("info_hash = ?", infoHash)
	if siteName != "" {
		query = query.Where("site_name = ?", siteName)
	}

	var records []model.TorrentMetadata
	if err := query.Order("updated_at DESC").Find(&records).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询 metadata 失败")
		return
	}

	type metaItem struct {
		model.TorrentMetadata
		TagsList        []string `json:"tags_list"`
		FlagsList       []string `json:"flags_list"`
		ScreenshotsList []string `json:"screenshots_list"`
	}

	items := make([]metaItem, 0, len(records))
	for _, rec := range records {
		item := metaItem{TorrentMetadata: rec}
		if rec.Tags != "" {
			json.Unmarshal([]byte(rec.Tags), &item.TagsList)
		}
		if rec.Flags != "" {
			json.Unmarshal([]byte(rec.Flags), &item.FlagsList)
		}
		if rec.Screenshots != "" {
			json.Unmarshal([]byte(rec.Screenshots), &item.ScreenshotsList)
		}
		items = append(items, item)
	}

	Success(w, map[string]interface{}{
		"items": items,
		"total": len(items),
	})
}

func (h *MetadataHandler) handleGetByHash(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) == 0 {
		Error(w, http.StatusBadRequest, 40001, "缺少 info_hash")
		return
	}
	infoHash := parts[len(parts)-1]

	var meta model.TorrentMetadata
	if err := h.db.WithContext(r.Context()).
		Where("info_hash = ?", infoHash).
		Order("updated_at DESC").
		First(&meta).Error; err != nil {
		Success(w, map[string]interface{}{"exists": false})
		return
	}

	Success(w, map[string]interface{}{"exists": true, "metadata": meta})
}

func (h *MetadataHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	infoHash := r.URL.Query().Get("info_hash")
	siteName := r.URL.Query().Get("site_name")
	if infoHash == "" {
		Error(w, http.StatusBadRequest, 40001, "info_hash is required")
		return
	}

	query := h.db.WithContext(r.Context()).Where("info_hash = ?", infoHash)
	if siteName != "" {
		query = query.Where("site_name = ?", siteName)
	}

	if err := query.Delete(&model.TorrentMetadata{}).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "删除 metadata 失败")
		return
	}
	Success(w, map[string]interface{}{"success": true})
}

func (h *MetadataHandler) handleUpdateType(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InfoHash     string `json:"info_hash"`
		SiteName     string `json:"site_name"`
		StandardType string `json:"standard_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "参数解析失败")
		return
	}
	if req.InfoHash == "" || req.StandardType == "" {
		Error(w, http.StatusBadRequest, 40001, "info_hash 和 standard_type 不能为空")
		return
	}

	validTypes := map[string]bool{
		"category.movie": true, "category.tv_series": true,
		"category.animation": true, "category.documentaries": true,
		"category.tv_shows": true, "category.music": true,
		"category.sports": true,
	}
	if !validTypes[req.StandardType] {
		Error(w, http.StatusBadRequest, 40001, "无效的 standard_type: "+req.StandardType)
		return
	}

	query := h.db.WithContext(r.Context()).Model(&model.TorrentMetadata{}).
		Where("info_hash = ?", req.InfoHash)
	if req.SiteName != "" {
		query = query.Where("site_name = ?", req.SiteName)
	}

	result := query.Update("standard_type", req.StandardType)
	if result.Error != nil {
		Error(w, http.StatusInternalServerError, 50000, "更新失败")
		return
	}
	if result.RowsAffected == 0 {
		meta := &model.TorrentMetadata{
			InfoHash:     req.InfoHash,
			SiteName:     req.SiteName,
			StandardType: req.StandardType,
			FetchSource:  "manual",
		}
		if err := h.db.WithContext(r.Context()).Create(meta).Error; err != nil {
			Error(w, http.StatusInternalServerError, 50000, "创建失败")
			return
		}
	}

	Success(w, map[string]interface{}{"success": true})
}
