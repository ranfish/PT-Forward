package publish

import (
	"context"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type SeedingConfirmation struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewSeedingConfirmation(db *gorm.DB, logger *zap.Logger) *SeedingConfirmation {
	return &SeedingConfirmation{db: db, logger: logger}
}

type DownloaderChecker interface {
	GetTorrentInfo(ctx context.Context, clientID uint, infoHash string) (*model.TorrentInfo, error)
}

func (s *SeedingConfirmation) CheckOnce(ctx context.Context, checker DownloaderChecker) error {
	var members []model.PublishGroupMember
	err := s.db.WithContext(ctx).
		Where("status IN ?", []model.MemberStatus{
			model.MemberStatusUploaded,
			model.MemberStatusInjected,
		}).
		Where("updated_at < ?", time.Now().Add(-5*time.Minute)).
		Find(&members).Error
	if err != nil {
		return publishError(ErrPublishDB, "query members for seeding confirmation", err)
	}

	if len(members) == 0 {
		return nil
	}

	if s.logger != nil {
		s.logger.Debug("seeding confirmation scan",
			zap.Int("members", len(members)),
		)
	}

	for _, member := range members {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		confirmed, err := s.confirmMember(ctx, checker, member)
		if err != nil {
			s.logger.Warn("seeding confirmation failed",
				zap.Uint("member_id", member.ID),
				zap.Error(err),
			)
			continue
		}

		if confirmed {
			if err := s.db.WithContext(ctx).Model(&member).
				Updates(map[string]interface{}{
					"status":     model.MemberStatusSeedingConfirmed,
					"updated_at": time.Now(),
				}).Error; err != nil {
				s.logger.Error("update member status failed",
					zap.Uint("member_id", member.ID),
					zap.Error(err),
				)
			} else if s.logger != nil {
				s.logger.Info("seeding confirmed",
					zap.Uint("member_id", member.ID),
					zap.String("site", member.SiteName),
				)
			}
		}
	}

	return nil
}

func (s *SeedingConfirmation) confirmMember(ctx context.Context, checker DownloaderChecker, member model.PublishGroupMember) (bool, error) {
	if member.InfoHash == "" {
		return false, nil
	}

	if member.ClientID == "" {
		return false, nil
	}

	var clientConfig model.ClientConfig
	if err := s.db.WithContext(ctx).Where("name = ?", member.ClientID).First(&clientConfig).Error; err != nil {
		return false, err
	}

	info, err := checker.GetTorrentInfo(ctx, clientConfig.ID, member.InfoHash)
	if err != nil {
		return false, err
	}

	if info == nil {
		return false, nil
	}

	return info.State == "uploading" || info.State == "stalledUP" ||
		info.State == "forcedUP" || info.IsFinished, nil
}
