package orphan

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/reseed"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Recovery struct {
	db             *gorm.DB
	siteProvider   model.SiteInfoProvider
	clientProvider model.DownloaderProvider
	logger         *zap.Logger
}

func NewRecovery(db *gorm.DB, sp model.SiteInfoProvider, cp model.DownloaderProvider, logger *zap.Logger) *Recovery {
	return &Recovery{
		db:             db,
		siteProvider:   sp,
		clientProvider: cp,
		logger:         logger.With(zap.String("component", "orphan-recovery")),
	}
}

func (r *Recovery) Recover(ctx context.Context, orphan *Entry) *RecoverResult {
	result := &RecoverResult{Orphan: orphan}
	stats := &SearchStats{}

	siteName, torrentID, method := r.tryDBMatch(ctx, orphan)
	if siteName == "" {
		siteName, torrentID, method = r.tryL2Search(ctx, orphan, stats)
	}

	result.SearchStats = stats

	if siteName == "" {
		result.Message = fmt.Sprintf("no matching torrent found on any site (searched: %d, skipped: %d, failed: %d)",
			stats.Searched, stats.Skipped, len(stats.FailedSites))
		return result
	}

	result.Found = true
	result.Method = method
	result.SiteName = siteName

	if err := r.downloadAndAdd(ctx, orphan, siteName, torrentID); err != nil {
		result.Found = false
		result.Message = fmt.Sprintf("recovery failed: %v", err)
		return result
	}

	result.Message = fmt.Sprintf("recovered from %s (torrent_id=%s, method=%s)", siteName, torrentID, method)
	return result
}

func (r *Recovery) tryDBMatch(ctx context.Context, orphan *Entry) (siteName, torrentID, method string) {
	var candidate model.PublishCandidate
	if err := r.db.WithContext(ctx).
		Where("torrent_name = ?", orphan.Name).
		Order("updated_at DESC").
		First(&candidate).Error; err == nil && candidate.SourceSite != "" && candidate.SourceTorrentID != "" {
		r.logger.Debug("orphan DB match: publish_candidate",
			zap.String("orphan", orphan.Name),
			zap.String("site", candidate.SourceSite),
			zap.String("torrent_id", candidate.SourceTorrentID))
		return candidate.SourceSite, candidate.SourceTorrentID, "db:publish_candidate"
	}

	var meta model.TorrentMetadata
	if err := r.db.WithContext(ctx).
		Where("title = ?", orphan.Name).
		First(&meta).Error; err == nil && meta.SiteName != "" && meta.TorrentID != "" {
		r.logger.Debug("orphan DB match: torrent_metadata",
			zap.String("orphan", orphan.Name),
			zap.String("site", meta.SiteName))
		return meta.SiteName, meta.TorrentID, "db:torrent_metadata"
	}

	return "", "", ""
}

func (r *Recovery) tryL2Search(ctx context.Context, orphan *Entry, stats *SearchStats) (siteName, torrentID, method string) {
	if r.siteProvider == nil {
		return "", "", ""
	}

	components := titleparser.ParseTitle(orphan.Name)
	groupName := components.ReleaseGroup
	sites := r.getSitePriority(ctx, groupName, orphan.Size)
	if len(sites) == 0 {
		return "", "", ""
	}
	stats.TotalSites = len(sites)

	searchKeyword := reseed.ExtractSearchKeyword(orphan.Name)
	if searchKeyword == "" {
		searchKeyword = orphan.Name
	}
	r.logger.Info("orphan L2 search starting",
		zap.String("orphan", orphan.Name),
		zap.String("keyword", searchKeyword),
		zap.Int("sites", len(sites)))

	type matchResult struct {
		site, torrentID, method string
	}

	searchCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	resultCh := make(chan matchResult, 1)
	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup
	var statsMu sync.Mutex

	for _, site := range sites {
		wg.Add(1)
		go func(site string) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-searchCtx.Done():
				return
			}

			if searchCtx.Err() != nil {
				return
			}

			config, err := r.siteProvider.GetSiteConfig(searchCtx, site)
			if err != nil || config == nil {
				statsMu.Lock()
				stats.Skipped++
				statsMu.Unlock()
				return
			}

			if config.BaseURL != "" {
				httpclient.ResetDomainCircuit(config.BaseURL)
				httpclient.GlobalLimiter.ManualUnfreeze(config.BaseURL)
			}

			adapter, err := r.siteProvider.GetAdapter(searchCtx, site)
			if err != nil || adapter == nil {
				statsMu.Lock()
				stats.Skipped++
				statsMu.Unlock()
				return
			}

			siteCtx, siteCancel := context.WithTimeout(searchCtx, 15*time.Second)
			results, err := adapter.SearchTorrents(siteCtx, config, searchKeyword, nil)
			siteCancel()

			statsMu.Lock()
			if err != nil {
				stats.FailedSites = append(stats.FailedSites, SiteFailure{
					Site:   site,
					Reason: err.Error(),
				})
				statsMu.Unlock()
				return
			}
			stats.Searched++
			statsMu.Unlock()

			if len(results) == 0 {
				return
			}

			for _, res := range results {
				if res.Size > 0 && compareSize(orphan.Size, res.Size) {
					r.logger.Info("orphan L2 match",
						zap.String("orphan", orphan.Name),
						zap.String("site", site),
						zap.String("torrent_id", res.TorrentID),
						zap.Int64("size", res.Size))
					select {
					case resultCh <- matchResult{site, res.TorrentID, "l2:search:" + site}:
					case <-searchCtx.Done():
					}
					return
				}
			}
		}(site)
	}

	allDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(allDone)
	}()

	select {
	case result := <-resultCh:
		return result.site, result.torrentID, result.method
	case <-allDone:
		return "", "", ""
	}
}

func (r *Recovery) getSitePriority(ctx context.Context, groupName string, orphanSize int64) []string {
	sites, err := r.siteProvider.ListSites(ctx)
	if err != nil || len(sites) == 0 {
		return nil
	}

	enabledSites := make([]string, 0, len(sites))
	for _, s := range sites {
		if s.Enabled {
			enabledSites = append(enabledSites, s.Name)
		}
	}

	if groupName == "" {
		return enabledSites
	}

	type siteFreq struct {
		Name string
		Freq int
	}
	var freqs []siteFreq
	r.db.WithContext(ctx).Raw(
		`SELECT site_name, COUNT(*) as freq FROM seeding_torrent_records WHERE site_name IN ? GROUP BY site_name ORDER BY freq DESC`,
		enabledSites,
	).Scan(&freqs)

	type groupSiteFreq struct {
		SiteName string
		Freq     int
	}
	var groupFreqs []groupSiteFreq
	r.db.WithContext(ctx).Raw(
		`SELECT site_name, COUNT(*) as freq FROM seeding_torrent_records WHERE site_name IN ? AND info_hash IN (
			SELECT DISTINCT info_hash FROM publish_candidates WHERE torrent_name LIKE ?
		) GROUP BY site_name ORDER BY freq DESC`,
		enabledSites, "%-"+groupName,
	).Scan(&groupFreqs)

	priority := make([]string, 0, len(enabledSites))
	seen := make(map[string]bool)
	for _, gf := range groupFreqs {
		priority = append(priority, gf.SiteName)
		seen[gf.SiteName] = true
	}
	for _, sf := range freqs {
		if !seen[sf.Name] {
			priority = append(priority, sf.Name)
			seen[sf.Name] = true
		}
	}
	for _, s := range enabledSites {
		if !seen[s] {
			priority = append(priority, s)
		}
	}

	return priority
}

func (r *Recovery) downloadAndAdd(ctx context.Context, orphan *Entry, siteName, torrentID string) error {
	config, err := r.siteProvider.GetSiteConfig(ctx, siteName)
	if err != nil || config == nil {
		return fmt.Errorf("get site config: %w", err)
	}
	adapter, err := r.siteProvider.GetAdapter(ctx, siteName)
	if err != nil || adapter == nil {
		return fmt.Errorf("get adapter: %w", err)
	}

	dlCtx, cancel := context.WithTimeout(ctx, 60*1000*1000*1000)
	defer cancel()
	torrentData, err := adapter.DownloadTorrent(dlCtx, config, torrentID)
	if err != nil {
		return fmt.Errorf("download torrent: %w", err)
	}
	if len(torrentData) == 0 {
		return fmt.Errorf("downloaded torrent data is empty")
	}

	client, err := r.clientProvider.Get(orphan.ClientID)
	if err != nil {
		return fmt.Errorf("get downloader client: %w", err)
	}

	savePath := orphan.SavePath
	if !orphan.IsDir {
		savePath = filepath.Dir(orphan.Path)
	}

	_, err = client.AddFromFile(ctx, torrentData, model.AddTorrentOptions{
		SavePath: savePath,
		Paused:   true,
	})
	if err != nil {
		return fmt.Errorf("add to downloader: %w", err)
	}

	r.logger.Info("orphan recovered",
		zap.String("orphan", orphan.Name),
		zap.String("site", siteName),
		zap.String("save_path", savePath))

	return nil
}

func compareSize(sourceBytes, resultBytes int64) bool {
	const gb = 1073741824.0
	const mb = 1048576.0
	if sourceBytes >= int64(gb) || resultBytes >= int64(gb) {
		sGB := math.Round(float64(sourceBytes)/gb*100) / 100
		rGB := math.Round(float64(resultBytes)/gb*100) / 100
		return sGB == rGB
	}
	sMB := math.Round(float64(sourceBytes)/mb*100) / 100
	rMB := math.Round(float64(resultBytes)/mb*100) / 100
	return sMB == rMB
}
