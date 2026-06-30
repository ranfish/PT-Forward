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

	if task.Status == status && task.Progress == progress {
		return
	}

	s.repo.UpdateStatus(ctx, task.ID, status, progress, ti.Error)

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
