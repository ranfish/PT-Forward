package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/ptgen"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type PTGenHandler struct {
	db       *gorm.DB
	logger   *zap.Logger
	provider *ptgen.Provider
}

func NewPTGenHandler(db *gorm.DB, logger *zap.Logger) *PTGenHandler {
	return &PTGenHandler{
		db:       db,
		logger:   logger,
		provider: ptgen.NewProvider(db, logger),
	}
}

func (h *PTGenHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimRight(path, "/")

	switch {
	case trimmed == "/api/v1/ptgen/query" || trimmed == "/api/v1/ptgen/query/":
		if r.Method == http.MethodPost {
			h.handleQuery(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return

	case trimmed == "/api/v1/ptgen/cache" || trimmed == "/api/v1/ptgen/cache/":
		switch r.Method {
		case http.MethodGet:
			h.handleListCache(w, r)
		case http.MethodDelete:
			h.handleCleanCache(w, r)
		default:
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	Error(w, http.StatusNotFound, 40400, "接口不存在")
}

func (h *PTGenHandler) handleQuery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.Query == "" {
		Error(w, http.StatusBadRequest, 40001, "query 不能为空")
		return
	}

	result, err := h.provider.Query(r.Context(), req.Query)
	if err != nil {
		Error(w, http.StatusBadGateway, 50001, "PTGen 查询失败，请稍后重试")
		return
	}

	Success(w, result)
}

func (h *PTGenHandler) handleListCache(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	q := h.db.Model(&model.PTGenCache{})
	if keyword := r.URL.Query().Get("keyword"); keyword != "" {
		escaped := strings.NewReplacer("%", "\\%", "_", "\\_").Replace(keyword)
		q = q.Where("query_key LIKE ? OR chinese_title LIKE ?", "%"+escaped+"%", "%"+escaped+"%")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询缓存总数失败")
		return
	}

	var caches []model.PTGenCache
	if err := q.Order("updated_at DESC").Offset(offset).Limit(pageSize).Find(&caches).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询缓存失败")
		return
	}

	Success(w, map[string]interface{}{
		"items": caches,
		"total": total,
		"page":  page,
	})
}

func (h *PTGenHandler) handleCleanCache(w http.ResponseWriter, r *http.Request) {
	retainDays := 30
	if d := r.URL.Query().Get("retainDays"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 {
			retainDays = v
		}
	}

	deleted, err := h.provider.CleanExpiredCache(r.Context(), retainDays)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "清理缓存失败")
		return
	}

	Success(w, map[string]interface{}{
		"deleted":    deleted,
		"retainDays": retainDays,
	})
}
