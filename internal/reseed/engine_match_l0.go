package reseed

import (
	"context"
	"strconv"
	"strings"
	"time"

	clientpkg "github.com/ranfish/pt-forward/internal/client"
	"github.com/ranfish/pt-forward/internal/fingerprint"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

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
