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
