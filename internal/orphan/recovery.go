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

	// 分类孤儿类型，按 Form 选择搜索策略
	var classification *util.DirClassification
	if siteName == "" && orphan.IsDir {
		classification, _ = util.ClassifyFromDir(orphan.Path, orphan.Name)
		if classification != nil {
			r.logger.Info("orphan classified",
				zap.String("orphan", orphan.Name),
				zap.String("category", classification.Type.Category),
				zap.String("form", classification.Type.Form),
				zap.Int("video_files", len(classification.VideoFiles)),
				zap.Int64("total_size", classification.TotalSize))
		}
	}

	if siteName == "" {
		form := ""
		if classification != nil {
			form = classification.Type.Form
		}
		hasVideoFiles := classification != nil && len(classification.VideoFiles) > 0

		switch {
		case form == "single_episode" || form == "partial_pack":
			// 单集/部分合集：直接走文件级搜索（用单集大小匹配）
			siteName, torrentID, method = r.tryFileLevelL2Search(ctx, orphan, stats)
		case form == "unknown" && hasVideoFiles:
			// S01 无 Complete 无 Exx + 有视频文件：直接走文件级（用文件名 SxxExx 搜索）
			siteName, torrentID, method = r.tryFileLevelL2Search(ctx, orphan, stats)
		default:
			// 全集/电影/音乐：走目录级搜索
			siteName, torrentID, method = r.tryL2Search(ctx, orphan, stats)
		}
	}

	result.SearchStats = stats

	if siteName != "" {
		result.Found = true
		result.Method = method
		result.SiteName = siteName

		// 确定注入校验用的 sourceSize/sourceName
		// 文件级搜索（单集/部分合集/unknown+视频）用最大视频文件大小
		// 目录级搜索（全集/电影）用目录总大小
		injectSize := orphan.Size
		injectName := orphan.Name
		if classification != nil && len(classification.VideoFiles) > 0 {
			form := classification.Type.Form
			if form == "single_episode" || form == "partial_pack" || form == "unknown" {
				injectSize = classification.VideoFiles[0].Size
				injectName = classification.VideoFiles[0].Name
			}
		}

		if err := r.downloadWithFallback(ctx, orphan, siteName, torrentID, targetClientID, injectSize, injectName); err != nil {
			// §59.185 ①: 失败必落日志——此前仅写 result.Message 返回，全日志零痕迹（城市
			// cuhash 案实证排查黑洞）
			r.logger.Error("orphan recovery failed",
				zap.String("orphan", orphan.Name),
				zap.String("site", siteName),
				zap.String("torrent_id", torrentID),
				zap.Error(err))
			result.Found = false
			result.Message = fmt.Sprintf("recovery failed: %v", err)
			return result
		}
		result.RecoveredCount = 1

		// 同站扩展：在命中的站点搜索其他集
		if classification != nil && len(classification.VideoFiles) > 1 {
			additional := r.expandSameSite(ctx, orphan, classification, siteName, targetClientID)
			result.RecoveredCount += additional
		}

		if result.RecoveredCount > 1 {
			result.Message = fmt.Sprintf("recovered %d episodes from %s (method=%s, includes same-site expansion)",
				result.RecoveredCount, siteName, method)
		} else {
			result.Message = fmt.Sprintf("recovered from %s (method=%s)", siteName, method)
		}
		return result
	}

	result.Message = fmt.Sprintf("no matching torrent found on any site (searched: %d, skipped: %d, failed: %d)",
		stats.Searched, stats.Skipped, len(stats.FailedSites))
	return result
}

// expandSameSite 在主恢复命中的站点上，搜索并恢复其他视频文件（同站扩展）。
// classification.VideoFiles[0] 已被主恢复恢复，从 [1] 开始。
func (r *Recovery) expandSameSite(ctx context.Context, orphan *Entry, classification *util.DirClassification, siteName, targetClientID string) int {
	if len(classification.VideoFiles) <= 1 {
		return 0
	}

	largestBase := strings.TrimSuffix(classification.VideoFiles[0].Name, filepath.Ext(classification.VideoFiles[0].Name))
	groupName := reseed.ExtractGroupName(largestBase)
	dirKeyword := reseed.ExtractSearchKeyword(orphan.Name)

	recovered := 0
	for i := 1; i < len(classification.VideoFiles); i++ {
		if ctx.Err() != nil {
			break
		}

		vf := classification.VideoFiles[i]
		fileBase := strings.TrimSuffix(vf.Name, filepath.Ext(vf.Name))
		fileKeyword := reseed.ExtractSearchKeyword(fileBase)

		if fileKeyword == "" || fileKeyword == dirKeyword {
			continue
		}

		tid := r.searchSingleSite(ctx, siteName, fileKeyword, groupName, vf.Size, fileBase)
		if tid == "" {
			continue
		}

		err := r.downloadAndAdd(ctx, orphan, siteName, tid, "", targetClientID, vf.Size, fileBase)
		if err != nil {
			r.logger.Warn("orphan same-site expansion failed",
				zap.String("file", vf.Name),
				zap.String("site", siteName),
				zap.Error(err))
			continue
		}

		recovered++
		r.logger.Info("orphan same-site expansion recovered",
			zap.String("file", vf.Name),
			zap.String("site", siteName),
			zap.String("torrent_id", tid))
	}

	return recovered
}

// searchSingleSite 在单个站点搜索并验证匹配。
// isRateLimitErr §59.185 A: 我方域限流器排队超时/冻结连坐类错误
// （"domain rate limit acquire failed"——请求未出进程，重试零站方压力）。
func isRateLimitErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "domain rate limit")
}

// searchWithBackoff §59.185 A: priority 搜索遇限流类错误退避重试一次
// （错误≠未命中——keepfrds 429 冻结 30s 连坐与批量窗口打满两形态实证；
// 20s 超时 + 10s 退避 + 20s 重试 = 50s 窗口覆盖 30s 冻结期）。
func (r *Recovery) searchWithBackoff(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, keyword string) ([]*model.SeedingSearchResult, error) {
	var results []*model.SeedingSearchResult
	var err error
	for attempt := 0; ; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		results, err = adapter.SearchTorrents(attemptCtx, config, keyword, nil)
		cancel()
		if err == nil || !isRateLimitErr(err) || attempt >= 1 || ctx.Err() != nil {
			return results, err
		}
		r.logger.Warn("orphan L2 priority: rate limited, backing off",
			zap.String("keyword", keyword), zap.Error(err))
		select {
		case <-time.After(10 * time.Second):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (r *Recovery) searchSingleSite(ctx context.Context, siteName, keyword, groupName string, sourceSize int64, sourceTitle string) string {
	config, err := r.siteProvider.GetSiteConfig(ctx, siteName)
	if err != nil || config == nil {
		return ""
	}
	adapter, err := r.siteProvider.GetAdapter(ctx, siteName)
	if err != nil || adapter == nil {
		return ""
	}

	searchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	results, err := adapter.SearchTorrents(searchCtx, config, keyword, nil)
	if err != nil {
		return ""
	}

	match, _ := reseed.VerifyMatchWithTruncationCheckAndSource(results, groupName, sourceSize, sourceTitle)
	if match == nil {
		return ""
	}
	return match.TorrentID
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

	return r.tryL2SearchCore(ctx, orphan, stats, fileKeyword, fileGroup, largestSize, baseName, nil)
}

func (r *Recovery) tryL2Search(ctx context.Context, orphan *Entry, stats *SearchStats) (siteName, torrentID, method string) {
	if orphan.IsDir && reseed.DetectMusicFromDir(orphan.Path) {
		musicKeyword := reseed.ExtractMusicKeyword(orphan.Name)
		r.logger.Info("orphan L2: music detected",
			zap.String("orphan", orphan.Name),
			zap.String("keyword", musicKeyword))
		if musicKeyword != "" {
			return r.tryL2SearchCore(ctx, orphan, stats, musicKeyword, "OpenCD", orphan.Size, orphan.Name, nil)
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

	return r.tryL2SearchCore(ctx, orphan, stats, searchKeyword, groupName, orphan.Size, orphan.Name, nil)
}

func (r *Recovery) tryL2SearchCore(ctx context.Context, orphan *Entry, stats *SearchStats, searchKeyword, groupName string, sourceSize int64, sourceTitle string, excludeSites map[string]bool) (siteName, torrentID, method string) {
	if r.siteProvider == nil {
		return "", "", ""
	}

	if sourceSize <= 0 {
		sourceSize = orphan.Size
	}
	if sourceTitle == "" {
		sourceTitle = orphan.Name
	}

	sites := r.getSitePriority(ctx, groupName, sourceSize)
	if len(sites) == 0 {
		return "", "", ""
	}
	stats.TotalSites = len(sites)

	// Phase 1: 源站优先——getSitePriority 已把 release_group_mappings(is_official) 的站排在 sites[0]
	// §59.185 A: 搜索走 searchWithBackoff（限流类错误退避重试——错误≠未命中）
	phase1Searched := ""
	if excludeSites == nil {
		excludeSites = map[string]bool{}
	}
	if groupName != "" && len(sites) > 1 && !excludeSites[sites[0]] {
		sourceSite := sites[0]
		phase1Searched = sourceSite
		r.logger.Info("orphan L2: searching source site first",
			zap.String("orphan", orphan.Name),
			zap.String("keyword", searchKeyword),
			zap.String("group", groupName),
			zap.String("source_site", sourceSite))

		cfgCtx, cfgCancel := context.WithTimeout(ctx, 10*time.Second)
		config, cfgErr := r.siteProvider.GetSiteConfig(cfgCtx, sourceSite)
		var adapter model.SiteAdapter
		if cfgErr == nil && config != nil {
			adapter, cfgErr = r.siteProvider.GetAdapter(cfgCtx, sourceSite)
		}
		cfgCancel()

		if cfgErr == nil && config != nil && adapter != nil {
			results, searchErr := r.searchWithBackoff(ctx, adapter, config, searchKeyword)
			if searchErr != nil {
				r.logger.Debug("orphan L2 priority: search error",
					zap.String("site", sourceSite), zap.Error(searchErr))
				stats.FailedSites = append(stats.FailedSites, SiteFailure{Site: sourceSite, Reason: searchErr.Error()})
			} else {
				stats.Searched++
				// §59.181 调试：打印每条结果的 size（定位 size_miss 根因）
				for _, rr := range results {
					r.logger.Debug("orphan L2 priority: result detail",
						zap.String("site", sourceSite),
						zap.String("tid", rr.TorrentID),
						zap.Int64("size", rr.Size),
						zap.String("title", rr.Title[:min(200, len(rr.Title))]),
						zap.Int64("orphan_size", orphan.Size))
				}
				r.logger.Info("orphan L2 priority: search results",
					zap.String("site", sourceSite),
					zap.Int("result_count", len(results)),
					zap.Int64("orphan_size", orphan.Size))

				match, filterStats := reseed.VerifyMatchWithTruncationCheckAndSource(results, groupName, sourceSize, sourceTitle)

				if match == nil {
					firstTitle := ""
					if len(results) > 0 {
						t := results[0].Title
						if len(t) > 80 {
							t = t[:80]
						}
						firstTitle = t
					}
					r.logger.Debug("orphan L2 priority: verify breakdown",
						zap.String("site", sourceSite),
						zap.Int("results", len(results)),
						zap.Int("empty_id", filterStats.EmptyID),
						zap.Int("group_refute", filterStats.GroupRefute),
						zap.Int("group_neutral", filterStats.GroupNeutral),
						zap.Int("sequel_refute", filterStats.SequelRefute),
						zap.Int("title_neutral", filterStats.TitleNeutral),
						zap.Int("size_invalid", filterStats.SizeInvalid),
						zap.Int("tech_refute", filterStats.TechRefute),
						zap.Int("version_refute", filterStats.VersionRefute),
						zap.Int("size_refute", filterStats.SizeRefute),
						zap.String("first_title", firstTitle),
						zap.String("expected_group", groupName))
				}

				if match != nil {
					r.logger.Info("orphan L2 match (priority)",
						zap.String("orphan", orphan.Name),
						zap.String("site", sourceSite),
						zap.String("torrent_id", match.TorrentID),
						zap.String("matched_title", match.Title))
					return sourceSite, match.TorrentID, "l2:priority:"+sourceSite
				}
				r.logger.Debug("orphan L2 priority: no match",
					zap.String("site", sourceSite))

				// §59.184 B 补线（priority 站）：B1 形态（主词无 CJK 且源标题有 CJK）
				// → 前导中文段补一轮 area=0（中文标题站索引中文名）
				if !reseed.HasCJKWord(searchKeyword) && reseed.HasCJKWord(sourceTitle) {
					if cnKW := reseed.LeadingCJKSegment(sourceTitle); cnKW != "" && cnKW != searchKeyword {
						r.logger.Debug("orphan L2 priority: chinese supplement search",
							zap.String("site", sourceSite),
							zap.String("keyword", cnKW))
						if retry, rErr := r.searchWithBackoff(ctx, adapter, config, cnKW); rErr == nil && len(retry) > 0 {
							if m2, _ := reseed.VerifyMatchWithTruncationCheckAndSource(retry, groupName, sourceSize, sourceTitle); m2 != nil {
								r.logger.Info("orphan L2 match (priority, chinese line)",
									zap.String("orphan", orphan.Name),
									zap.String("site", sourceSite),
									zap.String("torrent_id", m2.TorrentID))
								return sourceSite, m2.TorrentID, "l2:priority-cn:"+sourceSite
							}
						}
					}
				}
			}
		}
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
		if site == phase1Searched || excludeSites[site] {
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

			match, filterStats := reseed.VerifyMatchWithTruncationCheckAndSource(results2, groupName, sourceSize, sourceTitle)
			if match == nil {
				// §59.184 B 补线（全站轮）：B1 形态 → 前导中文段补一轮 area=0
				if !reseed.HasCJKWord(searchKeyword) && reseed.HasCJKWord(sourceTitle) {
					if cnKW := reseed.LeadingCJKSegment(sourceTitle); cnKW != "" && cnKW != searchKeyword {
						cnCtx, cnCancel := context.WithTimeout(searchCtx, 20*time.Second)
						if retry, rErr := adapter.SearchTorrents(cnCtx, config, cnKW, nil); rErr == nil && len(retry) > 0 {
							match, _ = reseed.VerifyMatchWithTruncationCheckAndSource(retry, groupName, sourceSize, sourceTitle)
						}
						cnCancel()
						if match != nil {
							r.logger.Info("orphan L2 match (chinese line)",
								zap.String("orphan", orphan.Name),
								zap.String("site", site),
								zap.String("torrent_id", match.TorrentID),
								zap.String("matched_title", match.Title),
								zap.Int64("orphan_size", orphan.Size),
								zap.Int64("matched_size", match.Size))
							select {
							case resultCh <- matchResult{site, match.TorrentID, "l2:search-cn:" + site}:
							case <-searchCtx.Done():
							}
							return
						}
					}
				}

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
					zap.Int("group_refute", filterStats.GroupRefute),
					zap.Int("group_neutral", filterStats.GroupNeutral),
					zap.Int("sequel_refute", filterStats.SequelRefute),
					zap.Int("title_neutral", filterStats.TitleNeutral),
					zap.Int("size_invalid", filterStats.SizeInvalid),
					zap.Int("tech_refute", filterStats.TechRefute),
					zap.Int("version_refute", filterStats.VersionRefute),
					zap.Int("size_refute", filterStats.SizeRefute),
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

// downloadWithFallback §59.185 ③: 下载失败三层兜底。
// 层1 胜者站下载（失败即落日志——① 黑洞修复）
// 层2 cuhash 模板站懒刷新 + 原站重试一次（cuhash 轮换窗口未知，6h 定时同步有缺口；
//     刷新即变化则回写 DB 后重试——保源站偏好，keepfrds 429 类失败不直接跳站）
// 层3 次优站递补：排除已败站重搜（优先站/中文线链路复用），最多递补 2 站
func (r *Recovery) downloadWithFallback(ctx context.Context, orphan *Entry, primarySite, primaryTid, targetClientID string, sourceSize int64, sourceName string) error {
	excluded := map[string]bool{}
	var lastErr error

	for attempt := 0; attempt < 3; attempt++ {
		site, tid := primarySite, primaryTid
		if attempt > 0 {
			// 层3: 次优站递补——排除已败站重搜取新胜者
			s2, t2 := r.retrySearchExcluding(ctx, orphan, excluded)
			if s2 == "" {
				break
			}
			site, tid = s2, t2
			r.logger.Info("orphan recovery: falling back to next site",
				zap.String("orphan", orphan.Name),
				zap.String("from_site", primarySite),
				zap.String("to_site", site),
				zap.String("torrent_id", tid))
		}
		excluded[site] = true

		err := r.downloadAndAdd(ctx, orphan, site, tid, "", targetClientID, sourceSize, sourceName)
		if err == nil {
			return nil
		}
		lastErr = fmt.Errorf("site=%s tid=%s: %w", site, tid, err)
		r.logger.Warn("orphan recovery: download attempt failed",
			zap.String("orphan", orphan.Name),
			zap.String("site", site),
			zap.String("torrent_id", tid),
			zap.Error(err))

		// 层2: cuhash 懒刷新 + 原站重试
		if r.tryCuhashRefresh(ctx, site) {
			if err2 := r.downloadAndAdd(ctx, orphan, site, tid, "", targetClientID, sourceSize, sourceName); err2 == nil {
				r.logger.Info("orphan recovery: succeeded after cuhash refresh",
					zap.String("orphan", orphan.Name),
					zap.String("site", site))
				return nil
			} else {
				r.logger.Warn("orphan recovery: retry after cuhash refresh still failed",
					zap.String("orphan", orphan.Name),
					zap.String("site", site),
					zap.Error(err2))
				lastErr = fmt.Errorf("site=%s tid=%s (cuhash refreshed): %w", site, tid, err2)
			}
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no downloadable candidate site")
	}
	return lastErr
}

// retrySearchExcluding §59.185 ③: 次优站递补搜索（排除已败站，2 分钟窗口）
func (r *Recovery) retrySearchExcluding(ctx context.Context, orphan *Entry, exclude map[string]bool) (string, string) {
	searchKeyword := reseed.ExtractSearchKeyword(orphan.Name)
	if searchKeyword == "" {
		searchKeyword = orphan.Name
	}
	groupName := reseed.ExtractGroupName(orphan.Name)
	retryCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	site, tid, _ := r.tryL2SearchCore(retryCtx, orphan, nil, searchKeyword, groupName, orphan.Size, orphan.Name, exclude)
	return site, tid
}

// tryCuhashRefresh §59.185 ②: cuhash 模板站懒刷新——下载失败立即抓首页 cuhash，
// 与当前凭证比对，变化则回写 sites.passkey（GetSiteConfig 无缓存直读 DB，即时生效）。
// 返回是否发生了有效刷新（非 cuhash 站/无变化/能力缺失均 false）。
func (r *Recovery) tryCuhashRefresh(ctx context.Context, siteName string) bool {
	config, err := r.siteProvider.GetSiteConfig(ctx, siteName)
	if err != nil || config == nil {
		return false
	}
	if !strings.Contains(config.DownloadURLTemplate, "cuhash=") && config.DownloadMode != "cuhash" {
		return false
	}
	adapter, err := r.siteProvider.GetAdapter(ctx, siteName)
	if err != nil || adapter == nil {
		return false
	}
	cr, ok := adapter.(model.CuhashScraper)
	if !ok {
		return false
	}
	refreshCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	fresh := cr.ScrapeCuhash(refreshCtx, config)
	cancel()
	if fresh == "" || fresh == config.Passkey {
		return false
	}
	if r.db == nil {
		return false
	}
	if err := r.db.WithContext(ctx).Model(&model.Site{}).
		Where("domain = ?", config.Domain).
		Update("passkey", fresh).Error; err != nil {
		r.logger.Warn("cuhash refresh: persist failed", zap.String("site", siteName), zap.Error(err))
		return false
	}
	r.logger.Info("cuhash refreshed on download failure",
		zap.String("site", siteName))
	return true
}

func (r *Recovery) downloadAndAdd(ctx context.Context, orphan *Entry, siteName, torrentID string, savePathOverride string, targetClientID string, sourceSize int64, sourceName string) error {
	if sourceSize <= 0 {
		sourceSize = orphan.Size
	}
	if sourceName == "" {
		sourceName = orphan.Name
	}
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

	if err := reseed.ValidateInjection(torrentData, sourceSize, sourceName, 0, 1.0); err != nil {
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
