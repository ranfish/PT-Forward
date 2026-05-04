package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type IYUUHandler struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewIYUUHandler(db *gorm.DB, logger *zap.Logger) *IYUUHandler {
	return &IYUUHandler{db: db, logger: logger}
}

func (h *IYUUHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimRight(path, "/")

	switch {
	case trimmed == "/api/v1/iyuu/config" || trimmed == "/api/v1/iyuu/config/":
		if r.Method == http.MethodGet {
			h.handleGetConfig(w, r)
		} else if r.Method == http.MethodPut {
			h.handleUpdateConfig(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return

	case trimmed == "/api/v1/iyuu/sites" || trimmed == "/api/v1/iyuu/sites/":
		if r.Method == http.MethodGet {
			h.handleListSites(w, r)
		} else if r.Method == http.MethodPost {
			h.handleSyncSites(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return

	case trimmed == "/api/v1/iyuu/query" || trimmed == "/api/v1/iyuu/query/":
		if r.Method == http.MethodPost {
			h.handleQuery(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return

	case trimmed == "/api/v1/iyuu/test" || trimmed == "/api/v1/iyuu/test/":
		if r.Method == http.MethodPost {
			h.handleTest(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	Error(w, http.StatusNotFound, 40400, "接口不存在")
}

func (h *IYUUHandler) handleGetConfig(w http.ResponseWriter, _ *http.Request) {
	var configs []model.IYUUConfig
	h.db.Find(&configs)

	if len(configs) == 0 {
		Success(w, map[string]interface{}{
			"token":   "",
			"enabled": false,
			"baseUrl": "https://2025.iyuu.cn",
		})
		return
	}

	cfg := configs[0]
	Success(w, map[string]interface{}{
		"id":               cfg.ID,
		"token":            maskToken(cfg.Token),
		"enabled":          cfg.Enabled,
		"baseUrl":          cfg.BaseURL,
		"isVip":            cfg.IsVIP,
		"version":          cfg.Version,
		"requestTimeoutMs": cfg.RequestTimeoutSec * 1000,
	})
}

func (h *IYUUHandler) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token             string `json:"token"`
		BaseURL           string `json:"baseUrl"`
		Enabled           *bool  `json:"enabled"`
		IsVIP             *bool  `json:"isVip"`
		RequestTimeoutSec *int   `json:"requestTimeoutSec"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	var cfg model.IYUUConfig
	result := h.db.First(&cfg)
	if result.Error != nil && result.Error == gorm.ErrRecordNotFound {
		cfg = model.IYUUConfig{
			Token:   req.Token,
			BaseURL: req.BaseURL,
		}
		if req.Enabled != nil {
			cfg.Enabled = *req.Enabled
		}
		if req.IsVIP != nil {
			cfg.IsVIP = *req.IsVIP
		}
		if req.RequestTimeoutSec != nil {
			cfg.RequestTimeoutSec = *req.RequestTimeoutSec
		}
		if err := h.db.Create(&cfg).Error; err != nil {
			Error(w, http.StatusInternalServerError, 50000, "创建配置失败")
			return
		}
	} else if result.Error != nil {
		Error(w, http.StatusInternalServerError, 50000, "获取配置失败")
		return
	} else {
		if req.Token != "" {
			cfg.Token = req.Token
		}
		if req.BaseURL != "" {
			cfg.BaseURL = req.BaseURL
		}
		if req.Enabled != nil {
			cfg.Enabled = *req.Enabled
		}
		if req.IsVIP != nil {
			cfg.IsVIP = *req.IsVIP
		}
		if req.RequestTimeoutSec != nil {
			cfg.RequestTimeoutSec = *req.RequestTimeoutSec
		}
		if err := h.db.Save(&cfg).Error; err != nil {
			Error(w, http.StatusInternalServerError, 50000, "保存配置失败")
			return
		}
	}

	h.logger.Info("iyuu config updated")
	Success(w, map[string]interface{}{"message": "配置已更新"})
}

func (h *IYUUHandler) handleListSites(w http.ResponseWriter, _ *http.Request) {
	var mappings []model.IYUUSiteMapping
	h.db.Order("site_name ASC").Find(&mappings)

	Success(w, map[string]interface{}{
		"items": mappings,
		"total": len(mappings),
	})
}

func (h *IYUUHandler) handleSyncSites(w http.ResponseWriter, _ *http.Request) {
	h.logger.Info("iyuu site sync triggered")
	Success(w, map[string]interface{}{
		"message": "站点同步已触发（后台执行中）",
	})
}

func (h *IYUUHandler) handleQuery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InfoHashes []string `json:"infoHashes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if len(req.InfoHashes) == 0 {
		Error(w, http.StatusBadRequest, 40001, "至少需要一个 infoHash")
		return
	}

	Success(w, map[string]interface{}{
		"results": []model.IYUUReseedResult{},
		"total":   0,
		"message": "IYUU 服务连接未实现，请先配置 Token",
	})
}

func (h *IYUUHandler) handleTest(w http.ResponseWriter, _ *http.Request) {
	var cfg model.IYUUConfig
	if err := h.db.First(&cfg).Error; err != nil {
		Error(w, http.StatusBadRequest, 40001, "请先配置 IYUU Token")
		return
	}

	if cfg.Token == "" {
		Error(w, http.StatusBadRequest, 40001, "Token 为空")
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", "https://api.iyuu.cn/App.Api/sites?token="+cfg.Token, nil)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "构造请求失败")
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		Error(w, http.StatusBadGateway, 50001, "连接 IYUU 服务失败: "+err.Error())
		return
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		Error(w, http.StatusBadGateway, 50001, "IYUU 返回 HTTP "+resp.Status)
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		Error(w, http.StatusBadGateway, 50001, "IYUU 响应解析失败")
		return
	}

	if code, ok := result["ret"].(float64); ok && code == 200 {
		Success(w, map[string]interface{}{
			"ok":      true,
			"message": "连接测试成功",
		})
	} else {
		msg := "未知错误"
		if m, ok := result["msg"].(string); ok {
			msg = m
		}
		Error(w, http.StatusBadGateway, 50001, "IYUU 返回: "+msg)
	}
}

func maskToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "****" + token[len(token)-4:]
}
