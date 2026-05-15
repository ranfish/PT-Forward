package watcher

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/client"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type CompletionWatcher struct {
	db        *gorm.DB
	clientMgr *client.Manager
	pipeline  *publish.Pipeline
	logger    *zap.Logger

	watchStore   sync.Map
	pollInterval time.Duration
}

func NewCompletionWatcher(db *gorm.DB, clientMgr *client.Manager, pipeline *publish.Pipeline, logger *zap.Logger) *CompletionWatcher {
	return &CompletionWatcher{
		db:           db,
		clientMgr:    clientMgr,
		pipeline:     pipeline,
		logger:       logger,
		pollInterval: 30 * time.Second,
	}
}

func (w *CompletionWatcher) SetPollInterval(d time.Duration) {
	w.pollInterval = d
}

func (w *CompletionWatcher) Start(ctx context.Context) error {
	w.recoverPendingWatches(ctx)

	go w.watchLoop(ctx)

	w.logger.Info("completion watcher started")
	return nil
}

func (w *CompletionWatcher) Stop() {
	w.logger.Info("completion watcher stopped")
}

func (w *CompletionWatcher) Watch(_ context.Context, clientName, infoHash string, candidateID uint) error {
	if clientName == "" || infoHash == "" {
		return &model.AppError{Code: 40001, Message: "client_name and info_hash are required"}
	}
	key := clientName + "|" + infoHash
	w.watchStore.Store(key, candidateID)
	w.logger.Debug("watch registered",
		zap.String("client", clientName),
		zap.String("info_hash", infoHash),
		zap.Uint("candidate_id", candidateID),
	)
	return nil
}

func (w *CompletionWatcher) SubmitCandidate(ctx context.Context, candidate model.PublishCandidate) error {
	if candidate.SourceSite == "" || candidate.SourceTorrentID == "" {
		return &model.AppError{Code: 40001, Message: "source_site and source_torrent_id are required"}
	}

	var existing model.PublishCandidate
	err := w.db.WithContext(ctx).
		Where("source_site = ? AND source_torrent_id = ? AND publish_status IN ?",
			candidate.SourceSite, candidate.SourceTorrentID,
			[]model.PublishCandidateStatus{model.CandidatePending, model.CandidateDownloading, model.CandidatePublishing},
		).First(&existing).Error
	if err == nil {
		w.logger.Debug("candidate already exists, skipping",
			zap.String("source_site", candidate.SourceSite),
			zap.String("torrent_id", candidate.SourceTorrentID),
			zap.Uint("existing_id", existing.ID),
		)
		return nil
	}

	if candidate.PublishStatus == "" {
		candidate.PublishStatus = model.CandidatePending
	}
	if candidate.Role == "" {
		candidate.Role = model.RoleDownload
	}

	if err := w.db.WithContext(ctx).Create(&candidate).Error; err != nil {
		return &model.AppError{Code: 50001, Message: "创建发布候选失败", Cause: err}
	}

	if candidate.ClientID != "" && candidate.InfoHash != "" {
		if err := w.Watch(ctx, candidate.ClientID, candidate.InfoHash, candidate.ID); err != nil {
			w.logger.Warn("failed to register watch for candidate",
				zap.Uint("candidate_id", candidate.ID),
				zap.Error(err),
			)
		}
	}

	w.logger.Info("candidate submitted",
		zap.Uint("id", candidate.ID),
		zap.String("source_site", candidate.SourceSite),
		zap.String("torrent_id", candidate.SourceTorrentID),
	)

	return nil
}

func (w *CompletionWatcher) watchLoop(ctx context.Context) {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.pollOnce(ctx)
		}
	}
}

func (w *CompletionWatcher) pollOnce(ctx context.Context) {
	if w.clientMgr == nil {
		return
	}

	w.watchStore.Range(func(key, value any) bool {
		if ctx.Err() != nil {
			return false
		}

		keyStr := key.(string)
		candidateID := value.(uint)

		parts := strings.SplitN(keyStr, "|", 2)
		if len(parts) != 2 {
			w.watchStore.Delete(key)
			return true
		}
		clientName, infoHash := parts[0], parts[1]

		dl, err := w.clientMgr.Get(clientName)
		if err != nil {
			w.logger.Debug("watch: downloader not available",
				zap.String("client", clientName),
				zap.Error(err),
			)
			return true
		}

		torrent, err := dl.GetTorrentByHash(ctx, infoHash)
		if err != nil {
			w.logger.Debug("watch: get torrent failed",
				zap.String("client", clientName),
				zap.String("info_hash", infoHash),
				zap.Error(err),
			)
			return true
		}

		if torrent == nil {
			w.logger.Warn("watch: torrent not found in downloader (orphan)",
				zap.String("client", clientName),
				zap.String("info_hash", infoHash),
			)
			w.watchStore.Delete(key)
			w.markCandidateOrphan(ctx, candidateID)
			return true
		}

		if torrent.IsFinished {
			w.watchStore.Delete(key)
			w.onWatchCompleted(ctx, candidateID, torrent)
		}

		return true
	})
}

func (w *CompletionWatcher) onWatchCompleted(ctx context.Context, candidateID uint, torrent *model.TorrentInfo) {
	var candidate model.PublishCandidate
	if err := w.db.WithContext(ctx).First(&candidate, candidateID).Error; err != nil {
		w.logger.Error("failed to load candidate for completion", zap.Uint("candidate_id", candidateID), zap.Error(err))
		return
	}

	if candidate.Role == model.RoleSource && candidate.ClientID != "" {
		sourceClient, err := w.clientMgr.Get(candidate.ClientID)
		if err != nil {
			w.logger.Error("failed to get source client for transfer", zap.Error(err))
		} else if sourceClient.GetReseedTargetID() != "" {
			reseedClientName, reseedHash, err := w.transferToReseed(ctx, &candidate, sourceClient, torrent)
			if err != nil {
				w.logger.Error("transfer to reseed failed, continuing with source client",
					zap.Uint("candidate_id", candidateID),
					zap.Error(err),
				)
			} else {
				w.logger.Info("transfer to reseed completed",
					zap.Uint("candidate_id", candidateID),
					zap.String("reseed_client", reseedClientName),
					zap.String("reseed_hash", reseedHash),
				)
				candidate.ClientID = reseedClientName
				candidate.InfoHash = reseedHash
				candidate.SourceClientID = sourceClient.GetName()
			}
		}
	}

	w.logger.Info("download completed, triggering publish",
		zap.Uint("candidate_id", candidateID),
		zap.String("save_path", torrent.SavePath),
	)

	now := time.Now()
	updates := map[string]interface{}{
		"download_completed": true,
		"completed_at":       &now,
		"local_save_path":    torrent.SavePath,
		"publish_status":     model.CandidateCompleted,
		"updated_at":         now,
	}
	if candidate.ClientID != "" {
		updates["client_id"] = candidate.ClientID
	}
	if candidate.InfoHash != "" {
		updates["info_hash"] = candidate.InfoHash
	}
	if candidate.SourceClientID != "" {
		updates["source_client_id"] = candidate.SourceClientID
	}

	if err := w.db.WithContext(ctx).Model(&model.PublishCandidate{}).
		Where("id = ?", candidateID).
		Updates(updates).Error; err != nil {
		w.logger.Error("failed to update candidate completion status",
			zap.Uint("candidate_id", candidateID),
			zap.Error(err),
		)
		return
	}

	if w.pipeline == nil {
		w.logger.Warn("pipeline not set, skipping publish",
			zap.Uint("candidate_id", candidateID),
		)
		return
	}

	if _, err := w.pipeline.PublishCandidate(ctx, candidateID); err != nil {
		w.logger.Error("publish candidate failed",
			zap.Uint("candidate_id", candidateID),
			zap.Error(err),
		)
	}
}

func (w *CompletionWatcher) transferToReseed(ctx context.Context, candidate *model.PublishCandidate, sourceClient model.DownloaderClient, torrent *model.TorrentInfo) (string, string, error) {
	reseedClient, err := w.clientMgr.Get(sourceClient.GetReseedTargetID())
	if err != nil {
		return "", "", fmt.Errorf("get reseed client %s: %w", sourceClient.GetReseedTargetID(), err)
	}

	torrentData, err := sourceClient.ExportTorrent(ctx, candidate.InfoHash)
	if err != nil {
		return "", "", fmt.Errorf("export torrent from source: %w", err)
	}

	reseedPath := client.MapPath(torrent.SavePath, sourceClient.GetSharedPaths())

	opts := model.AddTorrentOptions{
		SavePath: reseedPath,
		Category: "reseed",
		Paused:   false,
	}

	addResult, err := reseedClient.AddFromFile(ctx, torrentData, opts)
	if err != nil {
		return "", "", fmt.Errorf("add torrent to reseed client: %w", err)
	}

	if err := sourceClient.DeleteTorrent(ctx, candidate.InfoHash, false); err != nil {
		w.logger.Warn("failed to remove torrent from source after transfer (reseed already added)",
			zap.String("source_client", sourceClient.GetName()),
			zap.String("hash", candidate.InfoHash),
			zap.Error(err),
		)
	}

	return reseedClient.GetName(), addResult.InfoHash, nil
}

func (w *CompletionWatcher) markCandidateOrphan(ctx context.Context, candidateID uint) {
	if err := w.db.WithContext(ctx).Model(&model.PublishCandidate{}).
		Where("id = ? AND download_completed = ?", candidateID, false).
		Updates(map[string]interface{}{
			"publish_status": model.CandidateOrphan,
			"skip_reason":    "种子在下载器中不存在",
			"updated_at":     time.Now(),
		}).Error; err != nil {
		w.logger.Error("failed to mark candidate as orphan",
			zap.Uint("candidate_id", candidateID),
			zap.Error(err),
		)
	}
}

func (w *CompletionWatcher) recoverPendingWatches(ctx context.Context) {
	var candidates []model.PublishCandidate
	if err := w.db.WithContext(ctx).
		Where("download_completed = ? AND publish_status IN ?",
			false,
			[]model.PublishCandidateStatus{model.CandidatePending, model.CandidateDownloading},
		).Find(&candidates).Error; err != nil {
		w.logger.Warn("failed to recover pending watches", zap.Error(err))
		return
	}

	recovered := 0
	for _, c := range candidates {
		if c.ClientID != "" && c.InfoHash != "" {
			key := c.ClientID + "|" + c.InfoHash
			w.watchStore.Store(key, c.ID)
			recovered++
		}
	}

	if recovered > 0 {
		w.logger.Info("recovered pending watches", zap.Int("count", recovered))
	}
}

func (w *CompletionWatcher) ActiveWatchCount() int {
	count := 0
	w.watchStore.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

func (w *CompletionWatcher) IsWatching(clientName, infoHash string) bool {
	key := clientName + "|" + infoHash
	_, ok := w.watchStore.Load(key)
	return ok
}

func fmtCandidate(candidateID uint) string {
	return fmt.Sprintf("candidate-%d", candidateID)
}
