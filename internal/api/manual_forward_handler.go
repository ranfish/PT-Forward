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

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ManualForwardHandler struct {
	db        *gorm.DB
	logger    *zap.Logger
	pipeline  PublishPipeline
	siteMgr   SiteManager
	clientMgr MFClientProvider
	taskStore sync.Map
	taskSeq   atomic.Int64
}

type PublishPipeline interface {
	PublishCandidateFromManual(ctx context.Context, candidate *model.PublishCandidate) error
}

type SiteManager interface {
	ListSites(ctx context.Context) ([]model.Site, error)
	GetSiteConfig(ctx context.Context, siteURL string) (*model.SiteConfig, error)
}

type MFClientProvider interface {
	Get(clientID string) (model.DownloaderClient, error)
}

func NewManualForwardHandler(db *gorm.DB, logger *zap.Logger) *ManualForwardHandler {
	return &ManualForwardHandler{
		db:     db,
		logger: logger,
	}
}

func (h *ManualForwardHandler) SetPipeline(p PublishPipeline)        { h.pipeline = p }
func (h *ManualForwardHandler) SetSiteManager(s SiteManager)         { h.siteMgr = s }
func (h *ManualForwardHandler) SetClientProvider(c MFClientProvider) { h.clientMgr = c }

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
	case strings.HasSuffix(path, "/manual-forward/analyze/"):
		h.handlePollAnalyze(w, r)
	case strings.HasSuffix(path, "/manual-forward/eligible-targets"):
		if r.Method == http.MethodPost {
			h.handleEligibleTargets(w, r)
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

func (h *ManualForwardHandler) handleSeededTorrents(w http.ResponseWriter, r *http.Request) {
	if h.clientMgr == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "客户端管理器未初始化")
		return
	}

	clientIDStr := r.URL.Query().Get("client_id")
	if clientIDStr == "" {
		var clients []model.ClientConfig
		h.db.Where("enabled = ?", true).Find(&clients)
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

	client, err := h.clientMgr.Get(fmt.Sprintf("%d", clientID))
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
	mu        sync.RWMutex           `json:"-"`
	ID        int64                  `json:"id"`
	Status    string                 `json:"status"`
	Error     string                 `json:"error,omitempty"`
	Result    map[string]interface{} `json:"result,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

func (t *analyzeTask) setError(err string) {
	t.mu.Lock()
	t.Error = err
	t.Status = "failed"
	t.mu.Unlock()
}

func (t *analyzeTask) setResult(result map[string]interface{}) {
	t.mu.Lock()
	t.Result = result
	t.Status = "completed"
	t.mu.Unlock()
}

func (t *analyzeTask) snapshot() *analyzeTask {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return &analyzeTask{
		ID:        t.ID,
		Status:    t.Status,
		Error:     t.Error,
		Result:    t.Result,
		CreatedAt: t.CreatedAt,
	}
}

func (h *ManualForwardHandler) handleStartAnalyze(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientID uint   `json:"client_id"`
		InfoHash string `json:"info_hash"`
		Name     string `json:"name"`
		SavePath string `json:"save_path"`
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

	go h.runAnalyze(task, req.ClientID, req.InfoHash, req.Name, req.SavePath)

	Success(w, map[string]interface{}{"task_id": taskID})
}

func (h *ManualForwardHandler) runAnalyze(task *analyzeTask, clientID uint, infoHash, name, savePath string) {
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
	h.db.Where("enabled = ? AND is_source = ?", true, true).Find(&sites)
	sourceSite := ""
	for _, s := range sites {
		result["source_site"] = s.Name
		result["source_site_id"] = s.ID
		sourceSite = s.Name
		break
	}

	var exclusions []model.PublishExclusion
	h.db.Find(&exclusions)
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
	result["description"] = ""
	result["media_info"] = ""
	result["screenshots"] = []string{}
	result["poster_url"] = ""

	task.setResult(result)
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
	h.db.Where("enabled = ? AND is_target = ?", true, true).Find(&sites)

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
	candidate := &model.PublishCandidate{
		SourceSite:        req.SourceSite,
		InfoHash:          req.InfoHash,
		TorrentName:       req.TorrentName,
		ClientID:          fmt.Sprintf("%d", req.ClientID),
		TargetSites:       string(targetsJSON),
		PublishStatus:     model.CandidatePending,
		DownloadCompleted: true,
		Role:              "manual",
	}

	if err := h.db.Create(candidate).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("创建候选失败: %v", err))
		return
	}

	Success(w, map[string]interface{}{
		"candidate_id": candidate.ID,
		"status":       "created",
	})
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
		ids = append(ids, candidate.ID)
	}

	Success(w, map[string]interface{}{
		"created_count": len(ids),
		"candidate_ids": ids,
	})
}
