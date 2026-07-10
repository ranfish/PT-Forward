package coverage

import (
	"context"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type TrackerResolver interface {
	Resolve(trackerURL string) string
}

type SiteQueryer interface {
	GetSeededSites(ctx context.Context, infoHash string) ([]string, error)
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
	HasCount     int                  `json:"has_count"`
	TotalCount   int                  `json:"total_count"`
	TargetCount  int                  `json:"target_count"`
	Sites        []model.SiteCoverageCache `json:"sites"`
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

func (s *Service) QueryCoverage(ctx context.Context, infoHash string, trackers []string) ([]model.SiteCoverageCache, error) {
	now := time.Now()
	ttl24h := now.Add(24 * time.Hour)

	results := make(map[string]model.SiteCoverageCache)

	// L0: Tracker 解析
	if s.trackerResolver != nil {
		for _, trackerURL := range trackers {
			siteName := s.trackerResolver.Resolve(trackerURL)
			if siteName == "" {
				continue
			}
			if _, exists := results[siteName]; !exists {
				results[siteName] = model.SiteCoverageCache{
					InfoHash:   infoHash,
					SiteName:   siteName,
					Status:     model.CoverageConfirmedHas,
					Source:     model.CoverageSourceTracker,
					Confidence: 1.0,
					QueriedAt:  now,
					ExpiresAt:  ttl24h,
				}
			}
		}
	}

	// L1: 读 content_fingerprints 缓存（辅种引擎已填充）
	var fps []model.ContentFingerprint
	s.db.WithContext(ctx).
		Where("info_hash = ?", infoHash).
		Find(&fps)
	for _, fp := range fps {
		if _, exists := results[fp.SiteName]; !exists {
			results[fp.SiteName] = model.SiteCoverageCache{
				InfoHash:   infoHash,
				SiteName:   fp.SiteName,
				Status:     model.CoverageConfirmedHas,
				Source:     model.CoverageSourcePiecesHash,
				Confidence: 1.0,
				TorrentID:  fp.TorrentID,
				QueriedAt:  now,
				ExpiresAt:  ttl24h,
			}
		}
	}

	// L1 补充: 读 reseed_matches 缓存
	var matches []model.ReseedMatch
	s.db.WithContext(ctx).
		Where("source_info_hash = ? AND status IN ?", infoHash, []string{"matched", "injected"}).
		Find(&matches)
	for _, m := range matches {
		if _, exists := results[m.TargetSite]; !exists {
			results[m.TargetSite] = model.SiteCoverageCache{
				InfoHash:   infoHash,
				SiteName:   m.TargetSite,
				Status:     model.CoverageConfirmedHas,
				Source:     m.MatchMethod,
				Confidence: m.Confidence,
				TorrentID:  m.TargetTorrentID,
				QueriedAt:  now,
				ExpiresAt:  ttl24h,
			}
		}
	}

	// L2: IYUU 查询
	if s.iyuu != nil {
		iyuuSites, err := s.iyuu.GetSeededSites(ctx, infoHash)
		if err != nil {
			s.logger.Warn("coverage: IYUU query failed", zap.String("hash", infoHash), zap.Error(err))
		} else {
			for _, siteName := range iyuuSites {
				if _, exists := results[siteName]; !exists {
					results[siteName] = model.SiteCoverageCache{
						InfoHash:   infoHash,
						SiteName:   siteName,
						Status:     model.CoverageProbablyHas,
						Source:     model.CoverageSourceIYUU,
						Confidence: 0.9,
						QueriedAt:  now,
						ExpiresAt:  ttl24h,
					}
				}
			}
		}
	}

	// 持久化到 cache 表
	for _, r := range results {
		s.upsertCoverage(ctx, &r)
	}

	// 转为 slice 返回
	list := make([]model.SiteCoverageCache, 0, len(results))
	for _, r := range results {
		list = append(list, r)
	}
	return list, nil
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
