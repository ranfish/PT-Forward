package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/setting"
	"go.uber.org/zap"
)

type SettingsHandler struct {
	repo   *setting.Repository
	logger *zap.Logger
}

func NewSettingsHandler(repo *setting.Repository, logger *zap.Logger) *SettingsHandler {
	return &SettingsHandler{repo: repo, logger: logger}
}

func (h *SettingsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimRight(path, "/")

	if strings.HasSuffix(trimmed, "/settings/backup") {
		if r.Method == http.MethodGet {
			h.handleBackup(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	if strings.HasSuffix(trimmed, "/settings/restore") {
		if r.Method == http.MethodPost {
			h.handleRestore(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	if trimmed == "/api/v1/settings" || trimmed == "/api/v1/settings/" {
		if r.Method == http.MethodGet {
			h.handleList(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	remaining := strings.TrimPrefix(path, "/api/v1/settings/")
	remaining = strings.TrimRight(remaining, "/")

	if remaining == "" {
		h.handleList(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleGet(w, r, remaining)
	case http.MethodPut:
		h.handleSet(w, r, remaining)
	case http.MethodDelete:
		h.handleDelete(w, r, remaining)
	default:
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
	}
}

func (h *SettingsHandler) handleList(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	settings, err := h.repo.ListByPrefix(r.Context(), prefix)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询设置失败")
		return
	}

	Success(w, map[string]interface{}{
		"items": settings,
	})
}

func (h *SettingsHandler) handleGet(w http.ResponseWriter, r *http.Request, key string) {
	value, err := h.repo.Get(r.Context(), key)
	if err != nil {
		Error(w, http.StatusNotFound, 16001, "设置项不存在")
		return
	}

	Success(w, map[string]interface{}{
		"key":   key,
		"value": value,
	})
}

func (h *SettingsHandler) handleSet(w http.ResponseWriter, r *http.Request, key string) {
	var req struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	if err := h.repo.Set(r.Context(), key, req.Value); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "保存设置失败")
		return
	}

	h.logger.Info("setting updated", zap.String("key", key))
	Success(w, map[string]interface{}{
		"key":   key,
		"value": req.Value,
	})
}

func (h *SettingsHandler) handleDelete(w http.ResponseWriter, r *http.Request, key string) {
	if err := h.repo.Delete(r.Context(), key); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "删除设置失败")
		return
	}

	h.logger.Info("setting deleted", zap.String("key", key))
	Success(w, nil)
}

func (h *SettingsHandler) handleBackup(w http.ResponseWriter, r *http.Request) {
	settings, err := h.repo.ListAll(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, "备份设置失败")
		return
	}

	Success(w, map[string]interface{}{
		"settings": settings,
		"exported": true,
		"count":    len(settings),
		"backupAt": time.Now().Format(time.RFC3339),
	})
}

func (h *SettingsHandler) handleRestore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Settings map[string]string `json:"settings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if len(req.Settings) == 0 {
		Error(w, http.StatusBadRequest, 40001, "settings 不能为空")
		return
	}

	if err := h.repo.RestoreAll(r.Context(), req.Settings); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "恢复设置失败")
		return
	}

	h.logger.Info("settings restored", zap.Int("count", len(req.Settings)))
	Success(w, map[string]interface{}{
		"restored": true,
		"count":    len(req.Settings),
	})
}
