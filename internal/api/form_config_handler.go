// Package api 站点发布配置中心 handler（§59.147 切片 3：HTML 上传半自动）。
package api

import (
	"context"
	"strings"
	"encoding/json"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
)

// FormConfigHandler 配置中心端点。
type FormConfigHandler struct {
	db        *gorm.DB
	publisher *publish.Pipeline
}

// NewFormConfigHandler 创建 handler。
func NewFormConfigHandler(db *gorm.DB) *FormConfigHandler {
	return &FormConfigHandler{db: db}
}

// ServeHTTP /publish/form-config/{get|parse|apply}
func (h *FormConfigHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	switch {
	case strings.HasSuffix(path, "/publish/form-config/get") && r.Method == http.MethodGet:
		h.handleGet(w, r)
	case strings.HasSuffix(path, "/publish/form-config/parse") && r.Method == http.MethodPost:
		h.handleParse(w, r)
	case strings.HasSuffix(path, "/publish/form-config/apply") && r.Method == http.MethodPost:
		h.handleApply(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
	}
}

// handleGet GET ?site_name= → 当前配置
func (h *FormConfigHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	siteName := r.URL.Query().Get("site_name")
	if siteName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "site_name 必填"})
		return
	}
	site, err := h.loadSite(r.Context(), siteName)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"site_name": site.Name,
		"config":    model.ParseFormConfig(site.PublishFormConfig),
	})
}

// handleParse POST {site_name, html} → {draft, merged, diffs}
// HTML 即弃：内存解析，不落库不写日志（L2 敏感信息——表单含 auth/token）。
func (h *FormConfigHandler) handleParse(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SiteName string `json:"site_name"`
		HTML     string `json:"html"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SiteName == "" || req.HTML == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "site_name 与 html 必填"})
		return
	}
	site, err := h.loadSite(r.Context(), req.SiteName)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	draft := publish.ParsePublishFormHTML(req.HTML)
	if draft == nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "HTML 无可识别发布表单（select/tags checkbox 均未命中）"})
		return
	}
	merged, diffs := publish.MergeDraftWithCurrent(model.ParseFormConfig(site.PublishFormConfig), draft)
	writeJSON(w, http.StatusOK, map[string]any{
		"site_name": site.Name,
		"draft":     draft,
		"merged":    merged,
		"diffs":     diffs,
	})
}

// handleApply POST {site_name, config, note} → 落库 + 审计（diff 确认是唯一写入路径）。
func (h *FormConfigHandler) handleApply(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SiteName string                `json:"site_name"`
		Config   *model.PublishFormConfig `json:"config"`
		Note     string                `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SiteName == "" || req.Config == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "site_name 与 config 必填"})
		return
	}
	site, err := h.loadSite(r.Context(), req.SiteName)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	prev := site.PublishFormConfig
	if err := h.db.WithContext(r.Context()).Model(&model.Site{}).
		Where("id = ?", site.ID).
		Update("publish_form_config", req.Config.Serialize()).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	// L4 配置审计：变更写 operation_audit_logs（回滚=重上传 HTML 重 diff，不做版本管理）
	audit, _ := json.Marshal(map[string]any{
		"site": site.Name, "note": req.Note,
		"domains": len(req.Config.FormFields),
		"prev_len": len(prev), "new_len": len(req.Config.Serialize()),
	})
	h.db.WithContext(context.Background()).Create(&model.OperationAuditLog{
		Actor: "user", Module: "publish", Action: "form_config_apply",
		TargetType: "site", TargetID: site.Name,
		Detail: string(audit),
		CreatedAt: time.Now(),
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *FormConfigHandler) loadSite(ctx context.Context, name string) (*model.Site, error) {
	var site model.Site
	if err := h.db.WithContext(ctx).Where("name = ?", name).First(&site).Error; err != nil {
		return nil, err
	}
	return &site, nil
}
