package reseed

import (
	"context"
	"sync"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

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
