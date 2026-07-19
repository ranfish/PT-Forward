// Package publish 加种回下载器（§56.30）。
//
// 发布成功后自动加种到源下载器（共享存储，hash 检查跳过下载直接做种）。
// 加种失败不回滚（仅 Warn 日志 + PublishResultRecord 标记）。
package publish

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// TorrentDownloader 下载 .torrent 文件（adapter 实现）。
type TorrentDownloader interface {
	DownloadTorrent(ctx context.Context, siteName, torrentID string) ([]byte, error)
}

// TorrentPusher 推种到下载器（pusher 实现）。
type TorrentPusher interface {
	Push(ctx context.Context, clientID string, torrentData []byte, savePath string) error
}

// AutoReseedResult 加种结果。
type AutoReseedResult struct {
	Seeded bool   `json:"seeded"`
	Error  string `json:"error,omitempty"`
}

// AutoReseed §56.30: 发布成功后自动加种到源下载器。
//   downloader: 目标站 .torrent 下载器
//   pusher: 下载器推种接口
//   targetSite/torrentID: 目标站种子标识
//   sourceClientID: 源下载器 ID（文件已在此下载器共享存储）
//   savePath: 源种子的保存路径（复用）
// 加种失败不回滚，仅记录错误。
func AutoReseed(
	ctx context.Context,
	downloader TorrentDownloader,
	pusher TorrentPusher,
	targetSite, torrentID string,
	sourceClientID, savePath string,
	logger *zap.Logger,
) *AutoReseedResult {
	result := &AutoReseedResult{}

	if downloader == nil || pusher == nil {
		result.Error = "downloader 或 pusher 未配置"
		return result
	}

	// 1. 下载目标站 .torrent
	torrentData, err := downloader.DownloadTorrent(ctx, targetSite, torrentID)
	if err != nil {
		result.Error = "下载 .torrent 失败: " + err.Error()
		if logger != nil {
			logger.Warn("auto reseed: download torrent failed",
				zap.String("site", targetSite),
				zap.String("torrent_id", torrentID),
				zap.Error(err))
		}
		return result
	}

	if len(torrentData) == 0 {
		result.Error = "下载的 .torrent 数据为空"
		return result
	}

	// 2. 推种到下载器（source client，共享存储 hash 检查跳过下载）
	if err := pusher.Push(ctx, sourceClientID, torrentData, savePath); err != nil {
		result.Error = "推种失败: " + err.Error()
		if logger != nil {
			logger.Warn("auto reseed: push failed",
				zap.String("client_id", sourceClientID),
				zap.Error(err))
		}
		return result
	}

	result.Seeded = true
	if logger != nil {
		logger.Info("auto reseed: seeded successfully",
			zap.String("site", targetSite),
			zap.String("client_id", sourceClientID))
	}
	return result
}

// MarkRecordSeeded 更新 PublishResultRecord 的加种状态。
// 由调用方在 AutoReseed 返回后调用。
func MarkRecordSeeded(seeded bool, seedError string) (bool, *time.Time, string) {
	if seeded {
		now := time.Now()
		return true, &now, ""
	}
	return false, nil, seedError
}
