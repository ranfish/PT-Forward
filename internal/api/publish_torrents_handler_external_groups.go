package api

import (
	"net/http"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.577 刀1: 外站组注册表读端点（GET）。
// §59.578 刀3: 收录通道关闭（用户定案严格 migration-only）——
// create/delete 端点+函数撤除·收录走 migration（普查→seed→tag 模式·
// 842 普查→19 组先例）。理由：API 若留·各环境都能收录→多环境注册表
// 漂移（243 收录的组其它环境没有）——migration-only 才是单源。
// 收录后运维三步序列（§59.578 设计·必读）：migration 部署 →
// unsupported 视图批量重置（N）（存量烙印颗恢复）→ 获取未配置弹窗重取。

// handleListExternalGroups GET /publish/torrents/external-groups
func (h *PublishTorrentsHandler) handleListExternalGroups(w http.ResponseWriter, r *http.Request) {
	var groups []model.ExternalReleaseGroup
	if err := h.db.WithContext(r.Context()).Order("group_name").Find(&groups).Error; err != nil {
		Error(w, 500, 50000, "查询外站组注册表失败: "+err.Error())
		return
	}
	Success(w, map[string]interface{}{"items": groups, "total": len(groups)})
}
