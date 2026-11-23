package seeding

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ranfish/pt-forward/internal/event"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/pusher"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	scoringCutoffHours = 24 * time.Hour // §59.153: 72h→24h（消费方只要最新值+日志页近一天）
	syncGracePeriod    = 15 * time.Minute
	syncHardTimeout    = 2 * time.Hour
	sqliteVarLimit     = 500
)

type Engine struct {
	db               *gorm.DB
	logger           *zap.Logger
	clientProvider   model.DownloaderProvider
	clientProviderMu sync.RWMutex
	siteProvider     model.SiteInfoProvider
	freeEndMonitor   *FreeEndMonitor
	wsBroadcaster    event.WSBroadcaster
	mu               sync.RWMutex
	recordMap        map[string]*model.SeedingTorrentRecord
	emaStates        map[uint]*emaState
	maindataMu       sync.RWMutex
	maindataCache    map[uint]*maindataEntry // §59.251: 键=client UID
	fitTimer         *FitTimer
	freeWaitMonitor  *FreeWaitMonitor
	refreshCancel    context.CancelFunc
	wg               sync.WaitGroup
	pusher           *pusher.Pusher
	pendingEvents    chan *pusher.PushedEvent

	unregisteredChecking atomic.Bool

	// §59.31: 巡检调度状态（内存态，重启重建）
	//   patrolCursors: per-client 全量轮询游标（审计 #4：Engine 级单游标会让
	//     小客户端每 tick 重置，饿死大客户端的扫描段）
	//   patrolCooldowns: 怀疑池冷却（遗漏 A）
	patrolMu        sync.Mutex
	patrolCursors   map[uint]int
	patrolCooldowns map[string]*patrolCooldownEntry

	spaceAlarmMu   sync.Mutex
	spaceAlarmLast map[uint]time.Time

	discountCache   map[string]*discountCacheEntry
	discountCacheMu sync.Mutex
}

type discountCacheEntry struct {
	Result    *model.DiscountResult
	CheckedAt time.Time
}

type maindataEntry struct {
	Maindata        *model.Maindata
	FreeSpace       int64
	TotalDiskSpace  int64
	UpdatedAt       time.Time
	Rid             int
	TorrentMap      map[string]*model.TorrentInfo
	CyclesSinceFull int
}

type emaState struct {
	UploadSpeed   float64
	DownloadSpeed float64
}

const emaAlpha = 0.3

func NewEngine(db *gorm.DB, logger *zap.Logger) *Engine {
	logger = logger.With(zap.String("component", "seeding"))
	e := &Engine{
		db:              db,
		logger:          logger,
		recordMap:       make(map[string]*model.SeedingTorrentRecord),
		emaStates:       make(map[uint]*emaState),
		maindataCache:   make(map[uint]*maindataEntry),
		fitTimer:        NewFitTimer(),
		freeWaitMonitor: NewFreeWaitMonitor(db, logger),
		pendingEvents:   make(chan *pusher.PushedEvent, 1000),
		spaceAlarmLast:  make(map[uint]time.Time),
		discountCache:   make(map[string]*discountCacheEntry),
	}
	e.freeEndMonitor = NewFreeEndMonitor(db, nil, logger)
	e.freeEndMonitor.SetEngine(e)
	e.freeWaitMonitor.SetEngine(e)
	return e
}

// GetCachedTorrents 返回某个下载器在 maindataCache 中的全部种子（全部状态）。
// 如果该下载器不在缓存中（未配置/engine 未启动/首次同步未完成），返回 nil。
func (e *Engine) GetCachedTorrents(clientUID uint) []*model.TorrentInfo {
	e.maindataMu.RLock()
	defer e.maindataMu.RUnlock()
	entry, ok := e.maindataCache[clientUID]
	if !ok || entry == nil || len(entry.TorrentMap) == 0 {
		return nil
	}
	result := make([]*model.TorrentInfo, 0, len(entry.TorrentMap))
	for _, t := range entry.TorrentMap {
		if t != nil {
			result = append(result, t)
		}
	}
	return result
}

func (e *Engine) SetClientProvider(cp model.DownloaderProvider) {
	e.clientProviderMu.Lock()
	defer e.clientProviderMu.Unlock()
	e.clientProvider = cp
	if e.freeEndMonitor != nil {
		e.freeEndMonitor.client = cp
	}
}

func (e *Engine) getClientProvider() model.DownloaderProvider {
	e.clientProviderMu.RLock()
	defer e.clientProviderMu.RUnlock()
	return e.clientProvider
}

func (e *Engine) GetGlobalTransferStats(ctx context.Context) *model.GlobalTransferStats {
	result := &model.GlobalTransferStats{}
	if e.getClientProvider() == nil {
		return result
	}
	for _, clientUID := range e.getClientProvider().ListClients() {
		client, err := e.getClientProvider().Get(clientUID)
		if err != nil {
			continue
		}
		stats, err := client.GetGlobalTransferStats(ctx)
		if err != nil {
			e.logger.Debug("get global transfer stats failed",
				zap.String("client", client.GetName()),
				zap.Error(err))
			continue
		}
		result.AllTimeUpload += stats.AllTimeUpload
		result.AllTimeDownload += stats.AllTimeDownload
	}
	return result
}

func (e *Engine) GetTodayTransferDelta(ctx context.Context) *model.GlobalTransferStats {
	result := &model.GlobalTransferStats{}
	if e.getClientProvider() == nil {
		return result
	}
	for _, clientUID := range e.getClientProvider().ListClients() {
		client, err := e.getClientProvider().Get(clientUID)
		if err != nil {
			continue
		}
		currentStats, err := client.GetGlobalTransferStats(ctx)
		if err != nil {
			continue
		}
		var dbState model.SeedingClientState
		if err := e.db.WithContext(ctx).Where("client_uid = ?", clientUID).First(&dbState).Error; err != nil {
			continue
		}
		result.AllTimeUpload += currentStats.AllTimeUpload - dbState.DayStartUpload
		result.AllTimeDownload += currentStats.AllTimeDownload - dbState.DayStartDownload
	}
	return result
}

func (e *Engine) SetSiteProvider(sp model.SiteInfoProvider) {
	e.siteProvider = sp
}

func (e *Engine) SetWSBroadcaster(b event.WSBroadcaster) {
	e.wsBroadcaster = b
}

func recordKey(clientUID uint, infoHash string) string {
	return fmt.Sprintf("%d:%s", clientUID, infoHash)
}

func (e *Engine) Start(ctx context.Context) error {
	var records []model.SeedingTorrentRecord
	if err := e.db.WithContext(ctx).
		Where("status IN ?", []string{"pending", "seeding", "transferring", "paused_free_end", "paused_rule", "deleting"}).
		Find(&records).Error; err != nil {
		return seedingError(ErrSeedingDB, "load seeding records", err)
	}

	e.mu.Lock()
	for i := range records {
		// 重启时把 transferring 回退为 seeding（转移被中断，下轮 refreshMaindataLoop 重试）
		if records[i].Status == model.SeedingStatusTransferring {
			records[i].Status = model.SeedingStatusSeeding
		}
		key := recordKey(records[i].ClientUID, records[i].InfoHash)
		e.recordMap[key] = &records[i]
	}
	e.mu.Unlock()
	// DB 层面把残留的 transferring 回退为 seeding
	e.db.WithContext(ctx).Model(&model.SeedingTorrentRecord{}).
		Where("status = ?", model.SeedingStatusTransferring).
		Update("status", model.SeedingStatusSeeding)

	e.logger.Info("seeding engine started", zap.Int("records", len(records)))

	var failedRecords []model.SeedingTorrentRecord
	if err := e.db.WithContext(ctx).
		Where("status = ?", "delete_failed").
		Find(&failedRecords).Error; err != nil {
		e.logger.Warn("query failed records for cleanup", zap.Error(err))
	}
	for i := range failedRecords {
		rec := &failedRecords[i]
		e.logger.Info("cleaning up delete_failed record",
			zap.Uint("id", rec.ID),
			zap.String("info_hash", rec.InfoHash))
		if err := e.db.WithContext(ctx).Model(rec).Update("status", model.SeedingStatusDeleted).Error; err != nil {
			e.logger.Warn("update record status to deleted failed",
				zap.Uint("id", rec.ID),
				zap.Error(err))
		}
	}

	if e.freeEndMonitor != nil {
		e.freeEndMonitor.RecoverOnStartup(ctx)
	}

	if e.freeWaitMonitor != nil {
		e.freeWaitMonitor.RecoverOnStartup(ctx)
	}

	var recoveryRules []model.DeleteRule
	if err := e.db.WithContext(ctx).Where("enabled = ?", true).Find(&recoveryRules).Error; err != nil {
		e.logger.Warn("failed to load recovery rules, continuing without", zap.Error(err))
	}
	ruleMap := make(map[uint]model.DeleteRule)
	for _, r := range recoveryRules {
		ruleMap[r.ID] = r
	}

	var configs []model.SeedingClientConfig
	if err := e.db.WithContext(ctx).Where("enabled = ?", true).Find(&configs).Error; err != nil {
		e.logger.Warn("failed to load seeding configs, continuing without", zap.Error(err))
	}
	clientRuleIDs := make(map[uint]map[uint]bool)
	for _, cfg := range configs {
		ids := splitRuleIDs(cfg.DeleteRuleIDs)
		ruleSet := make(map[uint]bool, len(ids))
		for _, idStr := range ids {
			if id, err := strconv.ParseUint(idStr, 10, 32); err == nil {
				ruleSet[uint(id)] = true
			}
		}
		clientRuleIDs[cfg.ClientUID] = ruleSet
	}

	for i := range records {
		if records[i].FirstMatchedAt == nil {
			continue
		}
		t := *records[i].FirstMatchedAt
		ruleSet := clientRuleIDs[records[i].ClientUID]
		for ruleID := range ruleSet {
			if _, ok := ruleMap[ruleID]; ok {
				e.fitTimer.MarkMatched(ruleID, records[i].InfoHash, t)
			}
		}
	}

	refreshCtx, cancel := context.WithCancel(context.Background())
	e.refreshCancel = cancel
	e.wg.Add(1)
	go func() { defer e.wg.Done(); e.refreshMaindataLoop(refreshCtx) }()

	if e.pusher != nil {
		e.wg.Add(1)
		go func() { defer e.wg.Done(); e.consumeLoop(ctx) }()
	}

	e.wg.Add(1)
	go func() { defer e.wg.Done(); e.archiveLoop(refreshCtx) }()

	return nil
}

func (e *Engine) Stop(_ context.Context) error {
	if e.refreshCancel != nil {
		e.refreshCancel()
	}
	e.wg.Wait()
	if e.freeEndMonitor != nil {
		e.freeEndMonitor.StopAll()
	}
	e.logger.Info("seeding engine stopped", zap.Int("active_records", len(e.recordMap)))
	return nil
}

func defaultHRSeedTimeH(ctx context.Context, provider model.SiteInfoProvider, siteName string) int {
	if provider == nil || siteName == "" || siteName == "unknown" {
		return 72
	}
	cfg, err := provider.GetSiteConfig(ctx, siteName)
	if err != nil || cfg == nil {
		return 72
	}
	return cfg.HR.SeedTimeH()
}
