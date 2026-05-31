package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	dbimpl "github.com/ranfish/pt-forward/internal/db"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/seeding"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type DeleteRuleHandler struct {
	db         *gorm.DB
	logger     *zap.Logger
	clientMgr  ClientManager
}

func NewDeleteRuleHandler(db *gorm.DB, logger *zap.Logger, clientMgr ClientManager) *DeleteRuleHandler {
	return &DeleteRuleHandler{db: db, logger: logger, clientMgr: clientMgr}
}

func (h *DeleteRuleHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	trimmed := strings.TrimRight(path, "/")

	var prefix string
	var isRulesPath bool
	switch {
	case strings.HasPrefix(trimmed, "/api/v1/seeding/delete-rules"):
		prefix = "/api/v1/seeding/delete-rules"
	case strings.HasPrefix(trimmed, "/api/v1/seeding/rules"):
		prefix = "/api/v1/seeding/rules"
		isRulesPath = true
	default:
		Error(w, http.StatusNotFound, 40400, "路径不存在")
		return
	}

	if trimmed == prefix || trimmed == prefix+"/" {
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

	remaining := strings.TrimPrefix(trimmed, prefix+"/")
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
		if isRulesPath {
			h.handleTestRuleDryrun(w, r, id)
		} else {
			h.handleTestRule(w, r, id)
		}
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
		Alias               string `json:"alias"`
		Priority            int    `json:"priority"`
		Enabled             bool   `json:"enabled"`
		Type                string `json:"type"`
		Logic               string `json:"logic"`
		Conditions          string `json:"conditions"`
		Action              string `json:"action"`
		DeleteNum           int    `json:"delete_num"`
		RemoveData          bool   `json:"remove_data"`
		Expr                string `json:"expr"`
		FitTime             int    `json:"fit_time"`
		OnlyDeleteTorrent   bool   `json:"only_delete_torrent"`
		LimitSpeedBytes     int64  `json:"limit_speed_bytes"`
		ReannounceBefore    bool   `json:"reannounce_before"`
		ReannounceWaitMs    int    `json:"reannounce_wait_ms"`
		ReannounceRetries   int    `json:"reannounce_retries"`
		ReannounceIntervalMs int   `json:"reannounce_interval_ms"`
		CascadeDelete       bool   `json:"cascade_delete"`
		CascadeMaxDepth     int    `json:"cascade_max_depth"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.Alias == "" {
		Error(w, http.StatusBadRequest, 40001, "alias 为必填项")
		return
	}

	if req.Type == "expr" && req.Expr != "" {
		if err := seeding.ValidateExpr(req.Expr); err != nil {
			Error(w, http.StatusBadRequest, 14003, "表达式语法错误: "+err.Error())
			return
		}
	}

	rule := model.DeleteRule{
		Alias:               req.Alias,
		Priority:            req.Priority,
		Enabled:             req.Enabled,
		Type:                req.Type,
		Logic:               req.Logic,
		Conditions:          req.Conditions,
		Action:              req.Action,
		DeleteNum:           req.DeleteNum,
		RemoveData:          req.RemoveData,
		Expr:                req.Expr,
		FitTime:             req.FitTime,
		OnlyDeleteTorrent:   req.OnlyDeleteTorrent,
		LimitSpeedBytes:     req.LimitSpeedBytes,
		ReannounceBefore:    req.ReannounceBefore,
		ReannounceWaitMs:    req.ReannounceWaitMs,
		ReannounceRetries:   req.ReannounceRetries,
		ReannounceIntervalMs: req.ReannounceIntervalMs,
		CascadeDelete:       req.CascadeDelete,
		CascadeMaxDepth:     req.CascadeMaxDepth,
	}
	if rule.Type == "" {
		rule.Type = "normal"
	}
	if rule.Logic == "" {
		rule.Logic = "and"
	}
	if rule.Action == "" {
		rule.Action = "delete"
	}
	if rule.DeleteNum == 0 {
		rule.DeleteNum = 1
	}

	if err := dbimpl.ForceCreate(h.db, &rule); err != nil {
		Error(w, http.StatusInternalServerError, 50000, "创建规则失败")
		return
	}
	auditLog(r, "delete_rule", "create", "rule", fmt.Sprintf("%d", rule.ID), rule.Alias, "success")
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
	if v, ok := req["logic"].(string); ok {
		updates["logic"] = v
	}
	if v, ok := req["conditions"].(string); ok {
		updates["conditions"] = v
	}
	if v, ok := req["action"].(string); ok {
		updates["action"] = v
	}
	if v, ok := req["delete_num"].(float64); ok {
		updates["delete_num"] = int(v)
	}
	if v, ok := req["remove_data"].(bool); ok {
		updates["remove_data"] = v
	}
	if v, ok := req["expr"].(string); ok {
		updates["expr"] = v
	}
	if v, ok := req["fit_time"].(float64); ok {
		updates["fit_time"] = int(v)
	}
	if v, ok := req["only_delete_torrent"].(bool); ok {
		updates["only_delete_torrent"] = v
	}
	if v, ok := req["limit_speed_bytes"].(float64); ok {
		updates["limit_speed_bytes"] = int64(v)
	}
	if v, ok := req["reannounce_before"].(bool); ok {
		updates["reannounce_before"] = v
	}
	if v, ok := req["reannounce_wait_ms"].(float64); ok {
		updates["reannounce_wait_ms"] = int(v)
	}
	if v, ok := req["reannounce_retries"].(float64); ok {
		updates["reannounce_retries"] = int(v)
	}
	if v, ok := req["reannounce_interval_ms"].(float64); ok {
		updates["reannounce_interval_ms"] = int(v)
	}
	if v, ok := req["cascade_delete"].(bool); ok {
		updates["cascade_delete"] = v
	}
	if v, ok := req["cascade_max_depth"].(float64); ok {
		updates["cascade_max_depth"] = int(v)
	}
	if exprVal, ok := req["expr"].(string); ok && exprVal != "" {
		effectiveType, _ := req["type"].(string)
		if effectiveType == "" {
			effectiveType = rule.Type
		}
		if effectiveType == "expr" {
			if err := seeding.ValidateExpr(exprVal); err != nil {
				Error(w, http.StatusBadRequest, 14003, "表达式语法错误: "+err.Error())
				return
			}
		}
	}
	updates["updated_at"] = time.Now()

	if err := h.db.Model(&rule).Updates(updates).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "更新规则失败")
		return
	}
	h.db.First(&rule, id)
	auditLog(r, "delete_rule", "update", "rule", fmt.Sprintf("%d", id), rule.Alias, "success")
	Success(w, rule)
}

func (h *DeleteRuleHandler) handleDelete(w http.ResponseWriter, r *http.Request, id uint) {
	if err := h.db.Delete(&model.DeleteRule{}, id).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "删除规则失败")
		return
	}
	auditLog(r, "delete_rule", "delete", "rule", fmt.Sprintf("%d", id), "", "success")
	Success(w, nil)
}

func (h *DeleteRuleHandler) handleTestRule(w http.ResponseWriter, r *http.Request, id uint) {
	var rule model.DeleteRule
	if err := h.db.First(&rule, id).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "规则不存在")
		return
	}

	var records []model.SeedingTorrentRecord
	if err := h.db.Where("status = ?", "seeding").Limit(100).Find(&records).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询活跃种子失败")
		return
	}

	type torrentWithClient struct {
		ti       *model.TorrentInfo
		clientID string
	}
	var torrentEntries []torrentWithClient
	if h.clientMgr != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		for _, clientName := range h.clientMgr.ListClients() {
			dl, err := h.clientMgr.Get(clientName)
			if err != nil {
				continue
			}
			ts, err := dl.GetAllTorrents(ctx)
			if err != nil {
				continue
			}
			for _, t := range ts {
				torrentEntries = append(torrentEntries, torrentWithClient{ti: t, clientID: clientName})
			}
		}
	}

	recordMap := make(map[string]*model.SeedingTorrentRecord, len(records))
	for i := range records {
		recordMap[strings.ToLower(records[i].InfoHash)] = &records[i]
	}

	candidates := make([]struct {
		rec *model.SeedingTorrentRecord
		ti  *model.TorrentInfo
	}, 0)

	seen := make(map[string]bool)
	for _, entry := range torrentEntries {
		ti := entry.ti
		h := strings.ToLower(ti.Hash)
		if seen[h] {
			continue
		}
		seen[h] = true
		rec, hasRec := recordMap[strings.ToLower(ti.Hash)]
		if !hasRec {
			rec = &model.SeedingTorrentRecord{
				ClientID: entry.clientID,
				InfoHash: ti.Hash,
				Status:   model.SeedingStatusSeeding,
				Source:   "sync",
			}
		}
		candidates = append(candidates, struct {
			rec *model.SeedingTorrentRecord
			ti  *model.TorrentInfo
		}{rec: rec, ti: ti})
	}

	for _, rec := range records {
		h := strings.ToLower(rec.InfoHash)
		if seen[h] {
			continue
		}
		seen[h] = true
		candidates = append(candidates, struct {
			rec *model.SeedingTorrentRecord
			ti  *model.TorrentInfo
		}{rec: &rec, ti: nil})
	}

	matched := make([]map[string]interface{}, 0)
	for _, c := range candidates {
		hit := false

		if rule.Type == "expr" && rule.Expr != "" {
			rc := &seeding.RuleContext{
				Record:  c.rec,
				Torrent: c.ti,
				Now:     time.Now(),
			}
			ok, err := seeding.EvalExprForTest(rule.Expr, rc)
			if err == nil && ok {
				hit = true
			}
		}

		if !hit && rule.Conditions != "" {
			rc := &seeding.RuleContext{
				Record:  c.rec,
				Torrent: c.ti,
				Now:     time.Now(),
			}
			conditions := seeding.ParseConditions(rule.Conditions)
			if seeding.MatchContextWithLogic(rc, conditions, rule.Logic) {
				hit = true
			}
		}

		if hit {
			title := c.rec.TorrentID
			if c.ti != nil && c.ti.Name != "" {
				title = c.ti.Name
			}
			matched = append(matched, map[string]interface{}{
				"clientID":  c.rec.ClientID,
				"infoHash":  c.rec.InfoHash,
				"siteName":  c.rec.SiteName,
				"torrentID": c.rec.TorrentID,
				"title":     title,
			})
		}
	}

	Success(w, map[string]interface{}{
		"matched":       matched,
		"total":         len(matched),
		"totalScanned":  len(candidates),
		"rule":          rule,
	})
}

func (h *DeleteRuleHandler) handleTestRuleDryrun(w http.ResponseWriter, r *http.Request, id uint) {
	var rule model.DeleteRule
	if err := h.db.First(&rule, id).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "规则不存在")
		return
	}

	var req struct {
		TorrentName string `json:"torrentName"`
		Size        int64  `json:"size"`
		Seeders     int    `json:"seeders"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	matched := false
	reason := "未匹配"

	if rule.Type == "expr" && rule.Expr != "" {
		if err := seeding.ValidateExpr(rule.Expr); err != nil {
			reason = "表达式语法错误: " + err.Error()
		} else {
			rc := &seeding.RuleContext{
				Record: &model.SeedingTorrentRecord{
					SiteName: "test",
					Status:   model.SeedingStatusSeeding,
					Discount: model.DiscountNone,
					HasHR:    false,
					IsFree:   false,
				},
				Torrent: &model.TorrentInfo{
					Name:        req.TorrentName,
					TotalSize:   req.Size,
					NumComplete: req.Seeders,
				},
				FreeSpace: -1,
				Now:       time.Now(),
			}
			ok, err := seeding.EvalExprForTest(rule.Expr, rc)
			if err != nil {
				reason = "表达式求值错误: " + err.Error()
			} else if ok {
				matched = true
				reason = "表达式匹配: " + rule.Expr
			}
		}
	}

	if !matched && rule.Conditions != "" {
		rc := &seeding.RuleContext{
			Record: &model.SeedingTorrentRecord{
				SiteName:  "test",
				Status:    model.SeedingStatusSeeding,
				Discount:  model.DiscountNone,
				IsFree:    false,
				HasHR:     false,
				ClientID:  "test",
				TorrentID: "test",
			},
			Torrent: &model.TorrentInfo{
				Name:        req.TorrentName,
				TotalSize:   req.Size,
				NumComplete: req.Seeders,
			},
			FreeSpace: -1,
			Now:       time.Now(),
		}
		conditions := seeding.ParseConditions(rule.Conditions)
		if seeding.MatchContextWithLogic(rc, conditions, rule.Logic) {
			matched = true
			reason = fmt.Sprintf("条件匹配: %d 个条件", len(conditions))
		}
	}

	Success(w, map[string]interface{}{
		"matched": matched,
		"reason":  reason,
		"ruleId":  id,
	})
}
