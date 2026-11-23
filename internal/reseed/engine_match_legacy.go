package reseed

import (
	"context"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

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
