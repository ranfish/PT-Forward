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
	fetcher    *Fetcher
	repo       *Repository
	db         *gorm.DB
	logger     *zap.Logger
	filterEng  *filter.Engine
	dispatcher *event.Dispatcher

	mu    sync.RWMutex
	tasks map[uint]context.CancelFunc
}

func NewEngine(db *gorm.DB, logger *zap.Logger) *Engine {
	return &Engine{
		fetcher: NewFetcher(logger),
		repo:    NewRepository(db),
		db:      db,
		logger:  logger,
		tasks:   make(map[uint]context.CancelFunc),
	}
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
	for _, url := range sub.URLs {
		feed, err := e.fetcher.Fetch(ctx, url)
		if err != nil {
			e.logger.Warn("rss fetch failed",
				zap.String("subscription", sub.Name),
				zap.String("url", url),
				zap.Error(err))
			continue
		}

		var site model.Site
		if err := e.db.Where("name = ? OR domain = ?", sub.SiteName, sub.SiteName).First(&site).Error; err != nil {
			e.logger.Warn("site not found for subscription",
				zap.String("subscription", sub.Name),
				zap.String("site", sub.SiteName))
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
				if matchResult.Reject {
					e.logger.Debug("torrent rejected by filter rule",
						zap.String("torrent", event.TorrentID),
						zap.String("rule", matchResult.RuleName))
					continue
				}
				if !matchResult.Matched {
					e.logger.Debug("torrent did not match any filter rule",
						zap.String("torrent", event.TorrentID))
					continue
				}

				event.MatchedRule = &matchResult.RuleName
			}

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
				te.HRSeedTimeH = 0
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
