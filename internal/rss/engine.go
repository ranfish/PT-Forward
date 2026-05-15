package rss

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/event"
	"github.com/ranfish/pt-forward/internal/filter"
	"github.com/ranfish/pt-forward/internal/metrics"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const defaultFetchInterval = 5 * time.Minute

type Engine struct {
	fetcher        *Fetcher
	repo           *Repository
	db             *gorm.DB
	logger         *zap.Logger
	filterEng      *filter.Engine
	dispatcher     *event.Dispatcher
	siteProvider   model.SiteInfoProvider
	diskBudget     *DiskBudgetManager
	seedingCounter model.SeedingCollector
	wsBroadcaster  event.WSBroadcaster
	sideLoadMgr    *SideLoadManager
	configBus      *ConfigEventBus

	mu    sync.RWMutex
	tasks map[uint]context.CancelFunc
}

func NewEngine(db *gorm.DB, logger *zap.Logger) *Engine {
	return &Engine{
		fetcher:    NewFetcher(logger),
		repo:       NewRepository(db),
		db:         db,
		logger:     logger,
		diskBudget: NewDiskBudgetManager(logger),
		tasks:      make(map[uint]context.CancelFunc),
	}
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

func (e *Engine) checkDiskBudget(ctx context.Context, sub *model.RSSSubscription, size int64) error {
	clientProvider := e.siteProvider
	if clientProvider == nil {
		return nil
	}

	cp, ok := clientProvider.(interface {
		GetDownloaderClient(ctx context.Context, clientID string) (model.DownloaderClient, error)
	})
	if !ok {
		return nil
	}

	dlClient, err := cp.GetDownloaderClient(ctx, sub.ClientID)
	if err != nil {
		return nil
	}

	md, err := dlClient.GetMainData(ctx)
	if err != nil {
		return nil
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

func (e *Engine) CleanupOldData(ctx context.Context) (int64, error) {
	return e.repo.CleanupOldSeen(ctx, 30)
}

type DryRunResult struct {
	Total    int                      `json:"total"`
	Matched  int                      `json:"matched"`
	Rejected int                      `json:"rejected"`
	Skipped  int                      `json:"skipped"`
	Items    []DryRunItem             `json:"items"`
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
		feed, err := e.fetcher.Fetch(ctx, url)
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
	return len(h) > 0 && len(h) != 40 || (len(h) == 40 && h[:8] == "fakehash")
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
	defer e.mu.Unlock()

	stopped := len(e.tasks)
	for id, cancel := range e.tasks {
		cancel()
		delete(e.tasks, id)
	}
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

	fetchCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	go func() {
		defer cancel()
		e.fetchOnce(fetchCtx, sub)
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

	go func() {
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
	var site model.Site
	if err := e.db.WithContext(ctx).Where("name = ? OR domain = ?", sub.SiteName, sub.SiteName).First(&site).Error; err != nil {
		e.logger.Warn("site not found for subscription",
			zap.String("subscription", sub.Name),
			zap.String("site", sub.SiteName))
		return
	}

	for _, url := range sub.URLs {
		feed, err := e.fetcher.Fetch(ctx, url)
		if err != nil {
			e.logger.Warn("rss fetch failed",
				zap.String("subscription", sub.Name),
				zap.String("url", url),
				zap.Error(err))
			continue
		}

		events := e.fetcher.ParseItems(feed, sub, &site)

		metrics.RSSTorrentsFetched.WithLabelValues(sub.SiteName).Add(float64(len(events)))

		var torrentEvents []model.TorrentEvent
		newCount := 0
		for _, event := range events {
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
				metrics.RSSTorrentsFiltered.WithLabelValues(sub.SiteName, "already_seen").Inc()
				continue
			}

			if sub.DiskBudgetEnabled && sub.ClientID != "" && e.siteProvider != nil {
				if err := e.checkDiskBudget(ctx, sub, event.Size); err != nil {
					e.logger.Warn("torrent skipped by disk budget",
						zap.String("torrent", event.TorrentID),
						zap.Int64("size", event.Size),
						zap.Error(err))
					metrics.RSSTorrentsFiltered.WithLabelValues(sub.SiteName, "disk_budget").Inc()
					continue
				}
			}

			e.detectHRAndDiscount(ctx, event, site.Domain)

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
				evalCtx := &filter.EvalContext{
					Title:         event.Title,
					Size:          event.Size,
					SiteName:      event.SiteName,
					Free:          event.IsFree,
					DiscountLevel: discountStr,
				}
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
				evalCtx := &filter.EvalContext{
					Title:         event.Title,
					Size:          event.Size,
					SiteName:      event.SiteName,
					Free:          event.IsFree,
					DiscountLevel: discountStr,
				}
				if len(sub.RejectRuleIDs) > 0 {
					rejectResult, err := e.filterEng.MatchByIDs(ctx, sub.RejectRuleIDs, evalCtx)
					if err != nil {
						e.logger.Warn("reject rule match failed",
							zap.String("torrent", event.TorrentID),
							zap.Error(err))
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
					if err != nil {
						e.logger.Warn("accept rule match failed",
							zap.String("torrent", event.TorrentID),
							zap.Error(err))
					} else if !acceptResult.Matched {
						e.logger.Debug("torrent skipped: no accept rule matched",
							zap.String("torrent", event.TorrentID))
						metrics.RSSTorrentsFiltered.WithLabelValues(sub.SiteName, "no_accept_rule").Inc()
						continue
					}
				}
			}

			if e.filterEng != nil {
				matchResult, err := e.filterEng.Match(ctx, &filter.EvalContext{
					Title:         event.Title,
					Size:          event.Size,
					SiteName:      event.SiteName,
					Free:          event.IsFree,
					DiscountLevel: discountStr,
				})
				if err != nil {
					e.logger.Warn("filter match failed",
						zap.String("torrent", event.TorrentID),
						zap.Error(err))
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
				re, err := regexp.Compile(sub.RegexStr)
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
				Title:          event.Title,
				Size:           event.Size,
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

		if newCount > 0 {
			e.logger.Info("rss fetch completed",
				zap.String("subscription", sub.Name),
				zap.Int("total", len(events)),
				zap.Int("new", newCount),
				zap.Int("dispatched", len(torrentEvents)))
		}
	}
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
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

func (e *Engine) detectHRAndDiscount(ctx context.Context, event *model.RSSTorrentEvent, siteName string) {
	if e.siteProvider == nil || event.TorrentID == "" {
		return
	}

	adapter, err := e.siteProvider.GetAdapter(ctx, siteName)
	if err != nil {
		e.logger.Debug("获取适配器失败，跳过HR/折扣检测", zap.String("site", siteName), zap.Error(err))
		return
	}

	config, err := e.siteProvider.GetSiteConfig(ctx, siteName)
	if err != nil {
		e.logger.Debug("获取站点配置失败，跳过HR/折扣检测", zap.String("site", siteName), zap.Error(err))
		return
	}

	if hrResult, err := adapter.DetectHR(ctx, config, event.TorrentID); err == nil && hrResult != nil {
		event.HasHR = hrResult.HasHR
		if hrResult.HasHR {
			event.HRSeedTimeH = hrResult.SeedTimeH
			if event.HRSeedTimeH == 0 {
				event.HRSeedTimeH = 72
			}
		}
	}

	if discResult, err := adapter.DetectDiscount(ctx, config, event.TorrentID); err == nil && discResult != nil {
		if discResult.Level != model.DiscountNone {
			event.DiscountLevel = discResult.Level
			event.IsFree = discResult.Level == model.DiscountFree ||
				discResult.Level == model.Discount2xFree ||
				discResult.Level == model.Discount2x50
			if discResult.FreeEndAt != nil {
				event.FreeEndAt = discResult.FreeEndAt
			}
		}
	}
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
