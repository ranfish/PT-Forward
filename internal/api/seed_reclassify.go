package api

import (
	"context"
	"fmt"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"go.uber.org/zap"
)

// §59.598（classify 六态物化）: 重算公共单点——八钩子全经此（B1）。
//
// 物化=classifySeedStatusLite（唯一判定源）的缓存结果。本函数对选中的
// 快照行重算 status/group_name/no_transfer_until 三列并写回。所有失效
// 触发钩子（meta 写点/审核/清除/烙印/外部表 CRUD）统一调这里——禁止
// 各处手写 UPDATE（§59.26 漂移预防）。
//
// syncer（跨包）不走这里：新行 status 留空 → 直筛自愈通道补算（B2）。
//
// 批量语义：WHERE 形态选择（hash 精确 / 组名反查索引）——大环境（22 24606
// 行/路径）下 reclassifyAll 慎用（仅 migration backfill/自愈用）。
func (h *PublishTorrentsHandler) reclassifySnapshots(ctx context.Context, where string, args ...interface{}) error {
	var snaps []model.TorrentSnapshot
	if err := h.db.WithContext(ctx).Where(where, args...).Find(&snaps).Error; err != nil {
		return fmt.Errorf("reclassify: load snapshots: %w", err)
	}
	return h.reclassifyRows(ctx, snaps)
}

// reclassifyRows 按行集重算并写回（分批 UPDATE·单事务每批）。
func (h *PublishTorrentsHandler) reclassifyRows(ctx context.Context, snaps []model.TorrentSnapshot) error {
	for i := range snaps {
		snap := &snaps[i]
		var meta *model.TorrentMetadata
		var m model.TorrentMetadata
		if err := h.db.WithContext(ctx).Where("info_hash = ?", snap.Hash).First(&m).Error; err == nil {
			meta = &m
		}
		// isMusic 输入：与列表口径同源（§59.299 detail_source_json 的 music 键
		// 在场判定——music_metas 独立表不存在·音乐信息在 metadata 的 JSON 列）。
		isMusic := false
		if meta != nil {
			var count int64
			h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
				Where("info_hash = ? AND detail_source_json LIKE ?", meta.InfoHash, `%"music":{%`).
				Count(&count)
			isMusic = count > 0
		}
		status := h.classifySeedStatusLite(ctx, snap.Name, meta, isMusic, snap.FetchFailedAt != nil)
		groupName := publish.ExtractGroupName(snap.Name)
		if groupName == "" && meta != nil {
			groupName = publish.ExtractGroupName(meta.Title)
			if groupName == "" {
				groupName = groupFromReleaseGroupSlot(meta.ReleaseGroup)
			}
		}
		var until *time.Time
		if meta != nil {
			until = meta.NoTransferUntil
		}
		// 写回（仅变更时——避免无谓 UPDATE 放大 IO）
		if snap.Status != status || snap.GroupName != groupName || !timePtrEqual(snap.NoTransferUntil, until) {
			if err := h.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
				Where("hash = ? AND client_uid = ?", snap.Hash, snap.ClientUID).
				Updates(map[string]interface{}{
					"status":            status,
					"group_name":        groupName,
					"no_transfer_until": until,
				}).Error; err != nil {
				h.logger.Warn("reclassify: update snapshot failed",
					zap.String("hash", snap.Hash), zap.Uint("client_uid", snap.ClientUID), zap.Error(err))
			}
		}
	}
	return nil
}

// reclassifyByGroup O5: 外部表（组映射/外站注册表）CRUD 钩子——按物化
// group_name 列索引反查受影响行（免全表 LIKE 扫）。
func (h *PublishTorrentsHandler) reclassifyByGroup(ctx context.Context, groupName string) {
	if groupName == "" {
		return
	}
	if err := h.reclassifySnapshots(ctx, "group_name = ?", groupName); err != nil {
		h.logger.Warn("reclassify by group failed", zap.String("group", groupName), zap.Error(err))
	}
}

func timePtrEqual(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}
