package orphan

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/reseed"
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

	groupName := reseed.ExtractGroupName(orphan.Name)
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
		zap.String("group", groupName),
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

			siteCtx, siteCancel := context.WithTimeout(searchCtx, 20*time.Second)
			match, err := reseed.SearchAndVerifyMatch(siteCtx, adapter, config, searchKeyword, groupName, orphan.Size)
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

			if match != nil {
				r.logger.Info("orphan L2 match",
					zap.String("orphan", orphan.Name),
					zap.String("site", site),
					zap.String("torrent_id", match.TorrentID),
					zap.String("matched_title", match.Title),
					zap.Int64("orphan_size", orphan.Size),
					zap.Int64("matched_size", match.Size))
				select {
				case resultCh <- matchResult{site, match.TorrentID, "l2:search:" + site}:
				case <-searchCtx.Done():
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

	category := "orphan-recover"
	tags := []string{"orphan-recover", "from:" + siteName}
	if r.db != nil {
		var catVal, tagVal string
		r.db.Raw("SELECT value FROM system_settings WHERE key = 'orphan_recover_category' LIMIT 1").Scan(&catVal)
		if catVal != "" {
			category = catVal
		}
		r.db.Raw("SELECT value FROM system_settings WHERE key = 'orphan_recover_tags' LIMIT 1").Scan(&tagVal)
		if tagVal != "" {
			tags = strings.Split(tagVal, ",")
			for i := range tags {
				tags[i] = strings.TrimSpace(tags[i])
			}
			tags = append(tags, "from:"+siteName)
		}
	}

	addResult, err := client.AddFromFile(ctx, torrentData, model.AddTorrentOptions{
		SavePath: savePath,
		Category: category,
		Tags:     tags,
		Paused:   true,
	})
	if err != nil {
		return fmt.Errorf("add to downloader: %w", err)
	}

	infoHash := addResult.InfoHash
	if infoHash != "" {
		if recheckErr := waitForRecheck(ctx, client, infoHash, 120*time.Second); recheckErr != nil {
			r.logger.Warn("orphan recheck failed",
				zap.String("orphan", orphan.Name),
				zap.String("hash", infoHash),
				zap.Error(recheckErr))
		} else {
			if resumeErr := client.ResumeTorrent(ctx, infoHash); resumeErr != nil {
				r.logger.Warn("orphan resume failed",
					zap.String("orphan", orphan.Name),
					zap.String("hash", infoHash),
					zap.Error(resumeErr))
			}
		}
	}

	r.logger.Info("orphan recovered",
		zap.String("orphan", orphan.Name),
		zap.String("site", siteName),
		zap.String("save_path", savePath),
		zap.String("hash", infoHash))

	return nil
}

func waitForRecheck(ctx context.Context, dlClient model.DownloaderClient, infoHash string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	interval := 3 * time.Second
	gracePeriod := 15 * time.Second
	startTime := time.Now()

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		time.Sleep(interval)
		ti, err := dlClient.GetTorrentByHash(ctx, infoHash)
		if err != nil || ti == nil {
			continue
		}
		if strings.HasPrefix(ti.State, "checking") {
			continue
		}
		if ti.Progress >= 1.0 {
			return nil
		}
		if time.Since(startTime) < gracePeriod {
			continue
		}
		return fmt.Errorf("data verification incomplete: %.1f%% state=%s", ti.Progress*100, ti.State)
	}
	return fmt.Errorf("recheck timeout after %v", timeout)
}
