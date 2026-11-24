package reseed

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	dbimpl "github.com/ranfish/pt-forward/internal/db"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm/clause"
)

// matchRecordRetention §59.568 刀3: 匹配记录保留期（一年·同语义族）
const matchRecordRetention = 365 * 24 * time.Hour

func (e *Engine) CreateTask(ctx context.Context, task *model.ReseedTask) error {
	task.Status = model.ReseedTaskIdle
	return dbimpl.ForceCreate(e.db.WithContext(ctx), task)
}

func (e *Engine) GetTask(ctx context.Context, id uint) (*model.ReseedTask, error) {
	var task model.ReseedTask
	err := e.db.WithContext(ctx).First(&task, id).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (e *Engine) ListTasks(ctx context.Context) ([]model.ReseedTask, error) {
	var tasks []model.ReseedTask
	err := e.db.WithContext(ctx).Order("name ASC").Find(&tasks).Error
	return tasks, err
}

func (e *Engine) UpdateTask(ctx context.Context, task *model.ReseedTask) error {
	return e.db.WithContext(ctx).Save(task).Error
}

func (e *Engine) DeleteTask(ctx context.Context, id uint) error {
	e.CancelTask(id)
	return e.db.WithContext(ctx).Delete(&model.ReseedTask{}, id).Error
}

func (e *Engine) ListByClientID(ctx context.Context, clientID string) ([]model.ReseedTask, error) {
	var tasks []model.ReseedTask
	err := e.db.WithContext(ctx).
		Where("client_ids = ? OR client_ids LIKE ? OR client_ids LIKE ? OR client_ids LIKE ?",
			clientID,
			clientID+",%",
			"%,"+clientID+",%",
			"%,"+clientID).
		Find(&tasks).Error
	return tasks, err
}

func (e *Engine) ListEnabled(ctx context.Context) ([]model.ReseedTask, error) {
	var tasks []model.ReseedTask
	err := e.db.WithContext(ctx).
		Where("enabled = ? AND status IN ?", true, []model.ReseedTaskStatus{model.ReseedTaskIdle, model.ReseedTaskRunning}).
		Find(&tasks).Error
	return tasks, err
}

func (e *Engine) BatchSaveMatches(ctx context.Context, matches []*model.ReseedMatch) error {
	return e.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "client_uid"},
			{Name: "source_site"},
			{Name: "source_torrent_id"},
			{Name: "target_site"},
			{Name: "target_torrent_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"updated_at", "target_info_hash", "match_method", "confidence",
			"decision_type", "status", "fail_reason",
		}),
	}).Create(matches).Error
}

func (e *Engine) retryFailedForTask(ctx context.Context, task *model.ReseedTask, clientUIDs []uint) {
	maxRetries := task.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	retryInterval := time.Duration(task.RetryIntervalH) * time.Hour
	if retryInterval <= 0 {
		retryInterval = 24 * time.Hour
	}

	var matches []model.ReseedMatch
	err := e.db.WithContext(ctx).
		Where("status = ? AND retry_count < ?", model.MatchStatusFailed, maxRetries).
		Where("client_uid IN ?", clientUIDs).
		Where("next_retry_at IS NULL OR next_retry_at <= ?", time.Now()).
		Order("next_retry_at ASC").
		Limit(50).
		Find(&matches).Error
	if err != nil || len(matches) == 0 {
		return
	}

	siteSet := make(map[string]bool)
	for _, m := range matches {
		siteSet[m.TargetSite] = true
	}
	var targetSites []string
	for s := range siteSet {
		targetSites = append(targetSites, s)
	}

	ps := e.preloadSites(ctx, targetSites, nil)
	if ps == nil {
		return
	}

	retried, succeeded := 0, 0
loop:
	for i := range matches {
		if ctx.Err() != nil {
			break
		}
		m := &matches[i]
		if err := e.injectMatch(ctx, m, task, ps); err != nil {
			nextRetry := time.Now().Add(retryInterval)
			e.db.WithContext(ctx).Model(m).Updates(map[string]interface{}{
				"next_retry_at": &nextRetry,
			})
			retried++
			continue
		}
		succeeded++
		retried++

		if i < len(matches)-1 {
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				break loop
			}
		}
	}

	if retried > 0 {
		e.logger.Info("reseed retry completed",
			zap.Uint("task_id", task.ID),
			zap.Int("retried", retried),
			zap.Int("succeeded", succeeded))
	}
}

func (e *Engine) UpdateMatchStatus(ctx context.Context, id uint, status string, failReason string) error {
	return e.db.WithContext(ctx).
		Model(&model.ReseedMatch{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      status,
			"fail_reason": failReason,
			"updated_at":  time.Now(),
		}).Error
}

func (e *Engine) SaveMatch(ctx context.Context, match *model.ReseedMatch) error {
	err := e.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "client_uid"},
			{Name: "source_site"},
			{Name: "source_torrent_id"},
			{Name: "target_site"},
			{Name: "target_torrent_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"updated_at", "target_info_hash", "match_method", "confidence",
			"decision_type", "status", "fail_reason",
		}),
	}).Create(match).Error
	if err != nil {
		return err
	}
	// §59.535 刀2: 辅种匹配落库即同步覆盖表——「知道即落库」，消灭 matches
	// 表与覆盖表两张皮（任务天天跑覆盖表不涨的断层）。仅 matched/injected
	// （failed=匹配失败≠站上有）。
	e.syncMatchCoverage(ctx, match)
	return nil
}

// syncMatchCoverage §59.535 刀2: 单条 match→覆盖表同步。先到优先（对齐
// syncIYUUCoverage 先例与 QuerySharedCoverage.upsertCoverageIYUU 语义）：
// INSERT OR IGNORE 不覆盖已有行（tracker/pieces_hash 等先到高置信源保留）；
// 已有行仅补注入明细（torrent_id/target_info_hash 空缺时）+刷新 queried_at。
// expires_at 远期（§59.535 废弃——站上有=事实不因时间失效）。
func (e *Engine) syncMatchCoverage(ctx context.Context, m *model.ReseedMatch) {
	if m == nil || (m.Status != model.MatchStatusMatched && m.Status != model.MatchStatusInjected) {
		return
	}
	now := time.Now()
	res := e.db.WithContext(ctx).Exec(`INSERT OR IGNORE INTO site_coverage_cache
		(info_hash, site_name, status, source, confidence, torrent_id, target_info_hash, queried_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.SourceInfoHash, m.TargetSite, model.CoverageConfirmedHas, m.MatchMethod,
		m.Confidence, m.TargetTorrentID, m.TargetInfoHash, now, now.Add(matchRecordRetention))
	if res.Error != nil {
		e.logger.Debug("sync match coverage insert failed",
			zap.String("hash", m.SourceInfoHash), zap.String("site", m.TargetSite), zap.Error(res.Error))
		return
	}
	if res.RowsAffected > 0 {
		return
	}
	// 已有行：仅补空缺明细（不覆盖 status/source/confidence——先到优先）
	upd := e.db.WithContext(ctx).Exec(`UPDATE site_coverage_cache SET
		torrent_id = CASE WHEN (torrent_id = '' OR torrent_id IS NULL) AND ? != '' THEN ? ELSE torrent_id END,
		target_info_hash = CASE WHEN (target_info_hash = '' OR target_info_hash IS NULL) AND ? != '' THEN ? ELSE target_info_hash END,
		queried_at = ?
		WHERE info_hash = ? AND site_name = ?`,
		m.TargetTorrentID, m.TargetTorrentID, m.TargetInfoHash, m.TargetInfoHash,
		now, m.SourceInfoHash, m.TargetSite)
	if upd.Error != nil {
		e.logger.Debug("sync match coverage update failed",
			zap.String("hash", m.SourceInfoHash), zap.String("site", m.TargetSite), zap.Error(upd.Error))
	}
}

func (e *Engine) FindMatchesByInfoHash(ctx context.Context, infoHash string) ([]model.ReseedMatch, error) {
	var matches []model.ReseedMatch
	err := e.db.WithContext(ctx).
		Where("source_info_hash = ?", infoHash).
		Find(&matches).Error
	return matches, err
}

func (e *Engine) FindMatchByID(ctx context.Context, id uint) (*model.ReseedMatch, error) {
	var m model.ReseedMatch
	err := e.db.WithContext(ctx).First(&m, id).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (e *Engine) RetryMatch(ctx context.Context, id uint) (*model.ReseedMatch, error) {
	var m model.ReseedMatch
	if err := e.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}

	if m.Status != model.MatchStatusFailed {
		return nil, reseedError(ErrReseedGeneric, fmt.Sprintf("只能重试失败的匹配记录，当前状态: %s", m.Status), nil)
	}

	now := time.Now()
	newRetry := m.RetryCount + 1

	if err := e.db.WithContext(ctx).Model(&m).Updates(map[string]interface{}{
		"status":        model.MatchStatusMatched,
		"retry_count":   newRetry,
		"fail_reason":   "",
		"next_retry_at": &now,
	}).Error; err != nil {
		return nil, err
	}
	m.Status = model.MatchStatusMatched
	m.RetryCount = newRetry
	m.FailReason = ""
	m.NextRetryAt = &now
	return &m, nil
}

func (e *Engine) DeleteNegativeCache(ctx context.Context, infoHash, site string) (int64, error) {
	q := e.db.WithContext(ctx).Where("source_info_hash = ?", infoHash)
	if site != "" {
		q = q.Where("source_site = ?", site)
	}
	result := q.Delete(&model.ReseedNegativeCache{})
	return result.RowsAffected, result.Error
}

func (e *Engine) SetNegativeCache(ctx context.Context, sourceSite, sourceInfoHash, targetSite, method string, layerDepth int, ttl time.Duration) error {
	now := time.Now()
	expiresAt := now.Add(ttl)

	var existing model.ReseedNegativeCache
	err := e.db.WithContext(ctx).
		Where("source_info_hash = ? AND expires_at > ?", sourceInfoHash, now).
		First(&existing).Error
	if err == nil {
		for _, t := range strings.Split(existing.ExcludedTargets, ",") {
			if strings.TrimSpace(t) == targetSite {
				return nil
			}
		}
		newList := existing.ExcludedTargets
		if newList != "" {
			newList += ","
		}
		newList += targetSite
		return e.db.WithContext(ctx).Model(&existing).Updates(map[string]interface{}{
			"excluded_targets": newList,
			"last_method":      method,
			"layer_depth":      layerDepth,
			"expires_at":       expiresAt,
		}).Error
	}

	entry := &model.ReseedNegativeCache{
		SourceSite:      sourceSite,
		SourceInfoHash:  sourceInfoHash,
		ExcludedTargets: targetSite,
		LastMethod:      method,
		LayerDepth:      layerDepth,
		ExpiresAt:       expiresAt,
	}
	return e.db.WithContext(ctx).Create(entry).Error
}

func (e *Engine) GetNegativeCacheByHashes(ctx context.Context, hashes []string) ([]model.ReseedNegativeCache, error) {
	if len(hashes) == 0 {
		return nil, nil
	}
	var entries []model.ReseedNegativeCache
	for _, chunk := range chunkStrings(hashes, preloadBatchSize) {
		var partial []model.ReseedNegativeCache
		if err := e.db.WithContext(ctx).
			Where("source_info_hash IN ? AND expires_at > ?", chunk, time.Now()).
			Find(&partial).Error; err != nil {
			return entries, err
		}
		entries = append(entries, partial...)
	}
	return entries, nil
}

func (e *Engine) FlushNegativeCache(ctx context.Context) (int64, error) {
	result := e.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&model.ReseedNegativeCache{})
	return result.RowsAffected, result.Error
}

func (e *Engine) OnTorrentSeeding(parentCtx context.Context, record model.SeedingTorrentRecord, reseedClientIDs []uint) {
	ctx, cancel := context.WithTimeout(parentCtx, 5*time.Minute)
	defer cancel()

	e.logger.Info("auto reseed triggered",
		zap.String("site", record.SiteName),
		zap.String("info_hash", record.InfoHash),
		zap.Uints("reseed_client_uids", reseedClientIDs))

	if e.siteProvider == nil {
		e.logger.Warn("auto reseed: siteProvider not available")
		return
	}

	ps := e.preloadSites(ctx, nil, []string{record.SiteName})
	if ps == nil || len(ps.infos) == 0 {
		e.logger.Debug("auto reseed: no target sites available")
		return
	}

	infoHashes := []string{record.InfoHash}
	fpc := e.preloadFingerprints(ctx, infoHashes)
	negCache := e.preloadNegativeCache(ctx, infoHashes)
	existingMatchesMap, _ := e.preloadExistingMatches(ctx, infoHashes, nil)

	confirmedTargets := make(map[string]bool)
	for _, matches := range existingMatchesMap {
		for _, m := range matches {
			key := m.TargetSite + ":" + m.TargetTorrentID
			confirmedTargets[key] = true
		}
	}

	var recTitle string
	if fp := fpc.get(record.InfoHash, record.SiteName); fp != nil {
		recTitle = fp.Title
	}
	if !e.checkEligibility(ctx, recTitle, nil) {
		e.logger.Info("auto reseed: blocked by publish eligibility", zap.String("title", recTitle))
		return
	}

	task := &model.ReseedTask{
		SizeTolerancePercent: 1.0,
		MaxInjectionsPerRun:  10,
		ReseedCategory:       "cross-seed",
	}

	src := sourceTorrent{
		InfoHash:  record.InfoHash,
		SiteName:  record.SiteName,
		ClientUID: record.ClientUID,
	}
	candidates := e.findCandidates(ctx, src, ps, fpc, task.SizeTolerancePercent, task, negCache, nil, nil, nil, nil, nil)
	if len(candidates) == 0 {
		e.logger.Debug("auto reseed: no candidates found", zap.String("info_hash", record.InfoHash))
		return
	}

	e.logger.Info("auto reseed: candidates found",
		zap.String("info_hash", record.InfoHash),
		zap.Int("count", len(candidates)))

	for _, c := range candidates {
		targetKey := c.TargetSite + ":" + c.TargetTorrentID
		if confirmedTargets[targetKey] {
			continue
		}
		for _, clientID := range reseedClientIDs {
			match := &model.ReseedMatch{
				ClientUID:       clientID,
				SourceSite:      record.SiteName,
				SourceTorrentID: record.TorrentID,
				SourceInfoHash:  record.InfoHash,
				TargetSite:      c.TargetSite,
				TargetTorrentID: c.TargetTorrentID,
				TargetInfoHash:  c.TargetInfoHash,
				MatchMethod:     c.MatchMethod,
				Confidence:      c.Confidence,
				DecisionType:    string(model.DecisionMatch),
				Status:          model.MatchStatusMatched,
			}

			if err := e.SaveMatch(ctx, match); err != nil {
				e.logger.Warn("auto reseed: save match failed",
					zap.String("target_site", c.TargetSite),
					zap.Error(err))
				continue
			}

			if e.clientProvider == nil {
				continue
			}

			if err := e.injectMatch(ctx, match, task, ps); err != nil {
				e.logger.Warn("auto reseed: inject failed",
					zap.Uint("match_id", match.ID),
					zap.String("target_site", c.TargetSite),
					zap.Error(err))
				continue
			}
			e.logger.Info("auto reseed: injected",
				zap.String("source_hash", record.InfoHash),
				zap.String("target_site", c.TargetSite),
				zap.Uint("client_uid", clientID))
		}
	}
}

func allowedReseedRole(role string) bool {
	return role == "download" || role == "master_reseed" || role == "reseed"
}

func (e *Engine) ValidateClientRoles(ctx context.Context, clientIDs string) error {
	parts := ParseClientIDs(clientIDs)
	if len(parts) == 0 {
		return fmt.Errorf("client_ids 为空")
	}
	uintIDs := partsToUint(parts)
	if len(uintIDs) == 0 {
		return fmt.Errorf("client_ids 格式无效")
	}
	var clients []model.ClientConfig
	if err := e.db.WithContext(ctx).Select("id, name, role").Where("id IN ?", uintIDs).Find(&clients).Error; err != nil {
		return fmt.Errorf("查询下载器失败: %w", err)
	}
	for _, c := range clients {
		if !allowedReseedRole(c.Role) {
			return fmt.Errorf("下载器 %s（ID=%d）角色为 %s，辅种任务仅允许 download 或 master_reseed 角色的下载器", c.Name, c.ID, c.Role)
		}
	}
	return nil
}

// resolveClientIDsToUIDs §59.251: 直接返回 UID（名字解析层废除——ID 即标识）
func (e *Engine) resolveClientIDsToUIDs(ctx context.Context, ids string) []uint {
	parts := ParseClientIDs(ids)
	if len(parts) == 0 {
		return nil
	}
	return partsToUint(parts)
}

func (e *Engine) resolveSiteIDsToNames(ctx context.Context, ids string) []string {
	parts := ParseClientIDs(ids)
	if len(parts) == 0 {
		return nil
	}
	uintIDs := partsToUint(parts)
	if len(uintIDs) == 0 {
		return parts
	}
	var sites []model.Site
	if err := e.db.WithContext(ctx).Select("id, name").Where("id IN ?", uintIDs).Find(&sites).Error; err != nil {
		e.logger.Warn("resolve site IDs to names failed", zap.Error(err))
		return parts
	}
	if len(sites) == 0 {
		return parts
	}
	names := make([]string, 0, len(sites))
	for _, s := range sites {
		names = append(names, s.Name)
	}
	return names
}

func partsToUint(parts []string) []uint {
	result := make([]uint, 0, len(parts))
	for _, p := range parts {
		if v, err := strconv.ParseUint(p, 10, 32); err == nil {
			result = append(result, uint(v))
		}
	}
	return result
}

func ParseClientIDs(ids string) []string {
	if ids == "" {
		return nil
	}
	parts := strings.Split(ids, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func parseTags(tagsStr string, defaults ...string) []string {
	tags := ParseClientIDs(tagsStr)
	if len(tags) == 0 {
		return defaults
	}
	return tags
}
