package publish

import (
	"context"
	"fmt"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type LifecycleManager struct {
	db             *gorm.DB
	logger         *zap.Logger
	clientProvider model.DownloaderProvider

	globalPauseSeeders    int
	globalDeleteSeeders   int
	globalDeleteSeedHours int
}

func NewLifecycleManager(db *gorm.DB, logger *zap.Logger) *LifecycleManager {
	return &LifecycleManager{
		db:                    db,
		logger:                logger,
		globalPauseSeeders:    50,
		globalDeleteSeeders:   100,
		globalDeleteSeedHours: 168,
	}
}

func (m *LifecycleManager) SetClientProvider(cp model.DownloaderProvider) {
	m.clientProvider = cp
}

func (m *LifecycleManager) SetDefaults(pauseSeeders, deleteSeeders, deleteSeedHours int) {
	if pauseSeeders > 0 {
		m.globalPauseSeeders = pauseSeeders
	}
	if deleteSeeders > 0 {
		m.globalDeleteSeeders = deleteSeeders
	}
	if deleteSeedHours > 0 {
		m.globalDeleteSeedHours = deleteSeedHours
	}
}

type LifecycleCheckResult struct {
	CheckedGroups int
	PausedMembers int
	DeletedGroups int
	Errors        int
}

func (m *LifecycleManager) CheckOnce(ctx context.Context) (*LifecycleCheckResult, error) {
	m.reloadSettingsFromDB(ctx)

	result := &LifecycleCheckResult{}

	var groups []model.PublishGroup
	err := m.db.WithContext(ctx).
		Where("status IN ?", []model.PublishGroupStatus{
			model.GroupMonitoring,
			model.GroupPartiallyPaused,
			model.GroupAllPaused,
		}).Find(&groups).Error
	if err != nil {
		return nil, &model.AppError{Code: 50001, Message: "query lifecycle groups failed", Cause: err}
	}

	if len(groups) == 0 {
		return result, nil
	}

	for i := range groups {
		result.CheckedGroups++
		m.checkGroup(ctx, &groups[i], result)
	}

	m.logger.Info("lifecycle check completed",
		zap.Int("checked", result.CheckedGroups),
		zap.Int("paused", result.PausedMembers),
		zap.Int("deleted", result.DeletedGroups),
		zap.Int("errors", result.Errors),
	)

	return result, nil
}

func (m *LifecycleManager) checkGroup(ctx context.Context, group *model.PublishGroup, result *LifecycleCheckResult) {
	var members []model.PublishGroupMember
	if err := m.db.WithContext(ctx).Where("publish_group_id = ?", group.ID).Find(&members).Error; err != nil {
		m.logger.Warn("lifecycle: query members failed", zap.Uint("groupID", group.ID), zap.Error(err))
		result.Errors++
		return
	}

	if len(members) == 0 {
		return
	}

	pauseN, deleteN, deleteT := m.getEffectiveConfig(ctx, group.SubscriptionID)

	allAboveDelete := true
	hasTarget := false

	for i := range members {
		mem := &members[i]
		if mem.Role == "source" || mem.Status == model.MemberStatusDeleted {
			continue
		}
		hasTarget = true

		if mem.HRProtected && !mem.HRReleased {
			if mem.HRSeedStart != nil {
				elapsed := time.Since(*mem.HRSeedStart).Hours()
				if elapsed < float64(mem.HRMinSeedHours) {
					allAboveDelete = false
					continue
				}
				if err := m.db.WithContext(ctx).Model(mem).Update("hr_released", true).Error; err != nil {
					m.logger.Warn("update hr_released failed",
						zap.Uint("memberID", mem.ID),
						zap.Error(err))
				}
				m.removeHRTag(ctx, mem)
			}
		}

		if deleteT > 0 && group.SeedStartTime != nil {
			if time.Since(*group.SeedStartTime).Hours() >= float64(deleteT) {
				m.deleteGroup(ctx, group, members, result)
				return
			}
		}

		seeders := mem.Seeders
		if deleteN > 0 && seeders < deleteN {
			allAboveDelete = false
		}

		if pauseN > 0 && seeders >= pauseN && !mem.Paused &&
			mem.Status != model.MemberStatusPaused {
			m.pauseMember(ctx, mem, result)
		}
	}

	if deleteN > 0 && allAboveDelete && hasTarget {
		m.deleteGroup(ctx, group, members, result)
		return
	}

	m.transitionGroupStatus(ctx, group, result)
}

func (m *LifecycleManager) getEffectiveConfig(ctx context.Context, subscriptionID string) (pause, delete, hours int) {
	pause, delete, hours = m.globalPauseSeeders, m.globalDeleteSeeders, m.globalDeleteSeedHours

	if subscriptionID == "" {
		return
	}

	var sub model.RSSSubscription
	if err := m.db.WithContext(ctx).
		Where("id = ?", subscriptionID).
		First(&sub).Error; err != nil {
		return
	}

	if sub.LifecyclePauseSeeders > 0 {
		pause = sub.LifecyclePauseSeeders
	}
	if sub.LifecycleDeleteSeeders > 0 {
		delete = sub.LifecycleDeleteSeeders
	}
	if sub.LifecycleDeleteSeedHours > 0 {
		hours = sub.LifecycleDeleteSeedHours
	}
	return
}

func (m *LifecycleManager) pauseMember(ctx context.Context, mem *model.PublishGroupMember, result *LifecycleCheckResult) {
	if m.clientProvider != nil && mem.ClientID != "" && mem.InfoHash != "" {
		if dl, err := m.clientProvider.Get(mem.ClientID); err == nil {
			if err := dl.PauseTorrent(ctx, mem.InfoHash); err != nil {
				m.logger.Warn("lifecycle: pause torrent failed",
					zap.String("clientID", mem.ClientID),
					zap.String("infoHash", mem.InfoHash),
					zap.Error(err),
				)
			}
		}
	}

	now := time.Now()
	if err := m.db.WithContext(ctx).Model(mem).Updates(map[string]interface{}{
		"paused":    true,
		"status":    model.MemberStatusPaused,
		"status_at": &now,
	}).Error; err != nil {
		m.logger.Warn("update member to paused failed",
			zap.Uint("memberID", mem.ID),
			zap.Error(err))
	}
	result.PausedMembers++
	m.logger.Debug("lifecycle: member paused",
		zap.Uint("groupID", mem.PublishGroupID),
		zap.String("site", mem.SiteName),
		zap.Int("seeders", mem.Seeders),
	)
}

func (m *LifecycleManager) deleteGroup(ctx context.Context, group *model.PublishGroup, members []model.PublishGroupMember, result *LifecycleCheckResult) {
	deleteErrors := 0
	if m.clientProvider != nil {
		for i := range members {
			mem := &members[i]
			if mem.ClientID == "" || mem.InfoHash == "" {
				continue
			}
			if dl, err := m.clientProvider.Get(mem.ClientID); err == nil {
				if mem.HRProtected && !mem.HRReleased {
					m.removeHRTag(ctx, mem)
				}
				if err := dl.DeleteTorrent(ctx, mem.InfoHash, true); err != nil {
					m.logger.Warn("lifecycle: delete torrent failed",
						zap.String("clientID", mem.ClientID),
						zap.String("infoHash", mem.InfoHash),
						zap.Error(err),
					)
					deleteErrors++
				}
			}
		}
	}

	now := time.Now()
	if err := m.db.WithContext(ctx).Model(&model.PublishGroupMember{}).
		Where("publish_group_id = ? AND status != ?", group.ID, model.MemberStatusDeleted).
		Updates(map[string]interface{}{
			"status":    model.MemberStatusDeleted,
			"status_at": &now,
		}).Error; err != nil {
		m.logger.Warn("update group members to deleted failed",
			zap.Uint("groupID", group.ID),
			zap.Error(err))
	}

	if err := m.db.WithContext(ctx).Model(group).Updates(map[string]interface{}{
		"status":     model.GroupDeleted,
		"updated_at": now,
	}).Error; err != nil {
		m.logger.Warn("update group status to deleted failed",
			zap.Uint("groupID", group.ID),
			zap.Error(err))
	}

	result.DeletedGroups++
	if deleteErrors > 0 {
		result.Errors += deleteErrors
	}
	m.logger.Info("lifecycle: group deleted",
		zap.Uint("groupID", group.ID),
		zap.String("sourceSite", group.SourceSite),
	)
}

func (m *LifecycleManager) transitionGroupStatus(ctx context.Context, group *model.PublishGroup, result *LifecycleCheckResult) {
	var members []model.PublishGroupMember
	if err := m.db.WithContext(ctx).Where("publish_group_id = ?", group.ID).Find(&members).Error; err != nil {
		m.logger.Error("lifecycle: query group members failed",
			zap.Uint("groupID", group.ID),
			zap.Error(err),
		)
		return
	}

	allPaused := true
	anyPublishing := false
	allDone := true

	for _, mem := range members {
		switch mem.Status {
		case model.MemberStatusUploaded, model.MemberStatusSeedingConfirmed:
		case model.MemberStatusError, model.MemberStatusBanned, model.MemberStatusDeleted:
			allDone = false
		case model.MemberStatusUploading, model.MemberStatusInjected, model.MemberStatusDownloading:
			allDone = false
			anyPublishing = true
			allPaused = false
		case model.MemberStatusPaused:
			allDone = false
			allPaused = allPaused && mem.Paused
		default:
			allDone = false
			allPaused = false
		}
	}

	newStatus := group.Status
	switch {
	case allDone:
		newStatus = model.GroupMonitoring
	case allPaused:
		newStatus = model.GroupAllPaused
	case anyPublishing:
		newStatus = model.GroupPublishing
	}

	if newStatus != group.Status {
		if err := m.db.WithContext(ctx).Model(group).Updates(map[string]interface{}{
			"status":     newStatus,
			"updated_at": time.Now(),
		}).Error; err != nil {
			m.logger.Warn("update group status failed",
				zap.Uint("groupID", group.ID),
				zap.Error(err))
		}
	}
}

func (m *LifecycleManager) reloadSettingsFromDB(ctx context.Context) {
	var settings []struct {
		Key   string `gorm:"column:key"`
		Value string `gorm:"column:value"`
	}
	m.db.WithContext(ctx).Table("system_settings").
		Where("key LIKE ?", "lifecycle.%").
		Find(&settings)

	settingMap := make(map[string]string, len(settings))
	for _, s := range settings {
		settingMap[s.Key] = s.Value
	}

	if v := parseIntSetting(settingMap["lifecycle.pause_seeders"]); v > 0 {
		m.globalPauseSeeders = v
	}
	if v := parseIntSetting(settingMap["lifecycle.delete_seeders"]); v > 0 {
		m.globalDeleteSeeders = v
	}
	if v := parseIntSetting(settingMap["lifecycle.delete_seed_hours"]); v > 0 {
		m.globalDeleteSeedHours = v
	}
}

func parseIntSetting(s string) int {
	if s == "" {
		return 0
	}
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			return 0
		}
	}
	return n
}

func (m *LifecycleManager) removeHRTag(ctx context.Context, mem *model.PublishGroupMember) {
	if m.clientProvider == nil || mem.ClientID == "" || mem.InfoHash == "" {
		return
	}
	site := mem.HRSite
	if site == "" {
		site = mem.SiteName
	}
	if site == "" {
		return
	}
	hrTag := fmt.Sprintf("PROTECTED_HR_%s", site)
	if dl, err := m.clientProvider.Get(mem.ClientID); err == nil {
		if err := dl.RemoveTorrentTags(ctx, mem.InfoHash, []string{hrTag}); err != nil {
			m.logger.Warn("lifecycle: 移除 HR 保护标签失败",
				zap.String("clientID", mem.ClientID),
				zap.String("infoHash", mem.InfoHash),
				zap.Error(err),
			)
		}
	}
}
