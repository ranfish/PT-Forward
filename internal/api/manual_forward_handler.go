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

	"github.com/ranfish/pt-forward/internal/compliance"
	"github.com/ranfish/pt-forward/internal/imagehost"
	"github.com/ranfish/pt-forward/internal/metadata"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"github.com/ranfish/pt-forward/internal/site"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ManualForwardHandler struct {
	db            *gorm.DB
	logger        *zap.Logger
	pipeline      PublishPipeline
	siteMgr       SiteManager
	clientMgr     MFClientProvider
	seedingCache  SeedingCacheProvider
	declFilter    *publish.DeclarationFilter
	bdinfoScanner *publish.BDInfoScanner
	metadataFetcher MetadataFetcherProvider
	coverage        CoverageServiceProvider
	sourceDetector  *publish.SourceSiteDetector
	complianceChecker *compliance.Checker
	imageHostMgr    *imagehost.Manager
	taskStore    sync.Map
	taskSeq      atomic.Int64
	stopCh       chan struct{}
	stopOnce     sync.Once
}

// SeedingCacheProvider 从 seeding engine 读取已缓存的种子列表（避免直连下载器）。
type SeedingCacheProvider interface {
	GetCachedTorrents(clientName string) []*model.TorrentInfo
}

// CoverageServiceProvider §56.33 决策 C1：源站识别用 coverage 历史数据查 tid。
type CoverageServiceProvider interface {
	GetCachedCoverage(ctx context.Context, infoHash string) ([]model.SiteCoverageCache, error)
}

type MetadataFetcherProvider interface {
	GetMetadata(ctx context.Context, infoHash, siteName string) (*model.TorrentMetadata, bool)
	FetchAndStore(ctx context.Context, infoHash, siteName, torrentID string) (*model.TorrentMetadata, error)
	FetchAndStoreBySearch(ctx context.Context, infoHash, siteName, torrentName string, size int64) (*model.TorrentMetadata, error)
}

type PublishPipeline interface {
	PublishCandidate(ctx context.Context, id uint) (*model.PublishCandidate, error)
	AnalyzeTorrent(ctx context.Context, name, savePath string) (map[string]interface{}, error)
	AnalyzePTGen(ctx context.Context, name string) (*model.PTGenResult, error)
	AnalyzeLocalArtifacts(ctx context.Context, name, savePath string) (map[string]interface{}, error)
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
func (h *ManualForwardHandler) SetSeedingCache(s SeedingCacheProvider) { h.seedingCache = s }
func (h *ManualForwardHandler) SetDeclarationFilter(f *publish.DeclarationFilter) { h.declFilter = f }
func (h *ManualForwardHandler) SetBDInfoScanner(s *publish.BDInfoScanner) { h.bdinfoScanner = s }
func (h *ManualForwardHandler) SetMetadataFetcher(f MetadataFetcherProvider) { h.metadataFetcher = f }
func (h *ManualForwardHandler) SetCoverageService(c CoverageServiceProvider) { h.coverage = c }
func (h *ManualForwardHandler) SetSourceDetector(d *publish.SourceSiteDetector) { h.sourceDetector = d }
func (h *ManualForwardHandler) SetComplianceChecker(c *compliance.Checker)    { h.complianceChecker = c }
func (h *ManualForwardHandler) SetImageHostManager(m *imagehost.Manager)     { h.imageHostMgr = m }

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
	case strings.HasSuffix(path, "/manual-forward/refresh"):
		if r.Method == http.MethodPost {
			h.handleRefresh(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
	case strings.HasSuffix(path, "/manual-forward/parse-title") && r.Method == http.MethodPost:
		h.handleParseTitle(w, r)
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

type seededTorrent struct {
	InfoHash    string `json:"info_hash"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	SavePath    string `json:"save_path"`
	UploadSpeed int64  `json:"upload_speed"`
	Seeders     int    `json:"seeders"`
	State       string `json:"state"`
	ClientID    uint   `json:"client_id"`
	SourceSite  string `json:"source_site"`
}

// dedupSeededTorrents 按种子名称完全匹配去重。
// 同组多条时优先保留官组源站（release_group_mappings 映射命中）的那条，
// 无官组映射或组内无匹配则保留第一条。
func (h *ManualForwardHandler) dedupSeededTorrents(ctx context.Context, items []seededTorrent) []seededTorrent {
	if len(items) <= 1 {
		return items
	}

	nameGroups := map[string][]int{}
	nameOrder := []string{}
	for i, r := range items {
		if _, ok := nameGroups[r.Name]; !ok {
			nameOrder = append(nameOrder, r.Name)
		}
		nameGroups[r.Name] = append(nameGroups[r.Name], i)
	}

	if len(nameOrder) == len(items) {
		return items
	}

	deduped := make([]seededTorrent, 0, len(nameOrder))
	for _, name := range nameOrder {
		indices := nameGroups[name]
		if len(indices) == 1 {
			deduped = append(deduped, items[indices[0]])
			continue
		}

		picked := indices[0]
		groupName := publish.ExtractGroupName(name)
		if groupName != "" && h.sourceDetector != nil {
			officialSite := h.sourceDetector.LookupGroup(ctx, groupName)
			if officialSite != "" {
				for _, idx := range indices {
					if items[idx].SourceSite == officialSite {
						picked = idx
						break
					}
				}
			}
		}
		deduped = append(deduped, items[picked])
	}
	return deduped
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

	// 优先从 seeding engine 缓存读取（内存命中=毫秒级），缓存缺失才回退直连下载器
	var torrents []*model.TorrentInfo
	cacheHit := false
	if h.seedingCache != nil {
		if cached := h.seedingCache.GetCachedTorrents(cfg.Name); cached != nil {
			torrents = filterSeedingTorrents(cached)
			cacheHit = true
			h.logger.Debug("seeded-torrents from cache",
				zap.String("client", cfg.Name),
				zap.Int("count", len(torrents)))
		}
	}
	if !cacheHit {
		var err error
		torrents, err = client.GetSeedingTorrents(ctx)
		if err != nil {
			Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("获取种子列表失败: %v", err))
			return
		}
	}

	matcher := site.NewTrackerMatcher(h.db)

	results := make([]seededTorrent, 0, len(torrents))
	for _, t := range torrents {
		sourceSite := ""
		if t.TrackerURL != "" {
			sourceSite = matcher.Match(t.TrackerURL)
		}
		results = append(results, seededTorrent{
			InfoHash:    t.Hash,
			Name:        t.Name,
			Size:        t.TotalSize,
			SavePath:    t.SavePath,
			UploadSpeed: t.UploadSpeed,
			Seeders:     t.NumComplete,
			State:       t.State,
			ClientID:    uint(clientID),
			SourceSite:  sourceSite,
		})
	}

	results = h.dedupSeededTorrents(ctx, results)

	Success(w, results)
}

// filterSeedingTorrents 从 maindataCache 全量种子中过滤出做种/已完成的。
// qBittorrent 做种状态以 "UP" 结尾；Transmission 用 "uploading"（status 6）。
func filterSeedingTorrents(all []*model.TorrentInfo) []*model.TorrentInfo {
	result := make([]*model.TorrentInfo, 0, len(all))
	for _, t := range all {
		if t == nil || t.Removed {
			continue
		}
		if strings.HasSuffix(t.State, "UP") || t.State == "uploading" {
			result = append(result, t)
		}
	}
	return result
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
	FetchSource   string                 `json:"-"`
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
		Size             int64  `json:"size,omitempty"`
		SourceSite       string `json:"source_site,omitempty"`
		SourceTorrentID  string `json:"source_torrent_id,omitempty"`
		MetadataPriority string `json:"metadata_priority,omitempty"`
		FetchSource      string `json:"fetch_source,omitempty"`
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
		ID:          taskID,
		Status:      "running",
		CreatedAt:   time.Now(),
		FetchSource: req.FetchSource,
	}
	h.taskStore.Store(taskID, task)

	go h.runAnalyze(task, req.ClientID, req.InfoHash, req.Name, req.SavePath, req.Size, req.SourceSite, req.SourceTorrentID, req.MetadataPriority)

	Success(w, map[string]interface{}{"task_id": taskID})
}

// getTorrentSize 从下载器查询种子大小（FetchAndStoreBySearch L2 过滤用，§56.33 决策 A）。
// 查询失败返回 0（SearchAndVerifyMatch 会跳过 size 过滤）。
func (h *ManualForwardHandler) getTorrentSize(clientID uint, infoHash string) int64 {
	if h.clientMgr == nil || infoHash == "" {
		return 0
	}
	dlClient, err := h.clientMgr.Get(strconv.FormatUint(uint64(clientID), 10))
	if err != nil || dlClient == nil {
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	torrent, err := dlClient.GetTorrentByHash(ctx, infoHash)
	if err != nil || torrent == nil {
		return 0
	}
	return torrent.TotalSize
}

func pickNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func (h *ManualForwardHandler) runAnalyze(task *analyzeTask, clientID uint, infoHash, name, savePath string, frontendSize int64, frontendSourceSite, frontendTorrentID, metadataPriority string) {
	defer func() {
		if r := recover(); r != nil {
			task.setError(fmt.Sprintf("分析异常: %v", r))
		}
	}()

	result := map[string]interface{}{
		"name":         name,
		"info_hash":    infoHash,
		"save_path":    savePath,
		"client_id":    clientID,
		"fetch_source": task.FetchSource,
	}

	// ① 禁转预检（标题字符串快速预检）
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

	// ② 源站识别（§56.33 C1：SourceSiteDetector 小组名→站点 + coverage 取 tid）
	sourceSite := frontendSourceSite
	sourceTorrentID := frontendTorrentID
	if h.sourceDetector != nil && infoHash != "" {
		var coverageSites []model.SiteCoverageCache
		if h.coverage != nil {
			coverageSites, _ = h.coverage.GetCachedCoverage(context.Background(), infoHash)
		}
		detected := h.sourceDetector.Detect(context.Background(), name, infoHash, coverageSites)
		if detected.SourceSite != "" {
			if sourceSite == "" {
				sourceSite = detected.SourceSite
				result["source_site_id"] = detected.SourceSiteID
			}
			if sourceTorrentID == "" {
				sourceTorrentID = detected.TorrentID
			}
			result["group_name"] = detected.GroupName
		}
	}
	if sourceSite != "" {
		result["source_site"] = sourceSite
	}
	// fallback：默认第一个 is_source 站（Detect 未命中时）
	if sourceSite == "" {
		var sites []model.Site
		if err := h.db.Where("enabled = ? AND is_source = ?", true, true).Find(&sites).Error; err == nil {
			for _, s := range sites {
				sourceSite = s.Name
				result["source_site"] = sourceSite
				result["source_site_id"] = s.ID
				break
			}
		}
	}

	// ③ exclusions（保留）
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

	// ④ cachedMeta 查询（§56.33 A1：退化为"本地产物缓存"，命中只跳过本地产物生成）
	var cachedMeta model.TorrentMetadata
	hasLocalCache := false
	if infoHash != "" {
		if err := h.db.Where("info_hash = ? AND (media_info != '' OR screenshots != '')", infoHash).First(&cachedMeta).Error; err == nil {
			hasLocalCache = true
		}
	}

	// ⑤ D1 tid 链：torrent_metadata.TorrentID 历史缓存（FetchAndStore 曾写入）
	if sourceTorrentID == "" && sourceSite != "" && h.metadataFetcher != nil && infoHash != "" {
		if meta, ok := h.metadataFetcher.GetMetadata(context.Background(), infoHash, sourceSite); ok && meta != nil && meta.TorrentID != "" {
			sourceTorrentID = meta.TorrentID
		}
	}

	// ⑥ detail 源采集（§56.33 A1：cachedMeta 命中也不跳过；A：无 tid 时 L2 反查兜底）
	var detailFetched bool
	var detailFetchError string
	var detailMeta *model.TorrentMetadata
	var detailWg sync.WaitGroup
	if sourceSite != "" && h.metadataFetcher != nil && infoHash != "" {
		detailWg.Add(1)
		go func() {
			defer detailWg.Done()
			fetchCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			var fetchMeta *model.TorrentMetadata
			var err error
			if sourceTorrentID != "" {
				fetchMeta, err = h.metadataFetcher.FetchAndStore(fetchCtx, infoHash, sourceSite, sourceTorrentID)
			} else {
				// §56.33 决策 A：无 tid → FetchAndStoreBySearch L2 反查
				size := frontendSize
				if size == 0 {
					size = h.getTorrentSize(clientID, infoHash)
				}
				fetchMeta, err = h.metadataFetcher.FetchAndStoreBySearch(fetchCtx, infoHash, sourceSite, name, size)
			}
			if err != nil {
				h.logger.Warn("analyze: detail fetch failed",
					zap.String("site", sourceSite),
					zap.String("torrent_id", sourceTorrentID),
					zap.Error(err))
				detailFetchError = err.Error()
			} else if fetchMeta != nil {
				detailFetched = true
				detailMeta = fetchMeta
				h.logger.Info("analyze: detail fetched",
					zap.String("site", sourceSite),
					zap.Int("detail_json_len", len(fetchMeta.DetailSourceJSON)))
			}
		}()
	}

	// ⑦ PTGen（§56.33 P1：始终跑，数据最新鲜；不耗时）
	var ptgenResult *model.PTGenResult
	if h.pipeline != nil {
		ptgenCtx, ptgenCancel := context.WithTimeout(context.Background(), 60*time.Second)
		if r, err := h.pipeline.AnalyzePTGen(ptgenCtx, name); err == nil && r != nil {
			ptgenResult = r
		}
		ptgenCancel()
	}
	// P1 fallback：PTGen 实时查询空时，用 cachedMeta 历史 PTGen
	if ptgenResult == nil && hasLocalCache && cachedMeta.PTGenSourceJSON != "" {
		if src, err := metadata.UnmarshalPTGenSource(cachedMeta.PTGenSourceJSON); err == nil && src != nil {
			ptgenResult = &src.PTGenResult
		}
	}

	// local 源（§56.33 A1：cachedMeta 命中用缓存本地产物，否则跑 AnalyzeLocalArtifacts）
	var localMediaInfo, localBDInfo string
	var localScreenshots []string
	if hasLocalCache {
		localMediaInfo = cachedMeta.MediaInfo
		if cachedMeta.Screenshots != "" {
			localScreenshots = strings.Split(cachedMeta.Screenshots, "\n")
		}
	} else if h.pipeline != nil && savePath != "" {
		localCtx, localCancel := context.WithTimeout(context.Background(), 3*time.Minute)
		if r, err := h.pipeline.AnalyzeLocalArtifacts(localCtx, name, savePath); err == nil && r != nil {
			if v, ok := r["media_info"].(string); ok {
				localMediaInfo = v
			}
			if v, ok := r["screenshots"].([]string); ok {
				localScreenshots = v
			}
		}
		localCancel()
	}

	// ⑧ BDInfo 扫描（结果融入 local 源的 BDInfo 字段）
	if h.bdinfoScanner != nil && savePath != "" {
		bdinfoCtx, bdinfoCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		bdinfoReport, bdinfoErr := h.bdinfoScanner.ScanIfBD(bdinfoCtx, savePath, name, func(percent int, text string) {
			task.setProgress(percent, text)
		})
		bdinfoCancel()
		if bdinfoErr != nil {
			h.logger.Warn("analyze: BDInfo scan failed", zap.Error(bdinfoErr))
		}
		if bdinfoReport != "" {
			localBDInfo = bdinfoReport
		}
	}

	// ⑨ 等待 detail 采集 + 三源 Merge（§56.33 B1）
	detailWg.Wait()
	if detailFetched {
		result["detail_fetched"] = true
	}
	if detailFetchError != "" {
		result["detail_fetch_error"] = detailFetchError
	}

	// 构造三源（detail 从 detailMeta.DetailSourceJSON 反序列化）
	var detailSrc *metadata.DetailSourceJSON
	if detailMeta != nil && detailMeta.DetailSourceJSON != "" {
		if ds, err := metadata.UnmarshalDetailSource(detailMeta.DetailSourceJSON); err == nil {
			detailSrc = ds
		}
	}
	var ptgenSrc *metadata.PTGenSourceJSON
	if ptgenResult != nil {
		ps := metadata.PTGenToSource(*ptgenResult, time.Now())
		ptgenSrc = &ps
	}
	localSrc := metadata.LocalSourceJSON{
		MediaInfo:   localMediaInfo,
		BDInfo:      localBDInfo,
		Screenshots: localScreenshots,
		GeneratedAt: time.Now(),
	}

	// Merge（DetailFirst：源站详情页 > PTGen；MediaInfo/截图 Local > Detail）
	merged := metadata.Merge(detailSrc, ptgenSrc, &localSrc, metadata.MergeModeDetailFirst)

	// 用 merged 填充 result
	result["title"] = pickNonEmpty(merged.Title, name)
	result["subtitle"] = merged.Subtitle
	result["description"] = merged.Intro.Body
	result["media_info"] = merged.MediaInfo
	shots := merged.Intro.ScreenshotURLs()
	if shots == nil {
		shots = []string{}
	}
	result["screenshots"] = shots
	result["poster_url"] = merged.Intro.Poster
	result["imdb_link"] = merged.IMDbURL
	result["douban_link"] = merged.DoubanURL
	result["tmdb_link"] = merged.TMDbURL
	result["bdinfo"] = merged.BDInfo
	result["source_of"] = merged.SourceOf
	result["cached"] = hasLocalCache
	if len(merged.PTGen.Genre) > 0 {
		result["ptgen_genre"] = strings.Join(merged.PTGen.Genre, ",")
	}
	if merged.PTGen.Episodes != "" {
		result["ptgen_episodes"] = merged.PTGen.Episodes
	}

	// ⑩ 声明过滤（Merge 之后，过滤最终 description）
	if h.declFilter != nil {
		if desc, ok := result["description"].(string); ok && desc != "" {
			patterns := h.declFilter.GetPatterns(context.Background())
			fr := h.declFilter.Filter(desc, patterns)
			result["description"] = fr.CleanedText
			result["removed_declarations"] = fr.RemovedDecls
		}
	}

	// ⑪ 合规检查（detailMeta.Flags 最终判定，保留原逻辑）
	if detailMeta != nil && detailMeta.Flags != "" {
		var detailFlags []string
		if err := json.Unmarshal([]byte(detailMeta.Flags), &detailFlags); err == nil {
			for _, fl := range detailFlags {
				for _, kw := range []string{"禁转", "禁止转载", "谢绝转载", "严禁转载", "谢绝搬运", "独占", "限时禁转"} {
					if fl == kw {
						forbidden = true
						forbidReason = fmt.Sprintf("详情页标记 \"%s\"", kw)
						break
					}
				}
				if forbidden {
					break
				}
			}
			result["detail_flags"] = detailFlags
		}
	}
	result["forbidden"] = forbidden
	result["forbid_reason"] = forbidReason

	// ⑫ 标题解析 + MediaInfo 合并 + 标准化（§56.34 TechProfile 体系）
	mediaInfo := merged.MediaInfo
	effectiveTitle, _ := result["title"].(string)
	if effectiveTitle == "" {
		effectiveTitle = name
	}
	profile := titleparser.ParseTitleTech(effectiveTitle)
	if mediaInfo != "" {
		miTech := titleparser.ExtractMediaInfo(mediaInfo)
		titleparser.MergeMediaInfoInto(&profile, &miTech)
	}
	// DOM 源：媒介/分类 DOM > 标题 + 技术参数 fallback
	titleparser.MergeDOMInto(&profile, merged.Medium, merged.Resolution, merged.VideoCodec, merged.AudioCodec)
	components := titleparser.TechProfileToComponents(profile)
	// 分类推断
	sourceCat := ""
	if h.metadataFetcher != nil && infoHash != "" {
		if meta, ok := h.metadataFetcher.GetMetadata(context.Background(), infoHash, sourceSite); ok && meta != nil {
			if meta.StandardType != "" {
				sourceCat = meta.StandardType
			} else if meta.SourceCategory != "" {
				sourceCat = meta.SourceCategory
			}
		}
	}
	ptgenGenre, _ := result["ptgen_genre"].(string)
	ptgenEpisodes, _ := result["ptgen_episodes"].(string)
	category := titleparser.InferCategory(components, sourceCat, ptgenGenre, ptgenEpisodes)
	stdParams, _ := titleparser.StandardizeTechProfile(profile)
	stdParams.Type = category

	result["title_components"] = components
	result["standardized_params"] = stdParams
	result["tech_profile"] = profile

	task.setResult(result)

	if !hasLocalCache {
		h.persistAnalysis(infoHash, sourceSite, result, profile)
	}
}

func (h *ManualForwardHandler) persistAnalysis(infoHash, siteName string, result map[string]interface{}, profile titleparser.TechProfile) {
	if infoHash == "" {
		return
	}

	fetchSource := "analyze"
	if fs, ok := result["fetch_source"].(string); ok && fs != "" {
		fetchSource = fs
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
	bdinfo, _ := result["bdinfo"].(string)
	now := time.Now()

	// v0.0.255: 构建 MergedJSON（供 merge/preview API 读取，修复架构断裂）
	stdParams, _ := result["standardized_params"].(map[string]interface{})
	mergedJSON := buildMergedJSON(result, title, subtitle, desc, mediaInfo, bdinfo,
		poster, imdb, douban, tmdb, screenshots, stdParams)

	var meta model.TorrentMetadata
	h.db.Where("info_hash = ?", infoHash).First(&meta)

	// §59.19 TechProfile 平铺字段提取
	catStr := ""
	if t, ok := stdParams["type"]; ok {
		catStr, _ = t.(string)
	}
	tpFields := map[string]interface{}{
		"category":       catStr,
		"resolution":     profile.Resolution,
		"video_codec":    profile.VideoCodec,
		"audio_codec":    profile.AudioCodec,
		"audio_channels": profile.AudioChannels,
		"audio_tech":     profile.AudioTechnology,
		"hdr":            profile.HDR,
		"bit_depth":      profile.BitDepth,
		"source_type":    profile.SourceType,
		"specification":  profile.Specification,
		"source_platform": profile.SourcePlatform,
		"edition_info":   profile.EditionInfo,
		"region_code":    profile.RegionCode,
	}

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
			MergedJSON:  mergedJSON,
			FetchSource: fetchSource,
			FetchedAt:   now,
			Category:    catStr,
			Resolution:  profile.Resolution,
			VideoCodec:  profile.VideoCodec,
			AudioCodec:  profile.AudioCodec,
			AudioChannels: profile.AudioChannels,
			AudioTech:   profile.AudioTechnology,
			HDR:         profile.HDR,
			BitDepth:    profile.BitDepth,
			SourceType:  profile.SourceType,
			Specification: profile.Specification,
			SourcePlatform: profile.SourcePlatform,
			EditionInfo: profile.EditionInfo,
			RegionCode:  profile.RegionCode,
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
			"im_db_url":     imdb,
			"douban_url":   douban,
			"tm_db_url":    tmdb,
			"subtitle":     subtitle,
			"merged_json":  mergedJSON,
			"fetch_source": fetchSource,
			"fetched_at":   now,
		}
		for k, v := range tpFields {
			updates[k] = v
		}
		if err := h.db.Model(&meta).Updates(updates).Error; err != nil {
			h.logger.Warn("persist analysis: update failed", zap.Error(err))
		}
	}
}

// buildMergedJSON 从 analyze 结果构建 MergedMetadata JSON（v0.0.255）。
func buildMergedJSON(result map[string]interface{}, title, subtitle, desc, mediaInfo, bdinfo,
	poster, imdb, douban, tmdb, screenshots string, stdParams map[string]interface{}) string {

	var ssList []string
	if screenshots != "" {
		for _, s := range strings.Split(screenshots, "\n") {
			s = strings.TrimSpace(s)
			if s != "" {
				ssList = append(ssList, s)
			}
		}
	}

	merged := map[string]interface{}{
		"title":      title,
		"subtitle":   subtitle,
		"mediainfo":  mediaInfo,
		"bdinfo":     bdinfo,
		"im_db_url":   imdb,
		"douban_url": douban,
		"tm_db_url":  tmdb,
		"tags":       []string{},
		"flags":      []string{},
		"source_of":  map[string]string{},
		"info_hash":  result["info_hash"],
		"intro": map[string]interface{}{
			"body":        desc,
			"poster":      poster,
			"screenshots": ssList,
		},
	}
	if stdParams != nil {
		for k, v := range stdParams {
			merged[k] = v
		}
	}
	data, err := json.Marshal(merged)
	if err != nil {
		return ""
	}
	return string(data)
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
		Anonymous   bool     `json:"anonymous"`
		ScreenshotInDesc *bool `json:"screenshot_in_desc,omitempty"` // §56.27: nil=默认，true/false=用户指定
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

	// 合规检查（成人内容 / 禁转 / 禁转小组）— §56.39: 接入 subtitle（修复 DetectAdult 副标题检测遗漏）
	if h.complianceChecker != nil {
		if r := h.complianceChecker.CheckWithSiteAndSubtitle(r.Context(), req.TorrentName, req.Subtitle, req.SourceSite); !r.Passed {
			Error(w, http.StatusForbidden, 40301, fmt.Sprintf("合规拦截: [%s] %s", r.Category, r.Reason))
			return
		}
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
	// §56.29: 匿名发布
	if req.Anonymous {
		overrides["anonymous"] = true
	}
	// §56.27: ScreenshotInDesc toggle
	if req.ScreenshotInDesc != nil {
		overrides["screenshot_in_desc"] = *req.ScreenshotInDesc
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
			Anonymous   bool     `json:"anonymous"`
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
	var rejected []map[string]interface{}
	for _, item := range req.Items {
		if item.InfoHash == "" || item.ClientID == 0 || len(item.TargetSites) == 0 {
			continue
		}
		// 合规检查（成人内容 / 禁转 / 禁转小组）
		if h.complianceChecker != nil {
			if r := h.complianceChecker.CheckWithSite(r.Context(), item.TorrentName, item.SourceSite); !r.Passed {
				rejected = append(rejected, map[string]interface{}{
					"title":   item.TorrentName,
					"reason":  r.Reason,
					"category": r.Category,
				})
				continue
			}
		}
		targetsJSON, _ := json.Marshal(item.TargetSites)
		// §56.29: 批量发布支持匿名
		batchOverrides := map[string]interface{}{}
		if item.Anonymous {
			batchOverrides["anonymous"] = true
		}
		batchOverridesJSON, _ := json.Marshal(batchOverrides)
		candidate := &model.PublishCandidate{
			SourceSite:        item.SourceSite,
			InfoHash:          item.InfoHash,
			TorrentName:       item.TorrentName,
			ClientID:          fmt.Sprintf("%d", item.ClientID),
			TargetSites:       string(targetsJSON),
			PublishStatus:     model.CandidatePending,
			DownloadCompleted: true,
			Role:              "manual",
			UserOverrides:     string(batchOverridesJSON),
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
		"rejected":      rejected,
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

	// v0.0.255: 优先用 MergedJSON（persistAnalysis 写入），fallback 到三源 JSON
	if meta.MergedJSON != "" {
		result := map[string]interface{}{
			"merged":            json.RawMessage(meta.MergedJSON),
			"has_detail_source": meta.DetailSourceJSON != "",
			"has_ptgen_source":  meta.PTGenSourceJSON != "",
			"has_local_source":  meta.LocalSourceJSON != "",
			"last_merge_mode":   meta.LastMergeMode,
			"source":            "merged_json",
		}
		Success(w, result)
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

// handleRefresh §56.37: 分类型刷新（CrossSeedPanel 的 [重新获取] 按钮）。
// type: poster|screenshots|intro|mediainfo|rehost_screenshots
// 复用 AnalyzePTGen + AnalyzeLocalArtifacts。
func (h *ManualForwardHandler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if h.pipeline == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "pipeline 未配置")
		return
	}
	var req struct {
		Type        string   `json:"type"`
		Name        string   `json:"name"`
		SavePath    string   `json:"save_path"`
		InfoHash    string   `json:"info_hash"`
		SiteName    string   `json:"site_name"`
		Screenshots []string `json:"screenshots"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.Name == "" {
		Error(w, http.StatusBadRequest, 40001, "name 必填")
		return
	}

	ctx := r.Context()
	result := map[string]interface{}{}

	switch req.Type {
	case "poster", "intro":
		ptgen, err := h.pipeline.AnalyzePTGen(ctx, req.Name)
		if err != nil {
			Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("PTGen 失败: %v", err))
			return
		}
		if ptgen != nil {
			if req.Type == "poster" {
				result["poster"] = ptgen.PosterURL
				result["douban_link"] = ptgen.DoubanURL
				result["imdb_link"] = ptgen.IMDBURL
				result["tmdb_link"] = ptgen.TMDbURL
			} else {
				result["description"] = ptgen.RawBBCode
				result["subtitle"] = ptgen.ChineseTitle
			}
		}

	case "mediainfo":
		artifacts, err := h.pipeline.AnalyzeLocalArtifacts(ctx, req.Name, req.SavePath)
		if err != nil {
			Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("MediaInfo 获取失败: %v", err))
			return
		}
		if mi, ok := artifacts["media_info"]; ok {
			result["mediainfo"] = mi
		}

	case "screenshots":
		artifacts, err := h.pipeline.AnalyzeLocalArtifacts(ctx, req.Name, req.SavePath)
		if err != nil {
			Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("截图获取失败: %v", err))
			return
		}
		if ss, ok := artifacts["screenshots"]; ok {
			result["screenshots"] = ss
		}

	case "rehost_screenshots":
		if h.imageHostMgr == nil {
			Error(w, http.StatusServiceUnavailable, 50001, "图床管理器未配置")
			return
		}
		if len(req.Screenshots) == 0 {
			Error(w, http.StatusBadRequest, 40001, "screenshots 必填")
			return
		}
		rehosted := make([]string, 0, len(req.Screenshots))
		for _, url := range req.Screenshots {
			if url == "" {
				continue
			}
			r, err := h.imageHostMgr.Rehost(ctx, url)
			if err != nil || r == nil || r.URL == "" {
				h.logger.Warn("rehost screenshot failed, keeping original",
					zap.String("url", url), zap.Error(err))
				rehosted = append(rehosted, url)
				continue
			}
			rehosted = append(rehosted, r.URL)
		}
		result["screenshots"] = rehosted

	default:
		Error(w, http.StatusBadRequest, 40001, "未知 type: "+req.Type)
		return
	}

	// 如果有 info_hash + site_name，更新 DB
	if req.InfoHash != "" && req.SiteName != "" && len(result) > 0 {
		updates := map[string]interface{}{}
		if v, ok := result["poster"]; ok {
			updates["poster"] = v
		}
		if v, ok := result["description"]; ok {
			updates["description"] = v
		}
		if v, ok := result["mediainfo"]; ok {
			updates["mediainfo"] = v
		}
		if v, ok := result["screenshots"]; ok {
			if arr, ok := v.([]string); ok && len(arr) > 0 {
				jsonBytes, _ := json.Marshal(arr)
				updates["screenshots"] = string(jsonBytes)
			}
		}
		if v, ok := result["subtitle"]; ok {
			updates["subtitle"] = v
		}
		if len(updates) > 0 {
			h.db.WithContext(ctx).
				Model(&model.TorrentMetadata{}).
				Where("info_hash = ? AND site_name = ?", req.InfoHash, req.SiteName).
				Updates(updates)
		}
	}

	result["type"] = req.Type
	Success(w, result)
}

// handleParseTitle §56.40: 标题重新解析（不触发完整 analyze，只解析标题）
func (h *ManualForwardHandler) handleParseTitle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, 40001, "请求格式错误")
		return
	}
	if req.Title == "" {
		Error(w, http.StatusBadRequest, 40001, "title 必填")
		return
	}

	components := titleparser.ParseTitle(req.Title)
	profile := titleparser.ParseTitleTech(req.Title)
	stdParams, _ := titleparser.StandardizeTechProfile(profile)
	tcMap := titleparser.TechProfileToComponents(profile)
	category := titleparser.InferCategory(tcMap, "", "", "")

	Success(w, map[string]interface{}{
		"components":         components,
		"title_components":   tcMap,
		"standardized":       stdParams,
		"category":          category,
	})
}
