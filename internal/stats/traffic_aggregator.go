package stats

import (
	"context"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const snapshotRetentionDays = 7

type TrafficAggregator struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewTrafficAggregator(db *gorm.DB, logger *zap.Logger) *TrafficAggregator {
	return &TrafficAggregator{db: db, logger: logger.With(zap.String("component", "stats"))}
}

func (a *TrafficAggregator) AggregateHourly(ctx context.Context) error {
	now := time.Now()
	currentHour := now.Truncate(time.Hour)
	prevHour := currentHour.Add(-time.Hour)

	var snapshots []model.DownloaderSpeedSnapshot
	if err := a.db.WithContext(ctx).
		Where("recorded_at >= ? AND recorded_at < ?", prevHour, currentHour).
		Find(&snapshots).Error; err != nil {
		return err
	}

	if len(snapshots) == 0 {
		return nil
	}

	type clientKey struct {
		ClientUID uint
	}
	type clientAgg struct {
		uploadSum    float64
		downloadSum  float64
		peakUpload   int64
		peakDownload int64
		activeSum    int
		sampleCount  int
	}

	aggs := make(map[clientKey]*clientAgg)
	for _, s := range snapshots {
		key := clientKey{ClientUID: s.ClientUID}
		agg, ok := aggs[key]
		if !ok {
			agg = &clientAgg{}
			aggs[key] = agg
		}
		agg.uploadSum += float64(s.UploadSpeed)
		agg.downloadSum += float64(s.DownloadSpeed)
		if s.UploadSpeed > agg.peakUpload {
			agg.peakUpload = s.UploadSpeed
		}
		if s.DownloadSpeed > agg.peakDownload {
			agg.peakDownload = s.DownloadSpeed
		}
		agg.activeSum += s.ActiveTorrents
		agg.sampleCount++
	}

	for key, agg := range aggs {
		if agg.sampleCount == 0 {
			continue
		}
		avgUp := agg.uploadSum / float64(agg.sampleCount)
		avgDown := agg.downloadSum / float64(agg.sampleCount)
		avgActive := agg.activeSum / agg.sampleCount

		record := model.TrafficStatsHourly{
			ClientUID:          key.ClientUID,
			Hour:              prevHour,
			UploadedDelta:     int64(avgUp * 3600),
			DownloadedDelta:   int64(avgDown * 3600),
			AvgUploadSpeed:    avgUp,
			AvgDownloadSpeed:  avgDown,
			PeakUploadSpeed:   agg.peakUpload,
			PeakDownloadSpeed: agg.peakDownload,
			ActiveTorrents:    avgActive,
			SampleCount:       agg.sampleCount,
		}

		if err := a.db.WithContext(ctx).
			Where("client_uid = ? AND hour = ?", record.ClientUID, record.Hour).
			Assign(record).
			FirstOrCreate(&record).Error; err != nil {
			a.logger.Warn("traffic hourly upsert failed",
				zap.Uint("client_uid", record.ClientUID),
				zap.Time("hour", record.Hour),
				zap.Error(err))
		}
	}

	a.logger.Info("traffic hourly aggregated",
		zap.Time("hour", prevHour),
		zap.Int("clients", len(aggs)),
		zap.Int("snapshots", len(snapshots)))
	return nil
}
