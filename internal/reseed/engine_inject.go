package reseed

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/audit"
	clientpkg "github.com/ranfish/pt-forward/internal/client"
	"github.com/ranfish/pt-forward/internal/compliance"
	"github.com/ranfish/pt-forward/internal/fingerprint"
	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/util"
	"go.uber.org/zap"
)

// ValidateInjection 校验目标种子数据是否与源种子兼容，在注入下载器前调用。
// 返回 error 表示拒绝注入（包含拒绝原因），nil 表示通过或降级跳过。
//   - sourceSize: 0 跳过体积校验
//   - sourceName: "" 跳过组名校验
//   - sourceFileCount: 0 跳过文件结构校验
//   - sizeTolerance: 百分比阈值（如 1.0 = 1%），<=0 或 >5 时回退 1.0
func ValidateInjection(torrentData []byte, sourceSize int64, sourceName string, sourceFileCount int, sizeTolerance float64) error {
	if len(torrentData) == 0 {
		return fmt.Errorf("种子数据为空")
	}
	if sizeTolerance <= 0 || sizeTolerance > 5 {
		sizeTolerance = 1.0
	}

	meta, err := fingerprint.ComputeFromTorrent(torrentData)
	if err != nil {
		return nil
	}

	if sourceSize > 0 && meta.TotalSize > 0 {
		if diffPct := util.SizeDiffPercent(sourceSize, meta.TotalSize); diffPct > sizeTolerance {
			return fmt.Errorf("体积差异过大: 源 %.2f GiB vs 目标 %.2f GiB (%.1f%%, 阈值 %.0f%%)",
				float64(sourceSize)/1024/1024/1024,
				float64(meta.TotalSize)/1024/1024/1024,
				diffPct, sizeTolerance)
		}
	}

	if sourceName != "" && meta.Name != "" {
		sourceGroup := util.ExtractGroupName(sourceName)
		targetGroup := util.ExtractGroupName(meta.Name)
		if sourceGroup != "" && targetGroup != "" && !strings.EqualFold(sourceGroup, targetGroup) {
			return fmt.Errorf("制作组不同: 源 %s vs 目标 %s", sourceGroup, targetGroup)
		}
	}

	if sourceFileCount > 0 && meta.FileCount > 0 {
		ratio := float64(meta.FileCount) / float64(sourceFileCount)
		if ratio > 3 || ratio < 1.0/3 {
			return fmt.Errorf("文件结构不同: 源 %d 文件 vs 目标 %d 文件", sourceFileCount, meta.FileCount)
		}
	}

	return nil
}

func (e *Engine) checkEligibility(ctx context.Context, title string, task *model.ReseedTask) bool {
	if e.complianceChecker != nil && task != nil {
		result := e.complianceChecker.CheckWithTask(ctx, title, task)
		if !result.Passed {
			e.logger.Info("compliance blocked",
				zap.String("title", title),
				zap.String("category", result.Category),
				zap.String("reason", result.Reason))
			return false
		}
		return true
	}
	return checkPublishEligibility(title)
}

func checkPublishEligibility(title string) bool {
	if title == "" {
		return true
	}
	// §56.2x: MatchKeyword 防误伤（ASCII 词边界）——原裸 Contains 会把
	// "The Pornographers" 误判成人（"Porn" 子串），与 compliance.Checker 行为不一致
	for _, kw := range compliance.AdultKeywords {
		if compliance.MatchKeyword(kw, title) {
			return false
		}
	}
	for _, kw := range compliance.ForbiddenTransferKeywords {
		if compliance.MatchKeyword(kw, title) {
			return false
		}
	}
	for _, g := range compliance.ForbiddenGroups {
		if compliance.MatchKeyword(g, title) {
			return false
		}
	}
	return true
}

var errAlreadyExists = errors.New("torrent already exists in downloader")

func (e *Engine) injectMatch(ctx context.Context, match *model.ReseedMatch, task *model.ReseedTask, ps *preloadedSites) error {
	if ps == nil {
		return reseedError(ErrReseedConfig, "preloaded sites not available", nil)
	}

	if err := e.db.WithContext(ctx).Model(match).Updates(map[string]interface{}{
		"status":     model.MatchStatusInjecting,
		"updated_at": time.Now(),
	}).Error; err != nil {
		e.logger.Warn("update reseed match to injecting failed",
			zap.Uint("matchID", match.ID),
			zap.Error(err))
	}

	targetConfig := ps.configs[match.TargetSite]
	if targetConfig == nil {
		return e.failMatch(ctx, match, fmt.Sprintf("目标站配置未预加载: %s", match.TargetSite))
	}

	if httpclient.IsDomainCircuitOpen(targetConfig.Domain) {
		return e.failMatch(ctx, match, fmt.Sprintf("目标站熔断中: %s", match.TargetSite))
	}

	targetAdapter := ps.adapters[match.TargetSite]
	if targetAdapter == nil {
		return e.failMatch(ctx, match, fmt.Sprintf("目标站适配器未预加载: %s", match.TargetSite))
	}

	torrentData, err := targetAdapter.DownloadTorrent(ctx, targetConfig, match.TargetTorrentID)
	if err != nil {
		var appErr *model.AppError
		if errors.As(err, &appErr) && appErr.Code == errAdapterNotFoundCode {
			if e.currentCloudFPCache != nil {
				e.currentCloudFPCache.markDeleted(match.TargetSite, match.TargetTorrentID)
			}
			if e.deleteReporter != nil {
				reportSite := match.TargetSite
				if e.currentDomainResolver != nil {
					reportSite = e.currentDomainResolver.toDomain(match.TargetSite)
				}
				e.deleteReporter.Report(reportSite, match.TargetTorrentID)
			}
		}
		return e.failMatch(ctx, match, fmt.Sprintf("下载目标种子失败: %v", err))
	}

	dlClient, err := e.clientProvider.Get(match.ClientUID)
	if err != nil {
		return e.failMatch(ctx, match, fmt.Sprintf("获取下载器客户端失败: %v", err))
	}

	opts := model.AddTorrentOptions{
		Category: task.ReseedCategory,
		Tags:     parseTags(task.ReseedTags, "reseed", "pt-forward"),
		Paused:   true,
	}

	var sourceSize int64
	var sourceName string
	if match.SourceInfoHash != "" {
		sourceTorrent, serr := dlClient.GetTorrentByHash(ctx, match.SourceInfoHash)
		if serr == nil && sourceTorrent != nil {
			if sourceTorrent.SavePath != "" {
				opts.SavePath = sourceTorrent.SavePath
			}
			sourceSize = sourceTorrent.TotalSize
			sourceName = sourceTorrent.Name
		}
	}

	if len(torrentData) == 0 {
		return e.failMatch(ctx, match, "种子数据为空")
	}

	// 注入前校验（体积/组名/文件结构）
	sourceNameForCheck := ""
	sourceFileCountForCheck := 0
	if match.MatchMethod == "search_verify" {
		sourceNameForCheck = sourceName
		if e.fpRepo != nil && match.SourceInfoHash != "" {
			if srcFP, ferr := e.fpRepo.GetByInfoHash(ctx, match.SourceInfoHash); ferr == nil && srcFP != nil {
				sourceFileCountForCheck = srcFP.FileCount
			}
		}
	}
	if err := ValidateInjection(torrentData, sourceSize, sourceNameForCheck, sourceFileCountForCheck, task.InjectionSizeTolerance); err != nil {
		return e.failMatch(ctx, match, err.Error())
	}

	addResult, err := dlClient.AddFromFile(ctx, torrentData, opts)
	if err != nil {
		if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "exist") {
			e.logger.Debug("reseed torrent add returned exists (error path), verifying existence in downloader",
				zap.Uint("matchID", match.ID))
			return e.verifyDuplicateAndFinish(ctx, dlClient, match, "")
		}
		return e.failMatch(ctx, match, fmt.Sprintf("注入种子到下载器失败: %v", err))
	}

	if addResult.IsDuplicate {
		e.logger.Debug("reseed torrent add returned duplicate, verifying existence in downloader",
			zap.Uint("matchID", match.ID),
			zap.String("hash", addResult.InfoHash))
		return e.verifyDuplicateAndFinish(ctx, dlClient, match, addResult.InfoHash)
	}

	infoHash := addResult.InfoHash
	if infoHash == "" {
		return e.failMatch(ctx, match, "注入后未获取到 InfoHash")
	}

	recheckErr := e.waitForRecheck(ctx, dlClient, infoHash, 120*time.Second)
	if recheckErr != nil {
		_ = dlClient.PauseTorrent(ctx, infoHash)
		return e.failMatch(ctx, match, recheckErr.Error())
	}

	if err := dlClient.ResumeTorrent(ctx, infoHash); err != nil {
		e.logger.Warn("reseed restore seeding failed", zap.String("hash", infoHash), zap.Error(err))
	}

	now := time.Now()
	audit.Log("system", "reseed", "inject", "torrent", match.SourceInfoHash,
		fmt.Sprintf("辅种注入 client=%d %s→%s", match.ClientUID, match.SourceSite, match.TargetSite), "success")
	return e.db.WithContext(ctx).Model(match).Updates(map[string]interface{}{
		"status":           model.MatchStatusInjected,
		"target_info_hash": infoHash,
		"injected_at":      &now,
		"updated_at":       now,
		"directory":        opts.SavePath,
	}).Error
}

func (e *Engine) verifyDuplicateAndFinish(ctx context.Context, dlClient model.DownloaderClient, match *model.ReseedMatch, infoHash string) error {
	checkHash := infoHash
	if checkHash == "" {
		checkHash = match.TargetInfoHash
	}
	if checkHash == "" {
		checkHash = match.SourceInfoHash
	}

	if checkHash != "" {
		torrent, err := dlClient.GetTorrentByHash(ctx, checkHash)
		if err != nil {
			e.logger.Warn("failed to query downloader during duplicate verification, allowing",
				zap.Uint("matchID", match.ID),
				zap.String("hash", checkHash),
				zap.Error(err))
		} else if torrent == nil {
			e.logger.Info("reseed torrent marked as duplicate but not in downloader, skipping",
				zap.Uint("matchID", match.ID),
				zap.String("hash", checkHash))
			return e.failMatch(ctx, match, "种子被下载器标记为重复但实际不存在于活动列表")
		}
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":        model.MatchStatusSkipped,
		"injected_at":   &now,
		"decision_type": string(model.DecisionAlreadyExists),
		"updated_at":    now,
	}
	if infoHash != "" {
		updates["target_info_hash"] = infoHash
	}
	if err := e.db.WithContext(ctx).Model(match).Updates(updates).Error; err != nil {
		return err
	}
	return errAlreadyExists
}

func (e *Engine) waitForRecheck(ctx context.Context, dlClient model.DownloaderClient, infoHash string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	interval := 3 * time.Second
	gracePeriod := 15 * time.Second
	startTime := time.Now()

	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		time.Sleep(interval)
		ti, err := dlClient.GetTorrentByHash(ctx, infoHash)
		if err != nil || ti == nil {
			continue
		}
		if strings.HasPrefix(ti.State, "checking") {
			continue
		}
		if ti.Progress >= 1.0 {
			return nil
		}
		if time.Since(startTime) < gracePeriod {
			continue
		}
		return fmt.Errorf("数据未通过校验，进度 %.1f%%，状态 %s，请手动检查", ti.Progress*100, ti.State)
	}
	return fmt.Errorf("校验超时（%v），请手动检查", timeout)
}

func (e *Engine) failMatch(ctx context.Context, match *model.ReseedMatch, reason string) error {
	match.RetryCount++
	match.FailReason = reason

	decisionType := model.DecisionDownloadFailed
	switch {
	case strings.Contains(reason, "禁转") || strings.Contains(reason, "独占"):
		decisionType = model.DecisionBlockedRelease
	}

	if err := e.db.WithContext(ctx).Model(match).Updates(map[string]interface{}{
		"status":        model.MatchStatusFailed,
		"decision_type": string(decisionType),
		"fail_reason":   reason,
		"retry_count":   match.RetryCount,
		"updated_at":    time.Now(),
	}).Error; err != nil {
		if errors.Is(err, context.Canceled) {
			e.logger.Debug("failMatch update db skipped (context canceled)", zap.Uint("matchID", match.ID))
		} else {
			e.logger.Error("failMatch update db error", zap.Uint("matchID", match.ID), zap.Error(err))
		}
	}

	// 确定性失败（每次结果相同）设负面缓存，避免无限重试。
	// 瞬态失败（限流/超时/站点故障/数据校验）不设，允许快速重试。
	isDeterministic := decisionType == model.DecisionBlockedRelease ||
		strings.Contains(reason, "制作组不同") ||
		strings.Contains(reason, "体积差异过大") ||
		strings.Contains(reason, "文件结构不同") ||
		strings.Contains(reason, "种子数据为空")
	if isDeterministic {
		if err := e.SetNegativeCache(ctx, match.SourceSite, match.SourceInfoHash, match.TargetSite, match.MatchMethod, 1, 168*time.Hour); err != nil {
			e.logger.Debug("set negative cache failed", zap.Uint("matchID", match.ID), zap.Error(err))
		}
	}

	return reseedError(ErrReseedGeneric, reason, nil)
}

func hasMatchMethod(methodsStr, method string) bool {
	if methodsStr == "" {
		return true
	}
	for _, m := range ParseClientIDs(methodsStr) {
		if m == method {
			return true
		}
	}
	return false
}

func (e *Engine) filterIYUUResults(src sourceTorrent, iyuuResults map[string][]*model.IYUUReseedResult, sidMap map[int]string, targetSites, excludedSites []string) []model.Candidate {
	results := iyuuResults[src.InfoHash]
	if len(results) == 0 {
		return nil
	}

	exclSet := make(map[string]bool, len(excludedSites))
	for _, s := range excludedSites {
		exclSet[s] = true
	}

	targetSet := make(map[string]bool, len(targetSites))
	for _, s := range targetSites {
		targetSet[s] = true
	}

	var candidates []model.Candidate
	for _, result := range results {
		for _, target := range result.Targets {
			siteName := sidMap[target.Sid]
			if siteName == "" {
				continue
			}
			if exclSet[siteName] || siteName == src.SiteName {
				continue
			}
			if len(targetSet) > 0 && !targetSet[siteName] {
				continue
			}
			candidates = append(candidates, model.Candidate{
				TargetSite:      siteName,
				TargetTorrentID: fmt.Sprintf("%d", target.TorrentID),
				TargetInfoHash:  target.InfoHash,
				Confidence:      0.9,
				MatchMethod:     "iyuu",
			})
		}
	}

	return candidates
}

func (e *Engine) iyuuSidToSite(ctx context.Context, sid int) string {
	var mapping model.IYUUSiteMapping
	if err := e.db.WithContext(ctx).Where("iyuu_sid = ?", sid).First(&mapping).Error; err != nil {
		return ""
	}

	if mapping.SiteName != "" && e.siteProvider != nil {
		info, err := e.siteProvider.GetSiteInfo(ctx, mapping.SiteName)
		if err == nil && info != nil {
			return info.Name
		}
	}

	if mapping.SiteDomain != "" && e.siteProvider != nil {
		info, err := e.siteProvider.GetSiteInfoByURL(ctx, mapping.SiteDomain)
		if err == nil && info != nil {
			return info.Name
		}
	}

	return mapping.SiteName
}

// getOrComputePiecesHash 获取单个种子的 pieces_hash（DB 优先 + ExportTorrent 补全）。
func (e *Engine) getOrComputePiecesHash(ctx context.Context, infoHash string, client model.DownloaderClient) string {
	if e.fpRepo == nil {
		return ""
	}
	fp, err := e.fpRepo.GetByInfoHash(ctx, infoHash)
	if err == nil && fp != nil && fp.PiecesHash != "" {
		return fp.PiecesHash
	}
	if client == nil {
		return ""
	}
	torrentDir := client.GetTorrentDir()
	torrentData, err := clientpkg.ReadTorrentFile(torrentDir, infoHash)
	if err != nil || len(torrentData) == 0 {
		e.logger.Debug("getOrComputePiecesHash: read torrent failed", zap.String("hash", infoHash), zap.Error(err))
		return ""
	}
	fp2, err := e.fpRepo.ComputeAndSave(ctx, "", "", torrentData, "")
	if err != nil || fp2 == nil {
		return ""
	}
	return fp2.PiecesHash
}

// QueryIYUULive §59.480: IYUU 实时查询单 hash（仅 IYUU 段——获取链
// 兜底消费·无 pieces_hash 依赖）。缩写/异形名簇的根治通道：KC 50th 案
// 下载器名与皇后标题零词重叠不可搜·IYUU hash→(站,tid) 直达。
func (e *Engine) QueryIYUULive(ctx context.Context, infoHash string) []model.CoverageHit {
	if e.iyuuService == nil || infoHash == "" {
		return nil
	}
	sidMap := e.preloadIYUUSiteMappings(ctx)
	results, err := e.iyuuService.QueryReseed(ctx, []string{infoHash})
	if err != nil {
		return nil
	}
	var hits []model.CoverageHit
	for _, r := range results {
		for _, t := range r.Targets {
			if siteName, ok := sidMap[t.Sid]; ok && siteName != "" {
				hits = append(hits, model.CoverageHit{
					SiteName:  siteName,
					TorrentID: strconv.Itoa(t.TorrentID),
					Source:    "iyuu",
				})
			}
		}
	}
	return hits
}

// QuerySingleCoverage 查询单个种子的覆盖状态（IYUU + pieces_hash API）。
// 复用辅种引擎的查询方法，结果供覆盖查询写入 site_coverage_cache。
func (e *Engine) QuerySingleCoverage(ctx context.Context, infoHash string, client model.DownloaderClient) []model.CoverageHit {
	var hits []model.CoverageHit

	// IYUU 查询（§59.480 复用 QueryIYUULive 单点）
	hits = append(hits, e.QueryIYUULive(ctx, infoHash)...)

	// pieces_hash 获取
	piecesHash := e.getOrComputePiecesHash(ctx, infoHash, client)
	if piecesHash == "" {
		return hits
	}

	// pieces_hash API 查询（并发）
	if e.siteProvider == nil {
		return hits
	}
	var sites []model.Site
	e.db.WithContext(ctx).Where("enabled = ? AND is_reseed_target = ?", true, true).Find(&sites)

	type phResult struct {
		siteName string
		tid      int
		found    bool
	}
	var phResults []phResult
	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, site := range sites {
		adapter, err := e.siteProvider.GetAdapter(ctx, site.Domain)
		if err != nil || adapter == nil || !adapter.SupportsSearchByPiecesHash() {
			continue
		}
		searcher, ok := adapter.(piecesHashSearcher)
		if !ok {
			continue
		}
		config, err := e.siteProvider.GetSiteConfig(ctx, site.Domain)
		if err != nil || config == nil {
			continue
		}

		wg.Add(1)
		go func(sn string, cfg *model.SiteConfig, sr piecesHashSearcher) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			defer func() {
				if r := recover(); r != nil {
					e.logger.Error("QuerySingleCoverage panic recovered", zap.String("site", sn), zap.Any("panic", r))
				}
			}()

			result, err := sr.SearchByPiecesHash(ctx, cfg, []string{piecesHash})
			if err != nil {
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if tid, found := result[piecesHash]; found {
				phResults = append(phResults, phResult{siteName: sn, tid: tid, found: true})
			}
		}(site.Name, config, searcher)
	}
	wg.Wait()

	for _, r := range phResults {
		hits = append(hits, model.CoverageHit{
			SiteName:  r.siteName,
			TorrentID: strconv.Itoa(r.tid),
			Source:    "pieces_hash",
		})
	}

	return hits
}

// QueryBatchCoverage 批量查询多个种子的覆盖状态（IYUU 批量 + pieces_hash API 批量）。
// 复用辅种引擎的批量查询逻辑：IYUU 一次传所有 hash（Service 内部 200/批），pieces_hash 批量获取 + 并发站点查询。
func (e *Engine) QueryBatchCoverage(ctx context.Context, infoHashes []string, client model.DownloaderClient) map[string][]model.CoverageHit {
	result := make(map[string][]model.CoverageHit)

	// ① IYUU 批量查询（一次传所有 hash）
	if e.iyuuService != nil && len(infoHashes) > 0 {
		sidMap := e.preloadIYUUSiteMappings(ctx)
		iyuuResults, err := e.iyuuService.QueryReseed(ctx, infoHashes)
		if err == nil {
			for _, r := range iyuuResults {
				for _, t := range r.Targets {
					if siteName, ok := sidMap[t.Sid]; ok && siteName != "" {
						result[r.SourceInfoHash] = append(result[r.SourceInfoHash], model.CoverageHit{
							SiteName:  siteName,
							TorrentID: strconv.Itoa(t.TorrentID),
							Source:    "iyuu",
						})
					}
				}
			}
		}
	}

	// ② pieces_hash 批量获取（DB 优先 + ExportTorrent 补全）
	if e.fpRepo == nil || client == nil {
		return result
	}
	hashToPieces := make(map[string]string, len(infoHashes))
	var missing []string
	for _, ih := range infoHashes {
		// §59.210: 任务 ctx 过期短路——此前无检查，ctx 死后每哈希
		// GetByInfoHash 瞬时失败 × 万级 GORM 刷屏（PT31 12:05 一分钟
		// 1.28 万条 context deadline exceeded 实证）
		if ctx.Err() != nil {
			e.logger.Warn("pieces hash batch aborted: context expired",
				zap.Int("collected", len(hashToPieces)),
				zap.Int("remaining", len(infoHashes)-len(hashToPieces)-len(missing)))
			return result
		}
		fp, err := e.fpRepo.GetByInfoHash(ctx, ih)
		if err == nil && fp != nil && fp.PiecesHash != "" {
			hashToPieces[ih] = fp.PiecesHash
		} else {
			missing = append(missing, ih)
		}
	}
	// 补全缺失指纹（串行 ExportTorrent，但只对缺失的）
	for _, ih := range missing {
		if ctx.Err() != nil {
			break
		}
		ph := e.getOrComputePiecesHash(ctx, ih, client)
		if ph != "" {
			hashToPieces[ih] = ph
		}
	}
	if len(hashToPieces) == 0 {
		return result
	}

	// ③ pieces_hash API 批量查询（并发，每站批量 100/次）
	if e.siteProvider == nil {
		return result
	}
	var sites []model.Site
	e.db.WithContext(ctx).Where("enabled = ? AND is_reseed_target = ?", true, true).Find(&sites)

	// 去重 pieces_hash 列表
	allPieces := make([]string, 0, len(hashToPieces))
	seenPieces := make(map[string]bool)
	for _, ph := range hashToPieces {
		if !seenPieces[ph] {
			seenPieces[ph] = true
			allPieces = append(allPieces, ph)
		}
	}

	type siteBatchResult struct {
		siteName string
		matches  map[string]int
	}
	var batchResults []siteBatchResult
	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, site := range sites {
		adapter, err := e.siteProvider.GetAdapter(ctx, site.Domain)
		if err != nil || adapter == nil || !adapter.SupportsSearchByPiecesHash() {
			continue
		}
		searcher, ok := adapter.(piecesHashSearcher)
		if !ok {
			continue
		}
		config, err := e.siteProvider.GetSiteConfig(ctx, site.Domain)
		if err != nil || config == nil {
			continue
		}

		wg.Add(1)
		go func(sn string, cfg *model.SiteConfig, sr piecesHashSearcher) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			defer func() {
				if r := recover(); r != nil {
					e.logger.Error("QueryBatchCoverage panic recovered", zap.String("site", sn), zap.Any("panic", r))
				}
			}()

			siteMatches := make(map[string]int)
			for i := 0; i < len(allPieces); i += 100 {
				end := i + 100
				if end > len(allPieces) {
					end = len(allPieces)
				}
				batch := allPieces[i:end]
				matches, err := sr.SearchByPiecesHash(ctx, cfg, batch)
				if err != nil {
					return
				}
				for ph, tid := range matches {
					siteMatches[ph] = tid
				}
			}
			if len(siteMatches) > 0 {
				mu.Lock()
				batchResults = append(batchResults, siteBatchResult{siteName: sn, matches: siteMatches})
				mu.Unlock()
			}
		}(site.Name, config, searcher)
	}
	wg.Wait()

	// ④ 将 pieces_hash API 结果映射回 infoHash
	for infoHash, ph := range hashToPieces {
		for _, br := range batchResults {
			if tid, found := br.matches[ph]; found {
				result[infoHash] = append(result[infoHash], model.CoverageHit{
					SiteName:  br.siteName,
					TorrentID: strconv.Itoa(tid),
					Source:    "pieces_hash",
				})
			}
		}
	}

	return result
}
