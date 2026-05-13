package publish

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/description"
	"github.com/ranfish/pt-forward/internal/metrics"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/notification"
	"github.com/ranfish/pt-forward/internal/ptgen"
	"github.com/ranfish/pt-forward/internal/screenshot"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Pipeline struct {
	db                *gorm.DB
	logger            *zap.Logger
	siteProvider      model.SiteInfoProvider
	clientProvider    model.DownloaderProvider
	ptgen             *ptgen.Provider
	completionWatcher model.CompletionWatcher
	notifyService     *notification.Service
	screenshotConfig  *screenshot.Config
	artifactCache     *ArtifactCache
	torrentCache      *TorrentCache
}

func NewPipeline(db *gorm.DB, logger *zap.Logger) *Pipeline {
	return &Pipeline{db: db, logger: logger, ptgen: ptgen.NewProvider(db, logger)}
}

func (p *Pipeline) SetSiteProvider(sp model.SiteInfoProvider) {
	p.siteProvider = sp
}

func (p *Pipeline) SetClientProvider(cp model.DownloaderProvider) {
	p.clientProvider = cp
}

func (p *Pipeline) SetCompletionWatcher(w model.CompletionWatcher) {
	p.completionWatcher = w
}

func (p *Pipeline) SetNotifyService(ns *notification.Service) {
	p.notifyService = ns
}

func (p *Pipeline) SetScreenshotConfig(cfg screenshot.Config) {
	p.screenshotConfig = &cfg
}

func (p *Pipeline) SetArtifactCache(ac *ArtifactCache) {
	p.artifactCache = ac
}

func (p *Pipeline) SetTorrentCache(tc *TorrentCache) {
	p.torrentCache = tc
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

	if err := p.db.WithContext(ctx).Model(&model.PublishTask{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := p.db.WithContext(ctx).Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&tasks).Error
	return tasks, total, err
}

func (p *Pipeline) Update(ctx context.Context, task *model.PublishTask) error {
	return p.db.WithContext(ctx).Save(task).Error
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
	candidate, err := p.validateAndLoadCandidate(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.siteProvider == nil {
		candidate.PublishStatus = model.CandidatePublishing
		return candidate, nil
	}

	sourceDetail, sourceConfig, sourceAdapter, err := p.fetchSourceInfo(ctx, candidate)
	if err != nil {
		return nil, err
	}

	torrentData, err := sourceAdapter.DownloadTorrent(ctx, sourceConfig, candidate.SourceTorrentID)
	if err != nil {
		if err := p.UpdateCandidateStatus(ctx, id, model.CandidateFailed, fmt.Sprintf("下载源种子失败: %v", err)); err != nil {
			p.logger.Warn("更新候选状态失败", zap.Uint("id", id), zap.Error(err))
		}
		return nil, &model.AppError{Code: 50001, Message: "下载源种子失败", Cause: err}
	}

	targetSites := parseTargetSites(candidate.TargetSites)
	if len(targetSites) == 0 {
		candidate.PublishStatus = model.CandidatePublishing
		return candidate, nil
	}

	var lastErr error
	publishedCount := 0
	for _, target := range targetSites {
		if ctx.Err() != nil {
			break
		}
		published, err := p.publishToTarget(ctx, candidate, target, sourceDetail, torrentData)
		if err != nil {
			lastErr = err
		}
		if published {
			publishedCount++
		}
	}

	candidate.PublishStatus = p.finalizePublishStatus(ctx, id, publishedCount, lastErr)
	return candidate, nil
}

func (p *Pipeline) validateAndLoadCandidate(ctx context.Context, id uint) (*model.PublishCandidate, error) {
	var candidate model.PublishCandidate
	if err := p.db.WithContext(ctx).First(&candidate, id).Error; err != nil {
		return nil, err
	}

	eligible, reason := p.CheckPublishEligibility(ctx, &candidate, "")
	if !eligible {
		if err := p.UpdateCandidateStatus(ctx, id, model.CandidateSkipped, reason); err != nil {
			p.logger.Warn("更新候选状态失败", zap.Uint("id", id), zap.Error(err))
		}
		return nil, &model.AppError{Code: 40001, Message: fmt.Sprintf("发布合规检查未通过: %s", reason)}
	}

	if err := p.UpdateCandidateStatus(ctx, id, model.CandidatePublishing, ""); err != nil {
		return nil, &model.AppError{Code: 40001, Message: "更新候选状态失败", Cause: err}
	}

	return &candidate, nil
}

func (p *Pipeline) fetchSourceInfo(ctx context.Context, candidate *model.PublishCandidate) (*model.TorrentDetail, *model.SiteConfig, model.SiteAdapter, error) {
	sourceConfig, err := p.siteProvider.GetSiteConfig(ctx, candidate.SourceSite)
	if err != nil {
		_ = p.UpdateCandidateStatus(ctx, candidate.ID, model.CandidateFailed, fmt.Sprintf("获取源站配置失败: %v", err))
		return nil, nil, nil, &model.AppError{Code: 50001, Message: "获取源站配置失败", Cause: err}
	}

	sourceAdapter, err := p.siteProvider.GetAdapter(ctx, candidate.SourceSite)
	if err != nil {
		_ = p.UpdateCandidateStatus(ctx, candidate.ID, model.CandidateFailed, fmt.Sprintf("获取源站适配器失败: %v", err))
		return nil, nil, nil, &model.AppError{Code: 50001, Message: "获取源站适配器失败", Cause: err}
	}

	sourceDetail, err := sourceAdapter.GetTorrentDetail(ctx, sourceConfig, candidate.SourceTorrentID)
	if err != nil {
		p.logger.Warn("获取源种子详情失败", zap.Error(err))
	}

	if sourceDetail != nil {
		extraTexts := []string{}
		if sourceDetail.Subtitle != "" {
			extraTexts = append(extraTexts, sourceDetail.Subtitle)
		}
		if sourceDetail.Description != "" {
			extraTexts = append(extraTexts, sourceDetail.Description)
		}
		if len(extraTexts) > 0 {
			if eligible, reason := p.checkForbiddenContent(extraTexts); !eligible {
				_ = p.UpdateCandidateStatus(ctx, candidate.ID, model.CandidateSkipped, reason)
				return nil, nil, nil, &model.AppError{Code: 40001, Message: fmt.Sprintf("发布合规检查未通过: %s", reason)}
			}
		}
	}

	return sourceDetail, sourceConfig, sourceAdapter, nil
}

func (p *Pipeline) publishToTarget(ctx context.Context, candidate *model.PublishCandidate, targetSite string, sourceDetail *model.TorrentDetail, torrentData []byte) (bool, error) {
	eligible, reason := p.CheckPublishEligibility(ctx, candidate, targetSite)
	if !eligible {
		p.logger.Info("目标站发布排除", zap.String("target", targetSite), zap.String("reason", reason))
		return false, nil
	}

	targetConfig, err := p.siteProvider.GetSiteConfig(ctx, targetSite)
	if err != nil {
		p.logger.Warn("获取目标站配置失败", zap.String("site", targetSite), zap.Error(err))
		return false, nil
	}

	targetAdapter, err := p.siteProvider.GetAdapter(ctx, targetSite)
	if err != nil {
		p.logger.Warn("获取目标站适配器失败", zap.String("site", targetSite), zap.Error(err))
		return false, nil
	}

	title := candidate.TorrentName
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
					if err := p.CreateResult(ctx, &model.PublishResultRecord{
						CandidateID:  candidate.ID,
						SourceSite:   candidate.SourceSite,
						TargetSite:   targetSite,
						TorrentID:    dr.TorrentID,
						Status:       model.PublishResultSkipped,
						ErrorMessage: fmt.Sprintf("去重匹配: %s (size=%d)", dr.Title, dr.Size),
					}); err != nil {
						p.logger.Warn("记录发布结果失败", zap.Error(err))
					}
					return false, nil
				}
			}
		}
	}

	pubReq, err := p.buildPublishRequest(ctx, candidate, targetSite, sourceDetail, torrentData)
	if err != nil {
		return false, err
	}

	start := time.Now()
	resp, err := targetAdapter.UploadTorrent(ctx, targetConfig, pubReq)
	metrics.PublishDuration.WithLabelValues(targetSite).Observe(time.Since(start).Seconds())
	if err != nil {
		p.logger.Warn("上传到目标站失败",
			zap.String("target", targetSite),
			zap.Error(err),
		)
		metrics.PublishTasksTotal.WithLabelValues(targetSite, "failed").Inc()
		if err := p.CreateResult(ctx, &model.PublishResultRecord{
			CandidateID:  candidate.ID,
			SourceSite:   candidate.SourceSite,
			TargetSite:   targetSite,
			TorrentID:    candidate.SourceTorrentID,
			Status:       model.PublishResultFailed,
			ErrorMessage: err.Error(),
		}); err != nil {
			p.logger.Warn("记录发布结果失败", zap.Error(err))
		}
		return false, err
	}

	if err := p.CreateResult(ctx, &model.PublishResultRecord{
		CandidateID: candidate.ID,
		SourceSite:  candidate.SourceSite,
		TargetSite:  targetSite,
		TorrentID:   resp.TorrentID,
		Status:      model.PublishResultCompleted,
		PublishURL:  resp.DetailURL,
	}); err != nil {
		p.logger.Warn("记录发布结果失败", zap.Error(err))
	}

	metrics.PublishTasksTotal.WithLabelValues(targetSite, "completed").Inc()

	return true, nil
}

func (p *Pipeline) buildPublishRequest(ctx context.Context, candidate *model.PublishCandidate, targetSite string, sourceDetail *model.TorrentDetail, torrentData []byte) (*model.PublishRequest, error) {
	title := candidate.TorrentName
	descriptionText := ""
	if sourceDetail != nil {
		descriptionText = sourceDetail.Description
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
		if ptgenResult.TMDbURL != "" {
			if tmdbID := extractTMDBID(ptgenResult.TMDbURL); tmdbID != "" {
				if pubReq.ExtraFields == nil {
					pubReq.ExtraFields = make(map[string]string)
				}
				pubReq.ExtraFields["tmdb_id"] = tmdbID
			}
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
		if sourceDetail.AudioCodec != "" {
			pubReq.FormFields["audioCodec"] = sourceDetail.AudioCodec
		}
		if sourceDetail.Processing != "" {
			pubReq.FormFields["processing"] = sourceDetail.Processing
		}
		if sourceDetail.ReleaseGroup != "" {
			pubReq.FormFields["team"] = sourceDetail.ReleaseGroup
		}
		if sourceDetail.Region != "" {
			pubReq.FormFields["region"] = sourceDetail.Region
		}
		if sourceDetail.IMDbID != "" {
			pubReq.FormFields["imdb"] = sourceDetail.IMDbID
		}
		pubReq.MediaInfo = sourceDetail.MediaInfo
		pubReq.Screenshots = sourceDetail.Screenshots
	}

	if pubReq.DoubanLink != "" && pubReq.FormFields["douban"] == "" {
		pubReq.FormFields["douban"] = pubReq.DoubanLink
	}

	p.mapFieldValues(ctx, targetSite, pubReq.FormFields)

	return pubReq, nil
}

func (p *Pipeline) finalizePublishStatus(ctx context.Context, id uint, publishedCount int, lastErr error) model.PublishCandidateStatus {
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
		return model.CandidateDone
	}
	if lastErr != nil {
		if err := p.UpdateCandidateStatus(ctx, id, model.CandidateFailed, lastErr.Error()); err != nil {
			p.logger.Error("更新发布失败状态失败", zap.Uint("id", id), zap.Error(err))
		}
		return model.CandidateFailed
	}
	return model.CandidatePublishing
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
	now := time.Now()
	r := p.db.WithContext(ctx).Model(&model.PublishCandidate{}).
		Where("id = ? AND publish_status != ?", id, status).
		Updates(map[string]interface{}{
			"publish_status": status,
			"publish_result": result,
			"updated_at":     now,
		})
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected == 0 {
		p.logger.Debug("candidate status already updated, CAS skip",
			zap.Uint("id", id),
			zap.String("status", string(status)))
	}
	return nil
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

var (
	forbiddenTransferKeywords = []string{"禁转", "独占", "谢绝转载", "限时禁转", "严禁转载", "禁止转载", "谢绝搬运"}
	forbiddenTransferGroups   = []string{"CatEDU"}
	adultContentKeywords      = []string{"9KG", "9kg", "色情", "成人内容", "成人影片", "AV", "18+", "NSFW", "Adult", "XXX", "Porn", "Erotic", "Hentai"}
)

func containsAnyKeyword(text string, keywords []string) (string, bool) {
	lower := strings.ToLower(text)
	for _, kw := range keywords {
		if strings.Contains(text, kw) || strings.Contains(lower, strings.ToLower(kw)) {
			return kw, true
		}
	}
	return "", false
}

func (p *Pipeline) checkForbiddenContent(texts []string) (bool, string) {
	for _, text := range texts {
		if text == "" {
			continue
		}
		if kw, found := containsAnyKeyword(text, adultContentKeywords); found {
			return false, fmt.Sprintf("内容包含成人/色情关键词: %s (§30.5 规则 1)", kw)
		}
		if kw, found := containsAnyKeyword(text, forbiddenTransferKeywords); found {
			return false, fmt.Sprintf("标题/副标题包含禁止转载关键词: %s (§30.5 规则 2)", kw)
		}
		for _, grp := range forbiddenTransferGroups {
			if strings.Contains(text, grp) {
				return false, fmt.Sprintf("禁止转载小组资源: %s (§30.5 规则 3)", grp)
			}
		}
	}
	return true, ""
}

func (p *Pipeline) CheckPublishEligibility(ctx context.Context, candidate *model.PublishCandidate, targetSite string) (bool, string) {
	if eligible, reason := p.checkForbiddenContent([]string{candidate.TorrentName}); !eligible {
		return false, reason
	}

	if candidate.HasHR {
		return false, "源站种子存在 H&R (Hit and Run) 标记，跳过发布"
	}

	if targetSite != "" && candidate.SourceSite != "" {
		var exclusion model.PublishExclusion
		err := p.db.WithContext(ctx).
			Where("target_site = ? AND source_site = ?", targetSite, candidate.SourceSite).
			First(&exclusion).Error
		if err == nil {
			return false, fmt.Sprintf("源站 %s → 目标站 %s 存在发布排除规则", candidate.SourceSite, targetSite)
		}
	}

	return true, ""
}

func (p *Pipeline) ProcessPending(ctx context.Context) error {
	candidates, err := p.ListPendingCandidates(ctx, 100)
	if err != nil {
		return publishError(ErrPublishDB, "list pending candidates", err)
	}

	if len(candidates) == 0 {
		return nil
	}

	p.logger.Info("processing pending candidates", zap.Int("count", len(candidates)))

	for i := range candidates {
		c := &candidates[i]

		eligible, reason := p.CheckPublishEligibility(ctx, c, "")
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

		if c.Role == model.RoleDownload && !c.DownloadCompleted {
			if err := p.processDownloadCandidate(ctx, c); err != nil {
				p.logger.Warn("download candidate failed",
					zap.Uint("id", c.ID),
					zap.String("source_site", c.SourceSite),
					zap.String("torrent_id", c.SourceTorrentID),
					zap.Error(err),
				)
				if statusErr := p.UpdateCandidateStatus(ctx, c.ID, model.CandidateFailed, err.Error()); statusErr != nil {
					p.logger.Warn("update candidate status failed after download error",
						zap.Uint("id", c.ID),
						zap.Error(statusErr),
					)
				}
			}
		}
	}

	return nil
}

func (p *Pipeline) ProcessPendingGroups(ctx context.Context) error {
	var groups []model.PublishGroup
	err := p.db.WithContext(ctx).
		Where("status IN ?", []model.PublishGroupStatus{
			model.GroupActive,
			model.GroupPublishing,
		}).Find(&groups).Error
	if err != nil {
		return publishError(ErrPublishDB, "list active groups", err)
	}

	if len(groups) == 0 {
		return nil
	}

	p.logger.Debug("processing pending groups", zap.Int("count", len(groups)))

	for i := range groups {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		group := &groups[i]

		var members []model.PublishGroupMember
		if err := p.db.WithContext(ctx).
			Where("publish_group_id = ? AND status IN ?",
				group.ID,
				[]model.MemberStatus{
					model.MemberStatusNew,
					model.MemberStatusUploading,
					model.MemberStatusInjected,
				},
			).Find(&members).Error; err != nil {
			p.logger.Warn("query pending members failed",
				zap.Uint("groupID", group.ID),
				zap.Error(err),
			)
			continue
		}

		for j := range members {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err := p.ProcessMemberWithResume(ctx, &members[j]); err != nil {
				p.logger.Warn("process member failed",
					zap.Uint("groupID", group.ID),
					zap.Uint("memberID", members[j].ID),
					zap.String("site", members[j].SiteName),
					zap.Error(err),
				)
			}
		}

		if err := p.TransitionGroupLifecycle(ctx, group.ID); err != nil {
			p.logger.Warn("transition group lifecycle failed",
				zap.Uint("groupID", group.ID),
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

		var clientID string
		if ev.SourceID != "" {
			var sub struct {
				ClientID string `gorm:"column:client_id"`
			}
			if err := p.db.WithContext(ctx).Table("rss_subscriptions").Select("client_id").
				Where("id = ?", ev.SourceID).First(&sub).Error; err == nil {
				clientID = sub.ClientID
			}
		}

		candidate := &model.PublishCandidate{
			SubscriptionID:  ev.SourceID,
			SourceSite:      ev.SiteName,
			SourceTorrentID: ev.TorrentID,
			InfoHash:        ev.InfoHash,
			TorrentName:     ev.Title,
			Size:            ev.Size,
			Discount:        ev.Discount,
			HasHR:           ev.HasHR,
			PublishStatus:   model.CandidatePending,
			Role:            model.RoleDownload,
			ClientID:        clientID,
		}

		if err := p.CreateCandidate(ctx, candidate); err != nil {
			p.logger.Warn("create publish candidate failed",
				zap.String("torrent", ev.TorrentID),
				zap.Error(err),
			)
		} else if p.completionWatcher != nil && candidate.ClientID != "" && candidate.InfoHash != "" {
			if err := p.completionWatcher.SubmitCandidate(ctx, *candidate); err != nil {
				p.logger.Warn("submit candidate to watcher failed",
					zap.Uint("id", candidate.ID),
					zap.Error(err),
				)
			}
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

func extractTMDBID(tmdbURL string) string {
	re := regexp.MustCompile(`(?:themoviedb\.org|tmdb\.org)/(?:movie|tv)/(\d+)`)
	m := re.FindStringSubmatch(tmdbURL)
	if len(m) > 1 {
		return m[1]
	}
	return ""
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
	p.addStatusHistory(ctx, group.ID, "", model.MemberStatusNew, model.MemberStatusNew, "创建发布组")
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
	p.db.WithContext(ctx).Model(&model.PublishGroup{}).Count(&total)
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

	if err := p.db.WithContext(ctx).Model(&group).Updates(updates).Error; err != nil {
		return err
	}

	p.addStatusHistory(ctx, groupID, "", model.MemberStatus(oldStatus), model.MemberStatus(status), reason)
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

func (p *Pipeline) addStatusHistory(ctx context.Context, groupID uint, memberHash string, from model.MemberStatus, to model.MemberStatus, reason string) {
	history := &model.PublishGroupStatusHistory{
		PublishGroupID: groupID,
		MemberHash:     memberHash,
		OldStatus:      from,
		NewStatus:      to,
		Reason:         reason,
	}
	if err := p.db.WithContext(ctx).Create(history).Error; err != nil {
		p.logger.Warn("记录状态历史失败", zap.Uint("groupID", groupID), zap.Error(err))
	}
}

const (
	StepEligibility = 0
	StepDownload    = 1
	StepDetail      = 2
	StepDedup       = 3
	StepRender      = 4
	StepUpload      = 5
	_               = 6
	StepHRDetect = 7
)

func (p *Pipeline) ProcessMemberWithResume(ctx context.Context, member *model.PublishGroupMember) error {
	if member.PublishGroupID == 0 {
		return publishError(ErrPublishConfig, "member has no group", nil)
	}

	var group model.PublishGroup
	if err := p.db.WithContext(ctx).First(&group, member.PublishGroupID).Error; err != nil {
		return publishError(ErrPublishDB, "load group", err)
	}

	if p.siteProvider == nil {
		return publishError(ErrPublishConfig, "site provider not configured", nil)
	}

	sourceConfig, err := p.siteProvider.GetSiteConfig(ctx, group.SourceSite)
	if err != nil {
		return publishError(ErrPublishConfig, "get source config", err)
	}

	sourceAdapter, err := p.siteProvider.GetAdapter(ctx, group.SourceSite)
	if err != nil {
		return publishError(ErrPublishConfig, "get source adapter", err)
	}

	if member.LastCompletedStep < StepEligibility {
		title := group.SourceTorrentID
		var candidate model.PublishCandidate
		if err := p.db.WithContext(ctx).Where("id = ?", group.CandidateID).First(&candidate).Error; err == nil {
			if candidate.TorrentName != "" {
				title = candidate.TorrentName
			}
		}

		texts := []string{title}
		detail, detErr := sourceAdapter.GetTorrentDetail(ctx, sourceConfig, group.SourceTorrentID)
		if detErr == nil && detail != nil {
			if detail.Title != "" {
				texts[0] = detail.Title
			}
			if detail.Subtitle != "" {
				texts = append(texts, detail.Subtitle)
			}
			if detail.Description != "" {
				texts = append(texts, detail.Description)
			}
		}

		if eligible, reason := p.checkForbiddenContent(texts); !eligible {
			return p.failMember(ctx, member, StepEligibility, fmt.Sprintf("发布合规检查未通过: %s", reason))
		}

		tempCandidate := model.PublishCandidate{
			TorrentName: texts[0],
			SourceSite:  group.SourceSite,
			HasHR:       member.HRProtected,
		}
		if eligible, reason := p.CheckPublishEligibility(ctx, &tempCandidate, member.SiteName); !eligible {
			return p.failMember(ctx, member, StepEligibility, fmt.Sprintf("发布资格检查未通过: %s", reason))
		}

		if err := p.advanceStep(ctx, member, StepEligibility); err != nil {
			return p.failMember(ctx, member, StepEligibility, fmt.Sprintf("advanceStep failed: %v", err))
		}
	}

	var torrentData []byte
	if member.LastCompletedStep < StepDownload {
		td, dlErr := sourceAdapter.DownloadTorrent(ctx, sourceConfig, group.SourceTorrentID)
		if dlErr != nil {
			return p.failMember(ctx, member, StepDownload, fmt.Sprintf("下载源种子失败: %v", dlErr))
		}
		torrentData = td
		if err := p.advanceStep(ctx, member, StepDownload); err != nil {
			return p.failMember(ctx, member, StepDownload, fmt.Sprintf("advanceStep failed: %v", err))
		}
	}

	var sourceDetail *model.TorrentDetail
	if member.LastCompletedStep < StepDetail {
		detail, detErr := sourceAdapter.GetTorrentDetail(ctx, sourceConfig, group.SourceTorrentID)
		if detErr != nil {
			p.logger.Warn("获取源种子详情失败", zap.Error(detErr))
		}
		sourceDetail = detail
		if err := p.advanceStep(ctx, member, StepDetail); err != nil {
			p.logger.Warn("advanceStep detail failed", zap.Error(err))
		}
	}

	if member.LastCompletedStep < StepUpload {
		if torrentData == nil {
			td, dlErr := sourceAdapter.DownloadTorrent(ctx, sourceConfig, group.SourceTorrentID)
			if dlErr != nil {
				return p.failMember(ctx, member, StepUpload, fmt.Sprintf("resume 重新下载源种子失败: %v", dlErr))
			}
			torrentData = td
		}

		targetConfig, cfgErr := p.siteProvider.GetSiteConfig(ctx, member.SiteName)
		if cfgErr != nil {
			return p.failMember(ctx, member, StepUpload, fmt.Sprintf("获取目标站配置失败: %v", cfgErr))
		}

		targetAdapter, adpErr := p.siteProvider.GetAdapter(ctx, member.SiteName)
		if adpErr != nil {
			return p.failMember(ctx, member, StepUpload, fmt.Sprintf("获取目标站适配器失败: %v", adpErr))
		}

		title := group.SourceTorrentID
		if sourceDetail != nil && sourceDetail.Title != "" {
			title = sourceDetail.Title
		}

		if member.LastCompletedStep < StepDedup {
			if title != "" && title != group.SourceTorrentID {
				dedupResults, dedupErr := targetAdapter.SearchTorrents(ctx, targetConfig, title, nil)
				if dedupErr == nil {
					for _, dr := range dedupResults {
						if dr.Size > 0 && sourceDetail != nil && dr.Size == sourceDetail.Size {
							p.logger.Info("目标站已存在相同资源，跳过发布",
								zap.String("target", member.SiteName),
								zap.String("title", dr.Title),
								zap.Int64("size", dr.Size),
							)
							if err := p.advanceStep(ctx, member, StepDedup); err != nil {
								p.logger.Warn("advanceStep dedup failed", zap.Error(err))
							}
							return nil
						}
					}
				}
			}
			if err := p.advanceStep(ctx, member, StepDedup); err != nil {
				p.logger.Warn("advanceStep dedup failed", zap.Error(err))
			}
		}

		descriptionText := ""
		if sourceDetail != nil {
			descriptionText = sourceDetail.Description
		}

		var imdbLink, doubanLink string

		if member.LastCompletedStep < StepRender {
			descData := &model.DescriptionData{
				SourceSite: group.SourceSite,
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
				imdbLink = ptgenResult.IMDBURL
				doubanLink = ptgenResult.DoubanURL
			}

			if descriptionText == "" && descData.PTGenBody != "" {
				descriptionText = descData.PTGenBody
			}

			var descConfig model.SiteDescConfig
			siteInfo, siteInfoErr := p.siteProvider.GetSiteInfo(ctx, member.SiteName)
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

			if err := p.advanceStep(ctx, member, StepRender); err != nil {
				p.logger.Warn("advanceStep render failed", zap.Error(err))
			}
		}

		pubReq := &model.PublishRequest{
			TorrentData:     torrentData,
			Title:           title,
			Description:     descriptionText,
			SourceSite:      group.SourceSite,
			SourceInfoHash:  group.SourceHash,
			SourceTorrentID: group.SourceTorrentID,
			TargetSite:      member.SiteName,
			FormFields:      make(map[string]string),
			IMDbLink:        imdbLink,
			DoubanLink:      doubanLink,
		}

		if sourceDetail != nil {
			pubReq.MediaInfo = sourceDetail.MediaInfo
			pubReq.Screenshots = sourceDetail.Screenshots
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
			if sourceDetail.AudioCodec != "" {
				pubReq.FormFields["audioCodec"] = sourceDetail.AudioCodec
			}
			if sourceDetail.Processing != "" {
				pubReq.FormFields["processing"] = sourceDetail.Processing
			}
			if sourceDetail.ReleaseGroup != "" {
				pubReq.FormFields["team"] = sourceDetail.ReleaseGroup
			}
			if sourceDetail.Region != "" {
				pubReq.FormFields["region"] = sourceDetail.Region
			}
			if sourceDetail.IMDbID != "" {
				pubReq.FormFields["imdb"] = sourceDetail.IMDbID
			}
		}

		if pubReq.DoubanLink != "" && pubReq.FormFields["douban"] == "" {
			pubReq.FormFields["douban"] = pubReq.DoubanLink
		}

		p.mapFieldValues(ctx, member.SiteName, pubReq.FormFields)

		uploadStart := time.Now()
		resp, uploadErr := targetAdapter.UploadTorrent(ctx, targetConfig, pubReq)
		metrics.PublishDuration.WithLabelValues(member.SiteName).Observe(time.Since(uploadStart).Seconds())
		if uploadErr != nil {
			metrics.PublishTasksTotal.WithLabelValues(member.SiteName, "failed").Inc()
			return p.failMember(ctx, member, StepUpload, fmt.Sprintf("上传失败: %v", uploadErr))
		}

		metrics.PublishTasksTotal.WithLabelValues(member.SiteName, "completed").Inc()

		now := time.Now()
		p.db.WithContext(ctx).Model(member).Updates(map[string]interface{}{
			"torrent_id": resp.TorrentID,
			"status":     model.MemberStatusUploaded,
			"status_at":  &now,
		})
		if err := p.advanceStep(ctx, member, StepUpload); err != nil {
			p.logger.Warn("advanceStep upload failed", zap.Error(err))
		}
	}

	if member.LastCompletedStep < StepHRDetect {
		p.detectHR(ctx, member)
	}

	p.notifyPublishResult(ctx, member)

	return nil
}

func (p *Pipeline) detectHR(ctx context.Context, member *model.PublishGroupMember) {
	if member.TorrentID == "" {
		_ = p.advanceStep(ctx, member, StepHRDetect)
		return
	}

	targetConfig, cfgErr := p.siteProvider.GetSiteConfig(ctx, member.SiteName)
	if cfgErr != nil {
		p.logger.Warn("HR 检测: 获取目标站配置失败", zap.Error(cfgErr))
		_ = p.advanceStep(ctx, member, StepHRDetect)
		return
	}

	targetAdapter, adpErr := p.siteProvider.GetAdapter(ctx, member.SiteName)
	if adpErr != nil {
		p.logger.Warn("HR 检测: 获取目标站适配器失败", zap.Error(adpErr))
		_ = p.advanceStep(ctx, member, StepHRDetect)
		return
	}

	hrResult, hrErr := targetAdapter.DetectHR(ctx, targetConfig, member.TorrentID)
	if hrErr != nil {
		p.logger.Warn("HR 检测失败",
			zap.String("site", member.SiteName),
			zap.String("torrentID", member.TorrentID),
			zap.Error(hrErr),
		)
		_ = p.advanceStep(ctx, member, StepHRDetect)
		return
	}

	if hrResult != nil && hrResult.HasHR {
		hrSeedStart := time.Now()
		seedTimeH := hrResult.SeedTimeH
		if seedTimeH <= 0 {
			seedTimeH = 72
		}
		p.db.WithContext(ctx).Model(member).Updates(map[string]interface{}{
			"hr_protected":      true,
			"hr_min_seed_hours": seedTimeH,
			"hr_min_ratio":      hrResult.MinRatio,
			"hr_seed_start":     &hrSeedStart,
			"hr_site":           member.SiteName,
		})
		member.HRProtected = true
		member.HRSeedStart = &hrSeedStart
		member.HRMinSeedHours = seedTimeH

		hrTag := fmt.Sprintf("PROTECTED_HR_%s", member.SiteName)
		if p.clientProvider != nil && member.ClientID != "" && member.InfoHash != "" {
			if dl, dlErr := p.clientProvider.Get(member.ClientID); dlErr == nil {
				if tagErr := dl.SetTorrentTags(ctx, member.InfoHash, []string{hrTag}); tagErr != nil {
					p.logger.Warn("设置 HR 保护标签失败",
						zap.String("clientID", member.ClientID),
						zap.String("infoHash", member.InfoHash),
						zap.Error(tagErr),
					)
				}
			}
		}
		p.logger.Info("HR 检测: 种子已标记为 HR 保护",
			zap.Uint("memberID", member.ID),
			zap.String("site", member.SiteName),
			zap.Int("seedTimeH", seedTimeH),
		)
	}
	_ = p.advanceStep(ctx, member, StepHRDetect)
}

func (p *Pipeline) notifyPublishResult(ctx context.Context, member *model.PublishGroupMember) {
	if p.notifyService == nil {
		return
	}

	var group model.PublishGroup
	if err := p.db.WithContext(ctx).First(&group, member.PublishGroupID).Error; err != nil {
		return
	}

	var level, title, body string
	switch member.Status {
	case model.MemberStatusUploaded:
		level = "publish.success"
		title = fmt.Sprintf("发布成功 → %s", member.SiteName)
		body = fmt.Sprintf("种子 %s 已成功发布到 %s", group.SourceTorrentID, member.SiteName)
		if member.TorrentID != "" {
			body += fmt.Sprintf("（TorrentID: %s）", member.TorrentID)
		}
		if member.HRProtected {
			body += fmt.Sprintf("\n⚠️ HR 保护: 需保种 %d 小时", member.HRMinSeedHours)
		}
	case model.MemberStatusError:
		level = "publish.error"
		title = fmt.Sprintf("发布失败 → %s", member.SiteName)
		body = fmt.Sprintf("种子 %s 发布到 %s 失败", group.SourceTorrentID, member.SiteName)
		if member.LastError != "" {
			body += fmt.Sprintf(": %s", member.LastError)
		}
	default:
		return
	}

	msg := model.FormattedMessage{
		Title:   title,
		Message: body,
		Level:   level,
	}
	if err := p.notifyService.Send(ctx, msg); err != nil {
		p.logger.Warn("发布通知发送失败", zap.Error(err))
	}
}

func (p *Pipeline) advanceStep(ctx context.Context, member *model.PublishGroupMember, step int) error {
	if err := p.db.WithContext(ctx).Model(member).Updates(map[string]interface{}{
		"last_completed_step": step,
		"updated_at":          time.Now(),
	}).Error; err != nil {
		return err
	}
	member.LastCompletedStep = step
	return nil
}

func (p *Pipeline) failMember(ctx context.Context, member *model.PublishGroupMember, step int, reason string) error {
	now := time.Now()
	if err := p.db.WithContext(ctx).Model(member).Updates(map[string]interface{}{
		"status":     model.MemberStatusError,
		"last_error": reason,
		"error_at":   &now,
		"status_at":  &now,
	}).Error; err != nil {
		p.logger.Warn("failMember DB update failed", zap.Uint("memberID", member.ID), zap.Error(err))
	}
	return publishError(ErrPublishGeneric, fmt.Sprintf("step %d: %s", step, reason), nil)
}

func (p *Pipeline) mapFieldValues(ctx context.Context, targetSite string, fields map[string]string) {
	var mappings []model.SiteFieldMapping
	if err := p.db.WithContext(ctx).
		Where("site_name = ?", targetSite).
		Find(&mappings).Error; err != nil || len(mappings) == 0 {
		return
	}

	fieldMap := make(map[string]map[string]string)
	for _, m := range mappings {
		if _, ok := fieldMap[m.FieldType]; !ok {
			fieldMap[m.FieldType] = make(map[string]string)
		}
		fieldMap[m.FieldType][m.SourceValue] = m.TargetValue
	}

	mapKey := func(fieldType, value string) string {
		if m, ok := fieldMap[fieldType]; ok {
			if mapped, ok := m[value]; ok {
				return mapped
			}
		}
		return value
	}

	if v, ok := fields["category"]; ok {
		fields["category"] = mapKey("cat", v)
	}
	if v, ok := fields["resolution"]; ok {
		mapped := mapKey("standard_sel", v)
		if mapped == v {
			mapped = mapKey("resolution", v)
		}
		fields["resolution"] = mapped
	}
	if v, ok := fields["codec"]; ok {
		mapped := mapKey("codec_sel", v)
		if mapped == v {
			mapped = mapKey("videoCodec", v)
		}
		fields["codec"] = mapped
	}
	if v, ok := fields["source"]; ok {
		mapped := mapKey("source_sel", v)
		if mapped == v {
			mapped = mapKey("source", v)
		}
		fields["source"] = mapped
	}
}

func (p *Pipeline) processDownloadCandidate(ctx context.Context, c *model.PublishCandidate) error {
	if p.siteProvider == nil || p.clientProvider == nil {
		return fmt.Errorf("site provider or client provider not configured")
	}

	var sub struct {
		SavePath  string `gorm:"column:save_path"`
		Category  string `gorm:"column:category"`
		AddPaused bool   `gorm:"column:add_paused"`
		AutoTMM   bool   `gorm:"column:auto_tmm"`
	}
	subQuery := p.db.WithContext(ctx).Table("rss_subscriptions").
		Select("save_path, category, add_paused, auto_tmm").
		Where("id = ? AND deleted_at = ?", c.SubscriptionID, time.Time{})
	if err := subQuery.First(&sub).Error; err != nil {
		return fmt.Errorf("get subscription settings: %w", err)
	}

	var site model.Site
	if err := p.db.WithContext(ctx).Where("name = ?", c.SourceSite).First(&site).Error; err != nil {
		return fmt.Errorf("get site info: %w", err)
	}

	sourceConfig, err := p.siteProvider.GetSiteConfig(ctx, site.Domain)
	if err != nil {
		return fmt.Errorf("get source site config: %w", err)
	}
	sourceAdapter, err := p.siteProvider.GetAdapter(ctx, site.Domain)
	if err != nil {
		return fmt.Errorf("get source adapter: %w", err)
	}

	torrentData, err := sourceAdapter.DownloadTorrent(ctx, sourceConfig, c.SourceTorrentID)
	if err != nil {
		return fmt.Errorf("download torrent: %w", err)
	}

	dlClient, err := p.clientProvider.Get(c.ClientID)
	if err != nil {
		return fmt.Errorf("get downloader %s: %w", c.ClientID, err)
	}

	opts := model.AddTorrentOptions{
		SavePath: sub.SavePath,
		Category: sub.Category,
		Paused:   sub.AddPaused,
		AutoTMM:  sub.AutoTMM,
	}

	result, err := dlClient.AddFromFile(ctx, torrentData, opts)
	if err != nil {
		return fmt.Errorf("add torrent to downloader: %w", err)
	}

	_ = p.UpdateCandidateStatus(ctx, c.ID, model.CandidateDownloading, "")

	p.logger.Info("torrent added to downloader",
		zap.String("client", c.ClientID),
		zap.String("source_site", c.SourceSite),
		zap.String("torrent_id", c.SourceTorrentID),
		zap.String("name", c.TorrentName),
		zap.String("info_hash", func() string {
			if result != nil {
				return result.InfoHash
			}
			return ""
		}()),
	)

	if p.completionWatcher != nil && result != nil && result.InfoHash != "" {
		_ = p.completionWatcher.Watch(ctx, c.ClientID, result.InfoHash, c.ID)
	}

	return nil
}
