package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/compliance"
	"github.com/ranfish/pt-forward/internal/coverage"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"github.com/ranfish/pt-forward/internal/reseed"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type SiteProviderGetter interface {
	GetAdapter(ctx context.Context, domain string) (model.SiteAdapter, error)
	GetSiteConfig(ctx context.Context, domain string) (*model.SiteConfig, error)
}

type PublishTorrentsHandler struct {
	baseCtx               context.Context          // §59.562 刀3: service ctx——后台任务停机信号（main 根 ctx）
	executor              *publish.PublishExecutor // §59.156 切片 2 新发布执行器（复用 pipeline 组件）
	db                    *gorm.DB
	coverage              *coverage.Service
	clientMgr             MFClientProvider
	siteProvider          SiteProviderGetter
	sourceDetector        *publish.SourceSiteDetector
	declFilter            *publish.DeclarationFilter
	reseedEngine          *reseed.Engine
	metadataFetcher       MetadataFetcherProvider
	complianceChecker     *compliance.Checker
	seedPipeline          SeedArtifactAnalyzer
	bdinfoQueue           *publish.BDInfoScanQueue // §59.319 P1: 原盘扫描队列
	bdinfoScanner         *publish.BDInfoScanner   // §59.319 P1: 识别门磁盘侧
	shotStrategy          ScreenshotStrategyRunner
	screenshotCacheDays   int // §59.63: 截图链接缓存观察期（天，<=0 关闭）
	ptgen                 PTGenAnalyzer
	resourceResolver      *publish.ResourceResolver
	preflightCache        map[string]preflightEntry // §59.176: 站点发布前置检查缓存（5 分钟）
	preflightMu           sync.Mutex
	logger                *zap.Logger
	bgState               backgroundQueryState
	batchFetch            batchFetchState
	siteBatch             siteBatchState         // §59.166 一站多种批量任务
	recompute             recomputeState         // §59.514 附八 P2: 重建任务化
	noTransferRecheck     noTransferRecheckState // §59.539 禁转标记复检
	noTransferRecheckSync func()                 // §59.539 配置变更即时重注册 cron（main 注入）
	declBackfill          declBackfillState      // §59.516 v0.0.1461: 简介引用清洗任务
	strategySem           chan struct{}          // §59.58: 截图策略并发额度（批量链挤爆 CPU/代理实测定案）
}

type backgroundQueryState struct {
	mu     sync.Mutex
	active map[uint]bool // clientUID → querying
	total  int
	done   int
}

type batchFetchState struct {
	mu            sync.Mutex
	active        bool
	stopRequested bool // §59.495: graceful 停止——不再开始新颗·当前颗完整执行完
	total         int
	done          int
	failed        int
	items         []batchFetchItem
}

type batchFetchItem struct {
	Hash     string `json:"hash"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	SavePath string `json:"savePath"`
	Status   string `json:"status"` // pending / done / failed
	Error    string `json:"error,omitempty"`
}

// ═══ §59.166 一站多种批量发布任务（N 种×1 站——串行+间隔，轮询进度）═══

type siteBatchState struct {
	mu     sync.Mutex
	tasks  map[string]*siteBatchTask // taskID → task（含已完成待 TTL 清理）
	active map[string]string         // targetSite → 运行中 taskID（同站互斥）
}

type siteBatchTask struct {
	ID           string            `json:"task_id"`
	TargetSite   string            `json:"target_site"`
	Total        int               `json:"total"`
	Done         int               `json:"done"`
	CurrentTitle string            `json:"current_title,omitempty"`
	Results      []siteBatchResult `json:"results"`
	Finished     bool              `json:"finished"`
	Error        string            `json:"error,omitempty"`
	StartedAt    time.Time         `json:"started_at"`
	FinishedAt   time.Time         `json:"finished_at,omitempty"`
	// §59.513: graceful 停止（§59.495 批量获取同款）——stopRequested 由
	// cancel 端点置位；循环检查点（每颗开始前+熔断冷却 select）读取后
	// 当前颗完整执行完再终态（不打断不产半截数据）。
	StopRequested bool `json:"stop_requested,omitempty"`
}

type siteBatchResult struct {
	InfoHash  string `json:"info_hash"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	TorrentID string `json:"torrent_id,omitempty"`
	URL       string `json:"url,omitempty"`
}

func NewPublishTorrentsHandler(ctx context.Context, db *gorm.DB, logger *zap.Logger, pipeline *publish.Pipeline) *PublishTorrentsHandler {
	// §59.58: 容量 5——8 核实测每路 mpv ~6 核，5 路摊薄后单种子 ~145s < 4min ctx（60% 余量）。
	// CPU 总量守恒：并发数不改变批量总时长，只消除无界并发导致的 ctx 超时作废功。
	// §59.51 手动截图按钮不经过此信号量（独立单例，不与批量竞争）。
	if ctx == nil { // 附录A §2.5 边界防御
		ctx = context.Background()
	}
	return &PublishTorrentsHandler{
		baseCtx:             ctx,
		db:                  db,
		logger:              logger,
		executor:            publish.NewPublishExecutor(pipeline),
		bgState:             backgroundQueryState{active: make(map[uint]bool)},
		siteBatch:           siteBatchState{tasks: map[string]*siteBatchTask{}, active: map[string]string{}},
		resourceResolver:    publish.NewResourceResolver(db),
		strategySem:         make(chan struct{}, 5),
		screenshotCacheDays: 30, // §59.63: 观察期默认 30 天（SetScreenshotCacheDays 由 settings 覆盖）
	}
}

// SetBDInfoQueue §59.319 P1: 注入原盘扫描队列+识别器；完成回调（落库+簇传播+
// 规格重算）在此注册（装配期一次性）。
func (h *PublishTorrentsHandler) SetBDInfoQueue(q *publish.BDInfoScanQueue, s *publish.BDInfoScanner) {
	h.bdinfoQueue = q
	h.bdinfoScanner = s
	if q != nil {
		q.SetOnDone(h.onBDInfoScanDone)
	}
}

// clusterHashesDB 簇内全部 hash（client_uid+save_path+name 紧键——与
// propagateCluster* 同源；§59.319 P2 提包级供 manual refresh 链复用）。
func clusterHashesDB(db *gorm.DB, ctx context.Context, clientUID uint, savePath, name string) []string {
	if clientUID == 0 || savePath == "" || name == "" {
		return nil
	}
	var hashes []string
	db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
		Where("client_uid = ? AND save_path = ? AND name = ? AND is_hidden = 0",
			clientUID, savePath, name).
		Pluck("hash", &hashes)
	return hashes
}

// clusterHasBDInfoDB 簇查重：簇内任一行已有 bd_info 产物即无需重扫
// （§59.319 D4 簇级缓存——59-74 站同盘共享一次扫描）。
func clusterHasBDInfoDB(db *gorm.DB, ctx context.Context, clientUID uint, savePath, name string) bool {
	hashes := clusterHashesDB(db, ctx, clientUID, savePath, name)
	if len(hashes) == 0 {
		return false
	}
	var cnt int64
	db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("info_hash IN ? AND bd_info != ''", hashes).
		Limit(1).Count(&cnt)
	return cnt > 0
}

func (h *PublishTorrentsHandler) clusterHashes(ctx context.Context, clientUID uint, savePath, name string) []string {
	return clusterHashesDB(h.db, ctx, clientUID, savePath, name)
}

// clusterHasBDInfo 簇查重薄壳。
func (h *PublishTorrentsHandler) clusterHasBDInfo(ctx context.Context, clientUID uint, savePath, name string) bool {
	return clusterHasBDInfoDB(h.db, ctx, clientUID, savePath, name)
}

// onBDInfoScanDone §59.319 P1: 扫描完成回调（queue 串行线程执行）——
// 簇级产物落库（全簇行 bd_info 覆盖写）+ 伪 MI 清除（D8）+ 规格列重算
// （BDInfo 报告经 ExtractMediaInfo 格式分流自动解析；簇同名同 title
// 算一次写全簇）。
func (h *PublishTorrentsHandler) onBDInfoScanDone(task publish.BDInfoScanTask, report string, err error) {
	// §59.568 刀1: 本地任务类：BDInfo 后处理 5min——磁盘扫描后处理·失败=下轮重扫
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	hashes := h.clusterHashes(ctx, task.ClientUID, task.SavePath, task.Name)
	if err != nil {
		h.logger.Warn("bdinfo scan done with error",
			zap.String("name", task.Name), zap.Error(err))
		// §59.361 A: 失败落记录（簇级）——列表三态判定（失败≠扫描中），
		// 防推断式 disc_scan_pending 永转蓝圈骗用户（pt29 案：失败 24h+
		// 仍显示扫描中）
		if len(hashes) > 0 {
			now := time.Now()
			if uerr := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
				Where("info_hash IN ?", hashes).
				Updates(map[string]interface{}{
					"bd_scan_failed_at": now,
					"bd_scan_error":     err.Error(),
				}).Error; uerr != nil {
				h.logger.Warn("bdinfo scan failure record write failed",
					zap.String("name", task.Name), zap.Error(uerr))
			}
		}
		return
	}
	if report == "" {
		return
	}
	if len(hashes) == 0 {
		h.logger.Warn("bdinfo scan done but cluster empty", zap.String("name", task.Name))
		return
	}

	updates := map[string]interface{}{
		"bd_info":           report,
		"media_info":        "", // §59.319 D8: 原盘伪 MI（单 m2ts BDAV）清除
		"media_info_source": "",
		// §59.361 A: 成功清失败记录
		"bd_scan_failed_at": nil,
		"bd_scan_error":     "",
	}

	// 规格列重算：代表行取 title/DOM 源（簇行同名同标题——一次计算全簇复用）
	var rep model.TorrentMetadata
	if err := h.db.WithContext(ctx).Where("info_hash IN ?", hashes).First(&rep).Error; err == nil && rep.Title != "" {
		domMedium, domRes, domVideo, domAudio := titleparser.DOMFieldsFromDetailSource(rep.DetailSourceJSON)
		profile := titleparser.BuildTechProfile(rep.Title, report, domMedium, domRes, domVideo, domAudio)
		updates["resolution"] = profile.Resolution
		updates["video_codec"] = profile.VideoCodec
		updates["audio_codec"] = profile.AudioCodec
		updates["audio_channels"] = profile.AudioChannels
		updates["audio_tech"] = profile.AudioTechnology
		updates["audio_tracks"] = profile.AudioTracks
		updates["hdr"] = profile.HDR
		updates["bit_depth"] = profile.BitDepth
		updates["source_type"] = profile.SourceType
		updates["specification"] = profile.Specification
	}

	res := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("info_hash IN ?", hashes).
		Updates(updates)
	if res.Error != nil {
		h.logger.Error("bdinfo cluster persist failed",
			zap.String("name", task.Name), zap.Error(res.Error))
		return
	}
	h.logger.Info("bdinfo cluster persisted",
		zap.String("name", task.Name),
		zap.Int("cluster_rows", len(hashes)),
		zap.Int64("rows_affected", res.RowsAffected))

	// §59.319 附十七: 扫描落库后标签重推——清除重获场景 fetch 期推断时
	// BDInfo 空（高码/高帧数值判据缺失）；NFO 通道现在有数据。InferFull
	// 算一次（簇内输入同构），MergeTags 逐行（各行 existing 可能不同），
	// 直采优先+互斥仲裁语义既有（源站显式/用户编辑不覆盖，只增不删）
	// §59.319 附二十九: rep 是 DB 更新前读的（BDInfo=旧值/空）——
	// reinferTagsForCluster 需要新报告，直接传入 report 参数
	h.reinferTagsForCluster(ctx, task, rep, report, hashes)
}

// reinferTagsForCluster §59.319 附十七: 簇级标签重推（onDone 后置）。
func (h *PublishTorrentsHandler) reinferTagsForCluster(ctx context.Context, task publish.BDInfoScanTask, rep model.TorrentMetadata, bdinfoReport string, hashes []string) {
	if rep.Title == "" {
		return
	}
	var snapSize int64
	h.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
		Where("hash = ?", rep.InfoHash).Select("COALESCE(MAX(size),0)").Scan(&snapSize)
	// §59.319 附二十九: NFO 用扫描产出报告（rep 是更新前快照，BDInfo 恒空）
	// §59.500 刀3 MI 素材融合 a：同 inferFinalTags 单点（local→source）
	miForReinfer := rep.MediaInfo
	if miForReinfer == "" {
		miForReinfer = rep.SourceMediaInfo
	}
	inferred := publish.NewMediaTagInferer().InferFull(publish.TagInput{
		MediaInfo:   miForReinfer,
		Title:       rep.Title,
		Subtitle:    rep.Subtitle,
		Description: rep.Description,
		NFO:         bdinfoReport,
		Statement:   rep.Statement,
		Region:      rep.RegionCode,
		Size:        snapSize,
	})
	if len(inferred) == 0 {
		return
	}
	var rows []model.TorrentMetadata
	if err := h.db.WithContext(ctx).Where("info_hash IN ?", hashes).Find(&rows).Error; err != nil {
		return
	}
	updated := 0
	for i := range rows {
		// §59.500 刀1: 行级完整性闸门——不完整不推理（与发布门同语义）
		if missing := h.checkRequiredFields(&rows[i], detailSourceHasMusic(rows[i].DetailSourceJSON)); len(missing) > 0 {
			continue
		}
		var existing []string
		if rows[i].Tags != "" {
			if err := json.Unmarshal([]byte(rows[i].Tags), &existing); err != nil {
				existing = nil
			}
		}
		// §59.496 附二: existing 过铁证门（同 t0/t2 单点）
		all := publish.MergeTags(publish.CleanExistingForMerge(existing, rows[i].MediaInfo), inferred)
		data, _ := json.Marshal(all)
		if string(data) == rows[i].Tags {
			continue
		}
		if err := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
			Where("info_hash = ?", rows[i].InfoHash).
			Update("tags", string(data)).Error; err == nil {
			updated++
		}
	}
	if updated > 0 {
		h.logger.Info("bdinfo cluster tags reinferred",
			zap.String("name", task.Name),
			zap.Int("rows", updated),
			zap.Int("inferred", len(inferred)))
	}
}

// StartObservingCleanup §59.38: 观察期定时清理——日级扫描超 7 天的观察组并
// 执行两级清理（立即清理与定时共用 purgeObservingResource）。启动 +5min 避开
// 启动 IO 高峰，此后每 24h 一轮。
// §59.563 刀3: 接 baseCtx 停机通道（审计 02 §3.3——原 Background+无 stop
// 的永久 goroutine）：避峰等待与 ticker 轮均挂 Done；清理幂等（超 7 天扫描
// +删文件·下次重扫）→ 幂等型语义（停机中止可重查）。
func (h *PublishTorrentsHandler) StartObservingCleanup() {
	go func() {
		select {
		case <-time.After(5 * time.Minute):
		case <-h.baseCtx.Done():
			return
		}
		h.runObservingCleanup(h.baseCtx)
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-h.baseCtx.Done():
				return
			case <-ticker.C:
				h.runObservingCleanup(h.baseCtx)
			}
		}
	}()
}

func (h *PublishTorrentsHandler) runObservingCleanup(ctx context.Context) {
	cutoff := time.Now().Add(-observingGracePeriod)
	type obsGroup struct {
		ClientUID uint
		Name      string
	}
	var groups []obsGroup
	h.db.WithContext(ctx).
		Table("torrent_snapshots").
		Select("client_uid, name").
		Where("name != '' AND last_seen < ?", cutoff).
		Group("client_uid, name").
		Having("SUM(CASE WHEN is_hidden = 0 THEN 1 ELSE 0 END) = 0 AND SUM(CASE WHEN is_hidden = 1 THEN 1 ELSE 0 END) > 0").
		Find(&groups)
	if len(groups) == 0 {
		return
	}
	var totalSnaps, totalMetas int64
	for _, g := range groups {
		res := h.purgeObservingResource(ctx, g.ClientUID, g.Name)
		totalSnaps += res.snapRows
		totalMetas += res.metaRows
	}
	h.logger.Info("observing cleanup completed",
		zap.Int("groups", len(groups)),
		zap.Int64("snaps", totalSnaps),
		zap.Int64("metas", totalMetas))
}

func (h *PublishTorrentsHandler) SetCoverageService(s *coverage.Service) { h.coverage = s }

func (h *PublishTorrentsHandler) SetReseedEngine(e *reseed.Engine) { h.reseedEngine = e }

func (h *PublishTorrentsHandler) SetClientProvider(c MFClientProvider) { h.clientMgr = c }

func (h *PublishTorrentsHandler) SetSiteProvider(s SiteProviderGetter) { h.siteProvider = s }

func (h *PublishTorrentsHandler) SetSourceDetector(d *publish.SourceSiteDetector) {
	h.sourceDetector = d
}

func (h *PublishTorrentsHandler) SetDeclarationFilter(f *publish.DeclarationFilter) { h.declFilter = f }

func (h *PublishTorrentsHandler) SetMetadataFetcher(f MetadataFetcherProvider) { h.metadataFetcher = f }

func (h *PublishTorrentsHandler) SetComplianceChecker(c *compliance.Checker) { h.complianceChecker = c }

func (h *PublishTorrentsHandler) SetSeedPipeline(p SeedArtifactAnalyzer) { h.seedPipeline = p }

func (h *PublishTorrentsHandler) SetScreenshotStrategyRunner(p ScreenshotStrategyRunner) {
	h.shotStrategy = p
}

// SetScreenshotCacheDays §59.63: 截图链接缓存观察期（天）。<=0 关闭。
func (h *PublishTorrentsHandler) SetScreenshotCacheDays(days int) { h.screenshotCacheDays = days }

func (h *PublishTorrentsHandler) SetPTGenAnalyzer(p PTGenAnalyzer) { h.ptgen = p }

// §59.21: 本地产物分析接口（只跑 mediainfo，不跑截图）
type SeedArtifactAnalyzer interface {
	AnalyzeLocalArtifacts(ctx context.Context, name, savePath string) (map[string]interface{}, error)
}

// §59.53: 采集链截图策略（pipeline 实现）——auto 全策略/远程只转存
type ScreenshotStrategyRunner interface {
	ApplyScreenshotStrategy(ctx context.Context, name, savePath string, sourceScreenshots []string, isLocal bool, forceLocal bool, forcedSid int) []string
}

// PTGenAnalyzer PTGen 查询接口（§59.42 海报 fallback 链用）
type PTGenAnalyzer interface {
	AnalyzePTGen(ctx context.Context, name string) (*model.PTGenResult, error)
	// §59.236 ①: 主链 Force 通道（单条重获绕缓存——§59.173 语义）
	AnalyzePTGenForce(ctx context.Context, name string) (*model.PTGenResult, error)
}

func (h *PublishTorrentsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(r.URL.Path, "/")

	switch {
	case strings.HasSuffix(path, "/publish/torrents") && r.Method == http.MethodGet:
		h.handleListTorrents(w, r)
	case strings.HasSuffix(path, "/publish/torrents/batch-publish") && r.Method == http.MethodPost:
		h.handleBatchPublish(w, r)
	case strings.HasSuffix(path, "/publish/torrents/coverage") && r.Method == http.MethodPost:
		h.handleQueryCoverage(w, r)
	case strings.HasSuffix(path, "/publish/torrents/batch-coverage") && r.Method == http.MethodPost:
		h.handleBatchQueryCoverage(w, r)
	case strings.HasSuffix(path, "/publish/torrents/query-status") && r.Method == http.MethodGet:
		h.handleQueryStatus(w, r)
	case strings.HasSuffix(path, "/publish/torrents/detect-source") && r.Method == http.MethodPost:
		h.handleDetectSource(w, r)
	case strings.HasSuffix(path, "/publish/torrents/group-mappings") && r.Method == http.MethodGet:
		h.handleListGroupMappings(w, r)
	case strings.HasSuffix(path, "/publish/torrents/group-mappings/sites") && r.Method == http.MethodGet:
		h.handleListGroupedSiteNames(w, r)
	case strings.HasSuffix(path, "/publish/torrents/declaration-filters") && r.Method == http.MethodGet:
		h.handleGetDeclarationFilters(w, r)
	case strings.HasSuffix(path, "/publish/torrents/declaration-filters") && r.Method == http.MethodPut:
		h.handleSetDeclarationFilters(w, r)
	case strings.HasSuffix(path, "/publish/torrents/preview-title") && r.Method == http.MethodPost:
		h.handlePreviewTitle(w, r)
	case strings.HasSuffix(path, "/publish/torrents/preview-title-batch") && r.Method == http.MethodPost:
		h.handlePreviewTitleBatch(w, r)
	case strings.HasSuffix(path, "/publish/torrents/group-mappings") && r.Method == http.MethodPost:
		h.handleCreateGroupMapping(w, r)
	case strings.Contains(path, "/publish/torrents/group-mappings/") && r.Method == http.MethodPut:
		h.handleUpdateGroupMapping(w, r)
	case strings.Contains(path, "/publish/torrents/group-mappings/") && r.Method == http.MethodDelete:
		h.handleDeleteGroupMapping(w, r)
	case strings.HasSuffix(path, "/publish/seed-data") && r.Method == http.MethodGet:
		h.handleListSeedData(w, r)
	case strings.HasSuffix(path, "/publish/seed-data/batch-review") && r.Method == http.MethodPost:
		h.handleBatchReview(w, r)
	case strings.HasSuffix(path, "/publish/seed-data/batch-delete") && r.Method == http.MethodPost:
		h.handleBatchDelete(w, r)
	case strings.Contains(path, "/publish/seed-data/") && r.Method == http.MethodPut:
		h.handleSaveSeedData(w, r)
	case strings.HasSuffix(path, "/publish/stats") && r.Method == http.MethodGet:
		h.handleStats(w, r)
	case strings.HasSuffix(path, "/publish/coverage-cache") && r.Method == http.MethodGet:
		h.handleCoverageCache(w, r)
	case strings.HasSuffix(path, "/publish/source-priority") && r.Method == http.MethodGet:
		h.handleGetSourcePriority(w, r)
	case strings.HasSuffix(path, "/publish/source-priority") && r.Method == http.MethodPut:
		h.handleSetSourcePriority(w, r)
	case strings.HasSuffix(path, "/publish/fetch-priority") && r.Method == http.MethodGet:
		h.handleGetFetchPriority(w, r)
	case strings.HasSuffix(path, "/publish/fetch-priority") && r.Method == http.MethodPut:
		h.handleSetFetchPriority(w, r)
	case strings.HasSuffix(path, "/publish/seeds/audit-infohash") && r.Method == http.MethodPost:
		h.handleAuditInfoHash(w, r)
	// §59.539: 禁转标记复检——手动触发/状态/配置三端点
	case strings.HasSuffix(path, "/publish/seeds/no-transfer-recheck/run") && r.Method == http.MethodPost:
		h.handleNoTransferRecheckRun(w, r)
	case strings.HasSuffix(path, "/publish/seeds/no-transfer-recheck/status") && r.Method == http.MethodGet:
		h.handleNoTransferRecheckStatus(w, r)
	case strings.HasSuffix(path, "/publish/seeds/no-transfer-recheck/config") && r.Method == http.MethodPut:
		h.handleNoTransferRecheckConfigSave(w, r)
	case strings.HasSuffix(path, "/publish/seeds/recompute-profiles-task") && r.Method == http.MethodPost:
		h.handleRecomputeProfilesTask(w, r)
	case strings.HasSuffix(path, "/publish/seeds/recompute-progress") && r.Method == http.MethodGet:
		h.handleRecomputeProgress(w, r)
	case strings.HasSuffix(path, "/publish/seeds/recompute-cancel") && r.Method == http.MethodPost:
		h.handleRecomputeCancel(w, r)
	case strings.HasSuffix(path, "/publish/seeds/declaration-backfill-task") && r.Method == http.MethodPost:
		h.handleDeclBackfillTask(w, r)
	case strings.HasSuffix(path, "/publish/seeds/declaration-backfill-progress") && r.Method == http.MethodGet:
		h.handleDeclBackfillProgress(w, r)
	case strings.HasSuffix(path, "/publish/seeds/declaration-backfill-cancel") && r.Method == http.MethodPost:
		h.handleDeclBackfillCancel(w, r)
	case strings.HasSuffix(path, "/publish/seeds/fix-source-description") && r.Method == http.MethodPost:
		h.handleFixSourceDescription(w, r)
	case strings.HasSuffix(path, "/publish/seeds/fix-stale-dolby") && r.Method == http.MethodPost:
		h.handleFixStaleDolbyTags(w, r)
	case strings.HasSuffix(path, "/publish/bdinfo/backfill") && r.Method == http.MethodPost:
		h.handleBDInfoBackfill(w, r)
	case strings.HasSuffix(path, "/publish/bdinfo/rescan") && r.Method == http.MethodPost:
		h.handleBDInfoRescan(w, r) // §59.361 A: 单颗重扫（失败态入口）
	case strings.HasSuffix(path, "/publish/bdinfo/queue") && r.Method == http.MethodGet:
		h.handleBDInfoQueueStatus(w, r)
	case strings.HasSuffix(path, "/publish/seeds/execute") && r.Method == http.MethodPost:
		h.handleExecutePublish(w, r)
	case strings.HasSuffix(path, "/publish/seeds/execute-batch") && r.Method == http.MethodPost:
		h.handleExecutePublishBatch(w, r)
	case strings.HasSuffix(path, "/publish/seeds/batch-repush") && r.Method == http.MethodPost:
		h.handleBatchRepush(w, r)

	case strings.HasSuffix(path, "/publish/seeds/execute-site-batch") && r.Method == http.MethodPost:
		h.handleExecuteSiteBatch(w, r)
	case strings.HasSuffix(path, "/publish/seeds/site-batch-progress") && r.Method == http.MethodGet:
		h.handleSiteBatchProgress(w, r)
	case strings.HasSuffix(path, "/publish/seeds/site-batch-cancel") && r.Method == http.MethodPost:
		h.handleSiteBatchCancel(w, r)

	case strings.HasSuffix(path, "/publish/seeds/batch-fetch") && r.Method == http.MethodPost:
		h.handleBatchFetch(w, r)
	case strings.HasSuffix(path, "/publish/seeds/batch-fetch-progress") && r.Method == http.MethodGet:
		h.handleBatchFetchProgress(w, r)
	case strings.HasSuffix(path, "/publish/seeds/batch-fetch-cancel") && r.Method == http.MethodPost:
		h.handleBatchFetchCancel(w, r)
	case strings.HasSuffix(path, "/publish/seeds") && r.Method == http.MethodGet:
		h.handleListSeeds(w, r)
	case strings.HasSuffix(path, "/publish/seeds/unique-paths") && r.Method == http.MethodGet:
		h.handleSeedUniquePaths(w, r)
	case strings.HasSuffix(path, "/publish/seeds/observing/purge") && r.Method == http.MethodPost:
		h.handlePurgeObserving(w, r)
	case strings.HasSuffix(path, "/publish/seeds/covered-sites") && r.Method == http.MethodGet:
		h.handleCoveredSites(w, r)
	case strings.Contains(path, "/publish/seeds/") && strings.HasSuffix(path, "/fetch") && r.Method == http.MethodPost:
		h.handleFetchSingleSeed(w, r)
	case strings.Contains(path, "/publish/seeds/") && r.Method == http.MethodGet:
		h.handleGetSeed(w, r)
	case strings.Contains(path, "/publish/seeds/") && r.Method == http.MethodPut:
		h.handlePutSeed(w, r)
	case strings.Contains(path, "/publish/seeds/") && r.Method == http.MethodDelete:
		h.handleDeleteSeed(w, r)
	default:
		Error(w, http.StatusNotFound, 40400, "接口不存在")
	}
}
