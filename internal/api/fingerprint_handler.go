package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type FingerprintHandler struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewFingerprintHandler(db *gorm.DB, logger *zap.Logger) *FingerprintHandler {
	return &FingerprintHandler{db: db, logger: logger}
}

func (h *FingerprintHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimRight(path, "/")

	switch {
	case trimmed == "/api/v1/fingerprints" || trimmed == "/api/v1/fingerprints/":
		if r.Method == http.MethodGet {
			h.handleList(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return

	case strings.HasSuffix(trimmed, "/fingerprints/search"):
		h.handleSearch(w, r)
		return

	case strings.HasSuffix(trimmed, "/fingerprints/cache"):
		if r.Method == http.MethodDelete {
			h.handleDeleteCache(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	if strings.Contains(trimmed, "/fingerprints/") {
		id, err := parseUintParam(trimmed, "/api/v1/fingerprints/")
		if err != nil {
			Error(w, http.StatusBadRequest, 40001, "无效的指纹 ID")
			return
		}
		switch r.Method {
		case http.MethodGet:
			h.handleGet(w, r, id)
		case http.MethodDelete:
			h.handleDelete(w, r, id)
		default:
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	Error(w, http.StatusNotFound, 40400, "接口不存在")
}

func (h *FingerprintHandler) handleList(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	q := h.db.Model(&model.ContentFingerprint{})
	if siteName := r.URL.Query().Get("siteName"); siteName != "" {
		q = q.Where("site_name = ?", siteName)
	}

	var total int64
	q.Count(&total)

	var fingerprints []model.ContentFingerprint
	q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&fingerprints)

	Success(w, map[string]interface{}{
		"items": fingerprints,
		"total": total,
		"page":  page,
	})
}

func (h *FingerprintHandler) handleGet(w http.ResponseWriter, r *http.Request, id uint) {
	var fp model.ContentFingerprint
	if err := h.db.First(&fp, id).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "指纹不存在")
		return
	}
	Success(w, fp)
}

func (h *FingerprintHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	q := h.db.Model(&model.ContentFingerprint{})
	if infoHash := r.URL.Query().Get("infoHash"); infoHash != "" {
		q = q.Where("info_hash = ?", infoHash)
	} else if piecesHash := r.URL.Query().Get("piecesHash"); piecesHash != "" {
		q = q.Where("pieces_hash = ?", piecesHash)
	} else {
		Error(w, http.StatusBadRequest, 40001, "需要 infoHash 或 piecesHash 参数")
		return
	}

	var fingerprints []model.ContentFingerprint
	q.Find(&fingerprints)

	Success(w, map[string]interface{}{
		"items": fingerprints,
		"total": len(fingerprints),
	})
}

func (h *FingerprintHandler) handleDelete(w http.ResponseWriter, r *http.Request, id uint) {
	if err := h.db.Delete(&model.ContentFingerprint{}, id).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "删除指纹失败")
		return
	}
	Success(w, nil)
}

func (h *FingerprintHandler) handleDeleteCache(w http.ResponseWriter, r *http.Request) {
	result := h.db.Where("1 = 1").Delete(&model.SearchCache{})
	if result.Error != nil {
		Error(w, http.StatusInternalServerError, 50000, "清理缓存失败")
		return
	}
	Success(w, map[string]interface{}{
		"deleted": result.RowsAffected,
	})
}
