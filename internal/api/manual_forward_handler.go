package api

import (
	"sync"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/compliance"
	"github.com/ranfish/pt-forward/internal/imagehost"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ManualForwardHandler struct {
	db            *gorm.DB
	logger        *zap.Logger
	pipeline      PublishPipeline
	capture       screenshotCaptureState // §59.51 后台截图任务（全局单例）
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
	screenshotCacheDays int // §59.63: 手动捕获结果写穿缓存（观察期与自动链同配）
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
	FetchAndStoreBySearch(ctx context.Context, infoHash, siteName, torrentName string, size int64, sourceLocalMI ...string) (*model.TorrentMetadata, error)
	FetchAndStoreDirect(ctx context.Context, infoHash, siteName, torrentID, sourceName string) (*model.TorrentMetadata, error) // §59.61 D3
	FetchFromSiteNoFallback(ctx context.Context, infoHash, siteName, torrentID string) (*model.TorrentMetadata, error)        // §59.65 直取
	FetchAndStoreIYUU(ctx context.Context, infoHash, excludeSite string) (*model.TorrentMetadata, error)                      // §59.65 IYUU 末位兜底
}

type PublishPipeline interface {
	AnalyzeTorrent(ctx context.Context, name, savePath string) (map[string]interface{}, error)
	AnalyzePTGen(ctx context.Context, name string) (*model.PTGenResult, error)
	AnalyzeLocalArtifacts(ctx context.Context, name, savePath string) (map[string]interface{}, error)
	CaptureScreenshots(ctx context.Context, name, savePath string, sourceScreenshots []string) []string
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
	}
	return h
}

// SetScreenshotCacheDays §59.63: 手动捕获写穿缓存的观察期（天）。
func (h *ManualForwardHandler) SetScreenshotCacheDays(days int) { h.screenshotCacheDays = days }

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

func (h *ManualForwardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(r.URL.Path, "/")
	switch {
	case strings.HasSuffix(path, "/manual-forward/screenshot-capture") && r.Method == http.MethodPost:
		h.handleScreenshotCaptureStart(w, r)
	case strings.HasSuffix(path, "/manual-forward/screenshot-capture-progress") && r.Method == http.MethodGet:
		h.handleScreenshotCaptureProgress(w, r)
	case strings.HasSuffix(path, "/manual-forward/refresh"):
		if r.Method == http.MethodPost {
			h.handleRefresh(w, r)
		} else {
			Error(w, http.StatusMethodNotAllowed, 40001, "方法不允许")
		}
	default:
		Error(w, http.StatusNotFound, 40400, "接口不存在")
	}
}

func (h *ManualForwardHandler) refreshFromSource(ctx context.Context, infoHash, siteName, torrentName string) (*model.TorrentMetadata, error) {
	// §59.44: 资源视图解析——传入 hash 可能是列表保留行（无 tid），按资源键圈
	// hash 后查任一挂载行的 tid，避免不必要的搜索反查（挂载行有 tid 直接重抓）
	var hashes []string
	var row model.TorrentSnapshot
	if err := h.db.WithContext(ctx).
		Where("hash = ? AND is_hidden = ?", infoHash, false).
		First(&row).Error; err == nil && row.Name != "" {
		h.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
			Where("client_id = ? AND save_path = ? AND name = ? AND is_hidden = ?",
				row.ClientID, row.SavePath, row.Name, false).
			Pluck("hash", &hashes)
	}
	candidates := []string{infoHash}
	if len(hashes) > 0 {
		candidates = hashes
	}
	var existing model.TorrentMetadata
	h.db.WithContext(ctx).
		Where("info_hash IN ? AND site_name = ? AND torrent_id != ''", candidates, siteName).
		Order("updated_at DESC").
		First(&existing)
	if existing.TorrentID != "" {
		return h.metadataFetcher.FetchAndStore(ctx, existing.InfoHash, siteName, existing.TorrentID)
	}
	return h.metadataFetcher.FetchAndStoreBySearch(ctx, infoHash, siteName, torrentName, 0)
}

func (h *ManualForwardHandler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if h.pipeline == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "pipeline 未配置")
		return
	}
	var req struct {
		Type        string   `json:"type"`
		Name        string   `json:"name"`
		SavePath    string   `json:"savePath"`
		InfoHash    string   `json:"infoHash"`
		SiteName    string   `json:"siteName"`
		Screenshots []string `json:"screenshots"`
		ClientID    string   `json:"clientId"` // §59.21: 查 is_local
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

	// §59.21: 查下载器 is_local
	isLocal := true // 默认 true（向后兼容：无 client_id 时走本地路径）
	if req.ClientID != "" {
		var client model.ClientConfig
		if err := h.db.WithContext(ctx).Where("name = ?", req.ClientID).First(&client).Error; err == nil {
			isLocal = client.IsLocal
		}
	}

	switch req.Type {
	case "poster", "intro":
		// §59.42 延伸: PTGen query 优先级——DB douban_url > imdb_url > 种子名。
		// 现配 endpoint（doubaninfo/cspt）只接受资源 URL，种子名必然失败
		//（用户报"重新获取海报报 无法解析资源URL"根因）；与 applyPosterFallback 同款语义。
		query := h.resolvePTGenQuery(ctx, req.InfoHash, req.SiteName, req.Name)
		ptgen, err := h.pipeline.AnalyzePTGen(ctx, query)
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
			// §59.20: 不再返回 subtitle——副标题来自源站，PTGen 不覆盖
			}
		}

	case "mediainfo":
		if !isLocal {
			// §59.21: 转种上盒——从源站重新抓取
			// §59.36 审计: tid 链——DB 已有 tid 直接用；无 tid（元数据缺失场景）
			// 走 FetchAndStoreBySearch 反查（v0.0.557 原实现传空 tid 必然失败，
			// 维护模式屏蔽按钮期间无人踩到）
			if req.InfoHash != "" && req.SiteName != "" && h.metadataFetcher != nil {
				meta, err := h.refreshFromSource(ctx, req.InfoHash, req.SiteName, req.Name)
				if err != nil {
					Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("源站重新获取失败: %v", err))
					return
				}
				if meta != nil {
					// §59.36: 远程场景返回源站列——meta.MediaInfo 是本地列（远程下载器
					// 恒空），空则 fallback SourceMediaInfo（重抓源站得到的正是它）。
					// v0.0.557 is_local 改造遗漏。
					mi := meta.MediaInfo
					if mi == "" {
						mi = meta.SourceMediaInfo
					}
					if mi != "" {
						result["mediainfo"] = mi
					}
					if meta.BDInfo != "" {
						result["bdinfo"] = meta.BDInfo
					}
				}
			}
			break
		}
		artifacts, err := h.pipeline.AnalyzeLocalArtifacts(ctx, req.Name, req.SavePath)
		if err != nil {
			Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("MediaInfo 获取失败: %v", err))
			return
		}
		if mi, ok := artifacts["media_info"]; ok {
			result["mediainfo"] = mi
			// §59.171 C: 本地 MI 持久化源行 + 簇传播——原实现只返回表单，
			// 而 PUT 请求体无 mediainfo 字段，手动获取的 MI 无落库通道
			// （PT31 实锤：Tab5 获取"成功"仅表单内存，DB 恒空）。与手动截图
			// §59.169 行为对齐。
			if miStr, isStr := mi.(string); isStr && miStr != "" && req.InfoHash != "" && h.db != nil {
				q := h.db.Model(&model.TorrentMetadata{}).Where("info_hash = ?", req.InfoHash)
				if req.SiteName != "" {
					q = q.Where("site_name = ?", req.SiteName)
				}
				if err := q.Updates(map[string]interface{}{
					"media_info":        miStr,
					"media_info_source": "local",
				}).Error; err != nil {
					h.logger.Warn("refresh mediainfo persist failed",
						zap.String("hash", req.InfoHash[:min(10, len(req.InfoHash))]), zap.Error(err))
				} else {
					propagateClusterMediainfoDB(h.db, h.logger, ctx, req.ClientID, req.SavePath, req.Name, req.InfoHash, miStr)
				}
			}
		}
		// §59.20: 同时扫描 BDInfo（如有蓝光结构）
		if h.bdinfoScanner != nil && req.SavePath != "" {
			bdCtx, bdCancel := context.WithTimeout(ctx, 2*time.Minute)
			bdReport, bdErr := h.bdinfoScanner.ScanIfBD(bdCtx, req.SavePath, req.Name, nil)
			bdCancel()
			if bdErr != nil {
				h.logger.Warn("refresh: BDInfo scan failed", zap.Error(bdErr))
			}
			if bdReport != "" {
				result["bdinfo"] = bdReport
			}
		}

	case "screenshots":
		if !isLocal {
			// §59.21: 转种上盒——从源站重新抓取（§59.36 审计: tid 链同 mediainfo）
			if req.InfoHash != "" && req.SiteName != "" && h.metadataFetcher != nil {
				meta, err := h.refreshFromSource(ctx, req.InfoHash, req.SiteName, req.Name)
				if err != nil {
					Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("源站重新获取失败: %v", err))
					return
				}
				if meta != nil && len(model.ParseScreenshotColumn(meta.Screenshots)) > 0 {
					result["screenshots"] = model.ParseScreenshotColumn(meta.Screenshots)
				}
			}
			break
		}
		// §59.51: 本地 mpv 截图已迁移后台任务（POST /manual-forward/screenshot-capture +
		// 轮询 progress）——长任务绑 HTTP 请求生命周期会被前端超时取消（§59.50 审计）。
		// 本分支保留兼容旧客户端（同步退化行为），新前端按 seedIsLocal 分流不再调此路径。
		// §59.50: 本地 mpv 截图（local_upload 策略）——原 AnalyzeLocalArtifacts 是
		// MI 场景的 source_direct（跳过截图恒返回空），mpv/字幕检测/HDR/图床上传
		// 能力从未接到本按钮。源站截图作 fallback（本地全失败时回源站值）。
		capCtx, capCancel := context.WithTimeout(ctx, 5*time.Minute)
		defer capCancel()
		shots := h.pipeline.CaptureScreenshots(capCtx, req.Name, req.SavePath, nil)
		if len(shots) > 0 {
			result["screenshots"] = shots
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
	// §59.36 审计: updates 键必须是真实 DB 列名（mediainfo → media_info 拼写修复，
	// v0.0.557 起该列名错误导致 MI 重获从未持久化；且行不存在时静默 0 行——
	// 元数据缺失场景先建行再更新）
	if req.InfoHash != "" && req.SiteName != "" && len(result) > 0 {
		updates := map[string]interface{}{}
		if v, ok := result["poster"]; ok {
			updates["poster"] = v
		}
		if v, ok := result["description"]; ok {
			updates["description"] = v
		}
		if v, ok := result["mediainfo"]; ok {
			updates["media_info"] = v // DB 列名（torrent_metadata.media_info）
			if isLocal {
				updates["media_info_source"] = "local"
			}
		}
		if v, ok := result["bdinfo"]; ok {
			updates["bd_info"] = v
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
			// 行不存在（unfetched 种子直接点重获）→ 先建骨架行再更新
			var cnt int64
			h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
				Where("info_hash = ? AND site_name = ?", req.InfoHash, req.SiteName).
				Count(&cnt)
			if cnt == 0 {
				h.db.WithContext(ctx).Create(&model.TorrentMetadata{
					InfoHash: req.InfoHash,
					SiteName: req.SiteName,
					Title:    req.Name,
				})
			}
			h.db.WithContext(ctx).
				Model(&model.TorrentMetadata{}).
				Where("info_hash = ? AND site_name = ?", req.InfoHash, req.SiteName).
				Updates(updates)
		}
	}

	result["type"] = req.Type
	Success(w, result)
}

func (h *ManualForwardHandler) resolvePTGenQuery(ctx context.Context, infoHash, siteName, fallbackName string) string {
	if infoHash != "" {
		var m model.TorrentMetadata
		q := h.db.WithContext(ctx).
			Where("info_hash = ?", infoHash).
			Order("updated_at DESC")
		if siteName != "" {
			q = q.Where("site_name = ?", siteName)
		}
		if err := q.First(&m).Error; err == nil {
			if m.DoubanURL != "" {
				return m.DoubanURL
			}
			if m.IMDbURL != "" {
				return m.IMDbURL
			}
		}
	}
	return fallbackName
}

// screenshotCaptureState §59.51: 后台截图任务全局单例状态（内存态，batch-fetch 同款）。
type screenshotCaptureState struct {
	mu         sync.Mutex
	active     bool
	status     string // running / done / failed
	name       string // 发起时的种子名（前端会话一致性校验）
	screenshots []string
	error      string
}

// handleScreenshotCaptureStart §59.51: 启动后台 mpv 截图任务。
// is_local=true 专用（守卫拒绝非本地下载器）；全局单例；结果落库。
func (h *ManualForwardHandler) handleScreenshotCaptureStart(w http.ResponseWriter, r *http.Request) {
	if h.pipeline == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "pipeline 未配置")
		return
	}
	var req struct {
		Name     string `json:"name"`
		SavePath string `json:"savePath"`
		ClientID string `json:"clientId"`
		InfoHash string `json:"infoHash"`
		SiteName string `json:"siteName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.SavePath == "" || req.ClientID == "" {
		Error(w, http.StatusBadRequest, 40001, "name/savePath/clientId 必填")
		return
	}

	// §59.51 遗漏2: is_local 守卫——本端点只服务本地下载器
	var client model.ClientConfig
	if err := h.db.WithContext(r.Context()).Where("name = ?", req.ClientID).First(&client).Error; err != nil || !client.IsLocal {
		Error(w, http.StatusBadRequest, 40001, "该下载器非本地（is_local=false 请走源站重获）")
		return
	}

	// 全局单例
	h.capture.mu.Lock()
	if h.capture.active {
		busy := h.capture.name
		h.capture.mu.Unlock()
		Error(w, http.StatusConflict, 40901, "截图任务进行中: "+busy)
		return
	}
	h.capture.active = true
	h.capture.status = "running"
	h.capture.name = req.Name
	h.capture.screenshots = nil
	h.capture.error = ""
	h.capture.mu.Unlock()

	go func() {
		// §59.51 审计: panic 防护——CaptureScreenshots panic 会让 active 永久 true（单例死锁），
		// batch-fetch runBatchFetch defer recover 同款
		defer func() {
			if p := recover(); p != nil {
				h.capture.mu.Lock()
				h.capture.active = false
				h.capture.status = "failed"
				h.capture.error = fmt.Sprintf("internal panic: %v", p)
				h.capture.mu.Unlock()
			}
		}()
		// §59.51: 脱离 HTTP 请求生命周期（§59.50 审计根因）——Background + 5min
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		shots := h.pipeline.CaptureScreenshots(ctx, req.Name, req.SavePath, nil)

		h.capture.mu.Lock()
		defer h.capture.mu.Unlock()
		h.capture.active = false
		h.capture.screenshots = shots
		if len(shots) > 0 {
			h.capture.status = "done"
			// §59.63 Q4: 手动捕获行为不变（每次全新截传），结果写穿缓存——
			// 缓存语义 = 簇最新已知好链接，供下轮批量直接复用
			upsertClusterScreenshotCache(h.db, h.logger, h.screenshotCacheDays, req.ClientID, req.SavePath, req.Name, shots)
		} else {
			h.capture.status = "failed"
			h.capture.error = "mpv 截图失败（本地无可用视频文件或上传全失败）"
		}

		// 落库（事实源：编辑器重新打开从 detail 读到）
		if len(shots) > 0 && req.InfoHash != "" && h.db != nil {
			data, _ := json.Marshal(shots)
			q := h.db.Model(&model.TorrentMetadata{}).Where("info_hash = ?", req.InfoHash)
			if req.SiteName != "" {
				q = q.Where("site_name = ?", req.SiteName)
			}
			if err := q.Update("screenshots", string(data)).Error; err != nil {
				h.logger.Warn("screenshot capture persist failed",
					zap.String("hash", req.InfoHash[:min(10, len(req.InfoHash))]), zap.Error(err))
			}
			// §59.169: 手动截图簇传播——与策略路径（screenshot cache hit 分支）对齐，
			// 补簇内空截图行。海王2 45 副本死循环根因：手动只写单行。
			propagateClusterScreenshotsDB(h.db, h.logger, ctx, req.ClientID, req.SavePath, req.Name, req.InfoHash, string(data))
		}
	}()

	Success(w, map[string]interface{}{"started": true})
}

// handleScreenshotCaptureProgress §59.51: 轮询任务状态。
func (h *ManualForwardHandler) handleScreenshotCaptureProgress(w http.ResponseWriter, r *http.Request) {
	h.capture.mu.Lock()
	defer h.capture.mu.Unlock()
	Success(w, map[string]interface{}{
		"active":      h.capture.active,
		"status":      h.capture.status,
		"name":        h.capture.name,
		"screenshots": h.capture.screenshots,
		"error":       h.capture.error,
	})
}

func pickNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func pickNonZero(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}
