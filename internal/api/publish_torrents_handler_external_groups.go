package api

import (
	"net/http"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.577 刀1: 外站组注册表 CRUD（策展通道——no_mapping 长尾收录）。
// 形态镜像 group-mappings CRUD 先例（§59.230 族）：list/create/delete +
// syncGroupLexicon 联动（新词条即写即生效·识别+展示）+ sourceDetector
// RefreshCache 联动（LookupExternalGroup 缓存重建）。

// handleListExternalGroups GET /publish/torrents/external-groups
func (h *PublishTorrentsHandler) handleListExternalGroups(w http.ResponseWriter, r *http.Request) {
	var groups []model.ExternalReleaseGroup
	if err := h.db.WithContext(r.Context()).Order("group_name").Find(&groups).Error; err != nil {
		Error(w, 500, 50000, "查询外站组注册表失败: "+err.Error())
		return
	}
	Success(w, map[string]interface{}{"items": groups, "total": len(groups)})
}

// handleCreateExternalGroup POST /publish/torrents/external-groups
// body: {group_name, home_site?, notes?}——group_name 必填·大小写不敏感去重。
func (h *PublishTorrentsHandler) handleCreateExternalGroup(w http.ResponseWriter, r *http.Request) {
	var req model.ExternalReleaseGroup
	if !decodeJSONBody(w, r, &req) {
		return
	}
	req.GroupName = strings.TrimSpace(req.GroupName)
	if req.GroupName == "" {
		Error(w, 400, 40001, "组名不能为空")
		return
	}
	// 收录纪律：注册表只收权威形态（站方标题侧）·与 mappings 互斥
	// （已映射的组是官组·不是外站组——拒绝收录防语义污染）
	var count int64
	h.db.WithContext(r.Context()).Model(&model.ReleaseGroupMapping{}).
		Where("LOWER(group_name) = LOWER(?)", req.GroupName).Count(&count)
	if count > 0 {
		Error(w, 400, 40002, "该组已映射为系统官组（release_group_mappings）·不可收录外站注册表")
		return
	}
	h.db.WithContext(r.Context()).Model(&model.ExternalReleaseGroup{}).
		Where("LOWER(group_name) = LOWER(?)", req.GroupName).Count(&count)
	if count > 0 {
		Error(w, 400, 40003, "该组已收录")
		return
	}
	req.Source = "manual"
	if err := h.db.WithContext(r.Context()).Create(&req).Error; err != nil {
		Error(w, 500, 50000, "收录失败: "+err.Error())
		return
	}
	h.syncGroupLexicon(r.Context())
	if h.sourceDetector != nil {
		h.sourceDetector.RefreshCache(r.Context())
	}
	if h.logger != nil {
		h.logger.Info("external group registered", zap.String("group", req.GroupName))
	}
	Success(w, map[string]interface{}{"item": req})
}

// handleDeleteExternalGroup DELETE /publish/torrents/external-groups/{id}
func (h *PublishTorrentsHandler) handleDeleteExternalGroup(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		Error(w, 400, 40001, "无效 ID")
		return
	}
	if err := h.db.WithContext(r.Context()).Delete(&model.ExternalReleaseGroup{}, uint(id)).Error; err != nil {
		Error(w, 500, 50000, "删除失败: "+err.Error())
		return
	}
	h.syncGroupLexicon(r.Context())
	if h.sourceDetector != nil {
		h.sourceDetector.RefreshCache(r.Context())
	}
	Success(w, map[string]interface{}{"deleted": true})
}
