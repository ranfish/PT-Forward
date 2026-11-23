package reseed

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/util"
	"go.uber.org/zap"
)

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
	// §59.544 修1：无条件覆盖注册（cancel 永远指向当前在跑的任务）。
	e.mu.Lock()
	if old, ok := e.tasks[task.ID]; ok {
		old.cancel()
	}
	taskCtx, cancel := context.WithCancel(ctx) //nolint:gosec // cancel stored in e.tasks for later invocation
	tc := &taskControl{ctx: taskCtx, cancel: cancel}
	e.tasks[task.ID] = tc
	e.mu.Unlock()
	ctx = taskCtx

	// §59.544 修1 附：defer 自清理（指针比较确保只删自己的注册）。
	defer func() {
		tc.cancel()
		e.mu.Lock()
		if cur, ok := e.tasks[task.ID]; ok && cur == tc {
			delete(e.tasks, task.ID)
		}
		e.mu.Unlock()
	}()

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

	clientUIDs := e.resolveClientIDsToUIDs(ctx, task.ClientIDs)
	if len(clientUIDs) == 0 {
		return result, nil
	}

	if e.clientProvider == nil {
		return result, nil
	}

	e.retryFailedForTask(ctx, task, clientUIDs)

	// §59.558 PR-4: 准备段三 helper（源收集/参数解析/预载族——engine_match_prepare.go）
	src := e.collectRunTaskSources(ctx, clientUIDs)
	result.TotalSources = len(src.sourceTorrents)
	e.logRunTaskSources(task, result.TotalSources, src.nameSites)
	if len(src.sourceTorrents) == 0 {
		return result, nil
	}

	params := e.resolveRunTaskParams(ctx, task)

	infoHashes := make([]string, 0, len(src.sourceTorrents))
	for _, s := range src.sourceTorrents {
		infoHashes = append(infoHashes, s.InfoHash)
	}
	e.computeMissingFingerprints(ctx, src.sourceTorrents, infoHashes)

	e.logger.Info("preload started",
		zap.Int("sources", len(src.sourceTorrents)),
		zap.Strings("targetSites", params.targetSites))

	pl := e.preloadRunTaskData(ctx, task, src, params, infoHashes)

	// 上下文指针（defer 清理必须留主函数——闭包引用）
	e.currentDomainResolver = pl.dr
	defer func() { e.currentDomainResolver = nil }()
	e.currentCloudFPCache = pl.cfCache
	defer func() { e.currentCloudFPCache = nil }()

	l2s := newL2Stats()

	if task.EngineMode == model.ReseedModeSeedFeature {
		e.runSeedFeatureScan(ctx, task, pl.ps, src.sourceTorrents, params.sourceSites, pl.fpCache, pl.negCache, pl.phCache, pl.cfCache, l2s, pl.confirmedTargets, src.nameSites, pl.iyuuResults, pl.iyuuSidMap, params.targetSites, params.excludedSites, result)
	} else {
		e.runLegacyScan(ctx, task, pl.ps, src.sourceTorrents, params.sourceSites, pl.fpCache, params.sizeTolerance, pl.negCache, pl.phCache, pl.cfCache, l2s, pl.confirmedTargets, src.nameSites, pl.iyuuResults, pl.iyuuSidMap, params.targetSites, params.excludedSites, result)
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
	// §59.534 附：L0 hash batch 站级熔断——连续失败 3 次后本次任务内
	// 跳过该站 hash 通道（站方不可达时防逐源种超时爬行·243 实证 820 源
	// ×30s≈6.8h 零产出）。成功一次即清零。
	hashBatchFailures map[string]int
	// §59.534 附二：站级批量预查缓存（site→info_hash→result）。站方
	// hashes 端点限流 30 req/min（243 实证 429）——逐源种单查必撞限
	// （820 源÷30≈27min）；BatchMatchHashes ≤200/批 → 820 hash=5 请求
	// 一次查完缓存内存态。nil=该站未预查（回退单查+熔断）。
	// 并发：站 worker 并发写各自 site 键+matchAtSite 并发读——map 读写
	// 必须锁保护（Go map 并发写 panic 即使不同键）。
	hashBatchMu      sync.Mutex
	hashBatchResults map[string]map[string]*model.HashMatchResult
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

	// 站点 content_type 过滤：纯视频站不搜音乐种子，纯音乐站不搜视频种子
	if siteInfo.ContentType != "" {
		if fp := mc.fc.get(src.InfoHash, src.SiteName); fp != nil {
			torrentType := util.DetectContentType(fp.FileTreeParsed)
			if !util.ContentTypeCompatible(torrentType, siteInfo.ContentType) {
				return nil
			}
		}
	}

	if hasMatchMethod(mc.task.MatchMethods, "pieces_hash") {
		c := e.matchLayer0FromCache(src.InfoHash, src.SiteName, siteInfo.Name, mc.fc, mc.phCache)
		if c == nil {
			// §59.534: L0 实时通道——站方批量 hash 精确匹配（好大 API 独有·
			// info_hash 数学身份）。pieces-hash 缓存 miss 时兜底（缓存命中
			// 路径不变）。站级熔断：连续 3 次失败跳过（防超时爬行）。
			// §59.534 附二：批量预查缓存优先（一次 5 请求 vs 逐源 820 请求
			// ——hashes 端点 30 req/min 限流下唯一可行形态）。
			mc.hashBatchMu.Lock()
			siteCache := mc.hashBatchResults[siteInfo.Name]
			r := siteCache[strings.ToLower(src.InfoHash)]
			mc.hashBatchMu.Unlock()
			if r != nil {
				c = &model.Candidate{
					TargetSite:      siteInfo.Name,
					TargetTorrentID: r.TorrentID,
					TargetInfoHash:  r.InfoHash,
					Confidence:      1.0,
					MatchMethod:     "pieces_hash",
				}
			} else if siteCache != nil {
				// 预查已做过：缓存 miss=站上无此 hash（零请求）
			} else {
				mc.hashBatchMu.Lock()
				fails := mc.hashBatchFailures[siteInfo.Name]
				mc.hashBatchMu.Unlock()
				if fails < 3 {
					hb, failed := e.matchLayer0HashBatch(mc.ctx, adapter, siteConfig, src.InfoHash, siteInfo.Name)
					if failed {
						mc.hashBatchMu.Lock()
						mc.hashBatchFailures[siteInfo.Name]++
						tripped := mc.hashBatchFailures[siteInfo.Name] == 3
						mc.hashBatchMu.Unlock()
						if tripped && e.logger != nil {
							e.logger.Info("L0 hash batch 站级熔断（连续 3 次失败·本任务内跳过）",
								zap.String("site", siteInfo.Name))
						}
					} else if hb != nil {
						mc.hashBatchMu.Lock()
						mc.hashBatchFailures[siteInfo.Name] = 0
						mc.hashBatchMu.Unlock()
						c = hb
					}
				}
			}
		}
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
