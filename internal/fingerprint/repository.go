package fingerprint

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Repository struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewRepository(db *gorm.DB, logger *zap.Logger) *Repository {
	return &Repository{db: db, logger: logger}
}

func (r *Repository) GetByInfoHash(ctx context.Context, infoHash string) (*model.ContentFingerprint, error) {
	var fp model.ContentFingerprint
	err := r.db.WithContext(ctx).
		Where("info_hash = ?", infoHash).
		First(&fp).Error
	if err != nil {
		return nil, err
	}
	r.populateParsed(&fp)
	return &fp, nil
}

func (r *Repository) GetBySiteAndTorrentID(ctx context.Context, siteName, torrentID string) (*model.ContentFingerprint, error) {
	var fp model.ContentFingerprint
	err := r.db.WithContext(ctx).
		Where("site_name = ? AND torrent_id = ?", siteName, torrentID).
		First(&fp).Error
	if err != nil {
		return nil, err
	}
	r.populateParsed(&fp)
	return &fp, nil
}

func (r *Repository) Save(ctx context.Context, fp *model.ContentFingerprint) error {
	if fp.InfoHash == "" {
		return &model.AppError{Code: 40001, Message: "info_hash is required"}
	}
	return r.db.WithContext(ctx).Save(fp).Error
}

func (r *Repository) ComputeAndSave(ctx context.Context, siteName, torrentID string, torrentData []byte, title string) (*model.ContentFingerprint, error) {
	meta, err := ComputeFromTorrent(torrentData)
	if err != nil {
		return nil, &model.AppError{Code: 50001, Message: "计算指纹失败", Cause: err}
	}

	fileTreeJSON, err := json.Marshal(meta.FileTree)
	if err != nil {
		return nil, &model.AppError{Code: 50001, Message: "序列化文件树失败", Cause: err}
	}

	fp := &model.ContentFingerprint{
		InfoHash:        meta.InfoHash,
		SiteName:        siteName,
		TorrentID:       torrentID,
		PiecesHash:      meta.PiecesHash,
		TotalSize:       meta.TotalSize,
		FileCount:       meta.FileCount,
		LargestFileSize: meta.LargestFile,
		FileTree:        fileTreeJSON,
		FileTreeParsed:  meta.FileTree,
		Title:           title,
		FilesHash:       meta.FilesHash,
	}

	var existing model.ContentFingerprint
	err = r.db.WithContext(ctx).
		Where("site_name = ? AND torrent_id = ?", siteName, torrentID).
		First(&existing).Error
	if err == nil {
		fp.ID = existing.ID
		fp.CreatedAt = existing.CreatedAt
	} else {
		now := time.Now()
		fp.CreatedAt = now
	}
	fp.UpdatedAt = time.Now()

	if err := r.Save(ctx, fp); err != nil {
		return nil, fpError(ErrFPRepo, "save fingerprint", err)
	}

	r.logger.Info("fingerprint computed",
		zap.String("site", siteName),
		zap.String("torrent_id", torrentID),
		zap.String("info_hash", meta.InfoHash),
		zap.Int("files", meta.FileCount),
		zap.Int64("total_size", meta.TotalSize),
	)

	return fp, nil
}

func (r *Repository) FindByPiecesHash(ctx context.Context, piecesHash string) ([]model.ContentFingerprint, error) {
	var fps []model.ContentFingerprint
	err := r.db.WithContext(ctx).
		Where("pieces_hash = ?", piecesHash).
		Find(&fps).Error
	if err != nil {
		return nil, err
	}
	for i := range fps {
		r.populateParsed(&fps[i])
	}
	return fps, nil
}

func (r *Repository) GetByInfoHashAndSite(ctx context.Context, infoHash, siteName string) (*model.ContentFingerprint, error) {
	var fp model.ContentFingerprint
	err := r.db.WithContext(ctx).
		Where("info_hash = ? AND site_name = ?", infoHash, siteName).
		First(&fp).Error
	if err != nil {
		return nil, err
	}
	r.populateParsed(&fp)
	return &fp, nil
}

func (r *Repository) BatchGetByInfoHashes(ctx context.Context, infoHashes []string) ([]*model.ContentFingerprint, error) {
	if len(infoHashes) == 0 {
		return nil, nil
	}
	var fps []model.ContentFingerprint
	if err := r.db.WithContext(ctx).Where("info_hash IN ?", infoHashes).Find(&fps).Error; err != nil {
		return nil, err
	}
	result := make([]*model.ContentFingerprint, len(fps))
	for i := range fps {
		r.populateParsed(&fps[i])
		result[i] = &fps[i]
	}
	return result, nil
}

func (r *Repository) FindCandidatesBySite(ctx context.Context, siteName string, excludeInfoHash string, piecesHash string, totalSize int64, limit int) ([]model.ContentFingerprint, error) {
	if limit <= 0 {
		limit = 10
	}
	q := r.db.WithContext(ctx).Where("site_name = ? AND info_hash != ?", siteName, excludeInfoHash)
	switch {
	case piecesHash != "":
		q = q.Where("pieces_hash = ?", piecesHash)
	case totalSize > 0:
		q = q.Where("total_size = ?", totalSize)
	default:
		return nil, nil
	}
	var fps []model.ContentFingerprint
	if err := q.Limit(limit).Find(&fps).Error; err != nil {
		return nil, err
	}
	for i := range fps {
		r.populateParsed(&fps[i])
	}
	return fps, nil
}

func (r *Repository) FindByFilesHashAndSite(ctx context.Context, siteName, filesHash string) ([]model.ContentFingerprint, error) {
	if filesHash == "" {
		return nil, nil
	}
	var fps []model.ContentFingerprint
	err := r.db.WithContext(ctx).
		Where("site_name = ? AND files_hash = ?", siteName, filesHash).
		Find(&fps).Error
	if err != nil {
		return nil, err
	}
	for i := range fps {
		r.populateParsed(&fps[i])
	}
	return fps, nil
}

func (r *Repository) populateParsed(fp *model.ContentFingerprint) {
	if len(fp.FileTree) > 0 {
		var parsed map[string]int64
		if err := json.Unmarshal(fp.FileTree, &parsed); err == nil {
			fp.FileTreeParsed = parsed
		}
	}
}

func (r *Repository) BatchSave(ctx context.Context, fps []*model.ContentFingerprint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Create(&fps).Error
	})
}

func (r *Repository) CleanupOrphans(ctx context.Context, maxAgeDays int) (int64, error) {
	if maxAgeDays <= 0 {
		maxAgeDays = 30
	}
	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)

	result := r.db.WithContext(ctx).
		Where("updated_at < ? AND info_hash NOT IN (?)",
			cutoff,
			r.db.Table("seeding_torrent_records").Select("info_hash"),
		).
		Delete(&model.ContentFingerprint{})
	if result.Error != nil {
		return 0, fpError(ErrFPRepo, "cleanup orphan fingerprints", result.Error)
	}

	if result.RowsAffected > 0 {
		r.logger.Info("cleaned up orphan fingerprints",
			zap.Int64("count", result.RowsAffected),
			zap.Int("max_age_days", maxAgeDays),
		)
	}
	return result.RowsAffected, nil
}

func (r *Repository) GetSearchCache(ctx context.Context, site, cleanTitle string, totalSize int64) (*model.SearchCache, error) {
	var cache model.SearchCache
	err := r.db.WithContext(ctx).
		Where("site_name = ? AND clean_title = ? AND total_size = ? AND expires_at > ?",
			site, cleanTitle, totalSize, time.Now()).
		First(&cache).Error
	if err != nil {
		return nil, err
	}
	return &cache, nil
}

func (r *Repository) SaveSearchCache(ctx context.Context, site, cleanTitle string, totalSize int64, results []model.Candidate) error {
	resultsJSON, err := json.Marshal(results)
	if err != nil {
		return fpError(ErrFPRepo, "marshal search results", err)
	}

	cache := &model.SearchCache{
		SiteName:   site,
		CleanTitle: cleanTitle,
		TotalSize:  totalSize,
		Results:    string(resultsJSON),
		ExpiresAt:  time.Now().Add(24 * time.Hour),
	}

	var existing model.SearchCache
	findErr := r.db.WithContext(ctx).
		Where("site_name = ? AND clean_title = ? AND total_size = ?",
			site, cleanTitle, totalSize).
		First(&existing).Error
	if findErr == nil {
		cache.ID = existing.ID
		cache.CreatedAt = existing.CreatedAt
	}

	return r.db.WithContext(ctx).Save(cache).Error
}
