package seeding

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/client"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

var defaultUnregisteredKeywords = []string{
	"unregistered torrent",
	"unregistered",
	"torrent not found",
	"torrent not exist",
	"not registered",
	"unknown torrent",
	"invalid torrent",
	"torrent has been deleted",
}

func (e *Engine) getUnregisteredKeywords() []string {
	var val string
	row := e.db.Raw("SELECT value FROM system_settings WHERE key = 'seeding.unregistered_keywords' LIMIT 1").Row()
	if err := row.Scan(&val); err != nil || val == "" {
		return defaultUnregisteredKeywords
	}
	var keywords []string
	if json.Unmarshal([]byte(val), &keywords) == nil && len(keywords) > 0 {
		return keywords
	}
	return defaultUnregisteredKeywords
}

// checkUnregisteredTorrents §59.31 v2: 幽灵种子巡检——纯检测组件。
//
// 只标记（unregistered=true + msg 快照 + tracker 域名诊断），不做任何处置；
// 处置完全交给 /rules 删种规则体系（用户绑定到 /seeding 任务或 /downloads 下载器，
// 未绑定则无为）。
//
// 判定唯一权威：tracker msg 文本 × 关键词表（seeding.unregistered_keywords，
// /seeding/scoring UI 用户可编辑）。任一 tracker msg 命中即标记（PT 无跨站
// 多 tracker——那是作弊行为；多 tracker 必同站官方镜像，任一命中=站点删种）。
//
// 三档调度（按框架/版本自动分档）：
//   - TR：torrentMap 自带全部 tracker 消息（TrackerMsgs），每轮全量关键词匹配，
//     零额外请求、无冷却
//   - qb 5.2+：maindata HasTrackerError 圈怀疑池（仅调度信号，永不参与判定），
//     池内逐个 GetTrackerMessagesAll；查过不命中冷却 30 分钟
//   - qb <5.2：无可靠免费信号（不做启发式推断——宁可慢不可错），全量低频轮询：
//     每 10s tick 批 N=50（patrol.batch_size 可配），周期指针循环
//
// 标记单向：已标记种子跳过后续扫描（检测职责完成，移交规则）；
// 闭环出口=规则删种 → syncStaleRecords 回收 → 扫描池收缩。
func (e *Engine) checkUnregisteredTorrents(ctx context.Context, clientUID uint, dlClient model.DownloaderClient, torrentMap map[string]*model.TorrentInfo) {
	if !e.unregisteredChecking.CompareAndSwap(false, true) {
		return
	}
	defer e.unregisteredChecking.Store(false)

	keywords := e.getUnregisteredKeywords()
	if len(keywords) == 0 {
		return
	}

	// 收集本客户端的候选记录（活跃做种且未标记）
	e.mu.RLock()
	var candidates []*model.SeedingTorrentRecord
	for _, rec := range e.recordMap {
		if rec.ClientUID == clientUID && !rec.Unregistered &&
			(rec.Status == model.SeedingStatusSeeding || rec.Status == model.SeedingStatusPausedFreeEnd || rec.Status == model.SeedingStatusPausedRule) {
			candidates = append(candidates, rec)
		}
	}
	e.mu.RUnlock()

	if len(candidates) == 0 {
		return
	}

	framework := e.clientFramework(clientUID)
	switch framework {
	case "transmission":
		// TR 档：torrentMap 自带全部消息，全量匹配（零额外请求）
		e.patrolMatchFromTorrentMap(ctx, clientUID, candidates, torrentMap, keywords)
	case "qbittorrent":
		// qb 档（§59.31 审计修复 #1/#2）：5.2+ 信号字段只做调度加速，
		// 不改变覆盖语义——每个 tick 同时跑两路：
		//   ① 怀疑池即时确认（有信号者，带冷却 + 单 tick 上限防爆）
		//   ② 周期性全量兜底批扫（无信号者按游标轮转，覆盖
		//      PausedFreeEnd/PausedRule 不 announce → 信号恒零的候选，
		//      以及增量 maindata 信号被清零的窗口）
		// 旧版 qb（信号恒零）自然退化为纯 ②——与设计 <5.2 档一致。
		var suspects []*model.SeedingTorrentRecord
		var rest []*model.SeedingTorrentRecord
		for _, rec := range candidates {
			// §59.31 四审：torrentMap 键恒为小写（与 syncStaleRecords 等消费点一致，
			// 防 TR 2.x 大写 hashString 记录被静默跳过）
			ti, ok := torrentMap[strings.ToLower(rec.InfoHash)]
			if ok && (ti.HasTrackerError || ti.HasTrackerWarning || ti.HasOtherAnnounceError) {
				suspects = append(suspects, rec)
			} else {
				rest = append(rest, rec)
			}
		}
		if len(suspects) > 0 {
			e.patrolConfirmBatch(ctx, clientUID, dlClient, suspects, keywords)
		}
		if len(rest) > 0 {
			e.patrolFullScanBatch(ctx, clientUID, dlClient, rest, keywords)
		}
	default:
		// 未知框架：全量轮询兜底
		e.patrolFullScanBatch(ctx, clientUID, dlClient, candidates, keywords)
	}
}

// patrolMatchFromTorrentMap TR 档：torrentMap 全量消息关键词匹配（零额外请求）。
func (e *Engine) patrolMatchFromTorrentMap(ctx context.Context, clientUID uint, candidates []*model.SeedingTorrentRecord, torrentMap map[string]*model.TorrentInfo, keywords []string) {
	for _, rec := range candidates {
		if ctx.Err() != nil {
			return
		}
		ti, ok := torrentMap[strings.ToLower(rec.InfoHash)]
		if !ok || len(ti.TrackerMsgs) == 0 {
			continue
		}
		for _, tm := range ti.TrackerMsgs {
			msg := strings.TrimSpace(tm.Msg)
			if msg == "" || strings.EqualFold(msg, "success") || strings.EqualFold(msg, "ok") {
				continue
			}
			if hitKeyword(msg, keywords) {
				e.markUnregistered(ctx, rec, msg, tm.TrackerDomain())
				break
			}
		}
	}
}

// patrolConfirmBatch qb 5.2+ 档：怀疑池逐个确认（带冷却 + 单 tick 上限）。
// 冷却（遗漏 A）：查过不命中 30 分钟内不重查，防 tracker 站点故障反复浪费额度。
// 单 tick 上限（审计 #3）：站点故障爆发首 tick 冷却表为空，若不限量会全池
// 串行打完并阻塞 refreshMaindataLoop——限 50/tick，溢出留待下 tick（此时
// 已有冷却记录，不重复）。
func (e *Engine) patrolConfirmBatch(ctx context.Context, clientUID uint, dlClient trackerMsgGetter, suspects []*model.SeedingTorrentRecord, keywords []string) {
	now := time.Now()
	cooldown := e.patrolCooldownDuration()
	batchSize := e.patrolBatchSize() // §59.31 三审 NEW-F：外提，防循环内逐候选 SQL（#7 模式）
	checked := 0
	for _, rec := range suspects {
		if ctx.Err() != nil {
			return
		}
		if checked >= batchSize {
			return
		}
		if e.inUnregCooldown(clientUID, rec.InfoHash, now, cooldown) {
			continue
		}
		// §59.31 二审 NEW-2：尝试即计数（含失败）——错误风暴（CSRF 过期/客户端
		// 抖动导致逐个失败）同样消耗 tick 预算，防单 tick 串行打满全池
		checked++
		msgs, err := dlClient.GetTrackerMessagesAll(ctx, rec.InfoHash)
		if err != nil {
			continue
		}
		matched := false
		for _, tm := range msgs {
			msg := strings.TrimSpace(tm.Msg)
			if msg == "" {
				continue
			}
			if hitKeyword(msg, keywords) {
				e.markUnregistered(ctx, rec, msg, tm.TrackerDomain())
				matched = true
				break
			}
		}
		if !matched {
			e.setUnregCooldown(clientUID, rec.InfoHash, now)
		}
	}
}

// patrolFullScanBatch 全量兜底轮询（批 N，per-client 周期指针循环）。
// qb <5.2 主路径；qb 5.2+ 的无信号候选兜底（审计 #1）。
func (e *Engine) patrolFullScanBatch(ctx context.Context, clientUID uint, dlClient trackerMsgGetter, candidates []*model.SeedingTorrentRecord, keywords []string) {
	// §59.31 二审 A：按 InfoHash 排序使游标为确定性轮转（recordMap 迭代序随机，
	// 不排序时游标是对随机排列的滑窗采样，最坏覆盖延迟 ~2× 标称周期）
	sorted := make([]*model.SeedingTorrentRecord, len(candidates))
	copy(sorted, candidates)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].InfoHash < sorted[j].InfoHash })
	candidates = sorted

	batchSize := e.patrolBatchSize()
	e.patrolMu.Lock()
	if e.patrolCursors == nil {
		e.patrolCursors = make(map[uint]int)
	}
	cursor := e.patrolCursors[clientUID]
	if cursor >= len(candidates) {
		cursor = 0
	}
	end := cursor + batchSize
	if end > len(candidates) {
		end = len(candidates)
	}
	e.patrolCursors[clientUID] = end
	e.patrolMu.Unlock()
	batch := candidates[cursor:end]

	for _, rec := range batch {
		if ctx.Err() != nil {
			return
		}
		msgs, err := dlClient.GetTrackerMessagesAll(ctx, rec.InfoHash)
		if err != nil {
			continue
		}
		for _, tm := range msgs {
			msg := strings.TrimSpace(tm.Msg)
			if msg == "" {
				continue
			}
			if hitKeyword(msg, keywords) {
				e.markUnregistered(ctx, rec, msg, tm.TrackerDomain())
				break
			}
		}
	}
}

// markUnregistered §59.31: 标记幽灵种子（纯标记，无副作用——不暂停不删除）。
func (e *Engine) markUnregistered(ctx context.Context, rec *model.SeedingTorrentRecord, msg, trackerDomain string) {
	e.logger.Info("unregistered torrent detected",
		zap.Uint("client_uid", rec.ClientUID),
		zap.String("site", rec.SiteName),
		zap.String("torrent_id", rec.TorrentID),
		zap.String("info_hash", rec.InfoHash),
		zap.String("tracker", trackerDomain),
		zap.String("tracker_msg", msg),
		zap.String("disposition", "marked (rules consume)"))

	now := time.Now()
	if err := e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("id = ?", rec.ID).
		Updates(map[string]interface{}{
			"unregistered":         true,
			"unregistered_at":      now,
			"unregistered_msg":     msg,
			"unregistered_tracker": trackerDomain,
		}).Error; err != nil {
		e.logger.Warn("unregistered: update DB failed", zap.Error(err))
		return
	}

	e.mu.Lock()
	rec.Unregistered = true
	rec.UnregisteredAt = &now
	rec.UnregisteredMsg = msg
	rec.UnregisteredTracker = trackerDomain
	e.mu.Unlock()
}

// hitKeyword msg 文本 × 关键词表（大小写不敏感 Contains）。
func hitKeyword(msg string, keywords []string) bool {
	msgLower := strings.ToLower(msg)
	for _, kw := range keywords {
		if kw == "" {
			continue
		}
		if strings.Contains(msgLower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// trackerMsgGetter §59.31：巡检 batch 函数的最小依赖（便于单测 mock）。
type trackerMsgGetter interface {
	GetTrackerMessagesAll(ctx context.Context, hash string) ([]model.TrackerMessage, error)
}

// patrolCooldown §59.31 遗漏 A：怀疑池冷却状态（内存态，重启重建，不持久化）。
type patrolCooldownEntry struct {
	lastChecked time.Time
}

func (e *Engine) inUnregCooldown(clientUID uint, hash string, now time.Time, cooldown time.Duration) bool {
	e.patrolMu.Lock()
	defer e.patrolMu.Unlock()
	key := fmt.Sprintf("%d|%s", clientUID, hash)
	if entry, ok := e.patrolCooldowns[key]; ok {
		return now.Sub(entry.lastChecked) < cooldown
	}
	return false
}

func (e *Engine) setUnregCooldown(clientUID uint, hash string, now time.Time) {
	e.patrolMu.Lock()
	defer e.patrolMu.Unlock()
	if e.patrolCooldowns == nil {
		e.patrolCooldowns = make(map[string]*patrolCooldownEntry)
	}
	e.patrolCooldowns[fmt.Sprintf("%d|%s", clientUID, hash)] = &patrolCooldownEntry{lastChecked: now}
}

// patrolCooldownDuration 冷却时长（默认 30 分钟；全局可配 patrol.recheck_cooldown_min）。
func (e *Engine) patrolCooldownDuration() time.Duration {
	minutes := 30
	var val string
	row := e.db.Raw("SELECT value FROM system_settings WHERE key = 'patrol.recheck_cooldown_min' LIMIT 1").Row()
	if row != nil && row.Scan(&val) == nil && val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			minutes = n
		}
	}
	return time.Duration(minutes) * time.Minute
}

// patrolBatchSize 全量轮询批量（默认 50；全局可配 patrol.batch_size）。
func (e *Engine) patrolBatchSize() int {
	var val string
	row := e.db.Raw("SELECT value FROM system_settings WHERE key = 'patrol.batch_size' LIMIT 1").Row()
	if row != nil && row.Scan(&val) == nil && val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			return n
		}
	}
	return 50
}

// clientFramework 查下载器框架（clients.type：qbittorrent/transmission）。
// 每客户端每 tick 一次点查，结果语义稳定。
func (e *Engine) clientFramework(clientUID uint) string {
	var t string
	// §59.508 附二: WHERE name → id——§59.251 UID 化重构（2026-09-19）
	// 签名 name→uid 但 SQL 列未跟改，uint 查 name 列恒 miss → framework
	// 恒空 → patrol 幽灵巡检 switch 两档全不匹配 → qb/tr 全部下载器
	// 的 tracker 失联检测自 09-19 起静默失效两个月（零报错零日志=
	// 铁律"静默错"标准形态·UID 化漏网第六处）。
	if err := e.db.Raw("SELECT type FROM clients WHERE id = ? AND deleted_at IS NULL LIMIT 1", clientUID).Row().Scan(&t); err != nil {
		return ""
	}
	return t
}

// checkAutoTransfer 检测下载完成的 record，触发自动转移（§55.11 方案B）。
// 在 refreshMaindataOnce 里调用，复用已有的 torrentMap（10s 轮询），零额外网络成本。
func (e *Engine) checkAutoTransfer(ctx context.Context, clientUID uint, torrentMap map[string]*model.TorrentInfo, dlClient model.DownloaderClient) {
	if dlClient == nil {
		return
	}
	deadStates := map[string]bool{"stalledDL": true, "missingFiles": true, "error": true, "unknown": true}

	e.mu.RLock()
	var pending []string
	for key, rec := range e.recordMap {
		if rec.ClientUID != clientUID || !rec.AutoTransfer || rec.Status != model.SeedingStatusSeeding || len(rec.TransferClientUIDs) == 0 {
			continue
		}
		ti, ok := torrentMap[strings.ToLower(rec.InfoHash)]
		if !ok || ti == nil || ti.Progress < 1.0 || deadStates[ti.State] {
			continue
		}
		pending = append(pending, key)
	}
	e.mu.RUnlock()

	for _, key := range pending {
		// CAS 状态 seeding→transferring（防 refreshMaindataLoop 重复触发）
		e.mu.Lock()
		rec, ok := e.recordMap[key]
		if !ok || rec.Status != model.SeedingStatusSeeding {
			e.mu.Unlock()
			continue
		}
		rec.Status = model.SeedingStatusTransferring
		e.mu.Unlock()
		e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).Where("id = ?", rec.ID).Update("status", model.SeedingStatusTransferring)

		ti := torrentMap[strings.ToLower(rec.InfoHash)]
		if ti == nil {
			e.revertTransferring(ctx, key, rec.ID)
			continue
		}
		recCopy := *rec
		// §59.562 刀4: 转移任务派生自巡检 loop ctx（refreshCtx 族）——
		// 停机/Stop cancel 中止在途转移；transferring 中断回退 seeding
		// 是既有设计（Start 回滚+下轮重试）·非新增风险面。
		go e.transferRecord(ctx, recCopy, key)
	}
}

// transferRecord 转移种子到目标下载器：遍历目标 client.TransferTorrent → 删源(deleteData=false) → 删 record
func (e *Engine) transferRecord(ctx context.Context, rec model.SeedingTorrentRecord, key string) {
	if e.clientProvider == nil {
		e.logger.Warn("auto transfer: clientProvider nil", zap.String("hash", rec.InfoHash))
		e.revertTransferring(ctx, key, rec.ID)
		return
	}
	sourceClient, err := e.clientProvider.Get(rec.ClientUID)
	if err != nil {
		e.logger.Warn("auto transfer: get source client failed", zap.Uint("client_uid", rec.ClientUID), zap.Error(err))
		e.revertTransferring(ctx, key, rec.ID)
		return
	}

	successCount := 0
	for _, targetID := range rec.TransferClientUIDs {
		targetClient, err := e.clientProvider.Get(targetID)
		if err != nil {
			e.logger.Warn("auto transfer: get target client failed", zap.Uint("target_uid", targetID), zap.Error(err))
			continue
		}
		if _, err := client.TransferTorrent(ctx, sourceClient, targetClient, rec.InfoHash); err != nil {
			e.logger.Warn("auto transfer: transfer to target failed", zap.Uint("target_uid", targetID), zap.String("hash", rec.InfoHash), zap.Error(err))
			continue
		}
		successCount++
		e.logger.Info("auto transfer: added to target", zap.Uint("target_uid", targetID), zap.String("hash", rec.InfoHash))
	}

	if successCount == 0 {
		e.logger.Warn("auto transfer: all targets failed, reverting", zap.String("hash", rec.InfoHash))
		e.revertTransferring(ctx, key, rec.ID)
		return
	}

	// 删除源种子（deleteData=false！数据已移交目标下载器，不能删文件）
	if err := sourceClient.DeleteTorrent(ctx, rec.InfoHash, false); err != nil {
		e.logger.Warn("auto transfer: delete source failed (targets already added)", zap.String("hash", rec.InfoHash), zap.Error(err))
	}

	// 删 record（源种子已移交，seedingEngine 不再管理）
	e.mu.Lock()
	delete(e.recordMap, key)
	e.mu.Unlock()
	e.db.WithContext(ctx).Where("id = ?", rec.ID).Delete(&model.SeedingTorrentRecord{})

	e.logger.Info("auto transfer: completed",
		zap.String("hash", rec.InfoHash),
		zap.Uint("source", rec.ClientUID),
		zap.Int("targets", successCount))
}

// revertTransferring 转移失败回退 seeding，下轮 refreshMaindataLoop 重试
func (e *Engine) revertTransferring(ctx context.Context, key string, recID uint) {
	e.mu.Lock()
	if rec, ok := e.recordMap[key]; ok && rec.Status == model.SeedingStatusTransferring {
		rec.Status = model.SeedingStatusSeeding
	}
	e.mu.Unlock()
	e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).Where("id = ?", recID).Update("status", model.SeedingStatusSeeding)
}
