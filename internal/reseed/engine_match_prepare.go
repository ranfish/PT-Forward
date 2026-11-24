package reseed

// §59.558 PR-4: RunTask 编排拆分产物——任务准备段三 helper。
// 锁（tasks 注册）/defer 链（自清理/panic recover/终态回写/上下文指针清理）
// 全留主函数（语义敏感）——本文件只抽纯数据准备：源收集/参数解析/预载族。

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

// runTaskSources 源收集产物（§59.558 PR-4 抽取自 RunTask）。
type runTaskSources struct {
	sourceTorrents []sourceTorrent
	clientHashes   map[string]bool
	nameSites      map[string]map[string]bool
	seedTorrentIDs map[string]string
}

// seedRecordsIndexWarnThreshold §59.566 刀2: 做种索引全量加载的增长观测阈值
// （超出打 Warn 提示分批化评估——非硬限·匹配语义要求全量）。
const seedRecordsIndexWarnThreshold = 50000

// collectRunTaskSources 源收集：seedRecords 映射 + 全下载器遍历
// （进度门+tracker 解析+nameSites 聚合）。
func (e *Engine) collectRunTaskSources(ctx context.Context, clientUIDs []uint) *runTaskSources {
	s := &runTaskSources{
		clientHashes:   make(map[string]bool),
		nameSites:      make(map[string]map[string]bool),
		seedTorrentIDs: make(map[string]string),
	}
	var seedRecords []model.SeedingTorrentRecord
	// §59.566 刀2 定性：全量=辅种匹配索引的业务语义（覆盖全部做种记录才能
	// 不漏匹配·硬 LIMIT 会破坏正确性）；列已裁剪（info_hash+torrent_id 两短
	// 列·2 万级≈数 MB）。防御=增长观测：超阈值 Warn 提示未来需分批化。
	if err := e.db.WithContext(ctx).Select("info_hash, torrent_id").Find(&seedRecords).Error; err != nil {
		e.logger.Warn("query seed records failed", zap.Error(err))
	}
	if n := len(seedRecords); n > seedRecordsIndexWarnThreshold {
		e.logger.Warn("seed records index large - consider batching",
			zap.Int("count", n), zap.Int("threshold", seedRecordsIndexWarnThreshold))
	}
	for _, r := range seedRecords {
		if r.InfoHash != "" && r.TorrentID != "" {
			s.seedTorrentIDs[r.InfoHash] = r.TorrentID
		}
	}
	seenSourceNames := make(map[string]bool)
	for _, clientUID := range clientUIDs {
		dlClient, err := e.clientProvider.Get(clientUID)
		if err != nil {
			e.logger.Warn("failed to get downloader", zap.Uint("client_uid", clientUID), zap.Error(err))
			continue
		}
		allTorrents, err := dlClient.GetAllTorrents(ctx)
		if err != nil {
			e.logger.Warn("failed to get all torrents", zap.Uint("client_uid", clientUID), zap.Error(err))
			continue
		}
		for _, t := range allTorrents {
			s.clientHashes[t.Hash] = true
			if t.Progress < 1.0 {
				continue
			}
			siteName := ""
			if e.trackerResolver != nil {
				siteName = e.trackerResolver.Resolve(t.TrackerURL)
			}
			if siteName == "" {
				continue
			}
			if t.Name != "" {
				if s.nameSites[t.Name] == nil {
					s.nameSites[t.Name] = make(map[string]bool)
				}
				s.nameSites[t.Name][siteName] = true
				if seenSourceNames[t.Name] {
					continue
				}
				seenSourceNames[t.Name] = true
			}
			s.sourceTorrents = append(s.sourceTorrents, sourceTorrent{
				InfoHash:  t.Hash,
				TorrentID: s.seedTorrentIDs[t.Hash],
				SiteName:  siteName,
				ClientUID: clientUID,
				Name:      t.Name,
				SavePath:  t.SavePath,
			})
		}
	}
	return s
}

// runTaskParams 任务参数解析（§59.558 PR-4 抽取自 RunTask）。
type runTaskParams struct {
	sourceSites   []string
	targetSites   []string
	excludedSites []string
	sizeTolerance float64
}

// resolveRunTaskParams 源/目标/排除站点 ID→名 + 容差默认。
func (e *Engine) resolveRunTaskParams(ctx context.Context, task *model.ReseedTask) runTaskParams {
	p := runTaskParams{}
	if task.SourceSiteIDs != "" {
		p.sourceSites = e.resolveSiteIDsToNames(ctx, task.SourceSiteIDs)
	}
	p.targetSites = e.resolveSiteIDsToNames(ctx, task.TargetSiteIDs)
	if task.TargetSiteExcludes != "" {
		p.excludedSites = ParseClientIDs(task.TargetSiteExcludes)
	}
	p.sizeTolerance = task.SizeTolerancePercent
	if p.sizeTolerance <= 0 {
		p.sizeTolerance = 1.0
	}
	return p
}

// runTaskPreload 预载族产物（§59.558 PR-4 抽取自 RunTask）。
// 注意：currentDomainResolver/currentCloudFPCache 的 defer 清理留主函数
// （引用本结构字段·闭包不能跨函数）。
type runTaskPreload struct {
	ps               *preloadedSites
	fpCache          *fpCache
	existingMatches  map[string][]model.ReseedMatch
	deletedCount     int
	negCache         map[string]map[string]bool
	phCache          *piecesHashCache
	dr               *domainResolver
	cfCache          *cloudFPCache
	confirmedTargets map[string]bool
	iyuuResults      map[string][]*model.IYUUReseedResult
	iyuuSidMap       map[int]string
}

// preloadRunTaskData 预载族（调用顺序=原 RunTask 顺序零变化）：
// sites→fingerprints→existingMatches→negCache→piecesHash（条件）→
// domainResolver→cloudFP→贡献上报→IYUU 预载+覆盖联动。
func (e *Engine) preloadRunTaskData(ctx context.Context, task *model.ReseedTask, s *runTaskSources, p runTaskParams, infoHashes []string) *runTaskPreload {
	pl := &runTaskPreload{}
	pl.ps = e.preloadSites(ctx, p.targetSites, p.excludedSites)
	pl.fpCache = e.preloadFingerprints(ctx, infoHashes)
	pl.existingMatches, pl.deletedCount = e.preloadExistingMatches(ctx, infoHashes, s.clientHashes)
	pl.negCache = e.preloadNegativeCache(ctx, infoHashes)

	if hasMatchMethod(task.MatchMethods, "pieces_hash") {
		scanConc := task.ScanConcurrency
		if scanConc <= 0 {
			scanConc = 10
		}
		e.preflightCheck(ctx, pl.ps, scanConc)
		pl.phCache = e.preloadPiecesHashCache(ctx, s.sourceTorrents, pl.ps, pl.fpCache, pl.negCache, scanConc, task.ID)
	}

	pl.dr = buildDomainResolver(pl.ps)
	pl.cfCache = e.preloadCloudFingerprints(ctx, pl.fpCache, pl.dr)

	if pl.phCache != nil && e.contributeReporter != nil && e.cloudFPService != nil && e.cloudFPService.IsEnabled() {
		var contributeRecords []model.CloudFPContribute
		for siteName, hashMap := range pl.phCache.bySite {
			domain := pl.dr.toDomain(siteName)
			for ph, tid := range hashMap {
				contributeRecords = append(contributeRecords, model.CloudFPContribute{
					PiecesHash: ph,
					SiteName:   domain,
					TorrentID:  strconv.Itoa(tid),
				})
			}
		}
		if len(contributeRecords) > 0 {
			e.contributeReporter.Upload(contributeRecords)
		}
	}

	phSites := 0
	if pl.phCache != nil {
		phSites = len(pl.phCache.bySite)
	}
	e.logger.Info("preload completed",
		zap.Int("fpCache", len(pl.fpCache.byKey)),
		zap.Int("existingMatches", len(pl.existingMatches)),
		zap.Int("deletedInClient", pl.deletedCount),
		zap.Int("piecesHashSites", phSites))

	pl.confirmedTargets = make(map[string]bool)
	for _, matches := range pl.existingMatches {
		for _, m := range matches {
			key := m.TargetSite + ":" + m.TargetTorrentID
			pl.confirmedTargets[key] = true
		}
	}

	if task.EngineMode == model.ReseedModeIYUUCloud && e.iyuuService != nil && hasMatchMethod(task.MatchMethods, "iyuu") {
		pl.iyuuResults = e.preloadIYUUResults(ctx, task.ID, infoHashes)
		pl.iyuuSidMap = e.preloadIYUUSiteMappings(ctx)
	}
	// §59.26: IYUU 覆盖联动
	if pl.iyuuResults != nil && pl.iyuuSidMap != nil {
		e.syncIYUUCoverage(ctx, pl.iyuuResults, pl.iyuuSidMap)
	}
	return pl
}

// logRunTaskSources 源收集统计（原 RunTask 内联段·顺序保持）。
func (e *Engine) logRunTaskSources(task *model.ReseedTask, totalSources int, nameSites map[string]map[string]bool) {
	multiSiteCount := 0
	for _, sites := range nameSites {
		if len(sites) > 1 {
			multiSiteCount++
		}
	}
	e.logger.Debug("reseed source torrent parse completed",
		zap.Int("totalSeeding", totalSources),
		zap.Int("multiSiteNames", multiSiteCount),
		zap.String("taskID", fmt.Sprintf("%d", task.ID)))
}
