package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type CloudFPHandler struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewCloudFPHandler(db *gorm.DB, logger *zap.Logger) *CloudFPHandler {
	return &CloudFPHandler{db: db, logger: logger}
}

func (h *CloudFPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(r.URL.Path, "/")
	switch {
	case path == "/api/v1/cloud-fp/config":
		switch r.Method {
		case http.MethodGet:
			h.handleGetConfig(w, r)
		case http.MethodPut:
			h.handleUpdateConfig(w, r)
		default:
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
	case path == "/api/v1/cloud-fp/test":
		if r.Method == http.MethodPost {
			h.handleTest(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
	case path == "/api/v1/cloud-fp/status":
		if r.Method == http.MethodGet {
			h.handleStatus(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
	default:
		Error(w, http.StatusNotFound, 40400, "路径不存在")
	}
}

func (h *CloudFPHandler) handleGetConfig(w http.ResponseWriter, _ *http.Request) {
	var cfg model.CloudFPConfig
	if err := h.db.First(&cfg, 1).Error; err != nil {
		cfg = model.CloudFPConfig{ID: 1, RequestTimeoutSec: 10}
	}
	cfg.APIToken = maskCloudFPToken(cfg.APIToken)
	Success(w, cfg)
}

func (h *CloudFPHandler) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var input model.CloudFPConfig
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的请求体")
		return
	}
	input.ID = 1
	if input.RequestTimeoutSec <= 0 {
		input.RequestTimeoutSec = 10
	}
	if input.APIToken == "***" {
		var existing model.CloudFPConfig
		if err := h.db.First(&existing, 1).Error; err == nil {
			input.APIToken = existing.APIToken
		}
	}
	if err := h.db.Save(&input).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50001, "保存配置失败")
		return
	}
	input.APIToken = maskCloudFPToken(input.APIToken)
	Success(w, input)
}

func (h *CloudFPHandler) handleTest(w http.ResponseWriter, r *http.Request) {
	var cfg model.CloudFPConfig
	if err := h.db.First(&cfg, 1).Error; err != nil {
		Error(w, http.StatusBadRequest, 40001, "请先保存配置")
		return
	}
	if !cfg.Enabled || cfg.BaseURL == "" {
		Error(w, http.StatusBadRequest, 40001, "服务未启用或未配置 URL")
		return
	}
	client := &http.Client{Timeout: time.Duration(cfg.RequestTimeoutSec) * time.Second}
	req, err := http.NewRequestWithContext(r.Context(), "GET", cfg.BaseURL+"/api/v1/health", nil)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50001, "构造请求失败")
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		Error(w, http.StatusBadGateway, 50201, fmt.Sprintf("连接失败: %v", err))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		Error(w, http.StatusBadGateway, 50201, fmt.Sprintf("HTTP %d", resp.StatusCode))
		return
	}
	Success(w, map[string]interface{}{"status": "ok"})
}

func (h *CloudFPHandler) handleStatus(w http.ResponseWriter, _ *http.Request) {
	var cfg model.CloudFPConfig
	if err := h.db.First(&cfg, 1).Error; err != nil {
		Success(w, map[string]interface{}{"configured": false, "enabled": false})
		return
	}
	Success(w, map[string]interface{}{
		"configured":          true,
		"enabled":             cfg.Enabled,
		"base_url":            cfg.BaseURL,
		"request_timeout_sec": cfg.RequestTimeoutSec,
	})
}

func maskCloudFPToken(token string) string {
	if len(token) <= 4 {
		return "***"
	}
	return token[:2] + "***" + token[len(token)-2:]
}
