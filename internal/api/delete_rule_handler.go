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

type DeleteRuleHandler struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewDeleteRuleHandler(db *gorm.DB, logger *zap.Logger) *DeleteRuleHandler {
	return &DeleteRuleHandler{db: db, logger: logger}
}

func (h *DeleteRuleHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimRight(path, "/")

	if trimmed == "/api/v1/seeding/delete-rules" || trimmed == "/api/v1/seeding/delete-rules/" {
		if r.Method == http.MethodGet {
			h.handleList(w, r)
		} else if r.Method == http.MethodPost {
			h.handleCreate(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	remaining := strings.TrimPrefix(trimmed, "/api/v1/seeding/delete-rules/")
	if remaining == "" {
		h.handleList(w, r)
		return
	}

	parts := strings.SplitN(remaining, "/", 2)
	id, err := parseUintParam(parts[0], "")
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的规则 ID")
		return
	}

	if len(parts) == 2 && parts[1] == "test" && r.Method == http.MethodPost {
		h.handleTestRule(w, r, id)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleGet(w, r, id)
	case http.MethodPut:
		h.handleUpdate(w, r, id)
	case http.MethodDelete:
		h.handleDelete(w, r, id)
	default:
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
	}
}

func (h *DeleteRuleHandler) handleList(w http.ResponseWriter, _ *http.Request) {
	var rules []model.DeleteRule
	if err := h.db.Order("priority ASC, id ASC").Find(&rules).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询删种规则失败")
		return
	}
	Success(w, map[string]interface{}{
		"items": rules,
		"total": len(rules),
	})
}

func (h *DeleteRuleHandler) handleGet(w http.ResponseWriter, _ *http.Request, id uint) {
	var rule model.DeleteRule
	if err := h.db.First(&rule, id).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "规则不存在")
		return
	}
	Success(w, rule)
}

func (h *DeleteRuleHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Alias      string `json:"alias"`
		Priority   int    `json:"priority"`
		Enabled    bool   `json:"enabled"`
		Type       string `json:"type"`
		Conditions string `json:"conditions"`
		Action     string `json:"action"`
		DeleteNum  int    `json:"deleteNum"`
		RemoveData bool   `json:"removeData"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.Alias == "" {
		Error(w, http.StatusBadRequest, 40001, "alias 为必填项")
		return
	}

	rule := model.DeleteRule{
		Alias:      req.Alias,
		Priority:   req.Priority,
		Enabled:    req.Enabled,
		Type:       req.Type,
		Conditions: req.Conditions,
		Action:     req.Action,
		DeleteNum:  req.DeleteNum,
		RemoveData: req.RemoveData,
	}
	if rule.Type == "" {
		rule.Type = "normal"
	}
	if rule.Action == "" {
		rule.Action = "delete"
	}
	if rule.DeleteNum == 0 {
		rule.DeleteNum = 1
	}

	if err := h.db.Create(&rule).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("创建规则失败: %v", err))
		return
	}
	Success(w, rule)
}

func (h *DeleteRuleHandler) handleUpdate(w http.ResponseWriter, r *http.Request, id uint) {
	var rule model.DeleteRule
	if err := h.db.First(&rule, id).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "规则不存在")
		return
	}

	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	updates := make(map[string]interface{})
	if v, ok := req["alias"].(string); ok {
		updates["alias"] = v
	}
	if v, ok := req["priority"].(float64); ok {
		updates["priority"] = int(v)
	}
	if v, ok := req["enabled"].(bool); ok {
		updates["enabled"] = v
	}
	if v, ok := req["type"].(string); ok {
		updates["type"] = v
	}
	if v, ok := req["conditions"].(string); ok {
		updates["conditions"] = v
	}
	if v, ok := req["action"].(string); ok {
		updates["action"] = v
	}
	if v, ok := req["deleteNum"].(float64); ok {
		updates["delete_num"] = int(v)
	}
	if v, ok := req["removeData"].(bool); ok {
		updates["remove_data"] = v
	}
	updates["updated_at"] = time.Now()

	if err := h.db.Model(&rule).Updates(updates).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "更新规则失败")
		return
	}
	h.db.First(&rule, id)
	Success(w, rule)
}

func (h *DeleteRuleHandler) handleDelete(w http.ResponseWriter, _ *http.Request, id uint) {
	if err := h.db.Delete(&model.DeleteRule{}, id).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "删除规则失败")
		return
	}
	Success(w, nil)
}

func (h *DeleteRuleHandler) handleTestRule(w http.ResponseWriter, _ *http.Request, id uint) {
	var rule model.DeleteRule
	if err := h.db.First(&rule, id).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "规则不存在")
		return
	}

	var active []model.SeedingTorrentRecord
	h.db.Where("status = ?", "seeding").Limit(20).Find(&active)

	matched := make([]map[string]interface{}, 0)
	for _, rec := range active {
		matched = append(matched, map[string]interface{}{
			"clientID":  rec.ClientID,
			"infoHash":  rec.InfoHash,
			"siteName":  rec.SiteName,
			"torrentID": rec.TorrentID,
		})
	}

	Success(w, map[string]interface{}{
		"matched": matched,
		"total":   len(matched),
		"rule":    rule,
	})
}
