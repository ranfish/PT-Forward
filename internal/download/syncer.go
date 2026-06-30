package download

import (
	"context"
	"time"

	"github.com/ranfish/pt-forward/internal/client"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Syncer struct {
	db          *gorm.DB
	repo        *Repository
	clientMgr   *client.Manager
	logger      *zap.Logger
	intervalSec int
}

func NewSyncer(db *gorm.DB, clientMgr *client.Manager, logger *zap.Logger) *Syncer {
	return &Syncer{
		db:          db,
		repo:        NewRepository(db),
		clientMgr:   clientMgr,
		logger:      logger,
		intervalSec: 20,
	}
}

func (s *Syncer) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(s.intervalSec) * time.Second)
	defer ticker.Stop()

	s.logger.Info("download syncer started", zap.Int("interval_sec", s.intervalSec))
	s.sync(ctx)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("download syncer stopped")
			return
		case <-ticker.C:
			s.sync(ctx)
			s.processTransfers(ctx)
		}
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

	s.repo.UpdateProgress(ctx, task.ID, map[string]interface{}{
		"status":          status,
		"progress":        progress,
		"error_message":   ti.Error,
		"upload_speed":    ti.UploadSpeed,
		"download_speed":  ti.DownloadSpeed,
		"ratio":           ti.Ratio,
		"uploaded":        ti.Uploaded,
		"num_seeds":       ti.NumComplete,
		"num_peers":       ti.NumIncomplete,
	})

	if status == model.DownloadStatusCompleted && task.Status != model.DownloadStatusCompleted {
		s.logger.Info("download completed",
			zap.Uint("id", task.ID),
			zap.String("client", clientID),
			zap.String("name", ti.Name))
	}
}

func (s *Syncer) importTask(ctx context.Context, clientID string, ti *model.TorrentInfo) {
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

func (s *Syncer) processTransfers(ctx context.Context) {
	var tasks []model.DownloadTask
	s.db.WithContext(ctx).
		Where("status = ? AND transfer_status != ?",
			model.DownloadStatusCompleted,
			model.TransferStatusTransferred).
		Find(&tasks)

	for _, task := range tasks {
		s.processTransfer(ctx, &task)
	}
}

func (s *Syncer) processTransfer(ctx context.Context, task *model.DownloadTask) {
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

		torrentData, err := sourceClient.ExportTorrent(ctx, task.InfoHash)
		if err != nil {
			s.logger.Error("transfer: export torrent failed",
				zap.Uint("id", task.ID), zap.Error(err))
			s.repo.UpdateTransfer(ctx, task.ID, model.TransferStatusFailed, targetID, "")
			return
		}

		sourceTorrent, _ := sourceClient.GetTorrentByHash(ctx, task.InfoHash)
		var savePath string
		if sourceTorrent != nil {
			savePath = client.MapPath(sourceTorrent.SavePath, sourceClient.GetSharedPaths())
		}

		opts := model.AddTorrentOptions{
			SavePath: savePath,
			Paused:   false,
		}
		result, err := targetClient.AddFromFile(ctx, torrentData, opts)
		if err != nil {
			s.logger.Error("transfer: add to target failed",
				zap.Uint("id", task.ID), zap.String("target", targetID), zap.Error(err))
			s.repo.UpdateTransfer(ctx, task.ID, model.TransferStatusFailed, targetID, "")
			return
		}

		s.logger.Info("transfer: added to target",
			zap.Uint("id", task.ID),
			zap.String("target", targetID),
			zap.String("new_hash", result.InfoHash))

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
