package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/filter"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type FilterHandler struct {
	repo      *filter.Repository
	filterEng *filter.Engine
	db        *gorm.DB
	logger    *zap.Logger
}

func NewFilterHandler(repo *filter.Repository, filterEng *filter.Engine, db *gorm.DB, logger *zap.Logger) *FilterHandler {
	return &FilterHandler{repo: repo, filterEng: filterEng, db: db, logger: logger}
}

type createFilterRequest struct {
	Name       string                `json:"name"`
	RuleType   string                `json:"ruleType"`
	Priority   int                   `json:"priority,omitempty"`
	Conditions []model.RuleCondition `json:"conditions"`
	SavePath   string                `json:"savePath,omitempty"`
	Category   string                `json:"category,omitempty"`
	Tags       string                `json:"tags,omitempty"`
	Enabled    bool                  `json:"enabled"`
}

type updateFilterRequest struct {
	Name       *string                `json:"name,omitempty"`
	RuleType   *string                `json:"ruleType,omitempty"`
	Priority   *int                   `json:"priority,omitempty"`
	Conditions *[]model.RuleCondition `json:"conditions,omitempty"`
	SavePath   *string                `json:"savePath,omitempty"`
	Category   *string                `json:"category,omitempty"`
	Tags       *string                `json:"tags,omitempty"`
	Enabled    *bool                  `json:"enabled,omitempty"`
}

type filterResponse struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	RuleType  string    `json:"ruleType"`
	Priority  int       `json:"priority"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	Conditions []model.RuleCondition `json:"conditions"`
	SavePath   string                `json:"savePath,omitempty"`
	Category   string                `json:"category,omitempty"`
	Tags       string                `json:"tags,omitempty"`
}

var validRuleTypes = map[string]bool{
	"accept": true, "reject": true, "accept_and_reject": true,
}

func (h *FilterHandler) toResponse(r *model.FilterRule) filterResponse {
	return filterResponse{
		ID:         r.ID,
		Name:       r.Name,
		RuleType:   r.RuleType,
		Priority:   r.Priority,
		Enabled:    r.Enabled,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
		Conditions: r.Conditions,
		SavePath:   r.SavePath,
		Category:   r.Category,
		Tags:       r.Tags,
	}
}

func (h *FilterHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handleRouteByPath(w, r)
}

func (h *FilterHandler) handleRouteByPath(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimRight(path, "/")

	if trimmed == "/api/v1/filters/rules" {
		if r.Method == http.MethodGet {
			h.handleList(w, r)
			return
		}
		if r.Method == http.MethodPost {
			h.handleCreate(w, r)
			return
		}
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		return
	}

	remaining := strings.TrimPrefix(trimmed, "/api/v1/filters/rules/")
	if remaining == "" || remaining == "/" {
		switch r.Method {
		case http.MethodGet:
			h.handleList(w, r)
		case http.MethodPost:
			h.handleCreate(w, r)
		default:
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
		return
	}

	remaining = strings.TrimPrefix(path, "/api/v1/filters/rules/")
	remaining = strings.TrimRight(remaining, "/")

	switch r.Method {
	case http.MethodGet:
		h.handleGet(w, r, remaining)
	case http.MethodPut:
		h.handleUpdate(w, r, remaining)
	case http.MethodDelete:
		h.handleDelete(w, r, remaining)
	case http.MethodPost:
		if strings.HasSuffix(remaining, "/test") {
			idStr := strings.TrimSuffix(remaining, "/test")
			h.handleTest(w, r, idStr)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
	default:
		Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
	}
}

func (h *FilterHandler) handleList(w http.ResponseWriter, r *http.Request) {
	page, size := parsePagination(r)

	var total int64
	h.db.Model(&model.FilterRule{}).
		Count(&total)

	var rules []model.FilterRule
	h.db.
		Order("priority ASC, id ASC").
		Offset(offset(page, size)).Limit(size).
		Find(&rules)

	items := make([]filterResponse, 0, len(rules))
	for i := range rules {
		items = append(items, h.toResponse(&rules[i]))
	}

	Success(w, PaginatedResult{
		Items: items,
		Total: total,
		Page:  page,
		Size:  size,
	})
}

func (h *FilterHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createFilterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	if req.Name == "" {
		Error(w, http.StatusBadRequest, 40001, "name 为必填项")
		return
	}
	if req.RuleType == "" {
		req.RuleType = "accept"
	}
	if !validRuleTypes[req.RuleType] {
		Error(w, http.StatusBadRequest, 40001, "ruleType 必须为 accept/reject/accept_and_reject")
		return
	}
	if len(req.Conditions) == 0 {
		Error(w, http.StatusBadRequest, 40001, "至少需要一个条件")
		return
	}
	for i, c := range req.Conditions {
		if c.Key == "" {
			Error(w, http.StatusBadRequest, 40001, fmt.Sprintf("条件 %d 缺少 key", i))
			return
		}
		if c.CompareType == "" {
			Error(w, http.StatusBadRequest, 40001, fmt.Sprintf("条件 %d 缺少 compare_type", i))
			return
		}
	}

	exists, _ := h.repo.ExistsByName(r.Context(), req.Name, 0)
	if exists {
		Error(w, http.StatusConflict, 40900, "规则名称已存在")
		return
	}

	rule := &model.FilterRule{
		Name:       req.Name,
		RuleType:   req.RuleType,
		Priority:   req.Priority,
		Conditions: req.Conditions,
		SavePath:   req.SavePath,
		Category:   req.Category,
		Tags:       req.Tags,
		Enabled:    req.Enabled,
	}

	if err := h.repo.Create(r.Context(), rule); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "创建规则失败")
		return
	}

	h.logger.Info("filter rule created", zap.String("name", rule.Name), zap.String("type", rule.RuleType))
	Success(w, h.toResponse(rule))
}

func (h *FilterHandler) handleGet(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := parseUint(idStr)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的规则 ID")
		return
	}

	rule, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, 14001, "规则不存在")
		return
	}

	Success(w, h.toResponse(rule))
}

func (h *FilterHandler) handleUpdate(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := parseUint(idStr)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的规则 ID")
		return
	}

	rule, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, 14001, "规则不存在")
		return
	}

	var req updateFilterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	if req.Name != nil {
		exists, _ := h.repo.ExistsByName(r.Context(), *req.Name, id)
		if exists {
			Error(w, http.StatusConflict, 40900, "规则名称已存在")
			return
		}
		rule.Name = *req.Name
	}
	if req.RuleType != nil {
		if !validRuleTypes[*req.RuleType] {
			Error(w, http.StatusBadRequest, 40001, "不支持的 ruleType")
			return
		}
		rule.RuleType = *req.RuleType
	}
	if req.Priority != nil {
		rule.Priority = *req.Priority
	}
	if req.Conditions != nil {
		rule.Conditions = *req.Conditions
	}
	if req.SavePath != nil {
		rule.SavePath = *req.SavePath
	}
	if req.Category != nil {
		rule.Category = *req.Category
	}
	if req.Tags != nil {
		rule.Tags = *req.Tags
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}

	if err := h.repo.Update(r.Context(), rule); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "更新规则失败")
		return
	}

	h.logger.Info("filter rule updated", zap.String("name", rule.Name))
	Success(w, h.toResponse(rule))
}

func (h *FilterHandler) handleDelete(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := parseUint(idStr)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的规则 ID")
		return
	}

	_, err = h.repo.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, 14001, "规则不存在")
		return
	}

	if err := h.repo.Delete(r.Context(), id); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "删除规则失败")
		return
	}

	h.logger.Info("filter rule deleted", zap.Uint("id", id))
	Success(w, nil)
}

func (h *FilterHandler) handleTest(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := parseUint(idStr)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的规则 ID")
		return
	}

	rule, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, 14001, "规则不存在")
		return
	}

	var req struct {
		Title         string `json:"title"`
		Size          int64  `json:"size"`
		Uploader      string `json:"uploader"`
		SiteName      string `json:"siteName"`
		Category      string `json:"category"`
		Free          bool   `json:"free"`
		Tags          string `json:"tags"`
		DiscountLevel string `json:"discountLevel"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	evalCtx := &filter.EvalContext{
		Title:         req.Title,
		Size:          req.Size,
		Uploader:      req.Uploader,
		SiteName:      req.SiteName,
		Category:      req.Category,
		Free:          req.Free,
		DiscountLevel: req.DiscountLevel,
	}
	if req.Tags != "" {
		evalCtx.Tags = strings.Split(req.Tags, ",")
	}

	matched := matchRuleDirect(rule, evalCtx)
	Success(w, map[string]interface{}{
		"matched":  matched,
		"ruleName": rule.Name,
		"ruleType": rule.RuleType,
	})
}

func matchRuleDirect(rule *model.FilterRule, ctx *filter.EvalContext) bool {
	for _, cond := range rule.Conditions {
		if !filter.MatchConditionExport(cond, ctx) {
			return false
		}
	}
	return true
}

func parseUint(s string) (uint, error) {
	var n uint
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
