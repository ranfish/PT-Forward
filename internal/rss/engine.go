package rss

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/event"
	"github.com/ranfish/pt-forward/internal/filter"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Engine struct {
	fetcher      *Fetcher
	repo         *Repository
	db           *gorm.DB
	logger       *zap.Logger
	filterEng    *filter.Engine
	dispatcher   *event.Dispatcher
	siteProvider model.SiteInfoProvider
	diskBudget   *DiskBudgetManager

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

	go func() {
		time.Sleep(30 * time.Second)
		e.diskBudget.Release(ticket)
	}()

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

	for id, cancel := range e.tasks {
		cancel()
		delete(e.tasks, id)
	}
	e.logger.Info("rss engine stopped")
}

func (e *Engine) Trigger(ctx context.Context, subID uint) error {
	sub, err := e.repo.GetByID(ctx, subID)
	if err != nil {
		return &model.AppError{Code: 13002, Message: "订阅不存在"}
	}

	if !sub.Enabled {
		return &model.AppError{Code: 13003, Message: "订阅已禁用"}
	}

	fetchCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
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
	ctx, cancel := context.WithCancel(parentCtx)

	e.mu.Lock()
	if old, ok := e.tasks[sub.ID]; ok {
		old()
	}
	e.tasks[sub.ID] = cancel
	e.mu.Unlock()

	interval := parseCronInterval(sub.Cron)
	if interval < time.Minute {
		interval = 5 * time.Minute
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

		var torrentEvents []model.TorrentEvent
		newCount := 0
		for _, event := range events {
			isSeen, _ := e.repo.IsSeen(ctx, event.SiteName, event.TorrentID)
			if isSeen {
				continue
			}

			if sub.DiskBudgetEnabled && sub.ClientID != "" && e.siteProvider != nil {
				if err := e.checkDiskBudget(ctx, sub, event.Size); err != nil {
					e.logger.Warn("torrent skipped by disk budget",
						zap.String("torrent", event.TorrentID),
						zap.Int64("size", event.Size),
						zap.Error(err))
					continue
				}
			}

			e.detectHRAndDiscount(ctx, event, site.Domain)

			if sub.ScrapeFree && !event.IsFree {
				e.logger.Debug("torrent skipped: not free",
					zap.String("torrent", event.TorrentID),
					zap.Bool("free", event.IsFree))
				continue
			}

			if event.HasHR && e.siteHRStrategy(ctx, sub.SiteName) == "skip" {
				e.logger.Debug("torrent skipped by HR skip strategy",
					zap.String("torrent", event.TorrentID),
					zap.String("site", sub.SiteName))
				continue
			}

			if len(sub.Conditions) > 0 {
				evalCtx := &filter.EvalContext{
					Title:    event.Title,
					Size:     event.Size,
					SiteName: event.SiteName,
					Free:     event.IsFree,
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
					continue
				}
			}

			if e.filterEng != nil {
				matchResult, err := e.filterEng.Match(ctx, &filter.EvalContext{
					Title:    event.Title,
					Size:     event.Size,
					SiteName: event.SiteName,
					Free:     event.IsFree,
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
					continue
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
			if event.IsFree {
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
		return 5 * time.Minute
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

	return 5 * time.Minute
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
			event.IsFree = discResult.Level == model.DiscountFree ||
				discResult.Level == model.Discount2xFree ||
				discResult.Level == model.Discount2x50
			if discResult.FreeEndAt != nil {
				event.FreeEndAt = discResult.FreeEndAt
			}
		}
	}
}
