package orphan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/reseed"
	"github.com/ranfish/pt-forward/internal/util"
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

func (r *Recovery) Recover(ctx context.Context, orphan *Entry, targetClientID string) *RecoverResult {
	result := &RecoverResult{Orphan: orphan}
	stats := &SearchStats{}

	siteName, torrentID, method := r.tryDBMatch(ctx, orphan)
	if siteName == "" {
		siteName, torrentID, method = r.tryL2Search(ctx, orphan, stats)
	}
	if siteName == "" && orphan.IsDir && !reseed.DetectMusicFromDir(orphan.Path) {
		siteName, torrentID, method = r.tryFileLevelL2Search(ctx, orphan, stats)
	}

	result.SearchStats = stats

	if siteName != "" {
		result.Found = true
		result.Method = method
		result.SiteName = siteName
		if err := r.downloadAndAdd(ctx, orphan, siteName, torrentID, "", targetClientID); err != nil {
			result.Found = false
			result.Message = fmt.Sprintf("recovery failed: %v", err)
			return result
		}
		result.Message = fmt.Sprintf("recovered from %s (method=%s)", siteName, method)
		return result
	}

	result.Message = fmt.Sprintf("no matching torrent found on any site (searched: %d, skipped: %d, failed: %d)",
		stats.Searched, stats.Skipped, len(stats.FailedSites))
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

var searchableVideoExts = map[string]bool{
	".mkv": true, ".mp4": true, ".ts": true, ".m2ts": true, ".iso": true,
}

func (r *Recovery) tryFileLevelL2Search(ctx context.Context, orphan *Entry, stats *SearchStats) (siteName, torrentID, method string) {
	entries, err := os.ReadDir(orphan.Path)
	if err != nil || len(entries) == 0 {
		return "", "", ""
	}

	var largestFile string
	var largestSize int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !searchableVideoExts[ext] {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.Size() > largestSize {
			largestSize = info.Size()
			largestFile = e.Name()
		}
	}
	if largestFile == "" {
		return "", "", ""
	}

	baseName := strings.TrimSuffix(largestFile, filepath.Ext(largestFile))
	fileKeyword := reseed.ExtractSearchKeyword(baseName)
	fileGroup := reseed.ExtractGroupName(baseName)

	dirKeyword := reseed.ExtractSearchKeyword(orphan.Name)
	if fileKeyword == "" || reseed.KeywordHasNoTitle(fileKeyword) || fileKeyword == dirKeyword {
		return "", "", ""
	}

	r.logger.Info("orphan file-level L2: keyword from largest file",
		zap.String("orphan", orphan.Name),
		zap.String("file", largestFile),
		zap.String("keyword", fileKeyword),
		zap.String("group", fileGroup))

	return r.tryL2SearchCore(ctx, orphan, stats, fileKeyword, fileGroup)
}

func (r *Recovery) tryL2Search(ctx context.Context, orphan *Entry, stats *SearchStats) (siteName, torrentID, method string) {
	if orphan.IsDir && reseed.DetectMusicFromDir(orphan.Path) {
		musicKeyword := reseed.ExtractMusicKeyword(orphan.Name)
		r.logger.Info("orphan L2: music detected",
			zap.String("orphan", orphan.Name),
			zap.String("keyword", musicKeyword))
		if musicKeyword != "" {
			return r.tryL2SearchCore(ctx, orphan, stats, musicKeyword, "OpenCD")
		}
	}

	searchKeyword := reseed.ExtractSearchKeyword(orphan.Name)
	if searchKeyword == "" {
		searchKeyword = orphan.Name
	}
	groupName := reseed.ExtractGroupName(orphan.Name)

	if reseed.KeywordHasNoTitle(searchKeyword) {
		r.logger.Info("orphan L2: skipped (keyword has no title)",
			zap.String("orphan", orphan.Name),
			zap.String("keyword", searchKeyword),
			zap.String("group", groupName))
		return "", "", ""
	}

	return r.tryL2SearchCore(ctx, orphan, stats, searchKeyword, groupName)
}

func (r *Recovery) tryL2SearchCore(ctx context.Context, orphan *Entry, stats *SearchStats, searchKeyword, groupName string) (siteName, torrentID, method string) {
	if r.siteProvider == nil {
		return "", "", ""
	}

	sites := r.getSitePriority(ctx, groupName, orphan.Size)
	if len(sites) == 0 {
		return "", "", ""
	}
	stats.TotalSites = len(sites)

	// Phase 1: 源站优先——getSitePriority 已把 release_group_mappings(is_official) 的站排在 sites[0]
	phase1Searched := ""
	if groupName != "" && len(sites) > 1 {
		sourceSite := sites[0]
		phase1Searched = sourceSite
		r.logger.Info("orphan L2: searching source site first",
			zap.String("orphan", orphan.Name),
			zap.String("keyword", searchKeyword),
			zap.String("group", groupName),
			zap.String("source_site", sourceSite))

		searchCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		config, err := r.siteProvider.GetSiteConfig(searchCtx, sourceSite)
		if err == nil && config != nil {
			adapter, err := r.siteProvider.GetAdapter(searchCtx, sourceSite)
			if err == nil && adapter != nil {
				results, searchErr := adapter.SearchTorrents(searchCtx, config, searchKeyword, nil)
				if searchErr != nil {
					r.logger.Debug("orphan L2 priority: search error",
						zap.String("site", sourceSite), zap.Error(searchErr))
					stats.FailedSites = append(stats.FailedSites, SiteFailure{Site: sourceSite, Reason: searchErr.Error()})
				} else {
					stats.Searched++
					r.logger.Info("orphan L2 priority: search results",
						zap.String("site", sourceSite),
						zap.Int("result_count", len(results)),
						zap.Int64("orphan_size", orphan.Size))

					match, filterStats := reseed.VerifyMatchWithTruncationCheckAndSource(results, groupName, orphan.Size, orphan.Name)

					if match == nil {
						firstTitle := ""
						if len(results) > 0 {
							t := results[0].Title
							if len(t) > 80 { t = t[:80] }
							firstTitle = t
						}
						r.logger.Debug("orphan L2 priority: verify breakdown",
							zap.String("site", sourceSite),
							zap.Int("results", len(results)),
							zap.Int("empty_id", filterStats.EmptyID),
							zap.Int("group_miss", filterStats.GroupMiss),
							zap.Int("size_miss", filterStats.SizeMiss),
							zap.String("first_title", firstTitle),
							zap.String("expected_group", groupName))
					}

					if match != nil {
						cancel()
						r.logger.Info("orphan L2 match (priority)",
							zap.String("orphan", orphan.Name),
							zap.String("site", sourceSite),
							zap.String("torrent_id", match.TorrentID),
							zap.String("matched_title", match.Title))
						return sourceSite, match.TorrentID, "l2:priority:" + sourceSite
					}
					r.logger.Debug("orphan L2 priority: no match",
						zap.String("site", sourceSite))
				}
			}
		}
		cancel()
		r.logger.Info("orphan L2: source site miss, searching all sites",
			zap.String("source_site", sourceSite),
			zap.Int("remaining_sites", len(sites)-1))
	}

	// Phase 2: 并发搜索全部站
	r.logger.Info("orphan L2: searching all sites concurrently",
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
		if site == phase1Searched {
			continue
		}
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
				r.logger.Debug("orphan L2: site config failed",
					zap.String("site", site), zap.Error(err))
				statsMu.Lock()
				stats.Skipped++
				statsMu.Unlock()
				return
			}

			adapter, err := r.siteProvider.GetAdapter(searchCtx, site)
			if err != nil || adapter == nil {
				r.logger.Debug("orphan L2: adapter failed",
					zap.String("site", site), zap.Error(err))
				statsMu.Lock()
				stats.Skipped++
				statsMu.Unlock()
				return
			}

			siteCtx, siteCancel := context.WithTimeout(searchCtx, 20*time.Second)
			results2, err := adapter.SearchTorrents(siteCtx, config, searchKeyword, nil)
			siteCancel()

			if err != nil {
				r.logger.Debug("orphan L2: search error",
					zap.String("site", site),
					zap.String("keyword", searchKeyword),
					zap.Error(err))
				statsMu.Lock()
				stats.FailedSites = append(stats.FailedSites, SiteFailure{
					Site:   site,
					Reason: err.Error(),
				})
				statsMu.Unlock()
				return
			}

			statsMu.Lock()
			stats.Searched++
			statsMu.Unlock()

			if len(results2) == 0 {
				return
			}

			match, filterStats := reseed.VerifyMatchWithTruncationCheck(results2, groupName, orphan.Size)
			if match == nil {
				firstTitle := ""
				if len(results2) > 0 {
					t := results2[0].Title
					if len(t) > 80 { t = t[:80] }
					firstTitle = t
				}
				r.logger.Debug("orphan L2: verify breakdown",
					zap.String("site", site),
					zap.Int("results", len(results2)),
					zap.Int("empty_id", filterStats.EmptyID),
					zap.Int("group_miss", filterStats.GroupMiss),
					zap.Int("size_miss", filterStats.SizeMiss),
					zap.String("first_title", firstTitle),
					zap.String("expected_group", groupName))
				return
			}

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
		}(site)
	}

	allDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(allDone)
	}()

	select {
	case result := <-resultCh:
		cancel()
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
	siteContentTypeMap := make(map[string]string, len(sites))
	for _, s := range sites {
		if s.Enabled {
			enabledSites = append(enabledSites, s.Name)
			siteContentTypeMap[s.Name] = s.ContentType
		}
	}

	// 音乐资源（groupName="OpenCD"）：排除纯视频站
	isMusicSearch := groupName == "OpenCD"
	if isMusicSearch {
		filtered := make([]string, 0, len(enabledSites))
		for _, name := range enabledSites {
			if !util.ContentTypeCompatible("music", siteContentTypeMap[name]) {
				continue
			}
			filtered = append(filtered, name)
		}
		enabledSites = filtered
	}

	if groupName == "" {
		return enabledSites
	}

	priority := make([]string, 0, len(enabledSites))
	seen := make(map[string]bool)

	// 官组映射优先（OpenCD→皇后），确保主音乐站排第一
	if r.db != nil {
		var sourceSites []string
		r.db.WithContext(ctx).Model(&model.ReleaseGroupMapping{}).
			Where("LOWER(group_name) = LOWER(?) AND site_name IN ?", groupName, enabledSites).
			Order("is_official DESC").
			Pluck("site_name", &sourceSites)
		for _, site := range sourceSites {
			if !seen[site] {
				priority = append(priority, site)
				seen[site] = true
			}
		}
	}

	// 音乐搜索：其他纯音乐站次优先（海豚等）
	if isMusicSearch {
		for _, name := range enabledSites {
			if siteContentTypeMap[name] == "music" && !seen[name] {
				priority = append(priority, name)
				seen[name] = true
			}
		}
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

func (r *Recovery) getCategoryAndTags(siteName string) (string, []string) {
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
	return category, tags
}

func (r *Recovery) addTorrentWithRecheck(ctx context.Context, orphan *Entry, clientID string, torrentData []byte, savePath, category string, tags []string) error {
	client, err := r.clientProvider.Get(clientID)
	if err != nil {
		return fmt.Errorf("get downloader client: %w", err)
	}

	addResult, err := client.AddFromFile(ctx, torrentData, model.AddTorrentOptions{
		SavePath: savePath,
		Category: category,
		Tags:     tags,
		Paused:   true,
	})
	if err != nil {
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "already") || strings.Contains(lower, "exist") || strings.Contains(lower, "duplicate") {
			return nil
		}
		return fmt.Errorf("add to downloader: %w", err)
	}

	infoHash := addResult.InfoHash
	if infoHash != "" {
		if recheckErr := waitForRecheck(ctx, client, infoHash, 120*time.Second); recheckErr != nil {
			r.logger.Warn("orphan recheck incomplete",
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
	return nil
}

func (r *Recovery) downloadAndAdd(ctx context.Context, orphan *Entry, siteName, torrentID string, savePathOverride string, targetClientID string) error {
	config, err := r.siteProvider.GetSiteConfig(ctx, siteName)
	if err != nil || config == nil {
		return fmt.Errorf("get site config: %w", err)
	}
	adapter, err := r.siteProvider.GetAdapter(ctx, siteName)
	if err != nil || adapter == nil {
		return fmt.Errorf("get adapter: %w", err)
	}

	dlCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	torrentData, err := adapter.DownloadTorrent(dlCtx, config, torrentID)
	if err != nil {
		return fmt.Errorf("download torrent: %w", err)
	}
	if len(torrentData) == 0 {
		return fmt.Errorf("downloaded torrent data is empty")
	}

	if err := reseed.ValidateInjection(torrentData, orphan.Size, orphan.Name, 0, 1.0); err != nil {
		return fmt.Errorf("注入校验失败: %w", err)
	}

	clientID := targetClientID
	if clientID == "" && len(orphan.ClientIDs) > 0 {
		clientID = orphan.ClientIDs[0]
	}

	savePath := savePathOverride
	if savePath == "" {
		savePath = orphan.SavePath
		if !orphan.IsDir {
			savePath = filepath.Dir(orphan.Path)
		}
	}

	category, tags := r.getCategoryAndTags(siteName)

	if err := r.addTorrentWithRecheck(ctx, orphan, clientID, torrentData, savePath, category, tags); err != nil {
		return err
	}

	r.logger.Info("orphan recovered",
		zap.String("orphan", orphan.Name),
		zap.String("site", siteName),
		zap.String("save_path", savePath))

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
