package orphan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/fingerprint"
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

func (r *Recovery) Recover(ctx context.Context, orphan *Entry, targetClientID string) *RecoverResult {
	result := &RecoverResult{Orphan: orphan}
	stats := &SearchStats{}

	siteName, torrentID, method := r.tryDBMatch(ctx, orphan)
	if siteName == "" {
		siteName, torrentID, method = r.tryL2Search(ctx, orphan, stats)
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

	if orphan.IsDir {
		fileResults := r.tryFileLevelRecover(ctx, orphan)
		if len(fileResults) > 0 {
			recovered := 0
			for _, fr := range fileResults {
				if fr.Found {
					recovered++
				}
			}
			result.FileResults = fileResults
			if recovered > 0 {
				result.Found = true
				result.Message = fmt.Sprintf("%d/%d files recovered", recovered, len(fileResults))
				return result
			}
			result.Message = fmt.Sprintf("0/%d files recovered (file-level search completed, no matches)", len(fileResults))
			return result
		}
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

var skipSubDirs = map[string]bool{
	"BDMV": true, "SAMPLE": true, "Sample": true, "Subs": true, "Proof": true,
}

const maxFileLevelSearch = 20

func (r *Recovery) tryFileLevelRecover(ctx context.Context, orphan *Entry) []FileRecoverResult {
	entries, err := os.ReadDir(orphan.Path)
	if err != nil {
		return nil
	}

	for _, e := range entries {
		if e.IsDir() && skipSubDirs[e.Name()] {
			return nil
		}
	}

	diskFiles := make(map[string]int64)
	var videoFileNames []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if searchableVideoExts[ext] {
			info, err := e.Info()
			if err != nil {
				continue
			}
			diskFiles[e.Name()] = info.Size()
			videoFileNames = append(videoFileNames, e.Name())
		}
	}

	if len(diskFiles) == 0 || len(diskFiles) > maxFileLevelSearch {
		return nil
	}

	results := make([]FileRecoverResult, 0, len(videoFileNames))
	covered := make(map[string]bool)

	// §56.40: 优先从种子名提取关键词/制作组；提取不到时从目录内最大文件名提取
	// 老种子的种子名不标准（纯中文/特殊分隔符），但文件名遵循 PT 命名规则
	dirKeyword := reseed.ExtractSearchKeyword(orphan.Name)
	dirGroup := reseed.ExtractGroupName(orphan.Name)
	if dirKeyword == "" || dirGroup == "" {
		var largestFile string
		var largestSize int64
		for fname, fsize := range diskFiles {
			if fsize > largestSize {
				largestSize = fsize
				largestFile = fname
			}
		}
		if largestFile != "" {
			baseName := strings.TrimSuffix(largestFile, filepath.Ext(largestFile))
			if dirKeyword == "" {
				dirKeyword = reseed.ExtractSearchKeyword(baseName)
			}
			if dirGroup == "" {
				dirGroup = reseed.ExtractGroupName(baseName)
			}
			r.logger.Info("file-level recover: keyword from largest file",
				zap.String("orphan", orphan.Name),
				zap.String("file", largestFile),
				zap.String("keyword", dirKeyword),
				zap.String("group", dirGroup))
		}
	}
	r.logger.Info("file-level recover: directory search",
		zap.String("orphan", orphan.Name),
		zap.String("keyword", dirKeyword),
		zap.String("group", dirGroup),
		zap.Int("disk_files", len(diskFiles)))

	sites := r.getSitePriority(ctx, dirGroup, orphan.Size)
	searchCtx, searchCancel := context.WithTimeout(ctx, 3*time.Minute)
	defer searchCancel()

	for _, site := range sites {
		if searchCtx.Err() != nil {
			r.logger.Info("file-level recover: search context expired",
				zap.String("orphan", orphan.Name),
				zap.String("last_site", site))
			break
		}
		if len(covered) == len(diskFiles) {
			break
		}

		config, err := r.siteProvider.GetSiteConfig(searchCtx, site)
		if err != nil || config == nil {
			r.logger.Debug("file-level recover: site config failed",
				zap.String("site", site), zap.Error(err))
			continue
		}
		if config.BaseURL != "" {
			httpclient.ResetDomainCircuit(config.BaseURL)
			httpclient.GlobalLimiter.ManualUnfreeze(config.BaseURL)
		}
		adapter, err := r.siteProvider.GetAdapter(searchCtx, site)
		if err != nil || adapter == nil {
			r.logger.Debug("file-level recover: adapter failed",
				zap.String("site", site), zap.Error(err))
			continue
		}

		siteCtx, siteCancel := context.WithTimeout(searchCtx, 20*time.Second)
		results2, err := adapter.SearchTorrents(siteCtx, config, dirKeyword, nil)
		siteCancel()
		if err != nil {
			r.logger.Info("file-level recover: search error",
				zap.String("site", site),
				zap.String("keyword", dirKeyword),
				zap.Error(err))
			continue
		}
		if len(results2) == 0 {
			r.logger.Debug("file-level recover: no results",
				zap.String("site", site),
				zap.String("keyword", dirKeyword))
			continue
		}

		r.logger.Info("file-level recover: got results",
			zap.String("site", site),
			zap.Int("count", len(results2)),
			zap.String("first_title", results2[0].Title[:min(80, len(results2[0].Title))]))

		for _, res := range results2 {
			if res.TorrentID == "" {
				continue
			}
			if dirGroup != "" && !strings.Contains(res.Title, dirGroup) {
				r.logger.Debug("file-level recover: title group mismatch",
					zap.String("site", site),
					zap.String("title", res.Title[:min(80, len(res.Title))]),
					zap.String("expected_group", dirGroup))
				continue
			}

			r.logger.Info("file-level recover: downloading torrent",
				zap.String("site", site),
				zap.String("torrent_id", res.TorrentID),
				zap.String("title", res.Title[:min(80, len(res.Title))]))

			dlCtx, dlCancel := context.WithTimeout(searchCtx, 30*time.Second)
			torrentData, dlErr := adapter.DownloadTorrent(dlCtx, config, res.TorrentID)
			dlCancel()
			if dlErr != nil || len(torrentData) == 0 {
				r.logger.Info("file-level recover: download failed",
					zap.String("site", site),
					zap.String("torrent_id", res.TorrentID),
					zap.Error(dlErr))
				continue
			}

			meta, metaErr := fingerprint.ComputeFromTorrent(torrentData)
			if metaErr != nil || meta == nil {
				r.logger.Info("file-level recover: fingerprint failed",
					zap.String("site", site),
					zap.String("torrent_id", res.TorrentID),
					zap.Error(metaErr))
				continue
			}

			// 检查磁盘上的视频文件是否都能被种子覆盖
			// （种子可能含额外文件如 .jpg 封面、.nfo 等，不要求它们在磁盘上存在）
			torrentFileSet := make(map[string]bool)
			for torrentFile := range meta.FileTree {
				torrentFileSet[filepath.Base(torrentFile)] = true
			}

			var matchedFiles []string
			allDiskCovered := true
			for diskFile := range diskFiles {
				if covered[diskFile] {
					continue
				}
				if torrentFileSet[diskFile] {
					matchedFiles = append(matchedFiles, diskFile)
				} else {
					allDiskCovered = false
					r.logger.Info("file-level recover: disk file not in torrent",
						zap.String("site", site),
						zap.String("disk_file", diskFile),
						zap.Int("torrent_file_count", len(torrentFileSet)))
				}
			}

			if !allDiskCovered || len(matchedFiles) == 0 {
				continue
			}

			r.logger.Info("file-level match found",
				zap.String("orphan", orphan.Name),
				zap.String("site", site),
				zap.String("torrent_id", res.TorrentID),
				zap.Strings("matched_files", matchedFiles))

			parentSavePath := filepath.Dir(orphan.Path)
			clientID := ""
			if len(orphan.ClientIDs) > 0 {
				clientID = orphan.ClientIDs[0]
			}
			addEntry := &Entry{
				Path:      orphan.Path,
				Name:      orphan.Name,
				ClientIDs: orphan.ClientIDs,
				SavePath:  parentSavePath,
			}

			category, tags := r.getCategoryAndTags(site)
			addErr := r.addTorrentWithRecheck(ctx, addEntry, clientID, torrentData, parentSavePath, category, tags)
			if addErr != nil {
				r.logger.Warn("file-level add failed",
					zap.String("torrent_id", res.TorrentID),
					zap.Error(addErr))
				continue
			}

			for _, mf := range matchedFiles {
				covered[mf] = true
				results = append(results, FileRecoverResult{
					FileName: mf,
					Found:    true,
					SiteName: site,
					Message:  fmt.Sprintf("file-level:%s", res.TorrentID),
				})
			}
		}
	}

	for _, vf := range videoFileNames {
		if !covered[vf] {
			results = append(results, FileRecoverResult{
				FileName: vf,
				Found:    false,
				Message:  "not found",
			})
		}
	}

	return results
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
			r.logger.Debug("orphan L2: site config failed",
				zap.String("site", site), zap.Error(err))
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
			r.logger.Debug("orphan L2: adapter failed",
				zap.String("site", site), zap.Error(err))
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
			r.logger.Debug("orphan L2: search error",
				zap.String("site", site),
				zap.String("keyword", searchKeyword),
				zap.Error(err))
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
		} else {
			r.logger.Debug("orphan L2: no match",
				zap.String("site", site),
				zap.String("keyword", searchKeyword),
				zap.String("group", groupName))
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

	priority := make([]string, 0, len(enabledSites))
	seen := make(map[string]bool)

	if r.db != nil {
		var sourceSites []string
		r.db.WithContext(ctx).Model(&model.ReleaseGroupMapping{}).
			Where("group_name = ? AND site_name IN ?", groupName, enabledSites).
			Order("is_official DESC").
			Pluck("site_name", &sourceSites)
		for _, site := range sourceSites {
			if !seen[site] {
				priority = append(priority, site)
				seen[site] = true
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

func keysFromMap(m map[string]int64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
