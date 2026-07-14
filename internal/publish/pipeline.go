package publish

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/audit"
	"github.com/ranfish/pt-forward/internal/description"
	"github.com/ranfish/pt-forward/internal/imagehost"
	"github.com/ranfish/pt-forward/internal/metadata"
	"github.com/ranfish/pt-forward/internal/metrics"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/notification"
	"github.com/ranfish/pt-forward/internal/ptgen"
	"github.com/ranfish/pt-forward/internal/screenshot"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var reTMDBID = regexp.MustCompile(`(?:themoviedb\.org|tmdb\.org)/(?:movie|tv)/(\d+)`)

type Pipeline struct {
	db                *gorm.DB
	logger            *zap.Logger
	siteProvider      model.SiteInfoProvider
	clientProvider    model.DownloaderProvider
	ptgen             *ptgen.Provider
	completionWatcher model.CompletionWatcher
	notifyService     *notification.Service
	screenshotConfig  *screenshot.Config
	backpressureCtrl  *BackpressureController
	artifactGenerator *PublishArtifactGenerator
	limitGuard        *PublishLimitGuard
	declarationFilter *DeclarationFilter
	metadataFetcher   *metadata.Fetcher
	imageHostStrategy string
	imageHostMgr      *imagehost.Manager
	memberMu          sync.Map
}

func NewPipeline(db *gorm.DB, logger *zap.Logger) *Pipeline {
	logger = logger.With(zap.String("component", "publish"))
	return &Pipeline{
		db:        db,
		logger:    logger,
		ptgen:     ptgen.NewProvider(db, logger),
		limitGuard: NewPublishLimitGuard(db, logger),
	}
}

func (p *Pipeline) SetSiteProvider(sp model.SiteInfoProvider) {
	p.siteProvider = sp
}

func (p *Pipeline) SetClientProvider(cp model.DownloaderProvider) {
	p.clientProvider = cp
}

func (p *Pipeline) SetPTGenEndpoints(endpoints string) {
	if p.ptgen != nil {
		p.ptgen.SetEndpoints(endpoints)
	}
}

func (p *Pipeline) SetDeclarationFilter(df *DeclarationFilter) {
	p.declarationFilter = df
}

func (p *Pipeline) SetMetadataFetcher(f *metadata.Fetcher) {
	p.metadataFetcher = f
}

func (p *Pipeline) SetImageHostStrategy(strategy string) {
	if strategy != "" {
		p.imageHostStrategy = strategy
	}
}

func (p *Pipeline) SetImageHostManager(mgr *imagehost.Manager) {
	p.imageHostMgr = mgr
}

func (p *Pipeline) SetCompletionWatcher(w model.CompletionWatcher) {
	p.completionWatcher = w
}

func (p *Pipeline) SetNotifyService(ns *notification.Service) {
	p.notifyService = ns
}

func (p *Pipeline) SetScreenshotConfig(cfg screenshot.Config) {
	p.screenshotConfig = &cfg
	p.artifactGenerator = NewPublishArtifactGenerator(&cfg, p.logger)
}

func (p *Pipeline) SetBackpressureController(ctrl *BackpressureController) {
	p.backpressureCtrl = ctrl
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

func (p *Pipeline) DeleteTask(ctx context.Context, id uint) error {
	return p.db.WithContext(ctx).Delete(&model.PublishTask{}, id).Error
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

	var sourceDetail *model.TorrentDetail
	var torrentData []byte

	if candidate.SourceTorrentID != "" && p.siteProvider != nil {
		var sourceConfig *model.SiteConfig
		var sourceAdapter model.SiteAdapter
		sourceDetail, sourceConfig, sourceAdapter, err = p.fetchSourceInfo(ctx, candidate)
		if err != nil {
			return nil, err
		}
		torrentData, err = sourceAdapter.DownloadTorrent(ctx, sourceConfig, candidate.SourceTorrentID)
		if err != nil {
			if err := p.UpdateCandidateStatus(ctx, id, model.CandidateFailed, fmt.Sprintf("下载源种子失败: %v", err)); err != nil {
				p.logger.Warn("failed to update candidate status", zap.Uint("id", id), zap.Error(err))
			}
			return nil, &model.AppError{Code: 50001, Message: "下载源种子失败", Cause: err}
		}
	} else if candidate.DownloadCompleted && candidate.ClientID != "" && p.clientProvider != nil {
		torrentData, sourceDetail, err = p.fetchFromDownloader(ctx, candidate)
		if err != nil {
			if err := p.UpdateCandidateStatus(ctx, id, model.CandidateFailed, fmt.Sprintf("从下载器导出种子失败: %v", err)); err != nil {
				p.logger.Warn("failed to update candidate status", zap.Uint("id", id), zap.Error(err))
			}
			return nil, &model.AppError{Code: 50001, Message: "从下载器导出种子失败", Cause: err}
		}
	} else {
		if err := p.UpdateCandidateStatus(ctx, id, model.CandidateFailed, "无法获取种子文件（无源站ID且无法从下载器导出）"); err != nil {
			p.logger.Warn("failed to update candidate status", zap.Uint("id", id), zap.Error(err))
		}
		return nil, &model.AppError{Code: 50001, Message: "无法获取种子文件"}
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
			p.logger.Warn("failed to update candidate status", zap.Uint("id", id), zap.Error(err))
		}
		return nil, &model.AppError{Code: 40001, Message: fmt.Sprintf("发布合规检查未通过: %s", reason)}
	}

	if err := p.UpdateCandidateStatus(ctx, id, model.CandidatePublishing, ""); err != nil {
		return nil, &model.AppError{Code: 40001, Message: "failed to update candidate status", Cause: err}
	}

	return &candidate, nil
}

func (p *Pipeline) AnalyzeTorrent(ctx context.Context, name, savePath string) (map[string]interface{}, error) {
	result := map[string]interface{}{
		"description": "",
		"poster_url":  "",
		"media_info":  "",
		"screenshots": []string{},
		"douban_link": "",
		"imdb_link":   "",
		"tmdb_link":   "",
		"subtitle":    "",
	}

	// PTGen 查询（标题→简介/海报/链接）
	if p.ptgen != nil {
		if ptgenResult, err := p.ptgen.Query(ctx, name); err == nil && ptgenResult != nil {
			if ptgenResult.RawBBCode != "" {
				result["description"] = ptgenResult.RawBBCode
			}
			if ptgenResult.PosterURL != "" {
				result["poster_url"] = ptgenResult.PosterURL
			}
			if ptgenResult.DoubanURL != "" {
				result["douban_link"] = ptgenResult.DoubanURL
			}
			if ptgenResult.IMDBURL != "" {
				result["imdb_link"] = ptgenResult.IMDBURL
			}
			if ptgenResult.TMDbURL != "" {
				result["tmdb_link"] = ptgenResult.TMDbURL
			}
			if ptgenResult.ChineseTitle != "" {
				result["subtitle"] = ptgenResult.ChineseTitle
			}
			if len(ptgenResult.Genre) > 0 {
				result["ptgen_genre"] = strings.Join(ptgenResult.Genre, ",")
			}
			if ptgenResult.Episodes != "" {
				result["ptgen_episodes"] = ptgenResult.Episodes
			}
		}
	}

	// 本地产物（截图 + MediaInfo）
	if p.artifactGenerator != nil && savePath != "" {
		if artifact, err := p.artifactGenerator.Generate(ctx, savePath, "", nil); err == nil && artifact != nil {
			if artifact.MediaInfoText != "" {
				result["media_info"] = artifact.MediaInfoText
			}
			if len(artifact.ScreenshotURLs) > 0 {
				result["screenshots"] = artifact.ScreenshotURLs
			}
		} else if err != nil {
			p.logger.Warn("analyze: artifact generation failed", zap.Error(err))
		}
	}

	return result, nil
}

func (p *Pipeline) fetchFromDownloader(ctx context.Context, candidate *model.PublishCandidate) ([]byte, *model.TorrentDetail, error) {
	dlClient, err := p.clientProvider.Get(candidate.ClientID)
	if err != nil {
		return nil, nil, fmt.Errorf("获取下载器失败: %w", err)
	}

	torrentData, err := dlClient.ExportTorrent(ctx, candidate.InfoHash)
	if err != nil {
		return nil, nil, fmt.Errorf("导出种子失败: %w", err)
	}

	detail := &model.TorrentDetail{
		Title: candidate.TorrentName,
	}
	var savePath string
	if torrent, tErr := dlClient.GetTorrentByHash(ctx, candidate.InfoHash); tErr == nil && torrent != nil {
		detail.Size = torrent.TotalSize
		if detail.Title == "" {
			detail.Title = torrent.Name
		}
		savePath = torrent.SavePath
	}

	// 本地产物生成（截图 + MediaInfo）— 本地 MediaInfo 是发布前置条件（§48.9）
	if savePath == "" {
		return nil, nil, fmt.Errorf("local file path is empty, cannot verify file accessibility")
	}
	if p.artifactGenerator == nil {
		return nil, nil, fmt.Errorf("artifact generator not configured, cannot generate MediaInfo")
	}

	var sourceScreenshots []string
	var sourceMediaInfo string
	if p.metadataFetcher != nil {
		if meta, ok := p.metadataFetcher.GetMetadata(ctx, candidate.InfoHash, candidate.SourceSite); ok && meta != nil {
			if meta.Screenshots != "" {
				json.Unmarshal([]byte(meta.Screenshots), &sourceScreenshots)
			}
			if meta.SourceMediaInfo != "" {
				sourceMediaInfo = meta.SourceMediaInfo
			}
		}
	}

	strategy := p.imageHostStrategy
	if strategy == "" {
		strategy = "auto"
	}
	artifact, aErr := p.artifactGenerator.GenerateWithStrategy(ctx, savePath, sourceMediaInfo, sourceScreenshots, strategy)
	if aErr != nil {
		return nil, nil, fmt.Errorf("local MediaInfo generation failed: %w", aErr)
	}
	if artifact == nil || artifact.MediaInfoText == "" {
		return nil, nil, fmt.Errorf("local MediaInfo is empty, file may be corrupted or inaccessible")
	}
	detail.MediaInfo = artifact.MediaInfoText
	if len(artifact.ScreenshotURLs) > 0 {
		detail.Screenshots = artifact.ScreenshotURLs
	}

	// 存储 metadata（自动发布集成 §48.5）
	if p.metadataFetcher != nil {
		p.storeMetadataAsync(ctx, candidate, detail)
	}

	return torrentData, detail, nil
}

func (p *Pipeline) fetchSourceInfo(ctx context.Context, candidate *model.PublishCandidate) (*model.TorrentDetail, *model.SiteConfig, model.SiteAdapter, error) {
	sourceConfig, err := p.siteProvider.GetSiteConfig(ctx, candidate.SourceSite)
	if err != nil {
		if statusErr := p.UpdateCandidateStatus(ctx, candidate.ID, model.CandidateFailed, fmt.Sprintf("获取源站配置失败: %v", err)); statusErr != nil {
			p.logger.Warn("update candidate status failed", zap.Uint("id", candidate.ID), zap.Error(statusErr))
		}
		return nil, nil, nil, &model.AppError{Code: 50001, Message: "获取源站配置失败", Cause: err}
	}

	sourceAdapter, err := p.siteProvider.GetAdapter(ctx, candidate.SourceSite)
	if err != nil {
		if statusErr := p.UpdateCandidateStatus(ctx, candidate.ID, model.CandidateFailed, fmt.Sprintf("获取源站适配器失败: %v", err)); statusErr != nil {
			p.logger.Warn("update candidate status failed", zap.Uint("id", candidate.ID), zap.Error(statusErr))
		}
		return nil, nil, nil, &model.AppError{Code: 50001, Message: "获取源站适配器失败", Cause: err}
	}

	sourceDetail, err := sourceAdapter.GetTorrentDetail(ctx, sourceConfig, candidate.SourceTorrentID)
	if err != nil {
		p.logger.Warn("failed to get source torrent detail", zap.Error(err))
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
				if statusErr := p.UpdateCandidateStatus(ctx, candidate.ID, model.CandidateSkipped, reason); statusErr != nil {
					p.logger.Warn("update candidate status failed", zap.Uint("id", candidate.ID), zap.Error(statusErr))
				}
				return nil, nil, nil, &model.AppError{Code: 40001, Message: fmt.Sprintf("发布合规检查未通过: %s", reason)}
			}
		}
	}

	return sourceDetail, sourceConfig, sourceAdapter, nil
}

func (p *Pipeline) publishToTarget(ctx context.Context, candidate *model.PublishCandidate, targetSite string, sourceDetail *model.TorrentDetail, torrentData []byte) (bool, error) {
	var publishSucceeded bool
	if p.backpressureCtrl != nil {
		if err := p.backpressureCtrl.AcquireSlot(ctx, targetSite); err != nil {
			p.logger.Warn("backpressure: publish blocked",
				zap.String("target", targetSite),
				zap.Error(err))
			return false, err
		}
		defer func() {
			if p.backpressureCtrl != nil {
				p.backpressureCtrl.ReleaseSlot(targetSite, publishSucceeded)
			}
		}()
	}

	eligible, reason := p.CheckPublishEligibility(ctx, candidate, targetSite)
	if !eligible {
		p.logger.Info("target site publish excluded", zap.String("target", targetSite), zap.String("reason", reason))
		return false, nil
	}

	targetConfig, err := p.siteProvider.GetSiteConfig(ctx, targetSite)
	if err != nil {
		p.logger.Warn("failed to get target config", zap.String("site", targetSite), zap.Error(err))
		return false, nil
	}

	targetAdapter, err := p.siteProvider.GetAdapter(ctx, targetSite)
	if err != nil {
		p.logger.Warn("failed to get target adapter", zap.String("site", targetSite), zap.Error(err))
		return false, nil
	}

	title := candidate.TorrentName
	if p.siteProvider != nil && title != "" {
		dedupResults, dedupErr := targetAdapter.SearchTorrents(ctx, targetConfig, title, nil)
		if dedupErr == nil {
			for _, dr := range dedupResults {
				if dr.Size == candidate.Size && dr.Size > 0 {
					p.logger.Info("target site already has same resource, skipping",
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
					Title:        candidate.TorrentName,
					DownloaderID: candidate.ClientID,
				}); err != nil {
						p.logger.Warn("failed to record publish result", zap.Error(err))
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
	costMS := time.Since(start).Milliseconds()
	metrics.PublishDuration.WithLabelValues(targetSite).Observe(time.Since(start).Seconds())
	if err != nil {
		p.logger.Warn("upload to target site failed",
			zap.String("target", targetSite),
			zap.Error(err),
		)
		metrics.PublishTasksTotal.WithLabelValues(targetSite, "failed").Inc()
		audit.Log("system", "publish", "upload", "torrent", candidate.SourceTorrentID,
			fmt.Sprintf("发布失败 %s → %s: %s", candidate.SourceSite, targetSite, err.Error()), "failed")
		if err := p.CreateResult(ctx, &model.PublishResultRecord{
			CandidateID:  candidate.ID,
			SourceSite:   candidate.SourceSite,
			TargetSite:   targetSite,
			TorrentID:    candidate.SourceTorrentID,
			Status:       model.PublishResultFailed,
			ErrorMessage: err.Error(),
			Title:        candidate.TorrentName,
			DownloaderID: candidate.ClientID,
			CostMS:       costMS,
		}); err != nil {
			p.logger.Warn("failed to record publish result", zap.Error(err))
		}
		return false, err
	}
	audit.Log("system", "publish", "upload", "torrent", candidate.SourceTorrentID,
		fmt.Sprintf("发布 %s → %s", candidate.SourceSite, targetSite), "success")

	status := model.PublishResultCompleted
	if resp != nil && resp.IsExisting {
		status = model.PublishResultExists
	}
	if err := p.CreateResult(ctx, &model.PublishResultRecord{
		CandidateID: candidate.ID,
		SourceSite:  candidate.SourceSite,
		TargetSite:  targetSite,
		TorrentID:   resp.TorrentID,
		Status:      status,
		PublishURL:  resp.DetailURL,
		Title:       candidate.TorrentName,
		DownloaderID: candidate.ClientID,
		CostMS:      costMS,
	}); err != nil {
		p.logger.Warn("failed to record publish result", zap.Error(err))
	}

	metrics.PublishTasksTotal.WithLabelValues(targetSite, "completed").Inc()

	publishSucceeded = true
	return true, nil
}

type descResult struct {
	Text       string
	IMDbLink   string
	DoubanLink string
	TMDBID     string
}

func (p *Pipeline) renderDescription(ctx context.Context, sourceSite, targetSite, title string, sourceDetail *model.TorrentDetail) descResult {
	descriptionText := ""
	if sourceDetail != nil {
		descriptionText = sourceDetail.Description
	}

	descData := &model.DescriptionData{
		SourceSite: sourceSite,
	}
	if sourceDetail != nil {
		descData.MediaInfoText = sourceDetail.MediaInfo
		descData.Screenshots = sourceDetail.Screenshots
	}

	var result descResult
	ptgenResult, ptgenErr := p.queryPTGen(ctx, title)
	if ptgenErr == nil && ptgenResult != nil && ptgenResult.PosterURL != "" {
		descData.PosterURL = ptgenResult.PosterURL
		if ptgenResult.RawBBCode != "" {
			descData.PTGenBody = ptgenResult.RawBBCode
		}
		result.IMDbLink = ptgenResult.IMDBURL
		result.DoubanLink = ptgenResult.DoubanURL
		if ptgenResult.TMDbURL != "" {
			result.TMDBID = extractTMDBID(ptgenResult.TMDbURL)
		}
	} else if sourceDetail != nil && sourceDetail.PosterURL != "" {
		rehostedURL := p.rehostPoster(ctx, sourceDetail.PosterURL)
		descData.PosterURL = rehostedURL
	}

	if descriptionText == "" && descData.PTGenBody != "" {
		descriptionText = descData.PTGenBody
	}

	var descConfig model.SiteDescConfig
	siteInfo, siteInfoErr := p.siteProvider.GetSiteInfo(ctx, targetSite)
	if siteInfoErr == nil && siteInfo != nil {
		siteConfig, cfgErr := p.siteProvider.GetSiteConfig(ctx, targetSite)
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

	result.Text = descriptionText
	return result
}

func populateFormFields(fields map[string]string, detail *model.TorrentDetail) {
	if detail == nil {
		return
	}
	if detail.Category != "" {
		fields["category"] = detail.Category
	}
	if detail.Source != "" {
		fields["source"] = detail.Source
	}
	if detail.Resolution != "" {
		fields["resolution"] = detail.Resolution
	}
	if detail.Codec != "" {
		fields["codec"] = detail.Codec
	}
	if detail.AudioCodec != "" {
		fields["audioCodec"] = detail.AudioCodec
	}
	if detail.Processing != "" {
		fields["processing"] = detail.Processing
	}
	if detail.ReleaseGroup != "" {
		fields["team"] = detail.ReleaseGroup
	}
	if detail.Region != "" {
		fields["region"] = detail.Region
	}
	if detail.IMDbID != "" {
		fields["imdb"] = detail.IMDbID
	}
}

// applyUserOverrides 将手动转发时用户编辑的字段覆盖到 PublishRequest
// UserOverrides 是 JSON 字符串，包含 subtitle/statement/poster/douban_link/imdb_link/tmdb_link/tags/media_info/screenshots/description/bdinfo
func applyUserOverrides(pubReq *model.PublishRequest, overridesJSON string) {
	if overridesJSON == "" {
		return
	}
	var overrides map[string]interface{}
	if err := json.Unmarshal([]byte(overridesJSON), &overrides); err != nil {
		return
	}

	// 仅在用户提供了非空值时覆盖（不覆盖空值）
	if v, ok := overrides["subtitle"].(string); ok && v != "" {
		pubReq.Subtitle = v
	}
	if v, ok := overrides["description"].(string); ok && v != "" {
		pubReq.Description = v
	}
	if v, ok := overrides["media_info"].(string); ok && v != "" {
		pubReq.MediaInfo = v
	}
	if v, ok := overrides["bdinfo"].(string); ok && v != "" {
		pubReq.BDInfo = v
	}
	if v, ok := overrides["douban_link"].(string); ok && v != "" {
		pubReq.DoubanLink = v
	}
	if v, ok := overrides["imdb_link"].(string); ok && v != "" {
		pubReq.IMDbLink = v
	}
	if v, ok := overrides["tmdb_link"].(string); ok && v != "" {
		if pubReq.ExtraFields == nil {
			pubReq.ExtraFields = make(map[string]string)
		}
		pubReq.ExtraFields["tmdb_id"] = extractTMDBID(v)
	}
	// screenshots 是 []interface{}
	if screenshots, ok := overrides["screenshots"].([]interface{}); ok && len(screenshots) > 0 {
		var urls []string
		for _, s := range screenshots {
			if str, ok := s.(string); ok && str != "" {
				urls = append(urls, str)
			}
		}
		if len(urls) > 0 {
			pubReq.Screenshots = urls
		}
	}
	// tags 是 []interface{}
	if tags, ok := overrides["tags"].([]interface{}); ok && len(tags) > 0 {
		if pubReq.TagFields == nil {
			pubReq.TagFields = make(map[string]string)
		}
		for _, tag := range tags {
			if str, ok := tag.(string); ok && str != "" {
				pubReq.TagFields[str] = "1"
			}
		}
	}
}

// overridesString 从 UserOverrides JSON 中提取字符串值
func overridesString(overridesJSON, key string) (string, bool) {
	if overridesJSON == "" {
		return "", false
	}
	var overrides map[string]interface{}
	if err := json.Unmarshal([]byte(overridesJSON), &overrides); err != nil {
		return "", false
	}
	if v, ok := overrides[key].(string); ok {
		return v, true
	}
	return "", false
}

// applyTitleComponents 用用户编辑的标题组件覆盖表单字段
// 走标准化路径：原始值 → 标准键 → 规范显示名 → 表单字段
func applyTitleComponents(pubReq *model.PublishRequest, overridesJSON string) {
	if overridesJSON == "" {
		return
	}
	var overrides map[string]interface{}
	if err := json.Unmarshal([]byte(overridesJSON), &overrides); err != nil {
		return
	}
	tcRaw, ok := overrides["title_components"]
	if !ok || tcRaw == nil {
		return
	}
	tc, ok := tcRaw.(map[string]interface{})
	if !ok {
		return
	}

	// 构建 TitleComponents 并标准化
	components := titleparser.TitleComponents{
		Resolution:  getStringFromMap(tc, "resolution"),
		VideoCodec:  getStringFromMap(tc, "video_codec"),
		AudioCodec:  getStringFromMap(tc, "audio_codec"),
		Medium:      getStringFromMap(tc, "medium"),
		ReleaseGroup: getStringFromMap(tc, "release_group"),
	}
	stdParams, _ := titleparser.Standardize(components)

	// 用标准键逆向映射为规范显示名，再填入表单
	// 如果逆向映射失败（不在标准映射表中），回退到原始值
	if components.Resolution != "" {
		display := titleparser.ReverseLookup(stdParams.Resolution)
		if display == "" {
			display = components.Resolution
		}
		pubReq.FormFields["resolution"] = display
	}
	if components.VideoCodec != "" {
		display := titleparser.ReverseLookup(stdParams.VideoCodec)
		if display == "" {
			display = components.VideoCodec
		}
		pubReq.FormFields["codec"] = display
	}
	if components.AudioCodec != "" {
		display := titleparser.ReverseLookup(stdParams.AudioCodec)
		if display == "" {
			display = components.AudioCodec
		}
		pubReq.FormFields["audioCodec"] = display
	}
	if components.Medium != "" {
		display := titleparser.ReverseLookup(stdParams.Medium)
		if display == "" {
			display = components.Medium
		}
		pubReq.FormFields["source"] = display
	}
	if components.ReleaseGroup != "" {
		pubReq.FormFields["team"] = components.ReleaseGroup
	}
}

func getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func (p *Pipeline) buildPublishRequest(ctx context.Context, candidate *model.PublishCandidate, targetSite string, sourceDetail *model.TorrentDetail, torrentData []byte) (*model.PublishRequest, error) {
	title := candidate.TorrentName
	desc := p.renderDescription(ctx, candidate.SourceSite, targetSite, title, sourceDetail)

	pubReq := &model.PublishRequest{
		TorrentData:     torrentData,
		Title:           title,
		Description:     desc.Text,
		SourceSite:      candidate.SourceSite,
		SourceInfoHash:  candidate.InfoHash,
		SourceTorrentID: candidate.SourceTorrentID,
		TargetSite:      targetSite,
		FormFields:      make(map[string]string),
		IMDbLink:        desc.IMDbLink,
		DoubanLink:      desc.DoubanLink,
	}

	if desc.TMDBID != "" {
		pubReq.ExtraFields = map[string]string{"tmdb_id": desc.TMDBID}
	}

	if sourceDetail != nil {
		populateFormFields(pubReq.FormFields, sourceDetail)
		pubReq.Subtitle = sourceDetail.Subtitle
		pubReq.MediaInfo = sourceDetail.MediaInfo
		pubReq.Screenshots = sourceDetail.Screenshots
	}

	// 手动转发的用户编辑覆盖（优先于源站/PTGen 数据）
	applyUserOverrides(pubReq, candidate.UserOverrides)

	// 标题组件覆盖：用用户编辑的组件填充表单字段
	applyTitleComponents(pubReq, candidate.UserOverrides)

	// 截图默认在 description 内（大部分站点）；独立字段站点由适配器处理
	pubReq.ScreenshotInDesc = true
	if fw := p.getTargetFramework(ctx, targetSite); fw == "tnode" || fw == "zhuque" || fw == "haidan" || fw == "ptlgs" {
		pubReq.ScreenshotInDesc = false
	}

	// BDInfo 报告（如果有）
	if bdinfoText, ok := overridesString(candidate.UserOverrides, "bdinfo"); ok && bdinfoText != "" {
		pubReq.BDInfo = bdinfoText
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
			p.logger.Error("failed to update publish-completed status", zap.Uint("id", id), zap.Error(err))
		}
		return model.CandidateDone
	}
	if lastErr != nil {
		// 候选级重试：RetryCount < 3 → 改回 pending，等下次 ProcessPending 重试
		var fc model.PublishCandidate
		if err := p.db.WithContext(ctx).First(&fc, id).Error; err == nil && fc.RetryCount < 3 {
			now := time.Now()
			p.db.WithContext(ctx).Model(&model.PublishCandidate{}).Where("id = ?", id).Updates(map[string]interface{}{
				"publish_status": model.CandidatePending,
				"retry_count":    fc.RetryCount + 1,
				"publish_result": lastErr.Error(),
				"updated_at":     now,
			})
			p.logger.Info("candidate scheduled for retry",
				zap.Uint("id", id), zap.Int("retry", fc.RetryCount+1))
			return model.CandidatePending
		}
		if err := p.UpdateCandidateStatus(ctx, id, model.CandidateFailed, lastErr.Error()); err != nil {
			p.logger.Error("failed to update publish-failed status", zap.Uint("id", id), zap.Error(err))
		}
		return model.CandidateFailed
	}
	if err := p.UpdateCandidateStatus(ctx, id, model.CandidateSkipped, "所有目标站点均被去重或合规检查跳过"); err != nil {
		p.logger.Error("failed to update publish-skipped status", zap.Uint("id", id), zap.Error(err))
	}
	return model.CandidateSkipped
}

func (p *Pipeline) ListAllCandidates(ctx context.Context, page, pageSize int, status, search string) ([]model.PublishCandidate, int64, error) {
	var total int64
	q := p.db.WithContext(ctx).Model(&model.PublishCandidate{})
	if status != "" {
		q = q.Where("publish_status = ?", status)
	}
	if search != "" {
		like := "%" + search + "%"
		q = q.Where("torrent_name LIKE ? OR source_site LIKE ? OR target_sites LIKE ?", like, like, like)
	}
	if err := q.Count(&total).Error; err != nil {
		p.logger.Warn("query failed", zap.Error(err))
	}

	var candidates []model.PublishCandidate
	if err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&candidates).Error; err != nil {
		p.logger.Warn("query failed", zap.Error(err))
	}

	return candidates, total, nil
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

func (p *Pipeline) ListResultsFiltered(ctx context.Context, page, pageSize int, status, targetSite string) ([]model.PublishResultRecord, int64, error) {
	var results []model.PublishResultRecord
	var total int64
	q := p.db.WithContext(ctx).Model(&model.PublishResultRecord{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if targetSite != "" {
		q = q.Where("target_site = ?", targetSite)
	}
	if err := q.Count(&total).Error; err != nil {
		p.logger.Warn("query failed", zap.Error(err))
	}

	if err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&results).Error; err != nil {
		p.logger.Warn("query failed", zap.Error(err))
	}

	return results, total, nil
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
	// 1. 硬编码安全检查（始终运行，不可关闭）
	if eligible, reason := p.checkForbiddenContent([]string{candidate.TorrentName}); !eligible {
		return false, reason
	}

	if candidate.HasHR {
		return false, "源站种子存在 H&R (Hit and Run) 标记，跳过发布"
	}

	// 2. flags 检查（优先，来自 torrent_metadata）
	if p.checkFlagsFromMetadata(ctx, candidate.InfoHash, candidate.SourceSite) {
		return false, fmt.Sprintf("源站 flags 标记禁转/独占（torrent_metadata），跳过发布")
	}

	// 3. declaration_filter 检查（flags 不可用时兜底）
	if p.declarationFilter != nil {
		texts := []string{candidate.TorrentName}
		patterns := p.declarationFilter.GetPatterns(ctx)
		if len(patterns) > 0 {
			fr := p.declarationFilter.Filter(strings.Join(texts, " "), patterns)
			if len(fr.RemovedDecls) > 0 {
				return false, fmt.Sprintf("声明过滤命中: %v", fr.RemovedDecls)
			}
		}
	}

	// 硬编码互斥站点对（不可修改）
	if IsHardcodedExclusion(candidate.SourceSite, targetSite) {
		return false, fmt.Sprintf("源站 %s → 目标站 %s 为互斥站点（硬编码规则）", candidate.SourceSite, targetSite)
	}

	// 用户自定义排除规则
	if targetSite != "" && candidate.SourceSite != "" {
		var exclusion model.PublishExclusion
		err := p.db.WithContext(ctx).
			Where("target_site = ? AND source_site = ?", targetSite, candidate.SourceSite).
			First(&exclusion).Error
		if err == nil {
			return false, fmt.Sprintf("源站 %s → 目标站 %s 存在发布排除规则", candidate.SourceSite, targetSite)
		}
	}

	// 加种限制 Guard
	if p.limitGuard != nil {
		if allowed, reason := p.limitGuard.Check(ctx, targetSite); !allowed {
			return false, fmt.Sprintf("目标站 %s 加种限制: %s", targetSite, reason)
		}
	}

	return true, ""
}

func (p *Pipeline) checkFlagsFromMetadata(ctx context.Context, infoHash, siteName string) bool {
	if infoHash == "" {
		return false
	}
	var meta model.TorrentMetadata
	if err := p.db.WithContext(ctx).
		Where("info_hash = ?", infoHash).
		Order("updated_at DESC").
		First(&meta).Error; err != nil {
		return false
	}
	if meta.Flags == "" {
		return false
	}
	var flags []string
	if err := json.Unmarshal([]byte(meta.Flags), &flags); err != nil {
		return false
	}
	forbiddenFlags := map[string]bool{
		"禁转": true, "禁止转载": true, "谢绝转载": true,
		"严禁转载": true, "谢绝搬运": true, "独占": true, "限时禁转": true,
	}
	for _, f := range flags {
		if forbiddenFlags[f] {
			return true
		}
	}
	return false
}

// hardcodedExclusionPairs 硬编码互斥站点对（双向），禁止用户修改
var HardcodedExclusionPairs = map[string]map[string]bool{
	"不可说": {"优堡": true},
	"优堡":   {"不可说": true},
	"家园":   {"铂金家": true},
	"铂金家": {"家园": true},
}

func PublishHardcodedExclusionPairs() map[string]map[string]bool {
	return HardcodedExclusionPairs
}

func IsHardcodedExclusion(sourceSite, targetSite string) bool {
	if targets, ok := HardcodedExclusionPairs[sourceSite]; ok {
		return targets[targetSite]
	}
	return false
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

		// 原子抢占：status pending → publishing，防止并发重复处理
		result := p.db.WithContext(ctx).Model(&model.PublishCandidate{}).
			Where("id = ? AND publish_status = ?", c.ID, model.CandidatePending).
			Update("publish_status", model.CandidatePublishing)
		if result.Error != nil {
			p.logger.Warn("atomic claim failed",
				zap.Uint("id", c.ID),
				zap.Error(result.Error))
			continue
		}
		if result.RowsAffected == 0 {
			continue
		}
		c.PublishStatus = model.CandidatePublishing

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
			if c.Role == "manual" || c.RetryCount > 0 {
				go func(cid uint) {
					mctx, mcancel := context.WithTimeout(context.Background(), 5*time.Minute)
					defer mcancel()
					if _, merr := p.PublishCandidate(mctx, cid); merr != nil {
						p.logger.Warn("manual candidate publish failed", zap.Uint("id", cid), zap.Error(merr))
					}
				}(c.ID)
				continue
			}
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
			Where("publish_group_id = ? AND (status IN ? OR (status = ? AND retry_count < ?))",
				group.ID,
				[]model.MemberStatus{
					model.MemberStatusNew,
					model.MemberStatusUploading,
					model.MemberStatusInjected,
				},
				model.MemberStatusError,
				3,
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
	sourceIDs := make([]string, 0)
	for i := range events {
		if events[i].SourceID != "" {
			sourceIDs = append(sourceIDs, events[i].SourceID)
		}
	}

	clientIDMap := make(map[string]string, len(sourceIDs))
	if len(sourceIDs) > 0 {
		var results []struct {
			ID       uint   `gorm:"column:id"`
			ClientID string `gorm:"column:client_id"`
		}
		if err := p.db.WithContext(ctx).Table("rss_subscriptions").
			Select("id, client_id").
			Where("id IN ?", sourceIDs).
			Find(&results).Error; err != nil {
			p.logger.Warn("query subscription client IDs for notification", zap.Error(err))
		}
		for _, r := range results {
			clientIDMap[fmt.Sprintf("%d", r.ID)] = r.ClientID
		}
	}

	for i := range events {
		ev := &events[i]
		if ev.MatchedRuleName == "" {
			continue
		}

		var clientID string
		if ev.SourceID != "" {
			clientID = clientIDMap[ev.SourceID]
		}

		role := model.RoleDownload
		if ev.Metadata != nil {
			if r, ok := ev.Metadata["client_role"].(string); ok {
				switch model.PublishCandidateRole(r) {
				case model.RoleSource:
					role = model.RoleSource
				default:
					role = model.RoleDownload
				}
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
			Role:            role,
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
	m := reTMDBID.FindStringSubmatch(tmdbURL)
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
	if err := p.db.WithContext(ctx).Model(&model.PublishGroup{}).Count(&total).Error; err != nil {
		p.logger.Warn("count publish groups failed", zap.Error(err))
	}
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
		p.logger.Warn("failed to record status history", zap.Uint("groupID", groupID), zap.Error(err))
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
	StepHRDetect    = 7
)

func (p *Pipeline) ProcessMemberWithResume(ctx context.Context, member *model.PublishGroupMember) error {
	key := fmt.Sprintf("member:%d", member.ID)
	if _, loaded := p.memberMu.LoadOrStore(key, struct{}{}); loaded {
		p.logger.Debug("member already being processed, skip",
			zap.Uint("memberID", member.ID))
		return nil
	}
	defer p.memberMu.Delete(key)

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
			p.logger.Warn("failed to get source torrent detail", zap.Error(detErr))
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
							p.logger.Info("target site already has same resource, skipping",
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
		var imdbLink, doubanLink string

		if member.LastCompletedStep < StepRender {
			desc := p.renderDescription(ctx, group.SourceSite, member.SiteName, title, sourceDetail)
			descriptionText = desc.Text
			imdbLink = desc.IMDbLink
			doubanLink = desc.DoubanLink
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
			populateFormFields(pubReq.FormFields, sourceDetail)
			pubReq.MediaInfo = sourceDetail.MediaInfo
			pubReq.Screenshots = sourceDetail.Screenshots
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
		if err := p.db.WithContext(ctx).Model(member).Updates(map[string]interface{}{
			"torrent_id": resp.TorrentID,
			"status":     model.MemberStatusUploaded,
			"status_at":  &now,
		}).Error; err != nil {
			p.logger.Error("update member after upload failed", zap.Uint("memberID", member.ID), zap.Error(err))
		}
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
	advanceHR := func() {
		if err := p.advanceStep(ctx, member, StepHRDetect); err != nil {
			p.logger.Warn("advanceStep HR failed", zap.Uint("memberID", member.ID), zap.Error(err))
		}
	}

	if member.TorrentID == "" {
		advanceHR()
		return
	}

	targetConfig, cfgErr := p.siteProvider.GetSiteConfig(ctx, member.SiteName)
	if cfgErr != nil {
		p.logger.Warn("HR check: failed to get target config", zap.Error(cfgErr))
		advanceHR()
		return
	}

	targetAdapter, adpErr := p.siteProvider.GetAdapter(ctx, member.SiteName)
	if adpErr != nil {
		p.logger.Warn("HR check: failed to get target adapter", zap.Error(adpErr))
		advanceHR()
		return
	}

	hrResult, hrErr := targetAdapter.DetectHR(ctx, targetConfig, member.TorrentID)
	if hrErr != nil {
		p.logger.Warn("HR check failed",
			zap.String("site", member.SiteName),
			zap.String("torrentID", member.TorrentID),
			zap.Error(hrErr),
		)
		advanceHR()
		return
	}

	if hrResult != nil && hrResult.HasHR {
		hrSeedStart := time.Now()
		seedTimeH := hrResult.SeedTimeH
		if seedTimeH <= 0 {
			seedTimeH = 72
		}
		if err := p.db.WithContext(ctx).Model(member).Updates(map[string]interface{}{
			"hr_protected":      true,
			"hr_min_seed_hours": seedTimeH,
			"hr_min_ratio":      hrResult.MinRatio,
			"hr_seed_start":     &hrSeedStart,
			"hr_site":           member.SiteName,
		}).Error; err != nil {
			p.logger.Error("update member hr protection failed", zap.Uint("memberID", member.ID), zap.Error(err))
		}
		member.HRProtected = true
		member.HRSeedStart = &hrSeedStart
		member.HRMinSeedHours = seedTimeH

		hrTag := fmt.Sprintf("PROTECTED_HR_%s", member.SiteName)
		if p.clientProvider != nil && member.ClientID != "" && member.InfoHash != "" {
			if dl, dlErr := p.clientProvider.Get(member.ClientID); dlErr == nil {
				mergedTags := []string{hrTag}
				if current, gErr := dl.GetTorrentByHash(ctx, member.InfoHash); gErr == nil && current != nil {
					for _, t := range current.Tags {
						if t != hrTag {
							mergedTags = append(mergedTags, t)
						}
					}
				}
				if tagErr := dl.SetTorrentTags(ctx, member.InfoHash, mergedTags); tagErr != nil {
					p.logger.Warn("failed to set HR protect tag",
						zap.String("clientID", member.ClientID),
						zap.String("infoHash", member.InfoHash),
						zap.Error(tagErr),
					)
				}
			}
		}
		p.logger.Info("HR check: torrent marked as HR protected",
			zap.Uint("memberID", member.ID),
			zap.String("site", member.SiteName),
			zap.Int("seedTimeH", seedTimeH),
		)
	}
	advanceHR()
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
		p.logger.Warn("publish notification send failed", zap.Error(err))
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
		"status":      model.MemberStatusError,
		"last_error":  reason,
		"error_at":    &now,
		"status_at":   &now,
		"retry_count": member.RetryCount + 1,
	}).Error; err != nil {
		p.logger.Warn("failMember DB update failed", zap.Uint("memberID", member.ID), zap.Error(err))
	}
	member.RetryCount++
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
	if v, ok := fields["audioCodec"]; ok {
		mapped := mapKey("audiocodec_sel", v)
		if mapped == v {
			mapped = mapKey("audioCodec", v)
		}
		fields["audioCodec"] = mapped
	}
	if v, ok := fields["team"]; ok {
		mapped := mapKey("team_sel", v)
		if mapped == v {
			mapped = mapKey("team", v)
		}
		fields["team"] = mapped
	}
	if v, ok := fields["medium"]; ok {
		mapped := mapKey("medium_sel", v)
		if mapped == v {
			mapped = mapKey("medium", v)
		}
		fields["medium"] = mapped
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
		Where("id = ?", c.SubscriptionID)
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

	if err := p.UpdateCandidateStatus(ctx, c.ID, model.CandidateDownloading, ""); err != nil {
		p.logger.Warn("update candidate to downloading failed", zap.Uint("id", c.ID), zap.Error(err))
	}

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
		if err := p.completionWatcher.Watch(ctx, c.ClientID, result.InfoHash, c.ID); err != nil {
			p.logger.Warn("completion watcher register failed",
				zap.String("clientID", c.ClientID),
				zap.String("infoHash", result.InfoHash),
				zap.Error(err),
			)
		}
	}

	return nil
}

func (p *Pipeline) getTargetFramework(ctx context.Context, targetSite string) string {
	if p.siteProvider == nil {
		return ""
	}
	if siteInfo, err := p.siteProvider.GetSiteInfo(ctx, targetSite); err == nil && siteInfo != nil {
		return string(siteInfo.Framework)
	}
	return ""
}

func (p *Pipeline) storeMetadataAsync(ctx context.Context, candidate *model.PublishCandidate, detail *model.TorrentDetail) {
	if p.metadataFetcher == nil || candidate == nil || detail == nil {
		return
	}
	if candidate.InfoHash == "" || candidate.SourceSite == "" || candidate.SourceTorrentID == "" {
		return
	}
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		meta := &model.TorrentMetadata{
			InfoHash:        candidate.InfoHash,
			SiteName:        candidate.SourceSite,
			TorrentID:       candidate.SourceTorrentID,
			Title:           detail.Title,
			Subtitle:        detail.Subtitle,
			Description:     detail.Description,
			MediaInfo:       detail.MediaInfo,
			MediaInfoSource: "local_generated",
			FetchSource:     "auto_publish",
			FetchedAt:       time.Now(),
		}
		if detail.Category != "" {
			meta.SourceCategory = detail.Category
		}
		if len(detail.Tags) > 0 {
			if data, err := json.Marshal(detail.Tags); err == nil {
				meta.Tags = string(data)
			}
		}
		if len(detail.Screenshots) > 0 {
			if data, err := json.Marshal(detail.Screenshots); err == nil {
				meta.Screenshots = string(data)
			}
		}
		if detail.IMDbURL != "" {
			meta.IMDbURL = detail.IMDbURL
		}
		if detail.DoubanURL != "" {
			meta.DoubanURL = detail.DoubanURL
		}

		if err := p.db.WithContext(bgCtx).
			Where("info_hash = ? AND site_name = ?", meta.InfoHash, meta.SiteName).
			Assign(meta).
			FirstOrCreate(meta).Error; err != nil {
			p.logger.Debug("storeMetadataAsync: upsert failed",
				zap.String("info_hash", candidate.InfoHash),
				zap.Error(err))
		}
	}()
}

func (p *Pipeline) rehostPoster(ctx context.Context, sourceURL string) string {
	if sourceURL == "" {
		return ""
	}
	if p.imageHostMgr == nil || p.imageHostMgr.DefaultHost() == nil {
		return sourceURL
	}
	result, err := p.imageHostMgr.Rehost(ctx, sourceURL)
	if err != nil || result == nil || result.URL == "" {
		p.logger.Debug("poster rehost failed, using source URL",
			zap.String("source_url", sourceURL),
			zap.Error(err))
		return sourceURL
	}
	p.logger.Debug("poster rehosted",
		zap.String("source_url", sourceURL),
		zap.String("rehosted_url", result.URL))
	return result.URL
}
