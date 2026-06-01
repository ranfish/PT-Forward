package transmission

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

func (c *TRClient) GetTorrentByHash(ctx context.Context, hash string) (*model.TorrentInfo, error) {
	torrents, err := c.getTorrents(ctx, []string{hash}, allFields)
	if err != nil {
		return nil, err
	}
	if len(torrents) == 0 {
		return nil, nil
	}
	return torrents[0].toModel(), nil
}

func (c *TRClient) GetSeedingTorrents(ctx context.Context) ([]*model.TorrentInfo, error) {
	torrents, err := c.getTorrents(ctx, nil, allFields)
	if err != nil {
		return nil, err
	}
	result := make([]*model.TorrentInfo, 0, len(torrents))
	for i := range torrents {
		if isSeedingStatus(torrents[i].Status) {
			result = append(result, torrents[i].toModel())
		}
	}
	return result, nil
}

func (c *TRClient) GetAllTorrents(ctx context.Context) ([]*model.TorrentInfo, error) {
	torrents, err := c.getTorrents(ctx, nil, allFields)
	if err != nil {
		return nil, err
	}
	result := make([]*model.TorrentInfo, 0, len(torrents))
	for i := range torrents {
		result = append(result, torrents[i].toModel())
	}
	return result, nil
}

func (c *TRClient) GetTorrentsByPath(ctx context.Context, savePath string) ([]*model.TorrentInfo, error) {
	torrents, err := c.getTorrents(ctx, nil, allFields)
	if err != nil {
		return nil, err
	}
	result := make([]*model.TorrentInfo, 0)
	for i := range torrents {
		if torrents[i].DownloadDir == savePath {
			result = append(result, torrents[i].toModel())
		}
	}
	return result, nil
}

func (c *TRClient) GetMainData(ctx context.Context) (*model.Maindata, error) {
	torrents, err := c.getTorrents(ctx, nil, allFields)
	if err != nil {
		return nil, err
	}
	torrentMap := make(map[string]model.TorrentInfo, len(torrents))
	var totalSize int64
	var inflightBytes int64
	for _, t := range torrents {
		mt := *t.toModel()
		torrentMap[t.HashString] = mt
		totalSize += mt.TotalSize
		if t.PercentDone < 1.0 {
			inflightBytes += int64(float64(t.TotalSize) * (1.0 - t.PercentDone))
		}
	}
	freeSpace, _ := c.GetFreeSpace(ctx)
	dlSpeed, ulSpeed := c.getSessionStats(ctx)
	return &model.Maindata{
		Torrents:       torrentMap,
		FreeSpace:      freeSpace,
		TotalDiskSpace: freeSpace + totalSize,
		InflightBytes:  inflightBytes,
		ServerState:    model.ServerState{DownloadSpeed: dlSpeed, UploadSpeed: ulSpeed},
	}, nil
}

func (c *TRClient) getSessionStats(ctx context.Context) (int64, int64) {
	resp, err := c.rpcCall(ctx, "session-stats", nil)
	if err != nil {
		return 0, 0
	}
	var stats struct {
		DownloadSpeed int64 `json:"downloadSpeed"`
		UploadSpeed   int64 `json:"uploadSpeed"`
	}
	if err := json.Unmarshal(resp.Arguments, &stats); err != nil {
		return 0, 0
	}
	return stats.DownloadSpeed, stats.UploadSpeed
}

func (c *TRClient) GetMainDataIncremental(ctx context.Context, rid int) (*model.Maindata, int, error) {
	md, err := c.GetMainData(ctx)
	if err != nil {
		return nil, 0, err
	}
	return md, rid + 1, nil
}

func (c *TRClient) AddFromFile(ctx context.Context, data []byte, opts model.AddTorrentOptions) (*model.AddResult, error) {
	metainfo := base64.StdEncoding.EncodeToString(data)

	args := map[string]interface{}{
		"metainfo": metainfo,
		"paused":   opts.Paused,
	}
	if opts.SavePath != "" {
		args["download-dir"] = opts.SavePath
	}
	if opts.Category != "" || len(opts.Tags) > 0 {
		args["labels"] = buildTRLabels(opts.Category, opts.Tags)
	}

	resp, err := c.rpcCall(ctx, "torrent-add", args)
	if err != nil {
		return nil, err
	}

	var addResult struct {
		TorrentAdded *struct {
			HashString string `json:"hashString"`
			Name       string `json:"name"`
			ID         int    `json:"id"`
		} `json:"torrent-added"`
		TorrentDuplicate *struct {
			HashString string `json:"hashString"`
			Name       string `json:"name"`
			ID         int    `json:"id"`
		} `json:"torrent-duplicate"`
	}
	if err := json.Unmarshal(resp.Arguments, &addResult); err != nil {
		return nil, c.wrapErr(11007, "decode torrent-add response", err)
	}

	var hash, name string
	var isDup bool
	switch {
	case addResult.TorrentDuplicate != nil:
		hash = addResult.TorrentDuplicate.HashString
		name = addResult.TorrentDuplicate.Name
		isDup = true
	case addResult.TorrentAdded != nil:
		hash = addResult.TorrentAdded.HashString
		name = addResult.TorrentAdded.Name
	default:
		return nil, c.newErr(11007, "torrent-add returned no hash")
	}

	if !isDup && (opts.UploadLimit > 0 || opts.DownloadLimit > 0 || opts.RatioLimit > 0 || opts.SeedingTimeLimit > 0) {
		setArgs := map[string]interface{}{
			"ids": []string{hash},
		}
		if opts.UploadLimit > 0 {
			setArgs["uploadLimit"] = opts.UploadLimit / 1024
			setArgs["uploadLimited"] = true
		}
		if opts.DownloadLimit > 0 {
			setArgs["downloadLimit"] = opts.DownloadLimit / 1024
			setArgs["downloadLimited"] = true
		}
		if opts.RatioLimit > 0 {
			setArgs["seedRatioLimit"] = opts.RatioLimit
			setArgs["seedRatioMode"] = 1
		}
		if opts.SeedingTimeLimit > 0 {
			setArgs["seedIdleLimit"] = opts.SeedingTimeLimit
			setArgs["seedIdleMode"] = 1
		}
		if _, err := c.rpcCall(ctx, "torrent-set", setArgs); err != nil {
			c.logger.Warn("post-add torrent-set limits failed", zap.String("hash", hash), zap.Error(err))
		}
	}

	return &model.AddResult{
		InfoHash:    hash,
		Name:        name,
		IsDuplicate: isDup,
	}, nil
}

func (c *TRClient) ExportTorrent(ctx context.Context, hash string) ([]byte, error) {
	torrents, err := c.getTorrents(ctx, []string{hash}, []string{"torrentFile"})
	if err != nil || len(torrents) == 0 {
		return nil, c.newErr(11005, "torrent not found for export")
	}
	torrentPath := torrents[0].TorrentFile
	if torrentPath == "" {
		return nil, c.newErr(11005, "torrent file path is empty")
	}
	data, err := os.ReadFile(torrentPath) //nolint:gosec // torrentPath from client config, controlled by admin
	if err != nil {
		return nil, c.wrapErr(11002, "read torrent file", err)
	}
	return data, nil
}

func (c *TRClient) DeleteTorrent(ctx context.Context, hash string, deleteFiles bool) error {
	_, err := c.rpcCall(ctx, "torrent-remove", map[string]interface{}{
		"ids":               []string{hash},
		"delete-local-data": deleteFiles,
	})
	return err
}

func (c *TRClient) BatchDeleteTorrents(ctx context.Context, hashes []string, deleteFiles bool) error {
	_, err := c.rpcCall(ctx, "torrent-remove", map[string]interface{}{
		"ids":               hashes,
		"delete-local-data": deleteFiles,
	})
	return err
}

func (c *TRClient) PauseTorrent(ctx context.Context, hash string) error {
	return c.torrentAction(ctx, "torrent-stop", []string{hash})
}

func (c *TRClient) ResumeTorrent(ctx context.Context, hash string) error {
	return c.torrentAction(ctx, "torrent-start", []string{hash})
}

func (c *TRClient) Reannounce(ctx context.Context, hash string) error {
	return c.torrentAction(ctx, "torrent-reannounce", []string{hash})
}

func (c *TRClient) Recheck(ctx context.Context, hash string) error {
	return c.torrentAction(ctx, "torrent-verify", []string{hash})
}

func (c *TRClient) SetTorrentTags(ctx context.Context, hash string, tags []string) error {
	current, err := c.getLabels(ctx, hash)
	if err != nil {
		return err
	}
	var category string
	if len(current) > 0 {
		category = current[0]
	}
	return c.setLabels(ctx, hash, mergeLabels(current, category, tags))
}

func (c *TRClient) RemoveTorrentTags(ctx context.Context, hash string, tags []string) error {
	current, err := c.getLabels(ctx, hash)
	if err != nil {
		return err
	}
	return c.setLabels(ctx, hash, removeLabels(current, tags, true))
}

func (c *TRClient) SetCategory(ctx context.Context, hash string, category string) error {
	current, err := c.getLabels(ctx, hash)
	if err != nil {
		return err
	}
	var tags []string
	if len(current) > 1 {
		tags = current[1:]
	}
	return c.setLabels(ctx, hash, mergeLabels(current, category, tags))
}

func (c *TRClient) SetSavePath(ctx context.Context, hash string, savePath string) error {
	_, err := c.rpcCall(ctx, "torrent-set-location", map[string]interface{}{
		"ids":      []string{hash},
		"location": savePath,
		"move":     true,
	})
	return err
}

func (c *TRClient) SetSuperSeeding(ctx context.Context, hash string, enable bool) error {
	priority := "normal"
	if enable {
		priority = "high"
	}
	_, err := c.rpcCall(ctx, "torrent-set", map[string]interface{}{
		"ids":               []string{hash},
		"bandwidthPriority": priority,
	})
	return err
}

func (c *TRClient) SetUploadLimit(ctx context.Context, infoHash string, limitBytesPerSec int64) error {
	_, err := c.rpcCall(ctx, "torrent-set", map[string]interface{}{
		"ids":           []string{infoHash},
		"uploadLimit":   limitBytesPerSec / 1024,
		"uploadLimited": true,
	})
	return err
}

func (c *TRClient) PauseAllDownloads(ctx context.Context) error {
	return c.torrentAction(ctx, "torrent-stop", nil)
}

func (c *TRClient) ResumeAllDownloads(ctx context.Context) error {
	return c.torrentAction(ctx, "torrent-start", nil)
}

func (c *TRClient) GetFreeSpace(ctx context.Context) (int64, error) {
	resp, err := c.rpcCall(ctx, "session-get", map[string]interface{}{
		"fields": []string{"download-dir"},
	})
	if err != nil {
		return 0, err
	}
	var session struct {
		DownloadDir string `json:"download-dir"`
	}
	if err := json.Unmarshal(resp.Arguments, &session); err != nil {
		return 0, c.wrapErr(11002, "decode session-get", err)
	}

	if session.DownloadDir == "" {
		return 0, nil
	}

	resp2, err := c.rpcCall(ctx, "free-space", map[string]interface{}{
		"path": session.DownloadDir,
	})
	if err != nil {
		return 0, err
	}
	var space struct {
		SizeBytes int64 `json:"size-bytes"`
	}
	if err := json.Unmarshal(resp2.Arguments, &space); err != nil {
		return 0, c.wrapErr(11002, "decode free-space", err)
	}
	return space.SizeBytes, nil
}

func (c *TRClient) CheckExists(ctx context.Context, infoHash string) (bool, error) {
	t, err := c.GetTorrentByHash(ctx, infoHash)
	if err != nil {
		return false, err
	}
	return t != nil, nil
}

func (c *TRClient) GetGlobalTransferStats(ctx context.Context) (*model.GlobalTransferStats, error) {
	resp, err := c.rpcCall(ctx, "session-stats", nil)
	if err != nil {
		return nil, fmt.Errorf("session-stats rpc: %w", err)
	}
	var stats struct {
		CumulativeStats struct {
			UploadedBytes   int64 `json:"uploadedBytes"`
			DownloadedBytes int64 `json:"downloadedBytes"`
		} `json:"cumulative-stats"`
	}
	if err := json.Unmarshal(resp.Arguments, &stats); err != nil {
		return nil, fmt.Errorf("decode session-stats: %w", err)
	}
	return &model.GlobalTransferStats{
		AllTimeUpload:   stats.CumulativeStats.UploadedBytes,
		AllTimeDownload: stats.CumulativeStats.DownloadedBytes,
	}, nil
}
