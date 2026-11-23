package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
)

func (h *SiteHandler) handleOverrides(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "invalid site id")
		return
	}

	var site model.Site
	if err := h.repo.DB().First(&site, id).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "站点不存在")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleListOverrides(w, r, site.Name)
	case http.MethodPut:
		h.handleUpsertOverride(w, r, site.Name)
	case http.MethodDelete:
		h.handleDeleteOverride(w, r, site.Name)
	default:
		methodNotAllowed(w)
	}
}

func (h *SiteHandler) handleListOverrides(w http.ResponseWriter, _ *http.Request, siteName string) {
	var overrides []model.SiteConfigOverride
	if err := h.repo.DB().Where("site_name = ?", siteName).Find(&overrides).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "获取站点配置覆盖失败")
		return
	}
	Success(w, map[string]interface{}{
		"items": overrides,
		"total": len(overrides),
	})
}

func (h *SiteHandler) handleUpsertOverride(w http.ResponseWriter, r *http.Request, siteName string) {
	var req struct {
		FieldPath  string `json:"fieldPath"`
		FieldValue string `json:"fieldValue"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.FieldPath == "" {
		Error(w, http.StatusBadRequest, 40001, "fieldPath 为必填项")
		return
	}

	var existing model.SiteConfigOverride
	err := h.repo.DB().
		Where("site_name = ? AND field_path = ?", siteName, req.FieldPath).
		First(&existing).Error

	if err != nil {
		override := model.SiteConfigOverride{
			SiteName:   siteName,
			FieldPath:  req.FieldPath,
			FieldValue: req.FieldValue,
			Source:     "web_ui",
		}
		if err := h.repo.DB().Create(&override).Error; err != nil {
			Error(w, http.StatusInternalServerError, 50000, "创建覆盖失败")
			return
		}
		Success(w, map[string]interface{}{"id": override.ID, "message": "覆盖已创建"})
	} else {
		if err := h.repo.DB().Model(&existing).
			Update("field_value", req.FieldValue).Error; err != nil {
			Error(w, http.StatusInternalServerError, 50000, "更新覆盖失败")
			return
		}
		Success(w, map[string]interface{}{"id": existing.ID, "message": "覆盖已更新"})
	}
}

func (h *SiteHandler) handleDeleteOverride(w http.ResponseWriter, r *http.Request, siteName string) {
	fieldPath := r.URL.Query().Get("fieldPath")
	q := h.repo.DB().Where("site_name = ?", siteName)
	if fieldPath != "" {
		q = q.Where("field_path = ?", fieldPath)
	}
	result := q.Delete(&model.SiteConfigOverride{})
	if result.Error != nil {
		Error(w, http.StatusInternalServerError, 50000, "删除覆盖配置失败")
		return
	}
	Success(w, map[string]interface{}{
		"deleted": result.RowsAffected,
	})
}

func (h *SiteHandler) handleDomainFreezeByID(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "invalid site id")
		return
	}
	s, err := h.repo.GetByID(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, 40400, "站点不存在")
		return
	}
	h.handleDomainFreeze(w, r, s.Domain)
}

func (h *SiteHandler) handleFreezeStatus(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		freezeMap := httpclient.GlobalLimiter.GetDomainStatuses()

		var sites []model.Site
		h.db.Where("enabled = ?", true).Find(&sites)

		type freezeRow struct {
			Name   string `json:"name"`
			Domain string `json:"domain"`
			Frozen bool   `json:"frozen"`
			Reason string `json:"reason,omitempty"`
		}
		result := make([]freezeRow, 0, len(sites))
		for _, s := range sites {
			row := freezeRow{Name: s.Name, Domain: s.Domain}
			if fs, ok := freezeMap[s.Domain]; ok {
				row.Frozen = fs.Frozen
				row.Reason = fs.Reason
			}
			result = append(result, row)
		}
		Success(w, result)
	case http.MethodDelete:
		var req struct {
			Domain string `json:"domain"`
		}
		if !decodeJSONBody(w, r, &req) {
			return
		}
		if req.Domain == "" {
			Error(w, http.StatusBadRequest, 40001, "domain is required")
			return
		}
		httpclient.GlobalLimiter.ManualUnfreeze(req.Domain)
		Success(w, map[string]string{"status": "unfrozen", "domain": req.Domain})
	default:
		methodNotAllowed(w)
	}
}

func (h *SiteHandler) handleCircuitStatus(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		var circuitMap map[string]httpclient.CircuitStatus
		if httpclient.GlobalCircuitBreaker != nil {
			circuitMap = httpclient.GlobalCircuitBreaker.GetAllCircuitStatuses()
		}

		var sites []model.Site
		h.db.Where("enabled = ?", true).Find(&sites)

		type circuitRow struct {
			Name     string `json:"name"`
			Domain   string `json:"domain"`
			State    string `json:"state"`
			Failures int    `json:"failures"`
		}
		result := make([]circuitRow, 0, len(sites))
		for _, s := range sites {
			row := circuitRow{Name: s.Name, Domain: s.Domain, State: "closed"}
			if cs, ok := circuitMap[s.Domain]; ok {
				row.State = cs.State.String()
				row.Failures = cs.Failures
			}
			result = append(result, row)
		}
		Success(w, result)

	case http.MethodPost:
		if httpclient.GlobalCircuitBreaker == nil {
			Success(w, map[string]interface{}{})
			return
		}
		var req struct {
			Domain string `json:"domain"`
		}
		if !decodeJSONBody(w, r, &req) {
			return
		}
		if req.Domain == "" {
			Error(w, http.StatusBadRequest, 40001, "domain is required")
			return
		}
		httpclient.GlobalCircuitBreaker.ResetCircuit(req.Domain)
		Success(w, map[string]string{"status": "reset", "domain": req.Domain})

	default:
		methodNotAllowed(w)
	}
}

func (h *SiteHandler) handleDomainFreeze(w http.ResponseWriter, r *http.Request, domain string) {
	switch r.Method {
	case http.MethodPost:
		var req struct {
			Duration string `json:"duration"`
			Reason   string `json:"reason"`
		}
		if !decodeJSONBody(w, r, &req) {
			return
		}
		dur, err := time.ParseDuration(req.Duration)
		if err != nil {
			Error(w, http.StatusBadRequest, 40001, "invalid duration format")
			return
		}
		httpclient.GlobalLimiter.ManualFreeze(domain, dur, req.Reason)
		Success(w, map[string]string{"status": "frozen", "domain": domain})

	case http.MethodDelete:
		httpclient.GlobalLimiter.ManualUnfreeze(domain)
		Success(w, map[string]string{"status": "unfrozen", "domain": domain})

	case http.MethodGet:
		status := httpclient.GlobalLimiter.GetFreezeStatus(domain)
		Success(w, status)

	default:
		methodNotAllowed(w)
	}
}

func (h *SiteHandler) handleExclusions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		var exclusions []model.PublishExclusion
		if err := h.db.WithContext(r.Context()).Find(&exclusions).Error; err != nil {
			Error(w, http.StatusInternalServerError, 50000, "failed to list exclusions")
			return
		}
		// 先收集 DB 已有的 (source,target) 对，用于硬编码去重
		dbPairs := make(map[string]bool, len(exclusions))
		result := make([]map[string]interface{}, 0, len(exclusions)+8)
		for _, e := range exclusions {
			// §59.539 附: 同键 DB 行（硬编码机制上线前的历史手动创建）按
			// 硬编码影子展示——实际拦截语义=内置（DB 行删了代码层拦截还在，
			// 显示用户规则+可删按钮是双重谎言：DELETE 实被 403 拒）。影子
			// 行不参与删除（前端禁删按钮），展示与行为一致。
			isHC := publish.IsHardcodedExclusion(e.SourceSite, e.TargetSite) ||
				publish.IsHardcodedOneWayExclusion(e.SourceSite, e.TargetSite)
			result = append(result, map[string]interface{}{
				"id":           e.ID,
				"target_site":  e.TargetSite,
				"source_site":  e.SourceSite,
				"created_at":   e.CreatedAt,
				"is_hardcoded": isHC,
			})
			dbPairs[e.SourceSite+"→"+e.TargetSite] = true // 全收集去重（防与硬编码行双显）
		}
		for src, targets := range publish.PublishHardcodedExclusionPairs() {
			for tgt := range targets {
				key := src + "→" + tgt
				if dbPairs[key] {
					continue
				}
				result = append(result, map[string]interface{}{
					"target_site":  tgt,
					"source_site":  src,
					"is_hardcoded": true,
				})
			}
		}
		// §59.539: 硬编码单向互斥（源→目标单侧禁转·深度站定）——只读
		// 展示与双向硬编码区分（one_way 标记）。
		for src, targets := range publish.PublishHardcodedOneWayExclusions() {
			for tgt := range targets {
				key := src + "→" + tgt
				if dbPairs[key] {
					continue
				}
				result = append(result, map[string]interface{}{
					"target_site":  tgt,
					"source_site":  src,
					"is_hardcoded": true,
					"one_way":      true,
				})
			}
		}
		Success(w, result)

	case http.MethodPost:
		var req struct {
			TargetSite string `json:"target_site"`
			SourceSite string `json:"source_site"`
		}
		if !decodeJSONBody(w, r, &req) {
			return
		}
		if req.TargetSite == "" || req.SourceSite == "" {
			Error(w, http.StatusBadRequest, 40001, "target_site and source_site are required")
			return
		}
		exclusion := model.PublishExclusion{
			TargetSite: req.TargetSite,
			SourceSite: req.SourceSite,
		}
		if err := h.db.WithContext(r.Context()).Create(&exclusion).Error; err != nil {
			if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "UNIQUE") {
				Error(w, http.StatusConflict, 40900, "exclusion already exists")
				return
			}
			Error(w, http.StatusInternalServerError, 50000, "failed to create exclusion")
			return
		}
		Success(w, exclusion)

	case http.MethodDelete:
		var req struct {
			TargetSite string `json:"target_site"`
			SourceSite string `json:"source_site"`
		}
		if !decodeJSONBody(w, r, &req) {
			return
		}
		if publish.IsHardcodedExclusion(req.SourceSite, req.TargetSite) {
			Error(w, http.StatusForbidden, 40300, "硬编码互斥规则不可删除")
			return
		}
		// §59.539 回归审核 P1 修正：单向硬编码不加 DELETE 拦截——DELETE 只
		// 影响 DB 行（硬编码拦截在代码层 Eligibility·不依赖 DB 行存在）；
		// 若拦截会造成用户自建同键行死锁（POST 无拦可建→展示被硬编码去重
		// 吞→DELETE 被拒永删不掉）。用户行该删就删·删后单向拦截依然生效。
		result := h.db.WithContext(r.Context()).
			Where("target_site = ? AND source_site = ?", req.TargetSite, req.SourceSite).
			Delete(&model.PublishExclusion{})
		if result.Error != nil {
			Error(w, http.StatusInternalServerError, 50000, "删除排除规则失败")
			return
		}
		if result.RowsAffected == 0 {
			Error(w, http.StatusNotFound, 40400, "exclusion not found")
			return
		}
		Success(w, map[string]string{"status": "deleted"})

	default:
		methodNotAllowed(w)
	}
}
