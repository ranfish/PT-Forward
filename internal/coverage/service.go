package coverage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/fingerprint"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type TrackerResolver interface {
	Resolve(trackerURL string) string
}

type SiteQueryer interface {
	GetSeededSites(ctx context.Context, infoHash string) ([]string, error)
	BatchGetSeededSites(ctx context.Context, infoHashes []string) (map[string][]string, error)
}

type Service struct {
	db              *gorm.DB
	iyuu            SiteQueryer
	trackerResolver TrackerResolver
	logger          *zap.Logger
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
	InfoHash  string
	Trackers  []string
	TorrentDir string
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
		results := s.buildCoverageForItem(ctx, item, fpByHash, matchByHash, iyuuMap, now, ttl)
		for _, r := range results {
			s.upsertCoverage(ctx, &r)
		}
		s.upsertQueryState(ctx, item.InfoHash, now, ttl)
	}

	s.logger.Info("batch coverage query done",
		zap.Int("torrents", len(items)),
		zap.Int("iyuu_results", len(iyuuMap)))
	return nil
}

func (s *Service) buildCoverageForItem(
	ctx context.Context,
	item BatchItem,
	fpByHash map[string][]model.ContentFingerprint,
	matchByHash map[string][]model.ReseedMatch,
	iyuuMap map[string][]string,
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

	return results
}

// ComputePiecesHashFromDir 从 torrent_dir 读取 .torrent 文件并计算 pieces_hash
func ComputePiecesHashFromDir(torrentDir, infoHash string) (string, error) {
	if torrentDir == "" || infoHash == "" {
		return "", fmt.Errorf("torrent_dir or info_hash empty")
	}
	path := filepath.Join(torrentDir, strings.ToLower(infoHash)+".torrent")
	data, err := os.ReadFile(path)
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
		ExpiresAt:  now.Add(365 * 24 * time.Hour),
	}
	return s.upsertCoverage(ctx, &record)
}

func (s *Service) upsertCoverage(ctx context.Context, record *model.SiteCoverageCache) error {
	return s.db.WithContext(ctx).
		Where("info_hash = ? AND site_name = ?", record.InfoHash, record.SiteName).
		Assign(record).
		FirstOrCreate(record).Error
}

func (s *Service) upsertQueryState(ctx context.Context, infoHash string, queriedAt, expiresAt time.Time) error {
	state := model.CoverageQueryState{
		InfoHash:  infoHash,
		QueriedAt: queriedAt,
		ExpiresAt: expiresAt,
	}
	return s.db.WithContext(ctx).Save(&state).Error
}

// QueryCoverage 单种子查询（保留兼容）
func (s *Service) QueryCoverage(ctx context.Context, infoHash string, trackers []string) ([]model.SiteCoverageCache, error) {
	items := []BatchItem{{InfoHash: infoHash, Trackers: trackers}}
	if err := s.QueryBatchCoverage(ctx, items); err != nil {
		return nil, err
	}
	return s.GetCachedCoverage(ctx, infoHash)
}
