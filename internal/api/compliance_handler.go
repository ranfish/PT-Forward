package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/ranfish/pt-forward/internal/compliance"
	"github.com/ranfish/pt-forward/internal/model"
)

type ComplianceHandler struct {
	db      *gorm.DB
	logger  *zap.Logger
	checker *compliance.Checker
}

func NewComplianceHandler(db *gorm.DB, logger *zap.Logger) *ComplianceHandler {
	return &ComplianceHandler{db: db, logger: logger}
}

func (h *ComplianceHandler) SetChecker(c *compliance.Checker) { h.checker = c }

func (h *ComplianceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleList(w, r)
	case http.MethodPost:
		h.handleCreate(w, r)
	case http.MethodDelete:
		h.handleDelete(w, r)
	case http.MethodOptions:
		w.WriteHeader(http.StatusNoContent)
	default:
		Error(w, http.StatusMethodNotAllowed, 40500, "method not allowed")
	}
}

func (h *ComplianceHandler) handleList(w http.ResponseWriter, _ *http.Request) {
	var rules []model.ComplianceRule
	if err := h.db.Order("rule_type, source DESC, created_at").Find(&rules).Error; err != nil {
		h.logger.Error("list compliance rules", zap.Error(err))
		Error(w, http.StatusInternalServerError, 50000, "db error")
		return
	}
	Success(w, map[string]interface{}{"items": rules, "total": len(rules)})
}

type createComplianceRuleReq struct {
	RuleType string `json:"rule_type"`
	Pattern  string `json:"pattern"`
	Scope    string `json:"scope"`
}

func (h *ComplianceHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createComplianceRuleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "invalid body")
		return
	}
	req.RuleType = strings.TrimSpace(req.RuleType)
	req.Pattern = strings.TrimSpace(req.Pattern)
	if req.RuleType == "" || req.Pattern == "" {
		Error(w, http.StatusBadRequest, 40001, "rule_type and pattern are required")
		return
	}
	switch req.RuleType {
	case model.RuleTypeAdult, model.RuleTypeForbiddenKeyword, model.RuleTypeForbiddenGroup:
	default:
		Error(w, http.StatusBadRequest, 40001, "invalid rule_type")
		return
	}
	if req.Scope == "" {
		req.Scope = model.ScopeShare
	}
	switch req.Scope {
	case model.ScopeAll, model.ScopePublish, model.ScopeReseed, model.ScopeShare, model.ScopeDownload:
	default:
		Error(w, http.StatusBadRequest, 40001, "invalid scope")
		return
	}

	rule := model.ComplianceRule{
		RuleType: req.RuleType,
		Pattern:  req.Pattern,
		Scope:    req.Scope,
		Source:   "user",
	}
	if err := h.db.Create(&rule).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			Error(w, http.StatusConflict, 40900, "rule already exists")
			return
		}
		h.logger.Error("create compliance rule", zap.Error(err))
		Error(w, http.StatusInternalServerError, 50000, "db error")
		return
	}
	if h.checker != nil {
		h.checker.InvalidateCache()
	}
	Success(w, rule)
}

func (h *ComplianceHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/compliance/rules/")
	idStr = strings.TrimSuffix(idStr, "/")
	if idStr == "" {
		Error(w, http.StatusBadRequest, 40001, "id required")
		return
	}
	var id uint
	parsed, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || parsed == 0 {
		Error(w, http.StatusBadRequest, 40001, "invalid id")
		return
	}
	id = uint(parsed)

	var rule model.ComplianceRule
	if err := h.db.First(&rule, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Error(w, http.StatusNotFound, 40400, "rule not found")
			return
		}
		h.logger.Error("find compliance rule", zap.Error(err))
		Error(w, http.StatusInternalServerError, 50000, "db error")
		return
	}
	if rule.Source == "builtin" {
		Error(w, http.StatusForbidden, 40300, "builtin rule cannot be deleted")
		return
	}
	if err := h.db.Delete(&rule).Error; err != nil {
		h.logger.Error("delete compliance rule", zap.Error(err))
		Error(w, http.StatusInternalServerError, 50000, "db error")
		return
	}
	if h.checker != nil {
		h.checker.InvalidateCache()
	}
	Success(w, map[string]interface{}{"deleted": id})
}
