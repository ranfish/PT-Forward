package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/cookiecloud"
	"github.com/ranfish/pt-forward/internal/middleware"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type CookieCloudHandler struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewCookieCloudHandler(db *gorm.DB, logger *zap.Logger) *CookieCloudHandler {
	return &CookieCloudHandler{db: db, logger: logger}
}

func (h *CookieCloudHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimRight(path, "/")

	switch {
	case trimmed == "/api/v1/cookiecloud/config" || trimmed == "/api/v1/cookiecloud/config/":
		switch r.Method {
		case http.MethodGet:
			h.handleGetConfig(w, r)
		case http.MethodPut:
			h.handleUpdateConfig(w, r)
		default:
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return

	case trimmed == "/api/v1/cookiecloud/sync" || trimmed == "/api/v1/cookiecloud/sync/":
		if r.Method == http.MethodPost {
			h.handleSync(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return

	case trimmed == "/api/v1/cookiecloud/history" || trimmed == "/api/v1/cookiecloud/history/":
		if r.Method == http.MethodGet {
			h.handleListHistory(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return

	case trimmed == "/api/v1/cookiecloud/test" || trimmed == "/api/v1/cookiecloud/test/":
		if r.Method == http.MethodPost {
			h.handleTest(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	Error(w, http.StatusNotFound, 40400, "接口不存在")
}

func (h *CookieCloudHandler) handleGetConfig(w http.ResponseWriter, _ *http.Request) {
	var cfg model.CookieCloudConfig
	err := h.db.First(&cfg).Error
	if err != nil && err == gorm.ErrRecordNotFound {
		Success(w, map[string]interface{}{
			"serverUrl":    "",
			"uuid":         "",
			"syncEnabled":  false,
			"syncInterval": 60,
			"cryptoType":   "legacy",
		})
		return
	}
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "获取配置失败")
		return
	}

	Success(w, map[string]interface{}{
		"id":           cfg.ID,
		"serverUrl":    cfg.ServerURL,
		"uuid":         cfg.UUID,
		"syncEnabled":  cfg.SyncEnabled,
		"syncInterval": cfg.SyncInterval,
		"cryptoType":   cfg.CryptoType,
		"lastSyncAt":   cfg.LastSyncAt,
		"hasPassword":  cfg.Password != "",
	})
}

func (h *CookieCloudHandler) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerURL    string `json:"serverUrl"`
		UUID         string `json:"uuid"`
		Password     string `json:"password"`
		CryptoType   string `json:"cryptoType"`
		SyncEnabled  *bool  `json:"syncEnabled"`
		SyncInterval *int   `json:"syncInterval"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	if req.ServerURL != "" {
		if err := middleware.ValidatePublicURL(req.ServerURL); err != nil {
			Error(w, http.StatusBadRequest, 40001, "serverUrl 不合法: "+err.Error())
			return
		}
	}

	var cfg model.CookieCloudConfig
	result := h.db.First(&cfg)
	switch {
	case result.Error != nil && errors.Is(result.Error, gorm.ErrRecordNotFound):
		cfg = model.CookieCloudConfig{
			ServerURL:  req.ServerURL,
			UUID:       req.UUID,
			Password:   req.Password,
			CryptoType: req.CryptoType,
		}
		if req.SyncEnabled != nil {
			cfg.SyncEnabled = *req.SyncEnabled
		}
		if req.SyncInterval != nil {
			cfg.SyncInterval = *req.SyncInterval
		}
		if cfg.CryptoType == "" {
			cfg.CryptoType = "legacy"
		}
		if cfg.SyncInterval <= 0 {
			cfg.SyncInterval = 60
		}
		if err := h.db.Create(&cfg).Error; err != nil {
			Error(w, http.StatusInternalServerError, 50000, "创建配置失败")
			return
		}
	case result.Error != nil:
		Error(w, http.StatusInternalServerError, 50000, "获取配置失败")
		return
	default:
		if req.ServerURL != "" {
			cfg.ServerURL = req.ServerURL
		}
		if req.UUID != "" {
			cfg.UUID = req.UUID
		}
		if req.Password != "" {
			cfg.Password = req.Password
		}
		if req.CryptoType != "" {
			cfg.CryptoType = req.CryptoType
		}
		if req.SyncEnabled != nil {
			cfg.SyncEnabled = *req.SyncEnabled
		}
		if req.SyncInterval != nil {
			cfg.SyncInterval = *req.SyncInterval
		}
		if err := h.db.Save(&cfg).Error; err != nil {
			Error(w, http.StatusInternalServerError, 50000, "保存配置失败")
			return
		}
	}

	h.logger.Info("cookiecloud config updated", zap.String("component", "cookiecloud"))
	Success(w, map[string]interface{}{"message": "配置已更新"})
}

func (h *CookieCloudHandler) handleSync(w http.ResponseWriter, r *http.Request) {
	svc := cookiecloud.NewSyncService(h.db, h.logger)
	history, err := svc.SyncAll(r.Context())
	if err != nil {
		h.logger.Error("cookiecloud sync failed", zap.Error(err))
		Error(w, http.StatusInternalServerError, 50001, "CookieCloud 同步失败，请检查配置")
		return
	}
	Success(w, history)
}

func (h *CookieCloudHandler) handleListHistory(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if page < 1 {
		page = 1
	} else if page > 10000 {
		page = 10000
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int64
	h.db.Model(&model.CookieCloudSyncHistory{}).Count(&total)

	var histories []model.CookieCloudSyncHistory
	if err := h.db.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&histories).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询同步历史失败")
		return
	}

	Success(w, map[string]interface{}{
		"items": histories,
		"total": total,
		"page":  page,
	})
}

func (h *CookieCloudHandler) handleTest(w http.ResponseWriter, r *http.Request) {
	var cfg model.CookieCloudConfig
	if err := h.db.First(&cfg).Error; err != nil {
		Error(w, http.StatusBadRequest, 40001, "请先配置 CookieCloud")
		return
	}

	if cfg.ServerURL == "" || cfg.UUID == "" || cfg.Password == "" {
		Error(w, http.StatusBadRequest, 40001, "配置不完整（需要 serverUrl/uuid/password）")
		return
	}

	_, err := cookiecloud.FetchAndDecrypt(cfg.ServerURL, cfg.UUID, cfg.Password)
	if err != nil {
		h.logger.Error("cookiecloud test connection failed", zap.Error(err))
		Error(w, http.StatusBadGateway, 50001, "CookieCloud 连接失败，请检查服务器地址和密码")
		return
	}

	Success(w, map[string]interface{}{
		"ok":      true,
		"message": "连接测试成功",
	})
}
