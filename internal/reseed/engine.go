package reseed

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/ranfish/pt-forward/internal/fingerprint"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Engine struct {
	db             *gorm.DB
	logger         *zap.Logger
	siteProvider   model.SiteInfoProvider
	clientProvider model.DownloaderProvider
	iyuuService    model.IYUUService
	fpRepo         *fingerprint.Repository
	mu             sync.RWMutex
	tasks          map[uint]context.CancelFunc
}

func NewEngine(db *gorm.DB, logger *zap.Logger) *Engine {
	return &Engine{
		db:     db,
		logger: logger,
		tasks:  make(map[uint]context.CancelFunc),
	}
}

func (e *Engine) SetSiteProvider(sp model.SiteInfoProvider) {
	e.siteProvider = sp
}

func (e *Engine) SetFingerprintRepo(repo *fingerprint.Repository) {
	e.fpRepo = repo
}

func (e *Engine) SetClientProvider(cp model.DownloaderProvider) {
	e.clientProvider = cp
}

func (e *Engine) SetIYUUService(svc model.IYUUService) {
	e.iyuuService = svc
}

func (e *Engine) Start(ctx context.Context) error {
	var tasks []model.ReseedTask
	if err := e.db.WithContext(ctx).Where("enabled = ?", true).Find(&tasks).Error; err != nil {
		return reseedError(ErrReseedDB, "load reseed tasks", err)
	}

	for i := range tasks {
		e.startTask(ctx, &tasks[i])
	}

	e.logger.Info("reseed engine started", zap.Int("tasks", len(tasks)))
	return nil
}

func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	for id, cancel := range e.tasks {
		cancel()
		delete(e.tasks, id)
	}
	e.logger.Info("reseed engine stopped")
}

func (e *Engine) startTask(parentCtx context.Context, task *model.ReseedTask) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if old, ok := e.tasks[task.ID]; ok {
		old()
	}
	ctx, cancel := context.WithCancel(parentCtx)
	e.tasks[task.ID] = cancel
	e.db.WithContext(ctx).Model(task).Updates(map[string]interface{}{
		"status":     model.ReseedTaskIdle,
		"updated_at": time.Now(),
	})
}

func (e *Engine) CancelTask(taskID uint) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if cancel, ok := e.tasks[taskID]; ok {
		cancel()
		delete(e.tasks, taskID)
	}
}

type MatchInput struct {
	SourceInfoHash  string
	SourceSize      int64
	SourceTitle     string
	SourceSite      string
	TargetSite      string
	TargetTorrentID string
	TargetInfoHash  string
	TargetSize      int64
}

func MatchDecision(input MatchInput, sizeTolerance float64) model.DecisionType {
	if input.SourceInfoHash == input.TargetInfoHash && input.SourceInfoHash != "" {
		return model.DecisionSameInfoHash
	}

	if input.SourceSize == 0 || input.TargetSize == 0 {
		return model.DecisionNoDownloadLink
	}

	sizeDiff := float64(input.SourceSize-input.TargetSize) / float64(input.TargetSize) * 100
	if sizeDiff < 0 {
		sizeDiff = -sizeDiff
	}

	if sizeDiff <= sizeTolerance {
		if input.SourceSize == input.TargetSize {
			return model.DecisionMatch
		}
		return model.DecisionMatchSizeOnly
	}

	if sizeDiff <= sizeTolerance*5 {
		return model.DecisionFuzzySizeMismatch
	}

	return model.DecisionSizeMismatch
}

func (e *Engine) RunTask(ctx context.Context, task *model.ReseedTask) (*model.ReseedExecutionResult, error) {
	e.mu.Lock()
	if _, exists := e.tasks[task.ID]; !exists {
		ctx2, cancel := context.WithCancel(ctx)
		e.tasks[task.ID] = cancel
		ctx = ctx2
	}
	e.mu.Unlock()

	start := time.Now()
	e.db.WithContext(ctx).Model(task).Updates(map[string]interface{}{
		"status":     model.ReseedTaskRunning,
		"updated_at": start,
	})

	result := &model.ReseedExecutionResult{
		TaskID:      fmt.Sprintf("%d", task.ID),
		CompletedAt: time.Now(),
	}

	defer func() {
		result.Duration = time.Since(start).Seconds()
		status := model.ReseedTaskCompleted
		if ctx.Err() == context.Canceled {
			status = model.ReseedTaskCancelled
		} else if result.Failed > 0 && result.Matched == 0 {
			status = model.ReseedTaskFailed
		}
		e.db.WithContext(ctx).Model(task).Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		})
	}()

	clientIDs := ParseClientIDs(task.ClientIDs)
	if len(clientIDs) == 0 {
		return result, nil
	}

	var seedingRecords []model.SeedingTorrentRecord
	q := e.db.WithContext(ctx).Where("status = ?", "seeding")
	if len(clientIDs) == 1 {
		q = q.Where("client_id = ?", clientIDs[0])
	} else {
		q = q.Where("client_id IN ?", clientIDs)
	}
	if err := q.Find(&seedingRecords).Error; err != nil {
		return nil, &model.AppError{Code: 50001, Message: "查询做种记录失败", Cause: err}
	}
	result.TotalSources = len(seedingRecords)

	if len(seedingRecords) == 0 {
		return result, nil
	}

	var sourceSites []string
	if task.SourceSiteIDs != "" {
		sourceSites = ParseClientIDs(task.SourceSiteIDs)
	}

	targetSites := ParseClientIDs(task.TargetSiteIDs)

	var excludedSites []string
	if task.TargetSiteExcludes != "" {
		excludedSites = ParseClientIDs(task.TargetSiteExcludes)
	}

	sizeTolerance := task.SizeTolerancePercent
	if sizeTolerance <= 0 {
		sizeTolerance = 1.0
	}

	for _, rec := range seedingRecords {
		if ctx.Err() == context.Canceled {
			break
		}

		if len(sourceSites) > 0 {
			found := false
			for _, s := range sourceSites {
				if rec.SiteName == s {
					found = true
					break
				}
			}
			if !found {
				result.Skipped++
				continue
			}
		}

		var recTitle string
		if e.fpRepo != nil {
			if fp, fpErr := e.fpRepo.GetByInfoHashAndSite(ctx, rec.InfoHash, rec.SiteName); fpErr == nil && fp != nil {
				recTitle = fp.Title
			}
		}

		if !checkPublishEligibility(recTitle) {
			result.Blocked++
			continue
		}

		existingMatches, err := e.FindMatchesByInfoHash(ctx, rec.InfoHash)
		if err != nil {
			e.logger.Warn("查询已有匹配失败", zap.String("infoHash", rec.InfoHash), zap.Error(err))
			result.Failed++
			continue
		}
		if len(existingMatches) > 0 {
			result.DuplicateExists += len(existingMatches)
			continue
		}

		candidates := e.findCandidates(ctx, rec, targetSites, excludedSites, sizeTolerance, task)

		if task.EngineMode == "e2_auto" && e.iyuuService != nil && hasMatchMethod(task.MatchMethods, "iyuu") {
			iyuuCandidates := e.queryIYUU(ctx, rec, targetSites, excludedSites)
			if len(iyuuCandidates) > 0 {
				candidates = append(candidates, iyuuCandidates...)
			}
		}
		if len(candidates) == 0 {
			continue
		}

		for _, c := range candidates {
			if result.Matched >= task.MaxInjectionsPerRun && task.MaxInjectionsPerRun > 0 {
				break
			}

			if !checkPublishEligibility(recTitle) {
				result.Blocked++
				continue
			}

			decision := model.DecisionMatch
			switch {
			case c.TargetInfoHash == rec.InfoHash && c.TargetInfoHash != "":
				decision = model.DecisionSameInfoHash
			case c.MatchMethod == "iyuu":
				decision = model.DecisionMatch
			case c.MatchMethod == "fingerprint":
				decision = model.DecisionMatchPartial
			case c.MatchMethod == "size_title":
				decision = model.DecisionMatchSizeOnly
			}

			match := &model.ReseedMatch{
				ClientID:        rec.ClientID,
				SourceSite:      rec.SiteName,
				SourceTorrentID: rec.TorrentID,
				SourceInfoHash:  rec.InfoHash,
				TargetSite:      c.TargetSite,
				TargetTorrentID: c.TargetTorrentID,
				TargetInfoHash:  c.TargetInfoHash,
				MatchMethod:     c.MatchMethod,
				Confidence:      c.Confidence,
				DecisionType:    string(decision),
				Status:          model.MatchStatusMatched,
			}

			if err := e.SaveMatch(ctx, match); err != nil {
				e.logger.Warn("保存匹配结果失败",
					zap.String("sourceHash", rec.InfoHash),
					zap.String("targetSite", c.TargetSite),
					zap.Error(err),
				)
				result.Failed++
				continue
			}

			if e.clientProvider != nil && e.siteProvider != nil {
				if err := e.injectMatch(ctx, match, task); err != nil {
					e.logger.Warn("注入辅种失败",
						zap.Uint("matchID", match.ID),
						zap.String("targetSite", c.TargetSite),
						zap.Error(err),
					)
					result.Failed++
					continue
				}
				result.Injected++
			} else {
				result.Matched++
			}

			if task.InjectionIntervalS > 0 {
				jitter := 0
				if task.InjectionJitterS > 0 {
					jitter = rand.IntN(task.InjectionJitterS)
				}
				interval := time.Duration(task.InjectionIntervalS+jitter) * time.Second
				select {
				case <-time.After(interval):
				case <-ctx.Done():
					return result, nil
				}
			}
		}
	}

	return result, nil
}

func (e *Engine) findCandidates(ctx context.Context, rec model.SeedingTorrentRecord, targetSites, excludedSites []string, sizeTolerance float64, task *model.ReseedTask) []model.Candidate {
	if e.siteProvider == nil {
		return nil
	}

	var allSites []*model.SiteInfo
	if len(targetSites) > 0 {
		for _, siteName := range targetSites {
			info, err := e.siteProvider.GetSiteInfo(ctx, siteName)
			if err != nil {
				e.logger.Warn("获取目标站点信息失败", zap.String("site", siteName), zap.Error(err))
				continue
			}
			allSites = append(allSites, info)
		}
	} else {
		sites, err := e.siteProvider.ListSites(ctx)
		if err != nil {
			e.logger.Warn("列出站点失败", zap.Error(err))
			return nil
		}
		for _, s := range sites {
			if s.Name == rec.SiteName {
				continue
			}
			allSites = append(allSites, s)
		}
	}

	exclSet := make(map[string]bool, len(excludedSites))
	for _, s := range excludedSites {
		exclSet[s] = true
	}

	var candidates []model.Candidate

	for _, siteInfo := range allSites {
		if ctx.Err() == context.Canceled {
			break
		}
		if exclSet[siteInfo.Name] || !siteInfo.Enabled {
			continue
		}
		if siteInfo.Name == rec.SiteName {
			continue
		}

		siteConfig, err := e.siteProvider.GetSiteConfig(ctx, siteInfo.BaseURL)
		if err != nil {
			e.logger.Warn("获取站点配置失败", zap.String("site", siteInfo.Name), zap.Error(err))
			continue
		}

		adapter, err := e.siteProvider.GetAdapter(ctx, siteInfo.BaseURL)
		if err != nil {
			e.logger.Warn("获取适配器失败", zap.String("site", siteInfo.Name), zap.Error(err))
			continue
		}

		c := e.matchLayer1InfoHash(ctx, adapter, siteConfig, rec, siteInfo.Name)
		if c != nil {
			candidates = append(candidates, *c)
			continue
		}

		c = e.matchLayer2SizeTitle(ctx, adapter, siteConfig, rec, siteInfo.Name, sizeTolerance)
		if c != nil {
			candidates = append(candidates, *c)
			continue
		}

		c = e.matchLayer3Fingerprint(ctx, rec, siteInfo.Name)
		if c != nil {
			candidates = append(candidates, *c)
		}
	}

	return candidates
}

func (e *Engine) matchLayer1InfoHash(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, rec model.SeedingTorrentRecord, siteName string) *model.Candidate {
	if rec.InfoHash == "" {
		return nil
	}

	results, err := adapter.SearchTorrents(ctx, config, rec.InfoHash, nil)
	if err != nil {
		e.logger.Debug("Layer1 搜索失败", zap.String("site", siteName), zap.Error(err))
		return nil
	}

	for _, r := range results {
		if r.TorrentID == "" {
			continue
		}
		targetHash, err := adapter.GetTorrentInfoHash(ctx, config, r.TorrentID)
		if err != nil {
			continue
		}
		if targetHash == rec.InfoHash {
			return &model.Candidate{
				TargetSite:      siteName,
				TargetTorrentID: r.TorrentID,
				TargetInfoHash:  targetHash,
				Confidence:      1.0,
				MatchMethod:     "infohash_exact",
			}
		}
	}

	return nil
}

func (e *Engine) matchLayer2SizeTitle(ctx context.Context, adapter model.SiteAdapter, config *model.SiteConfig, rec model.SeedingTorrentRecord, siteName string, sizeTolerance float64) *model.Candidate {
	var fp *model.ContentFingerprint
	if e.fpRepo != nil {
		var err error
		fp, err = e.fpRepo.GetByInfoHashAndSite(ctx, rec.InfoHash, rec.SiteName)
		if err != nil {
			e.logger.Debug("Layer2 指纹查询失败", zap.String("site", rec.SiteName), zap.String("info_hash", rec.InfoHash), zap.Error(err))
			return nil
		}
	} else {
		var fpModel model.ContentFingerprint
		if err := e.db.WithContext(ctx).Where("info_hash = ? AND site_name = ?", rec.InfoHash, rec.SiteName).First(&fpModel).Error; err != nil {
			e.logger.Debug("Layer2 DB查询失败", zap.String("site", rec.SiteName), zap.String("info_hash", rec.InfoHash), zap.Error(err))
			return nil
		}
		fp = &fpModel
	}

	keyword := NormalizeTitle(fp.Title)
	if keyword == "" {
		return nil
	}

	results, err := adapter.SearchTorrents(ctx, config, keyword, nil)
	if err != nil {
		e.logger.Debug("Layer2 搜索失败", zap.String("site", siteName), zap.String("keyword", keyword), zap.Error(err))
		return nil
	}

	var best *model.Candidate
	for _, r := range results {
		if r.TorrentID == "" {
			continue
		}

		input := MatchInput{
			SourceSize: fp.TotalSize,
			TargetSize: r.Size,
		}
		decision := MatchDecision(input, sizeTolerance)

		if decision == model.DecisionMatch || decision == model.DecisionMatchSizeOnly || decision == model.DecisionSameInfoHash {
			confidence := 0.7
			if decision == model.DecisionMatch {
				confidence = 0.85
			} else if decision == model.DecisionSameInfoHash {
				confidence = 1.0
			}
			if best == nil || confidence > best.Confidence {
				best = &model.Candidate{
					TargetSite:      siteName,
					TargetTorrentID: r.TorrentID,
					Confidence:      confidence,
					MatchMethod:     "size_title",
				}
			}
		}
	}

	return best
}

func (e *Engine) matchLayer3Fingerprint(ctx context.Context, rec model.SeedingTorrentRecord, siteName string) *model.Candidate {
	var sourceFP *model.ContentFingerprint
	if e.fpRepo != nil {
		var err error
		sourceFP, err = e.fpRepo.GetByInfoHashAndSite(ctx, rec.InfoHash, rec.SiteName)
		if err != nil {
			e.logger.Debug("Layer3 源指纹查询失败", zap.String("site", rec.SiteName), zap.String("info_hash", rec.InfoHash), zap.Error(err))
			return nil
		}
	} else {
		var fp model.ContentFingerprint
		if err := e.db.WithContext(ctx).
			Where("info_hash = ? AND site_name = ?", rec.InfoHash, rec.SiteName).
			First(&fp).Error; err != nil {
			e.logger.Debug("Layer3 源指纹DB查询失败", zap.String("site", rec.SiteName), zap.String("info_hash", rec.InfoHash), zap.Error(err))
			return nil
		}
		sourceFP = &fp
	}

	if sourceFP.PiecesHash == "" && sourceFP.FilesHash == "" {
		return nil
	}

	var targetFPs []model.ContentFingerprint
	if e.fpRepo != nil {
		candidates, err := e.fpRepo.FindCandidatesBySite(ctx, siteName, rec.InfoHash, sourceFP.PiecesHash, sourceFP.TotalSize, 10)
		if err != nil {
			e.logger.Debug("Layer3 候选查询失败", zap.String("site", siteName), zap.Error(err))
			return nil
		}
		targetFPs = candidates
	} else {
		q := e.db.WithContext(ctx).Where("site_name = ? AND info_hash != ?", siteName, rec.InfoHash)
		if sourceFP.PiecesHash != "" {
			q = q.Where("pieces_hash = ?", sourceFP.PiecesHash)
		} else {
			q = q.Where("total_size = ?", sourceFP.TotalSize)
		}
		if err := q.Limit(10).Find(&targetFPs).Error; err != nil {
			return nil
		}
	}

	for _, tfp := range targetFPs {
		confidence := 0.6
		if sourceFP.PiecesHash != "" && tfp.PiecesHash == sourceFP.PiecesHash {
			confidence = 0.95
		}
		if sourceFP.TotalSize > 0 && tfp.TotalSize == sourceFP.TotalSize {
			confidence += 0.1
			if confidence > 1.0 {
				confidence = 1.0
			}
		}

		if tfp.TorrentID != "" {
			return &model.Candidate{
				TargetSite:      siteName,
				TargetTorrentID: tfp.TorrentID,
				TargetInfoHash:  tfp.InfoHash,
				Confidence:      confidence,
				MatchMethod:     "fingerprint",
			}
		}
	}

	return nil
}

func NormalizeTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}

	// Step 1: Unicode NFKD normalization equivalent — strip diacritics
	var norm strings.Builder
	for _, r := range title {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		norm.WriteRune(r)
	}
	title = norm.String()

	// Step 2: Lowercase
	title = strings.ToLower(title)

	// Step 3: Remove content within brackets and parentheses
	var clean strings.Builder
	depth := 0
	for _, r := range title {
		if r == '[' || r == '(' || r == '【' || r == '（' {
			depth++
			continue
		}
		if r == ']' || r == ')' || r == '】' || r == '）' {
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth == 0 {
			clean.WriteRune(r)
		}
	}
	title = clean.String()

	// Step 4: Collapse whitespace
	title = strings.Join(strings.Fields(title), " ")

	// Step 5: Trim
	title = strings.TrimSpace(title)

	// Step 6: Truncate at quality keywords
	stopWords := []string{"2160p", "1080p", "720p", "480p", "x264", "x265", "h264", "h265", "hevc", "web-dl", "bluray", "bdrip", "hdrip", "webrip", "remux"}
	lower := title
	for _, w := range stopWords {
		if idx := strings.Index(lower, w); idx > 3 {
			title = strings.TrimSpace(title[:idx])
			break
		}
	}

	if len(title) > 80 {
		title = title[:80]
	}

	return title
}

func (e *Engine) CreateTask(ctx context.Context, task *model.ReseedTask) error {
	task.Status = model.ReseedTaskIdle
	return e.db.WithContext(ctx).Create(task).Error
}

func (e *Engine) GetTask(ctx context.Context, id uint) (*model.ReseedTask, error) {
	var task model.ReseedTask
	err := e.db.WithContext(ctx).First(&task, id).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (e *Engine) ListTasks(ctx context.Context) ([]model.ReseedTask, error) {
	var tasks []model.ReseedTask
	err := e.db.WithContext(ctx).Order("name ASC").Find(&tasks).Error
	return tasks, err
}

func (e *Engine) UpdateTask(ctx context.Context, task *model.ReseedTask) error {
	return e.db.WithContext(ctx).Save(task).Error
}

func (e *Engine) DeleteTask(ctx context.Context, id uint) error {
	e.CancelTask(id)
	return e.db.WithContext(ctx).Delete(&model.ReseedTask{}, id).Error
}

func (e *Engine) ListByClientID(ctx context.Context, clientID string) ([]model.ReseedTask, error) {
	var tasks []model.ReseedTask
	err := e.db.WithContext(ctx).
		Where("client_ids = ? OR client_ids LIKE ? OR client_ids LIKE ? OR client_ids LIKE ?",
			clientID,
			clientID+",%",
			"%,"+clientID+",%",
			"%,"+clientID).
		Find(&tasks).Error
	return tasks, err
}

func (e *Engine) ListEnabled(ctx context.Context) ([]model.ReseedTask, error) {
	var tasks []model.ReseedTask
	err := e.db.WithContext(ctx).
		Where("enabled = ? AND status IN ?", true, []model.ReseedTaskStatus{model.ReseedTaskIdle, model.ReseedTaskRunning}).
		Find(&tasks).Error
	return tasks, err
}

func (e *Engine) BatchSaveMatches(ctx context.Context, matches []*model.ReseedMatch) error {
	return e.db.WithContext(ctx).Create(matches).Error
}

func (e *Engine) FindPendingRetry(ctx context.Context, limit int) ([]model.ReseedMatch, error) {
	var matches []model.ReseedMatch
	err := e.db.WithContext(ctx).
		Where("status = ? AND retry_count < max_retries AND next_retry_at <= ?", model.MatchStatusFailed, time.Now()).
		Order("next_retry_at ASC").
		Limit(limit).
		Find(&matches).Error
	return matches, err
}

func (e *Engine) UpdateMatchStatus(ctx context.Context, id uint, status string, failReason string) error {
	return e.db.WithContext(ctx).
		Model(&model.ReseedMatch{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      status,
			"fail_reason": failReason,
			"updated_at":  time.Now(),
		}).Error
}

func (e *Engine) SaveMatch(ctx context.Context, match *model.ReseedMatch) error {
	return e.db.WithContext(ctx).Create(match).Error
}

func (e *Engine) FindMatchesByInfoHash(ctx context.Context, infoHash string) ([]model.ReseedMatch, error) {
	var matches []model.ReseedMatch
	err := e.db.WithContext(ctx).
		Where("source_info_hash = ?", infoHash).
		Find(&matches).Error
	return matches, err
}

func (e *Engine) FindMatchByID(ctx context.Context, id uint) (*model.ReseedMatch, error) {
	var m model.ReseedMatch
	err := e.db.WithContext(ctx).First(&m, id).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (e *Engine) RetryMatch(ctx context.Context, id uint) (*model.ReseedMatch, error) {
	var m model.ReseedMatch
	if err := e.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}

	if m.Status != model.MatchStatusFailed {
		return nil, &model.AppError{Code: 40001, Message: fmt.Sprintf("只能重试失败的匹配记录，当前状态: %s", m.Status)}
	}

	now := time.Now()
	m.Status = model.MatchStatusMatched
	m.RetryCount++
	m.FailReason = ""
	m.NextRetryAt = &now

	if err := e.db.WithContext(ctx).Save(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (e *Engine) DeleteNegativeCache(ctx context.Context, infoHash, site string) (int64, error) {
	q := e.db.WithContext(ctx).Where("source_info_hash = ?", infoHash)
	if site != "" {
		q = q.Where("source_site = ?", site)
	}
	result := q.Delete(&model.ReseedNegativeCache{})
	return result.RowsAffected, result.Error
}

func (e *Engine) SetNegativeCache(ctx context.Context, sourceInfoHash, targetSite, reason, method string, layerDepth int, ttl time.Duration) error {
	entry := &model.ReseedNegativeCache{
		SourceSite:      targetSite,
		SourceInfoHash:  sourceInfoHash,
		ExcludedTargets: targetSite,
		LastMethod:      method,
		LayerDepth:      layerDepth,
		ExpiresAt:       time.Now().Add(ttl),
	}
	return e.db.WithContext(ctx).Create(entry).Error
}

func (e *Engine) GetNegativeCacheByHashes(ctx context.Context, hashes []string) ([]model.ReseedNegativeCache, error) {
	if len(hashes) == 0 {
		return nil, nil
	}
	var entries []model.ReseedNegativeCache
	err := e.db.WithContext(ctx).
		Where("source_info_hash IN ? AND expires_at > ?", hashes, time.Now()).
		Find(&entries).Error
	return entries, err
}

func (e *Engine) FlushNegativeCache(ctx context.Context) (int64, error) {
	result := e.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&model.ReseedNegativeCache{})
	return result.RowsAffected, result.Error
}

func (e *Engine) RunEnabledTasks(ctx context.Context) error {
	var tasks []model.ReseedTask
	if err := e.db.WithContext(ctx).Where("enabled = ?", true).Find(&tasks).Error; err != nil {
		return reseedError(ErrReseedDB, "query enabled reseed tasks", err)
	}

	if len(tasks) == 0 {
		return nil
	}

	e.logger.Info("running enabled reseed tasks", zap.Int("count", len(tasks)))

	for i := range tasks {
		if _, err := e.RunTask(ctx, &tasks[i]); err != nil {
			e.logger.Warn("reseed task failed",
				zap.Uint("task_id", tasks[i].ID),
				zap.String("name", tasks[i].Name),
				zap.Error(err),
			)
		}
	}

	return nil
}

func ParseClientIDs(ids string) []string {
	if ids == "" {
		return nil
	}
	parts := strings.Split(ids, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func checkPublishEligibility(title string) bool {
	if title == "" {
		return true
	}
	keywords := []string{"禁转", "独占", "谢绝转载", "限时禁转", "严禁转载"}
	for _, kw := range keywords {
		if strings.Contains(title, kw) {
			return false
		}
	}
	groups := []string{"CatEDU"}
	for _, g := range groups {
		if strings.Contains(title, g) {
			return false
		}
	}
	return true
}

func (e *Engine) injectMatch(ctx context.Context, match *model.ReseedMatch, task *model.ReseedTask) error {
	if e.siteProvider == nil || e.clientProvider == nil {
		return reseedError(ErrReseedConfig, "site provider or client provider not configured", nil)
	}

	e.db.WithContext(ctx).Model(match).Updates(map[string]interface{}{
		"status":     model.MatchStatusInjecting,
		"updated_at": time.Now(),
	})

	targetSiteInfo, err := e.siteProvider.GetSiteInfo(ctx, match.TargetSite)
	if err != nil {
		return e.failMatch(ctx, match, fmt.Sprintf("获取目标站信息失败: %v", err))
	}

	targetConfig, err := e.siteProvider.GetSiteConfig(ctx, targetSiteInfo.BaseURL)
	if err != nil {
		return e.failMatch(ctx, match, fmt.Sprintf("获取目标站配置失败: %v", err))
	}

	targetAdapter, err := e.siteProvider.GetAdapter(ctx, targetSiteInfo.BaseURL)
	if err != nil {
		return e.failMatch(ctx, match, fmt.Sprintf("获取目标站适配器失败: %v", err))
	}

	torrentData, err := targetAdapter.DownloadTorrent(ctx, targetConfig, match.TargetTorrentID)
	if err != nil {
		return e.failMatch(ctx, match, fmt.Sprintf("下载目标种子失败: %v", err))
	}

	dlClient, err := e.clientProvider.Get(match.ClientID)
	if err != nil {
		return e.failMatch(ctx, match, fmt.Sprintf("获取下载器客户端失败: %v", err))
	}

	opts := model.AddTorrentOptions{
		Category: task.ReseedCategory,
		Tags:     []string{"reseed", "pt-forward"},
		Paused:   false,
	}

	if len(torrentData) == 0 {
		return e.failMatch(ctx, match, "种子数据为空")
	}

	addResult, err := dlClient.AddFromFile(ctx, torrentData, opts)
	if err != nil {
		if strings.Contains(err.Error(), "already") || strings.Contains(err.Error(), "exist") {
			return e.failMatch(ctx, match, "种子已存在于下载器中")
		}
		return e.failMatch(ctx, match, fmt.Sprintf("注入种子到下载器失败: %v", err))
	}

	now := time.Now()
	return e.db.WithContext(ctx).Model(match).Updates(map[string]interface{}{
		"status":           model.MatchStatusInjected,
		"target_info_hash": addResult.InfoHash,
		"injected_at":      &now,
		"updated_at":       now,
	}).Error
}

func (e *Engine) failMatch(ctx context.Context, match *model.ReseedMatch, reason string) error {
	match.RetryCount++
	match.FailReason = reason

	decisionType := model.DecisionDownloadFailed
	switch {
	case strings.Contains(reason, "已存在"):
		decisionType = model.DecisionAlreadyExists
	case strings.Contains(reason, "禁转") || strings.Contains(reason, "独占"):
		decisionType = model.DecisionBlockedRelease
	}

	e.db.WithContext(ctx).Model(match).Updates(map[string]interface{}{
		"status":        model.MatchStatusFailed,
		"decision_type": string(decisionType),
		"fail_reason":   reason,
		"retry_count":   match.RetryCount,
		"updated_at":    time.Now(),
	})
	return reseedError(ErrReseedGeneric, reason, nil)
}

func hasMatchMethod(methodsStr, method string) bool {
	if methodsStr == "" {
		return true
	}
	for _, m := range ParseClientIDs(methodsStr) {
		if m == method {
			return true
		}
	}
	return false
}

func (e *Engine) queryIYUU(ctx context.Context, rec model.SeedingTorrentRecord, targetSites, excludedSites []string) []model.Candidate {
	results, err := e.iyuuService.QueryReseed(ctx, []string{rec.InfoHash})
	if err != nil {
		e.logger.Warn("IYUU 查询失败", zap.String("infoHash", rec.InfoHash), zap.Error(err))
		return nil
	}

	exclSet := make(map[string]bool, len(excludedSites))
	for _, s := range excludedSites {
		exclSet[s] = true
	}

	targetSet := make(map[string]bool, len(targetSites))
	for _, s := range targetSites {
		targetSet[s] = true
	}

	var candidates []model.Candidate
	for _, result := range results {
		if result.SourceInfoHash != rec.InfoHash {
			continue
		}
		for _, target := range result.Targets {
			siteName := e.iyuuSidToSite(ctx, target.Sid)
			if siteName == "" {
				continue
			}
			if exclSet[siteName] || siteName == rec.SiteName {
				continue
			}
			if len(targetSet) > 0 && !targetSet[siteName] {
				continue
			}
			candidates = append(candidates, model.Candidate{
				TargetSite:      siteName,
				TargetTorrentID: fmt.Sprintf("%d", target.TorrentID),
				TargetInfoHash:  target.InfoHash,
				Confidence:      0.9,
				MatchMethod:     "iyuu",
			})
		}
	}

	return candidates
}

func (e *Engine) iyuuSidToSite(ctx context.Context, sid int) string {
	var mapping model.IYUUSiteMapping
	if err := e.db.WithContext(ctx).Where("iyuu_sid = ?", sid).First(&mapping).Error; err != nil {
		return ""
	}

	if mapping.SiteName != "" && e.siteProvider != nil {
		info, err := e.siteProvider.GetSiteInfo(ctx, mapping.SiteName)
		if err == nil && info != nil {
			return info.Name
		}
	}

	if mapping.SiteDomain != "" && e.siteProvider != nil {
		info, err := e.siteProvider.GetSiteInfoByURL(ctx, mapping.SiteDomain)
		if err == nil && info != nil {
			return info.Name
		}
	}

	return mapping.SiteName
}
