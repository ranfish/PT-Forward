package seeding

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/dispatcher"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/util"
	"go.uber.org/zap"
	"gorm.io/gorm/clause"
)

func (e *Engine) AddSeedingRecord(ctx context.Context, record *model.SeedingTorrentRecord) error {
	if record.ClientUID == 0 || record.InfoHash == "" {
		return &model.AppError{Code: 40001, Message: "client_id and info_hash are required"}
	}

	key := recordKey(record.ClientUID, record.InfoHash)

	e.mu.Lock()
	if _, exists := e.recordMap[key]; exists {
		e.mu.Unlock()
		return &model.AppError{Code: 40900, Message: fmt.Sprintf("seeding record already exists: %s", key)}
	}

	if err := e.db.WithContext(ctx).Create(record).Error; err != nil {
		e.mu.Unlock()
		return err
	}

	e.recordMap[key] = record
	e.mu.Unlock()

	if e.freeEndMonitor != nil {
		e.freeEndMonitor.Schedule(record)
	}
	return nil
}

func (e *Engine) RemoveSeedingRecord(ctx context.Context, clientUID uint, infoHash string) error {
	e.logger.Info("RemoveSeedingRecord: manually removing seeding record",
		zap.Uint("client_uid", clientUID),
		zap.String("info_hash", infoHash))
	if err := e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("client_uid = ? AND info_hash = ?", clientUID, infoHash).
		Update("status", model.SeedingStatusDeleted).Error; err != nil {
		return err
	}

	e.mu.Lock()
	delete(e.recordMap, recordKey(clientUID, infoHash))
	e.mu.Unlock()

	if e.freeEndMonitor != nil {
		e.freeEndMonitor.Cancel(clientUID, infoHash)
	}

	return nil
}

func (e *Engine) GetActiveCount(clientUID uint) int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	count := 0
	for _, r := range e.recordMap {
		if r.ClientUID == clientUID && r.Status == model.SeedingStatusSeeding {
			count++
		}
	}
	return count
}

func (e *Engine) GetRecord(clientUID uint, infoHash string) (*model.SeedingTorrentRecord, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	r, ok := e.recordMap[recordKey(clientUID, infoHash)]
	if !ok {
		return nil, false
	}
	return r, true
}

func (e *Engine) ListByClient(ctx context.Context, clientUID uint) ([]model.SeedingTorrentRecord, error) {
	var records []model.SeedingTorrentRecord
	err := e.db.WithContext(ctx).
		Where("client_uid = ? AND status IN ?", clientUID,
			[]string{string(model.SeedingStatusPending), string(model.SeedingStatusSeeding), "paused_free_end", "paused_rule", "delete_failed"}).
		Find(&records).Error
	return records, err
}

func (e *Engine) saveFinalTraffic(ctx context.Context, rec *model.SeedingTorrentRecord, ti *model.TorrentInfo) {
	if ti == nil || rec == nil {
		return
	}

	traffic := &model.TorrentTraffic{
		ClientUID:     rec.ClientUID,
		InfoHash:      rec.InfoHash,
		SiteName:      rec.SiteName,
		Uploaded:      ti.Uploaded,
		Downloaded:    ti.Downloaded,
		UploadSpeed:   ti.UploadSpeed,
		DownloadSpeed: ti.DownloadSpeed,
		Ratio:         ti.Ratio,
		RecordedAt:    time.Now(),
	}
	if err := e.db.WithContext(ctx).Create(traffic).Error; err != nil {
		e.logger.Warn("save final torrent_traffic failed",
			zap.String("infoHash", rec.InfoHash),
			zap.Error(err))
	}

	if err := e.db.WithContext(ctx).Model(rec).Updates(map[string]interface{}{
		"final_uploaded":   ti.Uploaded,
		"final_downloaded": ti.Downloaded,
	}).Error; err != nil {
		e.logger.Warn("save final_uploaded to record failed",
			zap.Uint("id", rec.ID),
			zap.Error(err))
	}
}

func (e *Engine) UpdateStatus(ctx context.Context, id uint, status model.SeedingTorrentStatus, actionBy string) error {
	if err := e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":         status,
			"last_action_by": actionBy,
			"updated_at":     time.Now(),
		}).Error; err != nil {
		return err
	}

	e.mu.Lock()
	for _, r := range e.recordMap {
		if r.ID == id {
			r.Status = status
			r.LastActionBy = actionBy
			break
		}
	}
	e.mu.Unlock()
	return nil
}

func (e *Engine) PauseForFreeEnd(ctx context.Context, clientUID uint, infoHash string) error {
	key := recordKey(clientUID, infoHash)
	e.mu.Lock()
	if r, ok := e.recordMap[key]; ok {
		r.Status = model.SeedingStatusPausedFreeEnd
		r.IsFree = false
	}
	e.mu.Unlock()

	return e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("client_uid = ? AND info_hash = ?", clientUID, infoHash).
		Updates(map[string]interface{}{
			"status":         model.SeedingStatusPausedFreeEnd,
			"is_free":        false,
			"last_action_by": "free_end_pauser",
		}).Error
}

func (e *Engine) MarkFreeExpired(ctx context.Context, clientUID uint, infoHash string) error {
	return e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("client_uid = ? AND info_hash = ?", clientUID, infoHash).
		Updates(map[string]interface{}{
			"is_free":        false,
			"last_action_by": "free_end_keeper",
		}).Error
}

type ManagedCounts struct {
	Active int `json:"active"`
	Paused int `json:"paused"`
}

func (e *Engine) GetManagedCounts() *ManagedCounts {
	e.mu.RLock()
	defer e.mu.RUnlock()

	counts := &ManagedCounts{}
	for _, r := range e.recordMap {
		switch r.Status {
		case model.SeedingStatusSeeding:
			counts.Active++
		case model.SeedingStatusPausedFreeEnd, model.SeedingStatusPausedRule:
			counts.Paused++
		}
	}
	return counts
}

func (e *Engine) TotalActiveCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	count := 0
	for _, r := range e.recordMap {
		if r.Status == model.SeedingStatusSeeding {
			count++
		}
	}
	return count
}

// OnTorrents is a legacy dispatcher handler. Seeding Engine now receives events
// via EventBus (OnPushed → pendingEvents → consumeLoop). This method is kept
// for backward compatibility with subscriptions that have no ClientID.
//
// Deprecated: Use pusher.EventBus + OnPushed instead.
func (e *Engine) OnTorrents(ctx context.Context, events []model.TorrentEvent) error {
	subIDs := make([]string, 0)
	for i := range events {
		if events[i].SourceID != "" {
			subIDs = append(subIDs, events[i].SourceID)
		}
	}

	subMap := make(map[string]*model.RSSSubscription, len(subIDs))
	if len(subIDs) > 0 {
		var subs []model.RSSSubscription
		if err := e.db.WithContext(ctx).Where("id IN ?", subIDs).Find(&subs).Error; err != nil {
			e.logger.Warn("query subscriptions for scoring", zap.Error(err))
		}
		for i := range subs {
			subMap[fmt.Sprintf("%d", subs[i].ID)] = &subs[i]
		}
	}

	for i := range events {
		ev := &events[i]
		if ev.SourceID == "" {
			continue
		}

		clientUID := dispatcher.GetClientUID(ev)

		isQualified := ev.Discount == model.DiscountFree || ev.Discount == model.Discount2xFree

		if !isQualified {
			sub, ok := subMap[ev.SourceID]
			if ok {
				if sub.ScoringConfig.Include2xUp && (ev.Discount == model.Discount2xUp || ev.Discount == model.Discount2x50) {
					isQualified = true
				}

				if !isQualified && sub.FreeWaitEnabled {
					var checkBefore *time.Time
					if sub.FreeWaitMaxWaitSec > 0 {
						deadline := time.Now().Add(time.Duration(sub.FreeWaitMaxWaitSec) * time.Second)
						checkBefore = &deadline
					}
					e.freeWaitMonitor.Add(ev.SiteName, ev.TorrentID, ev.InfoHash, ev.Title, ev.Size, checkBefore, clientUID, ev.SourceID, ev.HasHR, ev.HRSeedTimeH, sub.FreeWaitRecheckSec, sub.FreeWaitMinRemain)
					continue
				}
			}
		}

		if !isQualified {
			continue
		}

		record := &model.SeedingTorrentRecord{
			ClientUID:      clientUID,
			InfoHash:       ev.InfoHash,
			SiteName:       ev.SiteName,
			TorrentID:      ev.TorrentID,
			Status:         model.SeedingStatusPending,
			Source:         "rss",
			SubscriptionID: ev.SourceID,
			Discount:       ev.Discount,
			IsFree:         ev.Discount == model.DiscountFree || ev.Discount == model.Discount2xFree,
			FreeEndAt:      ev.FreeEndAt,
			TorrentSize:    ev.Size,
		}
		if ev.HasHR {
			record.HasHR = true
			record.HRSeedTimeH = ev.HRSeedTimeH
		}

		if err := e.AddSeedingRecord(ctx, record); err != nil {
			e.logger.Debug("skip adding seeding record (may already exist)",
				zap.String("info_hash", ev.InfoHash),
				zap.Error(err),
			)
		}
	}
	return nil
}

func (e *Engine) ListConfigs(ctx context.Context) ([]*model.SeedingClientConfig, error) {
	var configs []*model.SeedingClientConfig
	if err := e.db.WithContext(ctx).Where("enabled = ?", true).Find(&configs).Error; err != nil {
		return nil, err
	}
	for _, c := range configs {
		if c.Role == "" {
			c.Role = "seeding"
		}
	}
	return configs, nil
}

func (e *Engine) GetConfigByID(ctx context.Context, id uint) (*model.SeedingClientConfig, error) {
	var cfg model.SeedingClientConfig
	if err := e.db.WithContext(ctx).First(&cfg, id).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (e *Engine) CreateConfig(ctx context.Context, cfg *model.SeedingClientConfig) error {
	return e.db.WithContext(ctx).Create(cfg).Error
}

func (e *Engine) UpdateConfig(ctx context.Context, cfg *model.SeedingClientConfig) error {
	return e.db.WithContext(ctx).Save(cfg).Error
}

func (e *Engine) DeleteConfig(ctx context.Context, id uint) error {
	return e.db.WithContext(ctx).Delete(&model.SeedingClientConfig{}, id).Error
}

// Deprecated: Add is legacy code. Seeding Engine now receives events via EventBus
// (OnPushed → pendingEvents → consumeLoop → Pusher.Push → createRecordFromPush).
func (e *Engine) Add(ctx context.Context, clientUID uint, event *model.TorrentEvent) error {
	var cfg model.SeedingClientConfig
	if err := e.db.WithContext(ctx).Where("client_uid = ?", clientUID).First(&cfg).Error; err == nil {
		if !IsWithinActiveWindow(cfg.ActiveTimeWindows) {
			e.logger.Debug("outside active time windows, skipping add",
				zap.Uint("client_uid", clientUID),
				zap.String("windows", cfg.ActiveTimeWindows))
			return nil
		}
	}

	key := recordKey(clientUID, event.InfoHash)

	e.mu.RLock()
	_, exists := e.recordMap[key]
	e.mu.RUnlock()
	if exists {
		return nil
	}

	record := &model.SeedingTorrentRecord{
		ClientUID:      clientUID,
		SiteName:       event.SiteName,
		TorrentID:      event.TorrentID,
		InfoHash:       event.InfoHash,
		Discount:       event.Discount,
		HasHR:          event.HasHR,
		HRSeedTimeH:    event.HRSeedTimeH,
		Source:         "rss",
		Status:         model.SeedingStatusPending,
		SubscriptionID: event.SourceID,
		IsFree:         event.Discount == model.DiscountFree || event.Discount == model.Discount2xFree,
		FreeEndAt:      event.FreeEndAt,
		TorrentSize:    event.Size,
	}

	if !record.IsFree && record.Discount != model.DiscountAssumeFree && e.siteProvider != nil && record.SiteName != "" {
		if siteInfo, infoErr := e.siteProvider.GetSiteInfo(ctx, record.SiteName); infoErr == nil && siteInfo != nil && siteInfo.AssumeFree {
			record.IsFree = true
			record.Discount = model.DiscountAssumeFree
		}
	}

	result := e.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "client_uid"}, {Name: "info_hash"}},
		DoNothing: true,
	}).Create(record)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		var loaded model.SeedingTorrentRecord
		if err := e.db.WithContext(ctx).
			Where("client_uid = ? AND info_hash = ?", clientUID, event.InfoHash).
			First(&loaded).Error; err != nil {
			return err
		}
		if loaded.Status == model.SeedingStatusDeleted {
			e.logger.Info("Add: restoring deleted seeding record",
				zap.Uint("client_uid", clientUID),
				zap.String("info_hash", event.InfoHash),
				zap.String("site_name", event.SiteName),
				zap.String("torrent_id", event.TorrentID),
				zap.String("prev_last_action_by", loaded.LastActionBy))
			now := time.Now()
			if err := e.db.WithContext(ctx).Model(&loaded).Updates(map[string]interface{}{
				"status":          model.SeedingStatusPending,
				"site_name":       event.SiteName,
				"torrent_id":      event.TorrentID,
				"discount":        event.Discount,
				"has_hr":          event.HasHR,
				"hr_seed_time_h":  event.HRSeedTimeH,
				"source":          "rss",
				"subscription_id": event.SourceID,
				"is_free":         record.IsFree,
				"free_end_at":     event.FreeEndAt,
				"torrent_size":    event.Size,
				"flushed_at":      nil,
				"last_action_by":  "",
				"updated_at":      now,
			}).Error; err != nil {
				return fmt.Errorf("restore deleted record: %w", err)
			}
			loaded.Status = model.SeedingStatusPending
			loaded.FlushedAt = nil
			loaded.LastActionBy = ""
			loaded.SiteName = event.SiteName
			loaded.TorrentID = event.TorrentID
			loaded.Discount = event.Discount
			loaded.HasHR = event.HasHR
			loaded.Source = "rss"
			loaded.SubscriptionID = event.SourceID
			loaded.IsFree = record.IsFree
			loaded.FreeEndAt = event.FreeEndAt
			loaded.TorrentSize = event.Size
			record = &loaded
		} else {
			e.logger.Debug("Add: record already exists, skipping",
				zap.Uint("client_uid", clientUID),
				zap.String("info_hash", event.InfoHash),
				zap.String("status", string(loaded.Status)))
			record = &loaded
		}
	} else {
		e.logger.Debug("Add: created new seeding record",
			zap.Uint("client_uid", clientUID),
			zap.String("info_hash", event.InfoHash),
			zap.String("site_name", event.SiteName),
			zap.String("torrent_id", event.TorrentID))
	}

	e.mu.Lock()
	e.recordMap[key] = record
	e.mu.Unlock()

	if e.freeEndMonitor != nil && record.FreeEndAt != nil {
		e.freeEndMonitor.Schedule(record)
	}
	return nil
}

func (e *Engine) Clear(ctx context.Context, clientUID uint) error {
	e.mu.Lock()
	for key, r := range e.recordMap {
		if r.ClientUID == clientUID && r.Source == "rss" {
			delete(e.recordMap, key)
		}
	}
	e.mu.Unlock()

	return e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("client_uid = ? AND source = ? AND status != ?", clientUID, "rss", model.SeedingStatusDeleted).
		Updates(map[string]interface{}{
			"status":         model.SeedingStatusDeleted,
			"last_action_by": "clear",
			"updated_at":     time.Now(),
		}).Error
}

func (e *Engine) CollectTrafficStats(ctx context.Context) error {
	if e.getClientProvider() == nil {
		return nil
	}

	clients := e.getClientProvider().ListClients()
	now := time.Now()

	for _, clientUID := range clients {
		var md *model.Maindata

		if cached := e.getCachedMaindata(clientUID); cached != nil {
			md = cached.Maindata
		}

		if md == nil {
			dlClient, err := e.getClientProvider().Get(clientUID)
			if err != nil {
				e.logger.Debug("failed to get downloader, skipping", zap.Uint("client_uid", clientUID), zap.Error(err))
				continue
			}

			var fetchErr error
			md, fetchErr = dlClient.GetMainData(ctx)
			if fetchErr != nil || md == nil {
				e.logger.Debug("failed to get downloader data, skipping", zap.Uint("client_uid", clientUID), zap.Error(fetchErr))
				continue
			}
		}

		var totalUpload, totalDownload int64
		for _, t := range md.Torrents {
			totalUpload += t.UploadSpeed
			totalDownload += t.DownloadSpeed
		}

		snapshot := &model.DownloaderSpeedSnapshot{
			ClientUID:      clientUID,
			UploadSpeed:    totalUpload,
			DownloadSpeed:  totalDownload,
			FreeSpaceBytes: md.FreeSpace,
			ActiveTorrents: len(md.Torrents),
			RecordedAt:     now,
		}
		if err := e.db.WithContext(ctx).Create(snapshot).Error; err != nil {
			e.logger.Warn("failed to write downloader speed snapshot", zap.Uint("client_uid", clientUID), zap.Error(err))
		}

		e.collectSiteTrafficDaily(ctx, clientUID, md, now)

		e.collectTorrentTraffic(ctx, clientUID, md, now)
	}

	return nil
}

func (e *Engine) collectTorrentTraffic(ctx context.Context, clientUID uint, md *model.Maindata, now time.Time) {
	var records []model.SeedingTorrentRecord
	e.db.WithContext(ctx).
		Where("client_uid = ? AND status IN ?", clientUID,
			[]string{string(model.SeedingStatusPending), string(model.SeedingStatusSeeding), "paused_free_end", "paused_rule"}).
		Find(&records)

	if len(records) == 0 || md == nil {
		return
	}

	trafficBatch := make([]*model.TorrentTraffic, 0, len(records))
	for _, rec := range records {
		ti, ok := md.Torrents[rec.InfoHash]
		if !ok {
			continue
		}
		trafficBatch = append(trafficBatch, &model.TorrentTraffic{
			ClientUID:     clientUID,
			InfoHash:      rec.InfoHash,
			SiteName:      rec.SiteName,
			Uploaded:      ti.Uploaded,
			Downloaded:    ti.Downloaded,
			UploadSpeed:   ti.UploadSpeed,
			DownloadSpeed: ti.DownloadSpeed,
			Ratio:         ti.Ratio,
			RecordedAt:    now,
		})
	}

	if len(trafficBatch) > 0 {
		// §59.152 融合方案（Vertex 三层模式）：
		// ① seeding_torrent_records UPSERT 最新累计（current_uploaded 列——历史总量零成本）
		// ② site_traffic_flows 写时聚合（内存差值→站点增量，7 天保留——查询永不扫全量）
		// ③ torrent_traffic 明细照写（当日增量原料，保留期 30→1 天由配置层改）
		siteDeltas := map[string]*model.SiteTrafficFlow{}
		for _, t := range trafficBatch {
			sd, ok := siteDeltas[t.SiteName]
			if !ok {
				sd = &model.SiteTrafficFlow{SiteName: t.SiteName, RecordedAt: now}
				siteDeltas[t.SiteName] = sd
			}
			// 与上次快照的差值（collect 前从 record 缓存查——rec.Uploaded 是上轮值）
			for _, r := range records {
				if r.InfoHash == t.InfoHash {
					if t.Uploaded > r.CurrentUploaded {
						sd.UploadDelta += t.Uploaded - r.CurrentUploaded
					}
					if t.Downloaded > r.CurrentDownloaded {
						sd.DownloadDelta += t.Downloaded - r.CurrentDownloaded
					}
					break
				}
			}
			// ① 单行 UPSERT 最新累计到 record
			e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
				Where("info_hash = ?", t.InfoHash).
				Updates(map[string]interface{}{
					"current_uploaded":   t.Uploaded,
					"current_downloaded": t.Downloaded,
				})
		}
		// ② 站点增量落库
		for _, sd := range siteDeltas {
			if sd.UploadDelta > 0 || sd.DownloadDelta > 0 {
				e.db.WithContext(ctx).Create(sd)
			}
		}

		// §59.37: 分块写入——SQLite 单条 INSERT 变量上限 999（默认），2 万种子
		// 单条批量写必然 too many SQL variables（243 实证每轮 2 次失败）。
		// 每行 9 列 → chunk 100 行 = 900 变量，安全上限内。
		const chunk = 100
		written, failed := 0, 0
		for i := 0; i < len(trafficBatch); i += chunk {
			end := i + chunk
			if end > len(trafficBatch) {
				end = len(trafficBatch)
			}
			if err := e.db.WithContext(ctx).Create(trafficBatch[i:end]).Error; err != nil {
				failed++
				if failed == 1 { // 首次失败记录，后续不刷屏
					e.logger.Warn("batch write torrent_traffic chunk failed",
						zap.Uint("client_uid", clientUID),
						zap.Int("chunk_index", i/chunk),
						zap.Error(err))
				}
			} else {
				written += end - i
			}
		}
		if failed > 0 {
			e.logger.Warn("batch write torrent_traffic partial failure",
				zap.Uint("client_uid", clientUID),
				zap.Int("written", written),
				zap.Int("failed_chunks", failed))
		}
	}
}

// collectSiteTrafficDaily writes instantaneous cumulative upload SUM to site_traffic_daily.
// Note: upload_delta is NOT an incremental value — it's overwritten each cycle with the
// current SUM of ti.Uploaded for active torrents. The stats API no longer uses this field
// for "today's upload" display (uses torrent_traffic max-min instead).
// site_traffic_daily is retained for site-level trend charts only.
func (e *Engine) collectSiteTrafficDaily(ctx context.Context, clientUID uint, md *model.Maindata, now time.Time) {
	today := util.LocalDayStart(now) // 16 刀1（v0.0.1664）: 本地午夜口径（V-01——原 Truncate(24h) 窗口错开 8 小时）

	var records []model.SeedingTorrentRecord
	e.db.WithContext(ctx).
		Where("client_uid = ? AND status IN ?", clientUID,
			[]string{string(model.SeedingStatusPending), string(model.SeedingStatusSeeding), "paused_free_end", "paused_rule"}).
		Find(&records)

	siteCount := make(map[string]int)
	siteUpload := make(map[string]int64)
	for _, rec := range records {
		siteCount[rec.SiteName]++
		if md != nil {
			if ti, ok := md.Torrents[rec.InfoHash]; ok {
				siteUpload[rec.SiteName] += ti.Uploaded
			}
		}
	}

	if len(siteCount) > 0 {
		siteNames := make([]string, 0, len(siteCount))
		for name := range siteCount {
			siteNames = append(siteNames, name)
		}
		var existingList []model.SiteTrafficDaily
		e.db.WithContext(ctx).
			Where("site_name IN ? AND date = ?", siteNames, today).
			Find(&existingList)
		existingMap := make(map[string]*model.SiteTrafficDaily, len(existingList))
		for i := range existingList {
			existingMap[existingList[i].SiteName] = &existingList[i]
		}

		for siteName, count := range siteCount {
			uploaded := siteUpload[siteName]
			if existing, ok := existingMap[siteName]; ok {
				updates := map[string]interface{}{
					"seeding_count": count,
					"upload_delta":  uploaded,
				}
				if err := e.db.WithContext(ctx).Model(existing).Updates(updates).Error; err != nil {
					e.logger.Warn("update site traffic daily failed",
						zap.String("site", siteName),
						zap.Error(err))
				}
			} else {
				entry := &model.SiteTrafficDaily{
					SiteName:     siteName,
					Date:         today,
					SeedingCount: count,
					TorrentCount: count,
					UploadDelta:  uploaded,
				}
				if createErr := e.db.WithContext(ctx).Create(entry).Error; createErr != nil {
					e.logger.Warn("create site traffic daily failed",
						zap.String("site", siteName),
						zap.Error(createErr),
					)
				}
			}
		}
	}
}

type RealTorrentCounts struct {
	Seeding     int   `json:"seeding"`
	Downloading int   `json:"downloading"`
	Paused      int   `json:"paused"`
	Total       int   `json:"total"`
	TotalSize   int64 `json:"totalSize"`
}

func (e *Engine) GetRealTorrentCounts() map[uint]*RealTorrentCounts {
	e.maindataMu.RLock()
	defer e.maindataMu.RUnlock()

	result := make(map[uint]*RealTorrentCounts)
	for clientUID, entry := range e.maindataCache {
		counts := &RealTorrentCounts{}
		for _, ti := range entry.Maindata.Torrents {
			if ti.Removed {
				continue
			}
			counts.Total++
			counts.TotalSize += ti.TotalSize
			state := strings.ToLower(ti.State)
			switch {
			case state == "uploading" || state == "stalledup" || state == "forcedup" ||
				state == "queuedup" || state == "checkingup":
				counts.Seeding++
			case state == "downloading" || state == "stalleddl" || state == "forceddl" ||
				state == "metadl" || state == "forcedmetadl" || state == "checkingdl" || state == "queueddl":
				counts.Downloading++
			case ti.IsPaused || state == "pausedup" || state == "stoppedup" ||
				strings.HasSuffix(state, "dl"):
				counts.Paused++
			}
		}
		result[clientUID] = counts
	}
	return result
}

func splitRuleIDs(s string) []string {
	parts := strings.Split(s, ",")
	var ids []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if _, err := strconv.Atoi(p); err == nil {
			ids = append(ids, p)
		}
	}
	return ids
}

func (e *Engine) applyConfig(cfg *model.SeedingClientConfig, defaultScore float64, defaultAge float64, defaultWeights CleanupWeights) (float64, float64, CleanupWeights) {
	minScore := defaultScore
	minAgeHours := defaultAge
	weights := defaultWeights
	// §59.122: 阈值配置化——DB 列非零覆盖默认（零值=默认, 兼容存量）
	if cfg != nil {
		if cfg.CleanupMinScore > 0 {
			minScore = cfg.CleanupMinScore
		}
		if cfg.CleanupMinAgeHours > 0 {
			minAgeHours = cfg.CleanupMinAgeHours
		}
	}

	if cfg.CleanupScoreWeights != "" {
		var w model.CleanupScoreWeights
		if err := json.Unmarshal([]byte(cfg.CleanupScoreWeights), &w); err == nil {
			if w.SeedHours > 0 {
				weights.SeedHours = w.SeedHours
			}
			if w.UploadSpeed > 0 {
				weights.UploadSpeed = w.UploadSpeed
			}
			if w.Ratio > 0 {
				weights.Ratio = w.Ratio
			}
			if w.DiskUsage > 0 {
				weights.DiskUsage = w.DiskUsage
			}
		}
	}

	return minScore, minAgeHours, weights
}

// archiveLoop §14.16/S-P1-7: 每 6 小时归档已删除/暂停的旧记录。
func (e *Engine) archiveLoop(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.archiveOldRecords(ctx)
		}
	}
}

func (e *Engine) archiveOldRecords(ctx context.Context) {
	cutoff := time.Now().AddDate(0, 0, -14)
	result := e.db.WithContext(ctx).
		Model(&model.SeedingTorrentRecord{}).
		Where("status IN ? AND updated_at < ?",
			[]string{string(model.SeedingStatusDeleted), string(model.SeedingStatusPausedFreeEnd), string(model.SeedingStatusPausedRule)},
			cutoff).
		Update("status", string(model.SeedingStatusArchived))
	if result.RowsAffected > 0 {
		e.logger.Info("archive: records archived",
			zap.Int64("count", result.RowsAffected),
			zap.Time("cutoff", cutoff))
	}

	physicalCutoff := time.Now().AddDate(0, 0, -30)
	delResult := e.db.WithContext(ctx).
		Where("status = ? AND updated_at < ?", string(model.SeedingStatusArchived), physicalCutoff).
		Delete(&model.SeedingTorrentRecord{})
	if delResult.RowsAffected > 0 {
		e.logger.Info("archive: records physically deleted",
			zap.Int64("count", delResult.RowsAffected),
			zap.Time("cutoff", physicalCutoff))
	}

	// 清理过期的 discount 缓存（超过 30 分钟未访问）
	e.discountCacheMu.Lock()
	evicted := 0
	for k, v := range e.discountCache {
		if time.Since(v.CheckedAt) > 30*time.Minute {
			delete(e.discountCache, k)
			evicted++
		}
	}
	e.discountCacheMu.Unlock()
	if evicted > 0 {
		e.logger.Debug("archive: discount cache evicted", zap.Int("count", evicted))
	}
}
