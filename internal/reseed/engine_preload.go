package reseed

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	clientpkg "github.com/ranfish/pt-forward/internal/client"
	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/util"
	"go.uber.org/zap"
)

type preloadedSites struct {
	infos      []*model.SiteInfo
	configs    map[string]*model.SiteConfig
	adapters   map[string]model.SiteAdapter
	siteLimits map[string]*model.Site
}

type siteLimitEntry struct {
	date  string
	count int
}

type siteLimiter struct {
	mu     sync.Mutex
	counts map[string]*siteLimitEntry
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

type l2Stats struct {
	mu           sync.Mutex
	searched     map[string]int
	noKeyword    int
	refuted      int // §59.184 三态反驳聚合（组名/续集/tech/版本/size）
	neutral      int // 中性（不可验证，不再拒绝）
	searchFailed int
	searchEmpty  int
	matched      int
	siteResults  map[string]string
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
		zap.Int("refuted", s.refuted),
		zap.Int("neutral", s.neutral),
		zap.Int("searchFailed", s.searchFailed),
		zap.Int("searchEmpty", s.searchEmpty),
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
	ClientUID uint
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

// taskControl §59.544 修1：任务取消控制柄。原 map[uint]context.CancelFunc
// 无法比较「注册者身份」（Go 的 func 不可 ==），RunTask 收尾无法区分 map
// 里的 cancel 是否仍是自己注册的（误删后来者注册=cancel 断链复发）。
// 存结构体指针即可精确比较。ctx 字段供 stopAll 等场景未来扩展。
type taskControl struct {
	ctx    context.Context
	cancel context.CancelFunc
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

// chunkStrings §59.547 提升至 util.ChunkStrings（api 列表端点跨包消费——
// 「私有跨包先提升公共」纪律）·此处保留薄包装零调用点改动。
func chunkStrings(slice []string, size int) [][]string {
	return util.ChunkStrings(slice, size)
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
				TLSPretend:    siteInfo.TLSPretend, // §59.333
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
				defer func() { _ = resp.Body.Close() }()
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
		siteAdapter := ps.adapters[siteInfo.Name]
		if siteConfig == nil || !supportsPiecesHash(siteConfig, siteAdapter) {
			continue
		}
		if siteConfig.Passkey == "" && siteConfig.Cookie == "" && siteConfig.APIKey == "" {
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
			e.logger.Info("piecesHash preload skip: type assertion失败", zap.String("site", siteInfo.Name))
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

	// §59.25: sitePiecesHashes 按目标站框架选择 pieces_hash 格式
	sitePiecesHashes := make(map[string]map[string]string)
	for _, src := range sources {
		fp := fc.get(src.InfoHash, src.SiteName)
		if fp == nil || fp.PiecesHash == "" {
			continue
		}
		for siteName, eligible := range eligibleSites {
			if siteName == src.SiteName {
				continue
			}
			if negCache != nil && negCache[src.InfoHash] != nil && negCache[src.InfoHash][siteName] {
				continue
			}
			ph := piecesHashForAdapter(fp, eligible.adapter)
			if ph == "" {
				// §59.25: 懒计算——旧指纹缺 PiecesHashBencode，从 .torrent 现算
				if eligible.adapter != nil && eligible.adapter.Framework() == "yemapt" && fp.PiecesHash != "" {
					ph = e.lazyComputeBencodeHash(ctx, src, fp)
				}
				if ph == "" {
					continue
				}
			}
			if sitePiecesHashes[siteName] == nil {
				sitePiecesHashes[siteName] = make(map[string]string)
			}
			sitePiecesHashes[siteName][ph] = src.InfoHash
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
					errStr := err.Error()
					if strings.Contains(errStr, "Too Many Attempts") || strings.Contains(errStr, "rate limit") {
						e.logger.Warn("pieces_hash rate limited, skipping remaining batches",
							zap.String("site", j.siteName),
							zap.Int("batch", k/batchSize+1),
							zap.Int("totalBatches", (len(j.hashes)+batchSize-1)/batchSize),
							zap.Int("completed", batchOK))
						break
					}
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
		src       sourceTorrent
		clientUID uint
	}

	clientCache := make(map[uint]model.DownloaderClient)
	var missing []missingEntry

	for _, src := range sources {
		if existing.get(src.InfoHash, src.SiteName) != nil {
			continue
		}
		if _, ok := clientCache[src.ClientUID]; !ok {
			dl, err := e.clientProvider.Get(src.ClientUID)
			if err != nil {
				continue
			}
			clientCache[src.ClientUID] = dl
		}
		missing = append(missing, missingEntry{src: src, clientUID: src.ClientUID})
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
		dlClient := clientCache[m.clientUID]
		torrentDir := dlClient.GetTorrentDir()
		torrentData, err := clientpkg.ReadTorrentFile(torrentDir, m.src.InfoHash)
		if err != nil {
			if computed == 0 {
				e.logger.Warn("read torrent file failed (first error)",
					zap.String("hash", m.src.InfoHash),
					zap.Uint("client_uid", m.clientUID),
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

// iyuuCoverageWindow §59.535 刀3: 从 iyuu 配置读覆盖任务窗口（调度周期推导）。
// 配置未启用/读取失败返回 false——辅种查询保守放行（不剔除）零行为变化。
func (e *Engine) iyuuCoverageWindow(ctx context.Context) (time.Duration, bool) {
	if e.iyuuService == nil {
		return 0, false
	}
	getCfg, ok := e.iyuuService.(interface {
		CoverageTaskConfig(ctx context.Context) (bool, string, error)
	})
	if !ok {
		return 0, false
	}
	enabled, schedule, err := getCfg.CoverageTaskConfig(ctx)
	if err != nil || !enabled {
		return 0, false
	}
	return model.CoverageWindow(schedule), true
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

	// §59.535 刀3: 共享层配额保护——窗口内已查过的 hash 剔除（它们的上次
	// 匹配在 matches 表·confirmedTargets 已覆盖防重复·新转发站等窗口过期
	// 后下轮发现——「多久查一轮缓存保鲜多久」辅种侧消费同一语义）。
	if len(deduped) > 0 {
		if window, ok := e.iyuuCoverageWindow(ctx); ok {
			var states []model.CoverageQueryState
			if err := e.db.WithContext(ctx).
				Where("info_hash IN ? AND queried_at > ?", deduped, time.Now().Add(-window)).
				Find(&states).Error; err == nil && len(states) > 0 {
				fresh := make(map[string]bool, len(states))
				for _, st := range states {
					fresh[st.InfoHash] = true
				}
				kept := deduped[:0]
				for _, h := range deduped {
					if !fresh[h] {
						kept = append(kept, h)
					}
				}
				if skipped := len(deduped) - len(kept); skipped > 0 {
					e.logger.Info("IYUU 查询共享窗口剔除（新鲜 hash 走缓存）",
						zap.Int("skipped", skipped), zap.Int("remaining", len(kept)),
						zap.Duration("window", window))
				}
				deduped = kept
			}
		}
	}
	if len(deduped) == 0 {
		return nil
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
		// §59.535 联合回归 P2: 查询成功→MarkQueried（窗口有效时）——双通道
		// 共享窗口闭环：辅种问过的 hash 覆盖任务窗口内跳过（此前辅种落库行
		// 但不标 query_state→覆盖任务启用后重问一次=共享层设计初衷漏洞）。
		// 停用时（无窗口）不标——停用态两通道独立无重问问题。
		if w, ok := e.iyuuCoverageWindow(ctx); ok {
			now := time.Now()
			for _, h := range deduped {
				if merr := e.db.WithContext(ctx).Exec(
					`INSERT INTO coverage_query_state (info_hash, queried_at, expires_at) VALUES (?,?,?)
					 ON CONFLICT(info_hash) DO UPDATE SET queried_at=excluded.queried_at, expires_at=excluded.expires_at`,
					h, now, now.Add(w)).Error; merr != nil {
					e.logger.Debug("IYUU mark queried failed", zap.String("hash", h), zap.Error(merr))
				}
			}
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
