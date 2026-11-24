package reseed

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/compliance"
	"github.com/ranfish/pt-forward/internal/fingerprint"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/scheduler"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const errAdapterNotFoundCode = 31006

type Engine struct {
	db                    *gorm.DB
	logger                *zap.Logger
	siteProvider          model.SiteInfoProvider
	clientProvider        model.DownloaderProvider
	iyuuService           model.IYUUService
	fpRepo                *fingerprint.Repository
	trackerResolver       *TrackerSiteResolver
	scheduler             *scheduler.Registry
	limiter               *siteLimiter
	mu                    sync.RWMutex
	tasks                 map[uint]*taskControl
	cloudFPService        model.CloudFPService
	deleteReporter        *deleteReporter
	contributeReporter    *contributeReporter
	currentCloudFPCache   *cloudFPCache
	currentDomainResolver *domainResolver
	complianceChecker     *compliance.Checker
}

func NewEngine(db *gorm.DB, logger *zap.Logger) *Engine {
	logger = logger.With(zap.String("component", "reseed"))
	return &Engine{
		db:      db,
		logger:  logger,
		limiter: newSiteLimiter(),
		tasks:   make(map[uint]*taskControl),
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
			if ferr := e.failMatch(ctx, m, fmt.Sprintf("任务不存在: %v", err)); ferr != nil {
				e.logger.Warn("failMatch 落库失败", zap.Uint("match_id", m.ID), zap.Error(ferr))
			}
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
		// §59.564 刀C: 注销失败 Warn（残留旧调度不可不可见——审计 03 §2.2）
		if err := e.scheduler.Unregister(name); err != nil {
			e.logger.Warn("reseed scheduler unregister failed",
				zap.Uint("task_id", task.ID), zap.String("name", name), zap.Error(err))
		}
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
	// §59.564 刀C: 注销失败 Warn（同 §2.2）
	if err := e.scheduler.Unregister(reseedSchedulerName(taskID)); err != nil {
		e.logger.Warn("reseed scheduler unregister failed",
			zap.Uint("task_id", taskID), zap.String("name", reseedSchedulerName(taskID)), zap.Error(err))
	}
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
	for id, tc := range e.tasks {
		tc.cancel()
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
		old.cancel()
	}
	ctx, cancel := context.WithCancel(parentCtx) //nolint:gosec // cancel stored in e.tasks for later invocation
	e.tasks[task.ID] = &taskControl{ctx: ctx, cancel: cancel}
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

	if tc, ok := e.tasks[taskID]; ok {
		tc.cancel()
		delete(e.tasks, taskID)
	}
}
