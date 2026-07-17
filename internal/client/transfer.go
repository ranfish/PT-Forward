package client

import (
	"context"
	"fmt"

	"github.com/ranfish/pt-forward/internal/model"
)

// TransferTorrent 从源下载器导出种子并添加到目标下载器（跨下载器转移做种，§55.11）。
// 含路径映射（MapPath）+ Category/Tags 保留。供 download/syncer（下载器级）与
// seeding/transferRecord（订阅级）共用，消除两处重复实现。
//
// 不删源——由调用方在确认目标添加成功后自行 DeleteTorrent(deleteFiles=false)，
// 以便多目标场景下所有目标都成功后再删源。
func TransferTorrent(ctx context.Context, source, target model.DownloaderClient, infoHash string) (*model.AddResult, error) {
	torrentData, err := source.ExportTorrent(ctx, infoHash)
	if err != nil {
		return nil, fmt.Errorf("export torrent: %w", err)
	}
	sourceTorrent, _ := source.GetTorrentByHash(ctx, infoHash)
	opts := model.AddTorrentOptions{Paused: false}
	if sourceTorrent != nil {
		opts.SavePath = MapPath(sourceTorrent.SavePath, source.GetSharedPaths())
		opts.Category = sourceTorrent.Category
		opts.Tags = sourceTorrent.Tags
	}
	return target.AddFromFile(ctx, torrentData, opts)
}
