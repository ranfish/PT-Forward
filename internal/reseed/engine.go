package reseed

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	dbimpl "github.com/ranfish/pt-forward/internal/db"
	"github.com/ranfish/pt-forward/internal/fingerprint"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type preloadedSites struct {
	infos    []*model.SiteInfo
	configs  map[string]*model.SiteConfig
	adapters map[string]model.SiteAdapter
}

// §33.32 — piecesHashSearcher is an optional capability interface.
// Adapters that support NexusPHP /api/pieces-hash implement this.
type piecesHashSearcher interface {
	SearchByPiecesHash(ctx context.Context, config *model.SiteConfig, piecesHashes []string) (map[string]int, error)
}

type fpCache struct {
	byKey map[string]*model.ContentFingerprint
}

func (c *fpCache) get(infoHash, siteName string) *model.ContentFingerprint {
	if c == nil {
		return nil
	}
	return c.byKey[infoHash+"|"+siteName]
}

type Engine struct {
	db             *gorm.DB
	logger         *zap.Logger
	siteProvider   model.SiteInfoProvider
	clientProvider model.DownloaderProvider
	iyuuService    model.IYUUService
	fpRepo         *fingerprint.Repository
	mu             sync.RWMutex
	tasks          map[uint]context.CancelFunc
}

func NewEngine(db *gorm.DB, logger *zap.Logger) *Engine {
	return &Engine{
		db:     db,
		logger: logger,
		tasks:  make(map[uint]context.CancelFunc),
	}
}

func (e *Engine) SetSiteProvider(sp model.SiteInfoProvider) {
	e.siteProvider = sp
}

func (e *Engine) SetFingerprintRepo(repo *fingerprint.Repository) {
	e.fpRepo = repo
}

func (e *Engine) SetClientProvider(cp model.DownloaderProvider) {
	e.clientProvider = cp
}

func (e *Engine) SetIYUUService(svc model.IYUUService) {
	e.iyuuService = svc
}

func (e *Engine) preloadSites(ctx context.Context, targetSites, excludedSites []string) *preloadedSites {
	if e.siteProvider == nil {
		return nil
	}

	exclSet := make(map[string]bool, len(excludedSites))
	for _, s := range excludedSites {
		exclSet[s] = true
	}

	var allSites []*model.SiteInfo
	if len(targetSites) > 0 {
		for _, siteName := range targetSites {
			info, err := e.siteProvider.GetSiteInfo(ctx, siteName)
			if err != nil {
				e.logger.Warn("获取目标站点信息失败", zap.String("site", siteName), zap.Error(err))
				continue
			}
			allSites = append(allSites, info)
		}
	} else {
		sites, err := e.siteProvider.ListSites(ctx)
		if err != nil {
			e.logger.Warn("列出站点失败", zap.Error(err))
			return nil
		}
		allSites = sites
	}

	var eligible []*model.SiteInfo
	configs := make(map[string]*model.SiteConfig)
	adapters := make(map[string]model.SiteAdapter)

	for _, info := range allSites {
		if exclSet[info.Name] || !info.Enabled {
			continue
		}

		config, err := e.siteProvider.GetSiteConfig(ctx, info.Name)
		if err != nil {
			e.logger.Warn("获取站点配置失败", zap.String("site", info.Name), zap.Error(err))
			continue
		}

		adapter, err := e.siteProvider.GetAdapter(ctx, info.Name)
		if err != nil {
			e.logger.Warn("获取适配器失败", zap.String("site", info.Name), zap.Error(err))
			continue
		}

		eligible = append(eligible, info)
		configs[info.Name] = config
		adapters[info.Name] = adapter
	}

	return &preloadedSites{
		infos:    eligible,
		configs:  configs,
		adapters: adapters,
	}
}

func (e *Engine) preloadFingerprints(ctx context.Context, records []model.SeedingTorrentRecord) *fpCache {
	if len(records) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var infoHashes []string
	for _, rec := range records {
		if !seen[rec.InfoHash] {
			seen[rec.InfoHash] = true
			infoHashes = append(infoHashes, rec.InfoHash)
		}
	}

	var fps []*model.ContentFingerprint
	if e.fpRepo != nil {
		batch, err := e.fpRepo.BatchGetByInfoHashes(ctx, infoHashes)
		if err != nil {
			e.logger.Warn("批量预加载指纹失败", zap.Error(err))
			return &fpCache{byKey: make(map[string]*model.ContentFingerprint)}
		}
		fps = batch
	} else {
		var batch []model.ContentFingerprint
		if err := e.db.WithContext(ctx).Where("info_hash IN ?", infoHashes).Find(&batch).Error; err != nil {
			e.logger.Warn("批量预加载指纹失败(DB)", zap.Error(err))
			return &fpCache{byKey: make(map[string]*model.ContentFingerprint)}
		}
		fps = make([]*model.ContentFingerprint, len(batch))
		for i := range batch {
			fps[i] = &batch[i]
		}
	}

	byKey := make(map[string]*model.ContentFingerprint, len(fps))
	for _, fp := range fps {
		byKey[fp.InfoHash+"|"+fp.SiteName] = fp
	}
	return &fpCache{byKey: byKey}
}

func (e *Engine) preloadExistingMatches(ctx context.Context, records []model.SeedingTorrentRecord) map[string][]model.ReseedMatch {
	if len(records) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var infoHashes []string
	for _, rec := range records {
		if !seen[rec.InfoHash] {
			seen[rec.InfoHash] = true
			infoHashes = append(infoHashes, rec.InfoHash)
		}
	}

	var matches []model.ReseedMatch
	if err := e.db.WithContext(ctx).
		Where("source_info_hash IN ? AND status IN ?", infoHashes, []model.ReseedMatchStatus{
			model.MatchStatusMatched,
			model.MatchStatusInjecting,
			model.MatchStatusInjected,
		}).
		Find(&matches).Error; err != nil {
		e.logger.Warn("批量预加载已有匹配失败", zap.Error(err))
		return make(map[string][]model.ReseedMatch)
	}

	result := make(map[string][]model.ReseedMatch, len(matches))
	for _, m := range matches {
		result[m.SourceInfoHash] = append(result[m.SourceInfoHash], m)
	}
	return result
}

func (e *Engine) preloadNegativeCache(ctx context.Context, records []model.SeedingTorrentRecord) map[string]map[string]bool {
	if len(records) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var infoHashes []string
	for _, rec := range records {
		if !seen[rec.InfoHash] {
			seen[rec.InfoHash] = true
			infoHashes = append(infoHashes, rec.InfoHash)
		}
	}

	entries, err := e.GetNegativeCacheByHashes(ctx, infoHashes)
	if err != nil {
		e.logger.Warn("预加载负面缓存失败", zap.Error(err))
		return make(map[string]map[string]bool)
	}

	result := make(map[string]map[string]bool)
	for _, entry := range entries {
		if result[entry.SourceInfoHash] == nil {
			result[entry.SourceInfoHash] = make(map[string]bool)
		}
		if entry.ExcludedTargets != "" {
			for _, t := range strings.Split(entry.ExcludedTargets, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					result[entry.SourceInfoHash][t] = true
				}
			}
		}
	}
	return result
}

func (e *Engine) preloadIYUUResults(ctx context.Context, records []model.SeedingTorrentRecord) map[string][]*model.IYUUReseedResult {
	if e.iyuuService == nil || len(records) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var infoHashes []string
	for _, rec := range records {
		if !seen[rec.InfoHash] {
			seen[rec.InfoHash] = true
			infoHashes = append(infoHashes, rec.InfoHash)
		}
	}

	results, err := e.iyuuService.QueryReseed(ctx, infoHashes)
	if err != nil {
		e.logger.Warn("IYUU 批量查询失败", zap.Error(err))
		return make(map[string][]*model.IYUUReseedResult)
	}

	byHash := make(map[string][]*model.IYUUReseedResult)
	for _, r := range results {
		byHash[r.SourceInfoHash] = append(byHash[r.SourceInfoHash], r)
	}
	return byHash
}

func (e *Engine) preloadIYUUSiteMappings(ctx context.Context) map[int]string {
	var mappings []model.IYUUSiteMapping
	if err := e.db.WithContext(ctx).Find(&mappings).Error; err != nil {
		return make(map[int]string)
	}
	result := make(map[int]string, len(mappings))
	for _, m := range mappings {
		siteName := m.SiteName
		if siteName != "" && e.siteProvider != nil {
			if info, err := e.siteProvider.GetSiteInfo(ctx, siteName); err == nil && info != nil {
				siteName = info.Name
			}
		}
		if siteName == "" && m.SiteDomain != "" && e.siteProvider != nil {
			if info, err := e.siteProvider.GetSiteInfoByURL(ctx, m.SiteDomain); err == nil && info != nil {
				siteName = info.Name
			}
		}
		if siteName != "" {
			result[m.IYUUSid] = siteName
		}
	}
	return result
}

func (e *Engine) Start(ctx context.Context) error {
	var tasks []model.ReseedTask
	if err := e.db.WithContext(ctx).Where("enabled = ?", true).Find(&tasks).Error; err != nil {
		return reseedError(ErrReseedDB, "load reseed tasks", err)
	}

	for i := range tasks {
		e.startTask(ctx, &tasks[i])
	}

	e.logger.Info("reseed engine started", zap.Int("tasks", len(tasks)))
	return nil
}

func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	stopped := len(e.tasks)
	for id, cancel := range e.tasks {
		cancel()
		delete(e.tasks, id)
	}
	e.logger.Info("reseed engine stopped", zap.Int("stopped_tasks", stopped))
}

func (e *Engine) startTask(parentCtx context.Context, task *model.ReseedTask) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if old, ok := e.tasks[task.ID]; ok {
		old()
	}
	ctx, cancel := context.WithCancel(parentCtx) //nolint:gosec // cancel stored in e.tasks for later invocation
	e.tasks[task.ID] = cancel
	if err := e.db.WithContext(ctx).Model(task).Updates(map[string]interface{}{
		"status":     model.ReseedTaskIdle,
		"updated_at": time.Now(),
	}).Error; err != nil {
		e.logger.Warn("update reseed task to idle failed",
			zap.Uint("taskID", task.ID),
			zap.Error(err))
	}
}

func (e *Engine) CancelTask(taskID uint) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if cancel, ok := e.tasks[taskID]; ok {
		cancel()
		delete(e.tasks, taskID)
	}
}

type MatchInput struct {
	SourceInfoHash  string
	SourceSize      int64
	SourceTitle     string
	SourceSite      string
	TargetSite      string
	TargetTorrentID string
	TargetInfoHash  string
	TargetSize      int64
}

func MatchDecision(input MatchInput, sizeTolerance float64) model.DecisionType {
	if input.SourceInfoHash == input.TargetInfoHash && input.SourceInfoHash != "" {
		return model.DecisionSameInfoHash
	}

	if input.SourceSize == 0 || input.TargetSize == 0 {
		return model.DecisionNoDownloadLink
	}

	sizeDiff := float64(input.SourceSize-input.TargetSize) / float64(input.TargetSize) * 100
	if sizeDiff < 0 {
		sizeDiff = -sizeDiff
	}

	if sizeDiff <= sizeTolerance {
		if input.SourceSize == input.TargetSize {
			return model.DecisionMatch
		}
		return model.DecisionMatchSizeOnly
	}

	if sizeDiff <= sizeTolerance*5 {
		return model.DecisionFuzzySizeMismatch
	}

	return model.DecisionSizeMismatch
}

func (e *Engine) RunTask(ctx context.Context, task *model.ReseedTask) (result *model.ReseedExecutionResult, retErr error) {
	e.mu.Lock()
	if _, exists := e.tasks[task.ID]; !exists {
		ctx2, cancel := context.WithCancel(ctx) //nolint:gosec // cancel stored in e.tasks for later invocation
		e.tasks[task.ID] = cancel
		ctx = ctx2
	}
	e.mu.Unlock()

	start := time.Now()
	if err := e.db.WithContext(ctx).Model(task).Updates(map[string]interface{}{
		"status":     model.ReseedTaskRunning,
		"updated_at": start,
	}).Error; err != nil {
		e.logger.Warn("update reseed task to running failed",
			zap.Uint("taskID", task.ID),
			zap.Error(err))
	}

	result = &model.ReseedExecutionResult{
		TaskID:      fmt.Sprintf("%d", task.ID),
		CompletedAt: time.Now(),
	}

	defer func() {
		if r := recover(); r != nil {
			e.logger.Error("reseed RunTask panic recovered",
				zap.Uint("taskID", task.ID),
				zap.Any("panic", r),
			)
			retErr = fmt.Errorf("reseed task panic: %v", r)
		}

		if result == nil {
			return
		}
		result.Duration = time.Since(start).Seconds()
		status := model.ReseedTaskCompleted
		if ctx.Err() == context.Canceled {
			status = model.ReseedTaskCancelled
		} else if retErr != nil || (result.Failed > 0 && result.Matched == 0) {
			status = model.ReseedTaskFailed
		}
		deferCtx, deferCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer deferCancel()
		if err := e.db.WithContext(deferCtx).Model(task).Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		}).Error; err != nil {
			e.logger.Warn("update reseed task final status failed",
				zap.Uint("taskID", task.ID),
				zap.Error(err))
		}
	}()

	clientIDs := ParseClientIDs(task.ClientIDs)
	if len(clientIDs) == 0 {
		return result, nil
	}

	var seedingRecords []model.SeedingTorrentRecord
	q := e.db.WithContext(ctx).Where("status = ?", "seeding")
	if len(clientIDs) == 1 {
		q = q.Where("client_id = ?", clientIDs[0])
	} else {
		q = q.Where("client_id IN ?", clientIDs)
	}
	if err := q.Find(&seedingRecords).Error; err != nil {
		return result, reseedError(ErrReseedDB, "查询做种记录失败", err)
	}
	result.TotalSources = len(seedingRecords)

	if len(seedingRecords) == 0 {
		return result, nil
	}

	var sourceSites []string
	if task.SourceSiteIDs != "" {
		sourceSites = ParseClientIDs(task.SourceSiteIDs)
	}

	targetSites := ParseClientIDs(task.TargetSiteIDs)

	var excludedSites []string
	if task.TargetSiteExcludes != "" {
		excludedSites = ParseClientIDs(task.TargetSiteExcludes)
	}

	sizeTolerance := task.SizeTolerancePercent
	if sizeTolerance <= 0 {
		sizeTolerance = 1.0
	}

	ps := e.preloadSites(ctx, targetSites, excludedSites)
	fpCache := e.preloadFingerprints(ctx, seedingRecords)
	existingMatchesMap := e.preloadExistingMatches(ctx, seedingRecords)
	negCache := e.preloadNegativeCache(ctx, seedingRecords)

	var iyuuResults map[string][]*model.IYUUReseedResult
	var iyuuSidMap map[int]string
	if task.EngineMode == "e2_auto" && e.iyuuService != nil && hasMatchMethod(task.MatchMethods, "iyuu") {
		iyuuResults = e.preloadIYUUResults(ctx, seedingRecords)
		iyuuSidMap = e.preloadIYUUSiteMappings(ctx)
	}

	for _, rec := range seedingRecords {
		if ctx.Err() == context.Canceled {
			break
		}

		if len(sourceSites) > 0 {
			found := false
			for _, s := range sourceSites {
				if rec.SiteName == s {
					found = true
					break
				}
			}
			if !found {
				result.Skipped++
				continue
			}
		}

		var recTitle string
		if fp := fpCache.get(rec.InfoHash, rec.SiteName); fp != nil {
			recTitle = fp.Title
		}

		if !checkPublishEligibility(recTitle) {
			result.Blocked++
			continue
		}

		if existingMatches := existingMatchesMap[rec.InfoHash]; len(existingMatches) > 0 {
			result.DuplicateExists += len(existingMatches)
			continue
		}

		candidates := e.findCandidates(ctx, rec, ps, fpCache, sizeTolerance, task, negCache)

		if iyuuResults != nil {
			iyuuCandidates := e.filterIYUUResults(rec, iyuuResults, iyuuSidMap, targetSites, excludedSites)
			if len(iyuuCandidates) > 0 {
				candidates = append(candidates, iyuuCandidates...)
			}
		}
		if len(candidates) == 0 {
			continue
		}

		concurrency := task.InjectionConcurrency
		if concurrency <= 0 {
			concurrency = 1
		}
		sem := make(chan struct{}, concurrency)
		var wg sync.WaitGroup
		var resultMu sync.Mutex

		for _, c := range candidates {
			resultMu.Lock()
			totalCount := result.Injected + result.Failed + result.Matched
			resultMu.Unlock()
			if totalCount >= task.MaxInjectionsPerRun && task.MaxInjectionsPerRun > 0 {
				break
			}

			if !checkPublishEligibility(recTitle) {
				resultMu.Lock()
				result.Blocked++
				resultMu.Unlock()
				continue
			}

			decision := model.DecisionMatch
			switch {
			case c.TargetInfoHash == rec.InfoHash && c.TargetInfoHash != "":
				decision = model.DecisionSameInfoHash
			case c.MatchMethod == "iyuu":
				decision = model.DecisionMatch
			case c.MatchMethod == "fingerprint":
				decision = model.DecisionMatchPartial
			case c.MatchMethod == "size_title":
				decision = model.DecisionMatchSizeOnly
			}

			match := &model.ReseedMatch{
				ClientID:        rec.ClientID,
				SourceSite:      rec.SiteName,
				SourceTorrentID: rec.TorrentID,
				SourceInfoHash:  rec.InfoHash,
				TargetSite:      c.TargetSite,
				TargetTorrentID: c.TargetTorrentID,
				TargetInfoHash:  c.TargetInfoHash,
				MatchMethod:     c.MatchMethod,
				Confidence:      c.Confidence,
				DecisionType:    string(decision),
				Status:          model.MatchStatusMatched,
			}

			if err := e.SaveMatch(ctx, match); err != nil {
				e.logger.Warn("保存匹配结果失败",
					zap.String("sourceHash", rec.InfoHash),
					zap.String("targetSite", c.TargetSite),
					zap.Error(err),
				)
				resultMu.Lock()
				result.Failed++
				resultMu.Unlock()
				continue
			}

			if e.clientProvider == nil {
				resultMu.Lock()
				result.Matched++
				resultMu.Unlock()
				continue
			}

			wg.Add(1)
			sem <- struct{}{}
			go func(m *model.ReseedMatch) {
				defer wg.Done()
				defer func() { <-sem }()

				if err := e.injectMatch(ctx, m, task, ps); err != nil {
					e.logger.Warn("注入辅种失败",
						zap.Uint("matchID", m.ID),
						zap.String("targetSite", m.TargetSite),
						zap.Error(err),
					)
					resultMu.Lock()
					result.Failed++
					resultMu.Unlock()
					return
				}
				resultMu.Lock()
				result.Injected++
				resultMu.Unlock()
			}(match)

			if task.InjectionIntervalS > 0 {
				jitter := 0
				if task.InjectionJitterS > 0 {
					jitter = rand.IntN(task.InjectionJitterS) //nolint:gosec // jitter does not need crypto/rand
				}
				interval := time.Duration(task.InjectionIntervalS+jitter) * time.Second
				timer := time.NewTimer(interval)
				select {
				case <-timer.C:
				case <-ctx.Done():
					timer.Stop()
					wg.Wait()
					return result, nil
				}
			}
		}
		wg.Wait()
	}

	return result, nil
}

func (e *Engine) findCandidates(ctx context.Context, rec model.SeedingTorrentRecord, ps *preloadedSites, fc *fpCache, sizeTolerance float64, task *model.ReseedTask, negCache map[string]map[string]bool) []model.Candidate {
	if ps == nil {
		return nil
	}

	var candidates []model.Candidate

	for _, siteInfo := range ps.infos {
		if ctx.Err() == context.Canceled {
			break
		}
		if siteInfo.Name == rec.SiteName {
			continue
		}

		if negCache != nil && negCache[rec.InfoHash] != nil && negCache[rec.InfoHash][siteInfo.Name] {
			continue
		}

		siteConfig := ps.configs[siteInfo.Name]
		if siteConfig == nil {
			continue
		}
		adapter := ps.adapters[siteInfo.Name]
		if adapter == nil {
			continue
		}

		c := e.matchLayer0PiecesHash(ctx, adapter, siteConfig, rec, siteInfo.Name, fc)
		if c != nil {
			candidates = append(candidates, *c)
			continue
		}

		c = e.matchLayer1InfoHash(ctx, adapter, siteConfig, rec, siteInfo.Name)
		if c != nil {
			candidates = append(candidates, *c)
			continue
		}

		c = e.matchLayer2SizeTitle(ctx, adapter, siteConfig, rec, siteInfo.Name, sizeTolerance, fc)
		if c != nil {
			candidates = append(candidates, *c)
			continue
		}

		c = e.matchLayer3Fingerprint(ctx, rec, siteInfo.Name, fc)
		if c != nil {
			candidates = append(candidates, *c)
		}
	}

	return candidates
}

func (e *Engine) matchLayer0PiecesHash(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, rec model.SeedingTorrentRecord, siteName string, fc *fpCache) *model.Candidate {
	if !adapter.SupportsSearchByPiecesHash() {
		return nil
	}
	searcher, ok := adapter.(piecesHashSearcher)
	if !ok {
		return nil
	}
	if config.Passkey == "" && config.Cookie == "" {
		return nil
	}

	fp := fc.get(rec.InfoHash, rec.SiteName)
	if fp == nil || fp.PiecesHash == "" {
		return nil
	}

	matches, err := searcher.SearchByPiecesHash(ctx, config, []string{fp.PiecesHash})
	if err != nil {
		e.logger.Debug("Layer0 pieces_hash API 失败",
			zap.String("site", siteName),
			zap.String("pieces_hash", fp.PiecesHash),
			zap.Error(err))
		return nil
	}

	torrentID, found := matches[fp.PiecesHash]
	if !found || torrentID == 0 {
		return nil
	}

	return &model.Candidate{
		TargetSite:      siteName,
		TargetTorrentID: strconv.Itoa(torrentID),
		Confidence:      1.0,
		MatchMethod:     "pieces_hash",
	}
}

func (e *Engine) matchLayer1InfoHash(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, rec model.SeedingTorrentRecord, siteName string) *model.Candidate {
	if rec.InfoHash == "" {
		return nil
	}

	results, err := adapter.SearchTorrents(ctx, config, rec.InfoHash, nil)
	if err != nil {
		e.logger.Debug("Layer1 搜索失败", zap.String("site", siteName), zap.Error(err))
		return nil
	}

	for _, r := range results {
		if r.TorrentID == "" {
			continue
		}
		targetHash, err := adapter.GetTorrentInfoHash(ctx, config, r.TorrentID)
		if err != nil {
			continue
		}
		if targetHash == rec.InfoHash {
			return &model.Candidate{
				TargetSite:      siteName,
				TargetTorrentID: r.TorrentID,
				TargetInfoHash:  targetHash,
				Confidence:      1.0,
				MatchMethod:     "infohash_exact",
			}
		}
	}

	return nil
}

func (e *Engine) matchLayer2SizeTitle(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, rec model.SeedingTorrentRecord, siteName string, sizeTolerance float64, fc *fpCache) *model.Candidate {
	fp := fc.get(rec.InfoHash, rec.SiteName)
	if fp == nil {
		return nil
	}

	keyword := NormalizeTitle(fp.Title)
	if keyword == "" {
		return nil
	}

	if e.fpRepo != nil {
		if cache, err := e.fpRepo.GetSearchCache(ctx, siteName, keyword, fp.TotalSize); err == nil {
			var cached []model.Candidate
			if json.Unmarshal([]byte(cache.Results), &cached) == nil && len(cached) > 0 {
				best := &cached[0]
				best.TargetSite = siteName
				return best
			}
			return nil
		}
	}

	results, err := adapter.SearchTorrents(ctx, config, keyword, nil)
	if err != nil {
		e.logger.Debug("Layer2 搜索失败", zap.String("site", siteName), zap.String("keyword", keyword), zap.Error(err))
		return nil
	}

	var best *model.Candidate
	for _, r := range results {
		if r.TorrentID == "" {
			continue
		}

		input := MatchInput{
			SourceSize: fp.TotalSize,
			TargetSize: r.Size,
		}
		decision := MatchDecision(input, sizeTolerance)

		if decision == model.DecisionMatch || decision == model.DecisionMatchSizeOnly || decision == model.DecisionSameInfoHash {
			confidence := 0.7
			if decision == model.DecisionMatch {
				confidence = 0.85
			} else if decision == model.DecisionSameInfoHash {
				confidence = 1.0
			}
			if best == nil || confidence > best.Confidence {
				best = &model.Candidate{
					TargetSite:      siteName,
					TargetTorrentID: r.TorrentID,
					Confidence:      confidence,
					MatchMethod:     "size_title",
				}
			}
		}
	}

	if e.fpRepo != nil {
		var toCache []model.Candidate
		if best != nil {
			toCache = []model.Candidate{*best}
		}
		if err := e.fpRepo.SaveSearchCache(ctx, siteName, keyword, fp.TotalSize, toCache); err != nil {
			e.logger.Debug("Layer2 保存搜索缓存失败", zap.String("site", siteName), zap.Error(err))
		}
	}

	return best
}

func (e *Engine) matchLayer3Fingerprint(ctx context.Context, rec model.SeedingTorrentRecord, siteName string, fc *fpCache) *model.Candidate {
	sourceFP := fc.get(rec.InfoHash, rec.SiteName)
	if sourceFP == nil {
		return nil
	}

	if sourceFP.PiecesHash == "" && sourceFP.FilesHash == "" {
		return nil
	}

	var targetFPs []model.ContentFingerprint
	if e.fpRepo != nil {
		candidates, err := e.fpRepo.FindCandidatesBySite(ctx, siteName, rec.InfoHash, sourceFP.PiecesHash, sourceFP.TotalSize, 10)
		if err != nil {
			e.logger.Debug("Layer3 候选查询失败", zap.String("site", siteName), zap.Error(err))
			return nil
		}
		targetFPs = candidates
	} else {
		q := e.db.WithContext(ctx).Where("site_name = ? AND info_hash != ?", siteName, rec.InfoHash)
		if sourceFP.PiecesHash != "" {
			q = q.Where("pieces_hash = ?", sourceFP.PiecesHash)
		} else {
			q = q.Where("total_size = ?", sourceFP.TotalSize)
		}
		if err := q.Limit(10).Find(&targetFPs).Error; err != nil {
			return nil
		}
	}

	for _, tfp := range targetFPs {
		confidence := 0.6
		if sourceFP.PiecesHash != "" && tfp.PiecesHash == sourceFP.PiecesHash {
			confidence = 0.95
		}
		if sourceFP.TotalSize > 0 && tfp.TotalSize == sourceFP.TotalSize {
			confidence += 0.1
			if confidence > 1.0 {
				confidence = 1.0
			}
		}

		if tfp.TorrentID != "" {
			return &model.Candidate{
				TargetSite:      siteName,
				TargetTorrentID: tfp.TorrentID,
				TargetInfoHash:  tfp.InfoHash,
				Confidence:      confidence,
				MatchMethod:     "fingerprint",
			}
		}
	}

	return nil
}

func NormalizeTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}

	// Step 1: Unicode NFKD normalization equivalent — strip diacritics
	var norm strings.Builder
	for _, r := range title {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		norm.WriteRune(r)
	}
	title = norm.String()

	// Step 2: Lowercase
	title = strings.ToLower(title)

	// Step 3: Remove content within brackets and parentheses
	var clean strings.Builder
	depth := 0
	for _, r := range title {
		if r == '[' || r == '(' || r == '【' || r == '（' {
			depth++
			continue
		}
		if r == ']' || r == ')' || r == '】' || r == '）' {
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth == 0 {
			clean.WriteRune(r)
		}
	}
	title = clean.String()

	// Step 4: Collapse whitespace
	title = strings.Join(strings.Fields(title), " ")

	// Step 5: Trim
	title = strings.TrimSpace(title)

	// Step 6: Truncate at quality keywords
	stopWords := []string{"2160p", "1080p", "720p", "480p", "x264", "x265", "h264", "h265", "hevc", "web-dl", "bluray", "bdrip", "hdrip", "webrip", "remux"}
	lower := title
	for _, w := range stopWords {
		if idx := strings.Index(lower, w); idx > 3 {
			title = strings.TrimSpace(title[:idx])
			break
		}
	}

	if len(title) > 80 {
		title = title[:80]
	}

	return title
}

func (e *Engine) CreateTask(ctx context.Context, task *model.ReseedTask) error {
	task.Status = model.ReseedTaskIdle
	return dbimpl.ForceCreate(e.db.WithContext(ctx), task)
}

func (e *Engine) GetTask(ctx context.Context, id uint) (*model.ReseedTask, error) {
	var task model.ReseedTask
	err := e.db.WithContext(ctx).First(&task, id).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (e *Engine) ListTasks(ctx context.Context) ([]model.ReseedTask, error) {
	var tasks []model.ReseedTask
	err := e.db.WithContext(ctx).Order("name ASC").Find(&tasks).Error
	return tasks, err
}

func (e *Engine) UpdateTask(ctx context.Context, task *model.ReseedTask) error {
	return e.db.WithContext(ctx).Save(task).Error
}

func (e *Engine) DeleteTask(ctx context.Context, id uint) error {
	e.CancelTask(id)
	return e.db.WithContext(ctx).Delete(&model.ReseedTask{}, id).Error
}

func (e *Engine) ListByClientID(ctx context.Context, clientID string) ([]model.ReseedTask, error) {
	var tasks []model.ReseedTask
	err := e.db.WithContext(ctx).
		Where("client_ids = ? OR client_ids LIKE ? OR client_ids LIKE ? OR client_ids LIKE ?",
			clientID,
			clientID+",%",
			"%,"+clientID+",%",
			"%,"+clientID).
		Find(&tasks).Error
	return tasks, err
}

func (e *Engine) ListEnabled(ctx context.Context) ([]model.ReseedTask, error) {
	var tasks []model.ReseedTask
	err := e.db.WithContext(ctx).
		Where("enabled = ? AND status IN ?", true, []model.ReseedTaskStatus{model.ReseedTaskIdle, model.ReseedTaskRunning}).
		Find(&tasks).Error
	return tasks, err
}

func (e *Engine) BatchSaveMatches(ctx context.Context, matches []*model.ReseedMatch) error {
	return e.db.WithContext(ctx).Create(matches).Error
}

func (e *Engine) FindPendingRetry(ctx context.Context, limit int) ([]model.ReseedMatch, error) {
	var matches []model.ReseedMatch
	err := e.db.WithContext(ctx).
		Where("status = ? AND retry_count < ?", model.MatchStatusFailed, 5).
		Where("next_retry_at IS NULL OR next_retry_at <= ?", time.Now()).
		Order("next_retry_at ASC").
		Limit(limit).
		Find(&matches).Error
	return matches, err
}

func (e *Engine) RetryFailedMatches(ctx context.Context) (int, int, error) {
	matches, err := e.FindPendingRetry(ctx, 50)
	if err != nil {
		return 0, 0, err
	}
	if len(matches) == 0 {
		return 0, 0, nil
	}

	siteSet := make(map[string]bool)
	for _, m := range matches {
		siteSet[m.TargetSite] = true
	}
	var targetSites []string
	for s := range siteSet {
		targetSites = append(targetSites, s)
	}

	ps := e.preloadSites(ctx, targetSites, nil)
	if ps == nil {
		return len(matches), 0, reseedError(ErrReseedConfig, "preload sites for retry failed", nil)
	}

	defaultTask := &model.ReseedTask{ReseedCategory: "cross-seed"}

	retried, succeeded := 0, 0
	for i := range matches {
		if ctx.Err() != nil {
			break
		}
		m := &matches[i]

		if e.clientProvider == nil {
			continue
		}

		if err := e.injectMatch(ctx, m, defaultTask, ps); err != nil {
			nextRetry := time.Now().Add(24 * time.Hour)
			e.db.WithContext(ctx).Model(m).Updates(map[string]interface{}{
				"next_retry_at": &nextRetry,
			})
			retried++
			continue
		}
		succeeded++
		retried++

		if i < len(matches)-1 {
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return retried, succeeded, nil
			}
		}
	}

	return retried, succeeded, nil
}

func (e *Engine) UpdateMatchStatus(ctx context.Context, id uint, status string, failReason string) error {
	return e.db.WithContext(ctx).
		Model(&model.ReseedMatch{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      status,
			"fail_reason": failReason,
			"updated_at":  time.Now(),
		}).Error
}

func (e *Engine) SaveMatch(ctx context.Context, match *model.ReseedMatch) error {
	return e.db.WithContext(ctx).Create(match).Error
}

func (e *Engine) FindMatchesByInfoHash(ctx context.Context, infoHash string) ([]model.ReseedMatch, error) {
	var matches []model.ReseedMatch
	err := e.db.WithContext(ctx).
		Where("source_info_hash = ?", infoHash).
		Find(&matches).Error
	return matches, err
}

func (e *Engine) FindMatchByID(ctx context.Context, id uint) (*model.ReseedMatch, error) {
	var m model.ReseedMatch
	err := e.db.WithContext(ctx).First(&m, id).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (e *Engine) RetryMatch(ctx context.Context, id uint) (*model.ReseedMatch, error) {
	var m model.ReseedMatch
	if err := e.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}

	if m.Status != model.MatchStatusFailed {
		return nil, reseedError(ErrReseedGeneric, fmt.Sprintf("只能重试失败的匹配记录，当前状态: %s", m.Status), nil)
	}

	now := time.Now()
	newRetry := m.RetryCount + 1

	if err := e.db.WithContext(ctx).Model(&m).Updates(map[string]interface{}{
		"status":        model.MatchStatusMatched,
		"retry_count":   newRetry,
		"fail_reason":   "",
		"next_retry_at": &now,
	}).Error; err != nil {
		return nil, err
	}
	m.Status = model.MatchStatusMatched
	m.RetryCount = newRetry
	m.FailReason = ""
	m.NextRetryAt = &now
	return &m, nil
}

func (e *Engine) DeleteNegativeCache(ctx context.Context, infoHash, site string) (int64, error) {
	q := e.db.WithContext(ctx).Where("source_info_hash = ?", infoHash)
	if site != "" {
		q = q.Where("source_site = ?", site)
	}
	result := q.Delete(&model.ReseedNegativeCache{})
	return result.RowsAffected, result.Error
}

func (e *Engine) SetNegativeCache(ctx context.Context, sourceSite, sourceInfoHash, targetSite, method string, layerDepth int, ttl time.Duration) error {
	entry := &model.ReseedNegativeCache{
		SourceSite:      sourceSite,
		SourceInfoHash:  sourceInfoHash,
		ExcludedTargets: targetSite,
		LastMethod:      method,
		LayerDepth:      layerDepth,
		ExpiresAt:       time.Now().Add(ttl),
	}
	return e.db.WithContext(ctx).Create(entry).Error
}

func (e *Engine) GetNegativeCacheByHashes(ctx context.Context, hashes []string) ([]model.ReseedNegativeCache, error) {
	if len(hashes) == 0 {
		return nil, nil
	}
	var entries []model.ReseedNegativeCache
	err := e.db.WithContext(ctx).
		Where("source_info_hash IN ? AND expires_at > ?", hashes, time.Now()).
		Find(&entries).Error
	return entries, err
}

func (e *Engine) FlushNegativeCache(ctx context.Context) (int64, error) {
	result := e.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&model.ReseedNegativeCache{})
	return result.RowsAffected, result.Error
}

func (e *Engine) OnTorrentSeeding(parentCtx context.Context, record model.SeedingTorrentRecord, reseedClientIDs []string) {
	ctx, cancel := context.WithTimeout(parentCtx, 5*time.Minute)
	defer cancel()

	e.logger.Info("auto reseed triggered",
		zap.String("site", record.SiteName),
		zap.String("info_hash", record.InfoHash),
		zap.Strings("reseed_client_ids", reseedClientIDs))

	if e.siteProvider == nil {
		e.logger.Warn("auto reseed: siteProvider not available")
		return
	}

	ps := e.preloadSites(ctx, nil, []string{record.SiteName})
	if ps == nil || len(ps.infos) == 0 {
		e.logger.Debug("auto reseed: no target sites available")
		return
	}

	records := []model.SeedingTorrentRecord{record}
	fpc := e.preloadFingerprints(ctx, records)
	negCache := e.preloadNegativeCache(ctx, records)
	existingMatchesMap := e.preloadExistingMatches(ctx, records)

	if len(existingMatchesMap[record.InfoHash]) > 0 {
		e.logger.Debug("auto reseed: already matched", zap.String("info_hash", record.InfoHash))
		return
	}

	var recTitle string
	if fp := fpc.get(record.InfoHash, record.SiteName); fp != nil {
		recTitle = fp.Title
	}
	if !checkPublishEligibility(recTitle) {
		e.logger.Info("auto reseed: blocked by publish eligibility", zap.String("title", recTitle))
		return
	}

	task := &model.ReseedTask{
		SizeTolerancePercent: 1.0,
		MaxInjectionsPerRun:  10,
		ReseedCategory:       "cross-seed",
	}

	candidates := e.findCandidates(ctx, record, ps, fpc, task.SizeTolerancePercent, task, negCache)
	if len(candidates) == 0 {
		e.logger.Debug("auto reseed: no candidates found", zap.String("info_hash", record.InfoHash))
		return
	}

	e.logger.Info("auto reseed: candidates found",
		zap.String("info_hash", record.InfoHash),
		zap.Int("count", len(candidates)))

	for _, c := range candidates {
		for _, clientID := range reseedClientIDs {
			match := &model.ReseedMatch{
				ClientID:        clientID,
				SourceSite:      record.SiteName,
				SourceTorrentID: record.TorrentID,
				SourceInfoHash:  record.InfoHash,
				TargetSite:      c.TargetSite,
				TargetTorrentID: c.TargetTorrentID,
				TargetInfoHash:  c.TargetInfoHash,
				MatchMethod:     c.MatchMethod,
				Confidence:      c.Confidence,
				DecisionType:    string(model.DecisionMatch),
				Status:          model.MatchStatusMatched,
			}

			if err := e.SaveMatch(ctx, match); err != nil {
				e.logger.Warn("auto reseed: save match failed",
					zap.String("target_site", c.TargetSite),
					zap.Error(err))
				continue
			}

			if e.clientProvider == nil {
				continue
			}

			if err := e.injectMatch(ctx, match, task, ps); err != nil {
				e.logger.Warn("auto reseed: inject failed",
					zap.Uint("match_id", match.ID),
					zap.String("target_site", c.TargetSite),
					zap.Error(err))
				continue
			}
			e.logger.Info("auto reseed: injected",
				zap.String("source_hash", record.InfoHash),
				zap.String("target_site", c.TargetSite),
				zap.String("client_id", clientID))
		}
	}
}

func (e *Engine) RunEnabledTasks(ctx context.Context) error {
	var tasks []model.ReseedTask
	if err := e.db.WithContext(ctx).Where("enabled = ?", true).Find(&tasks).Error; err != nil {
		return reseedError(ErrReseedDB, "query enabled reseed tasks", err)
	}

	if len(tasks) == 0 {
		return nil
	}

	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	now := time.Now()

	for i := range tasks {
		scheduleStr := tasks[i].Schedule
		if scheduleStr == "" {
			scheduleStr = "0 0 */6 * * *"
		}
		if parts := strings.Fields(scheduleStr); len(parts) == 5 {
			scheduleStr = "0 " + scheduleStr
		}

		sched, err := parser.Parse(scheduleStr)
		if err != nil {
			e.logger.Warn("invalid reseed task schedule, running immediately",
				zap.Uint("task_id", tasks[i].ID),
				zap.String("schedule", tasks[i].Schedule),
				zap.Error(err))
		} else {
			var nextRun time.Time
			if tasks[i].LastRunAt != nil {
				nextRun = sched.Next(*tasks[i].LastRunAt)
			}
			if !nextRun.IsZero() && nextRun.After(now) {
				continue
			}
		}

		if _, err := e.RunTask(ctx, &tasks[i]); err != nil {
			e.logger.Warn("reseed task failed",
				zap.Uint("task_id", tasks[i].ID),
				zap.String("name", tasks[i].Name),
				zap.Error(err),
			)
		}

		e.db.WithContext(ctx).Model(&tasks[i]).Updates(map[string]interface{}{
			"last_run_at": now,
			"updated_at":  now,
		})
	}

	return nil
}

func ParseClientIDs(ids string) []string {
	if ids == "" {
		return nil
	}
	parts := strings.Split(ids, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func checkPublishEligibility(title string) bool {
	if title == "" {
		return true
	}
	keywords := []string{"禁转", "独占", "谢绝转载", "限时禁转", "严禁转载"}
	for _, kw := range keywords {
		if strings.Contains(title, kw) {
			return false
		}
	}
	groups := []string{"CatEDU"}
	for _, g := range groups {
		if strings.Contains(title, g) {
			return false
		}
	}
	return true
}

func (e *Engine) injectMatch(ctx context.Context, match *model.ReseedMatch, task *model.ReseedTask, ps *preloadedSites) error {
	if ps == nil {
		return reseedError(ErrReseedConfig, "preloaded sites not available", nil)
	}

	if err := e.db.WithContext(ctx).Model(match).Updates(map[string]interface{}{
		"status":     model.MatchStatusInjecting,
		"updated_at": time.Now(),
	}).Error; err != nil {
		e.logger.Warn("update reseed match to injecting failed",
			zap.Uint("matchID", match.ID),
			zap.Error(err))
	}

	targetConfig := ps.configs[match.TargetSite]
	if targetConfig == nil {
		return e.failMatch(ctx, match, fmt.Sprintf("目标站配置未预加载: %s", match.TargetSite))
	}

	targetAdapter := ps.adapters[match.TargetSite]
	if targetAdapter == nil {
		return e.failMatch(ctx, match, fmt.Sprintf("目标站适配器未预加载: %s", match.TargetSite))
	}

	torrentData, err := targetAdapter.DownloadTorrent(ctx, targetConfig, match.TargetTorrentID)
	if err != nil {
		return e.failMatch(ctx, match, fmt.Sprintf("下载目标种子失败: %v", err))
	}

	dlClient, err := e.clientProvider.Get(match.ClientID)
	if err != nil {
		return e.failMatch(ctx, match, fmt.Sprintf("获取下载器客户端失败: %v", err))
	}

	opts := model.AddTorrentOptions{
		Category: task.ReseedCategory,
		Tags:     []string{"reseed", "pt-forward"},
		Paused:   false,
	}

	if match.SourceInfoHash != "" {
		sourceTorrent, serr := dlClient.GetTorrentByHash(ctx, match.SourceInfoHash)
		if serr == nil && sourceTorrent != nil && sourceTorrent.SavePath != "" {
			opts.SavePath = sourceTorrent.SavePath
		}
	}

	if len(torrentData) == 0 {
		return e.failMatch(ctx, match, "种子数据为空")
	}

	addResult, err := dlClient.AddFromFile(ctx, torrentData, opts)
	if err != nil {
		if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "exist") {
			return e.failMatch(ctx, match, "种子已存在于下载器中")
		}
		return e.failMatch(ctx, match, fmt.Sprintf("注入种子到下载器失败: %v", err))
	}

	now := time.Now()
	return e.db.WithContext(ctx).Model(match).Updates(map[string]interface{}{
		"status":           model.MatchStatusInjected,
		"target_info_hash": addResult.InfoHash,
		"injected_at":      &now,
		"updated_at":       now,
	}).Error
}

func (e *Engine) failMatch(ctx context.Context, match *model.ReseedMatch, reason string) error {
	match.RetryCount++
	match.FailReason = reason

	decisionType := model.DecisionDownloadFailed
	switch {
	case strings.Contains(reason, "已存在"):
		decisionType = model.DecisionAlreadyExists
	case strings.Contains(reason, "禁转") || strings.Contains(reason, "独占"):
		decisionType = model.DecisionBlockedRelease
	}

	if err := e.db.WithContext(ctx).Model(match).Updates(map[string]interface{}{
		"status":        model.MatchStatusFailed,
		"decision_type": string(decisionType),
		"fail_reason":   reason,
		"retry_count":   match.RetryCount,
		"updated_at":    time.Now(),
	}).Error; err != nil {
		e.logger.Error("failMatch update db error", zap.Uint("matchID", match.ID), zap.Error(err))
	}

	ttl := 24 * time.Hour
	switch decisionType {
	case model.DecisionAlreadyExists:
		ttl = 72 * time.Hour
	case model.DecisionBlockedRelease:
		ttl = 168 * time.Hour
	}
	if err := e.SetNegativeCache(ctx, match.SourceSite, match.SourceInfoHash, match.TargetSite, match.MatchMethod, 1, ttl); err != nil {
		e.logger.Debug("set negative cache failed", zap.Uint("matchID", match.ID), zap.Error(err))
	}

	return reseedError(ErrReseedGeneric, reason, nil)
}

func hasMatchMethod(methodsStr, method string) bool {
	if methodsStr == "" {
		return true
	}
	for _, m := range ParseClientIDs(methodsStr) {
		if m == method {
			return true
		}
	}
	return false
}

func (e *Engine) filterIYUUResults(rec model.SeedingTorrentRecord, iyuuResults map[string][]*model.IYUUReseedResult, sidMap map[int]string, targetSites, excludedSites []string) []model.Candidate {
	results := iyuuResults[rec.InfoHash]
	if len(results) == 0 {
		return nil
	}

	exclSet := make(map[string]bool, len(excludedSites))
	for _, s := range excludedSites {
		exclSet[s] = true
	}

	targetSet := make(map[string]bool, len(targetSites))
	for _, s := range targetSites {
		targetSet[s] = true
	}

	var candidates []model.Candidate
	for _, result := range results {
		for _, target := range result.Targets {
			siteName := sidMap[target.Sid]
			if siteName == "" {
				continue
			}
			if exclSet[siteName] || siteName == rec.SiteName {
				continue
			}
			if len(targetSet) > 0 && !targetSet[siteName] {
				continue
			}
			candidates = append(candidates, model.Candidate{
				TargetSite:      siteName,
				TargetTorrentID: fmt.Sprintf("%d", target.TorrentID),
				TargetInfoHash:  target.InfoHash,
				Confidence:      0.9,
				MatchMethod:     "iyuu",
			})
		}
	}

	return candidates
}

func (e *Engine) iyuuSidToSite(ctx context.Context, sid int) string {
	var mapping model.IYUUSiteMapping
	if err := e.db.WithContext(ctx).Where("iyuu_sid = ?", sid).First(&mapping).Error; err != nil {
		return ""
	}

	if mapping.SiteName != "" && e.siteProvider != nil {
		info, err := e.siteProvider.GetSiteInfo(ctx, mapping.SiteName)
		if err == nil && info != nil {
			return info.Name
		}
	}

	if mapping.SiteDomain != "" && e.siteProvider != nil {
		info, err := e.siteProvider.GetSiteInfoByURL(ctx, mapping.SiteDomain)
		if err == nil && info != nil {
			return info.Name
		}
	}

	return mapping.SiteName
}
