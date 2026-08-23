package download

import (
	"path/filepath"
	"context"
	"fmt"
	"time"

	"github.com/ranfish/pt-forward/internal/client"
	"github.com/ranfish/pt-forward/internal/companion"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/rule"
	"github.com/ranfish/pt-forward/internal/setting"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Syncer struct {
	db          *gorm.DB
	repo        *Repository
	clientMgr   *client.Manager
	ruleModule  *rule.Module
	fitTimer    *rule.FitTimer
	runtimeCfg  *setting.RuntimeConfig
	logger      *zap.Logger
	schedules   map[string]*clientSchedule
}

type clientSchedule struct {
	lastSync     time.Time
	lastEval     time.Time
	lastTransfer time.Time
}

func NewSyncer(db *gorm.DB, clientMgr *client.Manager, runtimeCfg *setting.RuntimeConfig, logger *zap.Logger) *Syncer {
	return &Syncer{
		db:         db,
		repo:       NewRepository(db),
		clientMgr:  clientMgr,
		ruleModule: rule.NewModule(db),
		fitTimer:   rule.NewFitTimer(),
		runtimeCfg: runtimeCfg,
		logger:     logger,
		schedules:  make(map[string]*clientSchedule),
	}
}

func (s *Syncer) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	s.restoreFitTimer(ctx)
	s.logger.Info("download syncer started")
	s.runOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("download syncer stopped")
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *Syncer) runOnce(ctx context.Context) {
	now := time.Now()
	clientNames := s.clientMgr.ListClients()

	for _, name := range clientNames {
		c, err := s.clientMgr.Get(name)
		if err != nil {
			continue
		}
		if c.GetRole() == "seeding" {
			continue
		}

		sched := s.schedules[name]
		if sched == nil {
			sched = &clientSchedule{}
			s.schedules[name] = sched
		}

		var cfg model.SeedingClientConfig
		hasCfg := s.db.WithContext(ctx).Where("client_id = ? AND role = ?", name, "download").First(&cfg).Error == nil

		syncInterval := 20 * time.Second
		evalInterval := 30 * time.Second
		transferInterval := 20 * time.Second

		if hasCfg {
			if d := cronToInterval(cfg.MainDataCron); d > 0 {
				syncInterval = d
			}
			if d := cronToInterval(cfg.AutoDeleteCron); d > 0 {
				evalInterval = d
			}
		}

		if sched.lastSync.IsZero() || now.Sub(sched.lastSync) >= syncInterval {
			s.syncClient(ctx, c)
			sched.lastSync = now
		}

		if hasCfg && cfg.Enabled && (sched.lastEval.IsZero() || now.Sub(sched.lastEval) >= evalInterval) {
			s.evaluateClientRules(ctx, c, &cfg)
			sched.lastEval = now
		}

		if sched.lastTransfer.IsZero() || now.Sub(sched.lastTransfer) >= transferInterval {
			s.processClientTransfers(ctx, name)
			sched.lastTransfer = now
		}
	}
}

func cronToInterval(cron string) time.Duration {
	parts := splitCSVSpace(cron)
	if len(parts) < 5 {
		return 0
	}

	// "*/N * * * *" → every N seconds
	if len(parts[0]) > 2 && parts[0][:2] == "*/" {
		if n, err := parseSimpleInt(parts[0][2:]); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}

	// "0 */N * * * *" → every N minutes
	if parts[0] == "0" && len(parts[1]) > 2 && parts[1][:2] == "*/" {
		if n, err := parseSimpleInt(parts[1][2:]); err == nil && n > 0 {
			return time.Duration(n) * time.Minute
		}
	}

	// "0 0 */N * * *" → every N hours
	if parts[0] == "0" && parts[1] == "0" && len(parts[2]) > 2 && parts[2][:2] == "*/" {
		if n, err := parseSimpleInt(parts[2][2:]); err == nil && n > 0 {
			return time.Duration(n) * time.Hour
		}
	}

	return 0
}

func parseSimpleInt(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func splitCSVSpace(s string) []string {
	var result []string
	current := ""
	for _, c := range s {
		if c == ' ' || c == '\t' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func (s *Syncer) restoreFitTimer(ctx context.Context) {
	var tasks []model.DownloadTask
	s.db.WithContext(ctx).
		Where("first_matched_at IS NOT NULL AND status != ?", model.DownloadStatusDeleted).
		Find(&tasks)
	for _, t := range tasks {
		s.fitTimer.MarkMatched(0, t.InfoHash, *t.FirstMatchedAt)
	}
	if len(tasks) > 0 {
		s.logger.Info("fit timer restored", zap.Int("entries", len(tasks)))
	}
}

func (s *Syncer) sync(ctx context.Context) {
	clientNames := s.clientMgr.ListClients()
	for _, name := range clientNames {
		c, err := s.clientMgr.Get(name)
		if err != nil {
			continue
		}
		role := c.GetRole()
		if role == "seeding" {
			continue
		}
		s.syncClient(ctx, c)
	}
}

func (s *Syncer) syncClient(ctx context.Context, c model.DownloaderClient) {
	clientID := c.GetName()

	torrents, err := c.GetAllTorrents(ctx)
	if err != nil {
		s.logger.Debug("failed to get torrents from client",
			zap.String("client", clientID),
			zap.Error(err))
		return
	}

	// 同步种子快照（含 save_path，用于种子配置页路径选择）
	s.syncSnapshots(ctx, clientID, torrents)

	torrentMap := make(map[string]*model.TorrentInfo, len(torrents))
	for _, t := range torrents {
		torrentMap[t.Hash] = t
	}

	existingHashes, err := s.repo.FindExistingHashes(ctx, clientID)
	if err != nil {
		s.logger.Debug("failed to get existing hashes",
			zap.String("client", clientID),
			zap.Error(err))
		return
	}

	for hash, ti := range torrentMap {
		if existingHashes[hash] {
			s.updateTaskProgress(ctx, clientID, ti)
		} else {
			s.importTask(ctx, clientID, ti)
		}
	}

	for hash := range existingHashes {
		if _, ok := torrentMap[hash]; !ok {
			task, err := s.repo.FindByClientAndHash(ctx, clientID, hash)
			if err == nil && task != nil && task.Status != model.DownloadStatusDeleted {
				s.repo.MarkDeleted(ctx, task.ID, "external")
				s.logger.Info("task auto-deleted (torrent removed externally)",
					zap.Uint("id", task.ID),
					zap.String("client", clientID),
					zap.String("hash", hash))
			}
		}
	}
}

// syncSnapshots 将下载器全量种子同步到 torrent_snapshots 表（含 save_path）。
// UPSERT by (hash, client_id)，消失的种子标记 is_hidden=true。
func (s *Syncer) syncSnapshots(ctx context.Context, clientID string, torrents []*model.TorrentInfo) {
	now := time.Now()
	seenHashes := make(map[string]bool, len(torrents))

	if len(torrents) == 0 {
		return
	}

	records := make([]model.TorrentSnapshot, 0, len(torrents))
	for _, t := range torrents {
		seenHashes[t.Hash] = true
		// §59.44: 路径规范化——TR 上报可能带尾斜杠（"PT6/SSD/" vs "PT6/SSD"），
		// 字符串差异会劈裂三元组资源键，Clean 统一形态
		records = append(records, model.TorrentSnapshot{
			Hash:     t.Hash,
			ClientID: clientID,
			Name:     t.Name,
			SavePath: filepath.Clean(t.SavePath),
			Size:     t.TotalSize,
			State:    t.State,
			Progress: t.Progress,
			Uploaded: t.Uploaded,
			IsHidden: false,
			LastSeen: now,
			Comment:  t.Comment, // §59.61: 簇直达判据凭证
		})
	}

	for i := range records {
		s.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "hash"}, {Name: "client_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"name", "save_path", "size", "state", "progress", "uploaded",
				"is_hidden", "last_seen", "updated_at", "comment", // §59.61
			}),
		}).Create(&records[i])
	}

	s.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
		Where("client_id = ? AND last_seen < ? AND is_hidden = ?", clientID, now, false).
		Update("is_hidden", true)
}

func (s *Syncer) updateTaskProgress(ctx context.Context, clientID string, ti *model.TorrentInfo) {
	task, err := s.repo.FindByClientAndHash(ctx, clientID, ti.Hash)
	if err != nil || task == nil {
		return
	}

	var status string
	progress := ti.Progress * 100

	switch {
	case ti.IsPaused:
		status = model.DownloadStatusPaused
	case ti.IsFinished:
		status = model.DownloadStatusCompleted
		progress = 100
	case ti.Error != "":
		status = model.DownloadStatusError
	default:
		status = model.DownloadStatusDownloading
	}

	if task.Status == model.DownloadStatusDeleted {
		return
	}

	if task.Status == status && task.Progress == progress && task.UploadSpeed == ti.UploadSpeed {
		return
	}

	updates := map[string]interface{}{
		"status":         status,
		"progress":       progress,
		"error_message":  ti.Error,
		"upload_speed":   ti.UploadSpeed,
		"download_speed": ti.DownloadSpeed,
		"ratio":          ti.Ratio,
		"uploaded":       ti.Uploaded,
		"num_seeds":      ti.NumComplete,
		"num_peers":      ti.NumIncomplete,
	}

	if status == model.DownloadStatusCompleted && task.Status != model.DownloadStatusCompleted {
		updates["completed_at"] = time.Now()
		s.logger.Info("download completed",
			zap.Uint("id", task.ID),
			zap.String("client", clientID),
			zap.String("name", ti.Name))
	}

	s.repo.UpdateProgress(ctx, task.ID, updates)
}

func (s *Syncer) importTask(ctx context.Context, clientID string, ti *model.TorrentInfo) {
	// §55.14 阶段4：RSS 推送的种子已由 seedingEngine 管（seeding_torrent_records），
	// 跳过导入避免双轨（record + download_task 重叠）
	var recordCount int64
	s.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("client_id = ? AND info_hash = ? AND status NOT IN ?", clientID, ti.Hash,
			[]string{string(model.SeedingStatusDeleted), string(model.SeedingStatusArchived)}).
		Count(&recordCount)
	if recordCount > 0 {
		return
	}

	task := &model.DownloadTask{
		Source:       "import",
		ClientID:     clientID,
		InfoHash:     ti.Hash,
		TorrentName:  ti.Name,
		SavePath:     ti.SavePath,
		TotalSize:    ti.TotalSize,
		Category:     ti.Category,
		Status:       model.DownloadStatusDownloading,
		Progress:     ti.Progress * 100,
	}

	if ti.IsFinished {
		task.Status = model.DownloadStatusCompleted
		task.Progress = 100
	}
	if ti.IsPaused {
		task.Status = model.DownloadStatusPaused
	}
	if ti.Error != "" {
		task.Status = model.DownloadStatusError
		task.ErrorMessage = ti.Error
	}

	if err := s.repo.Create(ctx, task); err != nil {
		s.logger.Debug("failed to import task",
			zap.String("client", clientID),
			zap.String("hash", ti.Hash),
			zap.Error(err))
		return
	}

	s.logger.Info("task auto-imported",
		zap.Uint("id", task.ID),
		zap.String("client", clientID),
		zap.String("name", ti.Name))
}

func (s *Syncer) processClientTransfers(ctx context.Context, clientName string) {
	var tasks []model.DownloadTask
	query := s.db.WithContext(ctx).
		Where("status = ? AND transfer_status != ? AND client_id = ?",
			model.DownloadStatusCompleted,
			model.TransferStatusTransferred,
			clientName)

	if cooldown := s.runtimeCfg.GetInt(ctx, setting.KeyTransferCooldownSeconds); cooldown > 0 {
		cutoff := time.Now().Add(-time.Duration(cooldown) * time.Second).UTC().Format("2006-01-02 15:04:05")
		query = query.Where("(completed_at IS NULL OR datetime(completed_at) < datetime(?))", cutoff)
	}

	query.Find(&tasks)

	for _, task := range tasks {
		s.processTransfer(ctx, &task)
	}

	// §55.16 修复 §55.14 副作用：role≠seeding 的 RSS 种子走 seeding record（不进 download_task），
	// 补一条 record 转移源，让下载器级 transferTargetId 对这类种子也生效
	s.processRecordTransfers(ctx, clientName)
}

// processRecordTransfers 处理 role≠seeding 的 seeding record 的下载器级转移（§55.16 修复 §55.14 副作用）。
// §55.14 统一引擎让 role≠seeding 种子走 seeding_torrent_records（不建 download_task），但
// processClientTransfers 原本只查 download_task → 下载器级 transferTargetId 永不触发。
// 此函数补这条链路：对 role≠seeding AND auto_transfer=false（订阅没配转移，由下载器级兜底）
// AND 下载器配了 transferTargetId 的 record，完成后转移到 transferTargetId。
// auto_transfer=true 的由订阅级 transferRecord 处理（§55.11 协调，互斥不重复）。
func (s *Syncer) processRecordTransfers(ctx context.Context, clientName string) {
	sourceClient, err := s.clientMgr.Get(clientName)
	if err != nil || sourceClient == nil {
		return
	}
	if sourceClient.GetRole() == "seeding" {
		return // 下载器级转移是 /downloads 体系职责，不碰刷流
	}
	targetID := sourceClient.GetTransferTargetID()
	if targetID == "" {
		return // 下载器没配转移目标
	}
	targetClient, err := s.clientMgr.Get(targetID)
	if err != nil || targetClient == nil {
		s.logger.Warn("record transfer: target client unavailable",
			zap.String("source", clientName), zap.String("target", targetID), zap.Error(err))
		return
	}

	var records []model.SeedingTorrentRecord
	s.db.WithContext(ctx).
		Where("client_id = ? AND role != ? AND status = ? AND auto_transfer = ?",
			clientName, "seeding", model.SeedingStatusSeeding, false).
		Find(&records)

	for i := range records {
		rec := &records[i]
		ti, err := sourceClient.GetTorrentByHash(ctx, rec.InfoHash)
		if err != nil || ti == nil {
			continue
		}
		if ti.Progress < 1.0 {
			continue // 未完成，跳过
		}

		// DB 层 CAS seeding→transferring（防 syncer 下轮与订阅级 transferRecord 并发）
		res := s.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
			Where("id = ? AND status = ?", rec.ID, model.SeedingStatusSeeding).
			Update("status", model.SeedingStatusTransferring)
		if res.Error != nil || res.RowsAffected == 0 {
			continue // 已被别处改状态，跳过
		}

		result, transferErr := client.TransferTorrent(ctx, sourceClient, targetClient, rec.InfoHash)
		if transferErr != nil {
			s.logger.Warn("record transfer: failed",
				zap.String("source", clientName), zap.String("target", targetID),
				zap.String("hash", rec.InfoHash), zap.Error(transferErr))
			s.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
				Where("id = ?", rec.ID).Update("status", model.SeedingStatusSeeding)
			continue
		}

		if err := sourceClient.DeleteTorrent(ctx, rec.InfoHash, false); err != nil {
			s.logger.Warn("record transfer: delete source failed (target already added)",
				zap.String("hash", rec.InfoHash), zap.Error(err))
		}

		s.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
			Where("id = ?", rec.ID).Update("status", model.SeedingStatusDeleted)

		s.logger.Info("record transfer: completed",
			zap.String("source", clientName), zap.String("target", targetID),
			zap.String("hash", rec.InfoHash),
			zap.Bool("duplicate", result.IsDuplicate))
	}
}

func (s *Syncer) processTransfer(ctx context.Context, task *model.DownloadTask) {
	// §55.11 协调：订阅级转移优先。若该种子被 seeding record 标记 AutoTransfer，由 transferRecord 处理，syncer 跳过。
	var seedingManaged int64
	s.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("client_id = ? AND info_hash = ? AND auto_transfer = ? AND status IN ?",
			task.ClientID, task.InfoHash, true,
			[]string{string(model.SeedingStatusSeeding), string(model.SeedingStatusTransferring)}).
		Count(&seedingManaged)
	if seedingManaged > 0 {
		s.logger.Debug("transfer: skipped (managed by subscription-level transfer)",
			zap.Uint("id", task.ID), zap.String("hash", task.InfoHash))
		return
	}

	sourceClient, err := s.clientMgr.Get(task.ClientID)
	if err != nil {
		s.logger.Warn("transfer: source client unavailable",
			zap.Uint("id", task.ID), zap.String("client", task.ClientID), zap.Error(err))
		return
	}

	targetID := sourceClient.GetTransferTargetID()
	if targetID == "" {
		s.repo.UpdateTransfer(ctx, task.ID, model.TransferStatusTransferred, "", "")
		return
	}

	targetClient, err := s.clientMgr.Get(targetID)
	if err != nil {
		s.logger.Warn("transfer: target client unavailable",
			zap.Uint("id", task.ID), zap.String("target", targetID), zap.Error(err))
		s.repo.UpdateTransfer(ctx, task.ID, model.TransferStatusFailed, targetID, "")
		return
	}

	if task.TransferStatus != model.TransferStatusPartial {
		s.repo.UpdateTransfer(ctx, task.ID, model.TransferStatusTransferring, targetID, "")

		result, err := client.TransferTorrent(ctx, sourceClient, targetClient, task.InfoHash)
		if err != nil {
			s.logger.Error("transfer: failed",
				zap.Uint("id", task.ID), zap.String("target", targetID), zap.Error(err))
			s.repo.UpdateTransfer(ctx, task.ID, model.TransferStatusFailed, targetID, "")
			return
		}

		if result.IsDuplicate {
			s.logger.Info("transfer: target already has torrent (skip add)",
				zap.Uint("id", task.ID),
				zap.String("target", targetID),
				zap.String("hash", result.InfoHash))
		} else {
			s.logger.Info("transfer: added to target",
				zap.Uint("id", task.ID),
				zap.String("target", targetID),
				zap.String("new_hash", result.InfoHash))
		}

		s.repo.UpdateTransfer(ctx, task.ID, model.TransferStatusTransferring, targetID, result.InfoHash)
		task.TransferHash = result.InfoHash
	}

	if err := sourceClient.DeleteTorrent(ctx, task.InfoHash, false); err != nil {
		s.logger.Warn("transfer: delete from source failed (partial)",
			zap.Uint("id", task.ID), zap.Error(err))
		s.repo.UpdateTransfer(ctx, task.ID, model.TransferStatusPartial, targetID, task.TransferHash)
		return
	}

	s.repo.UpdateTransfer(ctx, task.ID, model.TransferStatusTransferred, targetID, task.TransferHash)
	s.repo.UpdateClientAndHash(ctx, task.ID, targetID, task.TransferHash)

	s.logger.Info("transfer: completed",
		zap.Uint("id", task.ID),
		zap.String("source", task.ClientID),
		zap.String("target", targetID))
}

func (s *Syncer) evaluateClientRules(ctx context.Context, c model.DownloaderClient, cfg *model.SeedingClientConfig) {
	name := c.GetName()
	now := time.Now()

	ruleIDs := splitRuleIDs(cfg.DeleteRuleIDs)
	if len(ruleIDs) == 0 {
		return
	}

	rules, err := s.ruleModule.ListRulesByIDs(ctx, ruleIDs)
	if err != nil || len(rules) == 0 {
		return
	}

	torrents, err := c.GetAllTorrents(ctx)
	if err != nil {
		return
	}

	var rulePtrs []*model.DeleteRule
	for i := range rules {
		rulePtrs = append(rulePtrs, &rules[i])
	}

	var contexts []*rule.Context
	for _, ti := range torrents {
		contexts = append(contexts, rule.ContextFromTorrentInfo(ti, "", name, now))
	}

	activeHashes := make(map[string]bool)
	matchedSet := make(map[string]*model.DeleteRule)
	for _, rc := range contexts {
		for _, r := range rulePtrs {
			if rule.MatchRule(rc, r) {
				activeHashes[rc.InfoHash] = true
				matchedSet[rc.InfoHash] = r
				break
			}
		}
	}

	for _, r := range rulePtrs {
		s.fitTimer.ClearUnmatched(r.ID, activeHashes)
	}

	torrentMap := make(map[string]*model.TorrentInfo, len(torrents))
	for _, ti := range torrents {
		torrentMap[ti.Hash] = ti
	}

	for hash, r := range matchedSet {
		ti := torrentMap[hash]
		if ti == nil {
			continue
		}

		if r.FitTime > 0 {
			if !s.fitTimer.IsFit(r.ID, hash, r.FitTime, now) {
				s.fitTimer.MarkMatched(r.ID, hash, now)
				nowCopy := now
				s.db.WithContext(ctx).Model(&model.DownloadTask{}).
					Where("client_id = ? AND info_hash = ?", name, hash).
					Update("first_matched_at", &nowCopy)
				continue
			}
		}

		s.logger.Info("rule matched, deleting",
			zap.String("client", name),
			zap.String("hash", hash),
			zap.String("rule", r.Alias),
			zap.Uint("rule_id", r.ID))

		plan := companion.PlanDelete(ti, torrents, r.DeleteCompanions, true)

		for _, compHash := range plan.CompanionHashes {
			if err := c.DeleteTorrent(ctx, compHash, false); err != nil {
				s.logger.Warn("rule delete companion failed",
					zap.String("hash", compHash), zap.Error(err))
			}
		}

		if err := c.DeleteTorrent(ctx, plan.MainHash, plan.DeleteData); err != nil {
			s.logger.Warn("rule delete main failed",
				zap.String("hash", plan.MainHash), zap.Error(err))
		} else {
			action := model.DeleteActionWithCompanions
			if !r.DeleteCompanions {
				action = model.DeleteActionSiteOnly
			}
			s.db.WithContext(ctx).Model(&model.DownloadTask{}).
				Where("client_id = ? AND info_hash = ?", name, hash).
				Updates(map[string]interface{}{
					"status":           model.DownloadStatusDeleted,
					"deleted_at":       &now,
					"delete_action":    action,
					"first_matched_at": nil,
				})
			s.fitTimer.Remove(hash)
		}
	}

	for hash := range activeHashes {
		if _, matched := matchedSet[hash]; !matched {
			s.db.WithContext(ctx).Model(&model.DownloadTask{}).
				Where("client_id = ? AND info_hash = ?", name, hash).
				Update("first_matched_at", nil)
		}
	}
}

func splitRuleIDs(s string) []uint {
	if s == "" {
		return nil
	}
	var result []uint
	for _, part := range splitCSV(s) {
		if id, err := parseUint(part); err == nil && id > 0 {
			result = append(result, id)
		}
	}
	return result
}

func splitCSV(s string) []string {
	var result []string
	current := ""
	for _, c := range s {
		if c == ',' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func parseUint(s string) (uint, error) {
	var n uint
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errInvalidNumber
		}
		n = n*10 + uint(c-'0')
	}
	return n, nil
}

var errInvalidNumber = fmt.Errorf("invalid number")
