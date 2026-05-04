package publish

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/description"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/ptgen"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Pipeline struct {
	db           *gorm.DB
	logger       *zap.Logger
	siteProvider model.SiteInfoProvider
	ptgen        *ptgen.Provider
}

func NewPipeline(db *gorm.DB, logger *zap.Logger) *Pipeline {
	return &Pipeline{db: db, logger: logger, ptgen: ptgen.NewProvider(db, logger)}
}

func (p *Pipeline) SetSiteProvider(sp model.SiteInfoProvider) {
	p.siteProvider = sp
}

func (p *Pipeline) CreateTask(ctx context.Context, task *model.PublishTask) error {
	task.Status = model.PublishTaskPending
	return p.db.WithContext(ctx).Create(task).Error
}

func (p *Pipeline) GetTask(ctx context.Context, id uint) (*model.PublishTask, error) {
	var task model.PublishTask
	err := p.db.WithContext(ctx).First(&task, id).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (p *Pipeline) ListTasks(ctx context.Context, offset, limit int) ([]model.PublishTask, int64, error) {
	var tasks []model.PublishTask
	var total int64

	p.db.Model(&model.PublishTask{}).Count(&total)
	err := p.db.WithContext(ctx).Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&tasks).Error
	return tasks, total, err
}

func (p *Pipeline) UpdateTaskStatus(ctx context.Context, id uint, status model.PublishTaskStatus) error {
	return p.db.WithContext(ctx).Model(&model.PublishTask{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		}).Error
}

func (p *Pipeline) CreateCandidate(ctx context.Context, candidate *model.PublishCandidate) error {
	return p.db.WithContext(ctx).Create(candidate).Error
}

func (p *Pipeline) GetCandidate(ctx context.Context, id uint) (*model.PublishCandidate, error) {
	var candidate model.PublishCandidate
	err := p.db.WithContext(ctx).First(&candidate, id).Error
	if err != nil {
		return nil, err
	}
	return &candidate, nil
}

func (p *Pipeline) DeleteCandidate(ctx context.Context, id uint) error {
	return p.db.WithContext(ctx).Delete(&model.PublishCandidate{}, id).Error
}

func (p *Pipeline) PublishCandidate(ctx context.Context, id uint) (*model.PublishCandidate, error) {
	var candidate model.PublishCandidate
	if err := p.db.WithContext(ctx).First(&candidate, id).Error; err != nil {
		return nil, err
	}

	eligible, reason := p.CheckPublishEligibility(&candidate, "")
	if !eligible {
		_ = p.UpdateCandidateStatus(ctx, id, model.CandidateSkipped, reason)
		return nil, &model.AppError{Code: 40001, Message: fmt.Sprintf("发布合规检查未通过: %s", reason)}
	}

	if err := p.UpdateCandidateStatus(ctx, id, model.CandidatePublishing, ""); err != nil {
		return nil, &model.AppError{Code: 40001, Message: "更新候选状态失败", Cause: err}
	}

	if p.siteProvider == nil {
		candidate.PublishStatus = model.CandidatePublishing
		return &candidate, nil
	}

	sourceConfig, err := p.siteProvider.GetSiteConfig(ctx, candidate.SourceSite)
	if err != nil {
		p.logger.Warn("获取源站配置失败", zap.String("site", candidate.SourceSite), zap.Error(err))
		candidate.PublishStatus = model.CandidatePublishing
		return &candidate, nil
	}

	sourceAdapter, err := p.siteProvider.GetAdapter(ctx, candidate.SourceSite)
	if err != nil {
		p.logger.Warn("获取源站适配器失败", zap.String("site", candidate.SourceSite), zap.Error(err))
		candidate.PublishStatus = model.CandidatePublishing
		return &candidate, nil
	}

	torrentData, err := sourceAdapter.DownloadTorrent(ctx, sourceConfig, candidate.SourceTorrentID)
	if err != nil {
		_ = p.UpdateCandidateStatus(ctx, id, model.CandidateFailed, fmt.Sprintf("下载源种子失败: %v", err))
		return nil, &model.AppError{Code: 50001, Message: "下载源种子失败", Cause: err}
	}

	sourceDetail, err := sourceAdapter.GetTorrentDetail(ctx, sourceConfig, candidate.SourceTorrentID)
	if err != nil {
		p.logger.Warn("获取源种子详情失败", zap.Error(err))
	}

	targetSites := parseTargetSites(candidate.TargetSites)
	if len(targetSites) == 0 {
		candidate.PublishStatus = model.CandidatePublishing
		return &candidate, nil
	}

	var lastErr error
	publishedCount := 0

	for _, targetSite := range targetSites {
		if ctx.Err() != nil {
			break
		}

		targetConfig, err := p.siteProvider.GetSiteConfig(ctx, targetSite)
		if err != nil {
			p.logger.Warn("获取目标站配置失败", zap.String("site", targetSite), zap.Error(err))
			continue
		}

		targetAdapter, err := p.siteProvider.GetAdapter(ctx, targetSite)
		if err != nil {
			p.logger.Warn("获取目标站适配器失败", zap.String("site", targetSite), zap.Error(err))
			continue
		}

		title := candidate.TorrentName
		descriptionText := ""
		if sourceDetail != nil {
			descriptionText = sourceDetail.Description
		}

		dedupSkip := false
		if p.siteProvider != nil && title != "" {
			dedupResults, dedupErr := targetAdapter.SearchTorrents(ctx, targetConfig, title, nil)
			if dedupErr == nil {
				for _, dr := range dedupResults {
					if dr.Size == candidate.Size && dr.Size > 0 {
						p.logger.Info("目标站已存在相同资源，跳过发布",
							zap.String("target", targetSite),
							zap.String("title", dr.Title),
							zap.Int64("size", dr.Size),
						)
						_ = p.CreateResult(ctx, &model.PublishResultRecord{
							CandidateID:  id,
							SourceSite:   candidate.SourceSite,
							TargetSite:   targetSite,
							TorrentID:    dr.TorrentID,
							Status:       model.PublishResultSkipped,
							ErrorMessage: fmt.Sprintf("去重匹配: %s (size=%d)", dr.Title, dr.Size),
						})
						dedupSkip = true
						break
					}
				}
			}
		}
		if dedupSkip {
			continue
		}

		descData := &model.DescriptionData{
			SourceSite:    candidate.SourceSite,
			MediaInfoText: "",
			Screenshots:   nil,
		}
		if sourceDetail != nil {
			descData.MediaInfoText = sourceDetail.MediaInfo
			descData.Screenshots = sourceDetail.Screenshots
		}

		ptgenResult, ptgenErr := p.queryPTGen(ctx, title)
		if ptgenErr == nil && ptgenResult != nil {
			descData.PosterURL = ptgenResult.PosterURL
			if ptgenResult.RawBBCode != "" {
				descData.PTGenBody = ptgenResult.RawBBCode
			}
		}

		if descriptionText == "" && descData.PTGenBody != "" {
			descriptionText = descData.PTGenBody
		}

		var descConfig model.SiteDescConfig
		siteInfo, siteInfoErr := p.siteProvider.GetSiteInfo(ctx, targetSite)
		if siteInfoErr == nil && siteInfo != nil {
			siteConfig, cfgErr := p.siteProvider.GetSiteConfig(ctx, siteInfo.BaseURL)
			if cfgErr == nil && siteConfig != nil {
				descConfig = siteConfig.Publish.Description
			}
		}

		if descConfig.Format != "" || descConfig.TemplateOverride != "" {
			renderer := description.NewRenderer(descConfig.Format)
			if rendered, err := renderer.Render(descData, descConfig); err == nil && rendered != "" {
				descriptionText = rendered
			}
		}

		pubReq := &model.PublishRequest{
			TorrentData:     torrentData,
			Title:           title,
			Description:     descriptionText,
			SourceSite:      candidate.SourceSite,
			SourceInfoHash:  candidate.InfoHash,
			SourceTorrentID: candidate.SourceTorrentID,
			TargetSite:      targetSite,
			FormFields:      make(map[string]string),
		}

		if ptgenResult != nil {
			if ptgenResult.IMDBURL != "" {
				pubReq.IMDbLink = ptgenResult.IMDBURL
			}
			if ptgenResult.DoubanURL != "" {
				pubReq.DoubanLink = ptgenResult.DoubanURL
			}
		}

		if sourceDetail != nil {
			if sourceDetail.Category != "" {
				pubReq.FormFields["category"] = sourceDetail.Category
			}
			if sourceDetail.Source != "" {
				pubReq.FormFields["source"] = sourceDetail.Source
			}
			if sourceDetail.Resolution != "" {
				pubReq.FormFields["resolution"] = sourceDetail.Resolution
			}
			if sourceDetail.Codec != "" {
				pubReq.FormFields["codec"] = sourceDetail.Codec
			}
			if sourceDetail.IMDbID != "" {
				pubReq.FormFields["imdb"] = sourceDetail.IMDbID
			}
			pubReq.MediaInfo = sourceDetail.MediaInfo
			pubReq.Screenshots = sourceDetail.Screenshots
		}

		resp, err := targetAdapter.UploadTorrent(ctx, targetConfig, pubReq)
		if err != nil {
			p.logger.Warn("上传到目标站失败",
				zap.String("target", targetSite),
				zap.Error(err),
			)
			lastErr = err

			_ = p.CreateResult(ctx, &model.PublishResultRecord{
				CandidateID:  id,
				SourceSite:   candidate.SourceSite,
				TargetSite:   targetSite,
				TorrentID:    candidate.SourceTorrentID,
				Status:       model.PublishResultFailed,
				ErrorMessage: err.Error(),
			})
			continue
		}

		publishedCount++
		_ = p.CreateResult(ctx, &model.PublishResultRecord{
			CandidateID: id,
			SourceSite:  candidate.SourceSite,
			TargetSite:  targetSite,
			TorrentID:   resp.TorrentID,
			Status:      model.PublishResultCompleted,
			PublishURL:  resp.DetailURL,
		})
	}

	if publishedCount > 0 {
		now := time.Now()
		if err := p.db.WithContext(ctx).Model(&model.PublishCandidate{}).
			Where("id = ?", id).
			Updates(map[string]interface{}{
				"publish_status":     model.CandidateDone,
				"download_completed": true,
				"completed_at":       &now,
				"updated_at":         now,
			}).Error; err != nil {
			p.logger.Error("更新发布完成状态失败", zap.Uint("id", id), zap.Error(err))
		}
		candidate.PublishStatus = model.CandidateDone
	} else if lastErr != nil {
		if err := p.UpdateCandidateStatus(ctx, id, model.CandidateFailed, lastErr.Error()); err != nil {
			p.logger.Error("更新发布失败状态失败", zap.Uint("id", id), zap.Error(err))
		}
		candidate.PublishStatus = model.CandidateFailed
	}

	return &candidate, nil
}

func (p *Pipeline) ListPendingCandidates(ctx context.Context, limit int) ([]model.PublishCandidate, error) {
	var candidates []model.PublishCandidate
	q := p.db.WithContext(ctx).
		Where("publish_status IN ?", []string{"pending", "downloading"}).
		Order("created_at ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&candidates).Error
	return candidates, err
}

func (p *Pipeline) UpdateCandidateStatus(ctx context.Context, id uint, status model.PublishCandidateStatus, result string) error {
	return p.db.WithContext(ctx).Model(&model.PublishCandidate{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"publish_status": status,
			"publish_result": result,
			"updated_at":     time.Now(),
		}).Error
}

func (p *Pipeline) MarkDownloadCompleted(ctx context.Context, id uint, savePath, filePath string) error {
	now := time.Now()
	return p.db.WithContext(ctx).Model(&model.PublishCandidate{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"download_completed": true,
			"completed_at":       &now,
			"local_save_path":    savePath,
			"local_file_path":    filePath,
			"publish_status":     model.CandidateCompleted,
			"updated_at":         now,
		}).Error
}

func (p *Pipeline) CreateResult(ctx context.Context, result *model.PublishResultRecord) error {
	return p.db.WithContext(ctx).Create(result).Error
}

func (p *Pipeline) ListResults(ctx context.Context, candidateID uint, limit int) ([]model.PublishResultRecord, error) {
	var results []model.PublishResultRecord
	q := p.db.WithContext(ctx).Order("created_at DESC")
	if candidateID > 0 {
		q = q.Where("candidate_id = ?", candidateID)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&results).Error
	return results, err
}

func (p *Pipeline) CheckPublishEligibility(candidate *model.PublishCandidate, targetSite string) (bool, string) {
	title := candidate.TorrentName

	blockedKeywords := []string{"禁转", "独占", "谢绝转载", "限时禁转", "严禁转载"}
	for _, kw := range blockedKeywords {
		if strings.Contains(title, kw) {
			return false, fmt.Sprintf("标题包含禁止转载关键词: %s", kw)
		}
	}

	blockedGroups := []string{"CatEDU"}
	for _, group := range blockedGroups {
		if strings.Contains(title, group) {
			return false, fmt.Sprintf("禁止转载小组资源: %s", group)
		}
	}

	return true, ""
}

func (p *Pipeline) ProcessPending(ctx context.Context) error {
	candidates, err := p.ListPendingCandidates(ctx, 100)
	if err != nil {
		return fmt.Errorf("list pending candidates: %w", err)
	}

	if len(candidates) == 0 {
		return nil
	}

	p.logger.Info("processing pending candidates", zap.Int("count", len(candidates)))

	for i := range candidates {
		c := &candidates[i]

		eligible, reason := p.CheckPublishEligibility(c, "")
		if !eligible {
			if err := p.UpdateCandidateStatus(ctx, c.ID, model.CandidateSkipped, reason); err != nil {
				p.logger.Warn("update candidate status failed",
					zap.Uint("id", c.ID),
					zap.Error(err),
				)
			}
			continue
		}

		if c.DownloadCompleted {
			if err := p.UpdateCandidateStatus(ctx, c.ID, model.CandidateDone, ""); err != nil {
				p.logger.Warn("update candidate status failed",
					zap.Uint("id", c.ID),
					zap.Error(err),
				)
			}
			continue
		}

		if c.PublishStatus == model.CandidateCompleted {
			if err := p.MarkDownloadCompleted(ctx, c.ID, c.LocalSavePath, c.LocalFilePath); err != nil {
				p.logger.Warn("mark download completed failed",
					zap.Uint("id", c.ID),
					zap.Error(err),
				)
			}
			continue
		}

		if time.Since(c.CreatedAt) > 24*time.Hour && !c.DownloadCompleted {
			if err := p.UpdateCandidateStatus(ctx, c.ID, model.CandidateOrphan, "候选超过 24 小时未完成下载"); err != nil {
				p.logger.Warn("orphan candidate status failed",
					zap.Uint("id", c.ID),
					zap.Error(err),
				)
			}
			continue
		}

		if err := p.UpdateCandidateStatus(ctx, c.ID, model.CandidatePending, ""); err != nil {
			p.logger.Warn("update candidate status failed",
				zap.Uint("id", c.ID),
				zap.Error(err),
			)
		}
	}

	return nil
}

func parseTargetSites(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func (p *Pipeline) OnTorrents(ctx context.Context, events []model.TorrentEvent) error {
	for i := range events {
		ev := &events[i]
		if ev.MatchedRuleName == "" {
			continue
		}

		candidate := &model.PublishCandidate{
			SourceSite:      ev.SiteName,
			SourceTorrentID: ev.TorrentID,
			InfoHash:        ev.InfoHash,
			TorrentName:     ev.Title,
			Size:            ev.Size,
			Discount:        ev.Discount,
			PublishStatus:   model.CandidatePending,
			Role:            model.RoleDownload,
		}

		if err := p.CreateCandidate(ctx, candidate); err != nil {
			p.logger.Warn("create publish candidate failed",
				zap.String("torrent", ev.TorrentID),
				zap.Error(err),
			)
		}
	}
	return nil
}

func (p *Pipeline) queryPTGen(ctx context.Context, title string) (*model.PTGenResult, error) {
	if p.ptgen == nil {
		return nil, nil
	}
	if title == "" {
		return nil, nil
	}
	result, err := p.ptgen.Query(ctx, title)
	if err != nil {
		p.logger.Debug("ptgen query skipped",
			zap.String("title", title),
			zap.Error(err),
		)
		return nil, err
	}
	return result, nil
}

func (p *Pipeline) CreateGroup(ctx context.Context, candidateID uint, sourceHash, sourceSite, sourceTorrentID string) (*model.PublishGroup, error) {
	group := &model.PublishGroup{
		CandidateID:     candidateID,
		SourceHash:      sourceHash,
		SourceSite:      sourceSite,
		SourceTorrentID: sourceTorrentID,
		Status:          model.GroupActive,
	}
	if err := p.db.WithContext(ctx).Create(group).Error; err != nil {
		return nil, &model.AppError{Code: 50001, Message: "创建发布组失败", Cause: err}
	}
	p.addStatusHistory(ctx, group.ID, "", model.MemberStatusNew, "创建发布组")
	return group, nil
}

func (p *Pipeline) GetGroup(ctx context.Context, id uint) (*model.PublishGroup, error) {
	var group model.PublishGroup
	if err := p.db.WithContext(ctx).First(&group, id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func (p *Pipeline) ListGroups(ctx context.Context, offset, limit int) ([]model.PublishGroup, int64, error) {
	var groups []model.PublishGroup
	var total int64
	p.db.Model(&model.PublishGroup{}).Count(&total)
	err := p.db.WithContext(ctx).Order("created_at DESC").Offset(offset).Limit(limit).Find(&groups).Error
	return groups, total, err
}

func (p *Pipeline) AddGroupMember(ctx context.Context, groupID uint, member *model.PublishGroupMember) error {
	member.PublishGroupID = groupID
	if err := p.db.WithContext(ctx).Create(member).Error; err != nil {
		return &model.AppError{Code: 50001, Message: "添加组成员失败", Cause: err}
	}
	return nil
}

func (p *Pipeline) ListGroupMembers(ctx context.Context, groupID uint) ([]model.PublishGroupMember, error) {
	var members []model.PublishGroupMember
	err := p.db.WithContext(ctx).Where("publish_group_id = ?", groupID).Find(&members).Error
	return members, err
}

func (p *Pipeline) UpdateGroupStatus(ctx context.Context, groupID uint, status model.PublishGroupStatus, reason string) error {
	var group model.PublishGroup
	if err := p.db.WithContext(ctx).First(&group, groupID).Error; err != nil {
		return err
	}

	oldStatus := group.Status
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	if reason != "" {
		updates["last_error"] = reason
	}

	if err := p.db.Model(&group).Updates(updates).Error; err != nil {
		return err
	}

	p.addStatusHistory(ctx, groupID, "", model.MemberStatus(oldStatus), fmt.Sprintf("%s → %s: %s", oldStatus, status, reason))
	return nil
}

func (p *Pipeline) UpdateMemberStatus(ctx context.Context, memberID uint, status model.MemberStatus, reason string) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":     status,
		"status_at":  &now,
		"updated_at": now,
	}
	if reason != "" {
		updates["last_error"] = reason
	}
	return p.db.WithContext(ctx).Model(&model.PublishGroupMember{}).
		Where("id = ?", memberID).
		Updates(updates).Error
}

func (p *Pipeline) TransitionGroupLifecycle(ctx context.Context, groupID uint) error {
	var group model.PublishGroup
	if err := p.db.WithContext(ctx).First(&group, groupID).Error; err != nil {
		return err
	}

	var members []model.PublishGroupMember
	if err := p.db.WithContext(ctx).Where("publish_group_id = ?", groupID).Find(&members).Error; err != nil {
		return err
	}

	if len(members) == 0 {
		return nil
	}

	allDone := true
	anyFailed := false
	allPaused := true
	anyPublishing := false

	for _, m := range members {
		switch m.Status {
		case model.MemberStatusUploaded, model.MemberStatusSeedingConfirmed:
		case model.MemberStatusError, model.MemberStatusBanned, model.MemberStatusDeleted:
			anyFailed = true
			allDone = false
		case model.MemberStatusUploading, model.MemberStatusInjected, model.MemberStatusDownloading:
			allDone = false
			anyPublishing = true
			allPaused = false
		case model.MemberStatusPaused:
			allDone = false
			allPaused = allPaused && m.Paused
		default:
			allDone = false
			allPaused = false
		}
	}

	newStatus := group.Status
	switch {
	case allDone:
		newStatus = model.GroupMonitoring
	case anyFailed && !anyPublishing:
		newStatus = model.GroupPublishFailed
	case allPaused:
		newStatus = model.GroupAllPaused
	case anyPublishing:
		newStatus = model.GroupPublishing
	}

	if newStatus != group.Status {
		return p.UpdateGroupStatus(ctx, groupID, newStatus, "")
	}

	return nil
}

func (p *Pipeline) addStatusHistory(ctx context.Context, groupID uint, memberHash string, from model.MemberStatus, reason string) {
	history := &model.PublishGroupStatusHistory{
		PublishGroupID: groupID,
		MemberHash:     memberHash,
		OldStatus:      from,
		NewStatus:      from,
		Reason:         reason,
	}
	if err := p.db.WithContext(ctx).Create(history).Error; err != nil {
		p.logger.Warn("记录状态历史失败", zap.Uint("groupID", groupID), zap.Error(err))
	}
}

const (
	StepDownload    = 1
	StepDetail      = 2
	StepDedup       = 3
	StepRender      = 4
	StepUpload      = 5
	StepConfirmSeed = 6
)

func (p *Pipeline) ProcessMemberWithResume(ctx context.Context, member *model.PublishGroupMember) error {
	if member.PublishGroupID == 0 {
		return fmt.Errorf("member has no group")
	}

	var group model.PublishGroup
	if err := p.db.WithContext(ctx).First(&group, member.PublishGroupID).Error; err != nil {
		return fmt.Errorf("load group: %w", err)
	}

	if p.siteProvider == nil {
		return fmt.Errorf("site provider not configured")
	}

	sourceConfig, err := p.siteProvider.GetSiteConfig(ctx, group.SourceSite)
	if err != nil {
		return fmt.Errorf("get source config: %w", err)
	}

	sourceAdapter, err := p.siteProvider.GetAdapter(ctx, group.SourceSite)
	if err != nil {
		return fmt.Errorf("get source adapter: %w", err)
	}

	var torrentData []byte
	if member.LastCompletedStep < StepDownload {
		td, dlErr := sourceAdapter.DownloadTorrent(ctx, sourceConfig, group.SourceTorrentID)
		if dlErr != nil {
			return p.failMember(ctx, member, StepDownload, fmt.Sprintf("下载源种子失败: %v", dlErr))
		}
		torrentData = td
		p.advanceStep(ctx, member, StepDownload)
	}

	var sourceDetail *model.TorrentDetail
	if member.LastCompletedStep < StepDetail {
		detail, detErr := sourceAdapter.GetTorrentDetail(ctx, sourceConfig, group.SourceTorrentID)
		if detErr != nil {
			p.logger.Warn("获取源种子详情失败", zap.Error(detErr))
		}
		sourceDetail = detail
		p.advanceStep(ctx, member, StepDetail)
	}

	if member.LastCompletedStep < StepUpload {
		targetConfig, cfgErr := p.siteProvider.GetSiteConfig(ctx, member.SiteName)
		if cfgErr != nil {
			return p.failMember(ctx, member, StepUpload, fmt.Sprintf("获取目标站配置失败: %v", cfgErr))
		}

		targetAdapter, adpErr := p.siteProvider.GetAdapter(ctx, member.SiteName)
		if adpErr != nil {
			return p.failMember(ctx, member, StepUpload, fmt.Sprintf("获取目标站适配器失败: %v", adpErr))
		}

		pubReq := &model.PublishRequest{
			TorrentData:     torrentData,
			Title:           group.SourceTorrentID,
			SourceSite:      group.SourceSite,
			SourceInfoHash:  group.SourceHash,
			SourceTorrentID: group.SourceTorrentID,
			TargetSite:      member.SiteName,
			FormFields:      make(map[string]string),
		}

		if sourceDetail != nil {
			pubReq.Title = sourceDetail.Title
			pubReq.Description = sourceDetail.Description
			pubReq.MediaInfo = sourceDetail.MediaInfo
			pubReq.Screenshots = sourceDetail.Screenshots
			if sourceDetail.Category != "" {
				pubReq.FormFields["category"] = sourceDetail.Category
			}
			if sourceDetail.IMDbID != "" {
				pubReq.FormFields["imdb"] = sourceDetail.IMDbID
			}
		}

		resp, uploadErr := targetAdapter.UploadTorrent(ctx, targetConfig, pubReq)
		if uploadErr != nil {
			return p.failMember(ctx, member, StepUpload, fmt.Sprintf("上传失败: %v", uploadErr))
		}

		now := time.Now()
		p.db.WithContext(ctx).Model(member).Updates(map[string]interface{}{
			"torrent_id": resp.TorrentID,
			"status":     model.MemberStatusUploaded,
			"status_at":  &now,
		})
		p.advanceStep(ctx, member, StepUpload)
	}

	return nil
}

func (p *Pipeline) advanceStep(ctx context.Context, member *model.PublishGroupMember, step int) {
	p.db.WithContext(ctx).Model(member).Updates(map[string]interface{}{
		"last_completed_step": step,
		"updated_at":          time.Now(),
	})
	member.LastCompletedStep = step
}

func (p *Pipeline) failMember(ctx context.Context, member *model.PublishGroupMember, step int, reason string) error {
	now := time.Now()
	p.db.WithContext(ctx).Model(member).Updates(map[string]interface{}{
		"status":     model.MemberStatusError,
		"last_error": reason,
		"error_at":   &now,
		"status_at":  &now,
	})
	return fmt.Errorf("step %d: %s", step, reason)
}
