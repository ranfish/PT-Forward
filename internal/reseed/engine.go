package reseed

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	dbimpl "github.com/ranfish/pt-forward/internal/db"
	"github.com/ranfish/pt-forward/internal/fingerprint"
	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/scheduler"
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
	if _, ok := s.siteResults[site]; !ok {
		s.siteResults[site] = result
	}
}

func (s *l2Stats) log(e *Engine) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e.logger.Info("L2搜索验证统计",
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
		e.logger.Info("L2站点统计",
			zap.String("site", site),
			zap.Int("searchCount", s.searched[site]),
			zap.String("sampleResult", s.siteResults[site]))
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
	InfoHash string
	SiteName string
	ClientID string
	Name     string
	SavePath string
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
}

func NewEngine(db *gorm.DB, logger *zap.Logger) *Engine {
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

func (e *Engine) SetClientProvider(cp model.DownloaderProvider) {
	e.clientProvider = cp
}

func (e *Engine) SetIYUUService(svc model.IYUUService) {
	e.iyuuService = svc
}

func (e *Engine) SetTrackerResolver(resolver *TrackerSiteResolver) {
	e.trackerResolver = resolver
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
		e.db.WithContext(ctx).Where("name IN ?", siteNames).Find(&sites)
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
				e.logger.Warn("批量预加载指纹失败", zap.Error(err))
				return &fpCache{byKey: make(map[string]*model.ContentFingerprint)}
			}
			fps = append(fps, batch...)
		}
	} else {
		var batch []model.ContentFingerprint
		for _, chunk := range chunkStrings(deduped, preloadBatchSize) {
			var partial []model.ContentFingerprint
			if err := e.db.WithContext(ctx).Where("info_hash IN ?", chunk).Find(&partial).Error; err != nil {
				e.logger.Warn("批量预加载指纹失败(DB)", zap.Error(err))
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
			e.logger.Warn("批量预加载已有匹配失败", zap.Error(err))
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
				e.logger.Warn("站点连接性检测失败(重试后)，已熔断",
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
			e.logger.Warn("站点连接性检测失败(5xx)，已熔断",
				zap.String("site", sc.name),
				zap.String("domain", sc.domain),
				zap.Int("status", status))
			failed++
		}(c)
	}
	wg.Wait()

	e.logger.Info("站点连接性检测完成",
		zap.Int("total", len(checks)),
		zap.Int("failed", failed))
}

func (e *Engine) preloadPiecesHashCache(ctx context.Context, sources []sourceTorrent, ps *preloadedSites, fc *fpCache, negCache map[string]map[string]bool, scanConcurrency int) *piecesHashCache {
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
					e.logger.Error("pieces_hash 查询 panic recovered",
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
					e.logger.Warn("批量 pieces_hash 查询失败",
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

		e.logger.Info("pieces_hash 批量查询完成",
			zap.String("site", r.siteName),
			zap.Int("queried", r.queried),
			zap.Int("matched", len(r.results)))
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

	e.logger.Info("开始计算缺失指纹",
		zap.Int("missing", len(missing)),
		zap.Int("total", len(sources)))

	computed := 0
	for _, m := range missing {
		if ctx.Err() != nil {
			break
		}
		dlClient := clientCache[m.clientName]
		torrentData, err := dlClient.ExportTorrent(ctx, m.src.InfoHash)
		if err != nil {
			if computed == 0 {
				e.logger.Warn("导出种子文件失败（首个错误）",
					zap.String("hash", m.src.InfoHash),
					zap.String("client", m.clientName),
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
				e.logger.Warn("计算指纹失败（首个错误）",
					zap.String("hash", m.src.InfoHash),
					zap.Error(err))
			}
			continue
		}
		computed++
	}

	if computed > 0 {
		e.logger.Info("指纹计算完成", zap.Int("computed", computed), zap.Int("missing", len(missing)))
	}
}

func (e *Engine) preloadIYUUResults(ctx context.Context, infoHashes []string) map[string][]*model.IYUUReseedResult {
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

	results, err := e.iyuuService.QueryReseed(ctx, deduped)
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

	var sourceTorrents []sourceTorrent
	clientHashes := make(map[string]bool)
	seenSourceNames := make(map[string]bool)
	nameSites := make(map[string]map[string]bool)
	for _, clientName := range clientNames {
		dlClient, err := e.clientProvider.Get(clientName)
		if err != nil {
			e.logger.Warn("获取下载器失败", zap.String("client", clientName), zap.Error(err))
			continue
		}
		allTorrents, err := dlClient.GetAllTorrents(ctx)
		if err != nil {
			e.logger.Warn("获取所有种子失败", zap.String("client", clientName), zap.Error(err))
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
				InfoHash: t.Hash,
				SiteName: siteName,
				ClientID: clientName,
				Name:     t.Name,
				SavePath: t.SavePath,
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
	e.logger.Debug("辅种源种子解析完成",
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

	e.logger.Info("preload 开始",
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
		phCache = e.preloadPiecesHashCache(ctx, sourceTorrents, ps, fpCache, negCache, scanConc)
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
	e.logger.Info("preload 完成",
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
		iyuuResults = e.preloadIYUUResults(ctx, infoHashes)
		iyuuSidMap = e.preloadIYUUSiteMappings(ctx)
	}

	l2s := newL2Stats()

	matchCount := 0
	seenPiecesHashes := make(map[string]bool)
	injectedTargets := make(map[string]bool)
	for _, src := range sourceTorrents {
		if ctx.Err() != nil {
			e.logger.Warn("辅种主循环 context 取消", zap.Error(ctx.Err()))
			break
		}

		matchCount++
		if matchCount <= 5 || matchCount%500 == 0 {
			e.logger.Info("辅种进度",
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
		if fp := fpCache.get(src.InfoHash, src.SiteName); fp != nil {
			recTitle = fp.Title
		}

		if !checkPublishEligibility(recTitle) {
			result.Blocked++
			continue
		}

		if fp := fpCache.get(src.InfoHash, src.SiteName); fp != nil && fp.PiecesHash != "" {
			if seenPiecesHashes[fp.PiecesHash] {
				result.Skipped++
				continue
			}
			seenPiecesHashes[fp.PiecesHash] = true
		}

		candidates := e.findCandidates(ctx, src, ps, fpCache, sizeTolerance, task, negCache, phCache, cfCache, l2s, confirmedTargets, nameSites)

		if matchCount <= 10 {
			e.logger.Info("findCandidates 结果",
				zap.Int("idx", matchCount),
				zap.String("src_site", src.SiteName),
				zap.String("hash", truncHash(src.InfoHash)),
				zap.Int("candidates", len(candidates)))
		}

		if iyuuResults != nil {
			iyuuCandidates := e.filterIYUUResults(src, iyuuResults, iyuuSidMap, targetSites, excludedSites)
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
			targetKey := c.TargetSite + ":" + c.TargetTorrentID
			if confirmedTargets[targetKey] {
				resultMu.Lock()
				result.DuplicateExists++
				resultMu.Unlock()
				continue
			}
			if injectedTargets[c.TargetTorrentID] {
				continue
			}
			injectedTargets[c.TargetTorrentID] = true

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
					e.logger.Debug("站点辅种达到每日上限，跳过",
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
				ClientID:        src.ClientID,
				SourceSite:      src.SiteName,
				SourceTorrentID: src.InfoHash,
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
				e.logger.Warn("保存匹配结果失败",
					zap.String("sourceHash", src.InfoHash),
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

				e.logger.Info("injectMatch 开始",
					zap.Uint("matchID", m.ID),
					zap.String("targetSite", m.TargetSite),
					zap.String("targetTorrentID", m.TargetTorrentID))

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

			siteInterval := 0
			if sl, ok := ps.siteLimits[c.TargetSite]; ok {
				if c.MatchMethod == "iyuu" {
					if sl.IYUULimitInterval > 0 {
						siteInterval = sl.IYUULimitInterval
					} else if sl.ReseedLimitInterval > 0 {
						siteInterval = sl.ReseedLimitInterval
					}
				} else if sl.ReseedLimitInterval > 0 {
					siteInterval = sl.ReseedLimitInterval
				}
			}
			effectiveInterval := task.InjectionIntervalS
			if siteInterval > effectiveInterval {
				effectiveInterval = siteInterval
			}
			if effectiveInterval > 0 {
				jitter := 0
				if task.InjectionJitterS > 0 {
					jitter = rand.IntN(task.InjectionJitterS) //nolint:gosec // jitter does not need crypto/rand
				}
				interval := time.Duration(effectiveInterval+jitter) * time.Second
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

	l2s.log(e)
	return result, nil
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
					c = nil
				} else {
					return c
				}
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
		e.logger.Debug("L0 size验证搜索失败，放行",
			zap.String("site", siteName),
			zap.String("torrentID", targetTorrentID),
			zap.Error(err))
		return true
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
		e.logger.Debug("L0 size验证未找到目标种子，放行",
			zap.String("site", siteName),
			zap.String("torrentID", targetTorrentID))
		return true
	}
	if !CompareSizeDisplay(fp.TotalSize, targetSize) {
		e.logger.Warn("L0 pieces_hash命中但size不匹配，降级",
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

		if (keyword == "" || keywordStartsWithYear(keyword)) && len(fp.FileTreeParsed) > 0 {
			fileKeyword, fileGroup := extractFromFileTree(fp.FileTreeParsed)
			if fileKeyword != "" && !keywordStartsWithYear(fileKeyword) {
				e.logger.Debug("从视频文件名提取关键词",
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
		e.logger.Debug("音乐种子L2搜索",
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

	var filteredByGroup, filteredBySize, filteredByEmptyID int
	for _, r := range results {
		if r.TorrentID == "" {
			filteredByEmptyID++
			continue
		}
		if !isMusic && !strings.Contains(r.Title, groupName) {
			filteredByGroup++
			continue
		}
		if !CompareSizeDisplay(fp.TotalSize, r.Size) {
			filteredBySize++
			continue
		}
		if l2s != nil {
			l2s.record(siteName, "命中")
			l2s.mu.Lock()
			l2s.matched++
			l2s.mu.Unlock()
		}
		e.logger.Info("L2命中",
			zap.String("site", siteName),
			zap.String("keyword", keyword),
			zap.String("groupName", groupName),
			zap.Bool("music", isMusic),
			zap.String("targetTorrentID", r.TorrentID),
			zap.String("targetTitle", r.Title),
			zap.Int64("targetSize", r.Size),
			zap.Int64("sourceSize", fp.TotalSize))
		return &model.Candidate{
			TargetSite:      siteName,
			TargetTorrentID: r.TorrentID,
			Confidence:      0.95,
			MatchMethod:     "search_verify",
		}
	}

	e.logger.Info("L2搜索未匹配",
		zap.String("site", siteName),
		zap.String("keyword", keyword),
		zap.String("groupName", groupName),
		zap.Bool("music", isMusic),
		zap.Int("results", len(results)),
		zap.Int("noTorrentID", filteredByEmptyID),
		zap.Int("groupMismatch", filteredByGroup),
		zap.Int("sizeMismatch", filteredBySize))
	if l2s != nil {
		reason := fmt.Sprintf("未匹配(group=%d,size=%d)", filteredByGroup, filteredBySize)
		l2s.record(siteName, reason)
		l2s.mu.Lock()
		l2s.groupMismatch += filteredByGroup
		l2s.sizeMismatch += filteredBySize
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
		return ""
	}
	raw := truncateToResolution(rest)
	raw = strings.TrimLeft(raw, ".")
	if raw == "" {
		return ""
	}
	raw = strings.ReplaceAll(raw, ".", " ")
	raw = strings.Join(strings.Fields(raw), " ")
	return raw
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
	return s[:bestEnd]
}

func ExtractGroupName(title string) string {
	if title == "" {
		return ""
	}
	// Remove file extension
	clean := title
	for _, ext := range []string{".mkv", ".mp4", ".avi", ".ts", ".wmv", ".flv"} {
		if strings.HasSuffix(strings.ToLower(clean), ext) {
			clean = clean[:len(clean)-len(ext)]
			break
		}
	}
	// Find last '-'
	lastDash := strings.LastIndex(clean, "-")
	if lastDash < 0 || lastDash >= len(clean)-1 {
		return ""
	}
	group := clean[lastDash+1:]
	group = strings.TrimSpace(group)
	if group == "" {
		return ""
	}
	return group
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

func keywordStartsWithYear(keyword string) bool {
	if len(keyword) < 4 {
		return false
	}
	year, err := strconv.Atoi(keyword[:4])
	if err != nil {
		return false
	}
	return year >= 1920 && year <= 2030
}

var audioExtensions = []string{".flac", ".wav", ".ape", ".tta", ".wv", ".mp3", ".m4a", ".ogg", ".opus", ".aac", ".dsf", ".dff"}

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

func ExtractMusicKeyword(title string) string {
	if title == "" {
		return ""
	}
	s := musicStripCurlyBraces(title)
	s = musicProcessSquareBrackets(s)

	if result, ok := musicSceneNaming(s); ok {
		return musicNormalize(result)
	}

	s = musicStripDatePrefix(s)
	s = musicStripYearParens(s)
	s = musicStripTrailingFormat(s)
	return musicNormalize(s)
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
				break
			}
		}
	}

	if retried > 0 {
		e.logger.Info("辅种重试完成",
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
	if !checkPublishEligibility(recTitle) {
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
			e.logger.Info("辅种种子添加返回已存在（error 路径），验证下载器中是否真的存在",
				zap.Uint("matchID", match.ID))
			return e.verifyDuplicateAndFinish(ctx, dlClient, match, "")
		}
		return e.failMatch(ctx, match, fmt.Sprintf("注入种子到下载器失败: %v", err))
	}

	if addResult.IsDuplicate {
		e.logger.Info("辅种种子添加返回已存在（duplicate），验证下载器中是否真的存在",
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
		e.logger.Warn("辅种恢复做种失败", zap.String("hash", infoHash), zap.Error(err))
	}

	now := time.Now()
	return e.db.WithContext(ctx).Model(match).Updates(map[string]interface{}{
		"status":           model.MatchStatusInjected,
		"target_info_hash": infoHash,
		"injected_at":      &now,
		"updated_at":       now,
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
			e.logger.Warn("验证重复种子时查询下载器失败，放行",
				zap.Uint("matchID", match.ID),
				zap.String("hash", checkHash),
				zap.Error(err))
		} else if torrent == nil {
			e.logger.Info("辅种种子被标记为重复但下载器中不存在，跳过",
				zap.Uint("matchID", match.ID),
				zap.String("hash", checkHash))
			return e.failMatch(ctx, match, "种子被下载器标记为重复但实际不存在于活动列表")
		}
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":        model.MatchStatusInjected,
		"injected_at":   &now,
		"decision_type": string(model.DecisionAlreadyExists),
		"updated_at":    now,
	}
	if infoHash != "" {
		updates["target_info_hash"] = infoHash
	}
	return e.db.WithContext(ctx).Model(match).Updates(updates).Error
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
		e.logger.Error("failMatch update db error", zap.Uint("matchID", match.ID), zap.Error(err))
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
