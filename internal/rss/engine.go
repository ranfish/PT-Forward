package rss

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ranfish/pt-forward/internal/event"
	"github.com/ranfish/pt-forward/internal/filter"
	"github.com/ranfish/pt-forward/internal/metrics"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	regexSubCache   = make(map[string]*regexp.Regexp)
	regexSubCacheMu sync.RWMutex
	regexSubKeys    []string
)

const regexSubCacheMax = 256

func getOrCompileRegex(pattern string) (*regexp.Regexp, error) {
	regexSubCacheMu.RLock()
	re, ok := regexSubCache[pattern]
	regexSubCacheMu.RUnlock()
	if ok {
		return re, nil
	}
	regexSubCacheMu.Lock()
	defer regexSubCacheMu.Unlock()
	if re, ok := regexSubCache[pattern]; ok {
		return re, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	if len(regexSubCache) >= regexSubCacheMax && len(regexSubKeys) > 0 {
		delete(regexSubCache, regexSubKeys[0])
		regexSubKeys = regexSubKeys[1:]
	}
	regexSubCache[pattern] = re
	regexSubKeys = append(regexSubKeys, pattern)
	return re, nil
}

const defaultFetchInterval = 5 * time.Minute

type detectCacheEntry struct {
	hasHR         bool
	hrSeedTimeH   int
	discountLevel model.DiscountLevel
	isFree        bool
	freeEndAt     *time.Time
	cachedAt      time.Time
}

const detectCacheTTL = 5 * time.Minute

type Engine struct {
	fetcher        *Fetcher
	repo           *Repository
	db             *gorm.DB
	logger         *zap.Logger
	filterEng      *filter.Engine
	dispatcher     *event.Dispatcher
	siteProvider   model.SiteInfoProvider
	clientProvider model.DownloaderProvider
	diskBudget     *DiskBudgetManager
	seedingCounter model.SeedingCollector
	wsBroadcaster  event.WSBroadcaster
	sideLoadMgr    *SideLoadManager
	configBus      *ConfigEventBus
	pushLimiter    *PushLimiter

	detectCache sync.Map

	mu    sync.RWMutex
	tasks map[uint]context.CancelFunc
	wg    sync.WaitGroup

	fetchMu    map[uint]*sync.Mutex
	fetchMuIdx sync.Mutex
}

func NewEngine(db *gorm.DB, logger *zap.Logger) *Engine {
	return &Engine{
		fetcher:    NewFetcher(logger),
		repo:       NewRepository(db),
		db:         db,
		logger:     logger,
		diskBudget: NewDiskBudgetManager(logger),
		tasks:      make(map[uint]context.CancelFunc),
		fetchMu:    make(map[uint]*sync.Mutex),
	}
}

func (e *Engine) getFetchMutex(subID uint) *sync.Mutex {
	e.fetchMuIdx.Lock()
	defer e.fetchMuIdx.Unlock()
	if m, ok := e.fetchMu[subID]; ok {
		return m
	}
	m := &sync.Mutex{}
	e.fetchMu[subID] = m
	return m
}

func (e *Engine) siteHRStrategy(ctx context.Context, siteName string) string {
	if e.siteProvider == nil {
		return "protect"
	}
	config, err := e.siteProvider.GetSiteConfig(ctx, siteName)
	if err != nil || config == nil {
		return "protect"
	}
	if config.HRStrategy == "skip" || config.HRStrategy == "ignore" {
		return config.HRStrategy
	}
	return "protect"
}

func (e *Engine) checkDiskGuard(ctx context.Context, sub *model.RSSSubscription) error {
	if !sub.DiskGuardEnabled || sub.ClientID == "" {
		return nil
	}

	threshold := int64(sub.DiskGuardThreshold)
	if threshold <= 0 {
		return nil
	}

	if e.clientProvider == nil {
		return rssError(ErrRSSDisk, "磁盘守卫：下载器提供者未注入，拒绝放行（fail-closed）", nil)
	}

	dlClient, err := e.clientProvider.Get(sub.ClientID)
	if err != nil {
		return rssError(ErrRSSDisk, "磁盘守卫：获取下载器失败", err)
	}

	md, err := dlClient.GetMainData(ctx)
	if err != nil {
		return rssError(ErrRSSDisk, "磁盘守卫：获取下载器空间信息失败", err)
	}

	if md.FreeSpace < threshold {
		return rssError(ErrRSSDisk, fmt.Sprintf("磁盘守卫拦截: 剩余 %d 字节 < 阈值 %d 字节 (%.2f GB)", md.FreeSpace, threshold, float64(threshold)/1073741824), nil)
	}

	return nil
}

func (e *Engine) checkDiskBudget(ctx context.Context, sub *model.RSSSubscription, size int64) error {
	if e.clientProvider == nil {
		return rssError(ErrRSSDisk, "磁盘预算检查：下载器提供者未注入，拒绝放行（fail-closed）", nil)
	}

	dlClient, err := e.clientProvider.Get(sub.ClientID)
	if err != nil {
		return rssError(ErrRSSDisk, "磁盘预算检查：获取下载器失败", err)
	}

	md, err := dlClient.GetMainData(ctx)
	if err != nil {
		return rssError(ErrRSSDisk, "磁盘预算检查：获取下载器空间信息失败", err)
	}

	freeSpace := md.FreeSpace
	minGB := sub.DiskBudgetMinGB
	minBytes := int64(minGB * 1024 * 1024 * 1024)
	effectiveFree := freeSpace - minBytes - e.diskBudget.ReservedBytes(sub.ClientID)

	if effectiveFree < size {
		return rssError(ErrRSSDisk, fmt.Sprintf("磁盘预算不足: 可用 %d 字节, 需要 %d 字节 (保留 %.1fGB)", effectiveFree, size, minGB), nil)
	}

	ticket, err := e.diskBudget.Reserve(sub.ClientID, size, effectiveFree, 10*time.Minute)
	if err != nil {
		return err
	}

	time.AfterFunc(30*time.Second, func() {
		e.diskBudget.Release(ticket)
	})

	return nil
}

func (e *Engine) SetSiteProvider(sp model.SiteInfoProvider) {
	e.siteProvider = sp
}

func (e *Engine) SetClientProvider(cp model.DownloaderProvider) {
	e.clientProvider = cp
}

func (e *Engine) SetFilterEngine(fe *filter.Engine) {
	e.filterEng = fe
}

func (e *Engine) SetDispatcher(d *event.Dispatcher) {
	e.dispatcher = d
}

func (e *Engine) SetSeedingCounter(sc model.SeedingCollector) {
	e.seedingCounter = sc
}

func (e *Engine) SetWSBroadcaster(b event.WSBroadcaster) {
	e.wsBroadcaster = b
}

func (e *Engine) SetSideLoadManager(mgr *SideLoadManager) {
	e.sideLoadMgr = mgr
}

func (e *Engine) SetConfigEventBus(bus *ConfigEventBus) {
	e.configBus = bus
}

func (e *Engine) SetPushLimiter(limiter *PushLimiter) {
	e.pushLimiter = limiter
}

func (e *Engine) CleanupOldData(ctx context.Context) (int64, error) {
	return e.repo.CleanupOldSeen(ctx, 30)
}

type DryRunResult struct {
	Total    int          `json:"total"`
	Matched  int          `json:"matched"`
	Rejected int          `json:"rejected"`
	Skipped  int          `json:"skipped"`
	Items    []DryRunItem `json:"items"`
}

type DryRunItem struct {
	Title       string `json:"title"`
	TorrentID   string `json:"torrent_id"`
	InfoHash    string `json:"info_hash"`
	Size        int64  `json:"size"`
	Status      string `json:"status"`
	Reason      string `json:"reason,omitempty"`
	MatchedRule string `json:"matched_rule,omitempty"`
}

func (e *Engine) DryRun(ctx context.Context, sub *model.RSSSubscription) (*DryRunResult, error) {
	var site model.Site
	if err := e.db.WithContext(ctx).Where("name = ? OR domain = ?", sub.SiteName, sub.SiteName).First(&site).Error; err != nil {
		return nil, fmt.Errorf("site not found: %w", err)
	}

	result := &DryRunResult{}

	for _, url := range sub.URLs {
		feed, err := e.fetcher.FetchWithProxy(ctx, url, site.ProxyURL, site.SkipSSLVerify)
		if err != nil {
			return nil, fmt.Errorf("fetch RSS: %w", err)
		}

		events := e.fetcher.ParseItems(feed, sub, &site)
		result.Total += len(events)

		for _, ev := range events {
			item := DryRunItem{
				Title:     ev.Title,
				TorrentID: ev.TorrentID,
				InfoHash:  ev.InfoHash,
				Size:      ev.Size,
			}

			seen, err := e.repo.IsSeen(ctx, ev.SiteName, ev.TorrentID)
			if err != nil {
				e.logger.Warn("dryrun seen check failed", zap.Error(err))
			}
			if seen {
				item.Status = "skipped"
				item.Reason = "already_seen"
				result.Skipped++
				result.Items = append(result.Items, item)
				continue
			}

			if ev.MatchedRule != nil && *ev.MatchedRule != "" {
				item.Status = "matched"
				item.MatchedRule = *ev.MatchedRule
				result.Matched++
			} else {
				item.Status = "rejected"
				item.Reason = "no_rule_matched"
				result.Rejected++
			}

			result.Items = append(result.Items, item)
		}
	}

	return result, nil
}

func (e *Engine) needsSideLoading(ctx context.Context, siteName, infoHash string) bool {
	if e.siteProvider == nil {
		return false
	}
	info, err := e.siteProvider.GetSiteInfo(ctx, siteName)
	if err != nil || info == nil {
		return false
	}
	strategy := info.HashStrategy
	if strategy != string(model.HashBencode) && strategy != string(model.HashFakeFromID) {
		return false
	}
	return infoHash == "" || len(infoHash) != 40 || isFakeHash(infoHash)
}

func isFakeHash(h string) bool {
	if len(h) == 0 {
		return false
	}
	if len(h) != 40 {
		return true
	}
	return h[:8] == "fakehash"
}

func (e *Engine) Start(ctx context.Context) error {
	subs, err := e.repo.ListActive(ctx)
	if err != nil {
		return err
	}

	for i := range subs {
		e.startSubscription(ctx, &subs[i])
	}

	e.logger.Info("rss engine started", zap.Int("subscriptions", len(subs)))
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

	e.wg.Wait()
	e.logger.Info("rss engine stopped", zap.Int("stopped_subscriptions", stopped))
}

func (e *Engine) Trigger(ctx context.Context, subID uint) error {
	sub, err := e.repo.GetByID(ctx, subID)
	if err != nil {
		return &model.AppError{Code: 13002, Message: "订阅不存在"}
	}

	if !sub.Enabled {
		return &model.AppError{Code: 13003, Message: "订阅已禁用"}
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer cancel()
		e.logger.Debug("rss trigger: fetchOnce starting",
			zap.String("subscription", sub.Name),
			zap.Uint("id", sub.ID),
			zap.String("siteName", sub.SiteName),
			zap.Int("urlCount", len(sub.URLs)),
		)
		e.fetchOnce(fetchCtx, sub)
		e.logger.Debug("rss trigger: fetchOnce completed",
			zap.String("subscription", sub.Name),
			zap.Uint("id", sub.ID),
		)
	}()
	return nil
}

func (e *Engine) AddSubscription(ctx context.Context, sub *model.RSSSubscription) {
	e.startSubscription(ctx, sub)
}

func (e *Engine) RemoveSubscription(subID uint) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if cancel, ok := e.tasks[subID]; ok {
		cancel()
		delete(e.tasks, subID)
	}
}

func (e *Engine) startSubscription(parentCtx context.Context, sub *model.RSSSubscription) {
	ctx, cancel := context.WithCancel(parentCtx) //nolint:gosec // cancel stored in e.tasks for later invocation
	e.mu.Lock()
	if old, ok := e.tasks[sub.ID]; ok {
		old()
	}
	e.tasks[sub.ID] = cancel
	e.mu.Unlock()

	interval := parseCronInterval(sub.Cron)
	if interval < time.Minute {
		interval = defaultFetchInterval
	}

	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		e.fetchOnce(ctx, sub)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				freshSub, err := e.repo.GetByID(ctx, sub.ID)
				if err != nil {
					continue
				}
				if !freshSub.Enabled || freshSub.Paused {
					continue
				}
				e.fetchOnce(ctx, freshSub)
			}
		}
	}()
}

func (e *Engine) fetchOnce(ctx context.Context, sub *model.RSSSubscription) {
	fetchMu := e.getFetchMutex(sub.ID)
	if !fetchMu.TryLock() {
		e.logger.Debug("subscription fetch already in progress, skipping",
			zap.String("subscription", sub.Name),
			zap.Uint("id", sub.ID))
		return
	}
	defer fetchMu.Unlock()

	if e.pushLimiter != nil && !e.pushLimiter.Allow(fmt.Sprintf("%d", sub.ID)) {
		e.logger.Debug("subscription push rate limited",
			zap.String("subscription", sub.Name),
			zap.Uint("id", sub.ID))
		return
	}

	var site model.Site
	if err := e.db.WithContext(ctx).Where("name = ? OR domain = ?", sub.SiteName, sub.SiteName).First(&site).Error; err != nil {
		e.logger.Warn("site not found for subscription",
			zap.String("subscription", sub.Name),
			zap.String("site", sub.SiteName))
		return
	}

	for _, url := range sub.URLs {
		feed, err := e.fetcher.FetchWithProxy(ctx, url, site.ProxyURL, site.SkipSSLVerify)
		if err != nil {
			errMsg := err.Error()
			if strings.Contains(errMsg, "非 XML 内容") || strings.Contains(errMsg, "请求间隔") {
				e.logger.Debug("rss fetch rate-limited, skipping",
					zap.String("subscription", sub.Name),
					zap.String("url", url),
					zap.Error(err))
			} else {
				e.logger.Warn("rss fetch failed",
					zap.String("subscription", sub.Name),
					zap.String("url", url),
					zap.Error(err))
			}
			e.saveFetchLog(ctx, sub.ID, 0, 0, 0, "error", errMsg)
			continue
		}

		events := e.fetcher.ParseItems(feed, sub, &site)
		e.logger.Info("rss parse result",
			zap.String("subscription", sub.Name),
			zap.Int("events", len(events)))

		metrics.RSSTorrentsFetched.WithLabelValues(sub.SiteName).Add(float64(len(events)))

		var torrentEvents []model.TorrentEvent
		newCount := 0

		type detectItem struct {
			event *model.RSSTorrentEvent
			index int
		}
		var detectItems []detectItem

		for i, event := range events {
			isSeen, seenErr := e.repo.IsSeen(ctx, event.SiteName, event.TorrentID)
			if seenErr != nil {
				e.logger.Warn("check seen status failed, skipping torrent",
					zap.String("site", event.SiteName),
					zap.String("torrent", event.TorrentID),
					zap.Error(seenErr),
				)
				continue
			}
			if isSeen {
				e.logger.Debug("torrent already seen, skipping",
					zap.String("site", event.SiteName),
					zap.String("torrent", event.TorrentID))
				metrics.RSSTorrentsFiltered.WithLabelValues(sub.SiteName, "already_seen").Inc()
				continue
			}

			if sub.DiskGuardEnabled && sub.ClientID != "" && e.clientProvider != nil {
				if err := e.checkDiskGuard(ctx, sub); err != nil {
					e.logger.Warn("torrent skipped by disk guard",
						zap.String("torrent", event.TorrentID),
						zap.Int64("size", event.Size),
						zap.Error(err))
					metrics.RSSTorrentsFiltered.WithLabelValues(sub.SiteName, "disk_guard").Inc()
					continue
				}
			}

			if sub.DiskBudgetEnabled && sub.ClientID != "" && e.clientProvider != nil {
				if err := e.checkDiskBudget(ctx, sub, event.Size); err != nil {
					e.logger.Warn("torrent skipped by disk budget",
						zap.String("torrent", event.TorrentID),
						zap.Int64("size", event.Size),
						zap.Error(err))
					metrics.RSSTorrentsFiltered.WithLabelValues(sub.SiteName, "disk_budget").Inc()
					continue
				}
			}

			detectItems = append(detectItems, detectItem{event: event, index: i})
		}

		if len(detectItems) > 0 {
			const maxConcurrentDetect = 2
			sem := make(chan struct{}, maxConcurrentDetect)
			var wg sync.WaitGroup
			var slowCount int64

			for _, item := range detectItems {
				wg.Add(1)
				sem <- struct{}{}
				go func(it detectItem) {
					defer wg.Done()
					defer func() { <-sem }()
					t0 := time.Now()
					e.detectHRAndDiscount(ctx, it.event, site.Domain)
					if dur := time.Since(t0); dur > 2*time.Second {
						atomic.AddInt64(&slowCount, 1)
						e.logger.Warn("detectHRAndDiscount slow",
							zap.String("subscription", sub.Name),
							zap.String("torrent", it.event.TorrentID),
							zap.Duration("duration", dur))
					}
				}(item)
			}
			wg.Wait()

			if slowCount > 0 {
				e.logger.Info("detectHRAndDiscount batch summary",
					zap.String("subscription", sub.Name),
					zap.Int("total", len(detectItems)),
					zap.Int64("slow", slowCount))
			}
		}

		for _, item := range detectItems {
			event := item.event

			if sub.ScrapeFree && !event.IsFree {
				e.logger.Debug("torrent skipped: not free",
					zap.String("torrent", event.TorrentID),
					zap.Bool("free", event.IsFree))
				metrics.RSSTorrentsFiltered.WithLabelValues(sub.SiteName, "not_free").Inc()
				continue
			}

			if event.HasHR && e.siteHRStrategy(ctx, sub.SiteName) == "skip" {
				e.logger.Debug("torrent skipped by HR skip strategy",
					zap.String("torrent", event.TorrentID),
					zap.String("site", sub.SiteName))
				metrics.RSSTorrentsFiltered.WithLabelValues(sub.SiteName, "hr_skip").Inc()
				continue
			}

			discountStr := string(event.DiscountLevel)
			if discountStr == "" {
				if event.IsFree {
					discountStr = string(model.DiscountFree)
				} else {
					discountStr = string(model.DiscountNone)
				}
			}

			if len(sub.Conditions) > 0 {
				evalCtx := newEvalCtx(event, discountStr)
				allMatch := true
				for _, cond := range sub.Conditions {
					if !filter.MatchConditionExport(cond, evalCtx) {
						allMatch = false
						break
					}
				}
				if !allMatch {
					e.logger.Debug("torrent skipped by subscription conditions",
						zap.String("torrent", event.TorrentID))
					metrics.RSSTorrentsFiltered.WithLabelValues(sub.SiteName, "subscription_condition").Inc()
					continue
				}
			}

			if e.filterEng != nil && (len(sub.AcceptRuleIDs) > 0 || len(sub.RejectRuleIDs) > 0) {
				evalCtx := newEvalCtx(event, discountStr)
				if len(sub.RejectRuleIDs) > 0 {
					rejectResult, err := e.filterEng.MatchByIDs(ctx, sub.RejectRuleIDs, evalCtx)
					if err != nil {
						e.logger.Warn("reject rule match failed, skipping torrent (fail-closed)",
							zap.String("torrent", event.TorrentID),
							zap.Error(err))
						metrics.RSSTorrentsFiltered.WithLabelValues(sub.SiteName, "reject_rule_error").Inc()
						continue
					} else if rejectResult.Matched {
						e.logger.Debug("torrent rejected by subscription reject rules",
							zap.String("torrent", event.TorrentID),
							zap.String("rule", rejectResult.RuleName))
						metrics.RSSTorrentsFiltered.WithLabelValues(sub.SiteName, "subscription_reject_rule").Inc()
						continue
					}
				}
				if len(sub.AcceptRuleIDs) > 0 {
					acceptResult, err := e.filterEng.MatchByIDs(ctx, sub.AcceptRuleIDs, evalCtx)
					switch {
					case err != nil:
						e.logger.Warn("accept rule match failed, skipping torrent (fail-closed)",
							zap.String("torrent", event.TorrentID),
							zap.Error(err))
						metrics.RSSTorrentsFiltered.WithLabelValues(sub.SiteName, "accept_rule_error").Inc()
						continue
					case !acceptResult.Matched:
						e.logger.Debug("torrent skipped: no accept rule matched",
							zap.String("torrent", event.TorrentID))
						metrics.RSSTorrentsFiltered.WithLabelValues(sub.SiteName, "no_accept_rule").Inc()
						continue
					default:
						if acceptResult.SavePath != "" && sub.SavePath == "" {
							event.Metadata["filter_save_path"] = acceptResult.SavePath
						}
						if acceptResult.Category != "" && sub.Category == "" {
							event.Metadata["filter_category"] = acceptResult.Category
						}
						if acceptResult.Tags != "" && len(sub.Tags) == 0 {
							event.Metadata["filter_tags"] = acceptResult.Tags
						}
					}
				}
			}

			if e.filterEng != nil {
				matchResult, err := e.filterEng.Match(ctx, newEvalCtx(event, discountStr))
				if err != nil {
					e.logger.Warn("global filter match failed, skipping torrent (fail-closed)",
						zap.String("torrent", event.TorrentID),
						zap.Error(err))
					metrics.RSSTorrentsFiltered.WithLabelValues(sub.SiteName, "global_filter_error").Inc()
					continue
				}
				if matchResult.Matched && matchResult.Reject {
					e.logger.Debug("torrent rejected by global exclusion rule",
						zap.String("torrent", event.TorrentID),
						zap.String("rule", matchResult.RuleName))
					metrics.RSSTorrentsFiltered.WithLabelValues(sub.SiteName, "global_rule_reject").Inc()
					continue
				}
			}

			if sub.UseCustomRegex && sub.RegexStr != "" {
				re, err := getOrCompileRegex(sub.RegexStr)
				if err != nil {
					e.logger.Warn("invalid custom regex",
						zap.String("subscription", sub.Name),
						zap.String("regex", sub.RegexStr),
						zap.Error(err))
				} else {
					event.Title = re.ReplaceAllString(event.Title, sub.ReplaceStr)
				}
			}

			seen := &model.RSSTorrentSeen{
				SiteName:       event.SiteName,
				TorrentID:      event.TorrentID,
				SubscriptionID: uintToString(sub.ID),
				InfoHash:       event.InfoHash,
				IsFakeHash:     isFakeHash(event.InfoHash) || event.InfoHash == "" || len(event.InfoHash) != 40,
				Title:          event.Title,
				Size:           event.Size,
				IsFree:         event.IsFree,
				FreeLevel:      string(event.DiscountLevel),
				Discount:       model.DiscountLevel(discountStr),
				HasHR:          event.HasHR,
				HRSeedTimeH:    event.HRSeedTimeH,
				FreeEndAt:      event.FreeEndAt,
				Status:         "seen",
			}
			if err := e.repo.MarkSeen(ctx, seen); err != nil {
				e.logger.Warn("mark seen failed",
					zap.String("torrent", event.TorrentID),
					zap.Error(err))
				continue
			}

			newCount++

			te := model.TorrentEvent{
				SourceID:        uintToString(sub.ID),
				SiteName:        event.SiteName,
				TorrentID:       event.TorrentID,
				Title:           event.Title,
				DownloadURL:     event.DownloadURL,
				Size:            event.Size,
				InfoHash:        event.InfoHash,
				Discount:        model.DiscountNone,
				HasHR:           event.HasHR,
				MatchedRuleName: derefStr(event.MatchedRule),
				Metadata:        event.Metadata,
			}
			if event.DiscountLevel != "" {
				te.Discount = event.DiscountLevel
			} else if event.IsFree {
				te.Discount = model.DiscountFree
			}
			if event.FreeEndAt != nil {
				te.FreeEndAt = event.FreeEndAt
			}
			if event.HasHR {
				te.HRSeedTimeH = event.HRSeedTimeH
				if te.HRSeedTimeH == 0 {
					te.HRSeedTimeH = 72
				}
			}

			if e.needsSideLoading(ctx, event.SiteName, te.InfoHash) {
				te.RequiresSideLoading = true
				te.SideLoadStatus = model.SideLoadPending
				if e.sideLoadMgr != nil {
					if err := e.sideLoadMgr.Enqueue(&te, event.SiteName); err != nil {
						e.logger.Warn("side load enqueue failed, dispatching directly",
							zap.String("site", event.SiteName),
							zap.String("torrent_id", event.TorrentID),
							zap.Error(err))
						torrentEvents = append(torrentEvents, te)
					}
					continue
				}
			}

			torrentEvents = append(torrentEvents, te)

			e.logger.Debug("new torrent from RSS",
				zap.String("subscription", sub.Name),
				zap.String("title", event.Title),
				zap.String("torrentID", event.TorrentID),
				zap.String("matchedRule", derefStr(event.MatchedRule)))
		}

		if len(torrentEvents) > 0 && e.dispatcher != nil {
			if err := e.dispatcher.Dispatch(ctx, "rss_new", torrentEvents); err != nil {
				e.logger.Warn("dispatch events failed",
					zap.String("subscription", sub.Name),
					zap.Error(err))
			}
		}

		if e.seedingCounter != nil && sub.Enabled && sub.ClientID != "" {
			for i := range torrentEvents {
				if err := e.seedingCounter.Add(ctx, sub.ClientID, &torrentEvents[i]); err != nil {
					e.logger.Debug("seeding counter add failed",
						zap.String("torrent_id", torrentEvents[i].TorrentID),
						zap.Error(err))
				} else {
					e.repo.MarkStatus(ctx, torrentEvents[i].SiteName, torrentEvents[i].TorrentID, "pushed")
				}
			}
			subIDStr := uintToString(sub.ID)
			if _, flushErr := e.seedingCounter.Flush(ctx, subIDStr); flushErr != nil {
				e.logger.Warn("seeding counter flush failed",
					zap.String("subscription", sub.Name),
					zap.Error(flushErr))
			}
		}

		if newCount > 0 {
			e.logger.Info("rss fetch completed",
				zap.String("subscription", sub.Name),
				zap.Int("total", len(events)),
				zap.Int("new", newCount),
				zap.Int("dispatched", len(torrentEvents)))
		} else {
			e.logger.Debug("rss fetch completed: no new torrents",
				zap.String("subscription", sub.Name),
				zap.Int("total", len(events)))
		}
		e.saveFetchLog(ctx, sub.ID, len(events), newCount, len(torrentEvents), "ok", "")
	}
}

func (e *Engine) saveFetchLog(ctx context.Context, subID uint, total, newCount, dispatched int, status, errMsg string) {
	log := &model.RSSFetchLog{
		SubscriptionID: fmt.Sprintf("%d", subID),
		Total:          total,
		NewCount:       newCount,
		Dispatched:     dispatched,
		Status:         status,
		ErrorMsg:       errMsg,
	}
	if err := e.db.WithContext(ctx).Create(log).Error; err != nil {
		e.logger.Debug("save fetch log failed", zap.Error(err))
	}
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func newEvalCtx(event *model.RSSTorrentEvent, discountStr string) *filter.EvalContext {
	return &filter.EvalContext{
		Title:         event.Title,
		Size:          event.Size,
		SiteName:      event.SiteName,
		Free:          event.IsFree,
		DiscountLevel: discountStr,
		Uploader:      event.Uploader,
		Category:      event.Category,
		Tags:          event.Tags,
	}
}

func parseCronInterval(cron string) time.Duration {
	if cron == "" {
		return defaultFetchInterval
	}

	if parts := splitCron(cron); len(parts) == 5 {
		minuteField := parts[0]
		if len(minuteField) > 2 && minuteField[0] == '*' && minuteField[1] == '/' {
			var mins int
			if _, err := fmt.Sscanf(minuteField, "*/%d", &mins); err == nil && mins > 0 {
				return time.Duration(mins) * time.Minute
			}
		}
		if minuteField == "*" {
			return time.Minute
		}
	}

	return defaultFetchInterval
}

func splitCron(cron string) []string {
	var parts []string
	current := ""
	for _, c := range cron {
		if c == ' ' || c == '\t' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func uintToString(n uint) string {
	return fmt.Sprintf("%d", n)
}

const perTorrentDetectTimeout = 8 * time.Second

func (e *Engine) detectHRAndDiscount(ctx context.Context, event *model.RSSTorrentEvent, siteName string) {
	if e.siteProvider == nil || event.TorrentID == "" {
		return
	}

	cacheKey := siteName + ":" + event.TorrentID
	if cached, ok := e.detectCache.Load(cacheKey); ok {
		entry := cached.(*detectCacheEntry)
		if time.Since(entry.cachedAt) < detectCacheTTL {
			event.HasHR = entry.hasHR
			event.HRSeedTimeH = entry.hrSeedTimeH
			event.DiscountLevel = entry.discountLevel
			event.IsFree = entry.isFree
			event.FreeEndAt = entry.freeEndAt
			return
		}
		e.detectCache.Delete(cacheKey)
	}

	detectCtx, detectCancel := context.WithTimeout(ctx, perTorrentDetectTimeout)
	defer detectCancel()

	adapter, err := e.siteProvider.GetAdapter(detectCtx, siteName)
	if err != nil {
		e.logger.Warn("获取适配器失败，跳过HR/折扣检测", zap.String("site", siteName), zap.Error(err))
		return
	}

	config, err := e.siteProvider.GetSiteConfig(detectCtx, siteName)
	if err != nil {
		e.logger.Warn("获取站点配置失败，跳过HR/折扣检测", zap.String("site", siteName), zap.Error(err))
		return
	}

	if combined, ok := adapter.(model.CombinedHRDiscountDetector); ok {
		hrResult, discResult, err := combined.DetectHRAndDiscount(detectCtx, config, event.TorrentID)
		if err != nil {
			e.logger.Debug("combined detect failed", zap.String("site", siteName), zap.String("torrent", event.TorrentID), zap.Error(err))
			return
		}
		if hrResult != nil {
			event.HasHR = hrResult.HasHR
			if event.HasHR {
				event.HRSeedTimeH = hrResult.SeedTimeH
				if event.HRSeedTimeH == 0 {
					event.HRSeedTimeH = 72
				}
			}
		}
		if discResult != nil && discResult.Level != model.DiscountNone {
			event.DiscountLevel = discResult.Level
			event.IsFree = discResult.Level == model.DiscountFree ||
				discResult.Level == model.Discount2xFree ||
				discResult.Level == model.Discount2x50
			if discResult.FreeEndAt != nil {
				event.FreeEndAt = discResult.FreeEndAt
			}
		}
		e.detectCache.Store(cacheKey, &detectCacheEntry{
			hasHR:         event.HasHR,
			hrSeedTimeH:   event.HRSeedTimeH,
			discountLevel: event.DiscountLevel,
			isFree:        event.IsFree,
			freeEndAt:     event.FreeEndAt,
			cachedAt:      time.Now(),
		})
		return
	}

	type hrOut struct {
		result *model.HRResult
		err    error
	}
	type discOut struct {
		result *model.DiscountResult
		err    error
	}

	hrCh := make(chan hrOut, 1)
	discCh := make(chan discOut, 1)

	go func() {
		r, err := adapter.DetectHR(detectCtx, config, event.TorrentID)
		hrCh <- hrOut{result: r, err: err}
	}()

	go func() {
		r, err := adapter.DetectDiscount(detectCtx, config, event.TorrentID)
		discCh <- discOut{result: r, err: err}
	}()

	hr := <-hrCh
	if hr.err == nil && hr.result != nil {
		event.HasHR = hr.result.HasHR
		if event.HasHR {
			event.HRSeedTimeH = hr.result.SeedTimeH
			if event.HRSeedTimeH == 0 {
				event.HRSeedTimeH = 72
			}
		}
	} else if hr.err != nil {
		e.logger.Debug("detect HR failed", zap.String("site", siteName), zap.String("torrent", event.TorrentID), zap.Error(hr.err))
	}

	disc := <-discCh
	if disc.err == nil && disc.result != nil && disc.result.Level != model.DiscountNone {
		event.DiscountLevel = disc.result.Level
		event.IsFree = disc.result.Level == model.DiscountFree ||
			disc.result.Level == model.Discount2xFree ||
			disc.result.Level == model.Discount2x50
		if disc.result.FreeEndAt != nil {
			event.FreeEndAt = disc.result.FreeEndAt
		}
	}

	e.detectCache.Store(cacheKey, &detectCacheEntry{
		hasHR:         event.HasHR,
		hrSeedTimeH:   event.HRSeedTimeH,
		discountLevel: event.DiscountLevel,
		isFree:        event.IsFree,
		freeEndAt:     event.FreeEndAt,
		cachedAt:      time.Now(),
	})
}

func (e *Engine) RecheckWaiting(ctx context.Context) error {
	subs, err := e.repo.ListActive(ctx)
	if err != nil {
		return err
	}

	for i := range subs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		e.fetchOnce(ctx, &subs[i])
	}
	return nil
}

func (e *Engine) ExpireDiskBudget() {
	if e.diskBudget != nil {
		e.diskBudget.Expire()
	}
}
