package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/metadata"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"github.com/ranfish/pt-forward/internal/site"
	"go.uber.org/zap"
)

func (h *PublishTorrentsHandler) handleExecutePublish(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InfoHash     string   `json:"info_hash"`
		TargetSite   string   `json:"target_site"`
		Anonymous    bool     `json:"anonymous"`
		TagOverrides []string `json:"tag_overrides"`
		DryRun       bool     `json:"dry_run"`
		PushOnly     bool     `json:"push_only"`
		TorrentID    string   `json:"torrent_id"`
		PushClientID uint     `json:"push_client_id"`
		PushSavePath string   `json:"push_save_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// DryRun 400 案教训：解码失败曾被"必填"文案掩盖（push_client_id 空串入 uint）
		Error(w, http.StatusBadRequest, 40002, "请求体解码失败: "+err.Error())
		return
	}
	if req.InfoHash == "" || req.TargetSite == "" {
		Error(w, http.StatusBadRequest, 40001, "info_hash 与 target_site 必填")
		return
	}
	if h.executor == nil {
		Error(w, http.StatusServiceUnavailable, 50301, "执行器未初始化")
		return
	}
	// §59.156 回归审核：发布长流程（渲染+预检+上传+加种）脱离 HTTP 请求生命周期——
	// §59.51 教训（243 实测上传 context canceled）；独立 Background+10min 上限
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	result := h.executor.Execute(ctx, publish.ExecuteInput{
		InfoHash:     req.InfoHash,
		TargetSite:   req.TargetSite,
		Anonymous:    req.Anonymous,
		TagOverrides: req.TagOverrides,
		DryRun:       req.DryRun,
		PushOnly:     req.PushOnly,
		TorrentID:    req.TorrentID,
		PushClientID: req.PushClientID,
		PushSavePath: req.PushSavePath,
	})
	// §59.158: Success 信封（与 form-config 同款教训——裸写致前端取值 undefined）
	Success(w, map[string]any{"result": result})
}

// handleExecutePublishBatch §59.159: 一种多站批量发布——每站独立 execute 并行
// （BatchGroupID 分组落库；网络白名单内并发安全——PTGen 等在线依赖已移除）。

// preflightEntry §59.176: 前置检查缓存项。
type preflightEntry struct {
	result  *model.PublishPreflightResult
	expires time.Time
}

// preflightSite §59.176: 站点发布前置检查（公共方法——消费点：
// 一种多站/一站多种/单发三个入口）。5 分钟缓存（串行发布同站只拉一次；
// 禁发状态分钟级不会翻转）。未实现 Preflighter 的 adapter 返回 nil（放行——
// 不阻塞未适配站）。
func (h *PublishTorrentsHandler) preflightSite(ctx context.Context, siteName string) *model.PublishPreflightResult {
	if h.siteProvider == nil {
		return nil
	}
	h.preflightMu.Lock()
	if h.preflightCache == nil {
		h.preflightCache = make(map[string]preflightEntry)
	}
	if e, ok := h.preflightCache[siteName]; ok && time.Now().Before(e.expires) {
		h.preflightMu.Unlock()
		return e.result
	}
	h.preflightMu.Unlock()

	adapter, err := h.siteProvider.GetAdapter(ctx, siteName)
	if err != nil {
		return nil
	}
	pf, ok := adapter.(model.PublishPreflighter)
	if !ok {
		return nil
	}
	cfg, err := h.siteProvider.GetSiteConfig(ctx, siteName)
	if err != nil || cfg == nil {
		return nil
	}
	result, err := pf.PreflightPublish(ctx, cfg)
	if err != nil {
		h.logger.Warn("preflight publish failed（放行）",
			zap.String("site", siteName), zap.Error(err))
		return nil // 检查失败不阻塞发布（fail-open——上传链自会暴露问题）
	}
	h.preflightMu.Lock()
	h.preflightCache[siteName] = preflightEntry{result: result, expires: time.Now().Add(5 * time.Minute)}
	h.preflightMu.Unlock()
	return result
}

// preflightBlockMessage §59.176: 统一引导文案。
func preflightBlockMessage(reason string) string {
	return reason + "——请登录站点打开发布页查看处理（如修改被拒种子后权限自动恢复/检查用户组权限/重新登录）"
}

func (h *PublishTorrentsHandler) handleExecutePublishBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InfoHash     string   `json:"info_hash"`
		TargetSites  []string `json:"target_sites"`
		Anonymous    bool     `json:"anonymous"`
		TagOverrides []string `json:"tag_overrides"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.InfoHash == "" || len(req.TargetSites) == 0 {
		Error(w, http.StatusBadRequest, 40001, "info_hash 与 target_sites 必填")
		return
	}
	if h.executor == nil {
		Error(w, http.StatusServiceUnavailable, 50301, "执行器未初始化")
		return
	}
	batchID := fmt.Sprintf("%d", time.Now().UnixNano())
	type siteResult struct {
		Site      string                  `json:"site"`
		Status    string                  `json:"status"`
		Message   string                  `json:"message"`
		TorrentID string                  `json:"torrent_id,omitempty"`
		URL       string                  `json:"url,omitempty"`
		PreAudit  *publish.PreAuditResult `json:"pre_audit,omitempty"`
	}
	results := make([]siteResult, len(req.TargetSites))
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	for i, site := range req.TargetSites {
		wg.Add(1)
		go func(idx int, siteName string) {
			defer wg.Done()
			defer func() {
				if rec := recover(); rec != nil {
					results[idx] = siteResult{Site: siteName, Status: "failed", Message: fmt.Sprintf("panic: %v", rec)}
				}
			}()
			// §59.176: 站点发布前置检查——不可发布仅跳过该站（隔离语义：
			// 不阻塞可发布站），提示+日志
			if pf := h.preflightSite(ctx, siteName); pf != nil && !pf.Allowed {
				h.logger.Warn("preflight blocked site",
					zap.String("site", siteName), zap.String("reason", pf.Reason))
				results[idx] = siteResult{Site: siteName, Status: "preflight_blocked", Message: preflightBlockMessage(pf.Reason)}
				return
			}
			res := h.executor.Execute(ctx, publish.ExecuteInput{
				InfoHash:     req.InfoHash,
				TargetSite:   siteName,
				Anonymous:    req.Anonymous,
				TagOverrides: req.TagOverrides,
				BatchGroupID: batchID,
			})
			sr := siteResult{Site: siteName, Status: res.Status, Message: res.Message}
			if res.Upload != nil {
				sr.TorrentID = res.Upload.TorrentID
			}
			sr.URL = res.TargetTorrentURL
			sr.PreAudit = res.PreAudit
			results[idx] = sr
		}(i, site)
	}
	wg.Wait()
	Success(w, map[string]any{"batch_id": batchID, "results": results})
}

// handleExecuteSiteBatch §59.166 一站多种批量发布（N 种×1 站）：
// 串行+种间间隔（sites.publish_interval_seconds，clamp 1-60）——NP 站连续上传
// 反作弊风险，节奏拟人（用户定案）；单种失败不中断批次；统一 BatchGroupID。
// 任务化：立即返回 taskId，进度走 site-batch-progress 轮询（断线无伤）。
func (h *PublishTorrentsHandler) handleExecuteSiteBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InfoHashes []string `json:"infoHashes"`
		TargetSite string   `json:"targetSite"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if len(req.InfoHashes) == 0 || req.TargetSite == "" {
		Error(w, http.StatusBadRequest, 40001, "infoHashes 与 targetSite 必填")
		return
	}
	if len(req.InfoHashes) > 100 {
		Error(w, http.StatusBadRequest, 40002, "单批最多 100 个种子（流控防护）")
		return
	}
	if h.executor == nil {
		Error(w, http.StatusServiceUnavailable, 50301, "执行器未初始化")
		return
	}

	// §59.535 刀6: 覆盖判据 matcher——在 var site 局部变量遮蔽包名前构建
	covTM := site.NewTrackerMatcher(h.db)

	// 目标站校验：存在 + 发布配置启用
	var site model.Site
	if err := h.db.WithContext(r.Context()).Where("name = ?", req.TargetSite).First(&site).Error; err != nil {
		Error(w, http.StatusBadRequest, 40003, "目标站不存在: "+req.TargetSite)
		return
	}
	cfg := model.ParseFormConfig(site.PublishFormConfig)
	if cfg == nil || !cfg.Enabled {
		Error(w, http.StatusBadRequest, 40004, "目标站未启用发布配置: "+req.TargetSite)
		return
	}
	// §59.176: 批首发布前置检查——不可发布整批快速失败（92 次无效上传教训）
	if pf := h.preflightSite(r.Context(), req.TargetSite); pf != nil && !pf.Allowed {
		h.logger.Warn("site-batch preflight blocked",
			zap.String("site", req.TargetSite), zap.String("reason", pf.Reason))
		Error(w, http.StatusBadRequest, 40005, preflightBlockMessage(pf.Reason))
		return
	}
	interval := site.PublishIntervalSeconds
	if interval < 1 {
		interval = 1
	}
	if interval > 60 {
		interval = 60
	}

	// 同站互斥（运行中任务存在→拒）
	taskID := fmt.Sprintf("%d", time.Now().UnixNano())
	h.siteBatch.mu.Lock()
	if runID, running := h.siteBatch.active[req.TargetSite]; running {
		msg := "该站已有批量任务运行中，请等待完成"
		// §59.166 互斥提示带进度（用户定案 A 方案——信息透明）
		if t := h.siteBatch.tasks[runID]; t != nil {
			msg = fmt.Sprintf("该站已有批量任务运行中（%d/%d），请等待完成", t.Done, t.Total)
		}
		h.siteBatch.mu.Unlock()
		Error(w, http.StatusConflict, 40901, msg)
		return
	}
	task := &siteBatchTask{
		ID: taskID, TargetSite: req.TargetSite, Total: len(req.InfoHashes),
		Results:   make([]siteBatchResult, 0, len(req.InfoHashes)),
		StartedAt: time.Now(),
	}
	h.siteBatch.tasks[taskID] = task
	h.siteBatch.active[req.TargetSite] = taskID
	h.siteBatch.mu.Unlock()

	// §59.51 长任务铁律：脱离 HTTP 生命周期；动态超时 total×(60s+间隔) 封顶 24h
	budget := time.Duration(len(req.InfoHashes)) * (60*time.Second + time.Duration(interval)*time.Second)
	// §59.166 熔断重试余量：最坏 5 轮 × 35s 冷却/种
	budget += 30 * time.Minute
	if budget > 24*time.Hour {
		budget = 24 * time.Hour
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), budget)
		defer cancel()
		batchGroupID := taskID
		for i, hash := range req.InfoHashes {
			// §59.563 刀4: 服务停机检查点（baseCtx.Done 与 StopRequested 同
			// 语义位——batch fetch §59.562 刀3 同款：不再开始新颗·当前颗完整
			// 执行；颗内 ctx 保持 Background 派生不被停机中断——发布是站侧
			// 事实动作·半途取消不如完整落终态）。
			select {
			case <-h.baseCtx.Done():
				h.finishSiteBatch(task, "服务停机（已完成 "+fmt.Sprint(task.Done)+"/"+fmt.Sprint(task.Total)+"）")
				return
			default:
			}
			// §59.513: graceful 停止检查点①——每颗开始前（取消后当前颗已
			// 完整执行，未开始颗不再开始；§59.495 批量获取同款语义）。
			h.siteBatch.mu.Lock()
			stopped := task.StopRequested
			h.siteBatch.mu.Unlock()
			if stopped {
				h.finishSiteBatch(task, "用户取消（已完成 "+fmt.Sprint(task.Done)+"/"+fmt.Sprint(task.Total)+"）")
				return
			}
			if i > 0 {
				select {
				case <-ctx.Done():
					h.finishSiteBatch(task, "任务超时中断（已完成 "+fmt.Sprint(task.Done)+"/"+fmt.Sprint(task.Total)+"）")
					return
				case <-time.After(time.Duration(interval) * time.Second):
				}
			}
			// 当前种标题（进度显示——§59.316 附 A: 簇口径解析，失败留空）
			curTitle := h.siteBatchCurTitle(ctx, hash)
			h.siteBatch.mu.Lock()
			task.CurrentTitle = curTitle
			h.siteBatch.mu.Unlock()

			// §59.535 刀6 执行层 3b: 逐 (簇,站) 对覆盖过滤——簇已有该站→跳过
			// （不发请求不烧站方配额·记「站上已有·跳过」终态）。多选 UI 不过
			// 滤的兜底（N 对 N 在 UI 层无解·执行层逐格独立判断）。判据失败保
			// 守放行（发布链自有 dedup 三路防线）。
			if covered := publish.CoveredSitesOfCluster(ctx, h.db, covTM, hash); covered[req.TargetSite] {
				h.logger.Info("site-batch: 簇已有该站·跳过（覆盖过滤 §59.535 刀6）",
					zap.String("hash", hash), zap.String("site", req.TargetSite))
				h.siteBatch.mu.Lock()
				task.Results = append(task.Results, siteBatchResult{
					InfoHash: hash, Title: curTitle,
					Status: "skipped", Message: "站上已有·跳过（覆盖判据）",
				})
				task.Done++
				h.siteBatch.mu.Unlock()
				continue
			}

			// §59.166 回归审核补：per-seed recover——executor panic 若穿透会把任务
			// goroutine 炸掉（finishSiteBatch 永不调用 → 同站互斥永久锁死直到重启）。
			// recover 后按单种失败继续（批量语义）。
			execOnce := func() *publish.ExecuteResult {
				defer func() {
					if rec := recover(); rec != nil {
						h.logger.Error("site-batch executor panic",
							zap.String("site", req.TargetSite), zap.Any("panic", rec))
					}
				}()
				return h.executor.Execute(ctx, publish.ExecuteInput{
					InfoHash:     hash,
					TargetSite:   req.TargetSite,
					BatchGroupID: batchGroupID, // 匿名走站点级 form_config.Anonymous（§59.166 单源）
				})
			}
			// §59.166 韧性终案（用户确认）：熔断感知重试——circuit open=请求未发出
			// （熔断器发送前拒绝），重试零重复上传风险；冷却等待后重试（最多 5 轮），
			// 仍熔断则记 failed 继续。超时类错误绝不自动重试（请求可能已达站方，
			// 重试有重复上传风险——§59.159 上传判定二元化边界）。
			const circuitCooldown = 35 * time.Second
			const maxCircuitRetries = 5
			res := execOnce()
			if res == nil {
				res = &publish.ExecuteResult{Status: "failed", Message: "执行器 panic（已隔离，继续后续种子）"}
			}
			for attempt := 1; attempt <= maxCircuitRetries; attempt++ {
				if res == nil || res.Status != "failed" || !strings.Contains(res.Message, "circuit open") {
					break
				}
				h.siteBatch.mu.Lock()
				task.CurrentTitle = curTitle + fmt.Sprintf("（熔断冷却 %d/%d）", attempt, maxCircuitRetries)
				h.siteBatch.mu.Unlock()
				select {
				case <-ctx.Done():
					h.finishSiteBatch(task, "任务超时中断（已完成 "+fmt.Sprint(task.Done)+"/"+fmt.Sprint(task.Total)+"）")
					return
				case <-time.After(circuitCooldown):
				}
				// §59.513: graceful 停止检查点②——熔断冷却醒来后（等待期
				// 最多 35s·站点故障场景用户取消应尽快生效：当前颗熔断态
				// 本就未发出请求，停止无半截风险）。
				h.siteBatch.mu.Lock()
				stopped2 := task.StopRequested
				h.siteBatch.mu.Unlock()
				if stopped2 {
					h.finishSiteBatch(task, "用户取消（熔断冷却中·已完成 "+fmt.Sprint(task.Done)+"/"+fmt.Sprint(task.Total)+"）")
					return
				}
				h.logger.Info("site-batch 熔断重试",
					zap.String("site", req.TargetSite), zap.Int("attempt", attempt),
					zap.String("hash", hash))
				res = execOnce()
				if res == nil {
					res = &publish.ExecuteResult{Status: "failed", Message: "执行器 panic（已隔离，继续后续种子）"}
				}
			}
			if res != nil && res.Status == "failed" && strings.Contains(res.Message, "circuit open") {
				res.Message = "站点熔断持续打开（已等待 5 轮冷却）: " + res.Message
			}
			sr := siteBatchResult{InfoHash: hash, Title: curTitle, Status: res.Status, Message: res.Message}
			if res.Upload != nil {
				sr.TorrentID = res.Upload.TorrentID
			}
			sr.URL = res.TargetTorrentURL
			h.siteBatch.mu.Lock()
			task.Results = append(task.Results, sr)
			task.Done++
			task.CurrentTitle = ""
			h.siteBatch.mu.Unlock()
		}
		h.finishSiteBatch(task, "")
	}()

	Success(w, map[string]any{"task_id": taskID, "total": len(req.InfoHashes), "interval_seconds": interval})
}

// siteBatchCurTitle §59.316 附 A: 批量发布进度标题——簇口径解析（ResourceResolver
// 紧键 (client,path,name)+hidden 兜底）。原直查 info_hash 在簇代表换站后失配→
// 漂移簇进度标题恒空（243 八连案同族残留）。resolver 未注入（测试）回退直查。
func (h *PublishTorrentsHandler) siteBatchCurTitle(ctx context.Context, hash string) string {
	if h.resourceResolver != nil {
		if rv := h.resourceResolver.ResolveResource(ctx, hash); rv != nil && rv.Meta != nil {
			return rv.Meta.Title
		}
		return ""
	}
	var m model.TorrentMetadata
	if err := h.db.WithContext(ctx).Select("title").
		Where("info_hash = ?", hash).First(&m).Error; err == nil {
		return m.Title
	}
	return ""
}

// finishSiteBatch 收尾（解锁同站互斥+终态时间戳）。
func (h *PublishTorrentsHandler) finishSiteBatch(task *siteBatchTask, errMsg string) {
	h.siteBatch.mu.Lock()
	defer h.siteBatch.mu.Unlock()
	task.Finished = true
	task.Error = errMsg
	task.FinishedAt = time.Now()
	if h.siteBatch.active[task.TargetSite] == task.ID {
		delete(h.siteBatch.active, task.TargetSite)
	}
}

// handleSiteBatchCancel §59.513: 批量发布 graceful 停止（§59.495 批量
// 获取同款——用户报 Case：网络故障/站点故障时批量发布无法取消只能干等
// 全部失败）。幂等三态：无任务 no-op / 运行中置 stopRequested（当前颗
// 完整执行完后终态·循环检查点读标志）/ 已完成 no-op。同站互斥由
// finishSiteBatch 正常释放（不立即删 active 防新批插入语义与
// §59.495 一致——当前颗完成前该站仍锁）。
func (h *PublishTorrentsHandler) handleSiteBatchCancel(w http.ResponseWriter, r *http.Request) {
	site := r.URL.Query().Get("site")
	if site == "" {
		Error(w, http.StatusBadRequest, 40001, "site 必填")
		return
	}
	h.siteBatch.mu.Lock()
	runID, running := h.siteBatch.active[site]
	if running {
		if t := h.siteBatch.tasks[runID]; t != nil {
			t.StopRequested = true
			h.siteBatch.mu.Unlock()
			Success(w, map[string]interface{}{
				"message": "停止请求已接受——当前种子完成后终止（已完成 " + fmt.Sprint(t.Done) + "/" + fmt.Sprint(t.Total) + "）",
				"task_id": runID,
			})
			return
		}
	}
	h.siteBatch.mu.Unlock()
	Success(w, map[string]interface{}{"message": "无运行中任务（已停止或已完成）"})
}

// handleSiteBatchProgress §59.166 批量任务进度轮询：
// ?task_id= 指定任务；?site=X&active=1 查该站运行中任务（页面挂载恢复——断线无伤）。
// 惰性 TTL 清理：完成任务 30 分钟后移除（发布日志有全量，内存态只保近期）。
func (h *PublishTorrentsHandler) handleSiteBatchProgress(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	h.siteBatch.mu.Lock()
	defer h.siteBatch.mu.Unlock()

	// TTL 清理（30 分钟）
	now := time.Now()
	for id, t := range h.siteBatch.tasks {
		if t.Finished && now.Sub(t.FinishedAt) > 30*time.Minute {
			delete(h.siteBatch.tasks, id)
		}
	}

	if q.Get("active") == "1" {
		siteName := q.Get("site")
		if siteName != "" {
			// 指定站查询（原口径）
			if id, ok := h.siteBatch.active[siteName]; ok {
				Success(w, h.siteBatch.tasks[id])
				return
			}
			Success(w, nil) // 无运行中任务
			return
		}
		// §59.166 终稿回归审核补：不带 site → 全部活跃任务（选站不持久化的
		// 前提下，页面刷新后恢复进度需要无锚查询——active map 直接列出）
		actives := make([]*siteBatchTask, 0, len(h.siteBatch.active))
		for _, id := range h.siteBatch.active {
			if t, ok := h.siteBatch.tasks[id]; ok {
				actives = append(actives, t)
			}
		}
		sort.Slice(actives, func(i, j int) bool { return actives[i].StartedAt.Before(actives[j].StartedAt) })
		if len(actives) == 0 {
			Success(w, nil)
			return
		}
		Success(w, actives)
		return
	}
	id := q.Get("task_id")
	if t, ok := h.siteBatch.tasks[id]; ok {
		Success(w, t)
		return
	}
	Error(w, http.StatusNotFound, 40401, "任务不存在或已过期（30 分钟 TTL）")
}

// ptgenViable §59.341 ②（方案 a）: PTGen 可行性判定——获取站数据质量的
// 隐式门（详情页有 douban/imdb 键或 title 可搜中）。簇凭证/IYUU 循环内
// "获取成功但 PTGen 不可行"→ 不 break 换下一候选（From S01 案：PT地带
// 获取成功但发布者未填豆瓣链接 → incomplete 陷阱）。PTGen 有缓存——同
// 键重复判定近零成本（同资源不同站键高度趋同）。
// h.ptgen == nil（组件未装配）时恒 true（不过滤）。
// filterStatementForFetch §59.542 刀3 补: 获取链声明过滤（DeclFilter 回调
// 的 handler 侧实现——词表与发布链 3105 同源 GetPatterns·declFilter 未装配
// 时原样返回）。
func (h *PublishTorrentsHandler) filterStatementForFetch(ctx context.Context, statement string) string {
	if h.declFilter == nil || statement == "" {
		return statement
	}
	patterns := h.declFilter.GetPatterns(ctx)
	return h.declFilter.Filter(statement, patterns).CleanedText
}

func (h *PublishTorrentsHandler) ptgenViable(ctx context.Context, meta *model.TorrentMetadata) bool {
	if h.ptgen == nil || meta == nil {
		return true
	}
	// §59.490: 音乐种恒可行——PTGen 是影视质量门语义：音乐种 douban/imdb
	// 恒空+title 纯音乐大多搜不中 → 误判"获取站数据质量差"换下一站白烧
	// （School Rumble 案双面坑：搜不中→换站白烧配额；搜得中→不换站但被
	// runMainlinePTGen 污染）。音乐数据质量由 music 字段在场性判定。
	if metadata.HasMusic(meta.DetailSourceJSON) {
		return true
	}
	keys := mainlineQueryKeys(meta.DoubanURL, meta.IMDbURL, meta.Title)
	if len(keys) == 0 {
		return false
	}
	for _, k := range keys {
		r, err := h.ptgen.AnalyzePTGen(ctx, k)
		if err == nil && r != nil && r.RawBBCode != "" {
			return true
		}
	}
	return false
}

// handleBatchRepush §59.374 B2: 批量补推——按记录行原配置重放 PushOnly
// （ids → 查记录 → executor.PushOnly 并发限 2 → 汇总）。幂等安全
// （下载器自动校验——§59.374 B1 同理）。
func (h *PublishTorrentsHandler) handleBatchRepush(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []uint `json:"ids"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if len(req.IDs) == 0 {
		Error(w, http.StatusBadRequest, 40001, "ids 必填")
		return
	}
	if len(req.IDs) > 100 {
		Error(w, http.StatusBadRequest, 40001, "单次最多 100 条")
		return
	}
	if h.executor == nil {
		Error(w, http.StatusServiceUnavailable, 50301, "执行器未初始化")
		return
	}

	var records []model.PublishResultRecord
	// §59.509 附六: 判据对齐前端 canRepush 事实判据（tid 在手且本地未做种
	// ——§59.509 前端勾选已改·后端此处漏跟：duplicate 行（seed_error 空）
	// 被旧判据"加种失败态"全滤→「无可补推记录」误拒 714 行可补推实况）。
	// downloader_id/save_path 同查=PushOnly 重放的必要执行条件。
	if err := h.db.WithContext(r.Context()).
		Where("id IN ? AND torrent_id != '' AND torrent_id != '0' AND seeded = 0 AND downloader_id != 0 AND save_path != ''", req.IDs).
		Find(&records).Error; err != nil || len(records) == 0 {
		Error(w, http.StatusNotFound, 40400, "无可补推记录（需 tid 在手且本地未做种——站上已有·可补推/已上传未做种均可）")
		return
	}

	type repushOutcome struct {
		ID     uint   `json:"id"`
		Status string `json:"status"` // pushed / failed
		Error  string `json:"error,omitempty"`
	}
	outcomes := make([]repushOutcome, len(records))
	sem := make(chan struct{}, 2)
	var wg sync.WaitGroup
	var mu sync.Mutex
	// §59.374 回归修复：Background 独立预算（对齐 execute-batch 既有模式）——
	// r.Context() 随客户端断开 cancel（axios 30s 超时 vs 100 条×重试链最坏
	// 远超），会中途砍灭全部补推（§59.345 同族）；Background 保证落库完成。
	bgCtx, bgCancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer bgCancel()
	for i, rec := range records {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, rec model.PublishResultRecord) {
			defer wg.Done()
			defer func() { <-sem }()
			// panic recover 对齐 execute-batch：单条 panic 不击穿进程
			defer func() {
				if rec2 := recover(); rec2 != nil {
					mu.Lock()
					outcomes[idx] = repushOutcome{ID: rec.ID, Status: "failed",
						Error: fmt.Sprintf("panic: %v", rec2)}
					mu.Unlock()
				}
			}()
			out := repushOutcome{ID: rec.ID, Status: "failed"}
			res := h.executor.Execute(bgCtx, publish.ExecuteInput{
				InfoHash:     rec.SourceInfoHash,
				TargetSite:   rec.TargetSite,
				TorrentID:    rec.TorrentID,
				PushOnly:     true,
				PushClientID: rec.DownloaderID,
				PushSavePath: rec.SavePath,
			})
			if res != nil && res.Status == "pushed" {
				out.Status = "pushed"
			} else if res != nil {
				out.Error = res.Message
			}
			mu.Lock()
			outcomes[idx] = out
			mu.Unlock()
		}(i, rec)
	}
	wg.Wait()

	pushed := 0
	for _, o := range outcomes {
		if o.Status == "pushed" {
			pushed++
		}
	}
	h.logger.Info("batch repush done",
		zap.Int("requested", len(req.IDs)),
		zap.Int("matched", len(records)),
		zap.Int("pushed", pushed))
	Success(w, map[string]interface{}{
		"requested": len(req.IDs),
		"matched":   len(records),
		"pushed":    pushed,
		"failed":    len(records) - pushed,
		"outcomes":  outcomes,
	})
}
