package reseed

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	clientpkg "github.com/ranfish/pt-forward/internal/client"
	"github.com/ranfish/pt-forward/internal/fingerprint"
	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
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
		// §59.534 附二：预初始化（map 并发写 panic 防御——多站 worker 并发）
		hashBatchFailures: map[string]int{},
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
			// §59.534 附二：站级批量预查——matchAtSite 循环前一次查完
			// 该站全部源 hash（BatchMatchHashes ≤200/批自动切批）。hashes
			// 端点限流 30 req/min——批量预查 5 请求 vs 逐源 820 请求（必
			// 撞限流）。失败静默跳过（回退单查+熔断路径）。mc 并发写：各
			// worker 只写自己的 site 键——map 按键隔离（预查结果写入时
			// 无并发冲突·读侧在 matchAtSite 同 worker 内）。
			e.preloadHashBatchForSite(mc, workerCtx, siteInfo, eligible)
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
			ClientUID:       sr.src.ClientUID,
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
					ClientUID:       src.ClientUID,
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
				ClientUID:       src.ClientUID,
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
	// §59.25: phCache 存的是框架特定的 hash，需要尝试两种格式
	torrentID, found := phCache.get(siteName, fp.PiecesHash)
	if !found && fp.PiecesHashBencode != "" {
		torrentID, found = phCache.get(siteName, fp.PiecesHashBencode)
	}
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
		// §59.25: 搜索失败时降级放行，由注入阶段 ValidateInjection 精确校验
		e.logger.Info("L0 size verification search failed, deferring to injection validation",
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
		// §59.25: 搜索结果中找不到目标种子时降级放行（如野马按 torrentID 搜不到）
		// 注入阶段 ValidateInjection 会从 .torrent 文件精确校验 size
		e.logger.Info("L0 size verification target not in search results, deferring to injection validation",
			zap.String("site", siteName),
			zap.String("torrentID", targetTorrentID))
		return true
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

// supportsPiecesHash 检查站点是否支持 pieces_hash 去重（DB 字段或适配器方法）
func supportsPiecesHash(config *model.SiteConfig, adapter model.SiteAdapter) bool {
	if config != nil && config.SupportsPiecesHashAPI {
		return true
	}
	if adapter != nil && adapter.SupportsSearchByPiecesHash() {
		return true
	}
	return false
}

// piecesHashForAdapter §59.25: 按目标站框架选择 pieces_hash 格式
func piecesHashForAdapter(fp *model.ContentFingerprint, adapter model.SiteAdapter) string {
	if fp == nil {
		return ""
	}
	if adapter != nil && adapter.Framework() == "yemapt" {
		return fp.PiecesHashBencode
	}
	return fp.PiecesHash
}

// lazyComputeBencodeHash §59.25: 旧指纹缺 PiecesHashBencode 时从 .torrent 现算并回写
func (e *Engine) lazyComputeBencodeHash(ctx context.Context, src sourceTorrent, fp *model.ContentFingerprint) string {
	if e.clientProvider == nil {
		return ""
	}
	dlClient, err := e.clientProvider.Get(src.ClientUID)
	if err != nil {
		return ""
	}
	torrentDir := dlClient.GetTorrentDir()
	torrentData, err := clientpkg.ReadTorrentFile(torrentDir, src.InfoHash)
	if err != nil {
		e.logger.Debug("lazyComputeBencodeHash: read torrent failed",
			zap.String("hash", src.InfoHash), zap.Error(err))
		return ""
	}
	meta, err := fingerprint.ComputeFromTorrent(torrentData)
	if err != nil || meta.PiecesHashBencode == "" {
		return ""
	}
	// 回写 DB 和内存
	if e.fpRepo != nil {
		if err := e.fpRepo.UpdateField(ctx, fp.InfoHash, fp.SiteName, "pieces_hash_bencode", meta.PiecesHashBencode); err != nil {
			e.logger.Warn("pieces_hash_bencode 回写失败", zap.String("hash", src.InfoHash), zap.Error(err))
		}
	}
	fp.PiecesHashBencode = meta.PiecesHashBencode
	e.logger.Info("lazyComputeBencodeHash: computed and saved",
		zap.String("hash", src.InfoHash))
	return meta.PiecesHashBencode
}

// syncIYUUCoverage §59.26: 将 IYUU 查询结果同步到 site_coverage_cache
func (e *Engine) syncIYUUCoverage(ctx context.Context, iyuuResults map[string][]*model.IYUUReseedResult, iyuuSidMap map[int]string) {
	now := time.Now()
	ttl := now.Add(7 * 24 * time.Hour)

	// 收集所有 (hash, site, torrent_id) 三元组
	type covEntry struct {
		infoHash   string
		siteName   string
		torrentID  string
		targetHash string // §59.535 联合回归 P3: 注入依赖列（对齐共享层落库形态）
	}
	var entries []covEntry
	for hash, results := range iyuuResults {
		for _, r := range results {
			for _, t := range r.Targets {
				siteName := iyuuSidMap[t.Sid]
				if siteName == "" {
					continue
				}
				entries = append(entries, covEntry{
					infoHash:   hash,
					siteName:   siteName,
					torrentID:  strconv.Itoa(t.TorrentID),
					targetHash: strings.ToLower(t.InfoHash),
				})
			}
		}
	}

	if len(entries) == 0 {
		return
	}

	// 步骤 1: 批量 INSERT OR IGNORE（每 500 行一批）
	batchSize := 500
	for i := 0; i < len(entries); i += batchSize {
		end := i + batchSize
		if end > len(entries) {
			end = len(entries)
		}
		batch := entries[i:end]

		var placeholders []string
		var args []interface{}
		for _, e := range batch {
			placeholders = append(placeholders, "(?, ?, ?, ?, ?, ?, ?, ?, ?)")
			args = append(args,
				e.infoHash, e.siteName, "probably_has", "iyuu",
				0.9, e.torrentID, e.targetHash, now, ttl,
			)
		}
		sql := "INSERT OR IGNORE INTO site_coverage_cache (info_hash, site_name, status, source, confidence, torrent_id, target_info_hash, queried_at, expires_at) VALUES " +
			strings.Join(placeholders, ", ")
		e.db.WithContext(ctx).Exec(sql, args...)
	}

	// 步骤 2: 补全已有记录的 torrent_id（confidence >= 0.9 且 torrent_id 为空）
	for _, ent := range entries {
		if ent.torrentID == "" {
			continue
		}
		e.db.WithContext(ctx).Model(&model.SiteCoverageCache{}).
			Where("info_hash = ? AND site_name = ? AND (torrent_id = '' OR torrent_id IS NULL) AND confidence >= ?", ent.infoHash, ent.siteName, 0.9).
			Updates(map[string]interface{}{
				"torrent_id":       ent.torrentID,
				"target_info_hash": ent.targetHash, // §59.535 联合回归 P3
				"expires_at":       ttl,
				"queried_at":       now,
			})
	}

	e.logger.Info("syncIYUUCoverage: synced",
		zap.Int("entries", len(entries)))
}

// preloadHashBatchForSite §59.534 附二：站级批量预查——该站全部源 hash
// 一次查完（≤200/批自动切批）。成功填 mc.hashBatchResults[site]（含空
// map=已查无命中语义——缓存 miss 与未预查可区分）；失败留 nil（回退
// 逐源单查+熔断）。仅 HashBatchMatcher 站且任务勾选 pieces_hash 时执行。
func (e *Engine) preloadHashBatchForSite(mc *matchConfig, ctx context.Context, siteInfo *model.SiteInfo, eligible []sourceTorrent) {
	if mc.task == nil || !hasMatchMethod(mc.task.MatchMethods, "pieces_hash") {
		return
	}
	adapter := mc.ps.adapters[siteInfo.Name]
	config := mc.ps.configs[siteInfo.Name]
	if adapter == nil || config == nil {
		return
	}
	hm, ok := adapter.(model.HashBatchMatcher)
	if !ok {
		return
	}
	if config.APIKey == "" && config.Cookie == "" && config.Passkey == "" {
		return
	}
	seen := map[string]bool{}
	hashes := make([]string, 0, len(eligible))
	for _, src := range eligible {
		h := strings.ToLower(src.InfoHash)
		if h != "" && !seen[h] {
			seen[h] = true
			hashes = append(hashes, h)
		}
	}
	if len(hashes) == 0 {
		return
	}
	start := time.Now()
	results, err := hm.BatchMatchHashes(ctx, config, hashes)
	if err != nil {
		if e.logger != nil {
			e.logger.Info("L0 hash batch 预查失败（回退逐源单查+熔断）",
				zap.String("site", siteInfo.Name), zap.Int("hashes", len(hashes)),
				zap.Duration("elapsed", time.Since(start)), zap.Error(err))
		}
		return
	}
	cache := make(map[string]*model.HashMatchResult, len(results))
	for _, r := range results {
		if r != nil && r.InfoHash != "" {
			cache[strings.ToLower(r.InfoHash)] = r
		}
	}
	mc.hashBatchMu.Lock()
	if mc.hashBatchResults == nil {
		mc.hashBatchResults = map[string]map[string]*model.HashMatchResult{}
	}
	mc.hashBatchResults[siteInfo.Name] = cache
	mc.hashBatchMu.Unlock()
	if e.logger != nil {
		e.logger.Info("L0 hash batch 预查完成",
			zap.String("site", siteInfo.Name),
			zap.Int("queried", len(hashes)), zap.Int("hits", len(cache)),
			zap.Duration("elapsed", time.Since(start)))
	}
}

// matchLayer0HashBatch §59.534: L0 站方批量 hash 实时通道（model.HashBatchMatcher
// 类型断言——好大 API BatchMatchHashes）。单 hash 查询（批 API 的单元素形态·
// 站内 worker 顺序消费自然限流）。失败静默降级 L1/L2（独立信息源不阻塞）。
func (e *Engine) matchLayer0HashBatch(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, sourceInfoHash, siteName string) (*model.Candidate, bool) {
	hm, ok := adapter.(model.HashBatchMatcher)
	if !ok {
		return nil, false
	}
	if config.APIKey == "" && config.Cookie == "" && config.Passkey == "" {
		return nil, false
	}
	results, err := hm.BatchMatchHashes(ctx, config, []string{strings.ToLower(sourceInfoHash)})
	if err != nil {
		if e.logger != nil {
			e.logger.Debug("L0 hash batch query failed",
				zap.String("site", siteName), zap.Error(err))
		}
		return nil, true // true=请求失败（调用方计数熔断）
	}
	for _, r := range results {
		if strings.EqualFold(r.InfoHash, sourceInfoHash) && r.TorrentID != "" {
			return &model.Candidate{
				TargetSite:      siteName,
				TargetTorrentID: r.TorrentID,
				TargetInfoHash:  r.InfoHash,
				Confidence:      1.0,
				MatchMethod:     "pieces_hash",
			}, false
		}
	}
	return nil, false
}

func (e *Engine) matchLayer0PiecesHash(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, sourceInfoHash, sourceSiteName, siteName string, fc *fpCache) *model.Candidate {
	if !supportsPiecesHash(config, adapter) {
		return nil
	}
	if !adapter.SupportsSearchByPiecesHash() {
		return nil
	}
	searcher, ok := adapter.(piecesHashSearcher)
	if !ok {
		return nil
	}
	if config.Passkey == "" && config.Cookie == "" && config.APIKey == "" {
		return nil
	}

	fp := fc.get(sourceInfoHash, sourceSiteName)
	if fp == nil || fp.PiecesHash == "" {
		return nil
	}

	// §59.25: 按目标站框架选择 pieces_hash 格式
	ph := piecesHashForAdapter(fp, adapter)
	if ph == "" {
		return nil
	}

	matches, err := searcher.SearchByPiecesHash(ctx, config, []string{ph})
	if err != nil {
		e.logger.Debug("Layer0 pieces_hash API failed",
			zap.String("site", siteName),
			zap.String("pieces_hash", ph),
			zap.Error(err))
		return nil
	}

	torrentID, found := matches[ph]
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

	isMusic := util.DetectContentType(fp.FileTreeParsed) == "music"

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

	// §59.184 D1: 无组名早退删除——早期种/纯中文种（场景2）靠 标题/tech/size
	// 三门验证兜底；孤儿（getSitePriority 组名空→全站照搜）与元数据获取本就无此门，
	// 辅种为唯一异类，三业务对齐。残余敞口（同碟 REMUX 变体误匹配）由注入侧
	// pieces-hash 终审兜底。

	if isMusic {
		e.logger.Debug("music torrent L2 search",
			zap.String("site", siteName),
			zap.String("title", fp.Title),
			zap.String("keyword", keyword))
	}

	results, err := adapter.SearchTorrents(ctx, config, keyword, nil)
	if err != nil {
		// §59.544 修3：任务取消路径不打噪声（CancelTask 后每颗都 err·
		// 正常收尾不需要 825 条 Warn）。
		if ctx.Err() == nil {
			// §59.544 修3：L2 搜索失败单条日志——原只进 l2s 内存统计
			// （任务结束汇总才可见），站方 hang 期 30s/颗静默爬行完全
			// 不可见（本案 79 分钟零日志=排查黑洞·§59.1529 同款）。
			e.logger.Warn("L2 search failed",
				zap.String("site", siteName),
				zap.String("keyword", keyword),
				zap.Error(err))
		}
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

	match, filterStats := VerifyMatchWithTruncationCheckAndSource(results, groupName, fp.TotalSize, fp.Title)

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
		zap.Int("emptyId", filterStats.EmptyID),
		zap.Int("groupRefute", filterStats.GroupRefute),
		zap.Int("groupNeutral", filterStats.GroupNeutral),
		zap.Int("sequelRefute", filterStats.SequelRefute),
		zap.Int("titleNeutral", filterStats.TitleNeutral),
		zap.Int("sizeInvalid", filterStats.SizeInvalid),
		zap.Int("techRefute", filterStats.TechRefute),
		zap.Int("versionRefute", filterStats.VersionRefute),
		zap.Int("sizeRefute", filterStats.SizeRefute))
	if l2s != nil {
		refuted := filterStats.GroupRefute + filterStats.SequelRefute + filterStats.TechRefute + filterStats.VersionRefute + filterStats.SizeRefute
		reason := fmt.Sprintf("未匹配(refute=%d,neutral=%d)", refuted, filterStats.GroupNeutral+filterStats.TitleNeutral)
		l2s.record(siteName, reason)
		l2s.mu.Lock()
		l2s.refuted += refuted
		l2s.neutral += filterStats.GroupNeutral + filterStats.TitleNeutral
		l2s.mu.Unlock()
	}

	return nil
}

type L2MatchResult struct {
	TorrentID string
	Title     string
	Size      int64
}

// MatchFilterStats §59.184 三态化：反驳按原因拆分、不可验证计中性
// （原 GroupMiss/SizeMiss/TitleMiss 杂烩计数消融——§59.182 排查被误导教训）。
type MatchFilterStats struct {
	EmptyID       int
	GroupRefute   int // 组名反驳：显式异组名后缀（同碟 REMUX 体积不可分，组名唯一判别器）
	GroupNeutral  int // 组名中性：无显式组名后缀（中文标题/音乐/截断）
	SequelRefute  int // 标题反驳：续集号不同
	TitleNeutral  int // 标题中性：无命中（原 TitleMiss 拒绝 → 三态改中性）
	SizeInvalid   int // size<=0 运营性拒绝（非身份判断）
	TechRefute    int // 规则 A：字段双方有值且不等价
	VersionRefute int // 规则 B：候选有版本 Token 源无（G2 修复后含无括号 REPACK）
	SizeRefute    int // 超显示等价窗
}

func VerifyMatchWithStats(results []*model.SeedingSearchResult, groupName string, sourceSize int64) (*L2MatchResult, *MatchFilterStats) {
	return VerifyMatchWithStatsAndSource(results, groupName, sourceSize, "")
}

// VerifyMatchWithStatsAndSource §59.184 三态门主链。
// 每门三态：确认（正向证据）/ 中性（语言形态无法验证→不杀）/ 反驳（身份矛盾→一票否决）。
// 裁决：任一反驳→拒；≥1 确认且 0 反驳→放行；全中性→拒。
// 排序（G3）：精确字节短路保留；通过者按 确认门数降序 → |diff| 升序。
// sourceTitle 非空时按规则 A（字段不等价→反驳）/ B（候选版本 Token 源无→禁等价匹配）过滤。
func VerifyMatchWithStatsAndSource(results []*model.SeedingSearchResult, groupName string, sourceSize int64, sourceTitle string) (*L2MatchResult, *MatchFilterStats) {
	var srcProfile *titleparser.TechProfile
	if sourceTitle != "" {
		p := titleparser.ParseTitleTech(sourceTitle)
		srcProfile = &p
	}

	// §59.214: 品牌兄弟预扫描——同组（标题含组名或中性）+ size 窗内 +
	// 无品牌词的候选存在 = 品牌构成判别器。
	hasPlainBrandSibling := false
	if srcProfile != nil && !isBrandEdition(srcProfile.EditionInfo) {
		for _, r := range results {
			if r.Size <= 0 || !CompareSizeDisplay(sourceSize, r.Size) {
				continue
			}
			if groupName != "" && !strings.Contains(strings.ToLower(r.Title), strings.ToLower(groupName)) {
				continue
			}
			if p := titleparser.ParseTitleTech(r.Title); !isBrandEdition(p.EditionInfo) {
				hasPlainBrandSibling = true
				break
			}
		}
	}

	stats := &MatchFilterStats{}
	var best *L2MatchResult
	var bestConfirms int
	var bestDiff int64

	meaningfulWords := extractMeaningfulTitleWords(sourceTitle, groupName)
	sourceCJK := extractCJKSubstrings(sourceTitle)

	for _, r := range results {
		if r.TorrentID == "" {
			stats.EmptyID++
			continue
		}
		confirms := 0

		// 门2 组名（三态）：确认（含预期组名）优先于反驳短路——
		// 中文副标题内嵌组名（如 "UBits官方DIY"）无后缀形态由 contains 命中。
		if groupName != "" {
			if strings.Contains(strings.ToLower(r.Title), strings.ToLower(groupName)) {
				confirms++
			} else if cg := ExtractGroupName(r.Title); cg != "" && !strings.EqualFold(cg, groupName) && !sameGroupFamily(cg, groupName) {
				// §59.187 ①: 同族拼写变体豁免（UBits/UBbits 同映射优堡官方）——
				// 豁免不计确认（保守），靠标题/tech/size 确认通过
				stats.GroupRefute++
				continue
			} else {
				stats.GroupNeutral++
			}
		}

		// 门3 标题（三态）：续集号反驳保持；无命中中性（原 TitleMiss 消融）。
		if sourceTitle != "" {
			switch titleKeywordState(meaningfulWords, sourceCJK, sourceTitle, r.Title) {
			case titleStateRefute:
				stats.SequelRefute++
				continue
			case titleStateConfirm:
				confirms++
			default:
				stats.TitleNeutral++
			}
		}

		// 门4 size>0：运营性必填（size 恒有比较能力，无中性态）。
		if r.Size <= 0 {
			stats.SizeInvalid++
			continue
		}

		// 门5 tech 规则 A：双方有值且不等价 → 反驳（等价组豁免保持）。
		if srcProfile != nil && techProfileConflict(*srcProfile, r.Title) {
			stats.TechRefute++
			continue
		}
		// tech 确认：≥1 字段双方有值且等价。
		if srcProfile != nil && techProfileConfirm(*srcProfile, r.Title) {
			confirms++
		}

		// 门6 精确字节短路（优先于版本 Token——保持既有语义：字节相同即同一 torrent）。
		if r.Size == sourceSize {
			return &L2MatchResult{
				TorrentID: r.TorrentID,
				Title:     r.Title,
				Size:      r.Size,
			}, stats
		}

		// 门7 版本 Token（规则 B）：候选有版本定义源无 → 仅允许精确匹配。
		// §59.214: 品牌血统反驳相对化——仅当同组同 size 窗非品牌兄弟存在
		// （集合级预扫描 hasPlainBrandSibling）时反驳品牌候选（克拉之膝双版
		// 消歧保持）；无兄弟时品牌词不构成判别器（密阳案：唯一候选即 CC 版，
		// 中文名不写 CC 是命名习惯——绝对反驳假杀实证）。
		if srcProfile != nil {
			brandRefute := false
			if hasPlainBrandSibling && !isBrandEdition(srcProfile.EditionInfo) {
				if p := titleparser.ParseTitleTech(r.Title); isBrandEdition(p.EditionInfo) {
					brandRefute = true
				}
			}
			if brandRefute || techProfileVersionDefined(*srcProfile, r.Title) {
				stats.VersionRefute++
				continue
			}
		}

		// 门8 size 显示等价（v3）：超窗=内容级反驳。
		if !CompareSizeDisplay(sourceSize, r.Size) {
			stats.SizeRefute++
			continue
		}
		confirms++ // size 等价确认

		// 全中性零证据 → 拒（size 门恒有结论，此为防御性条款）。
		if confirms == 0 {
			continue
		}

		diff := r.Size - sourceSize
		if diff < 0 {
			diff = -diff
		}
		if best == nil || confirms > bestConfirms || (confirms == bestConfirms && diff < bestDiff) {
			best = &L2MatchResult{
				TorrentID: r.TorrentID,
				Title:     r.Title,
				Size:      r.Size,
			}
			bestConfirms = confirms
			bestDiff = diff
		}
	}
	if best != nil {
		return best, stats
	}
	return nil, stats
}

// titleKeywordState §59.184: 标题门三态化（原 titleKeywordRelevant 的三态重构；
// 后者保留供 loose 轮/AudioConflictCandidates 等未三态化消费方使用）。
type titleState int

const (
	titleStateNeutral titleState = iota
	titleStateConfirm
	titleStateRefute
)

func titleKeywordState(meaningfulWords []string, sourceCJK []string, sourceTitle, candidateTitle string) titleState {
	// 续集号比较（硬反驳）：双方都有续集号且不同 → 拒绝
	sourceSequel := extractSequelNumber(sourceTitle)
	candidateSequel := extractSequelNumber(candidateTitle)
	if sourceSequel > 0 && candidateSequel > 0 && sourceSequel != candidateSequel {
		return titleStateRefute
	}

	if candidateTitle == "" {
		return titleStateNeutral
	}
	if len(meaningfulWords) == 0 && len(sourceCJK) == 0 {
		return titleStateNeutral
	}
	candLower := strings.ToLower(candidateTitle)
	if len(meaningfulWords) > 0 {
		for _, w := range meaningfulWords {
			if strings.Contains(candLower, w) {
				return titleStateConfirm
			}
		}
		// §59.184: 英文词在纯中文候选中零命中=无法验证（中性），原 return false 拒绝消融
		return titleStateNeutral
	}
	candCJK := extractCJKSubstrings(candidateTitle)
	if len(sourceCJK) > 0 && len(candCJK) > 0 {
		for _, s := range sourceCJK {
			if strings.Contains(candidateTitle, s) {
				return titleStateConfirm
			}
		}
		return titleStateNeutral
	}
	return titleStateNeutral // 原 true 技术性放行 → 显式中性
}

// techProfileConfirm §59.184: tech 门确认态——≥1 字段双方有值且等价
// （等价判定与 techProfileConflictFields 同口径：EqualFold + spec/codec 等价组）。
func techProfileConfirm(src titleparser.TechProfile, candidateTitle string) bool {
	cand := titleparser.ParseTitleTech(candidateTitle)
	fields := []struct{ name, s, c string }{
		{"Resolution", src.Resolution, cand.Resolution},
		{"VideoCodec", src.VideoCodec, cand.VideoCodec},
		{"AudioCodec", src.AudioCodec, cand.AudioCodec},
		{"HDR", src.HDR, cand.HDR},
		{"Specification", src.Specification, cand.Specification},
		{"RegionCode", src.RegionCode, cand.RegionCode},
		{"SourceType", src.SourceType, cand.SourceType},
		// §59.197: 3D 封装补入——HSBS×HOU 不同封装=不同种（300勇士案：
		// 站内两版同体积同组，双显异值才反驳；省略/"3D"泛词为空值放行）
		{"Stereo3D", src.Stereo3D, cand.Stereo3D},
	}
	for _, f := range fields {
		if f.s == "" || f.c == "" {
			continue
		}
		if strings.EqualFold(f.s, f.c) {
			return true
		}
		if f.name == "Resolution" && resolutionEquivalent(f.s, f.c) {
			return true
		}
		if f.name == "SourceType" && sourceTypeEquivalent(f.s, f.c) {
			return true
		}
		if f.name == "Specification" && specEquivalent(f.s, f.c) {
			return true
		}
		if f.name == "VideoCodec" && codecEquivalent(f.s, f.c) {
			return true
		}
	}
	return false
}

// resolutionEquivalent §59.184: 分辨率显示变体同义（4K≡2160p/8K≡4320p/2K≡1440p）。
// 经词典 variants 映射到同一 standard_key 判等——中文副标题 "4K UHD 原盘"（§59.181
// 同族路径）与英文 "2160p" 由提取器返回原始 token，比较侧规范化兜底。
func resolutionEquivalent(a, b string) bool {
	if strings.EqualFold(a, b) {
		return true
	}
	ka := titleparser.LookupDictKey("resolution", a)
	return ka != "" && ka == titleparser.LookupDictKey("resolution", b)
}

// sourceTypeEquivalent §59.185: 源类型等价——剥连字符/空格后大小写不敏感比较。
// 同血统书写变体归一（"UHD Blu-ray" ≡ "UHD BluRay"、"Blu-ray" ≡ "BluRay"——
// Just Mercy 案实证两形态并存）。血统不同（BluRay vs DVD）不等价 → 反驳。
func sourceTypeEquivalent(a, b string) bool {
	norm := func(s string) string {
		s = strings.ToLower(s)
		s = strings.ReplaceAll(s, "-", "")
		s = strings.ReplaceAll(s, " ", "")
		// §59.193: UHD 前缀消解——源 "4K Blu-ray"（4K 被 Res 消费，ST 剩 "Blu-ray"）
		// vs 候选 "2160p UHD Blu-ray"（UHD 归 ST）——同为 UHD 蓝光仅归属路径不同
		// （Queen tid=3020 tech_refute 误杀实证）。分辨率差异由 Resolution 门独立把关，
		// 此处消解前缀安全。
		if t := strings.TrimPrefix(s, "uhd"); t != "" {
			s = t
		}
		return s
	}
	na, nb := norm(a), norm(b)
	return na != "" && na == nb
}

// GroupFamilyResolver §59.187 ①: 组名→官方映射站点域（同站=同族拼写变体）。
// 启动时由 main 注入（release_group_mappings 查询闭包）；nil 时退化为纯字符串比较。
// 背景：UBbits/UBits 同族变体（v0.0.473 映射层已认，验证层未接线——
// 回到未来II tid=109991 误杀案：映射表只在 priority 选站被查询，组名反驳是原串比较）。
var GroupFamilyResolver func(group string) string

// sameGroupFamily: 两拼写均官方映射到同一站点 = 同族 → 组名反驳豁免。
func sameGroupFamily(a, b string) bool {
	if GroupFamilyResolver == nil {
		return false
	}
	fa, fb := GroupFamilyResolver(a), GroupFamilyResolver(b)
	return fa != "" && fa == fb
}

// techProfileConflict 规则 A：源标题和候选标题都有某 Token 且值不同 → 冲突。
// §59.35: 等价组迁移至公共字典（dict/*.json equivalence_group，titleparser.Equivalent）。
// web_delivery = {WEB-DL, WEBRip}；hevc_impl = {x265, HEVC}；avc_impl = {x264, AVC}。
// 仅比较视图使用（辅种冲突豁免）；重组/发布视图禁止等价替换。
func specEquivalent(a, b string) bool {
	return titleparser.Equivalent(strings.TrimSpace(a), strings.TrimSpace(b))
}

func codecEquivalent(a, b string) bool {
	return titleparser.Equivalent(strings.TrimSpace(a), strings.TrimSpace(b))
}

func techProfileConflict(src titleparser.TechProfile, candidateTitle string) bool {
	return techProfileConflictFields(src, candidateTitle, false)
}

// techProfileConflictFields 字段级冲突检查。skipAudio 豁免 AudioCodec 冲突
// （§59.36 元数据获取宽松场景：站点文件名/标题音轨标注不一致是站点数据瑕疵，
// 非不同资源信号；视频编码/分辨率/HDR/规格仍严格——错那些的元数据才致命）。
// §59.59 修复: 字段清单补 Resolution——原清单缺失致 1080p vs 2160p 不判冲突
// （时空奇旅案：本地 2160p UHD 错配 1080p REPACK2 元数据，仲裁候选与 loose 轮
// 双路径漏放行。分辨率是版本身份强信号，任何标注差异都不等价，无需豁免组）。
func techProfileConflictFields(src titleparser.TechProfile, candidateTitle string, skipAudio bool) bool {
	cand := titleparser.ParseTitleTech(candidateTitle)
	fields := []struct{ name, s, c string }{
		{"Resolution", src.Resolution, cand.Resolution},
		{"VideoCodec", src.VideoCodec, cand.VideoCodec},
		{"AudioCodec", src.AudioCodec, cand.AudioCodec},
		{"HDR", src.HDR, cand.HDR},
		{"Specification", src.Specification, cand.Specification},
		{"RegionCode", src.RegionCode, cand.RegionCode},
		// §59.185: SourceType 补入——源类型不同=不同血统（用户定案；
		// 旋律时光 BluRay 孤儿误配 DVDrip 种实证：ST 不在字段表零反驳放行）
		{"SourceType", src.SourceType, cand.SourceType},
		// §59.197: 3D 封装补入——HSBS×HOU 不同封装=不同种（300勇士案：
		// 站内两版同体积同组，双显异值才反驳；省略/"3D"泛词为空值放行）
		{"Stereo3D", src.Stereo3D, cand.Stereo3D},
	}
	for _, f := range fields {
		if f.name == "AudioCodec" && skipAudio {
			continue
		}
		if f.s != "" && f.c != "" && !strings.EqualFold(f.s, f.c) {
			// §59.184: Resolution 显示变体等价豁免（4K ≡ 2160p）
			if f.name == "Resolution" && resolutionEquivalent(f.s, f.c) {
				continue
			}
			// §59.185: SourceType 书写变体等价豁免（UHD Blu-ray ≡ UHD BluRay）
			if f.name == "SourceType" && sourceTypeEquivalent(f.s, f.c) {
				continue
			}
			// §59.30: Specification 等价组豁免（WEB-DL ≈ WEBRip 站点标注差异）
			if f.name == "Specification" && specEquivalent(f.s, f.c) {
				continue
			}
			// §59.30: VideoCodec 等价组豁免（x265 ≈ HEVC 标注习惯差异）
			if f.name == "VideoCodec" && codecEquivalent(f.s, f.c) {
				continue
			}
			return true
		}
	}
	return false
}

// techProfileVersionDefined 规则 B：候选有版本定义 Token（ReleaseVersion/RegionCode/SourcePlatform）而源无。
// stripBracketsForVersionCheck §59.181: 剥离站点标签括号内容（半角[]与全角【】）。
// 副标题匹配路径加固：优堡 "[热门]【DIY 原盘 00884】正义的慈悲"（tid=109323 实证）
// 中的 【DIY 原盘 00884】 是站点分类标签，与资源元数据无关；其中的 "DIY" 在
// 英文主标题路径被 splitMedium 消费为 "Blu-ray DIY"，但中文副标题路径无
// Blu-ray 上下文，"DIY" 绕过媒介消费误入 platform 词典 → SourcePlatform="DIY"
// → techProfileVersionDefined 误判版本差异 → fuzzy 被阻断 → size_miss。
// 剥离括号内容消除噪音，保留两条匹配路径各自的正确行为。
var reBracketContent = regexp.MustCompile(`\[[^\]]*\]|【[^】]*】`)

func stripBracketsForVersionCheck(s string) string {
	return reBracketContent.ReplaceAllString(s, "")
}

func techProfileVersionDefined(src titleparser.TechProfile, candidateTitle string) bool {
	candidateTitle = stripBracketsForVersionCheck(candidateTitle)
	cand := titleparser.ParseTitleTech(candidateTitle)
	// §59.191 b: EditionInfo（营销版式词 Remaster/Ultimate Cut/Anniversary…）
	// 移出反驳——本地中文命名常省略版式词、站方名常更全（侠女/宾虚/卡萨布兰卡
	// 反向案三连实证系统性误杀）；物理身份交给 size 显示等价指纹。
	// ReleaseVersion（REPACK/PROPER 重发布标记）保留——重新打包≠同发布。
	if cand.ReleaseVersion != "" && src.ReleaseVersion == "" {
		return true
	}
	if cand.RegionCode != "" && src.RegionCode == "" {
		return true
	}
	if cand.SourcePlatform != "" && src.SourcePlatform == "" {
		return true
	}
	// §59.214: 品牌反驳移出本函数相对化——绝对反驳在密阳案假杀唯一正确
	// 候选（tid=2670 CC 版即本地版本，中文名不写 CC 是命名习惯；本地
	// size 差 42KB 未走门6 短路实证）。品牌词仅在候选集内存在同组同窗
	// 非品牌兄弟时才构成判别器（克拉之膝双版消歧），由调用方集合级判定。
	return false
}

// isBrandEdition §59.207: 碟片发行商品牌 EditionInfo 判定（血统类）。
func isBrandEdition(ed string) bool {
	if ed == "" {
		return false
	}
	lower := strings.ToLower(ed)
	for _, b := range []string{"criterion", "moc", "wac", "warner archive", "cc"} { // §59.514: CC 归一值（词表三 pattern 统一后品牌反驳适配）
		if strings.Contains(lower, b) {
			return true
		}
	}
	return false
}

func VerifyMatch(results []*model.SeedingSearchResult, groupName string, sourceSize int64) *L2MatchResult {
	match, _ := VerifyMatchWithStats(results, groupName, sourceSize)
	return match
}

func VerifyMatchWithTruncationCheck(results []*model.SeedingSearchResult, groupName string, sourceSize int64) (*L2MatchResult, *MatchFilterStats) {
	return VerifyMatchWithTruncationCheckAndSource(results, groupName, sourceSize, "")
}

// VerifyMatchWithTruncationCheckAndSource §59.184 消融：三态化后组名不可见=中性
// （CSS 截断/中文标题自然放行），空组名重跑与截断豁免条款冗余，收敛为直接委托。
// 保留签名兼容既有调用方（orphan/reseed/metadata）。
func VerifyMatchWithTruncationCheckAndSource(results []*model.SeedingSearchResult, groupName string, sourceSize int64, sourceTitle string) (*L2MatchResult, *MatchFilterStats) {
	return VerifyMatchWithStatsAndSource(results, groupName, sourceSize, sourceTitle)
}

// AudioConflictCandidates §59.36 修订: 从候选中提取"仅因音频 token 冲突被拒"的
// ambiguous 候选（组名/标题相关性/size 全通过，视频编码/HDR/规格等其余字段无冲突）。
// 供元数据获取层 MI 仲裁；辅种/注入路径不消费（严格性保护）。
func AudioConflictCandidates(results []*model.SeedingSearchResult, groupName string, sourceSize int64, sourceTitle string) []*L2MatchResult {
	var srcProfile *titleparser.TechProfile
	if sourceTitle != "" {
		p := titleparser.ParseTitleTech(sourceTitle)
		srcProfile = &p
	}
	meaningfulWords := extractMeaningfulTitleWords(sourceTitle, groupName)
	sourceCJK := extractCJKSubstrings(sourceTitle)
	var out []*L2MatchResult
	for _, r := range results {
		if r.TorrentID == "" || r.Size <= 0 {
			continue
		}
		if groupName != "" && !strings.Contains(strings.ToLower(r.Title), strings.ToLower(groupName)) {
			if !strings.Contains(r.Title, "..") || !hasResolutionToken(r.Title) {
				continue
			}
		}
		if sourceTitle != "" && !titleKeywordRelevant(meaningfulWords, sourceCJK, sourceTitle, r.Title) {
			continue
		}
		if srcProfile == nil {
			continue
		}
		// 仅音频冲突（其余字段无冲突）才可仲裁；其他冲突仍出局
		if techProfileConflictFields(*srcProfile, r.Title, true) {
			continue // skipAudio 下仍冲突 = 视频/HDR/规格冲突，不可仲裁
		}
		if !techProfileConflictFields(*srcProfile, r.Title, false) {
			continue // 全字段无冲突——这不是冲突候选（正常匹配路径已处理）
		}
		out = append(out, &L2MatchResult{TorrentID: r.TorrentID, Title: r.Title, Size: r.Size})
	}
	return out
}

// ResolveAudioConflict §59.36 修订: MI 仲裁三级链——音频 token 冲突候选的仲裁。
//
// 证据可靠性: local MI（真测量）> 站内 MI（发布者粘贴）> token（描述）。
// 源侧必须是 local MI（双侧"声称 MI"比对会假确定性——源站 MI 贴错场景）。
//
// 参数:
//   - candidates: AudioConflictCandidates 提取的 ambiguous 候选（主轮+loose 轮合并）
//   - sourceLocalMI: 源侧本地文件 MediaInfo 文本（空 = 不可得 → 全体降级②）
//   - candidateMI: 候选 tid → 站内 MI 文本（fetcher 抓详情页现场收集）
//
// 返回: 仲裁胜出候选（①MI 一致 or ②降级放行），nil = 无可放行候选。
// AudioCodec canonical 比较（Atmos 不参与）；任一侧解析空值 = 证据不足不判拒（走②）。
func ResolveAudioConflict(candidates []*L2MatchResult, sourceLocalMI string, candidateMI map[string]string) *L2MatchResult {
	if len(candidates) == 0 {
		return nil
	}
	// ① 源 local MI 可得 → 仲裁路径
	srcAudio := ""
	if sourceLocalMI != "" {
		srcAudio = titleparser.ExtractMediaInfo(sourceLocalMI).AudioCodec
	}
	if sourceLocalMI != "" && srcAudio != "" {
		var fallback *L2MatchResult
		for _, c := range candidates {
			mi := candidateMI[c.TorrentID]
			if mi == "" {
				if fallback == nil {
					fallback = c // 候选 MI 缺失 → ②降级（记首个，一致者优先后被覆盖）
				}
				continue
			}
			candAudio := titleparser.ExtractMediaInfo(mi).AudioCodec
			if candAudio == "" {
				if fallback == nil {
					fallback = c // 解析空值 = 证据不足 → ②
				}
				continue
			}
			if strings.EqualFold(srcAudio, candAudio) {
				return c // MI 一致 → 同资源，确定性胜出
			}
			// MI 不一致 → 该候选出局（真不同版本）
		}
		return fallback
	}
	// ② 源 local MI 不可得 → skipAudio 盲放行（v0.0.644 行为，首个候选）
	return candidates[0]
}

func SearchAndVerifyMatch(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, keyword, groupName string, sourceSize int64, sourceTitle string) (*L2MatchResult, error) {
	m, _, err := SearchAndVerifyMatchWithResults(ctx, adapter, config, keyword, groupName, sourceSize, sourceTitle)
	return m, err
}

// SearchAndVerifyMatchWithResults §59.36 修订: 额外返回主轮（area=0）原始搜索结果，
// 供元数据获取层收集音频冲突候选做 MI 仲裁（主轮候选相关性最强，不可丢）。
// 辅种/注入路径用 SearchAndVerifyMatch（行为不变）。
func SearchAndVerifyMatchWithResults(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, keyword, groupName string, sourceSize int64, sourceTitle string) (*L2MatchResult, []*model.SeedingSearchResult, error) {
	if keyword == "" {
		return nil, nil, nil
	}
	results, err := adapter.SearchTorrents(ctx, config, keyword, nil)
	if err != nil {
		return nil, nil, err
	}
	match, _ := VerifyMatchWithTruncationCheckAndSource(results, groupName, sourceSize, sourceTitle)
	var allResults []*model.SeedingSearchResult
	if match == nil {
		allResults = results
	}

	// §59.26 CJK 降级重试：混合关键词（中文+英文）在部分站点（如 keepfrds
	// 标题=中文/副标题=英文）search_area=0 下跨字段无法命中。
	// 无结果且关键词同时含 CJK 和 ASCII 词时，分别用两种纯词重搜：
	//   ① 剥离 CJK（保留英文，适用标题=英文的站点）
	//   ② 剥离 ASCII（保留中文，适用标题=中文的站点，如 keepfrds 互换前）
	if match == nil && hasCJKWord(keyword) {
		for _, retryKW := range []string{stripCJKWords(keyword), stripASCIIWords(keyword)} {
			if retryKW == "" || retryKW == keyword {
				continue
			}
			// §59.196: 纯 CJK 重试词（站标题索引英文、中文副标题不在标题索引的
			// 形态——springsunday 侠女案）追加 area=1 简介搜索一轮：简介含中文
			// 片名可命中，严格验证（size+组名确认）把关。
			opts := []*model.SearchOptions{nil}
			if hasCJKWord(retryKW) && stripASCIIWords(retryKW) == retryKW {
				opts = append(opts, &model.SearchOptions{SearchArea: "1"})
			}
			for _, opt := range opts {
				retry, rErr := adapter.SearchTorrents(ctx, config, retryKW, opt)
				if rErr != nil {
					continue
				}
				allResults = append(allResults, retry...)
				if m, _ := VerifyMatchWithTruncationCheckAndSource(retry, groupName, sourceSize, sourceTitle); m != nil {
					return m, nil, nil
				}
			}
		}
	}

	// §59.184 B 链内补线：B1 形态（keyword 无 CJK 且 sourceTitle 有 CJK——提取期
	// 剥过中文前缀的签名）→ 从 sourceTitle 提取前导中文段补一轮 area=0（中文标题
	// 站索引中文名）。英线命中短路则本线永不触发；与 area=1 正交。
	if match == nil && !hasCJKWord(keyword) && hasCJKWord(sourceTitle) {
		if cnKW := leadingCJKSegment(sourceTitle); cnKW != "" && cnKW != keyword {
			if retry, rErr := adapter.SearchTorrents(ctx, config, cnKW, nil); rErr == nil && len(retry) > 0 {
				allResults = append(allResults, retry...)
				if m, _ := VerifyMatchWithTruncationCheckAndSource(retry, groupName, sourceSize, sourceTitle); m != nil {
					return m, nil, nil
				}
			}
		}
	}

	// §59.30 area=1 降级重试：部分站点（keepfrds）英文种子名不在标题索引，
	// 纯英文关键词（剧集 WEB-DL 类）search_area=0 搜不到。用简介字段重搜一轮。
	if match == nil {
		if retry, rErr := adapter.SearchTorrents(ctx, config, keyword, &model.SearchOptions{SearchArea: "1"}); rErr == nil && len(retry) > 0 {
			allResults = append(allResults, retry...)
			if m, _ := VerifyMatchWithTruncationCheckAndSource(retry, groupName, sourceSize, sourceTitle); m != nil {
				return m, nil, nil
			}
		}
	}
	return match, allResults, nil
}

// hasResolutionWord 判断标题是否含分辨率词（2160p/1080p/720p 等）。
func hasResolutionToken(s string) bool {
	lower := strings.ToLower(s)
	for _, kw := range []string{"2160p", "1080p", "1080i", "720p", "576p", "480p", "1440p", "4320p"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// SearchAndVerifyLoose §59.30: 元数据获取专用宽松验证——跳过 size 匹配，
// 仅按 组名 + 标题相关性 + TechProfile 冲突校验。适用站内同资源重发
// （REPACK 替换）导致 size 与本地不一致的场景。禁止用于注入决策
// （注入必须走 VerifyMatchWithTruncationCheckAndSource + ValidateInjection）。
// searchRoundsLoose §59.30/§59.477: 宽松轮关键词渐进降级 rounds——
// 候选筛选由 pick 注入（视频 loosePick / 音乐 musicLoosePick）。
func searchRoundsLoose(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, keyword string, pick func([]*model.SeedingSearchResult) *L2MatchResult) *L2MatchResult {
	if keyword == "" {
		return nil
	}
	// §59.30 渐进降关键词：NexusPHP 多词 AND 语义，长关键词（含分辨率/媒介词）
	// 在简介搜索（area=1）下可能全词无法命中。逐轮缩短：
	//   ① 原关键词（area1）
	//   ② 剥离分辨率词（area1）
	//   ③ 短词：剥 CJK+媒介+年份（area0 → area1）
	// §59.30 频率收敛：keepfrds 等站对连续搜索静默限流（返回默认列表），
	// loose 轮请求上限 4 次，命中即停。
	orderedKWs := []string{keyword}
	if kw2 := stripResolutionTokens(keyword); kw2 != "" && kw2 != keyword {
		orderedKWs = append(orderedKWs, kw2)
	}
	short := StripYearToken(stripCJKWords(stripResolutionTokens(stripMediumTokensLoose(keyword))))
	if short == "" || short == keyword {
		if hasCJKWord(keyword) {
			short = stripCJKWords(keyword)
		}
	}
	// §59.196: 短词标题锚守卫——短词剥完仅剩技术词（无 CJK 且无 ≥4 字母词，
	// 如 "1970 1080p"）时中止该轮：光技术词 AND 搜索返回同年同规格任意种子
	// （侠女 1970×Valerie 1970 错挂形态），技术词不构成资源身份。
	if short != "" && short != keyword && !keywordHasTitleAnchor(short) {
		short = ""
	}
	if short != "" && short != keyword {
		orderedKWs = append(orderedKWs, short)
	}

	type round struct {
		kw  string
		opt *model.SearchOptions
	}
	rounds := []round{
		{keyword, &model.SearchOptions{SearchArea: "1"}},
	}
	if len(orderedKWs) > 1 {
		rounds = append(rounds, round{orderedKWs[1], &model.SearchOptions{SearchArea: "1"}})
	}
	if len(orderedKWs) > 2 {
		rounds = append(rounds, round{orderedKWs[2], nil})
		rounds = append(rounds, round{orderedKWs[2], &model.SearchOptions{SearchArea: "1"}})
	}
	for i, rd := range rounds {
		if i >= 4 {
			break
		}
		results, err := adapter.SearchTorrents(ctx, config, rd.kw, rd.opt)
		if err != nil || len(results) == 0 {
			continue
		}
		if m := pick(results); m != nil {
			return m
		}
	}
	return nil
}

// SearchAndVerifyLoose §59.30 宽松降级（视频）——size 豁免·标题相关性
// 放宽。"元数据与版本无关"假设=视频语义（同作品不同版本）。
func SearchAndVerifyLoose(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, keyword, groupName, sourceTitle string) *L2MatchResult {
	return searchRoundsLoose(ctx, adapter, config, keyword, func(results []*model.SeedingSearchResult) *L2MatchResult {
		return loosePick(results, groupName, sourceTitle)
	})
}

// SearchAndVerifyMusicLoose §59.477 音乐宽松轮——关键词降级 rounds 复用
// （§59.30 同款），pick 换音乐门：
//  1. 标题强匹配：词边界命中 ≥max(2, 过半词)——防 any-word 子串伪命中
//     （"time"⊆"Times" 单词放行 Prince 案实证）；CJK 段 ≥2 字任一命中
//     （中文专辑名）等价强匹配
//  2. size 带 [0.25x, 4x]：同专辑格式变体（FLAC/MP3 双版）放行·
//     跨专辑/分卷拒绝（61GB 源×Vol.41 0.36GB=170x 案）
//  3. 无锚词（无有效词且无 CJK）不进宽松——宁漏勿误
//
// 视频 SearchAndVerifyLoose 原样零触碰。
func SearchAndVerifyMusicLoose(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, keyword, sourceTitle string, sourceSize int64) *L2MatchResult {
	return searchRoundsLoose(ctx, adapter, config, keyword, func(results []*model.SeedingSearchResult) *L2MatchResult {
		return musicLoosePick(results, sourceTitle, sourceSize)
	})
}

// musicLoosePick §59.477 音乐宽松轮候选筛选（纯函数可测）。
func musicLoosePick(results []*model.SeedingSearchResult, sourceTitle string, sourceSize int64) *L2MatchResult {
	words := extractMeaningfulTitleWords(sourceTitle, "")
	cjk := extractCJKSubstrings(sourceTitle)
	if len(words) == 0 && len(cjk) == 0 {
		return nil // 无锚词不进宽松（宁漏勿误）
	}
	for _, r := range results {
		if r.TorrentID == "" || r.Size <= 0 {
			continue
		}
		// size 带 [0.25x, 4x]（源与候选双向）
		if sourceSize > 0 {
			if r.Size*4 < sourceSize || r.Size > sourceSize*4 {
				continue
			}
		}
		if !musicTitleStrongMatch(words, cjk, r.Title) {
			continue
		}
		return &L2MatchResult{TorrentID: r.TorrentID, Title: r.Title, Size: r.Size}
	}
	return nil
}

// musicTitleStrongMatch §59.477 音乐标题强匹配：词边界（防子串伪命中）
// + 命中数阈值 max(2, 过半)；CJK 段 ≥2 字任一命中等价强匹配。
func musicTitleStrongMatch(words []string, cjk []string, candidateTitle string) bool {
	candLower := strings.ToLower(candidateTitle)
	if len(words) > 0 {
		// §59.486: 门下限与关键词词数挂钩——单词关键词（厂牌/艺术家名
		// 下载器名就是一词·Monstercat 83.5GB 案）max(2,..) 恒 2 → 单词
		// 1 命中永拒全候选灭（iyuu fallback exhausted 终态）。单词整词
		// 边界命中=强标识（词边界已防子串伪命中·size 带兜底）；多词
		// 维持 max(2, 过半) 不变（§59.477 毒案防线零放松）。
		need := max(2, (len(words)+1)/2)
		if len(words) < need {
			need = len(words)
		}
		hits := 0
		for _, w := range words {
			if wordBoundaryIn(w, candLower) {
				hits++
			}
		}
		if hits >= need {
			return true
		}
	}
	for _, s := range cjk {
		if len([]rune(s)) >= 2 && strings.Contains(candidateTitle, s) {
			return true
		}
	}
	return false
}

// wordBoundaryIn §59.477: 词边界包含（w 已小写）——"time" 不命中
// "Times"（边界）·命中 "time" 独立词形。
func wordBoundaryIn(w, lowerText string) bool {
	if w == "" {
		return false
	}
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(w) + `\b`).MatchString(lowerText)
}

// loosePick 宽松候选筛选（组名/标题相关/TechProfile 三层，各含标注差异豁免）。
func loosePick(results []*model.SeedingSearchResult, groupName, sourceTitle string) *L2MatchResult {
	for _, r := range results {
		if r.TorrentID == "" {
			continue
		}
		// §59.196: 双盲拒绝——源标题含 CJK×候选无 CJK×源无 meaningful 英文词
		// （纯中文名种子）：titleKeywordRelevant 尾部盲放行（return true）曾致
		// 同年同组不同电影错挂（侠女×Valerie 案）。loose 宁漏勿错——正确路径
		// 是主轮（纯 CJK area=1 补线 + size/组名确认）或 ①coverage_tid 直达。
		if hasCJKWord(sourceTitle) && !hasCJKWord(r.Title) &&
			len(extractMeaningfulTitleWords(sourceTitle, groupName)) == 0 {
			continue
		}
		// 组名（CSS 截断标题放宽，与主路径一致；
		// §59.30 纯中文标题（如 keepfrds 部分行 </a> 后无英文名）不含组名，
		// 候选含源标题 CJK 片段时视为同资源放行）
		if groupName != "" && !strings.Contains(strings.ToLower(r.Title), strings.ToLower(groupName)) {
			truncRelaxed := strings.Contains(r.Title, "..") && hasResolutionToken(r.Title)
			cjkRelaxed := false
			for _, cjk := range extractCJKSubstrings(sourceTitle) {
				if len([]rune(cjk)) >= 2 && strings.Contains(r.Title, cjk) {
					cjkRelaxed = true
					break
				}
			}
			if !truncRelaxed && !cjkRelaxed {
				continue
			}
		}
		// 标题相关性 + TechProfile
		// §59.30: 候选纯中文标题（无英文词）时 meaningfulWords 英文匹配必败，
		// 需补 CJK 分支判定（源中文名命中候选中文标题）
		if !titleKeywordRelevant(extractMeaningfulTitleWords(sourceTitle, groupName), extractCJKSubstrings(sourceTitle), sourceTitle, r.Title) {
			cjkHit := false
			for _, cjk := range extractCJKSubstrings(sourceTitle) {
				if len([]rune(cjk)) >= 2 && strings.Contains(r.Title, cjk) {
					cjkHit = true
					break
				}
			}
			if !cjkHit {
				continue
			}
		}
		// §59.30: CSS 截断标题（含 ..）的 token 不完整（如 DTS-H.. 解析成 DTS），
		// TechProfile 比较失真 → 跳过冲突检查，靠组名+标题相关性+size（主轮）兜底
		// §59.36: 音频冲突豁免（元数据获取场景）——站点数据源头的文件名/标题
		// 音轨标注不一致（FRDS 挽救计划案例：文件名 DTS-HD MA、站内标题 TrueHD7.1，
		// .torrent 实证同一资源，内容物实为 TrueHD+国配 AC-3）。音频规格差异
		// 不影响海报/声明/简介正确性；注入层 ValidateInjection 独立兜底。
		if !strings.Contains(r.Title, "..") {
			p := titleparser.ParseTitleTech(sourceTitle)
			if techProfileConflictFields(p, r.Title, true) {
				continue
			}
		}
		return &L2MatchResult{TorrentID: r.TorrentID, Title: r.Title, Size: r.Size}
	}
	return nil
}

// hasCJKWord 判断关键词是否含 CJK 词（触发降级重试的条件）。
func hasCJKWord(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// CompareSizeDisplay §59.184 v3: 显示舍入等价 + 1MB 绝对地板（替代 2% 容差）。
//
// 语义：站方显示值 = true_bytes 按其显示档（1024 基两位小数）舍入的结果，
// parseSizeStr 反解析出的 resultBytes 是舍入近似值；孤儿/本地震磁盘字节为真值。
// 同一种子两侧差的数学边界 = ½ 显示单位，按候选解析值量级重建显示档：
//
//	≥1TiB → ±0.005×1024⁴（±5.37GB）；≥1GiB → ±0.005×1024³（±5.37MB）；MB 档 → ±5KB
//
// 1MB 地板兜 2% 的真实成因（v0.0.484 考古勘误）：跨站种子文件集微差
// （站方 NFO ~52KB，647.31 vs 647.26 MiB 案）——附属文件场景不存在，此为真因。
// Just Mercy 实数锚：109323 差 5.09MB（窗内）/ 108076 差 5.38MB（窗外）。
// 假设：站方显示 1024 基与 parseSizeStr 同基；若个别站 1000 基会全量失配
// （偏差 7.4%，2% 时代同样失配）——非新增风险，全量失配时第一诊断项=查显示基。
func CompareSizeDisplay(sourceBytes, resultBytes int64) bool {
	if sourceBytes <= 0 || resultBytes <= 0 {
		return false
	}
	diff := sourceBytes - resultBytes
	if diff < 0 {
		diff = -diff
	}
	window := sizeDisplayWindow(resultBytes)
	if floor := int64(1024 * 1024); window < floor {
		window = floor
	}
	return diff <= window
}

// sizeDisplayWindow 按候选解析值量级重建站方显示档的舍入半窗（1024 基）。
func sizeDisplayWindow(parsedBytes int64) int64 {
	switch {
	case parsedBytes >= 1024*1024*1024*1024:
		return 1024 * 1024 * 1024 * 1024 / 200
	case parsedBytes >= 1024*1024*1024:
		return 1024 * 1024 * 1024 / 200
	default:
		return 1024 * 1024 / 200
	}
}
