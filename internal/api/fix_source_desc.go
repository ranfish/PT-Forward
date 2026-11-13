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
// v0.0.1463 修复：逐行 UPDATE 在生产库（19310 行·忙库 WAL）120s 超时——
// 根因为 1667 次 GORM 单行 Update+每次 WAL 写入。改为单条批量 UPDATE
// （一条 SQL 原子完成·SQLite 全表扫描+LIKE 三模式评估仍在秒级）。
//
// POST /publish/seeds/fix-source-description
// 幂等·同步·返回 {total, changed}。
func (h *PublishTorrentsHandler) handleFixSourceDescription(w http.ResponseWriter, r *http.Request) {
	// JS 残留标记（description_from_range 朴素标签剥离的穿透产物特征）
	const jsWhere = `(source_description LIKE '%jQuery%' OR source_description LIKE '%function %' OR source_description LIKE '%viewpeerlist%') AND source_description != ''`

	// 预筛计数（与 UPDATE 同 WHERE·前端提示用）
	var total int64
	if err := h.db.WithContext(r.Context()).Model(&model.TorrentMetadata{}).
		Where(jsWhere).
		Count(&total).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询失败: "+err.Error())
		return
	}
	if total == 0 {
		Success(w, map[string]interface{}{"message": "无残留", "total": 0, "changed": 0})
		return
	}

	// 单条批量 UPDATE（原子·一条 WAL 写入）
	res := h.db.WithContext(r.Context()).Model(&model.TorrentMetadata{}).
		Where(jsWhere).
		Update("source_description", "")
	if res.Error != nil {
		Error(w, http.StatusInternalServerError, 50000, "清洗失败: "+res.Error.Error())
		return
	}
	if h.logger != nil {
		h.logger.Info("fix-source-description done",
			zap.Int64("total", total), zap.Int64("changed", res.RowsAffected))
	}
	Success(w, map[string]interface{}{
		"message": "清洗完成",
		"total":   total,
		"changed": res.RowsAffected,
	})
}
