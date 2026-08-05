package reseed

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/ranfish/pt-forward/internal/audit"
	clientpkg "github.com/ranfish/pt-forward/internal/client"
	"github.com/ranfish/pt-forward/internal/compliance"
	dbimpl "github.com/ranfish/pt-forward/internal/db"
	"github.com/ranfish/pt-forward/internal/fingerprint"
	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/scheduler"
	"github.com/ranfish/pt-forward/internal/util"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const errAdapterNotFoundCode = 31006

type preloadedSites struct {
	infos       []*model.SiteInfo
	configs     map[string]*model.SiteConfig
	adapters    map[string]model.SiteAdapter
	siteLimits  map[string]*model.Site
}

type siteLimitEntry struct {
	date  string
	count int
}

type siteLimiter struct {
	mu      sync.Mutex
	counts  map[string]*siteLimitEntry
}

func newSiteLimiter() *siteLimiter {
	return &siteLimiter{counts: make(map[string]*siteLimitEntry)}
}

func (l *siteLimiter) checkAndIncr(siteName string, maxCount int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if maxCount <= 0 {
		return true
	}
	today := time.Now().Format("2006-01-02")
	entry := l.counts[siteName]
	if entry == nil || entry.date != today {
		entry = &siteLimitEntry{date: today, count: 0}
		l.counts[siteName] = entry
	}
	if entry.count >= maxCount {
		return false
	}
	entry.count++
	return true
}

func (l *siteLimiter) getCount(siteName string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	entry := l.counts[siteName]
	if entry == nil || entry.date != today {
		return 0
	}
	return entry.count
}

type l2Stats struct {
	mu             sync.Mutex
	searched       map[string]int
	noKeyword      int
	noGroup        int
	searchFailed   int
	searchEmpty    int
	groupMismatch  int
	sizeMismatch   int
	matched        int
	siteResults    map[string]string
}

func newL2Stats() *l2Stats {
	return &l2Stats{
		searched:    make(map[string]int),
		siteResults: make(map[string]string),
	}
}

func (s *l2Stats) record(site string, result string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.searched[site]++
	s.siteResults[site] = result
}

func (s *l2Stats) log(e *Engine) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e.logger.Info("L2 search verification stats",
		zap.Int("searched", len(s.searched)),
		zap.Int("noKeyword", s.noKeyword),
		zap.Int("noGroup", s.noGroup),
		zap.Int("searchFailed", s.searchFailed),
		zap.Int("searchEmpty", s.searchEmpty),
		zap.Int("groupMismatch", s.groupMismatch),
		zap.Int("sizeMismatch", s.sizeMismatch),
		zap.Int("matched", s.matched))
	sites := make([]string, 0, len(s.siteResults))
	for site := range s.siteResults {
		sites = append(sites, site)
	}
	sort.Strings(sites)
	for _, site := range sites {
		e.logger.Info("L2 site stats",
			zap.String("site", site),
			zap.Int("searchCount", s.searched[site]),
			zap.String("sampleResult", s.siteResults[site]))
	}
}

func drainChannel[T any](ch <-chan T) {
	for range ch {
	}
}

type piecesHashCache struct {
	bySite       map[string]map[string]int
	queriedSites map[string]bool
}

func (c *piecesHashCache) get(siteName, piecesHash string) (int, bool) {
	if c == nil {
		return 0, false
	}
	m, ok := c.bySite[siteName]
	if !ok {
		return 0, false
	}
	tid, ok := m[piecesHash]
	return tid, ok
}

func (c *piecesHashCache) wasQueried(siteName string) bool {
	if c == nil {
		return false
	}
	return c.queriedSites[siteName]
}

// §33.32 — piecesHashSearcher is an optional capability interface.
// Adapters that support NexusPHP /api/pieces-hash implement this.
type piecesHashSearcher interface {
	SearchByPiecesHash(ctx context.Context, config *model.SiteConfig, piecesHashes []string) (map[string]int, error)
}

type sourceTorrent struct {
	InfoHash  string
	TorrentID string
	SiteName  string
	ClientID  string
	Name      string
	SavePath  string
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
	db                   *gorm.DB
	logger               *zap.Logger
	siteProvider         model.SiteInfoProvider
	clientProvider       model.DownloaderProvider
	iyuuService          model.IYUUService
	fpRepo               *fingerprint.Repository
	trackerResolver      *TrackerSiteResolver
	scheduler            *scheduler.Registry
	limiter              *siteLimiter
	mu                   sync.RWMutex
	tasks                map[uint]context.CancelFunc
	cloudFPService       model.CloudFPService
	deleteReporter       *deleteReporter
	contributeReporter   *contributeReporter
	currentCloudFPCache  *cloudFPCache
	currentDomainResolver *domainResolver
	complianceChecker    *compliance.Checker
}

func NewEngine(db *gorm.DB, logger *zap.Logger) *Engine {
	logger = logger.With(zap.String("component", "reseed"))
	return &Engine{
		db:      db,
		logger:  logger,
		limiter: newSiteLimiter(),
		tasks:   make(map[uint]context.CancelFunc),
	}
}

func (e *Engine) SetSiteProvider(sp model.SiteInfoProvider) {
	e.siteProvider = sp
}

func (e *Engine) SetFingerprintRepo(repo *fingerprint.Repository) {
	e.fpRepo = repo
}

func (e *Engine) DB() *gorm.DB {
	return e.db
}

func (e *Engine) SetClientProvider(cp model.DownloaderProvider) {
	e.clientProvider = cp
}

func (e *Engine) SetIYUUService(svc model.IYUUService) {
	e.iyuuService = svc
}

func (e *Engine) SetTrackerResolver(resolver *TrackerSiteResolver) {
	e.trackerResolver = resolver
}

func (e *Engine) SetComplianceChecker(c *compliance.Checker) {
	e.complianceChecker = c
}


// StartInjectionConsumer 启动后台注入消费者（对齐 IYUU 两阶段架构）
func (e *Engine) StartInjectionConsumer(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		e.logger.Info("reseed injection consumer started")
		for {
			select {
			case <-ctx.Done():
				e.logger.Info("reseed injection consumer stopped")
				return
			case <-ticker.C:
				e.processPendingInjections(ctx)
			}
		}
	}()
}

// processPendingInjections 消费 status=matched 的记录，逐步注入
func (e *Engine) processPendingInjections(ctx context.Context) {
	if e.clientProvider == nil {
		return
	}

	var matches []model.ReseedMatch
	if err := e.db.WithContext(ctx).
		Where("status = ?", model.MatchStatusMatched).
		Order("created_at ASC").
		Limit(5).
		Find(&matches).Error; err != nil || len(matches) == 0 {
		return
	}

	ps := e.preloadSites(ctx, nil, nil)
	if ps == nil {
		return
	}

	for i := range matches {
		if ctx.Err() != nil {
			return
		}
		m := &matches[i]

		// CAS: matched → injecting（原子更新，防止并发消费）
		result := e.db.WithContext(ctx).Model(&model.ReseedMatch{}).
			Where("id = ? AND status = ?", m.ID, model.MatchStatusMatched).
			Update("status", model.MatchStatusInjecting)
		if result.Error != nil || result.RowsAffected == 0 {
			continue
		}

		var task model.ReseedTask
		if err := e.db.WithContext(ctx).First(&task, m.TaskID).Error; err != nil {
			e.failMatch(ctx, m, fmt.Sprintf("任务不存在: %v", err))
			continue
		}

		e.logger.Debug("injection consumer: injectMatch",
			zap.Uint("matchID", m.ID),
			zap.String("targetSite", m.TargetSite),
			zap.String("targetTorrentID", m.TargetTorrentID))

		if err := e.injectMatch(ctx, m, &task, ps); err != nil {
			if errors.Is(err, errAlreadyExists) {
				continue // verifyDuplicateAndFinish 已处理状态
			}
			continue // failMatch 已在 injectMatch 内调用
		}

		// 注入间隔
		interval := task.InjectionIntervalS
		if interval <= 0 {
			interval = 1
		}
		select {
		case <-time.After(time.Duration(interval) * time.Second):
		case <-ctx.Done():
			return
		}
	}
}

func (e *Engine) SetScheduler(registry *scheduler.Registry) {
	e.scheduler = registry
}

func (e *Engine) SetCloudFPService(svc model.CloudFPService) {
	e.cloudFPService = svc
	if svc != nil {
		e.deleteReporter = newDeleteReporter(svc, e.logger)
		e.contributeReporter = newContributeReporter(svc, e.logger)
	}
}

func reseedSchedulerName(taskID uint) string {
	return fmt.Sprintf("reseed_task_%d", taskID)
}

func (e *Engine) SyncTaskSchedule(ctx context.Context, task *model.ReseedTask) {
	if e.scheduler == nil {
		return
	}
	name := reseedSchedulerName(task.ID)
	if !task.Enabled {
		_ = e.scheduler.Unregister(name)
		return
	}
	handler := func(ctx context.Context) error {
		_, err := e.RunTask(ctx, task)
		if err != nil {
			e.logger.Warn("reseed task failed",
				zap.Uint("task_id", task.ID),
				zap.String("name", task.Name),
				zap.Error(err))
		}
		e.db.WithContext(ctx).Model(task).Updates(map[string]interface{}{
			"last_run_at": time.Now(),
			"updated_at":  time.Now(),
		})
		return nil
	}
	if err := e.scheduler.Unregister(name); err == nil {
	}
	schedule := task.Schedule
	if schedule == "" {
		schedule = "0 */6 * * *"
	}
	if err := e.scheduler.Register(name, "reseed", schedule, handler); err != nil {
		e.logger.Warn("failed to register reseed task schedule",
			zap.Uint("task_id", task.ID),
			zap.String("schedule", schedule),
			zap.Error(err))
	}
}

func (e *Engine) RemoveTaskSchedule(taskID uint) {
	if e.scheduler == nil {
		return
	}
	_ = e.scheduler.Unregister(reseedSchedulerName(taskID))
}

func (e *Engine) RegisterAllTaskSchedules(ctx context.Context) {
	if e.scheduler == nil {
		return
	}
	var tasks []model.ReseedTask
	if err := e.db.WithContext(ctx).Where("enabled = ?", true).Find(&tasks).Error; err != nil {
		e.logger.Warn("failed to load reseed tasks for scheduler", zap.Error(err))
		return
	}
	for i := range tasks {
		e.SyncTaskSchedule(ctx, &tasks[i])
	}
	e.logger.Info("reseed task schedules registered", zap.Int("count", len(tasks)))
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
				e.logger.Warn("failed to get target site info", zap.String("site", siteName), zap.Error(err))
				continue
			}
			allSites = append(allSites, info)
		}
	} else {
		sites, err := e.siteProvider.ListSites(ctx)
		if err != nil {
			e.logger.Warn("failed to list sites", zap.Error(err))
			return nil
		}
		allSites = sites
	}

	var eligible []*model.SiteInfo
	configs := make(map[string]*model.SiteConfig)
	adapters := make(map[string]model.SiteAdapter)
	siteLimits := make(map[string]*model.Site)

	var siteNames []string
	for _, info := range allSites {
		if exclSet[info.Name] || !info.Enabled {
			continue
		}
		siteNames = append(siteNames, info.Name)
	}

	if len(siteNames) > 0 {
		var sites []model.Site
		if err := e.db.WithContext(ctx).Where("name IN ?", siteNames).Find(&sites).Error; err != nil {
			e.logger.Warn("query sites for limits failed", zap.Error(err))
		}
		for i := range sites {
			siteLimits[sites[i].Name] = &sites[i]
		}
	}

	for _, info := range allSites {
		if exclSet[info.Name] || !info.Enabled {
			continue
		}

		config, err := e.siteProvider.GetSiteConfig(ctx, info.Name)
		if err != nil {
			e.logger.Warn("failed to get site config", zap.String("site", info.Name), zap.Error(err))
			continue
		}

		adapter, err := e.siteProvider.GetAdapter(ctx, info.Name)
		if err != nil {
			e.logger.Warn("failed to get adapter", zap.String("site", info.Name), zap.Error(err))
			continue
		}

		eligible = append(eligible, info)
		configs[info.Name] = config
		adapters[info.Name] = adapter
	}

	return &preloadedSites{
		infos:      eligible,
		configs:    configs,
		adapters:   adapters,
		siteLimits: siteLimits,
	}
}

const preloadBatchSize = 500

func chunkStrings(slice []string, size int) [][]string {
	var chunks [][]string
	for i := 0; i < len(slice); i += size {
		end := i + size
		if end > len(slice) {
			end = len(slice)
		}
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}

func (e *Engine) preloadFingerprints(ctx context.Context, infoHashes []string) *fpCache {
	if len(infoHashes) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var deduped []string
	for _, ih := range infoHashes {
		if !seen[ih] {
			seen[ih] = true
			deduped = append(deduped, ih)
		}
	}

	var fps []*model.ContentFingerprint
	if e.fpRepo != nil {
		for _, chunk := range chunkStrings(deduped, preloadBatchSize) {
			batch, err := e.fpRepo.BatchGetByInfoHashes(ctx, chunk)
			if err != nil {
				e.logger.Warn("batch preload fingerprints failed", zap.Error(err))
				return &fpCache{byKey: make(map[string]*model.ContentFingerprint)}
			}
			fps = append(fps, batch...)
		}
	} else {
		var batch []model.ContentFingerprint
		for _, chunk := range chunkStrings(deduped, preloadBatchSize) {
			var partial []model.ContentFingerprint
			if err := e.db.WithContext(ctx).Where("info_hash IN ?", chunk).Find(&partial).Error; err != nil {
				e.logger.Warn("batch preload fingerprints failed (DB)", zap.Error(err))
				return &fpCache{byKey: make(map[string]*model.ContentFingerprint)}
			}
			batch = append(batch, partial...)
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

func (e *Engine) preloadExistingMatches(ctx context.Context, infoHashes []string, clientHashes map[string]bool) (map[string][]model.ReseedMatch, int) {
	if len(infoHashes) == 0 {
		return nil, 0
	}

	var matches []model.ReseedMatch
	for _, chunk := range chunkStrings(infoHashes, preloadBatchSize) {
		var partial []model.ReseedMatch
		if err := e.db.WithContext(ctx).
			Where("source_info_hash IN ?", chunk).
			Find(&partial).Error; err != nil {
			e.logger.Warn("batch preload existing matches failed", zap.Error(err))
			return make(map[string][]model.ReseedMatch), 0
		}
		matches = append(matches, partial...)
	}

	deletedCount := 0
	result := make(map[string][]model.ReseedMatch, len(matches))
	for _, m := range matches {
		switch m.Status {
		case model.MatchStatusFailed:
			continue
		case model.MatchStatusInjected:
			if m.TargetInfoHash != "" {
				if clientHashes != nil && !clientHashes[m.TargetInfoHash] {
					deletedCount++
					continue
				}
			} else if clientHashes != nil {
				deletedCount++
				continue
			}
		}
		result[m.SourceInfoHash] = append(result[m.SourceInfoHash], m)
	}
	return result, deletedCount
}

func (e *Engine) preloadNegativeCache(ctx context.Context, infoHashes []string) map[string]map[string]bool {
	if len(infoHashes) == 0 {
		return nil
	}

	entries, err := e.GetNegativeCacheByHashes(ctx, infoHashes)
	if err != nil {
		e.logger.Warn("preload negative cache failed", zap.Error(err))
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

func (e *Engine) preflightCheck(ctx context.Context, ps *preloadedSites, concurrency int) {
	if ps == nil || concurrency <= 0 {
		return
	}

	type siteCheck struct {
		name    string
		baseURL string
		domain  string
		client  *http.Client
	}

	var checks []siteCheck
	for _, siteInfo := range ps.infos {
		config := ps.configs[siteInfo.Name]
		if config == nil || config.BaseURL == "" || config.Domain == "" {
			continue
		}
		if httpclient.IsDomainCircuitOpen(config.Domain) {
			continue
		}
		checks = append(checks, siteCheck{
			name:    siteInfo.Name,
			baseURL: config.BaseURL,
			domain:  config.Domain,
			client: httpclient.NewSiteHTTPClient(httpclient.SiteHTTPConfig{
				Domain:        config.Domain,
				Timeout:       10 * time.Second,
				ProxyURL:      config.ProxyURL,
				SkipSSLVerify: config.SkipSSLVerify,
			}),
		})
	}

	if len(checks) == 0 {
		return
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	failed := 0

	for _, c := range checks {
		wg.Add(1)
		go func(sc siteCheck) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			doCheck := func() (int, error) {
				checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()
				req, err := http.NewRequestWithContext(checkCtx, http.MethodHead, sc.baseURL, nil)
				if err != nil {
					return 0, err
				}
				resp, err := sc.client.Do(req)
				if err != nil {
					return 0, err
				}
				defer resp.Body.Close()
				return resp.StatusCode, nil
			}

			status, err := doCheck()
			if err != nil {
				time.Sleep(500 * time.Millisecond)
				status, err = doCheck()
			}
			if err != nil {
				httpclient.TripDomainCircuit(sc.domain)
				e.logger.Warn("site connectivity check failed (after retry), circuit broken",
					zap.String("site", sc.name),
					zap.String("domain", sc.domain),
					zap.Error(err))
				failed++
				return
			}
			if status >= 200 && status < 500 {
				return
			}
			httpclient.TripDomainCircuit(sc.domain)
			e.logger.Warn("site connectivity check failed (5xx), circuit broken",
				zap.String("site", sc.name),
				zap.String("domain", sc.domain),
				zap.Int("status", status))
			failed++
		}(c)
	}
	wg.Wait()

	e.logger.Info("site connectivity check completed",
		zap.Int("total", len(checks)),
		zap.Int("failed", failed))
}

func (e *Engine) preloadPiecesHashCache(ctx context.Context, sources []sourceTorrent, ps *preloadedSites, fc *fpCache, negCache map[string]map[string]bool, scanConcurrency int, taskID uint) *piecesHashCache {
	if ps == nil || fc == nil || len(sources) == 0 {
		return nil
	}

	eligibleSites := make(map[string]struct {
		config  *model.SiteConfig
		adapter model.SiteAdapter
	})

	for _, siteInfo := range ps.infos {
		siteConfig := ps.configs[siteInfo.Name]
		if siteConfig == nil || !siteConfig.SupportsPiecesHashAPI {
			continue
		}
		if siteConfig.Passkey == "" && siteConfig.Cookie == "" {
			continue
		}
		if httpclient.IsDomainCircuitOpen(siteConfig.Domain) {
			continue
		}
		adapter := ps.adapters[siteInfo.Name]
		if adapter == nil || !adapter.SupportsSearchByPiecesHash() {
			continue
		}
		if _, ok := adapter.(piecesHashSearcher); !ok {
			continue
		}
		eligibleSites[siteInfo.Name] = struct {
			config  *model.SiteConfig
			adapter model.SiteAdapter
		}{config: siteConfig, adapter: adapter}
	}

	if len(eligibleSites) == 0 {
		return nil
	}

	sitePiecesHashes := make(map[string]map[string]string)
	for _, src := range sources {
		fp := fc.get(src.InfoHash, src.SiteName)
		if fp == nil || fp.PiecesHash == "" {
			continue
		}
		for siteName := range eligibleSites {
			if siteName == src.SiteName {
				continue
			}
			if negCache != nil && negCache[src.InfoHash] != nil && negCache[src.InfoHash][siteName] {
				continue
			}
			if sitePiecesHashes[siteName] == nil {
				sitePiecesHashes[siteName] = make(map[string]string)
			}
			sitePiecesHashes[siteName][fp.PiecesHash] = src.InfoHash
		}
	}

	if len(sitePiecesHashes) == 0 {
		return nil
	}

	cache := &piecesHashCache{
		bySite:       make(map[string]map[string]int),
		queriedSites: make(map[string]bool),
	}

	type queryJob struct {
		siteName string
		config   *model.SiteConfig
		searcher piecesHashSearcher
		hashes   []string
	}

	var jobs []queryJob
	for siteName, phMap := range sitePiecesHashes {
		es := eligibleSites[siteName]
		searcher := es.adapter.(piecesHashSearcher)
		allHashes := make([]string, 0, len(phMap))
		for ph := range phMap {
			allHashes = append(allHashes, ph)
		}
		jobs = append(jobs, queryJob{
			siteName: siteName,
			config:   es.config,
			searcher: searcher,
			hashes:   allHashes,
		})
	}

	if scanConcurrency <= 0 {
		scanConcurrency = 10
	}

	type queryResult struct {
		siteName string
		results  map[string]int
		batchOK  int
		queried  int
	}

	results := make([]queryResult, len(jobs))
	sem := make(chan struct{}, scanConcurrency)
	var wg sync.WaitGroup

	for i, job := range jobs {
		wg.Add(1)
		go func(idx int, j queryJob) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			defer func() {
				if r := recover(); r != nil {
					e.logger.Error("pieces_hash query panic recovered",
						zap.String("site", j.siteName),
						zap.Any("panic", r))
				}
			}()

			const batchSize = 100
			siteResults := make(map[string]int)
			batchOK := 0

			for k := 0; k < len(j.hashes); k += batchSize {
				end := k + batchSize
				if end > len(j.hashes) {
					end = len(j.hashes)
				}
				batch := j.hashes[k:end]

				matches, err := j.searcher.SearchByPiecesHash(ctx, j.config, batch)
				if err != nil {
					e.logger.Warn("batch pieces_hash query failed",
						zap.String("site", j.siteName),
						zap.Int("batch", k/batchSize+1),
						zap.Int("totalBatches", (len(j.hashes)+batchSize-1)/batchSize),
						zap.Error(err))
					continue
				}
				batchOK++

				for ph, tid := range matches {
					siteResults[ph] = tid
				}
			}

			results[idx] = queryResult{
				siteName: j.siteName,
				results:  siteResults,
				batchOK:  batchOK,
				queried:  len(j.hashes),
			}
		}(i, job)
	}
	wg.Wait()

	for _, r := range results {
		if r.siteName == "" {
			continue
		}
		if r.batchOK > 0 {
			cache.queriedSites[r.siteName] = true
		}
		if len(r.results) > 0 {
			cache.bySite[r.siteName] = r.results
		}

		e.logger.Info("pieces_hash batch query completed",
			zap.String("site", r.siteName),
			zap.Int("queried", r.queried),
			zap.Int("matched", len(r.results)))

		if taskID > 0 {
			e.db.WithContext(ctx).Create(&model.ReseedFeatureLog{
				TaskID:  taskID,
				Site:    r.siteName,
				Queried: r.queried,
				Matched: len(r.results),
				Status:  "success",
			})
		}
	}

	return cache
}

func (e *Engine) computeMissingFingerprints(ctx context.Context, sources []sourceTorrent, infoHashes []string) {
	if e.fpRepo == nil || e.clientProvider == nil || len(infoHashes) == 0 {
		return
	}

	existing := e.preloadFingerprints(ctx, infoHashes)

	type missingEntry struct {
		src        sourceTorrent
		clientName string
	}

	clientCache := make(map[string]model.DownloaderClient)
	var missing []missingEntry

	for _, src := range sources {
		if existing.get(src.InfoHash, src.SiteName) != nil {
			continue
		}
		dlClient, ok := clientCache[src.ClientID]
		if !ok {
			var err error
			dlClient, err = e.clientProvider.Get(src.ClientID)
			if err != nil {
				continue
			}
			clientCache[src.ClientID] = dlClient
		}
		missing = append(missing, missingEntry{src: src, clientName: src.ClientID})
	}

	if len(missing) == 0 {
		return
	}

	e.logger.Info("started computing missing fingerprints",
		zap.Int("missing", len(missing)),
		zap.Int("total", len(sources)))

	computed := 0
	for _, m := range missing {
		if ctx.Err() != nil {
			break
		}
		dlClient := clientCache[m.clientName]
		torrentDir := dlClient.GetTorrentDir()
		torrentData, err := clientpkg.ReadTorrentFile(torrentDir, m.src.InfoHash)
		if err != nil {
			if computed == 0 {
				e.logger.Warn("read torrent file failed (first error)",
					zap.String("hash", m.src.InfoHash),
					zap.String("client", m.clientName),
					zap.String("torrent_dir", torrentDir),
					zap.Error(err))
			}
			continue
		}
		if len(torrentData) == 0 {
			continue
		}

		_, err = e.fpRepo.ComputeAndSave(ctx, m.src.SiteName, "", torrentData, m.src.Name)
		if err != nil {
			if computed == 0 {
				e.logger.Warn("fingerprint computation failed (first error)",
					zap.String("hash", m.src.InfoHash),
					zap.Error(err))
			}
			continue
		}
		computed++
	}

	if computed > 0 {
		e.logger.Info("fingerprint computation completed", zap.Int("computed", computed), zap.Int("missing", len(missing)))
	}
}

func (e *Engine) preloadIYUUResults(ctx context.Context, taskID uint, infoHashes []string) map[string][]*model.IYUUReseedResult {
	if e.iyuuService == nil || len(infoHashes) == 0 {
		return nil
	}

	var deduped []string
	seen := make(map[string]bool)
	for _, ih := range infoHashes {
		if !seen[ih] {
			seen[ih] = true
			deduped = append(deduped, ih)
		}
	}

	start := time.Now()
	results, err := e.iyuuService.QueryReseed(ctx, deduped)
	durationMs := int(time.Since(start) / time.Millisecond)

	byHash := make(map[string][]*model.IYUUReseedResult)
	logStatus := "success"
	logMsg := ""
	responseTargets := 0
	if err != nil {
		logStatus = "error"
		logMsg = err.Error()
		e.logger.Warn("IYUU batch query failed", zap.Error(err))
	} else {
		for _, r := range results {
			byHash[r.SourceInfoHash] = append(byHash[r.SourceInfoHash], r)
			responseTargets += len(r.Targets)
		}
	}

	if taskID > 0 {
		log := &model.ReseedIYUULog{
			TaskID:          taskID,
			RequestHashes:   len(deduped),
			ResponseTargets: responseTargets,
			MatchedHashes:   len(byHash),
			Status:          logStatus,
			Message:         logMsg,
			DurationMs:      durationMs,
		}
		if createErr := e.db.WithContext(ctx).Create(log).Error; createErr != nil {
			e.logger.Warn("failed to save IYUU API log", zap.Error(createErr))
		}
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
	// Reset stale "running" tasks from previous process crash/restart
	if result := e.db.WithContext(ctx).Model(&model.ReseedTask{}).
		Where("status = ?", "running").
		Update("status", "idle"); result.Error != nil {
		e.logger.Warn("failed to reset stale running tasks", zap.Error(result.Error))
	} else if result.RowsAffected > 0 {
		e.logger.Info("reset stale running tasks", zap.Int64("count", result.RowsAffected))
	}

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

	stopped := len(e.tasks)
	for id, cancel := range e.tasks {
		cancel()
		delete(e.tasks, id)
	}
	e.mu.Unlock()

	if e.deleteReporter != nil {
		e.deleteReporter.Close()
	}
	if e.contributeReporter != nil {
		e.contributeReporter.Close()
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
		} else if retErr != nil || (result.Failed > 0 && result.Injected == 0 && result.Matched == 0) {
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

	clientNames := e.resolveClientIDsToNames(ctx, task.ClientIDs)
	if len(clientNames) == 0 {
		return result, nil
	}

	if e.clientProvider == nil {
		return result, nil
	}

	e.retryFailedForTask(ctx, task, clientNames)

	// 预加载 seeding_torrent_records 的 InfoHash→TorrentID 映射（关联源站数字种子ID）
	var seedRecords []model.SeedingTorrentRecord
		if err := e.db.WithContext(ctx).Select("info_hash, torrent_id").Find(&seedRecords).Error; err != nil {
		e.logger.Warn("query seed records failed", zap.Error(err))
	}
	seedTorrentIDs := make(map[string]string, len(seedRecords))
	for _, r := range seedRecords {
		if r.InfoHash != "" && r.TorrentID != "" {
			seedTorrentIDs[r.InfoHash] = r.TorrentID
		}
	}

	var sourceTorrents []sourceTorrent
	clientHashes := make(map[string]bool)
	seenSourceNames := make(map[string]bool)
	nameSites := make(map[string]map[string]bool)
	for _, clientName := range clientNames {
		dlClient, err := e.clientProvider.Get(clientName)
		if err != nil {
			e.logger.Warn("failed to get downloader", zap.String("client", clientName), zap.Error(err))
			continue
		}
		allTorrents, err := dlClient.GetAllTorrents(ctx)
		if err != nil {
			e.logger.Warn("failed to get all torrents", zap.String("client", clientName), zap.Error(err))
			continue
		}
		for _, t := range allTorrents {
			clientHashes[t.Hash] = true
			if t.Progress < 1.0 {
				continue
			}
			siteName := ""
			if e.trackerResolver != nil {
				siteName = e.trackerResolver.Resolve(t.TrackerURL)
			}
			if siteName == "" {
				continue
			}
			if t.Name != "" {
				if nameSites[t.Name] == nil {
					nameSites[t.Name] = make(map[string]bool)
				}
				nameSites[t.Name][siteName] = true
				if seenSourceNames[t.Name] {
					continue
				}
				seenSourceNames[t.Name] = true
			}
			sourceTorrents = append(sourceTorrents, sourceTorrent{
				InfoHash:  t.Hash,
				TorrentID: seedTorrentIDs[t.Hash],
				SiteName:  siteName,
				ClientID:  clientName,
				Name:      t.Name,
				SavePath:  t.SavePath,
			})
		}
	}
	result.TotalSources = len(sourceTorrents)

	multiSiteCount := 0
	for _, sites := range nameSites {
		if len(sites) > 1 {
			multiSiteCount++
		}
	}
	e.logger.Debug("reseed source torrent parse completed",
		zap.Int("totalSeeding", result.TotalSources),
		zap.Int("multiSiteNames", multiSiteCount),
		zap.String("taskID", fmt.Sprintf("%d", task.ID)))

	if len(sourceTorrents) == 0 {
		return result, nil
	}

	var sourceSites []string
	if task.SourceSiteIDs != "" {
		sourceSites = e.resolveSiteIDsToNames(ctx, task.SourceSiteIDs)
	}

	targetSites := e.resolveSiteIDsToNames(ctx, task.TargetSiteIDs)

	var excludedSites []string
	if task.TargetSiteExcludes != "" {
		excludedSites = ParseClientIDs(task.TargetSiteExcludes)
	}

	sizeTolerance := task.SizeTolerancePercent
	if sizeTolerance <= 0 {
		sizeTolerance = 1.0
	}

	var infoHashes []string
	for _, src := range sourceTorrents {
		infoHashes = append(infoHashes, src.InfoHash)
	}

	e.computeMissingFingerprints(ctx, sourceTorrents, infoHashes)

	e.logger.Info("preload started",
		zap.Int("sources", len(sourceTorrents)),
		zap.Strings("targetSites", targetSites))

	ps := e.preloadSites(ctx, targetSites, excludedSites)
	fpCache := e.preloadFingerprints(ctx, infoHashes)
	existingMatchesMap, deletedCount := e.preloadExistingMatches(ctx, infoHashes, clientHashes)
	negCache := e.preloadNegativeCache(ctx, infoHashes)

	var phCache *piecesHashCache
	if hasMatchMethod(task.MatchMethods, "pieces_hash") {
		scanConc := task.ScanConcurrency
		if scanConc <= 0 {
			scanConc = 10
		}
		e.preflightCheck(ctx, ps, scanConc)
		phCache = e.preloadPiecesHashCache(ctx, sourceTorrents, ps, fpCache, negCache, scanConc, task.ID)
	}

	dr := buildDomainResolver(ps)
	e.currentDomainResolver = dr
	defer func() { e.currentDomainResolver = nil }()

	cfCache := e.preloadCloudFingerprints(ctx, fpCache, dr)
	e.currentCloudFPCache = cfCache
	defer func() { e.currentCloudFPCache = nil }()

	if phCache != nil && e.contributeReporter != nil && e.cloudFPService != nil && e.cloudFPService.IsEnabled() {
		var contributeRecords []model.CloudFPContribute
		for siteName, hashMap := range phCache.bySite {
			domain := dr.toDomain(siteName)
			for ph, tid := range hashMap {
				contributeRecords = append(contributeRecords, model.CloudFPContribute{
					PiecesHash: ph,
					SiteName:   domain,
					TorrentID:  strconv.Itoa(tid),
				})
			}
		}
		if len(contributeRecords) > 0 {
			e.contributeReporter.Upload(contributeRecords)
		}
	}

	phSites := 0
	if phCache != nil {
		phSites = len(phCache.bySite)
	}
	e.logger.Info("preload completed",
		zap.Int("fpCache", len(fpCache.byKey)),
		zap.Int("existingMatches", len(existingMatchesMap)),
		zap.Int("deletedInClient", deletedCount),
		zap.Int("piecesHashSites", phSites))

	confirmedTargets := make(map[string]bool)
	for _, matches := range existingMatchesMap {
		for _, m := range matches {
			key := m.TargetSite + ":" + m.TargetTorrentID
			confirmedTargets[key] = true
		}
	}

	var iyuuResults map[string][]*model.IYUUReseedResult
	var iyuuSidMap map[int]string
	if task.EngineMode == model.ReseedModeIYUUCloud && e.iyuuService != nil && hasMatchMethod(task.MatchMethods, "iyuu") {
		iyuuResults = e.preloadIYUUResults(ctx, task.ID, infoHashes)
		iyuuSidMap = e.preloadIYUUSiteMappings(ctx)
	}

	l2s := newL2Stats()

	if task.EngineMode == model.ReseedModeSeedFeature {
		e.runSeedFeatureScan(ctx, task, ps, sourceTorrents, sourceSites, fpCache, negCache, phCache, cfCache, l2s, confirmedTargets, nameSites, iyuuResults, iyuuSidMap, targetSites, excludedSites, result)
	} else {
		e.runLegacyScan(ctx, task, ps, sourceTorrents, sourceSites, fpCache, sizeTolerance, negCache, phCache, cfCache, l2s, confirmedTargets, nameSites, iyuuResults, iyuuSidMap, targetSites, excludedSites, result)
	}

	if task.EngineMode == model.ReseedModeSeedFeature {
		l2s.log(e)
	}
	return result, nil
}

type matchConfig struct {
	ctx              context.Context
	ps               *preloadedSites
	fc               *fpCache
	task             *model.ReseedTask
	negCache         map[string]map[string]bool
	phCache          *piecesHashCache
	cfCache          *cloudFPCache
	l2s              *l2Stats
	confirmedTargets map[string]bool
	nameSites        map[string]map[string]bool
}

func (e *Engine) matchAtSite(mc *matchConfig, src sourceTorrent, siteInfo *model.SiteInfo) *model.Candidate {
	if siteInfo.Name == src.SiteName {
		return nil
	}
	if mc.nameSites != nil {
		if sites := mc.nameSites[src.Name]; sites != nil && sites[siteInfo.Name] {
			return nil
		}
	}
	if mc.negCache != nil && mc.negCache[src.InfoHash] != nil && mc.negCache[src.InfoHash][siteInfo.Name] {
		return nil
	}
	siteConfig := mc.ps.configs[siteInfo.Name]
	if siteConfig == nil {
		return nil
	}
	adapter := mc.ps.adapters[siteInfo.Name]
	if adapter == nil {
		return nil
	}
	if httpclient.IsDomainCircuitOpen(siteConfig.Domain) {
		return nil
	}

	if hasMatchMethod(mc.task.MatchMethods, "pieces_hash") {
		c := e.matchLayer0FromCache(src.InfoHash, src.SiteName, siteInfo.Name, mc.fc, mc.phCache)
		if c != nil {
			targetKey := siteInfo.Name + ":" + c.TargetTorrentID
			if mc.confirmedTargets != nil && mc.confirmedTargets[targetKey] {
				return nil
			}
			if !e.verifyL0Size(mc.ctx, adapter, siteConfig, mc.fc.get(src.InfoHash, src.SiteName), c.TargetTorrentID, siteInfo.Name) {
				return nil
			}
			return c
		}
	}

	if hasMatchMethod(mc.task.MatchMethods, "fingerprint") {
		if mc.phCache != nil && mc.phCache.wasQueried(siteInfo.Name) {
		} else {
			c := e.matchLayer1FromCloudCache(src.InfoHash, src.SiteName, siteInfo.Name, mc.fc, mc.cfCache)
			if c != nil {
				return c
			}
		}
	}

	if hasMatchMethod(mc.task.MatchMethods, "size_title") {
		if mc.phCache != nil && mc.phCache.wasQueried(siteInfo.Name) {
		} else {
			c := e.matchLayer2SearchVerify(mc.ctx, adapter, siteConfig, src.InfoHash, src.SiteName, siteInfo.Name, mc.fc, mc.l2s)
			if c != nil {
				return c
			}
		}
	}

	return nil
}

// runSeedFeatureScan implements per-site worker model (Model C):
// each site gets a dedicated goroutine that processes all eligible torrents sequentially.
// Sites run fully in parallel; site-internal rate limiting is natural (one request at a time per site).
func (e *Engine) runSeedFeatureScan(
	ctx context.Context,
	task *model.ReseedTask,
	ps *preloadedSites,
	sourceTorrents []sourceTorrent,
	sourceSites []string,
	fpc *fpCache,
	negCache map[string]map[string]bool,
	phCache *piecesHashCache,
	cfCache *cloudFPCache,
	l2s *l2Stats,
	confirmedTargets map[string]bool,
	nameSites map[string]map[string]bool,
	iyuuResults map[string][]*model.IYUUReseedResult,
	iyuuSidMap map[int]string,
	targetSites []string,
	excludedSites []string,
	result *model.ReseedExecutionResult,
) {
	// Phase 1: Pre-filter eligible torrents (single-threaded)
	var eligible []sourceTorrent
	seenPiecesHashes := make(map[string]bool)
	for _, src := range sourceTorrents {
		if len(sourceSites) > 0 {
			found := false
			for _, s := range sourceSites {
				if src.SiteName == s {
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
		if fp := fpc.get(src.InfoHash, src.SiteName); fp != nil {
			recTitle = fp.Title
		}
		if !e.checkEligibility(ctx, recTitle, task) {
			result.Blocked++
			continue
		}
		if fp := fpc.get(src.InfoHash, src.SiteName); fp != nil && fp.PiecesHash != "" {
			if seenPiecesHashes[fp.PiecesHash] {
				result.Skipped++
				continue
			}
			seenPiecesHashes[fp.PiecesHash] = true
		}
		eligible = append(eligible, src)
	}

	e.logger.Info("seed feature scan started",
		zap.Int("eligible", len(eligible)),
		zap.Int("total", len(sourceTorrents)),
		zap.Int("targetSites", len(ps.infos)))

	if len(eligible) == 0 || ps == nil {
		return
	}

	// Phase 2: Per-site workers
	type siteResult struct {
		src       sourceTorrent
		candidate model.Candidate
		recTitle  string
	}

	resultCh := make(chan siteResult, 200)
	mc := &matchConfig{
		ctx:              ctx,
		ps:               ps,
		fc:               fpc,
		task:             task,
		negCache:         negCache,
		phCache:          phCache,
		cfCache:          cfCache,
		l2s:              l2s,
		confirmedTargets: confirmedTargets,
		nameSites:        nameSites,
	}

	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()

	var siteWg sync.WaitGroup
	for i := range ps.infos {
		siteWg.Add(1)
		go func(siteInfo *model.SiteInfo) {
			defer siteWg.Done()
			defer func() {
				if r := recover(); r != nil {
					e.logger.Error("matchAtSite panic recovered",
						zap.String("site", siteInfo.Name),
						zap.Any("panic", r))
				}
			}()
			for _, src := range eligible {
				if workerCtx.Err() != nil {
					return
				}
				c := e.matchAtSite(mc, src, siteInfo)
				if c != nil {
					var recTitle string
					if fp := fpc.get(src.InfoHash, src.SiteName); fp != nil {
						recTitle = fp.Title
					}
					select {
					case resultCh <- siteResult{src: src, candidate: *c, recTitle: recTitle}:
					case <-workerCtx.Done():
						return
					}
				}
			}
		}(ps.infos[i])
	}

	go func() {
		siteWg.Wait()
		close(resultCh)
	}()

	// Phase 3: Result consumer (single goroutine, no mutex needed)
	injectedTargets := make(map[string]bool)
	matchCount := 0
	for sr := range resultCh {
		if ctx.Err() != nil {
			break
		}

		c := sr.candidate
		targetKey := c.TargetSite + ":" + c.TargetTorrentID
		if confirmedTargets[targetKey] {
			result.DuplicateExists++
			continue
		}
		if injectedTargets[c.TargetTorrentID] {
			continue
		}
		injectedTargets[c.TargetTorrentID] = true

		totalCount := result.Injected + result.Failed + result.Matched
		if totalCount >= task.MaxInjectionsPerRun && task.MaxInjectionsPerRun > 0 {
			e.logger.Info("max injections per run reached, stopping",
				zap.Int("limit", task.MaxInjectionsPerRun))
			workerCancel()
			go func() { drainChannel(resultCh) }()
			break
		}

		if !e.checkEligibility(ctx, sr.recTitle, task) {
			result.Blocked++
			continue
		}

		if sl, ok := ps.siteLimits[c.TargetSite]; ok {
			var limitCount int
			if c.MatchMethod == "iyuu" {
				limitCount = sl.IYUULimitCount
				if limitCount <= 0 {
					limitCount = sl.ReseedLimitCount
				}
			} else {
				limitCount = sl.ReseedLimitCount
			}
			if limitCount > 0 && !e.limiter.checkAndIncr(c.TargetSite, limitCount) {
				e.logger.Debug("site reseed daily limit reached, skipping",
					zap.String("targetSite", c.TargetSite),
					zap.Int("limit", limitCount),
				)
				continue
			}
		}

		decision := model.DecisionMatch
		switch {
		case c.TargetInfoHash == sr.src.InfoHash && c.TargetInfoHash != "":
			decision = model.DecisionSameInfoHash
		case c.MatchMethod == "iyuu":
			decision = model.DecisionMatch
		case c.MatchMethod == "fingerprint" || c.MatchMethod == "cloud_fingerprint":
			decision = model.DecisionMatchPartial
		case c.MatchMethod == "size_title":
			decision = model.DecisionMatchSizeOnly
		}

		match := &model.ReseedMatch{
			TaskID:          task.ID,
			ClientID:        sr.src.ClientID,
			SourceSite:      sr.src.SiteName,
			SourceTorrentID: sr.src.TorrentID,
			SourceInfoHash:  sr.src.InfoHash,
			TargetSite:      c.TargetSite,
			TargetTorrentID: c.TargetTorrentID,
			TargetInfoHash:  c.TargetInfoHash,
			MatchMethod:     c.MatchMethod,
			Confidence:      c.Confidence,
			DecisionType:    string(decision),
			Status:          model.MatchStatusMatched,
		}

		if err := e.SaveMatch(ctx, match); err != nil {
			e.logger.Warn("failed to save match results",
				zap.String("sourceHash", sr.src.InfoHash),
				zap.String("targetSite", c.TargetSite),
				zap.Error(err),
			)
			result.Failed++
			continue
		}

		result.Matched++
		matchCount++
		if matchCount <= 10 || matchCount%500 == 0 {
			e.logger.Info("reseed progress",
				zap.Int("matched", result.Matched),
				zap.Int("duplicates", result.DuplicateExists),
				zap.Int("failed", result.Failed))
		}
	}

	// IYUU fallback for unprocessed torrents
	if iyuuResults != nil {
		for _, src := range eligible {
			if ctx.Err() != nil {
				break
			}
			iyuuCandidates := e.filterIYUUResults(src, iyuuResults, iyuuSidMap, targetSites, excludedSites)
			for _, c := range iyuuCandidates {
				if injectedTargets[c.TargetTorrentID] {
					continue
				}
				injectedTargets[c.TargetTorrentID] = true

				match := &model.ReseedMatch{
					TaskID:          task.ID,
					ClientID:        src.ClientID,
					SourceSite:      src.SiteName,
					SourceTorrentID: src.TorrentID,
					SourceInfoHash:  src.InfoHash,
					TargetSite:      c.TargetSite,
					TargetTorrentID: c.TargetTorrentID,
					TargetInfoHash:  c.TargetInfoHash,
					MatchMethod:     c.MatchMethod,
					Confidence:      c.Confidence,
					DecisionType:    string(model.DecisionMatch),
					Status:          model.MatchStatusMatched,
				}
				if err := e.SaveMatch(ctx, match); err == nil {
					result.Matched++
				}
			}
		}
	}
}

// runLegacyScan preserves the original per-torrent loop for IYUU cloud mode.
func (e *Engine) runLegacyScan(
	ctx context.Context,
	task *model.ReseedTask,
	ps *preloadedSites,
	sourceTorrents []sourceTorrent,
	sourceSites []string,
	fpc *fpCache,
	sizeTolerance float64,
	negCache map[string]map[string]bool,
	phCache *piecesHashCache,
	cfCache *cloudFPCache,
	l2s *l2Stats,
	confirmedTargets map[string]bool,
	nameSites map[string]map[string]bool,
	iyuuResults map[string][]*model.IYUUReseedResult,
	iyuuSidMap map[int]string,
	targetSites []string,
	excludedSites []string,
	result *model.ReseedExecutionResult,
) {
	matchCount := 0
	seenPiecesHashes := make(map[string]bool)
	injectedTargets := make(map[string]bool)
	for _, src := range sourceTorrents {
		if ctx.Err() != nil {
			e.logger.Warn("reseed main loop context canceled", zap.Error(ctx.Err()))
			break
		}

		matchCount++
		if matchCount <= 5 || matchCount%500 == 0 {
			e.logger.Info("reseed progress",
				zap.Int("processed", matchCount),
				zap.Int("total", len(sourceTorrents)),
				zap.Int("matched", result.Matched),
				zap.Int("skipped", result.Skipped))
		}

		if len(sourceSites) > 0 {
			found := false
			for _, s := range sourceSites {
				if src.SiteName == s {
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
		if fp := fpc.get(src.InfoHash, src.SiteName); fp != nil {
			recTitle = fp.Title
		}

		if !e.checkEligibility(ctx, recTitle, task) {
			result.Blocked++
			continue
		}

		if fp := fpc.get(src.InfoHash, src.SiteName); fp != nil && fp.PiecesHash != "" {
			if seenPiecesHashes[fp.PiecesHash] {
				result.Skipped++
				continue
			}
			seenPiecesHashes[fp.PiecesHash] = true
		}

		candidates := e.findCandidates(ctx, src, ps, fpc, sizeTolerance, task, negCache, phCache, cfCache, l2s, confirmedTargets, nameSites)

		if iyuuResults != nil {
			iyuuCandidates := e.filterIYUUResults(src, iyuuResults, iyuuSidMap, targetSites, excludedSites)
			if len(iyuuCandidates) > 0 {
				candidates = append(candidates, iyuuCandidates...)
			}
		}
		if len(candidates) == 0 {
			continue
		}

		for _, c := range candidates {
			targetKey := c.TargetSite + ":" + c.TargetTorrentID
			if confirmedTargets[targetKey] {
				result.DuplicateExists++
				continue
			}
			if injectedTargets[c.TargetTorrentID] {
				continue
			}
			injectedTargets[c.TargetTorrentID] = true

			totalCount := result.Injected + result.Failed + result.Matched
			if totalCount >= task.MaxInjectionsPerRun && task.MaxInjectionsPerRun > 0 {
				break
			}

			if !e.checkEligibility(ctx, recTitle, task) {
				result.Blocked++
				continue
			}

			if sl, ok := ps.siteLimits[c.TargetSite]; ok {
				var limitCount int
				if c.MatchMethod == "iyuu" {
					limitCount = sl.IYUULimitCount
					if limitCount <= 0 {
						limitCount = sl.ReseedLimitCount
					}
				} else {
					limitCount = sl.ReseedLimitCount
				}
				if limitCount > 0 && !e.limiter.checkAndIncr(c.TargetSite, limitCount) {
					e.logger.Debug("site reseed daily limit reached, skipping",
						zap.String("targetSite", c.TargetSite),
						zap.Int("limit", limitCount),
					)
					continue
				}
			}

			decision := model.DecisionMatch
			switch {
			case c.TargetInfoHash == src.InfoHash && c.TargetInfoHash != "":
				decision = model.DecisionSameInfoHash
			case c.MatchMethod == "iyuu":
				decision = model.DecisionMatch
			case c.MatchMethod == "fingerprint" || c.MatchMethod == "cloud_fingerprint":
				decision = model.DecisionMatchPartial
			case c.MatchMethod == "size_title":
				decision = model.DecisionMatchSizeOnly
			}

			match := &model.ReseedMatch{
				TaskID:          task.ID,
				ClientID:        src.ClientID,
				SourceSite:      src.SiteName,
				SourceTorrentID: src.TorrentID,
				SourceInfoHash:  src.InfoHash,
				TargetSite:      c.TargetSite,
				TargetTorrentID: c.TargetTorrentID,
				TargetInfoHash:  c.TargetInfoHash,
				MatchMethod:     c.MatchMethod,
				Confidence:      c.Confidence,
				DecisionType:    string(decision),
				Status:          model.MatchStatusMatched,
			}

			if err := e.SaveMatch(ctx, match); err != nil {
				e.logger.Warn("failed to save match results",
					zap.String("sourceHash", src.InfoHash),
					zap.String("targetSite", c.TargetSite),
					zap.Error(err),
				)
				result.Failed++
				continue
			}

			result.Matched++
		}
	}
}

func (e *Engine) findCandidates(ctx context.Context, src sourceTorrent, ps *preloadedSites, fc *fpCache, sizeTolerance float64, task *model.ReseedTask, negCache map[string]map[string]bool, phCache *piecesHashCache, cfCache *cloudFPCache, l2s *l2Stats, confirmedTargets map[string]bool, nameSites map[string]map[string]bool) []model.Candidate {
	if ps == nil {
		return nil
	}

	matchSingleSite := func(siteInfo *model.SiteInfo) *model.Candidate {
		if siteInfo.Name == src.SiteName {
			return nil
		}
		if nameSites != nil {
			if sites := nameSites[src.Name]; sites != nil && sites[siteInfo.Name] {
				return nil
			}
		}
		if negCache != nil && negCache[src.InfoHash] != nil && negCache[src.InfoHash][siteInfo.Name] {
			return nil
		}
		siteConfig := ps.configs[siteInfo.Name]
		if siteConfig == nil {
			return nil
		}
		adapter := ps.adapters[siteInfo.Name]
		if adapter == nil {
			return nil
		}
		if httpclient.IsDomainCircuitOpen(siteConfig.Domain) {
			return nil
		}

			if hasMatchMethod(task.MatchMethods, "pieces_hash") {
				c := e.matchLayer0FromCache(src.InfoHash, src.SiteName, siteInfo.Name, fc, phCache)
				if c != nil {
					targetKey := siteInfo.Name + ":" + c.TargetTorrentID
					if confirmedTargets != nil && confirmedTargets[targetKey] {
						return nil
					}
					if !e.verifyL0Size(ctx, adapter, siteConfig, fc.get(src.InfoHash, src.SiteName), c.TargetTorrentID, siteInfo.Name) {
						return nil
					}
					return c
				}
			}

		if hasMatchMethod(task.MatchMethods, "fingerprint") {
			if phCache != nil && phCache.wasQueried(siteInfo.Name) {
				// L0 pieces_hash API 已成功查询该站，权威否定，跳过 L1
			} else {
				c := e.matchLayer1FromCloudCache(src.InfoHash, src.SiteName, siteInfo.Name, fc, cfCache)
				if c != nil {
					return c
				}
			}
		}

		if hasMatchMethod(task.MatchMethods, "size_title") {
			if phCache != nil && phCache.wasQueried(siteInfo.Name) {
				// pieces_hash batch query already ran for this site and found no match.
				// Skip L2.
			} else {
				c := e.matchLayer2SearchVerify(ctx, adapter, siteConfig, src.InfoHash, src.SiteName, siteInfo.Name, fc, l2s)
				if c != nil {
					return c
				}
			}
		}

		return nil
	}

	concurrency := task.ScanConcurrency
	if concurrency <= 0 {
		concurrency = 5
	}

	results := make([]*model.Candidate, len(ps.infos))
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)

	for i, si := range ps.infos {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func(idx int, siteInfo *model.SiteInfo) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			defer func() {
				if r := recover(); r != nil {
					e.logger.Error("matchSingleSite panic recovered",
						zap.String("site", siteInfo.Name),
						zap.Any("panic", r))
				}
			}()
			results[idx] = matchSingleSite(siteInfo)
		}(i, si)
	}
	wg.Wait()

	var candidates []model.Candidate
	seenTargets := make(map[string]bool)
	for _, c := range results {
		if c == nil {
			continue
		}
		if !seenTargets[c.TargetTorrentID] {
			seenTargets[c.TargetTorrentID] = true
			candidates = append(candidates, *c)
		}
	}

	return candidates
}

func (e *Engine) matchLayer0FromCache(sourceInfoHash, sourceSiteName, siteName string, fc *fpCache, phCache *piecesHashCache) *model.Candidate {
	if phCache == nil {
		return nil
	}
	fp := fc.get(sourceInfoHash, sourceSiteName)
	if fp == nil || fp.PiecesHash == "" {
		return nil
	}
	torrentID, found := phCache.get(siteName, fp.PiecesHash)
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

func (e *Engine) verifyL0Size(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, fp *model.ContentFingerprint, targetTorrentID string, siteName string) bool {
	if fp == nil || fp.TotalSize == 0 {
		return true
	}
	results, err := adapter.SearchTorrents(ctx, config, targetTorrentID, nil)
	if err != nil {
		e.logger.Warn("L0 size verification search failed, downgrading (fail-closed)",
			zap.String("site", siteName),
			zap.String("torrentID", targetTorrentID),
			zap.Error(err))
		return false
	}
	var targetSize int64
	found := false
	for _, r := range results {
		if r.TorrentID == targetTorrentID {
			targetSize = r.Size
			found = true
			break
		}
	}
	if !found {
		e.logger.Warn("L0 size verification target not in results, downgrading (fail-closed)",
			zap.String("site", siteName),
			zap.String("torrentID", targetTorrentID))
		return false
	}
	if !CompareSizeDisplay(fp.TotalSize, targetSize) {
		e.logger.Warn("L0 pieces_hash hit but size mismatch, downgrading",
			zap.String("site", siteName),
			zap.String("torrentID", targetTorrentID),
			zap.Int64("sourceSize", fp.TotalSize),
			zap.Int64("targetSize", targetSize))
		return false
	}
	return true
}

func (e *Engine) matchLayer0PiecesHash(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, sourceInfoHash, sourceSiteName, siteName string, fc *fpCache) *model.Candidate {
	if !config.SupportsPiecesHashAPI {
		return nil
	}
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

	fp := fc.get(sourceInfoHash, sourceSiteName)
	if fp == nil || fp.PiecesHash == "" {
		return nil
	}

	matches, err := searcher.SearchByPiecesHash(ctx, config, []string{fp.PiecesHash})
	if err != nil {
		e.logger.Debug("Layer0 pieces_hash API failed",
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

func (e *Engine) matchLayer2SizeTitle(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, sourceInfoHash, sourceSiteName, siteName string, sizeTolerance float64, fc *fpCache) *model.Candidate {
	return e.matchLayer2SearchVerify(ctx, adapter, config, sourceInfoHash, sourceSiteName, siteName, fc, nil)
}

func (e *Engine) matchLayer2SearchVerify(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, sourceInfoHash, sourceSiteName, siteName string, fc *fpCache, l2s *l2Stats) *model.Candidate {
	fp := fc.get(sourceInfoHash, sourceSiteName)
	if fp == nil || fp.Title == "" {
		return nil
	}

	isMusic := detectContentType(fp.FileTreeParsed) == "music"

	var keyword, groupName string
	if isMusic {
		keyword = ExtractMusicKeyword(fp.Title)
	} else {
		keyword = ExtractSearchKeyword(fp.Title)
		groupName = ExtractGroupName(fp.Title)

		if (keyword == "" || KeywordStartsWithYear(keyword)) && len(fp.FileTreeParsed) > 0 {
			fileKeyword, fileGroup := extractFromFileTree(fp.FileTreeParsed)
			if fileKeyword != "" && !KeywordStartsWithYear(fileKeyword) {
				e.logger.Debug("extracting keywords from video filename",
					zap.String("title", fp.Title),
					zap.String("originalKeyword", keyword),
					zap.String("fileKeyword", fileKeyword),
					zap.String("fileGroup", fileGroup))
				keyword = fileKeyword
				if fileGroup != "" {
					groupName = fileGroup
				}
			}
		}
	}

	if keyword == "" {
		if l2s != nil {
			l2s.mu.Lock()
			l2s.noKeyword++
			l2s.mu.Unlock()
		}
		return nil
	}

	if !isMusic && groupName == "" {
		if l2s != nil {
			l2s.mu.Lock()
			l2s.noGroup++
			l2s.mu.Unlock()
		}
		return nil
	}

	if isMusic {
		e.logger.Debug("music torrent L2 search",
			zap.String("site", siteName),
			zap.String("title", fp.Title),
			zap.String("keyword", keyword))
	}

	results, err := adapter.SearchTorrents(ctx, config, keyword, nil)
	if err != nil {
		if l2s != nil {
			l2s.record(siteName, "搜索失败")
			l2s.mu.Lock()
			l2s.searchFailed++
			l2s.mu.Unlock()
		}
		return nil
	}

	if len(results) == 0 {
		if l2s != nil {
			l2s.record(siteName, "搜索无结果")
			l2s.mu.Lock()
			l2s.searchEmpty++
			l2s.mu.Unlock()
		}
		return nil
	}

	match, filterStats := VerifyMatchWithTruncationCheck(results, groupName, fp.TotalSize)

	if match != nil {
		if l2s != nil {
			l2s.record(siteName, "命中")
			l2s.mu.Lock()
			l2s.matched++
			l2s.mu.Unlock()
		}
		e.logger.Info("L2 matched",
			zap.String("site", siteName),
			zap.String("keyword", keyword),
			zap.String("groupName", groupName),
			zap.Bool("music", isMusic),
			zap.String("targetTorrentID", match.TorrentID),
			zap.String("targetTitle", match.Title),
			zap.Int64("targetSize", match.Size),
			zap.Int64("sourceSize", fp.TotalSize))
		return &model.Candidate{
			TargetSite:      siteName,
			TargetTorrentID: match.TorrentID,
			Confidence:      0.95,
			MatchMethod:     "search_verify",
		}
	}

	e.logger.Info("L2 search no match",
		zap.String("site", siteName),
		zap.String("keyword", keyword),
		zap.String("groupName", groupName),
		zap.Bool("music", isMusic),
		zap.Int("results", len(results)),
		zap.Int("noTorrentID", filterStats.EmptyID),
		zap.Int("groupMismatch", filterStats.GroupMiss),
		zap.Int("sizeMismatch", filterStats.SizeMiss))
	if l2s != nil {
		reason := fmt.Sprintf("未匹配(group=%d,size=%d)", filterStats.GroupMiss, filterStats.SizeMiss)
		l2s.record(siteName, reason)
		l2s.mu.Lock()
		l2s.groupMismatch += filterStats.GroupMiss
		l2s.sizeMismatch += filterStats.SizeMiss
		l2s.mu.Unlock()
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

var resolutionKeywords = []string{"2160p", "1080p", "1080i", "720p", "576p", "576i", "480p", "480i", "1440p", "4320p", "4k"}

func ExtractSearchKeyword(title string) string {
	if title == "" {
		return ""
	}
	rest := stripChinesePrefix(title)
	if rest == "" {
		// stripChinesePrefix 失败（如 三国.全95集.2010...，分隔符后是中文）
		// 在原始标题上截取年份兜底，产出"剧名 年份"
		rest = truncateToYear(title)
		if rest == "" {
			return ""
		}
	}

	// 剧集检测：stripChinesePrefix 后以 S01/S01E01 等开头 = 中文剧名被跳过了
	// 保留中文剧名用于搜索（NexusPHP 支持中文搜索）
	trimmed := strings.TrimLeft(rest, ". ")
	if seasonPattern.MatchString(trimmed) {
		rest = title // 回退原始标题，保留中文剧名
	}

	raw := truncateToResolution(rest)
	if raw == "" {
		raw = truncateToYear(rest)
	}
	raw = strings.TrimLeft(raw, ".")
	if raw == "" {
		return ""
	}
	raw = strings.ReplaceAll(raw, ".", " ")
	raw = stripBrandColonPrefix(raw)
	// 去掉介质/来源词和地区码
	for _, term := range mediumAndRegionTerms {
		raw = strings.ReplaceAll(raw, term, " ")
	}
	// 去掉中文集数描述（全16集、16集等）
	raw = chineseEpisodeCountRe.ReplaceAllString(raw, " ")
	raw = strings.Join(strings.Fields(raw), " ")
	raw = stripApostrophes(raw)
	if KeywordHasNoTitle(raw) {
		if fb := chineseTitleFallback(title); fb != "" && fb != raw {
			return stripApostrophes(fb)
		}
	}
	return raw
}

// stripApostrophes 剥离直撇号和弯撇号，防止 NexusPHP SQL LIKE 注入式失败。
func stripApostrophes(s string) string {
	s = strings.ReplaceAll(s, "'", " ")
	s = strings.ReplaceAll(s, "\u2019", " ")
	return strings.Join(strings.Fields(s), " ")
}

// chineseTitleFallback 在英文关键词提取失败（hasNoTitle=true）时，
// 用原始标题截到第一个年份，保留中文标题用于 NexusPHP 中文搜索。
// "乾隆王朝.2002.40集全..." → "乾隆王朝 2002"
func chineseTitleFallback(title string) string {
	fb := truncateToYear(title)
	if fb == "" {
		return ""
	}
	fb = strings.ReplaceAll(fb, ".", " ")
	fb = stripBrandColonPrefix(fb)
	fb = chineseEpisodeCountRe.ReplaceAllString(fb, " ")
	fb = strings.Join(strings.Fields(fb), " ")
	return fb
}

// stripBrandColonPrefix 剥离"品牌+中文描述："前缀。
// 当冒号（：或:）前的部分同时含 ASCII 字母和 CJK 字符时（如 "HBO史诗巨著"），
// 判定为频道/品牌描述前缀，返回冒号后的内容。
// 纯 CJK 标题（如 "忍者神龟：变种时代"）不剥离。
func stripBrandColonPrefix(s string) string {
	for _, colon := range []string{"：", ":"} {
		idx := strings.Index(s, colon)
		if idx <= 0 {
			continue
		}
		prefix := s[:idx]
		hasASCII, hasCJK := false, false
		for _, r := range prefix {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				hasASCII = true
			}
			if (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF) {
				hasCJK = true
			}
		}
		if hasASCII && hasCJK {
			return s[idx+len(colon):]
		}
	}
	return s
}

var seasonPattern = regexp.MustCompile(`(?i)^S\d{1,2}(?:E\d{1,3})?`)

var chineseEpisodeCountRe = regexp.MustCompile(`全?\d+集全?`)

var mediumAndRegionTerms = []string{
	// 介质/来源词（变体问题：Blu-ray/Bluray、Web-DL/WebDL）
	"Blu-ray", "Bluray", "Blu ray", "BluRay", "Blue-ray",
	"Web-DL", "WebDL", "Web DL", "WEBRip",
	"HDTV", "UHD", "HD-DVD", "HDDVD",
	"DVDR", "DVDRip", "Remux", "REMUX",
	// 地区码（§56.34 field #6，仅原盘类，非标题内容）
	"GBR", "USA", "JPN", "HKG", "TWN", "KOR", "EUR",
	"CAN", "AUS", "FRA", "GER", "CZE", "NOR", "ITA",
}

func stripChinesePrefix(title string) string {
	i := 0
	inBracket := false
	for i < len(title) {
		r, size := utf8.DecodeRuneInString(title[i:])
		if r == '[' || r == '【' {
			inBracket = true
			i += size
			continue
		}
		if inBracket && (r == ']' || r == '】') {
			inBracket = false
			i += size
			if i < len(title) {
				r2, _ := utf8.DecodeRuneInString(title[i:])
				if r2 == '.' || r2 == ' ' || r2 == ']' || r2 == '】' {
					i++
				}
			}
			continue
		}
		if inBracket {
			i += size
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			// ASCII 紧跟 CJK（无分隔符）：可能是中文标题的续集号/编号（II、5、3D、VIII），
			// 也可能是英文标题开头（Back）。≤3 字符或全大写/纯数字 ≤4 字符视为续集号跳过。
			if i > 0 {
				prevRune, _ := utf8.DecodeLastRuneInString(title[:i])
				if (prevRune >= 0x4E00 && prevRune <= 0x9FFF) || (prevRune >= 0x3400 && prevRune <= 0x4DBF) {
					segLen := 0
					allUpperOrDigit := true
					j := i
					for j < len(title) {
						r2, size2 := utf8.DecodeRuneInString(title[j:])
						if (r2 >= 'a' && r2 <= 'z') || (r2 >= 'A' && r2 <= 'Z') || (r2 >= '0' && r2 <= '9') {
							if r2 >= 'a' && r2 <= 'z' {
								allUpperOrDigit = false
							}
							segLen++
							j += size2
						} else {
							break
						}
					}
					if segLen <= 3 || (allUpperOrDigit && segLen == 4) {
						i = j
						continue
					}
				}
			}
			return title[i:]
		}
		if r == '.' || r == ' ' || r == '+' || r == '-' || r == '_' {
			prev := i
			i += size
			found := false
			for i < len(title) {
				r2, size2 := utf8.DecodeRuneInString(title[i:])
				if (r2 >= 'a' && r2 <= 'z') || (r2 >= 'A' && r2 <= 'Z') || (r2 >= '0' && r2 <= '9') {
					found = true
					break
				}
				if r2 == '.' || r2 == ' ' || r2 == '+' || r2 == '-' || r2 == '_' {
					i += size2
					continue
				}
				break
			}
			if !found {
				return ""
			}
			return title[prev:]
		}
		i += size
	}
	return ""
}

func truncateToResolution(s string) string {
	lower := strings.ToLower(s)
	bestIdx := -1
	bestEnd := 0
	for _, kw := range resolutionKeywords {
		idx := strings.Index(lower, kw)
		if idx >= 0 && (bestIdx < 0 || idx < bestIdx) {
			bestIdx = idx
			bestEnd = idx + len(kw)
		}
	}
	if bestIdx < 0 {
		return ""
	}
	end := bestEnd
	// 3D 检测：标题含 3D 时，延伸到 HSBS/HOU（区分左右半宽和上下半宽）
	if has3D(lower) {
		afterRes := s[bestEnd:]
		afterLower := lower[bestEnd:]
		for _, spec := range []string{"hsbs", "hou"} {
			idx := strings.Index(afterLower, spec)
			if idx >= 0 {
				pos := idx + len(spec)
				// 确认是独立词（前面是 . 或空格）
				if idx == 0 || afterRes[idx-1] == '.' || afterRes[idx-1] == ' ' || afterRes[idx-1] == '-' {
					end = bestEnd + pos
					break
				}
			}
		}
	}
	return s[:end]
}

var re3D = regexp.MustCompile(`(?i)\b3d\b`)

func has3D(lowerS string) bool {
	return re3D.MatchString(lowerS)
}

var yearTruncateRe = regexp.MustCompile(`(?:19|20)\d{2}(?:-(?:19|20)\d{2})?`)

// truncateToYear 截取到第一个年份（含），用于无分辨率的种子名（如 DVDRip）。
// 跳过位置 0 的年份——标题本身以年份开头（如 "2012世界末日.2009..."）时，
// 位置 0 的年份是片名组成部分，不是发布年份分隔符。
func truncateToYear(s string) string {
	searchStart := 0
	for {
		loc := yearTruncateRe.FindStringIndex(s[searchStart:])
		if loc == nil {
			return ""
		}
		absStart := searchStart + loc[0]
		absEnd := searchStart + loc[1]
		if absStart == 0 {
			searchStart = absEnd
			continue
		}
		return s[:absEnd]
	}
}

func ExtractGroupName(title string) string {
	return util.ExtractGroupName(title)
}

type L2MatchResult struct {
	TorrentID string
	Title     string
	Size      int64
}

type MatchFilterStats struct {
	EmptyID   int
	GroupMiss int
	SizeMiss  int
}

func VerifyMatchWithStats(results []*model.SeedingSearchResult, groupName string, sourceSize int64) (*L2MatchResult, *MatchFilterStats) {
	stats := &MatchFilterStats{}
	for _, r := range results {
		if r.TorrentID == "" {
			stats.EmptyID++
			continue
		}
		if groupName != "" && !strings.Contains(r.Title, groupName) {
			stats.GroupMiss++
			continue
		}
		if r.Size <= 0 || !CompareSizeDisplay(sourceSize, r.Size) {
			stats.SizeMiss++
			continue
		}
		return &L2MatchResult{
			TorrentID: r.TorrentID,
			Title:     r.Title,
			Size:      r.Size,
		}, stats
	}
	return nil, stats
}

func VerifyMatch(results []*model.SeedingSearchResult, groupName string, sourceSize int64) *L2MatchResult {
	match, _ := VerifyMatchWithStats(results, groupName, sourceSize)
	return match
}

func VerifyMatchWithTruncationCheck(results []*model.SeedingSearchResult, groupName string, sourceSize int64) (*L2MatchResult, *MatchFilterStats) {
	match, stats := VerifyMatchWithStats(results, groupName, sourceSize)
	if match == nil && groupName != "" {
		needFallback := false
		for _, r := range results {
			if strings.HasSuffix(strings.TrimSpace(r.Title), "..") {
				needFallback = true
				break
			}
		}
		if !needFallback && stats.GroupMiss > 0 {
			needFallback = true
		}
		if needFallback {
			return VerifyMatchWithStats(results, "", sourceSize)
		}
	}
	return match, stats
}

func SearchAndVerifyMatch(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, keyword, groupName string, sourceSize int64) (*L2MatchResult, error) {
	if keyword == "" {
		return nil, nil
	}
	results, err := adapter.SearchTorrents(ctx, config, keyword, nil)
	if err != nil {
		return nil, err
	}
	match, _ := VerifyMatchWithTruncationCheck(results, groupName, sourceSize)
	return match, nil
}

var videoExtensions = []string{".mkv", ".mp4", ".avi", ".ts", ".m2ts", ".wmv", ".flv", ".mov"}

func findMainVideoFile(fileTree map[string]int64) string {
	var bestPath string
	var bestSize int64
	for path, size := range fileTree {
		lower := strings.ToLower(path)
		for _, ext := range videoExtensions {
			if strings.HasSuffix(lower, ext) && size > bestSize {
				bestPath = path
				bestSize = size
				break
			}
		}
	}
	return bestPath
}

func extractFromFileTree(fileTree map[string]int64) (keyword, groupName string) {
	videoFile := findMainVideoFile(fileTree)
	if videoFile == "" {
		return "", ""
	}
	if idx := strings.LastIndex(videoFile, "/"); idx >= 0 {
		videoFile = videoFile[idx+1:]
	}
	keyword = ExtractSearchKeyword(videoFile)
	groupName = ExtractGroupName(videoFile)
	return
}

func KeywordStartsWithYear(keyword string) bool {
	if len(keyword) < 4 {
		return false
	}
	year, err := strconv.Atoi(keyword[:4])
	if err != nil {
		return false
	}
	return year >= 1920 && year <= 2030
}

// KeywordHasNoTitle 判断关键词是否缺少有效标题内容（应跳过 L2，走文件级恢复）。
// 三种情况：
//   - 空关键词
//   - 以年份开头且后续只有规格词（纯中文标题残留，如 "2016 1080p BluRay"）
//   - 第一个词是 1-3 位纯数字（续集编号，如 招魂2 → "2 2016 1080p"）
//
// 以年份开头但后续含 CJK 字符或非规格英文词时不视为"无标题"
// （片名本身以年份开头，如 "2001太空漫游"、"2001 A Space Odyssey"）。
func KeywordHasNoTitle(keyword string) bool {
	if keyword == "" {
		return true
	}
	if KeywordStartsWithYear(keyword) {
		return !hasTitleAfterYear(keyword[4:])
	}
	fields := strings.Fields(keyword)
	if len(fields) > 0 && len(fields[0]) <= 3 {
		if _, err := strconv.Atoi(fields[0]); err == nil {
			return true
		}
	}
	return false
}

// hasTitleAfterYear 检查年份前缀之后是否有有效标题内容：
// CJK 字符或 4+ 字母且非规格术语的英文词。
func hasTitleAfterYear(rest string) bool {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return false
	}
	for _, r := range rest {
		if (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF) {
			return true
		}
	}
	for _, f := range strings.Fields(rest) {
		lower := strings.ToLower(strings.Trim(f, "0123456789"))
		if len(lower) >= 4 && !yearPrefixSpecTerms[lower] {
			return true
		}
	}
	return false
}

var yearPrefixSpecTerms = map[string]bool{
	"bluray": true, "remux": true, "remastered": true,
	"web-dl": true, "webdl": true, "webrip": true,
	"hdtv": true, "dvdrip": true, "hd-dvd": true,
	"atmos": true, "truehd": true,
}

var audioExtensions = []string{".flac", ".wav", ".ape", ".tta", ".wv", ".mp3", ".m4a", ".ogg", ".opus", ".aac", ".dsf", ".dff", ".wma", ".aiff", ".m4b"}

func detectContentType(fileTree map[string]int64) string {
	if len(fileTree) == 0 {
		return "video"
	}
	hasAudio := false
	hasVideo := false
	for path := range fileTree {
		lower := strings.ToLower(path)
		for _, ext := range audioExtensions {
			if strings.HasSuffix(lower, ext) {
				hasAudio = true
			}
		}
		for _, ext := range videoExtensions {
			if strings.HasSuffix(lower, ext) {
				hasVideo = true
			}
		}
	}
	if hasAudio && !hasVideo {
		return "music"
	}
	return "video"
}

// DetectMusicFromDir 读取目录内容，判断是否为音乐资源（有音频文件且无视频文件）。
// 检查直接子文件和一级子目录（CD1/CD2 等分碟结构）。
// SACD ISO 镜像（.iso 无标准音频扩展名）通过目录名含 "SACD" 识别。
func DetectMusicFromDir(dirPath string) bool {
	// SACD（Super Audio CD）是纯音频格式，目录名含 SACD 直接判定为音乐
	if strings.Contains(strings.ToUpper(filepath.Base(dirPath)), "SACD") {
		return true
	}

	hasAudio := detectAudioInDir(dirPath)
	if !hasAudio {
		// 检查一级子目录（CD1/CD2 分碟结构）
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			return false
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if detectAudioInDir(filepath.Join(dirPath, e.Name())) {
				hasAudio = true
				break
			}
		}
	}
	return hasAudio
}

// detectAudioInDir 检查目录内是否有音频文件（无视频文件）。
func detectAudioInDir(dirPath string) bool {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false
	}
	hasAudio := false
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		for _, v := range videoExtensions {
			if ext == v {
				return false
			}
		}
		for _, a := range audioExtensions {
			if ext == a {
				hasAudio = true
			}
		}
	}
	return hasAudio
}

func ExtractMusicKeyword(title string) string {
	if title == "" {
		return ""
	}

	// Scene naming: Artist-Album-24BIT-FLAC-2023-GRP
	if result, ok := musicSceneNaming(title); ok {
		return musicNormalize(result)
	}

	// Freeform naming: Artist - Album Year [Params]
	if keyword, ok := musicFreeformNaming(title); ok {
		keyword = musicStripFormatNoise(keyword)
		return musicNormalize(keyword)
	}

	// Fallback: no ' - ' separator, process brackets and noise
	s := musicStripCurlyBraces(title)
	s = musicProcessSquareBrackets(s)
	s = musicStripDatePrefix(s)
	s = musicStripYearParens(s)
	s = musicStripTrailingFormat(s)
	s = musicStripFormatNoise(s)
	return musicNormalize(s)
}

var musicFreeformDashRe = regexp.MustCompile(`\s+[-–—]\s+`)
var musicAlbumYearRe = regexp.MustCompile(`\s+[\(\[\{]?(19|20)\d{2}[\)\]\}]?`)

// musicFreeformNaming 处理 "Artist - Album Year [Params]" 格式。
// 第一个 ' - '/' – ' 分割 Artist 和 Album；Album 中的年份标记名称/参数分界线。
// 年份保留（区分同名专辑），年份后内容全部丢弃（FLAC/目录号/规格等参数）。
func musicFreeformNaming(title string) (string, bool) {
	prefixYear, s := musicExtractDatePrefix(title)

	dashLoc := musicFreeformDashRe.FindStringIndex(s)
	if dashLoc == nil {
		return "", false
	}
	artist := strings.TrimSpace(s[:dashLoc[0]])
	rest := strings.TrimSpace(s[dashLoc[1]:])
	if artist == "" || rest == "" {
		return "", false
	}

	yearLoc := musicAlbumYearRe.FindStringIndex(rest)

	var album string
	year := prefixYear
	if yearLoc != nil {
		album = musicStripAllBrackets(strings.TrimSpace(rest[:yearLoc[0]]))
		if year == "" {
			year = strings.Trim(rest[yearLoc[0]:yearLoc[1]], " ()[]{}")
		}
	} else {
		album = musicStripAllBrackets(rest)
	}

	if album == "" {
		album = rest
	}

	keyword := album
	if year != "" {
		keyword += " " + year
	}
	return keyword, true
}

// musicExtractDatePrefix 提取日期前缀中的年份。
// "2013 - Artist..." → ("2013", "Artist...")
// "[2021.09.09] Artist..." → ("2021", "Artist...")
func musicExtractDatePrefix(s string) (year, rest string) {
	if m := regexp.MustCompile(`^(\d{4})\s*[-–—]\s+`).FindStringSubmatch(s); m != nil {
		return m[1], s[len(m[0]):]
	}
	if m := regexp.MustCompile(`^\[(\d{4})(?:\.\d{2}(?:\.\d{2})?)?\]\s*`).FindStringSubmatch(s); m != nil {
		return m[1], s[len(m[0]):]
	}
	if m := regexp.MustCompile(`^(\d{4})\.\d{2}\.\d{2}\s*[-–—]?\s*`).FindStringSubmatch(s); m != nil {
		return m[1], s[len(m[0]):]
	}
	return "", s
}

// musicStripAllBrackets 剥离所有括号及其内容。
func musicStripAllBrackets(s string) string {
	for _, pair := range [][2]string{{"(", ")"}, {"[", "]"}, {"{", "}"}} {
		for {
			i := strings.Index(s, pair[0])
			if i < 0 {
				break
			}
			j := strings.Index(s[i:], pair[1])
			if j < 0 {
				break
			}
			s = s[:i] + " " + s[i+j+len(pair[1]):]
		}
	}
	return strings.TrimSpace(s)
}

func musicStripCurlyBraces(s string) string {
	for {
		start := strings.Index(s, "{")
		if start < 0 {
			break
		}
		end := strings.Index(s[start:], "}")
		if end < 0 {
			break
		}
		s = s[:start] + " " + s[start+end+1:]
	}
	return s
}

var musicBracketNoisePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(FLAC|APE|WAV|MP3|DSD|DSF|CUE|SACD|ISO)`),
	regexp.MustCompile(`(?i)(bit|kHz|KHz)`),
	regexp.MustCompile(`^\d+-\d+$`),
	regexp.MustCompile(`(?i)^(EAC|XLD|OpenCD)$`),
	regexp.MustCompile(`(?i)(Genie|KKBOX|Bugs|Tidal|MQA|Spotify)`),
	regexp.MustCompile(`^\d{4}(\.\d{2}(\.\d{2})?)?$|^\d{6}$|^\d{8}$`),
	regexp.MustCompile(`(?i)(版|初回|限定|復刻|Remaster|Edition)`),
	regexp.MustCompile(`(?i)^(Album|Single|EP|Live|Mini)`),
	regexp.MustCompile(`C\d{2}|例大祭|Comiket|ボーマス|M3-`),
	regexp.MustCompile(`^[A-Z]{2,5}[-_]?\d{2,6}`),
	regexp.MustCompile(`(?i)^(US|EU|JP|KR|HK|TW|CN|DE|SE|FI|FR|NL|IT|AU|UK)$`),
}

func musicIsBracketNoise(content string) bool {
	if len([]rune(content)) > 20 {
		return false
	}
	for _, p := range musicBracketNoisePatterns {
		if p.MatchString(content) {
			return true
		}
	}
	return false
}

func musicProcessSquareBrackets(s string) string {
	for _, pair := range [][2]string{{"[", "]"}, {"【", "】"}} {
		open, closeCh := pair[0], pair[1]
		for {
			start := strings.Index(s, open)
			if start < 0 {
				break
			}
			end := strings.Index(s[start:], closeCh)
			if end < 0 {
				break
			}
			content := s[start+len(open) : start+end]
			rest := s[start+end+len(closeCh):]
			if musicIsBracketNoise(content) {
				s = s[:start] + " " + rest
			} else {
				s = s[:start] + " " + content + " " + rest
			}
		}
	}
	return s
}

var musicSceneAnchors = map[string]bool{
	"CD": true, "CDR": true, "CDEP": true, "CDM": true,
	"WEB": true, "WEBFLAC": true, "2CD": true, "3CD": true,
	"DVD": true, "VINYL": true, "12INCH_VINYL": true, "7_INCH_VINYL": true,
	"16BIT": true, "24BIT": true,
	"FLAC": true, "FLACME": true,
	"REMASTERED": true, "REPACK": true, "GOLD": true,
	"BOOTLEG": true, "PROMO": true, "SPLIT": true,
	"SINGLE": true, "EP": true,
}

var musicCountryCodes = map[string]bool{
	"DE": true, "SE": true, "FI": true, "CN": true,
	"US": true, "JP": true, "KR": true, "UK": true,
	"FR": true, "NL": true, "IT": true, "AU": true,
}

var musicYearRe = regexp.MustCompile(`^(19|20)\d{2}$`)
var musicDeluxeRe = regexp.MustCompile(`(?i)^(Deluxe|Limited)`)

func musicIsMetadataAnchor(seg string) bool {
	upper := strings.ToUpper(seg)
	if musicSceneAnchors[upper] || musicCountryCodes[upper] {
		return true
	}
	if strings.HasPrefix(seg, "(") {
		return true
	}
	if musicYearRe.MatchString(seg) || musicDeluxeRe.MatchString(seg) {
		return true
	}
	return false
}

func musicSceneNaming(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if strings.Contains(s, " ") || hasCJKChar(s) || !strings.Contains(s, "-") {
		return s, false
	}
	segs := strings.Split(s, "-")
	var filtered []string
	for _, seg := range segs {
		if seg != "" && seg != "_" {
			filtered = append(filtered, seg)
		}
	}
	anchorIdx := -1
	for i, seg := range filtered {
		if musicIsMetadataAnchor(seg) {
			anchorIdx = i
			break
		}
	}
	if anchorIdx <= 0 {
		return s, false
	}
	keep := filtered[:anchorIdx]
	if len(keep) < 1 {
		return s, false
	}
	return strings.Join(keep, "-"), true
}

var musicDatePrefixPatterns = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`^\d{4}\s*[-–—]\s+`), ""},
	{regexp.MustCompile(`^\d{4}\.\d{2}\.\d{2}\s*[-–—]?\s*`), ""},
	{regexp.MustCompile(`^\d{4}\.\d{2}\s+`), ""},
	{regexp.MustCompile(`^\d{4}\.\s+`), ""},
	{regexp.MustCompile(`^\d{4}-([A-Z])`), "$1"},
}

func musicStripDatePrefix(s string) string {
	for _, p := range musicDatePrefixPatterns {
		s = p.re.ReplaceAllString(s, p.repl)
	}
	return s
}

var musicFYearRe = regexp.MustCompile(`(19|20)\d{2}`)
var musicFFormatRe = regexp.MustCompile(`(?i)^(FLAC|WAV|APE|MP3|DSD|M4A|OGG)$`)
var musicFTypeRe = regexp.MustCompile(`(?i)^(album|EP|Single)$`)

func musicShouldStripParen(content string) bool {
	return musicFYearRe.MatchString(content) ||
		musicFFormatRe.MatchString(content) ||
		musicFTypeRe.MatchString(content)
}

func musicStripYearParens(s string) string {
	for _, pair := range [][2]string{{"(", ")"}, {"（", "）"}} {
		open, closeCh := pair[0], pair[1]
		var buf strings.Builder
		i := 0
		for i < len(s) {
			idx := strings.Index(s[i:], open)
			if idx < 0 {
				buf.WriteString(s[i:])
				break
			}
			closeIdx := strings.Index(s[i+idx+len(open):], closeCh)
			if closeIdx < 0 {
				buf.WriteString(s[i:])
				break
			}
			content := strings.TrimSpace(s[i+idx+len(open) : i+idx+len(open)+closeIdx])
			parenEnd := i + idx + len(open) + closeIdx + len(closeCh)
			buf.WriteString(s[i : i+idx])
			if musicShouldStripParen(content) {
				buf.WriteByte(' ')
			} else {
				buf.WriteString(s[i+idx : parenEnd])
			}
			i = parenEnd
		}
		s = buf.String()
	}
	return s
}

var musicTrailingFormatPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\s+(?:-\s+)?(?:FLAC|Flac|flac)(?:\s*分轨)?$`),
	regexp.MustCompile(`\s+分轨$`),
}

func musicStripTrailingFormat(s string) string {
	for {
		changed := false
		for _, p := range musicTrailingFormatPatterns {
			newS := p.ReplaceAllString(s, "")
			if newS != s {
				s = newS
				changed = true
			}
		}
		s = strings.TrimRight(s, " -")
		if !changed {
			break
		}
	}
	return s
}

var musicFormatNoisePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(FLAC|APE|WAV|MP3|AAC|M4A|OGG|DSD|DSF|CUE)\b`),
	regexp.MustCompile(`(?i)\b\d+bit\b`),
	regexp.MustCompile(`\b\d{1,2}-\d{2,3}\b`), // 24-96, 16-44 (bitdepth-samplerate)
}

func musicStripFormatNoise(s string) string {
	for _, p := range musicFormatNoisePatterns {
		s = p.ReplaceAllString(s, " ")
	}
	return s
}

var musicInvisibleRunes = map[rune]bool{
	'\u200E': true, '\u200B': true, '\u200C': true, '\u200D': true, '\uFEFF': true,
}

var musicSymbolsToSpace = map[rune]bool{
	'(': true, ')': true, '[': true, ']': true, '{': true, '}': true,
	'（': true, '）': true, '【': true, '】': true,
	'《': true, '》': true, '「': true, '」': true,
	'『': true, '』': true, '〈': true, '〉': true,
	':': true, '：': true, ';': true, '；': true,
	'-': true, '–': true, '—': true, '‐': true, '－': true,
	'~': true, '～': true, '〜': true,
	'.': true, '．': true, '。': true,
	',': true, '，': true, '、': true,
	'!': true, '！': true, '?': true, '？': true,
	'+': true, '#': true, '@': true, '$': true, '%': true,
	'/': true, '／': true, '=': true, '＝': true,
	'^': true, '`': true, '\\': true, '<': true, '>': true,
	'|': true, '｜': true, '&': true, '＆': true,
	'…': true, '_': true, '・': true, '·': true, '•': true,
	'*': true, '＊': true,
	'\'': true, '\u2018': true, '\u2019': true,
	'"': true, '\u201C': true, '\u201D': true,
	'†': true, '′': true, '″': true, '→': true,
	'∶': true, '‧': true, '⁄': true,
}

var musicMultiSpaceRe = regexp.MustCompile(`\s+`)

func musicNormalize(s string) string {
	var buf strings.Builder
	for _, r := range s {
		if musicInvisibleRunes[r] {
			continue
		}
		if r >= '\uFF01' && r <= '\uFF5E' {
			r = r - '\uFEE0'
		} else if r == '\u3000' {
			r = ' '
		}
		if musicSymbolsToSpace[r] {
			buf.WriteRune(' ')
		} else {
			buf.WriteRune(r)
		}
	}
	result := musicMultiSpaceRe.ReplaceAllString(buf.String(), " ")
	return strings.TrimSpace(result)
}

func hasCJKChar(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			return true
		}
	}
	return false
}

func CompareSizeDisplay(sourceBytes, resultBytes int64) bool {
	if sourceBytes <= 0 || resultBytes <= 0 {
		return false
	}
	diff := sourceBytes - resultBytes
	if diff < 0 {
		diff = -diff
	}
	// 2% 容差：音乐目录含封面/CUE/LOG 等附属文件，总体积略大于纯种子体积。
	// 视频不同 encode 也可能有微小体积差异。下载器自动校验兜底误匹配。
	return float64(diff)/float64(resultBytes) <= 0.02
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
	return e.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "client_id"},
			{Name: "source_site"},
			{Name: "source_torrent_id"},
			{Name: "target_site"},
			{Name: "target_torrent_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"updated_at", "target_info_hash", "match_method", "confidence",
			"decision_type", "status", "fail_reason",
		}),
	}).Create(matches).Error
}

func (e *Engine) retryFailedForTask(ctx context.Context, task *model.ReseedTask, clientNames []string) {
	maxRetries := task.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	retryInterval := time.Duration(task.RetryIntervalH) * time.Hour
	if retryInterval <= 0 {
		retryInterval = 24 * time.Hour
	}

	var matches []model.ReseedMatch
	err := e.db.WithContext(ctx).
		Where("status = ? AND retry_count < ?", model.MatchStatusFailed, maxRetries).
		Where("client_id IN ?", clientNames).
		Where("next_retry_at IS NULL OR next_retry_at <= ?", time.Now()).
		Order("next_retry_at ASC").
		Limit(50).
		Find(&matches).Error
	if err != nil || len(matches) == 0 {
		return
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
		return
	}

	retried, succeeded := 0, 0
loop:
	for i := range matches {
		if ctx.Err() != nil {
			break
		}
		m := &matches[i]
		if err := e.injectMatch(ctx, m, task, ps); err != nil {
			nextRetry := time.Now().Add(retryInterval)
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
				break loop
			}
		}
	}

	if retried > 0 {
		e.logger.Info("reseed retry completed",
			zap.Uint("task_id", task.ID),
			zap.Int("retried", retried),
			zap.Int("succeeded", succeeded))
	}
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
	return e.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "client_id"},
			{Name: "source_site"},
			{Name: "source_torrent_id"},
			{Name: "target_site"},
			{Name: "target_torrent_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"updated_at", "target_info_hash", "match_method", "confidence",
			"decision_type", "status", "fail_reason",
		}),
	}).Create(match).Error
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
	return e.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "source_info_hash"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"excluded_targets", "last_method", "layer_depth", "expires_at",
		}),
	}).Create(entry).Error
}

func (e *Engine) GetNegativeCacheByHashes(ctx context.Context, hashes []string) ([]model.ReseedNegativeCache, error) {
	if len(hashes) == 0 {
		return nil, nil
	}
	var entries []model.ReseedNegativeCache
	for _, chunk := range chunkStrings(hashes, preloadBatchSize) {
		var partial []model.ReseedNegativeCache
		if err := e.db.WithContext(ctx).
			Where("source_info_hash IN ? AND expires_at > ?", chunk, time.Now()).
			Find(&partial).Error; err != nil {
			return entries, err
		}
		entries = append(entries, partial...)
	}
	return entries, nil
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

	infoHashes := []string{record.InfoHash}
	fpc := e.preloadFingerprints(ctx, infoHashes)
	negCache := e.preloadNegativeCache(ctx, infoHashes)
	existingMatchesMap, _ := e.preloadExistingMatches(ctx, infoHashes, nil)

	confirmedTargets := make(map[string]bool)
	for _, matches := range existingMatchesMap {
		for _, m := range matches {
			key := m.TargetSite + ":" + m.TargetTorrentID
			confirmedTargets[key] = true
		}
	}

	var recTitle string
	if fp := fpc.get(record.InfoHash, record.SiteName); fp != nil {
		recTitle = fp.Title
	}
	if !e.checkEligibility(ctx, recTitle, nil) {
		e.logger.Info("auto reseed: blocked by publish eligibility", zap.String("title", recTitle))
		return
	}

	task := &model.ReseedTask{
		SizeTolerancePercent: 1.0,
		MaxInjectionsPerRun:  10,
		ReseedCategory:       "cross-seed",
	}

	src := sourceTorrent{
		InfoHash: record.InfoHash,
		SiteName: record.SiteName,
		ClientID: record.ClientID,
	}
	candidates := e.findCandidates(ctx, src, ps, fpc, task.SizeTolerancePercent, task, negCache, nil, nil, nil, nil, nil)
	if len(candidates) == 0 {
		e.logger.Debug("auto reseed: no candidates found", zap.String("info_hash", record.InfoHash))
		return
	}

	e.logger.Info("auto reseed: candidates found",
		zap.String("info_hash", record.InfoHash),
		zap.Int("count", len(candidates)))

	for _, c := range candidates {
		targetKey := c.TargetSite + ":" + c.TargetTorrentID
		if confirmedTargets[targetKey] {
			continue
		}
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

func allowedReseedRole(role string) bool {
	return role == "download" || role == "master_reseed" || role == "reseed"
}

func (e *Engine) ValidateClientRoles(ctx context.Context, clientIDs string) error {
	parts := ParseClientIDs(clientIDs)
	if len(parts) == 0 {
		return fmt.Errorf("client_ids 为空")
	}
	uintIDs := partsToUint(parts)
	if len(uintIDs) == 0 {
		return fmt.Errorf("client_ids 格式无效")
	}
	var clients []model.ClientConfig
	if err := e.db.WithContext(ctx).Select("id, name, role").Where("id IN ?", uintIDs).Find(&clients).Error; err != nil {
		return fmt.Errorf("查询下载器失败: %w", err)
	}
	for _, c := range clients {
		if !allowedReseedRole(c.Role) {
			return fmt.Errorf("下载器 %s（ID=%d）角色为 %s，辅种任务仅允许 download 或 master_reseed 角色的下载器", c.Name, c.ID, c.Role)
		}
	}
	return nil
}

func (e *Engine) resolveClientIDsToNames(ctx context.Context, ids string) []string {
	parts := ParseClientIDs(ids)
	if len(parts) == 0 {
		return nil
	}
	uintIDs := partsToUint(parts)
	if len(uintIDs) == 0 {
		return parts
	}
	var clients []model.ClientConfig
	if err := e.db.WithContext(ctx).Select("id, name").Where("id IN ?", uintIDs).Find(&clients).Error; err != nil {
		e.logger.Warn("resolve client IDs to names failed", zap.Error(err))
		return parts
	}
	if len(clients) == 0 {
		return parts
	}
	names := make([]string, 0, len(clients))
	for _, c := range clients {
		names = append(names, c.Name)
	}
	return names
}

func (e *Engine) resolveSiteIDsToNames(ctx context.Context, ids string) []string {
	parts := ParseClientIDs(ids)
	if len(parts) == 0 {
		return nil
	}
	uintIDs := partsToUint(parts)
	if len(uintIDs) == 0 {
		return parts
	}
	var sites []model.Site
	if err := e.db.WithContext(ctx).Select("id, name").Where("id IN ?", uintIDs).Find(&sites).Error; err != nil {
		e.logger.Warn("resolve site IDs to names failed", zap.Error(err))
		return parts
	}
	if len(sites) == 0 {
		return parts
	}
	names := make([]string, 0, len(sites))
	for _, s := range sites {
		names = append(names, s.Name)
	}
	return names
}

func truncHash(h string) string {
	if len(h) > 16 {
		return h[:16]
	}
	return h
}

func partsToUint(parts []string) []uint {
	result := make([]uint, 0, len(parts))
	for _, p := range parts {
		if v, err := strconv.ParseUint(p, 10, 32); err == nil {
			result = append(result, uint(v))
		}
	}
	return result
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

func parseTags(tagsStr string, defaults ...string) []string {
	tags := ParseClientIDs(tagsStr)
	if len(tags) == 0 {
		return defaults
	}
	return tags
}

func (e *Engine) checkEligibility(ctx context.Context, title string, task *model.ReseedTask) bool {
	if e.complianceChecker != nil && task != nil {
		result := e.complianceChecker.CheckWithTask(ctx, title, task)
		if !result.Passed {
			e.logger.Info("compliance blocked",
				zap.String("title", title),
				zap.String("category", result.Category),
				zap.String("reason", result.Reason))
			return false
		}
		return true
	}
	return checkPublishEligibility(title)
}

func checkPublishEligibility(title string) bool {
	if title == "" {
		return true
	}
	for _, kw := range compliance.AdultKeywords {
		if strings.Contains(title, kw) || strings.Contains(strings.ToLower(title), strings.ToLower(kw)) {
			return false
		}
	}
	for _, kw := range compliance.ForbiddenTransferKeywords {
		if strings.Contains(title, kw) {
			return false
		}
	}
	for _, g := range compliance.ForbiddenGroups {
		if strings.Contains(title, g) {
			return false
		}
	}
	return true
}

var errAlreadyExists = errors.New("torrent already exists in downloader")

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

	if httpclient.IsDomainCircuitOpen(targetConfig.Domain) {
		return e.failMatch(ctx, match, fmt.Sprintf("目标站熔断中: %s", match.TargetSite))
	}

	targetAdapter := ps.adapters[match.TargetSite]
	if targetAdapter == nil {
		return e.failMatch(ctx, match, fmt.Sprintf("目标站适配器未预加载: %s", match.TargetSite))
	}

	torrentData, err := targetAdapter.DownloadTorrent(ctx, targetConfig, match.TargetTorrentID)
	if err != nil {
		var appErr *model.AppError
		if errors.As(err, &appErr) && appErr.Code == errAdapterNotFoundCode {
			if e.currentCloudFPCache != nil {
				e.currentCloudFPCache.markDeleted(match.TargetSite, match.TargetTorrentID)
			}
			if e.deleteReporter != nil {
				reportSite := match.TargetSite
				if e.currentDomainResolver != nil {
					reportSite = e.currentDomainResolver.toDomain(match.TargetSite)
				}
				e.deleteReporter.Report(reportSite, match.TargetTorrentID)
			}
		}
		return e.failMatch(ctx, match, fmt.Sprintf("下载目标种子失败: %v", err))
	}

	dlClient, err := e.clientProvider.Get(match.ClientID)
	if err != nil {
		return e.failMatch(ctx, match, fmt.Sprintf("获取下载器客户端失败: %v", err))
	}

	opts := model.AddTorrentOptions{
		Category: task.ReseedCategory,
		Tags:     parseTags(task.ReseedTags, "reseed", "pt-forward"),
		Paused:   true,
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
		e.logger.Debug("reseed torrent add returned exists (error path), verifying existence in downloader",
			zap.Uint("matchID", match.ID))
		return e.verifyDuplicateAndFinish(ctx, dlClient, match, "")
	}
	return e.failMatch(ctx, match, fmt.Sprintf("注入种子到下载器失败: %v", err))
	}

	if addResult.IsDuplicate {
		e.logger.Debug("reseed torrent add returned duplicate, verifying existence in downloader",
			zap.Uint("matchID", match.ID),
			zap.String("hash", addResult.InfoHash))
		return e.verifyDuplicateAndFinish(ctx, dlClient, match, addResult.InfoHash)
	}

	infoHash := addResult.InfoHash
	if infoHash == "" {
		return e.failMatch(ctx, match, "注入后未获取到 InfoHash")
	}

	recheckErr := e.waitForRecheck(ctx, dlClient, infoHash, 120*time.Second)
	if recheckErr != nil {
		_ = dlClient.PauseTorrent(ctx, infoHash)
		return e.failMatch(ctx, match, recheckErr.Error())
	}

	if err := dlClient.ResumeTorrent(ctx, infoHash); err != nil {
		e.logger.Warn("reseed restore seeding failed", zap.String("hash", infoHash), zap.Error(err))
	}

	now := time.Now()
	audit.Log("system", "reseed", "inject", "torrent", match.SourceInfoHash,
		fmt.Sprintf("辅种注入 client=%s %s→%s", match.ClientID, match.SourceSite, match.TargetSite), "success")
	return e.db.WithContext(ctx).Model(match).Updates(map[string]interface{}{
		"status":           model.MatchStatusInjected,
		"target_info_hash": infoHash,
		"injected_at":      &now,
		"updated_at":       now,
		"directory":        opts.SavePath,
	}).Error
}

func (e *Engine) verifyDuplicateAndFinish(ctx context.Context, dlClient model.DownloaderClient, match *model.ReseedMatch, infoHash string) error {
	checkHash := infoHash
	if checkHash == "" {
		checkHash = match.TargetInfoHash
	}
	if checkHash == "" {
		checkHash = match.SourceInfoHash
	}

	if checkHash != "" {
		torrent, err := dlClient.GetTorrentByHash(ctx, checkHash)
		if err != nil {
			e.logger.Warn("failed to query downloader during duplicate verification, allowing",
				zap.Uint("matchID", match.ID),
				zap.String("hash", checkHash),
				zap.Error(err))
		} else if torrent == nil {
			e.logger.Info("reseed torrent marked as duplicate but not in downloader, skipping",
				zap.Uint("matchID", match.ID),
				zap.String("hash", checkHash))
			return e.failMatch(ctx, match, "种子被下载器标记为重复但实际不存在于活动列表")
		}
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":        model.MatchStatusSkipped,
		"injected_at":   &now,
		"decision_type": string(model.DecisionAlreadyExists),
		"updated_at":    now,
	}
	if infoHash != "" {
		updates["target_info_hash"] = infoHash
	}
	if err := e.db.WithContext(ctx).Model(match).Updates(updates).Error; err != nil {
		return err
	}
	return errAlreadyExists
}

func (e *Engine) waitForRecheck(ctx context.Context, dlClient model.DownloaderClient, infoHash string, timeout time.Duration) error {
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
		return fmt.Errorf("数据未通过校验，进度 %.1f%%，状态 %s，请手动检查", ti.Progress*100, ti.State)
	}
	return fmt.Errorf("校验超时（%v），请手动检查", timeout)
}

func (e *Engine) failMatch(ctx context.Context, match *model.ReseedMatch, reason string) error {
	match.RetryCount++
	match.FailReason = reason

	decisionType := model.DecisionDownloadFailed
	switch {
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
		if errors.Is(err, context.Canceled) {
			e.logger.Debug("failMatch update db skipped (context canceled)", zap.Uint("matchID", match.ID))
		} else {
			e.logger.Error("failMatch update db error", zap.Uint("matchID", match.ID), zap.Error(err))
		}
	}

	// 不设负面缓存：瞬态失败（限流/超时/站点故障）允许快速重试。
	// 种子不存在的情况由 pieces_hash API 自然处理——下次查询不会返回已删除的种子。
	// 仅对禁转/独占设置长 TTL 负面缓存。
	if decisionType == model.DecisionBlockedRelease {
		if err := e.SetNegativeCache(ctx, match.SourceSite, match.SourceInfoHash, match.TargetSite, match.MatchMethod, 1, 168*time.Hour); err != nil {
			e.logger.Debug("set negative cache failed", zap.Uint("matchID", match.ID), zap.Error(err))
		}
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

func (e *Engine) filterIYUUResults(src sourceTorrent, iyuuResults map[string][]*model.IYUUReseedResult, sidMap map[int]string, targetSites, excludedSites []string) []model.Candidate {
	results := iyuuResults[src.InfoHash]
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
			if exclSet[siteName] || siteName == src.SiteName {
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

// getOrComputePiecesHash 获取单个种子的 pieces_hash（DB 优先 + ExportTorrent 补全）。
func (e *Engine) getOrComputePiecesHash(ctx context.Context, infoHash string, client model.DownloaderClient) string {
	if e.fpRepo == nil {
		return ""
	}
	fp, err := e.fpRepo.GetByInfoHash(ctx, infoHash)
	if err == nil && fp != nil && fp.PiecesHash != "" {
		return fp.PiecesHash
	}
	if client == nil {
		return ""
	}
	torrentDir := client.GetTorrentDir()
	torrentData, err := clientpkg.ReadTorrentFile(torrentDir, infoHash)
	if err != nil || len(torrentData) == 0 {
		e.logger.Debug("getOrComputePiecesHash: read torrent failed", zap.String("hash", infoHash), zap.Error(err))
		return ""
	}
	fp2, err := e.fpRepo.ComputeAndSave(ctx, "", "", torrentData, "")
	if err != nil || fp2 == nil {
		return ""
	}
	return fp2.PiecesHash
}

// QuerySingleCoverage 查询单个种子的覆盖状态（IYUU + pieces_hash API）。
// 复用辅种引擎的查询方法，结果供覆盖查询写入 site_coverage_cache。
func (e *Engine) QuerySingleCoverage(ctx context.Context, infoHash string, client model.DownloaderClient) []model.CoverageHit {
	var hits []model.CoverageHit

	// IYUU 查询
	if e.iyuuService != nil {
		sidMap := e.preloadIYUUSiteMappings(ctx)
		results, err := e.iyuuService.QueryReseed(ctx, []string{infoHash})
		if err == nil {
			for _, r := range results {
				for _, t := range r.Targets {
					if siteName, ok := sidMap[t.Sid]; ok && siteName != "" {
						hits = append(hits, model.CoverageHit{
							SiteName:  siteName,
							TorrentID: strconv.Itoa(t.TorrentID),
							Source:    "iyuu",
						})
					}
				}
			}
		}
	}

	// pieces_hash 获取
	piecesHash := e.getOrComputePiecesHash(ctx, infoHash, client)
	if piecesHash == "" {
		return hits
	}

	// pieces_hash API 查询（并发）
	if e.siteProvider == nil {
		return hits
	}
	var sites []model.Site
	e.db.WithContext(ctx).Where("enabled = ? AND is_target = ?", true, true).Find(&sites)

	type phResult struct {
		siteName string
		tid      int
		found    bool
	}
	var phResults []phResult
	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, site := range sites {
		adapter, err := e.siteProvider.GetAdapter(ctx, site.Domain)
		if err != nil || adapter == nil || !adapter.SupportsSearchByPiecesHash() {
			continue
		}
		searcher, ok := adapter.(piecesHashSearcher)
		if !ok {
			continue
		}
		config, err := e.siteProvider.GetSiteConfig(ctx, site.Domain)
		if err != nil || config == nil {
			continue
		}

		wg.Add(1)
		go func(sn string, cfg *model.SiteConfig, sr piecesHashSearcher) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			defer func() {
				if r := recover(); r != nil {
					e.logger.Error("QuerySingleCoverage panic recovered", zap.String("site", sn), zap.Any("panic", r))
				}
			}()

			result, err := sr.SearchByPiecesHash(ctx, cfg, []string{piecesHash})
			if err != nil {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if tid, found := result[piecesHash]; found {
				phResults = append(phResults, phResult{siteName: sn, tid: tid, found: true})
			}
		}(site.Name, config, searcher)
	}
	wg.Wait()

	for _, r := range phResults {
		hits = append(hits, model.CoverageHit{
			SiteName:  r.siteName,
			TorrentID: strconv.Itoa(r.tid),
			Source:    "pieces_hash",
		})
	}

	return hits
}

// QueryBatchCoverage 批量查询多个种子的覆盖状态（IYUU 批量 + pieces_hash API 批量）。
// 复用辅种引擎的批量查询逻辑：IYUU 一次传所有 hash（Service 内部 200/批），pieces_hash 批量获取 + 并发站点查询。
func (e *Engine) QueryBatchCoverage(ctx context.Context, infoHashes []string, client model.DownloaderClient) map[string][]model.CoverageHit {
	result := make(map[string][]model.CoverageHit)

	// ① IYUU 批量查询（一次传所有 hash）
	if e.iyuuService != nil && len(infoHashes) > 0 {
		sidMap := e.preloadIYUUSiteMappings(ctx)
		iyuuResults, err := e.iyuuService.QueryReseed(ctx, infoHashes)
		if err == nil {
			for _, r := range iyuuResults {
				for _, t := range r.Targets {
					if siteName, ok := sidMap[t.Sid]; ok && siteName != "" {
						result[r.SourceInfoHash] = append(result[r.SourceInfoHash], model.CoverageHit{
							SiteName:  siteName,
							TorrentID: strconv.Itoa(t.TorrentID),
							Source:    "iyuu",
						})
					}
				}
			}
		}
	}

	// ② pieces_hash 批量获取（DB 优先 + ExportTorrent 补全）
	if e.fpRepo == nil || client == nil {
		return result
	}
	hashToPieces := make(map[string]string, len(infoHashes))
	var missing []string
	for _, ih := range infoHashes {
		fp, err := e.fpRepo.GetByInfoHash(ctx, ih)
		if err == nil && fp != nil && fp.PiecesHash != "" {
			hashToPieces[ih] = fp.PiecesHash
		} else {
			missing = append(missing, ih)
		}
	}
	// 补全缺失指纹（串行 ExportTorrent，但只对缺失的）
	for _, ih := range missing {
		if ctx.Err() != nil {
			break
		}
		ph := e.getOrComputePiecesHash(ctx, ih, client)
		if ph != "" {
			hashToPieces[ih] = ph
		}
	}
	if len(hashToPieces) == 0 {
		return result
	}

	// ③ pieces_hash API 批量查询（并发，每站批量 100/次）
	if e.siteProvider == nil {
		return result
	}
	var sites []model.Site
	e.db.WithContext(ctx).Where("enabled = ? AND is_target = ?", true, true).Find(&sites)

	// 去重 pieces_hash 列表
	allPieces := make([]string, 0, len(hashToPieces))
	seenPieces := make(map[string]bool)
	for _, ph := range hashToPieces {
		if !seenPieces[ph] {
			seenPieces[ph] = true
			allPieces = append(allPieces, ph)
		}
	}

	type siteBatchResult struct {
		siteName string
		matches  map[string]int
	}
	var batchResults []siteBatchResult
	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, site := range sites {
		adapter, err := e.siteProvider.GetAdapter(ctx, site.Domain)
		if err != nil || adapter == nil || !adapter.SupportsSearchByPiecesHash() {
			continue
		}
		searcher, ok := adapter.(piecesHashSearcher)
		if !ok {
			continue
		}
		config, err := e.siteProvider.GetSiteConfig(ctx, site.Domain)
		if err != nil || config == nil {
			continue
		}

		wg.Add(1)
		go func(sn string, cfg *model.SiteConfig, sr piecesHashSearcher) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			defer func() {
				if r := recover(); r != nil {
					e.logger.Error("QueryBatchCoverage panic recovered", zap.String("site", sn), zap.Any("panic", r))
				}
			}()

			siteMatches := make(map[string]int)
			for i := 0; i < len(allPieces); i += 100 {
				end := i + 100
				if end > len(allPieces) {
					end = len(allPieces)
				}
				batch := allPieces[i:end]
				matches, err := sr.SearchByPiecesHash(ctx, cfg, batch)
				if err != nil {
					return
				}
				for ph, tid := range matches {
					siteMatches[ph] = tid
				}
			}
			if len(siteMatches) > 0 {
				mu.Lock()
				batchResults = append(batchResults, siteBatchResult{siteName: sn, matches: siteMatches})
				mu.Unlock()
			}
		}(site.Name, config, searcher)
	}
	wg.Wait()

	// ④ 将 pieces_hash API 结果映射回 infoHash
	for infoHash, ph := range hashToPieces {
		for _, br := range batchResults {
			if tid, found := br.matches[ph]; found {
				result[infoHash] = append(result[infoHash], model.CoverageHit{
					SiteName:  br.siteName,
					TorrentID: strconv.Itoa(tid),
					Source:    "pieces_hash",
				})
			}
		}
	}

	return result
}
