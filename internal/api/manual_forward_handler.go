package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ranfish/pt-forward/internal/metadata"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ManualForwardHandler struct {
	db           *gorm.DB
	logger       *zap.Logger
	pipeline     PublishPipeline
	siteMgr      SiteManager
	clientMgr    MFClientProvider
	declFilter   *publish.DeclarationFilter
	bdinfoScanner *publish.BDInfoScanner
	metadataFetcher MetadataFetcherProvider
	taskStore    sync.Map
	taskSeq      atomic.Int64
	stopCh       chan struct{}
	stopOnce     sync.Once
}

type MetadataFetcherProvider interface {
	GetMetadata(ctx context.Context, infoHash, siteName string) (*model.TorrentMetadata, bool)
	FetchAndStore(ctx context.Context, infoHash, siteName, torrentID string) (*model.TorrentMetadata, error)
}

type PublishPipeline interface {
	PublishCandidate(ctx context.Context, id uint) (*model.PublishCandidate, error)
	AnalyzeTorrent(ctx context.Context, name, savePath string) (map[string]interface{}, error)
}

type SiteManager interface {
	ListSites(ctx context.Context) ([]*model.SiteInfo, error)
	GetSiteConfig(ctx context.Context, siteURL string) (*model.SiteConfig, error)
}

type MFClientProvider interface {
	Get(clientID string) (model.DownloaderClient, error)
}

func NewManualForwardHandler(db *gorm.DB, logger *zap.Logger) *ManualForwardHandler {
	h := &ManualForwardHandler{
		db:     db,
		logger: logger,
		stopCh: make(chan struct{}),
	}
	go h.cleanupTaskStore()
	return h
}

func (h *ManualForwardHandler) SetPipeline(p PublishPipeline)        { h.pipeline = p }
func (h *ManualForwardHandler) SetSiteManager(s SiteManager)         { h.siteMgr = s }
func (h *ManualForwardHandler) SetClientProvider(c MFClientProvider) { h.clientMgr = c }
func (h *ManualForwardHandler) SetDeclarationFilter(f *publish.DeclarationFilter) { h.declFilter = f }
func (h *ManualForwardHandler) SetBDInfoScanner(s *publish.BDInfoScanner) { h.bdinfoScanner = s }
func (h *ManualForwardHandler) SetMetadataFetcher(f MetadataFetcherProvider) { h.metadataFetcher = f }

func (h *ManualForwardHandler) Close() {
	h.stopOnce.Do(func() { close(h.stopCh) })
}

func (h *ManualForwardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(r.URL.Path, "/")
	switch {
	case strings.HasSuffix(path, "/manual-forward/seeded-torrents"):
		if r.Method == http.MethodGet {
			h.handleSeededTorrents(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
	case strings.HasSuffix(path, "/manual-forward/analyze") && r.Method == http.MethodPost:
		h.handleStartAnalyze(w, r)
	case strings.Contains(path, "/manual-forward/analyze/") && r.Method == http.MethodGet:
		h.handlePollAnalyze(w, r)
	case strings.HasSuffix(path, "/manual-forward/eligible-targets"):
		if r.Method == http.MethodPost {
			h.handleEligibleTargets(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
	case strings.HasSuffix(path, "/manual-forward/merge"):
		if r.Method == http.MethodPost {
			h.handleMerge(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
	case strings.HasSuffix(path, "/manual-forward/preview"):
		if r.Method == http.MethodPost {
			h.handlePreviewFields(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
	case strings.HasSuffix(path, "/manual-forward/submit"):
		if r.Method == http.MethodPost {
			h.handleSubmit(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
	case strings.HasSuffix(path, "/manual-forward/batch-submit"):
		if r.Method == http.MethodPost {
			h.handleBatchSubmit(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
	default:
		Error(w, http.StatusNotFound, 40400, "接口不存在")
	}
}

const taskStoreTTL = 30 * time.Minute

func (h *ManualForwardHandler) cleanupTaskStore() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-h.stopCh:
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-taskStoreTTL)
			deleted := 0
			h.taskStore.Range(func(key, value any) bool {
				task, ok := value.(*analyzeTask)
				if !ok || task.CreatedAt.Before(cutoff) {
					h.taskStore.Delete(key)
					deleted++
				}
				return true
			})
			if deleted > 0 && h.logger != nil {
				h.logger.Debug("manual forward task store cleanup",
					zap.Int("deleted", deleted),
				)
			}
		}
	}
}

func (h *ManualForwardHandler) handleSeededTorrents(w http.ResponseWriter, r *http.Request) {
	if h.clientMgr == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "客户端管理器未初始化")
		return
	}

	clientIDStr := r.URL.Query().Get("client_id")
	if clientIDStr == "" {
		var clients []model.ClientConfig
		if err := h.db.Where("enabled = ?", true).Find(&clients).Error; err != nil {
			h.logger.Warn("query failed", zap.Error(err))
		}

		if len(clients) == 0 {
			Success(w, []interface{}{})
			return
		}
		clientIDStr = fmt.Sprintf("%d", clients[0].ID)
	}
	clientID, err := strconv.ParseUint(clientIDStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的 client_id")
		return
	}

	var cfg model.ClientConfig
	if err := h.db.First(&cfg, clientID).Error; err != nil {
		Error(w, http.StatusNotFound, 40400, "下载器不存在")
		return
	}

	client, err := h.clientMgr.Get(cfg.Name)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("连接下载器失败: %v", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	torrents, err := client.GetSeedingTorrents(ctx)
	if err != nil {
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("获取种子列表失败: %v", err))
		return
	}

	type SeededTorrent struct {
		InfoHash    string `json:"info_hash"`
		Name        string `json:"name"`
		Size        int64  `json:"size"`
		SavePath    string `json:"save_path"`
		UploadSpeed int64  `json:"upload_speed"`
		Seeders     int    `json:"seeders"`
		State       string `json:"state"`
		ClientID    uint   `json:"client_id"`
	}

	var results []SeededTorrent
	for _, t := range torrents {
		results = append(results, SeededTorrent{
			InfoHash:    t.Hash,
			Name:        t.Name,
			Size:        t.TotalSize,
			SavePath:    t.SavePath,
			UploadSpeed: t.UploadSpeed,
			Seeders:     t.NumComplete,
			State:       t.State,
			ClientID:    uint(clientID),
		})
	}

	Success(w, results)
}

type analyzeTask struct {
	mu            sync.RWMutex           `json:"-"`
	ID            int64                  `json:"id"`
	Status        string                 `json:"status"`
	Error         string                 `json:"error,omitempty"`
	Result        map[string]interface{} `json:"result,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	Progress      int                    `json:"progress,omitempty"`
	ProgressText  string                 `json:"progress_text,omitempty"`
}

func (t *analyzeTask) setError(err string) {
	t.mu.Lock()
	t.Error = err
	t.Status = "failed"
	t.Progress = 0
	t.ProgressText = ""
	t.mu.Unlock()
}

func (t *analyzeTask) setResult(result map[string]interface{}) {
	t.mu.Lock()
	t.Result = result
	t.Status = "completed"
	t.Progress = 0
	t.ProgressText = ""
	t.mu.Unlock()
}

func (t *analyzeTask) setProgress(percent int, text string) {
	t.mu.Lock()
	t.Progress = percent
	t.ProgressText = text
	t.mu.Unlock()
}

func (t *analyzeTask) snapshot() *analyzeTask {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return &analyzeTask{
		ID:           t.ID,
		Status:       t.Status,
		Error:        t.Error,
		Result:       t.Result,
		CreatedAt:    t.CreatedAt,
		Progress:     t.Progress,
		ProgressText: t.ProgressText,
	}
}

func (h *ManualForwardHandler) handleStartAnalyze(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID         uint   `json:"client_id"`
		InfoHash         string `json:"info_hash"`
		Name             string `json:"name"`
		SavePath         string `json:"save_path"`
		SourceSite       string `json:"source_site,omitempty"`
		SourceTorrentID  string `json:"source_torrent_id,omitempty"`
		MetadataPriority string `json:"metadata_priority,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.InfoHash == "" {
		Error(w, http.StatusBadRequest, 40001, "info_hash 必填")
		return
	}
	if req.ClientID == 0 {
		Error(w, http.StatusBadRequest, 40001, "client_id 必填")
		return
	}

	taskID := h.taskSeq.Add(1)
	task := &analyzeTask{
		ID:        taskID,
		Status:    "running",
		CreatedAt: time.Now(),
	}
	h.taskStore.Store(taskID, task)

	go h.runAnalyze(task, req.ClientID, req.InfoHash, req.Name, req.SavePath, req.SourceSite, req.SourceTorrentID, req.MetadataPriority)

	Success(w, map[string]interface{}{"task_id": taskID})
}

func (h *ManualForwardHandler) runAnalyze(task *analyzeTask, clientID uint, infoHash, name, savePath, frontendSourceSite, frontendTorrentID, metadataPriority string) {
	defer func() {
		if r := recover(); r != nil {
			task.setError(fmt.Sprintf("分析异常: %v", r))
		}
	}()

	result := map[string]interface{}{
		"name":      name,
		"info_hash": infoHash,
		"save_path": savePath,
		"client_id": clientID,
	}

	var sites []model.Site
	if err := h.db.Where("enabled = ? AND is_source = ?", true, true).Find(&sites).Error; err != nil {
		h.logger.Warn("query failed", zap.Error(err))
	}

	sourceSite := ""
	for _, s := range sites {
		result["source_site"] = s.Name
		result["source_site_id"] = s.ID
		sourceSite = s.Name
		break
	}

	// §56.14 决策 3: 前端传值优先 > 反查 > 跳过详情采集
	sourceTorrentID := frontendTorrentID
	if frontendSourceSite != "" {
		sourceSite = frontendSourceSite
		result["source_site"] = sourceSite
	}

	// §56.14: 如果有 source_torrent_id，触发详情页采集（填充 detail_source_json）
	if sourceTorrentID != "" && sourceSite != "" && h.metadataFetcher != nil && infoHash != "" {
		fetchCtx, fetchCancel := context.WithTimeout(context.Background(), 30*time.Second)
		if fetchMeta, err := h.metadataFetcher.FetchAndStore(fetchCtx, infoHash, sourceSite, sourceTorrentID); err != nil {
			h.logger.Warn("analyze: detail fetch failed",
				zap.String("site", sourceSite),
				zap.String("torrent_id", sourceTorrentID),
				zap.Error(err))
			result["detail_fetch_error"] = err.Error()
		} else if fetchMeta != nil {
			result["detail_fetched"] = true
			h.logger.Info("analyze: detail fetched and stored",
				zap.String("site", sourceSite),
				zap.Int("detail_json_len", len(fetchMeta.DetailSourceJSON)))
		}
		fetchCancel()
	}

	var exclusions []model.PublishExclusion
	if err := h.db.Find(&exclusions).Error; err != nil {
		h.logger.Warn("query failed", zap.Error(err))
	}

	blockedTargets := []string{}
	for _, exc := range exclusions {
		if exc.SourceSite == sourceSite {
			blockedTargets = append(blockedTargets, exc.TargetSite)
		}
	}
	result["blocked_targets"] = blockedTargets

	nameLower := strings.ToLower(name)
	forbidden := false
	forbidReason := ""
	for _, kw := range []string{"禁转", "独占", "谢绝转载", "限时禁转", "禁止转载"} {
		if strings.Contains(nameLower, kw) {
			forbidden = true
			forbidReason = fmt.Sprintf("标题包含 \"%s\"", kw)
			break
		}
	}
	result["forbidden"] = forbidden
	result["forbid_reason"] = forbidReason

	result["title"] = name

	// Check torrent_metadata for cached analysis (idempotent regeneration)
	var cachedMeta model.TorrentMetadata
	hasCache := false
	if infoHash != "" {
		if err := h.db.Where("info_hash = ? AND (media_info != '' OR screenshots != '')", infoHash).First(&cachedMeta).Error; err == nil {
			hasCache = true
		}
	}

	if hasCache {
		result["description"] = cachedMeta.Description
		result["media_info"] = cachedMeta.MediaInfo
		result["poster_url"] = cachedMeta.Poster
		result["imdb_link"] = cachedMeta.IMDbURL
		result["douban_link"] = cachedMeta.DoubanURL
		result["tmdb_link"] = cachedMeta.TMDbURL
		result["subtitle"] = cachedMeta.Subtitle
		if cachedMeta.Screenshots != "" {
			result["screenshots"] = strings.Split(cachedMeta.Screenshots, "\n")
		} else {
			result["screenshots"] = []string{}
		}
		result["cached"] = true
	} else if h.pipeline != nil {
		analyzeCtx, analyzeCancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer analyzeCancel()
		if analyzeResult, analyzeErr := h.pipeline.AnalyzeTorrent(analyzeCtx, name, savePath); analyzeErr == nil && analyzeResult != nil {
			for _, k := range []string{"description", "media_info", "screenshots", "poster_url", "douban_link", "imdb_link", "tmdb_link", "subtitle"} {
				if v, ok := analyzeResult[k]; ok {
					result[k] = v
				}
			}
		}
	} else {
		result["description"] = ""
		result["media_info"] = ""
		result["screenshots"] = []string{}
		result["poster_url"] = ""
	}

	// 声明过滤
	if h.declFilter != nil {
		desc, _ := result["description"].(string)
		if desc != "" {
			patterns := h.declFilter.GetPatterns(context.Background())
			fr := h.declFilter.Filter(desc, patterns)
			result["description"] = fr.CleanedText
			result["removed_declarations"] = fr.RemovedDecls
		}
	}

	// BDInfo 扫描（检测到蓝光原盘时）
	if h.bdinfoScanner != nil && savePath != "" {
		bdinfoCtx, bdinfoCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		bdinfoReport, bdinfoErr := h.bdinfoScanner.ScanIfBD(bdinfoCtx, savePath, func(percent int, text string) {
			task.setProgress(percent, text)
		})
		bdinfoCancel()
		if bdinfoErr != nil {
			h.logger.Warn("analyze: BDInfo scan failed", zap.Error(bdinfoErr))
		}
		if bdinfoReport != "" {
			result["bdinfo"] = bdinfoReport
		}
	}

	// 标题解析 + MediaInfo 纠正 + 标准化
	mediaInfo, _ := result["media_info"].(string)
	components := titleparser.ParseTitle(name)
	if mediaInfo != "" {
		if err := titleparser.CorrectWithMediaInfo(&components, mediaInfo); err != nil {
			h.logger.Warn("analyze: MediaInfo correction failed", zap.Error(err))
		}
	}
	// 分类推断
	sourceCat := ""
	// 查 torrent_metadata 获取已缓存的源站分类
	if h.metadataFetcher != nil && infoHash != "" {
		if meta, ok := h.metadataFetcher.GetMetadata(context.Background(), infoHash, sourceSite); ok && meta != nil {
			if meta.StandardType != "" {
				sourceCat = meta.StandardType
			} else if meta.SourceCategory != "" {
				sourceCat = meta.SourceCategory
			}
			if meta.Subtitle != "" && result["subtitle"] == nil {
				result["subtitle"] = meta.Subtitle
			}
			if meta.IMDbURL != "" && result["imdb_link"] == nil {
				result["imdb_link"] = meta.IMDbURL
			}
			if meta.DoubanURL != "" && result["douban_link"] == nil {
				result["douban_link"] = meta.DoubanURL
			}
		}
	}
	ptgenGenre := ""
	if g, ok := result["ptgen_genre"].(string); ok {
		ptgenGenre = g
	}
	ptgenEpisodes := ""
	if e, ok := result["ptgen_episodes"].(string); ok {
		ptgenEpisodes = e
	}
	category := titleparser.InferCategory(components, sourceCat, ptgenGenre, ptgenEpisodes)
	// 标准化
	stdParams, _ := titleparser.Standardize(components)
	stdParams.Type = category

	result["title_components"] = components
	result["standardized_params"] = stdParams

	task.setResult(result)

	if !hasCache {
		h.persistAnalysis(infoHash, sourceSite, result)
	}
}

func (h *ManualForwardHandler) persistAnalysis(infoHash, siteName string, result map[string]interface{}) {
	if infoHash == "" {
		return
	}

	screenshots := ""
	if ss, ok := result["screenshots"]; ok {
		switch v := ss.(type) {
		case []string:
			screenshots = strings.Join(v, "\n")
		case []interface{}:
			parts := make([]string, 0, len(v))
			for _, s := range v {
				if str, ok := s.(string); ok {
					parts = append(parts, str)
				}
			}
			screenshots = strings.Join(parts, "\n")
		}
	}

	title, _ := result["title"].(string)
	desc, _ := result["description"].(string)
	mediaInfo, _ := result["media_info"].(string)
	poster, _ := result["poster_url"].(string)
	imdb, _ := result["imdb_link"].(string)
	douban, _ := result["douban_link"].(string)
	tmdb, _ := result["tmdb_link"].(string)
	subtitle, _ := result["subtitle"].(string)
	now := time.Now()

	var meta model.TorrentMetadata
	h.db.Where("info_hash = ?", infoHash).First(&meta)

	if meta.ID == 0 {
		meta = model.TorrentMetadata{
			InfoHash:    infoHash,
			SiteName:    siteName,
			Title:       title,
			Description: desc,
			MediaInfo:   mediaInfo,
			Screenshots: screenshots,
			Poster:      poster,
			IMDbURL:     imdb,
			DoubanURL:   douban,
			TMDbURL:     tmdb,
			Subtitle:    subtitle,
			FetchSource: "analyze",
			FetchedAt:   now,
		}
		if err := h.db.Create(&meta).Error; err != nil {
			h.logger.Warn("persist analysis: create failed", zap.Error(err))
		}
	} else {
		updates := map[string]interface{}{
			"title":        title,
			"description":  desc,
			"media_info":   mediaInfo,
			"screenshots":  screenshots,
			"poster":       poster,
			"imdb_url":     imdb,
			"douban_url":   douban,
			"tmdb_url":     tmdb,
			"subtitle":     subtitle,
			"fetch_source": "analyze",
			"fetched_at":   now,
		}
		if err := h.db.Model(&meta).Updates(updates).Error; err != nil {
			h.logger.Warn("persist analysis: update failed", zap.Error(err))
		}
	}
}

func (h *ManualForwardHandler) handlePollAnalyze(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	taskIDStr := parts[len(parts)-1]
	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, 40001, "无效的 task_id")
		return
	}

	val, ok := h.taskStore.Load(taskID)
	if !ok {
		Error(w, http.StatusNotFound, 40400, "分析任务不存在")
		return
	}
	task := val.(*analyzeTask)
	Success(w, task.snapshot())
}

func (h *ManualForwardHandler) handleEligibleTargets(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceSite     string   `json:"source_site"`
		BlockedTargets []string `json:"blocked_targets"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}

	var sites []model.Site
	if err := h.db.Where("enabled = ? AND is_target = ?", true, true).Find(&sites).Error; err != nil {
		h.logger.Warn("query failed", zap.Error(err))
	}

	blockedSet := map[string]bool{}
	for _, t := range req.BlockedTargets {
		blockedSet[t] = true
	}

	type EligibleTarget struct {
		ID       uint   `json:"id"`
		Name     string `json:"name"`
		Domain   string `json:"domain"`
		BaseURL  string `json:"base_url"`
		AuthType string `json:"auth_type"`
		Blocked  bool   `json:"blocked"`
	}

	var targets []EligibleTarget
	for _, s := range sites {
		if s.Name == req.SourceSite {
			continue
		}
		targets = append(targets, EligibleTarget{
			ID:       s.ID,
			Name:     s.Name,
			Domain:   s.Domain,
			BaseURL:  s.BaseURL,
			AuthType: s.AuthType,
			Blocked:  blockedSet[s.Name],
		})
	}

	Success(w, targets)
}

func (h *ManualForwardHandler) handleSubmit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID    uint     `json:"client_id"`
		InfoHash    string   `json:"info_hash"`
		SourceSite  string   `json:"source_site"`
		SourceID    uint     `json:"source_site_id"`
		TorrentName string   `json:"title"`
		Description string   `json:"description"`
		MediaInfo   string   `json:"media_info"`
		Screenshots []string `json:"screenshots"`
		TargetSites []string `json:"target_sites"`
		PosterURL   string   `json:"poster_url"`
		Subtitle    string   `json:"subtitle"`
		Statement   string   `json:"statement"`
		Poster      string   `json:"poster"`
		DoubanLink  string   `json:"douban_link"`
		ImdbLink    string   `json:"imdb_link"`
		TmdbLink    string   `json:"tmdb_link"`
		Tags        []string `json:"tags"`
		TitleComponents map[string]string `json:"title_components"`
		BDInfo       string   `json:"bdinfo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.InfoHash == "" || len(req.TargetSites) == 0 {
		Error(w, http.StatusBadRequest, 40001, "info_hash 和 target_sites 必填")
		return
	}
	if req.ClientID == 0 {
		Error(w, http.StatusBadRequest, 40001, "client_id 必填")
		return
	}
	if req.SourceSite == "" {
		Error(w, http.StatusBadRequest, 40001, "source_site 必填")
		return
	}

	targetsJSON, _ := json.Marshal(req.TargetSites)

	// 额外字段存入 user_overrides JSON
	if req.PosterURL == "" {
		req.PosterURL = req.Poster
	}
	overrides := map[string]interface{}{
		"subtitle":    req.Subtitle,
		"statement":   req.Statement,
		"poster":      req.PosterURL,
		"douban_link": req.DoubanLink,
		"imdb_link":   req.ImdbLink,
		"tmdb_link":   req.TmdbLink,
		"tags":        req.Tags,
		"media_info":  req.MediaInfo,
		"screenshots": req.Screenshots,
		"description": req.Description,
	}
	if len(req.TitleComponents) > 0 {
		overrides["title_components"] = req.TitleComponents
	}
	if req.BDInfo != "" {
		overrides["bdinfo"] = req.BDInfo
	}
	overridesJSON, _ := json.Marshal(overrides)

	candidate := &model.PublishCandidate{
		SourceSite:        req.SourceSite,
		InfoHash:          req.InfoHash,
		TorrentName:       req.TorrentName,
		ClientID:          fmt.Sprintf("%d", req.ClientID),
		TargetSites:       string(targetsJSON),
		PublishStatus:     model.CandidatePending,
		DownloadCompleted: true,
		Role:              "manual",
		UserOverrides:     string(overridesJSON),
	}

	if err := h.db.Create(candidate).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("创建候选失败: %v", err))
		return
	}

	// 手动转发提交后立即异步触发 7 步管线（避免等 30min publish_pending tick）
	h.triggerPublishAsync(candidate.ID)

	Success(w, map[string]interface{}{
		"candidate_id": candidate.ID,
		"status":       "publishing",
	})
}

// triggerPublishAsync 异步触发候选发布，不阻塞 submit 响应。
// 结果写 publish_result_records，前端通过 /publish/results?candidate_id= 或 WS 查看进度。
func (h *ManualForwardHandler) triggerPublishAsync(candidateID uint) {
	if h.pipeline == nil {
		h.logger.Warn("manual forward: pipeline not configured, candidate will wait for publish_pending tick",
			zap.Uint("candidate_id", candidateID))
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		if _, err := h.pipeline.PublishCandidate(ctx, candidateID); err != nil {
			h.logger.Warn("manual forward: async publish failed",
				zap.Uint("candidate_id", candidateID),
				zap.Error(err))
		}
	}()
}

func (h *ManualForwardHandler) handleBatchSubmit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items []struct {
			ClientID    uint     `json:"client_id"`
			InfoHash    string   `json:"info_hash"`
			SourceSite  string   `json:"source_site"`
			SourceID    uint     `json:"source_site_id"`
			TorrentName string   `json:"title"`
			TargetSites []string `json:"target_sites"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if len(req.Items) == 0 {
		Error(w, http.StatusBadRequest, 40001, "items 不能为空")
		return
	}
	if len(req.Items) > 100 {
		Error(w, http.StatusBadRequest, 40001, "单次最多 100 条")
		return
	}

	var ids []uint
	for _, item := range req.Items {
		if item.InfoHash == "" || item.ClientID == 0 || len(item.TargetSites) == 0 {
			continue
		}
		targetsJSON, _ := json.Marshal(item.TargetSites)
		candidate := &model.PublishCandidate{
			SourceSite:        item.SourceSite,
			InfoHash:          item.InfoHash,
			TorrentName:       item.TorrentName,
			ClientID:          fmt.Sprintf("%d", item.ClientID),
			TargetSites:       string(targetsJSON),
			PublishStatus:     model.CandidatePending,
			DownloadCompleted: true,
			Role:              "manual",
		}
		if err := h.db.Create(candidate).Error; err != nil {
			h.logger.Warn("batch submit: create candidate failed",
				zap.String("hash", item.InfoHash),
				zap.Error(err))
			continue
		}
		h.triggerPublishAsync(candidate.ID)
		ids = append(ids, candidate.ID)
	}

	Success(w, map[string]interface{}{
		"created_count": len(ids),
		"candidate_ids": ids,
	})
}

// handleMerge §56.14 Q1: 前端 toggle 切换时重新合并三源数据。
// 请求体: { info_hash, mode }
// mode: "ptgen_first"（默认）| "detail_first"
// 从 DB 读三源 JSON，调 metadata.Merge 合并，返回 MergedMetadata。
func (h *ManualForwardHandler) handleMerge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InfoHash string `json:"info_hash"`
		Mode     string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求参数错误")
		return
	}
	if req.InfoHash == "" {
		Error(w, http.StatusBadRequest, 40001, "info_hash 不能为空")
		return
	}
	if req.Mode == "" {
		req.Mode = model.MetadataPriorityDefault
	}

	// 从 DB 读 torrent_metadata（取最新一条）
	var meta model.TorrentMetadata
	if err := h.db.WithContext(r.Context()).
		Where("info_hash = ?", req.InfoHash).
		Order("updated_at DESC").
		First(&meta).Error; err != nil {
		Error(w, http.StatusNotFound, 40401, "未找到该种子的元数据")
		return
	}

	// 反序列化三源 JSON
	detail, err := metadata.UnmarshalDetailSource(meta.DetailSourceJSON)
	if err != nil {
		h.logger.Warn("merge: unmarshal detail_source_json failed", zap.Error(err))
	}
	ptgen, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON)
	if err != nil {
		h.logger.Warn("merge: unmarshal ptgen_source_json failed", zap.Error(err))
	}
	local, err := metadata.UnmarshalLocalSource(meta.LocalSourceJSON)
	if err != nil {
		h.logger.Warn("merge: unmarshal local_source_json failed", zap.Error(err))
	}

	// 合并
	merged := metadata.Merge(detail, ptgen, local, metadata.MergeMode(req.Mode))

	// 附带三源状态（UI 显示用）
	result := map[string]interface{}{
		"merged":           merged,
		"has_detail_source": meta.DetailSourceJSON != "",
		"has_ptgen_source":  meta.PTGenSourceJSON != "",
		"has_local_source":  meta.LocalSourceJSON != "",
		"last_merge_mode":   meta.LastMergeMode,
	}

	Success(w, result)
}

// handlePreviewFields §56.24 Q3: 字段预览接口（reverse mapping UI）。
// 请求体: { info_hash, target_site, mode }
// 返回: PreviewResponse（字段列表 + 来源徽标 + 完整度检查）。
func (h *ManualForwardHandler) handlePreviewFields(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InfoHash    string            `json:"info_hash"`
		TargetSite  string            `json:"target_site"`
		Mode        string            `json:"mode"`
		UserOverrides map[string]string `json:"user_overrides,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求参数错误")
		return
	}
	if req.InfoHash == "" {
		Error(w, http.StatusBadRequest, 40001, "info_hash 不能为空")
		return
	}
	if req.Mode == "" {
		req.Mode = model.MetadataPriorityDefault
	}

	// 从 DB 读 torrent_metadata
	var meta model.TorrentMetadata
	if err := h.db.WithContext(r.Context()).
		Where("info_hash = ?", req.InfoHash).
		Order("updated_at DESC").
		First(&meta).Error; err != nil {
		Error(w, http.StatusNotFound, 40401, "未找到该种子的元数据")
		return
	}

	// 构建预览
	builder := metadata.NewPreviewBuilder()
	resp := builder.BuildPreviewFromMeta(&meta, req.UserOverrides, req.TargetSite, req.Mode)
	Success(w, resp)
}
