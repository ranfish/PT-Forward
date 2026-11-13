package api

import (
	"net/http"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

// §59.516 v0.0.1462: source_description JS 残留一次性清洗（API-only·无前端——
// 憨憨站方 Tailwind 改版移除简介区后 description_from_range 通道产出的
// JS 挂件垃圾·删配置后不再新增·本端点清存量。source_description 发布链
// 零消费（§59.516 追记消费方审计）·全清空安全。
//
// POST /publish/seeds/fix-source-description
// 幂等·同步（1667 行 SQLite 亚秒级）·返回 {total, changed}。
func (h *PublishTorrentsHandler) handleFixSourceDescription(w http.ResponseWriter, r *http.Request) {
	// JS 残留标记（description_from_range 朴素标签剥离的穿透产物特征）
	jsMarkers := []string{"jQuery", "function ", "viewpeerlist"}

	var rows []model.TorrentMetadata
	q := h.db.WithContext(r.Context()).Model(&model.TorrentMetadata{}).
		Where("source_description != ''")
	for i, m := range jsMarkers {
		if i == 0 {
			q = q.Where("source_description LIKE ?", "%"+m+"%")
		} else {
			q = q.Or("source_description LIKE ?", "%"+m+"%")
		}
	}
	if err := q.Find(&rows).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询失败: "+err.Error())
		return
	}

	changed := 0
	for _, row := range rows {
		if err := h.db.WithContext(r.Context()).Model(&model.TorrentMetadata{}).
			Where("info_hash = ?", row.InfoHash).
			Update("source_description", "").Error; err != nil {
			if h.logger != nil {
				h.logger.Error("fix-source-desc update failed",
					zap.String("hash", row.InfoHash), zap.Error(err))
			}
			continue
		}
		changed++
	}
	if h.logger != nil {
		h.logger.Info("fix-source-description done",
			zap.Int("total", len(rows)), zap.Int("changed", changed))
	}
	Success(w, map[string]interface{}{
		"message": "清洗完成",
		"total":   len(rows),
		"changed": changed,
	})
}
