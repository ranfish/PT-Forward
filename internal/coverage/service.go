package coverage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/fingerprint"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// coverageRecordTTL §59.568 刀3: 覆盖记录 TTL（一年·与 fingerprint/reseed 同语义族；
// §59.535: expires_at 语义已废弃——站上有=事实·仅新写行兜底值）
const coverageRecordTTL = 365 * 24 * time.Hour

type TrackerResolver interface {
	Resolve(trackerURL string) string
}

type SiteQueryer interface {
	GetSeededSites(ctx context.Context, infoHash string) ([]string, error)
	BatchGetSeededSites(ctx context.Context, infoHashes []string) (map[string][]string, error)
	// BatchGetCoverageHits §59.535: 共享查询层专用——完整三元组（站/tid/目标站 hash）。
	BatchGetCoverageHits(ctx context.Context, infoHashes []string) (map[string][]model.CoverageHit, error)
}

type Service struct {
	db              *gorm.DB
	iyuu            SiteQueryer
	trackerResolver TrackerResolver
	logger          *zap.Logger
	covRun          coverageRunState // §59.535 刀3: 定时任务运行态
}

func NewService(db *gorm.DB, iyuu SiteQueryer, resolver TrackerResolver, logger *zap.Logger) *Service {
	return &Service{
		db:              db,
		iyuu:            iyuu,
		trackerResolver: resolver,
		logger:          logger,
	}
}

type CoverageSummary struct {
	HasCount    int                       `json:"has_count"`
	TotalCount  int                       `json:"total_count"`
	TargetCount int                       `json:"target_count"`
	Sites       []model.SiteCoverageCache `json:"sites"`
}

func (s *Service) GetCachedCoverage(ctx context.Context, infoHash string) ([]model.SiteCoverageCache, error) {
	var records []model.SiteCoverageCache
	err := s.db.WithContext(ctx).
		Where("info_hash = ?", infoHash).
		Find(&records).Error
	return records, err
}

func (s *Service) GetBatchCachedCoverage(ctx context.Context, infoHashes []string) (map[string][]model.SiteCoverageCache, error) {
	if len(infoHashes) == 0 {
		return map[string][]model.SiteCoverageCache{}, nil
	}
	var records []model.SiteCoverageCache
	err := s.db.WithContext(ctx).
		Where("info_hash IN ?", infoHashes).
		Find(&records).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string][]model.SiteCoverageCache)
	for _, r := range records {
		result[r.InfoHash] = append(result[r.InfoHash], r)
	}
	return result, nil
}

func (s *Service) GetBatchQueryState(ctx context.Context, infoHashes []string) (map[string]bool, error) {
	if len(infoHashes) == 0 {
		return map[string]bool{}, nil
	}
	var states []model.CoverageQueryState
	err := s.db.WithContext(ctx).
		Where("info_hash IN ? AND expires_at > ?", infoHashes, time.Now()).
		Find(&states).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool)
	for _, st := range states {
		result[st.InfoHash] = true
	}
	return result, nil
}

type BatchItem struct {
	InfoHash   string
	Trackers   []string
	TorrentDir string
}

type pubHit struct {
	InfoHash   string
	TargetSite string
	TorrentID  string
}

func (s *Service) QueryBatchCoverage(ctx context.Context, items []BatchItem) error {
	if len(items) == 0 {
		return nil
	}

	now := time.Now()
	ttl := now.Add(24 * time.Hour)

	// 收集全部 info_hashes
	allHashes := make([]string, 0, len(items))
	for _, item := range items {
		allHashes = append(allHashes, item.InfoHash)
	}

	// 读已有 content_fingerprints 缓存（L1 数据源）
	var fps []model.ContentFingerprint
	s.db.WithContext(ctx).Where("info_hash IN ?", allHashes).Find(&fps)
	fpByHash := make(map[string][]model.ContentFingerprint)
	for _, fp := range fps {
		fpByHash[fp.InfoHash] = append(fpByHash[fp.InfoHash], fp)
	}

	// 读已有 reseed_matches 缓存（L1 补充）
	var matches []model.ReseedMatch
	s.db.WithContext(ctx).
		Where("source_info_hash IN ? AND status IN ?", allHashes, []string{"matched", "injected"}).
		Find(&matches)
	matchByHash := make(map[string][]model.ReseedMatch)
	for _, m := range matches {
		matchByHash[m.SourceInfoHash] = append(matchByHash[m.SourceInfoHash], m)
	}

	// 读 reseed_negative_caches（L1 负缓存：辅种引擎确认不存在的站点）
	var negCaches []model.ReseedNegativeCache
	s.db.WithContext(ctx).
		Where("source_info_hash IN ? AND expires_at > ?", allHashes, now).
		Find(&negCaches)
	negByHash := make(map[string]map[string]bool)
	for _, nc := range negCaches {
		if nc.ExcludedTargets == "" {
			continue
		}
		if negByHash[nc.SourceInfoHash] == nil {
			negByHash[nc.SourceInfoHash] = make(map[string]bool)
		}
		for _, site := range strings.Split(nc.ExcludedTargets, ",") {
			site = strings.TrimSpace(site)
			if site != "" {
				negByHash[nc.SourceInfoHash][site] = true
			}
		}
	}

	// 读 publish_result_records（L1 发布记录：我们自己的发布成功 = 确认存在）
	var pubResults []pubHit
	s.db.WithContext(ctx).
		Table("publish_result_records").
		Select("publish_candidates.info_hash AS info_hash, publish_result_records.target_site AS target_site, publish_result_records.torrent_id AS torrent_id").
		Joins("JOIN publish_candidates ON publish_result_records.candidate_id = publish_candidates.id").
		Where("publish_candidates.info_hash IN ? AND publish_result_records.status = ?", allHashes, model.PublishResultCompleted).
		Scan(&pubResults)
	pubByHash := make(map[string][]pubHit)
	for _, pr := range pubResults {
		pubByHash[pr.InfoHash] = append(pubByHash[pr.InfoHash], pr)
	}

	// L2: 批量 IYUU 查询
	var iyuuMap map[string][]string
	if s.iyuu != nil {
		var err error
		iyuuMap, err = s.iyuu.BatchGetSeededSites(ctx, allHashes)
		if err != nil {
			s.logger.Warn("batch coverage: IYUU query failed", zap.Error(err))
			iyuuMap = map[string][]string{}
		}
	} else {
		iyuuMap = map[string][]string{}
	}

	// 逐个种子处理 L0 + L1 + L2 结果并写缓存
	for _, item := range items {
		results := s.buildCoverageForItem(ctx, item, fpByHash, matchByHash, iyuuMap, negByHash, pubByHash, now, ttl)
		for _, r := range results {
			if err := s.upsertCoverage(ctx, &r); err != nil {
				s.logger.Warn("upsertCoverage failed", zap.String("hash", r.InfoHash), zap.Error(err))
			}
		}
		if err := s.upsertQueryState(ctx, item.InfoHash, now, ttl); err != nil {
			s.logger.Warn("upsertQueryState failed", zap.String("hash", item.InfoHash), zap.Error(err))
		}
	}

	s.logger.Info("batch coverage query done",
		zap.Int("torrents", len(items)),
		zap.Int("iyuu_results", len(iyuuMap)),
		zap.Int("neg_cache_records", len(negCaches)),
		zap.Int("publish_records", len(pubResults)))
	return nil
}

func (s *Service) buildCoverageForItem(
	ctx context.Context,
	item BatchItem,
	fpByHash map[string][]model.ContentFingerprint,
	matchByHash map[string][]model.ReseedMatch,
	iyuuMap map[string][]string,
	negByHash map[string]map[string]bool,
	pubByHash map[string][]pubHit,
	now, ttl time.Time,
) map[string]model.SiteCoverageCache {
	results := make(map[string]model.SiteCoverageCache)

	// L0: Tracker 解析
	if s.trackerResolver != nil {
		for _, trackerURL := range item.Trackers {
			siteName := s.trackerResolver.Resolve(trackerURL)
			if siteName == "" {
				continue
			}
			if _, exists := results[siteName]; !exists {
				results[siteName] = model.SiteCoverageCache{
					InfoHash:   item.InfoHash,
					SiteName:   siteName,
					Status:     model.CoverageConfirmedHas,
					Source:     model.CoverageSourceTracker,
					Confidence: 1.0,
					QueriedAt:  now,
					ExpiresAt:  ttl,
				}
			}
		}
	}

	// L1a: content_fingerprints 缓存
	for _, fp := range fpByHash[item.InfoHash] {
		if _, exists := results[fp.SiteName]; !exists {
			results[fp.SiteName] = model.SiteCoverageCache{
				InfoHash:   item.InfoHash,
				SiteName:   fp.SiteName,
				Status:     model.CoverageConfirmedHas,
				Source:     model.CoverageSourcePiecesHash,
				Confidence: 1.0,
				TorrentID:  fp.TorrentID,
				QueriedAt:  now,
				ExpiresAt:  ttl,
			}
		}
	}

	// L1b: reseed_matches 缓存
	for _, m := range matchByHash[item.InfoHash] {
		if _, exists := results[m.TargetSite]; !exists {
			results[m.TargetSite] = model.SiteCoverageCache{
				InfoHash:   item.InfoHash,
				SiteName:   m.TargetSite,
				Status:     model.CoverageConfirmedHas,
				Source:     m.MatchMethod,
				Confidence: m.Confidence,
				TorrentID:  m.TargetTorrentID,
				QueriedAt:  now,
				ExpiresAt:  ttl,
			}
		}
	}

	// L2: IYUU 结果
	for _, siteName := range iyuuMap[item.InfoHash] {
		if _, exists := results[siteName]; !exists {
			results[siteName] = model.SiteCoverageCache{
				InfoHash:   item.InfoHash,
				SiteName:   siteName,
				Status:     model.CoverageProbablyHas,
				Source:     model.CoverageSourceIYUU,
				Confidence: 0.9,
				QueriedAt:  now,
				ExpiresAt:  ttl,
			}
		}
	}

	// L1c: reseed_negative_caches（辅种引擎确认不存在）
	for siteName := range negByHash[item.InfoHash] {
		if _, exists := results[siteName]; !exists {
			results[siteName] = model.SiteCoverageCache{
				InfoHash:   item.InfoHash,
				SiteName:   siteName,
				Status:     model.CoverageConfirmedNot,
				Source:     model.CoverageSourcePiecesHash,
				Confidence: 0.95,
				QueriedAt:  now,
				ExpiresAt:  ttl,
			}
		}
	}

	// L1d: publish_result_records（我们自己的发布成功记录）
	for _, pr := range pubByHash[item.InfoHash] {
		if _, exists := results[pr.TargetSite]; !exists {
			results[pr.TargetSite] = model.SiteCoverageCache{
				InfoHash:   item.InfoHash,
				SiteName:   pr.TargetSite,
				Status:     model.CoverageConfirmedHas,
				Source:     model.CoverageSourcePublish,
				Confidence: 1.0,
				TorrentID:  pr.TorrentID,
				QueriedAt:  now,
				ExpiresAt:  now.Add(coverageRecordTTL),
			}
		}
	}

	return results
}

// ComputePiecesHashFromDir 从 torrent_dir 读取 .torrent 文件并计算 pieces_hash
func ComputePiecesHashFromDir(torrentDir, infoHash string) (string, error) {
	if torrentDir == "" || infoHash == "" {
		return "", fmt.Errorf("torrent_dir or info_hash empty")
	}
	path := filepath.Join(torrentDir, strings.ToLower(infoHash)+".torrent")
	data, err := os.ReadFile(path) //nolint:gosec // 内部种子目录拼接
	if err != nil {
		return "", fmt.Errorf("read torrent file %s: %w", path, err)
	}
	meta, err := fingerprint.ComputeFromTorrent(data)
	if err != nil {
		return "", fmt.Errorf("compute pieces_hash: %w", err)
	}
	return meta.PiecesHash, nil
}

func (s *Service) UpdateFromPublishResult(ctx context.Context, infoHash, siteName string) error {
	now := time.Now()
	record := model.SiteCoverageCache{
		InfoHash:   infoHash,
		SiteName:   siteName,
		Status:     model.CoverageConfirmedHas,
		Source:     model.CoverageSourcePublish,
		Confidence: 1.0,
		QueriedAt:  now,
		ExpiresAt:  now.Add(coverageRecordTTL),
	}
	return s.upsertCoverage(ctx, &record)
}

func (s *Service) upsertCoverage(ctx context.Context, record *model.SiteCoverageCache) error {
	// 17 刀1（v0.0.1668）: V-01 根修——Assign/FirstOrCreate 同指针别名
	// 致已存在行刷新静默失效（GORM finisher_api.go:354 先 Find(dest) 写回
	// DB 旧值·attrs 同指针被旧值污染→Updates 等价用旧值更新）。修复形态
	// 复用 §56.33 正例（13b17321a·fetcher.store）：attrs 用值副本·dest
	// 用本体。二次写入路径（handler_torrents ① IYUU 先写→② tracker 后写
	// 覆盖）从「后写失效」修正为「后写生效」。
	attrs := *record
	return s.db.WithContext(ctx).
		Where("info_hash = ? AND site_name = ?", record.InfoHash, record.SiteName).
		Assign(&attrs).
		FirstOrCreate(record).Error
}

func (s *Service) UpsertCoverage(ctx context.Context, record *model.SiteCoverageCache) error {
	return s.upsertCoverage(ctx, record)
}

// coverageRunState §59.535 刀3: 覆盖定时任务运行态（防重入+状态透出）。
type coverageRunState struct {
	mu        sync.Mutex
	running   bool
	lastRun   time.Time
	lastErr   string
	lastStats string
}

// RunCoverageSync §59.535 刀3: 覆盖定时任务体——全量轮询快照表 100% 完成
// hash（做种中+暂停均含·progress>=1.0·未完成不查）走共享查询层（窗口=调度
// 周期：窗口内零请求跳过=天然断点续跑；窗口外打包问 IYUU 落库）。防重入：
// 已在跑返回 ErrCoverageRunning。
var ErrCoverageRunning = errors.New("coverage sync already running")

func (s *Service) RunCoverageSync(ctx context.Context, window time.Duration) error {
	s.covRun.mu.Lock()
	if s.covRun.running {
		s.covRun.mu.Unlock()
		return ErrCoverageRunning
	}
	s.covRun.running = true
	s.covRun.mu.Unlock()
	defer func() {
		s.covRun.mu.Lock()
		s.covRun.running = false
		s.covRun.lastRun = time.Now()
		s.covRun.mu.Unlock()
	}()

	start := time.Now()
	var hashes []string
	if err := s.db.WithContext(ctx).
		Model(&model.TorrentSnapshot{}).
		Where("progress >= ?", 1.0).
		Pluck("hash", &hashes).Error; err != nil {
		s.setCovStats("", err)
		return err
	}
	if len(hashes) == 0 {
		s.setCovStats("0 hashes", nil)
		return nil
	}
	res, err := s.QuerySharedCoverage(ctx, hashes, window)
	stats := fmt.Sprintf("hashes=%d covered=%d elapsed=%s", len(hashes), len(res), time.Since(start).Round(time.Second))
	s.setCovStats(stats, err)
	if err != nil {
		s.logger.Warn("coverage sync finished with IYUU error (cached rows still returned)", zap.String("stats", stats), zap.Error(err))
	}
	s.logger.Info("coverage sync done", zap.String("stats", stats))
	return nil
}

func (s *Service) setCovStats(stats string, err error) {
	s.covRun.mu.Lock()
	defer s.covRun.mu.Unlock()
	s.covRun.lastStats = stats
	if err != nil {
		s.covRun.lastErr = err.Error()
	} else {
		s.covRun.lastErr = ""
	}
}

// CoverageStatus §59.535 刀3: 任务状态透出（running/last_run/last_stats）。
func (s *Service) CoverageStatus() map[string]interface{} {
	s.covRun.mu.Lock()
	defer s.covRun.mu.Unlock()
	return map[string]interface{}{
		"running":    s.covRun.running,
		"last_run":   s.covRun.lastRun,
		"last_stats": s.covRun.lastStats,
		"last_error": s.covRun.lastErr,
	}
}

// QuerySharedCoverage §59.535: IYUU 共享查询层——两通道（辅种任务/定时覆盖任务）
// 共用「问 IYUU」公共段，同一 hash 窗口内全局只被问一次（谁先问都落库共享）。
// 窗口语义=「多久查一轮，缓存就保鲜多久」（窗口=调度周期·调用方推导传入）：
//  1. 窗口内已查（coverage_query_state.queried_at·hash 级不分来源）→零请求，
//     直接返回覆盖表现存行
//  2. 窗口外的 hash 打包问 IYUU（BatchGetCoverageHits·QueryReseed 内部 200/批
//     +批间延迟+连续错误熔断——分批续跑基础已有）
//  3. 结果落库：probably_has·source=iyuu·含 target_info_hash；先到优先（同
//     (hash,site) 已有 tracker/pieces_hash 高置信行不降级·只补 tid/target_hash）
//  4. MarkQueried 标记本轮已查（含 IYUU 返回空的——查过了没有新信息）
//  5. 聚合返回全量（confirmed_has/probably_has·降级含窗口外旧数据）
//
// IYUU 失败降级：返回现存行 + err 透出（调用方决策——辅种宁可旧数据不断流）。
func (s *Service) QuerySharedCoverage(ctx context.Context, infoHashes []string, window time.Duration) (map[string][]model.SiteCoverageCache, error) {
	cleaned := make([]string, 0, len(infoHashes))
	seen := make(map[string]bool, len(infoHashes))
	for _, h := range infoHashes {
		if h != "" && !seen[h] {
			seen[h] = true
			cleaned = append(cleaned, h)
		}
	}
	if len(cleaned) == 0 {
		return map[string][]model.SiteCoverageCache{}, nil
	}
	now := time.Now()
	freshBefore := now.Add(-window)

	// ① 窗口分流
	var states []model.CoverageQueryState
	if err := s.db.WithContext(ctx).Where("info_hash IN ?", cleaned).Find(&states).Error; err != nil {
		s.logger.Warn("shared coverage: load query state failed", zap.Error(err))
	}
	freshSet := make(map[string]bool, len(states))
	for _, st := range states {
		if st.QueriedAt.After(freshBefore) {
			freshSet[st.InfoHash] = true
		}
	}
	stale := make([]string, 0, len(cleaned))
	for _, h := range cleaned {
		if !freshSet[h] {
			stale = append(stale, h)
		}
	}

	// ②③④ 窗口外问 IYUU + 落库 + 标记
	var iyuuErr error
	if len(stale) > 0 && s.iyuu != nil {
		hits, err := s.iyuu.BatchGetCoverageHits(ctx, stale)
		if err != nil {
			iyuuErr = err
			s.logger.Warn("shared coverage: IYUU query failed, degrading to cached rows",
				zap.Int("hashes", len(stale)), zap.Error(err))
		} else {
			ttl := now.Add(coverageRecordTTL) // §59.535: expires_at 废弃（站上有=事实不因时间失效·站方删种才失效）
			for hash, list := range hits {
				for _, hit := range list {
					rec := &model.SiteCoverageCache{
						InfoHash:       hash,
						SiteName:       hit.SiteName,
						Status:         model.CoverageProbablyHas,
						Source:         model.CoverageSourceIYUU,
						Confidence:     0.9,
						TorrentID:      hit.TorrentID,
						TargetInfoHash: hit.TargetInfoHash,
						QueriedAt:      now,
						ExpiresAt:      ttl,
					}
					if uerr := s.upsertCoverageIYUU(ctx, rec); uerr != nil {
						s.logger.Warn("shared coverage: upsert failed",
							zap.String("hash", hash), zap.String("site", hit.SiteName), zap.Error(uerr))
					}
				}
			}
			for _, h := range stale {
				if merr := s.upsertQueryState(ctx, h, now, now.Add(window)); merr != nil {
					s.logger.Warn("shared coverage: mark queried failed", zap.String("hash", h), zap.Error(merr))
				}
			}
			s.logger.Info("shared coverage: IYUU query done",
				zap.Int("queried", len(stale)), zap.Int("hit_hashes", len(hits)))
		}
	}

	// ⑤ 聚合返回（全量 cleaned·has 两态·降级含旧数据）
	var rows []model.SiteCoverageCache
	if err := s.db.WithContext(ctx).
		Where("info_hash IN ? AND status IN ?", cleaned,
			model.CoverageHasStatuses).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string][]model.SiteCoverageCache, len(cleaned))
	for _, r := range rows {
		out[r.InfoHash] = append(out[r.InfoHash], r)
	}
	return out, iyuuErr
}

// upsertCoverageIYUU §59.535: IYUU 行先到优先——同 (hash,site) 已有行（tracker/
// pieces_hash 等高置信源）不降级（status/source/confidence 保留），只补齐 IYUU
// 明细（torrent_id/target_info_hash）+刷新 queried_at；无行则插入 probably_has。
func (s *Service) upsertCoverageIYUU(ctx context.Context, rec *model.SiteCoverageCache) error {
	var existing model.SiteCoverageCache
	err := s.db.WithContext(ctx).
		Where("info_hash = ? AND site_name = ?", rec.InfoHash, rec.SiteName).
		First(&existing).Error
	if err == nil {
		return s.db.WithContext(ctx).Model(&existing).
			Updates(map[string]interface{}{
				"torrent_id":       rec.TorrentID,
				"target_info_hash": rec.TargetInfoHash,
				"queried_at":       rec.QueriedAt,
			}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if cerr := s.db.WithContext(ctx).Create(rec).Error; cerr != nil {
		// 并发首写竞争（两通道同 (hash,site) 同时 Create 撞唯一索引）→回退补明细
		var exist model.SiteCoverageCache
		if serr := s.db.WithContext(ctx).
			Where("info_hash = ? AND site_name = ?", rec.InfoHash, rec.SiteName).
			First(&exist).Error; serr == nil {
			return s.db.WithContext(ctx).Model(&exist).
				Updates(map[string]interface{}{
					"torrent_id":       rec.TorrentID,
					"target_info_hash": rec.TargetInfoHash,
					"queried_at":       rec.QueriedAt,
				}).Error
		}
		return cerr
	}
	return nil
}

func (s *Service) GetCoveredSiteNames(ctx context.Context, infoHash string) ([]string, error) {
	var names []string
	err := s.db.WithContext(ctx).
		Model(&model.SiteCoverageCache{}).
		Where("info_hash = ? AND status IN ?", infoHash,
			model.CoverageHasStatuses).
		Pluck("site_name", &names).Error
	return names, err
}

func (s *Service) upsertQueryState(ctx context.Context, infoHash string, queriedAt, expiresAt time.Time) error {
	state := model.CoverageQueryState{
		InfoHash:  infoHash,
		QueriedAt: queriedAt,
		ExpiresAt: expiresAt,
	}
	return s.db.WithContext(ctx).Save(&state).Error
}

// MarkQueried 标记查询完成（供外部调用）。
func (s *Service) MarkQueried(ctx context.Context, infoHash string, queriedAt, expiresAt time.Time) {
	if err := s.upsertQueryState(ctx, infoHash, queriedAt, expiresAt); err != nil {
		s.logger.Warn("MarkQueried upsert failed", zap.String("hash", infoHash), zap.Error(err))
	}
}

// QueryCoverage 单种子查询（保留兼容）
func (s *Service) QueryCoverage(ctx context.Context, infoHash string, trackers []string) ([]model.SiteCoverageCache, error) {
	items := []BatchItem{{InfoHash: infoHash, Trackers: trackers}}
	if err := s.QueryBatchCoverage(ctx, items); err != nil {
		return nil, err
	}
	return s.GetCachedCoverage(ctx, infoHash)
}
