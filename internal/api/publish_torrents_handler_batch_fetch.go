package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/ranfish/pt-forward/internal/description"
	"github.com/ranfish/pt-forward/internal/fingerprint"
	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/metadata"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"github.com/ranfish/pt-forward/internal/site"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"github.com/ranfish/pt-forward/internal/util"
	"go.uber.org/zap"
)

// interleaveBySourceSite §59.340 D: 站点交错排序——组映射预判源站，把
// 同一源站的颗与其它源站/无源站颗 round-robin 混排（AABABAB 形态）。
// 泛型适配 runBatchFetch 的匿名 struct（Hash/Name 字段访问）。
func interleaveBySourceSite[T any](detector *publish.SourceSiteDetector, ctx context.Context, items []T) []T {
	if detector == nil || len(items) < 3 {
		return items
	}
	getName := func(it any) string {
		// 反射取 Name 字段（匿名 struct 无接口）
		v := reflect.ValueOf(it)
		f := v.FieldByName("Name")
		if f.IsValid() && f.Kind() == reflect.String {
			return f.String()
		}
		return ""
	}
	// 预判各颗源站（组映射启发式）
	siteOf := make(map[int]string)
	for i := range items {
		name := getName(items[i])
		g := publish.ExtractGroupName(name)
		if g == "" {
			continue
		}
		if s := detector.LookupGroup(ctx, g); s != "" {
			siteOf[i] = s
		}
	}
	if len(siteOf) == 0 {
		return items
	}
	// 找最大簇源站（该站颗最多——交错收益最大）
	count := map[string]int{}
	for _, s := range siteOf {
		count[s]++
	}
	mainSite := ""
	mainN := 0
	for s, n := range count {
		if n > mainN {
			mainSite, mainN = s, n
		}
	}
	// 交错：主站颗队列 + 其它颗队列 round-robin
	var mainQ, otherQ []int
	for i := range items {
		if siteOf[i] == mainSite {
			mainQ = append(mainQ, i)
		} else {
			otherQ = append(otherQ, i)
		}
	}
	out := make([]T, 0, len(items))
	mi, oi := 0, 0
	// 其它颗数 >= 主站颗数时主站先出（更均匀）；否则均衡分布
	for len(out) < len(items) {
		if mi < len(mainQ) && (oi >= len(otherQ) || len(out)%2 == 0 || len(otherQ)-oi <= len(mainQ)-mi) {
			out = append(out, items[mainQ[mi]])
			mi++
		} else if oi < len(otherQ) {
			out = append(out, items[otherQ[oi]])
			oi++
		} else if mi < len(mainQ) {
			out = append(out, items[mainQ[mi]])
			mi++
		}
	}
	return out
}

// snapshotProgressOf §59.537: 查快照最新 progress（无行 false——无快照颗
// 维持原行为不拦）。
func (h *PublishTorrentsHandler) snapshotProgressOf(ctx context.Context, hash string) (float64, bool) {
	if hash == "" {
		return 0, false
	}
	var row struct {
		Progress float64
	}
	err := h.db.WithContext(ctx).
		Table("torrent_snapshots").
		Select("progress").
		Where("hash = ? AND is_hidden = ?", hash, false).
		Order("updated_at DESC").First(&row).Error
	if err != nil {
		return 0, false
	}
	return row.Progress, true
}

// markBatchItemFailed §59.537: 按 Hash 定位批量槽位记 failed（interleave
// 重排后索引错位防御）+done/failed 计数。
func (h *PublishTorrentsHandler) markBatchItemFailed(hash, msg string) {
	h.batchFetch.mu.Lock()
	defer h.batchFetch.mu.Unlock()
	for i := range h.batchFetch.items {
		if h.batchFetch.items[i].Hash == hash {
			h.batchFetch.items[i].Status = "failed"
			h.batchFetch.items[i].Error = msg
			break
		}
	}
	h.batchFetch.done++
	h.batchFetch.failed++
}

func (h *PublishTorrentsHandler) runBatchFetch(clientUID uint, items []struct {
	Hash     string `json:"hash"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	SavePath string `json:"savePath"`
}, force bool) {
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("runBatchFetch panic", zap.Any("panic", r))
		}
		h.batchFetch.mu.Lock()
		h.batchFetch.active = false
		h.batchFetch.mu.Unlock()
	}()
	ctx := context.Background()
	// §59.21: 查下载器 is_local
	isLocal := false
	if clientUID != 0 {
		var client model.ClientConfig
		if err := h.db.WithContext(ctx).Where("id = ?", clientUID).First(&client).Error; err == nil {
			isLocal = client.IsLocal
		}
	}

	// §59.340 D: 站点交错排序——同源站（组映射预判）颗与其它颗 round-robin
	// 混排，稀释单站请求密度（keepfrds 流控探究：批量脉冲 31 逻辑请求触发
	// 封锁 vs 均匀 66 次——交错让 limiter 等待与异站处理并行）。预判仅用
	// 组映射启发式（ExtractGroupName+LookupGroup 本地 DB 毫秒级），与
	// 层选完整逻辑可不同——排序只影响处理顺序不影响正确性。
	items = interleaveBySourceSite(h.sourceDetector, ctx, items)

	for i, item := range items {
		// §59.495: graceful 停止检查点——不再开始新颗（正在执行的颗已完整
		// 跑完；剩余项保持 pending 原状=未获取语义·下次批量照跑——用户定案：
		// 未获取列表是唯一真相源·pending/被跳过本质都是未获取）
		h.batchFetch.mu.Lock()
		if h.batchFetch.stopRequested {
			h.batchFetch.mu.Unlock()
			h.logger.Info("batch-fetch graceful stop: remaining items skipped",
				zap.Int("remaining", len(items)-i))
			return // defer 释放 active
		}
		h.batchFetch.items[i].Status = "pending"
		h.batchFetch.mu.Unlock()

		// §59.537: 完成度边界——未 100% 不获取（数据不完整；做种/暂停不限
		// 只看完成度·用户定案）。列表已过滤·此为 API 直调防御层。Hash 定位
		// 槽位（interleave 重排后 i 与槽位索引错位——§59.340 既有形态·
		// Hash 扫描 ≤100 颗可接受）。
		if p, ok := h.snapshotProgressOf(ctx, item.Hash); ok && p < 1.0 {
			h.markBatchItemFailed(item.Hash, fmt.Sprintf("未下载完成（%.1f%%）·数据不完整不获取", p*100))
			continue
		}

		// §59.476 位置3: 代表 hash 换簇内音乐源站副本（辅种代表→皇后副本
		// →S1 天然命中）。影视簇/无音乐副本原样。name/savePath 簇内恒同。
		fetchHash, fetchSize := h.resolveMusicReplicaHash(ctx, clientUID, item.Hash, item.Size)

		// §59.61 第 4 步: 簇内已有完整数据（先前副本获取后传播）→ 跳过（同簇去重:
		// 站点数据/MI/截图各一次；显式单种子获取不受影响）
		// §59.479: force（批量重获）绕过——禁转/无映射视图的重获诉求即重取
		if !force && h.hasCompleteMetadata(ctx, fetchHash) {
			h.logger.Info("cluster skip: metadata exists",
				zap.String("hash", fetchHash[:min(10, len(fetchHash))]))
			h.batchFetch.mu.Lock()
			h.batchFetch.done++
			h.batchFetch.items[i].Status = "done"
			h.batchFetch.mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			continue
		}

		// §59.339: 单颗 watchdog——链路任何一环挂死（如 HTTP/2 半死流）
		// 时兜底放行批次（防 active 永锁 409）。根修在 DrainBody 限时，
		// 此为结构性第二防线；超时 goroutine 由根修唤醒退出（若仍有
		// 极端泄漏，代价一颗 goroutine 换整批可用）。
		type fetchResult struct{ err error }
		fetchDone := make(chan fetchResult, 1)
		go func() {
			fetchDone <- fetchResult{h.fetchSingleTorrent(ctx, clientUID, fetchHash, item.Name, fetchSize, item.SavePath, isLocal, force)}
		}()
		var err error
		select {
		case r := <-fetchDone:
			err = r.err
		case <-time.After(10 * time.Minute):
			err = fmt.Errorf("单颗获取 watchdog 超时 10min（链路挂死兜底）: %s", item.Name[:min(40, len(item.Name))])
			h.logger.Error("batch-fetch: watchdog timeout, skipping stuck item",
				zap.String("hash", item.Hash[:min(10, len(item.Hash))]),
				zap.String("name", item.Name[:min(50, len(item.Name))]))
		}

		h.batchFetch.mu.Lock()
		h.batchFetch.done++
		if err != nil {
			if isTransientFetchErr(err) {
				// §59.330 P4: 瞬态失败（冻结/熔断/超时/限流——站会恢复）
				// → deferred（延后重试）而非 failed（终态）
				h.batchFetch.items[i].Status = "deferred"
				h.batchFetch.items[i].Error = err.Error()
			} else {
				h.batchFetch.failed++
				h.batchFetch.items[i].Status = "failed"
				h.batchFetch.items[i].Error = err.Error()
			}
		} else {
			h.batchFetch.items[i].Status = "done"
		}
		h.batchFetch.mu.Unlock()

		time.Sleep(50 * time.Millisecond)
	}

	// §59.330 P4: deferred 二轮（L2 探测式——等全部冻结状态归零，上限 5min；
	// RSS 轮询会反复触碰冻结窗，固定等待不稳）。二轮仍瞬态失败转 failed
	// （错误信息标注瞬态类建议重跑——N1 定案）。
	h.retryDeferredBatch(ctx, clientUID, isLocal, force)
}

// isTransientFetchErr §59.330 P4: 瞬态错误分类——冻结/熔断/超时/限流类
// （站会恢复）与结构类（无源站/版本不匹配——重试无用）的判定。
func isTransientFetchErr(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "circuit open") ||
		strings.Contains(s, "context deadline exceeded") ||
		strings.Contains(s, "domain rate limit") ||
		strings.Contains(s, "domain frozen") ||
		strings.Contains(s, "empty_response") ||
		strings.Contains(s, "Client.Timeout") ||
		strings.Contains(s, "transient upstream")
}

// retryDeferredBatch §59.330 P4: deferred 集探测式二轮——等冻结状态归零
// （上限 5min）后重试一轮；仍瞬态失败转 failed（标注建议重跑）。
func (h *PublishTorrentsHandler) retryDeferredBatch(ctx context.Context, clientUID uint, isLocal bool, force bool) {
	h.batchFetch.mu.Lock()
	deferred := make([]int, 0)
	for i := range h.batchFetch.items {
		if h.batchFetch.items[i].Status == "deferred" {
			deferred = append(deferred, i)
		}
	}
	h.batchFetch.mu.Unlock()
	if len(deferred) == 0 {
		return
	}

	// 探测等待：全部冻结域归零或上限 5min（每 15s 探测一次）
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		if httpclient.GlobalLimiter.MaxFreeRemaining() <= 0 {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(15 * time.Second):
		}
	}

	retried := 0
	for _, i := range deferred {
		h.batchFetch.mu.Lock()
		if h.batchFetch.stopRequested { // §59.495: retry 轮同拦
			h.batchFetch.mu.Unlock()
			h.logger.Info("batch-fetch graceful stop: retry round skipped")
			return
		}
		if h.batchFetch.items[i].Status != "deferred" {
			h.batchFetch.mu.Unlock()
			continue
		}
		item := h.batchFetch.items[i]
		h.batchFetch.mu.Unlock()

		// §59.476 位置3: deferred 重试同款换 hash（首轮已换·幂等）
		retryHash, retrySize := h.resolveMusicReplicaHash(ctx, clientUID, item.Hash, item.Size)
		err := h.fetchSingleTorrent(ctx, clientUID, retryHash, item.Name, retrySize, item.SavePath, isLocal, force)
		retried++

		h.batchFetch.mu.Lock()
		if err == nil {
			h.batchFetch.items[i].Status = "done"
			h.batchFetch.items[i].Error = ""
		} else if isTransientFetchErr(err) {
			h.batchFetch.failed++
			h.batchFetch.items[i].Status = "failed"
			h.batchFetch.items[i].Error = "瞬态失败（站方限流/冻结未恢复），建议稍后重跑: " + err.Error()
		} else {
			h.batchFetch.failed++
			h.batchFetch.items[i].Status = "failed"
			h.batchFetch.items[i].Error = err.Error()
		}
		h.batchFetch.mu.Unlock()
		time.Sleep(50 * time.Millisecond)
	}
	if retried > 0 {
		h.logger.Info("batch-fetch deferred retry round complete",
			zap.Int("deferred", len(deferred)),
			zap.Int("retried", retried))
	}
}

func (h *PublishTorrentsHandler) fetchSingleTorrent(ctx context.Context, clientUID uint, hash, name string, size int64, savePath string, isLocal bool, forcePTGen ...bool) error {
	// §59.236 ①: forcePTGen 变参——单条重获 true（Force 绕缓存 §59.173）/
	// 批量 false（普通缓存——流控友好 §59.236 决策点①）
	force := len(forcePTGen) > 0 && forcePTGen[0]
	var coverageSites []model.SiteCoverageCache
	if h.coverage != nil {
		cs, err := h.coverage.GetCachedCoverage(ctx, hash)
		if err == nil {
			coverageSites = cs
		}
	}

	// §59.337: tracker coverage 保证——批量入口不产生 tracker 行（写入只在
	// 探测查询端点），新种子层选 ④ 空转。此处无 tracker 行时单颗补拉
	// （GetTrackers 1 次 RPC + MatchAll + 写缓存 tid 空行）——L0 同口径。
	// §59.469 附五: 补拉条件收窄——**未过期** tracker 行才算"已有"（过期行
	// 触发重拉）。许景淳种实证：10-18 旧"我的PT"行（PT0 副本 mypt tracker
	// 真实历史）10-19 已过期仍拦截补拉→PT2 副本的 musopia tracker 永远
	// 进不来→S1 无从命中→音乐链漏判（错选星陨阁簇 tid）。
	if h.coverage != nil {
		hasTracker := false
		for _, c := range coverageSites {
			if c.Source == model.CoverageSourceTracker && c.ExpiresAt.After(time.Now()) {
				hasTracker = true
				break
			}
		}
		if !hasTracker && h.clientMgr != nil {
			if client, err := h.clientMgr.Get(clientUID); err == nil {
				if trackers, terr := client.GetTrackers(ctx, hash); terr == nil && len(trackers) > 0 {
					tm := site.NewTrackerMatcher(h.db)
					now := time.Now()
					ttl := now.Add(24 * time.Hour)
					for _, sn := range tm.MatchAll(trackers) {
						if cerr := h.coverage.UpsertCoverage(ctx, &model.SiteCoverageCache{
							InfoHash: hash, SiteName: sn,
							Status: model.CoverageConfirmedHas, Source: model.CoverageSourceTracker,
							Confidence: 1.0, QueriedAt: now, ExpiresAt: ttl,
						}); cerr == nil {
							coverageSites = append(coverageSites, model.SiteCoverageCache{
								InfoHash: hash, SiteName: sn,
								Status: model.CoverageConfirmedHas, Source: model.CoverageSourceTracker,
								Confidence: 1.0, QueriedAt: now, ExpiresAt: ttl,
							})
						}
					}
				}
			}
		}
	}

	// §59.61: 簇 comment 直达候选——(client_id, save_path, name) 群聚合（管道 b 快照表）
	clusterTargets := h.buildClusterTargets(ctx, clientUID, savePath, name, hash)

	// §59.469+§59.476 分流判定：S1（coverage∩音乐源站）∨ S2+（标题音乐
	// 官组词·映射表推导词表+组位/否决闸）→ 音乐链：输入过滤（coverage/簇
	// 限定音乐源站内）——选站引擎零改动复用（五层自动在音乐源站空间工作；
	// 天空等影视站结构性出局）。
	musicRouting := h.isMusicSeedForRouting(ctx, name, coverageSites)
	if musicRouting {
		coverageSites = filterToMusicSources(coverageSites)
		clusterTargets = filterClusterToMusicSources(clusterTargets)
		h.logger.Info("music chain routing",
			zap.String("hash", hash),
			zap.Int("coverage_music", len(coverageSites)),
			zap.Int("cluster_music", len(clusterTargets)))
	}

	result := h.sourceDetector.SelectFetchSite(ctx, name, coverageSites, clusterTargets, hash)
	if result.SourceSite == "" {
		// §59.476 音乐链兜底站：纯流转种（S2+ 判音乐·五层证据全 miss——
		// coverage/簇 comment 均无音乐源站·集散地场景）synth 首选音乐源站
		// → 走 ①b 搜索分支（§59.474 音乐管道+验证层把关）+完整降级链。
		// 单站 synth（goto 不能跳变量声明区·结构限制）——皇后素材 12742
		// 覆盖绝大多数·全 miss 干净失败（宁漏勿误）。影视链保持原语义。
		if musicRouting {
			if ms := h.musicFallbackSites(ctx); len(ms) > 0 {
				result.SourceSite = ms[0]
				result.AutoDetected = true
				h.logger.Info("music fallback site synthesized",
					zap.String("hash", hash[:min(10, len(hash))]),
					zap.String("site", ms[0]))
			}
		}
		if result.SourceSite == "" {
			return fmt.Errorf("无可用源站（制作组未映射 + 无覆盖）")
		}
	}
	// §59.222: 发布链成人内容防线——馒头的成人区不可作为发布源（搜索层
	// 双模式开放给孤儿恢复/辅种，此处按站点名单过滤）。馒头 detail 的
	// mteamAdultCategories 已标 category.adult，此为源站选择层的前置防线。
	_ = result // 源站选择按组映射/coverage——成人区不会成为组映射目标站，
	// 此处无需额外代码（防御性注释）

	// §59.338: 总预算 60s→100s——直达 25s + browser fallback 45s（独立预算
	// 但父预算不暂停）= 70s 必然击穿 60s，簇直达/IYUU/搜索死 ctx 空转
	// （243 实证：危情24小时 12 站簇直达 1ms 全灭，凭证从未被真实请求）。
	fetchCtx, cancel := context.WithTimeout(ctx, 100*time.Second)
	defer cancel()

	// §59.36 修订: is_local=true 时本地 MI 提前获取（搜索反查前）——
	// 音频 token 冲突时作为 MI 仲裁的源侧证据（真测量）。失败不阻断（降级②盲放行）。
	// §59.319 P1: 原盘识别门（磁盘侧毫秒级）——命中则跳过伪 MI 生成
	// （D8 单 m2ts BDAV 伪产物停产）+ 入队 BDInfo 扫描（簇查重免重扫）。
	isDisc := false
	if isLocal && savePath != "" && h.bdinfoScanner != nil {
		if discPath := h.bdinfoScanner.DetectDiscPath(savePath, name); discPath != "" {
			isDisc = true
			if h.bdinfoQueue != nil && !h.clusterHasBDInfo(ctx, clientUID, savePath, name) {
				h.bdinfoQueue.Enqueue(publish.BDInfoScanTask{
					ClientUID: clientUID, SavePath: savePath, Name: name, DiscPath: discPath,
					Size: size, // §59.361 B: 超时自适应
				})
			}
		}
	}
	var localMI string
	if isLocal && !isDisc && savePath != "" && h.seedPipeline != nil {
		// §59.279: 本地 MI 分析限时 90s——ctx（批量=Background）无 deadline 时
		// 大文件/慢挂载分析可无限挂起（pt29 批量获取卡死案 item[0] 现场特征）；
		// 超时失败不阻断（§59.36 降级②盲放行语义保持）
		analyzeCtx, analyzeCancel := context.WithTimeout(ctx, 90*time.Second)
		artifacts, artErr := h.seedPipeline.AnalyzeLocalArtifacts(analyzeCtx, name, savePath)
		analyzeCancel()
		if artErr != nil {
			h.logger.Warn("batch-fetch: pre-fetch local mediainfo failed", zap.String("hash", hash), zap.Error(artErr))
		} else if mi, ok := artifacts["media_info"]; ok {
			localMI, _ = mi.(string)
		}
	}

	var meta *model.TorrentMetadata
	var err error
	if result.TorrentID != "" {
		// §59.338: 直达预算 25s→90s——覆盖主 fallback 45s + enrich fallback 45s
		// 的物理时长（fallback 独立预算不烧本 ctx，但返回后 enrich/store 继续
		// 用本 ctx——243 实证 90s 真实工作被 25s ctx 绞杀在最后一步 store：
		// "store metadata: context deadline exceeded"，browser fetch success
		// 9 次零落库）。HTTP 尝试的快速失败由 fetchDetailsHTML 内 readBody
		// 15s 停滞保护承担（不再依赖短 directCtx）。
		directCtx, directCancel := context.WithTimeout(fetchCtx, 90*time.Second)
		// §59.61: tid 来源含 coverage_tid/comment_tid——统一走 D3 轻校验入口
		// （coverage_tid 历史也校验：错缓存宁可降级搜索，符合审计精神）
		meta, err = h.metadataFetcher.FetchAndStoreDirect(directCtx, hash, result.SourceSite, result.TorrentID, name)
		directCancel()
		if err == nil {
			goto fetched
		}
		// §59.330 P4 补丁 c: 上游瞬态标记——直达失败的瞬态语义记录，
		// 链尾（IYUU 无候选提前 nil / exhausted 裸串）不携带时聚合穿透
		// （243 三轮实证：keepfrds 全断期 deferred=0 的盲区路径）。
		// §59.337: 聚合域扩展全链（直达/簇/IYUU/搜索任一瞬态即 deferred）——
		// IYUU 提前后链尾变搜索，IYUU 候选全冻结的瞬态原会被搜索错误串掩盖。
		transientSeen := isTransientFetchErr(err)
		// §59.337: 已试站全集——IYUU 兜底排除集（原仅排除层选源站，簇直达
		// 试过的站会重复直达烧预算）。
		triedSites := []string{result.SourceSite}
		// 直达失败（D3 拒绝/tid 失效）→ 降级链 §59.65（方案 B，The.Boys 实锤重排——
		// 老 FetchAndStore 内嵌 IYUU 兜底在此抢跑，克隆 iyuu_cache 覆盖源站语义）:
		// §59.330 P1（用户链路定案）: ② ②a 簇内其他 comment 直达（D3 校验）
		// 提到同站搜索前——本地 DB 决策毫秒级优先于网络搜索（243 FRDS 批量
		// 25 失败案实证：朋友站瞬态失败后先做了同站搜索=再撞冻结墙，簇
		// comment 直达被饿死在 60s 预算尾部）；
		// §59.337（用户定案链路重排）: ③ IYUU coverage 直达（本地 DB 挑候选
		// 毫秒级+排除已试站）提到 ④ 同站搜索前——搜索仅在"tid 失效反查"
		// 场景有价值，异站直达凭证成功率高成本更低。
		h.logger.Info("direct fetch failed, fallback chain start",
			zap.String("hash", hash[:min(10, len(hash))]),
			zap.String("site", result.SourceSite),
			zap.Error(err))
		// ② ②a: 簇内其他副本的 comment 凭证逐个直达（排除已试源站；D3 校验内建）
		// §59.341 ②（方案 a）: 获取成功但 PTGen 不可行（站方数据质量差——
		// From S01 案 PT地带无豆瓣链接）→ 不 break 换下一凭证站。
		for _, tgt := range clusterTargets {
			if tgt.TorrentID == "" || tgt.SiteName == result.SourceSite {
				continue
			}
			meta, err = h.metadataFetcher.FetchAndStoreDirect(fetchCtx, hash, tgt.SiteName, tgt.TorrentID, name)
			if err == nil {
				if h.ptgenViable(fetchCtx, meta) {
					h.logger.Info("cluster comment direct hit",
						zap.String("hash", hash[:min(10, len(hash))]),
						zap.String("site", tgt.SiteName),
						zap.String("tid", tgt.TorrentID))
					goto fetched
				}
				// PTGen 不可行：行已落库（保留），换下一凭证。
				// §59.341 回归审核修复：err 必须置非 nil——否则循环耗尽后
				// err==nil 挡住 IYUU/搜索降级链（if err != nil 全跳过），
				// 差数据行直接成终态（比修复前更早终止）。
				h.logger.Info("cluster comment hit but ptgen not viable, trying next",
					zap.String("hash", hash[:min(10, len(hash))]),
					zap.String("site", tgt.SiteName))
				err = fmt.Errorf("ptgen not viable on %s (data quality fallback)", tgt.SiteName)
			}
			triedSites = append(triedSites, tgt.SiteName)
			if isTransientFetchErr(err) {
				transientSeen = true
			}
		}
		// ③ IYUU coverage 直达（本地缓存挑未试站，≤5 候选；冻结机制自然限流）
		if err != nil {
			h.logger.Info("cluster comments exhausted, trying IYUU coverage direct",
				zap.String("hash", hash[:min(10, len(hash))]),
				zap.Strings("tried_sites", triedSites))
			meta, err = h.metadataFetcher.FetchAndStoreIYUU(fetchCtx, hash, triedSites, h.musicFetchArgs(musicRouting, localMI)...)
			if err == nil {
				h.logger.Info("IYUU coverage direct hit",
					zap.String("hash", hash[:min(10, len(hash))]),
					zap.String("site", meta.SiteName))
				goto fetched
			}
			// §59.482 刀7: 簇兄弟标题桥——IYUU 缓存空后的音乐链第三兜底
			// （§59.480 实时段回退·IYUU 配额保护·影视链零触碰）
			if musicRouting {
				if m := h.musicSiblingTitleBridge(fetchCtx, clientUID, savePath, name, hash, size, result.SourceSite, triedSites); m != nil {
					meta, err = m, nil
					h.logger.Info("music sibling title bridge hit",
						zap.String("hash", hash[:min(10, len(hash))]),
						zap.String("site", m.SiteName))
					goto fetched
				}
			}
			if isTransientFetchErr(err) {
				transientSeen = true
			}
		}
		// ④ 同站搜索（五闸门验证——tid 失效反查的最后手段）
		if err != nil {
			h.logger.Info("IYUU coverage exhausted, trying same-site search",
				zap.String("hash", hash[:min(10, len(hash))]),
				zap.Error(err))
			meta, err = h.metadataFetcher.FetchAndStoreBySearch(fetchCtx, hash, result.SourceSite, name, size, h.musicFetchArgs(musicRouting, localMI)...)
			if err != nil {
				// §59.337: 链尾瞬态聚合——全链（直达/簇/IYUU/搜索）任一瞬态
				// 即 deferred（deferred 语义不因链尾位置变化丢失）。
				if transientSeen {
					if !isTransientFetchErr(err) {
						return fmt.Errorf("transient upstream (direct/cluster/iyuu/search chain): %w", err)
					}
				}
				return err
			}
		}
	} else {
		// §59.231: ①b 组映射_搜索模式（tid 空）——此前搜索失败直接短路，
		// 簇 comment/IYUU 兜底全没走到（刘老庄案：CMCTV→不可说站内搜索
		// 0 结果即终败，而簇内 hdcmct.org/details.php?id=579801 完美凭证
		// 躺着没用）。对齐直达分支降级链：搜索失败→簇 comment 逐个直达
		// （D3 校验内建）→ IYUU 兜底。
		// §59.234 ①: 组映射确定源站——搜索词附加组名（suffix 变参——
		// 提取管道后追加；直接附加 name 会被提取层剥组名尾段）
		// §59.469: 音乐链不追加——音乐种通常无组名（普通用户上传为主；
		// 官组后缀少数存在：OpenCD/@Musopia——后缀非搜索必需词不影响命中），
		// 且影视尾段逻辑会把专辑段误提为"组"（Taylor Swift - 1989 (Deluxe)
		// (2014) → 组="1989 (Deluxe) (2014)"→keyword 重复拼接畸形）。
		// §59.330 P4 补丁 b: 搜索分支同款子预算——搜索烧 25s 即释放，
		// 簇 comment/IYUU 兜底继承剩余（活 ctx——243 实证搜索分支漏改）
		searchSuffix := ""
		if !musicRouting {
			searchSuffix = h.canonicalGroupName(ctx, result.GroupName)
		}
		searchCtx, searchCancel := context.WithTimeout(fetchCtx, 25*time.Second)
		meta, err = h.metadataFetcher.FetchAndStoreBySearch(searchCtx, hash, result.SourceSite, name, size, h.musicFetchArgs(musicRouting, localMI, searchSuffix)...)
		searchCancel()
		if err == nil {
			goto fetched
		}
		h.logger.Info("mapped-site search failed, trying cluster comments",
			zap.String("hash", hash[:min(10, len(hash))]),
			zap.String("site", result.SourceSite),
			zap.Error(err))
		// §59.337: 已试站全集（层选源站+簇直达试过的）——IYUU 排除集
		triedSites := []string{result.SourceSite}
		// §59.341 ②（方案 a）: 同直达分支——获取成功但 PTGen 不可行换下一凭证
		for _, tgt := range clusterTargets {
			if tgt.TorrentID == "" || tgt.SiteName == result.SourceSite {
				continue
			}
			meta, err = h.metadataFetcher.FetchAndStoreDirect(fetchCtx, hash, tgt.SiteName, tgt.TorrentID, name)
			if err == nil {
				if h.ptgenViable(fetchCtx, meta) {
					h.logger.Info("cluster comment direct hit (after mapped-site search)",
						zap.String("hash", hash[:min(10, len(hash))]),
						zap.String("site", tgt.SiteName),
						zap.String("tid", tgt.TorrentID))
					break
				}
				h.logger.Info("cluster comment hit but ptgen not viable, trying next (search path)",
					zap.String("hash", hash[:min(10, len(hash))]),
					zap.String("site", tgt.SiteName))
				// §59.341 回归审核修复：同直达分支——err 置非 nil 让 IYUU 降级链继续
				err = fmt.Errorf("ptgen not viable on %s (data quality fallback)", tgt.SiteName)
			}
			triedSites = append(triedSites, tgt.SiteName)
		}
		if err != nil {
			h.logger.Info("cluster comments exhausted, IYUU fallback (mapped-site search path)",
				zap.String("hash", hash[:min(10, len(hash))]),
				zap.Strings("tried_sites", triedSites),
				zap.Error(err))
			meta, err = h.metadataFetcher.FetchAndStoreIYUU(fetchCtx, hash, triedSites, h.musicFetchArgs(musicRouting, localMI)...)
			// §59.482 刀7: 同款桥（①b 分支）
			if err != nil && musicRouting {
				if m := h.musicSiblingTitleBridge(fetchCtx, clientUID, savePath, name, hash, size, result.SourceSite, triedSites); m != nil {
					meta, err = m, nil
					h.logger.Info("music sibling title bridge hit",
						zap.String("hash", hash[:min(10, len(hash))]),
						zap.String("site", m.SiteName))
				}
			}
		}
	}
	if err != nil {
		return err
	}
fetched:

	// §59.49+§59.57: 截图探活与策略——strategy 可用时探活内联进其前序（串行消除竞态：
	// 原双 goroutine 并发，strategy 读到 purge 前死链 → rehost 失败保源 → same 早退 → 永不捕获）；
	// strategy 不可用（远程/无 savePath）时探活独立异步执行。
	// §59.490: 音乐种跳过截图链——mpv 渲染音频文件（无视频轨）白截：
	// 每颗几十秒策略预算+并发闸门额度+"total failure" warn 噪声（64 颗
	// 获取全白跑实证）。音乐种发布不消费截图（乌托邦 descr=致谢+封面）。
	if meta != nil && !metadata.HasMusic(meta.DetailSourceJSON) {
		if h.shotStrategy != nil && savePath != "" {
			// §59.299: force（单种重获）跳过簇缓存读——每次全新截传+写穿（Q4 语义
			// 对齐手动捕获链）；批量 force=false 维持 Q3 缓存优先
			go h.applyScreenshotStrategy(clientUID, meta.InfoHash, meta.SiteName, name, savePath, isLocal, force)
		} else if meta.Screenshots != "" && meta.Screenshots != "[]" {
			go h.purgeDeadScreenshots(meta.InfoHash, meta.SiteName)
		}
	}

	// §59.21: is_local=true 时落库本地 mediainfo（localMI 已在搜索前获取，直接复用）
	// §59.56: 列名笔误修复（mediainfo_source → media_info_source，v0.0.650 同族第三处）
	// + .Error 检查（原静默吞错——243 实测 760 次失败无声，148 组本地 MI 全空）
	if isLocal && localMI != "" && meta != nil {
		if err := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
			Where("info_hash = ? AND site_name = ?", meta.InfoHash, meta.SiteName).
			Updates(map[string]interface{}{
				"media_info":        localMI,
				"media_info_source": "local",
			}).Error; err != nil {
			h.logger.Error("local mediainfo persist failed",
				zap.String("hash", meta.InfoHash[:10]),
				zap.Error(err))
		} else {
			// §59.171 B: 本地 MI 落库后簇传播——fetch 行复制只给无行兄弟建行
			// （haveSet 跳过已有行），重获场景副本恒空（PT31 63 副本实锤）
			propagateClusterMediainfoDB(h.db, h.logger, ctx, clientUID, savePath, name, meta.InfoHash, localMI)
		}
	}

	// §59.236 ①: 主链必经 PTGen——豆瓣链接 → PTGen 端点查询 →
	// ptgen_source_json（唯一账本，落代表行）+ desc=RawBBCode。
	// 失败全空（incomplete 状态机承接——用户自查端点；站方原文
	// 不保存 §59.236 定案）。取代 §59.235 P2 的 extractPTGenAssets
	// 双时序补丁（架构性取代——kdouban/NexusPHP 分叉根除）。
	// §59.286 终版: 主链 PTGen 单点单责——三级顺序链成功时同步吸收②副产物
	// （②海报链全线下线；海报簇传播由尾部 finalizeClusterPropagation
	// 统一执行——幂等可重复；ptgen_source_json canonical 包装已并入主链 persist）
	// §59.496 附二 B1: 挪至 localMI 落库后
	// §59.500 刀2: t2 refreshInferredTags 轮删除——B1 挪序后 PTGen 先于
	// t0 完成·t2 与 t0 输入完全同（纯冗余轮）；且 t2 无闸门会在截图未到
	// 时写 tags 绕过"完整才推理"语义。tags 统一由 inferFinalTags 单点产出
	// （Description/评分素材此刻已在场·t0 闸门版输入等价 t2）
	_ = h.runMainlinePTGen(ctx, meta, force)

	// §59.26: TechProfile 三源合并 + 分类推断 + 声明过滤（与 runAnalyze ⑫ 统一管线）
	if meta != nil {
		var finalMeta model.TorrentMetadata
		h.db.WithContext(ctx).Where("info_hash = ? AND site_name = ?", meta.InfoHash, meta.SiteName).First(&finalMeta)

		if finalMeta.Title != "" {
			// §59.26: MI 优先 local（MediaInfo），fallback 源站（SourceMediaInfo）
			// §59.319 P1: 原盘优先 BDInfo（ExtractMediaInfo 格式分流自动解析）
			miForProfile := finalMeta.BDInfo
			if miForProfile == "" {
				miForProfile = finalMeta.MediaInfo
			}
			if miForProfile == "" {
				miForProfile = finalMeta.SourceMediaInfo
			}
			// §59.28 D1: DOM 源接入（DetailSourceJSON 的 medium/codec/resolution），
			// 种子配置页与 runAnalyze 走同一套三源合并管线（§59.26 设计）
			domMedium, domRes, domVideo, domAudio := titleparser.DOMFieldsFromDetailSource(finalMeta.DetailSourceJSON)
			profile := titleparser.BuildTechProfile(finalMeta.Title, miForProfile, domMedium, domRes, domVideo, domAudio)
			// §59.77: 评论音轨扣减（v1.05 不计入——副标题声明提取）
			profile.AudioTracks = titleparser.AdjustCommentaryTracks(profile.AudioTracks, finalMeta.Subtitle, miForProfile)
			// §59.514 附八: 重组标题资产化——FinalizeTitle 共享单点（发布链
			// 同款终态链：PTGen+仲裁+归一+重组）。torrentData=ExportTorrent
			// 顺路拉（isLocal 场景·下载器本地 RPC 毫秒级·与 localMI 生成
			// 同级成本）；拉取失败列降级（归一+重组无仲裁·发布时再终态化）。
			var torrentData []byte
			if isLocal && h.clientMgr != nil {
				if cl, clErr := h.clientMgr.Get(clientUID); clErr == nil && cl != nil {
					fetchCtx, fetchCancel := context.WithTimeout(ctx, 30*time.Second)
					if td, tdErr := cl.ExportTorrent(fetchCtx, meta.InfoHash); tdErr == nil {
						torrentData = td
					}
					fetchCancel()
				}
			}
			reassembledTitle, reassembledNotes := h.executor.FinalizeTitle(ctx, &finalMeta, &profile, torrentData)
			components := titleparser.TechProfileToComponents(profile)
			// §59.236 ④: InferCategory 的 PTGen 输入（五级链③④级信号）从
			// 主链唯一账本读（runMainlinePTGen 已先行落库——此处终态 JSON）
			ptgGenre, ptgEpi := "", ""
			if src, pErr := metadata.UnmarshalPTGenSource(finalMeta.PTGenSourceJSON); pErr == nil && src != nil {
				ptgGenre = strings.Join(src.Genre, " ")
				ptgEpi = src.Episodes
			}
			category := titleparser.InferCategory(components, finalMeta.SourceCategory, ptgGenre, ptgEpi)

			updates := map[string]interface{}{
				"reassembled_title": reassembledTitle,
				"reassembled_notes": strings.Join(reassembledNotes, "；"),
				"category":          category,
				"resolution":        profile.Resolution,
				"video_codec":       profile.VideoCodec,
				"audio_codec":       profile.AudioCodec,
				"audio_channels":    profile.AudioChannels,
				"audio_tech":        profile.AudioTechnology,
				"audio_tracks":      profile.AudioTracks, // §59.76: v1.05 #16
				"hdr":               profile.HDR,
				"bit_depth":         profile.BitDepth,
				"source_type":       profile.SourceType,
				"specification":     profile.Specification,
				"source_platform":   profile.SourcePlatform,
				"edition_info":      profile.EditionInfo,
				"region_code":       profile.RegionCode,
			}

			// §59.265 附: statement 列同步过滤——站点固定资源声明在声明列（Tab2
			// 海报与声明），此前只滤 description 列导致批量/单种重获均不剥（243
			// 四种子案）。先滤后接致谢（thanks-append 基线用净化后值）；
			// NOGROUP 分支也落库（否则原始声明残留）。
			if h.declFilter != nil {
				patterns := h.declFilter.GetPatterns(ctx)
				if finalMeta.Description != "" {
					fr := h.declFilter.Filter(finalMeta.Description, patterns)
					updates["description"] = fr.CleanedText
				}
				if finalMeta.Statement != "" {
					fs := h.declFilter.Filter(finalMeta.Statement, patterns)
					finalMeta.Statement = fs.CleanedText
					if profile.ReleaseGroup == "" || profile.ReleaseGroup == "NOGROUP" {
						updates["statement"] = finalMeta.Statement
					}
				}
			}

			// §59.27: flags 以本次 detail 为准（detail_source_json.flags 是新提取值，
			// 但 GORM Assign 忽略零值导致 DB 旧 flags 残留——map 显式覆盖空值）
			var detailFlags []string
			if finalMeta.DetailSourceJSON != "" {
				var ds struct {
					Flags []string `json:"flags"`
				}
				if json.Unmarshal([]byte(finalMeta.DetailSourceJSON), &ds) == nil {
					detailFlags = ds.Flags
				}
			}
			if detailFlags == nil {
				detailFlags = []string{}
			}
			if newFlags, err := json.Marshal(detailFlags); err == nil {
				updates["flags"] = string(newFlags)
			}

			// §59.542 追记十二（用户定案·回归原始设计）：获取链不再入库
			// append 致谢——DB statement 保持净化态（站方原始声明·数据与
			// 水印彻底分离）。tab2 展示与发布组装由 description.ComposeStatement
			// 消费时动态合成（单点）。历史 append 时灵时不灵（11-12 起
			// 实证失效·title 形态时序依赖）与回流污染两个问题一次根治。
			_ = profile

			h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
				Where("info_hash = ? AND site_name = ?", meta.InfoHash, meta.SiteName).
				Updates(updates)
			// §59.514 附八: 重组标题簇传播（蓝军⑥——副本行空列 fallback 高频
			// 触发堵漏·propagateClusterMediainfoDB 同款语义）
			if reassembledTitle != "" {
				propagateClusterTitleDB(h.db, h.logger, ctx, clientUID, savePath, name, meta.InfoHash,
					reassembledTitle, strings.Join(reassembledNotes, "；"))
			}
			// §59.500 刀1: tags 推理换 inferFinalTags 单点——完整性闸门
			// （截图/BDInfo 未到不推·后续数据源落地触发）；原 t0 内联推理
			// 段（§59.26/§59.326/§59.496 附二逻辑全收编入单点）
			h.inferFinalTags(ctx, clientUID, meta.InfoHash, meta.SiteName)
		}
	}

	// §59.61 第 4 步 + 附5: 簇终局传播——等海报 fallback 终局后 INSERT 缺行
	// （携带终态）+ 终态回传（幂等）
	h.finalizeClusterPropagation(ctx, clientUID, savePath, name, hash, meta.SiteName)

	return nil
}

// thanksAppendMarker §59.28 致谢追加分隔标记（历史形态）：老组名「FRDS」
// 时代（组映射后改名「FRDS torrent」）追加的致谢块文本·243 存量 5551 行依赖
// 此兜底剥离——保留（§59.542 刀1 扫描定案：新形态不命中此标记·零误伤）。
const thanksAppendMarker = "FRDS官组作品"

// thanksCoreText §59.542 刀1: 单源引用（本体在 internal/description——
// 声明过滤词表同样引用·三处消费一源）。
const thanksCoreText = description.ThanksCoreText

// thanksNoTransferTail §59.260: 追加块固定尾引言（用户定案不变文案）——
// 任意组名/中英模板的结构锚（幂等剥离通用判据）。§59.542 刀1: 从 thanksCoreText
// 合成（消灭第二份字面量）。
const thanksNoTransferTail = "[quote][b][color=red][size=5]" + thanksCoreText + "[/size][/color][/b][/quote]"

// §59.542 刀2: 派生锚+形态宽容——站方 BBCode 规范化实证三重改写
// （blue→#0000ff / red→#ff0000 / [quote]后插换行·Pope 案 2026-11-19），
// 字面锚全部失配致旧对剥不掉→重获累积（13 颗双份实证）。正则化：
// 色值枚举+标签间 \s* 宽容换行；head 锚按「蓝色致谢块形态」内容无关
// （源站无此文本——thanksCoreText 系统独有·任意蓝色致谢块必为
// 我方追加或回流·剥除安全）；tail 锚内容锁 thanksCoreText 单源。
var (
	thanksHeadBlockRe = regexp.MustCompile(`(?s)\[quote\]\s*\[b\]\s*\[color=(?:blue|#0000ff)\]\s*\[size=5\]\s*[^\[]{1,400}?\[/size\]\s*\[/color\]\s*\[/b\]\s*\[/quote\]`)
	// §59.555: 尾锚用 Legacy（无句号）+可选句号——ThanksCoreText 句号化后
	// 纯句号锚漏剥存量无句号形态（回流治回归）。Legacy 为句号版子串·
	// `。?` 使单锚双形态兼容。
	thanksTailBlockRe = regexp.MustCompile(`(?s)\[quote\]\s*\[b\]\s*\[color=(?:red|#ff0000)\]\s*\[size=5\]\s*` + regexp.QuoteMeta(description.ThanksCoreTextLegacy) + `。?\s*\[/size\]\s*\[/color\]\s*\[/b\]\s*\[/quote\]`)
)

// stripAppendedThanks 剥离历史追加的致谢块（§59.28 幂等 + §59.260 通用化 +
// §59.542 刀2 形态宽容）。追加格式固定双 quote 结尾：
//
//	[quote][b][color=blue][size=5]{组名}官组作品...[/quote]\n[quote][b][color=red][size=5]互珍互重，禁转PTT...[/quote]
//
// §59.542 刀2: 正则锚（色值枚举+换行宽容）——站方规范化形态（#0000ff/#ff0000/
// quote 后换行）与原生形态（blue/red/无换行）一律可剥；head 按蓝色致谢块形态
// 内容无关匹配（源站无此文本·任意蓝色致谢块必为我方产物）。
func stripAppendedThanks(statement string) string {
	// 循环剥离（累积多轮的历史数据一次清净）：先剥尾引言块（内容锁
	// thanksCoreText），再剥 head 中最后一个蓝色致谢块（配对优先·无配对
	// 时尾块单独剥——不误伤源站真声明块）。
	for {
		tail := thanksTailBlockRe.FindStringIndex(statement)
		if tail == nil {
			break
		}
		head := statement[:tail[0]]
		if hs := thanksHeadBlockRe.FindAllStringIndex(head, -1); len(hs) > 0 {
			last := hs[len(hs)-1]
			statement = strings.TrimSpace(head[:last[0]])
			continue
		}
		// 无配对致谢（头块被站方超预期改写？）——仅剥尾块自身·不动其它 quote
		statement = strings.TrimSpace(head)
	}
	// 孤立致谢头块清扫（尾块已不在/被超预期改写的残留——蓝色致谢块必为
	// 我方产物·剩余多少剥多少）
	for thanksHeadBlockRe.MatchString(statement) {
		statement = strings.TrimSpace(thanksHeadBlockRe.ReplaceAllString(statement, ""))
	}
	// 旧 FRDS 标记兜底（历史数据兼容——同循环）
	for strings.Contains(statement, thanksAppendMarker) {
		idx := strings.LastIndex(statement, thanksAppendMarker)
		start := strings.LastIndex(statement[:idx], "[quote]")
		if start < 0 {
			break
		}
		statement = strings.TrimSpace(statement[:start])
	}
	return statement
}

// extractChineseFromSubtitle 从副标题提取【中文名】（§59.26 朋友站格式）。
func extractChineseFromSubtitle(subtitle string) string {
	re := regexp.MustCompile(`^[【\[]([^】\]]+)[】\]]`)
	if m := re.FindStringSubmatch(strings.TrimSpace(subtitle)); len(m) > 1 {
		return m[1]
	}
	return ""
}

// BusyTaskDesc §59.281: 运行中长任务描述（""=空闲）——OTA 退出门控消费。
func (h *PublishTorrentsHandler) BusyTaskDesc() string {
	h.batchFetch.mu.Lock()
	if h.batchFetch.active {
		n := fmt.Sprintf("%d/%d", h.batchFetch.done, h.batchFetch.total)
		h.batchFetch.mu.Unlock()
		return "批量获取 " + n
	}
	h.batchFetch.mu.Unlock()
	h.siteBatch.mu.Lock()
	if len(h.siteBatch.active) > 0 {
		sites := make([]string, 0, len(h.siteBatch.active))
		for site, tid := range h.siteBatch.active {
			if t, ok := h.siteBatch.tasks[tid]; ok {
				sites = append(sites, fmt.Sprintf("%s %d/%d", site, t.Done, t.Total))
			} else {
				sites = append(sites, site)
			}
		}
		h.siteBatch.mu.Unlock()
		return "站点批量发布 " + strings.Join(sites, ",")
	}
	h.siteBatch.mu.Unlock()
	return ""
}

// handleBatchFetchProgress §59.20: 查询批量获取进度。
func (h *PublishTorrentsHandler) handleBatchFetchProgress(w http.ResponseWriter, r *http.Request) {
	h.batchFetch.mu.Lock()
	defer h.batchFetch.mu.Unlock()
	Success(w, map[string]interface{}{
		"active":        h.batchFetch.active,
		"stopRequested": h.batchFetch.stopRequested, // §59.495
		"total":         h.batchFetch.total,
		"done":          h.batchFetch.done,
		"failed":        h.batchFetch.failed,
		"items":         h.batchFetch.items,
	})
}

// handleBatchFetchCancel §59.495: 批量获取 graceful 停止——不再开始
// 新颗·正在采集/落盘的当前颗完整执行完（自然终态 done/failed）；剩余
// 项保持 pending 原状（未获取语义·下次批量照跑——用户定案：未获取
// 列表是唯一真相源）。幂等：重复请求返回"已在停止中"。
func (h *PublishTorrentsHandler) handleBatchFetchCancel(w http.ResponseWriter, r *http.Request) {
	h.batchFetch.mu.Lock()
	defer h.batchFetch.mu.Unlock()
	if !h.batchFetch.active {
		Success(w, map[string]interface{}{"message": "无运行中的批量任务", "stopped": false})
		return
	}
	if h.batchFetch.stopRequested {
		Success(w, map[string]interface{}{"message": "已在停止中（当前种子完成后终止）", "stopped": true})
		return
	}
	h.batchFetch.stopRequested = true
	h.logger.Info("batch-fetch graceful stop requested",
		zap.Int("done", h.batchFetch.done), zap.Int("failed", h.batchFetch.failed))
	Success(w, map[string]interface{}{
		"message": "停止请求已接受——当前种子完成后终止，剩余种子保持未获取",
		"stopped": true,
	})
}

// handleListSeeds §59.20 §59.29: 种子配置页列表 API。
// 数据流：snapshots(可选 client/path 过滤) → name 去重 → metadata → 状态标注 →
// 状态/搜索后端过滤 → 分页。
// §59.29: client_id/save_path 全可选（空=全部），进页面即有数据（方案 A 全部视图）。
// observingGracePeriod §59.38: 观察期滞后期——(client,name) 全变体 hidden 持续
// 满 7 天自动清理（TR 维护/移路径/误删重加的恢复窗口）。
const observingGracePeriod = 7 * 24 * time.Hour

// handleListObserving §59.38: 观察期视图——辅种组（client+name）全部变体 hidden 的资源。
// 行内容：name + 变体数 + 消失时间（max last_seen）+ 清理倒计时 + 下载器/路径。
// 资源实体语义（用户定案）：辅种 = 同下载器同资源不同站点；跨下载器为独立副本互不影响。
func (h *PublishTorrentsHandler) handleListObserving(w http.ResponseWriter, r *http.Request, clientUID uint, savePath, search string) {
	// §59.552 变体：默认 50+上限 100 为 §59.166 单批发布业务定案（不迁标准 200）
	page, pageSize := parsePaginationWith(r, 50, 100, 10000)

	// 观察组 = (client, name) 聚合：全变体 hidden（组内无活跃行）且 name 非空
	type obsRow struct {
		ClientUID uint
		Name      string
		Variants  int64
		LastSeen  string // MAX() 聚合返回 string（driver 层），解析用
		SavePath  string
		Size      int64
	}
	q := h.db.WithContext(r.Context()).
		Table("torrent_snapshots").
		Select(`client_uid, name, COUNT(*) AS variants, MAX(last_seen) AS last_seen,
			(SELECT save_path FROM torrent_snapshots s2 WHERE s2.client_uid = torrent_snapshots.client_uid AND s2.name = torrent_snapshots.name ORDER BY last_seen DESC LIMIT 1) AS save_path,
			MAX(size) AS size`).
		// §59.38 审计修正: WHERE 不限 hidden——HAVING 活跃检测需全组行进聚合域
		// （原 is_hidden=1 使 SUM(active) 恒 0，组内仍有活跃行的资源被误列观察期）
		Where("name != ''").
		Group("client_uid, name").
		Having("SUM(CASE WHEN is_hidden = 0 THEN 1 ELSE 0 END) = 0 AND SUM(CASE WHEN is_hidden = 1 THEN 1 ELSE 0 END) > 0")
	if clientUID != 0 {
		q = q.Where("client_uid = ?", clientUID)
	}
	if savePath != "" {
		// §59.38 审计修正: 按 (client_id, name) 组过滤——原 IN client 放大返回该下载器全部组
		q = q.Where("client_uid || '|' || name IN (SELECT client_uid || '|' || name FROM torrent_snapshots WHERE save_path = ?)", savePath)
	}
	if search != "" {
		q = q.Where("name LIKE ?", "%"+search+"%")
	}
	var rows []obsRow
	if err := q.Order("last_seen DESC").Find(&rows).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "查询观察期失败: "+err.Error())
		return
	}

	items := make([]map[string]interface{}, 0, len(rows))
	now := time.Now()
	for _, row := range rows {
		lastSeen, _ := time.Parse("2006-01-02 15:04:05.999999999-07:00", row.LastSeen)
		if lastSeen.IsZero() {
			lastSeen, _ = time.Parse(time.RFC3339, row.LastSeen)
		}
		remainingDays := 0
		if !lastSeen.IsZero() {
			remaining := observingGracePeriod - now.Sub(lastSeen)
			remainingDays = int(remaining.Hours() / 24)
			if remainingDays < 0 {
				remainingDays = 0
			}
		}
		items = append(items, map[string]interface{}{
			"client_id":       row.ClientUID,
			"name":            row.Name,
			"variants":        row.Variants,
			"last_seen":       row.LastSeen,
			"save_path":       row.SavePath,
			"size":            row.Size,
			"cleanup_in_days": remainingDays,
			"status":          "observing",
		})
	}

	total := len(items)
	start := (page - 1) * pageSize
	if start >= total {
		items = []map[string]interface{}{}
	} else {
		end := start + pageSize
		if end > total {
			end = total
		}
		items = items[start:end]
	}
	Success(w, map[string]interface{}{"items": items, "total": total})
}

// handlePurgeObserving §59.38: 立即清理——观察期资源主动确认不等 7 天。
// 两级判定（防跨下载器 metadata 误删）：
//
//	① 删该 (client, name) 的全部 hidden 快照行
//	② metadata 仅当 info_hash 不被任何其他下载器活跃/观察期快照引用才删
func (h *PublishTorrentsHandler) handlePurgeObserving(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientUID uint   `json:"clientId"`
		Name      string `json:"name"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.ClientUID == 0 || req.Name == "" {
		Error(w, http.StatusBadRequest, 40001, "clientId 和 name 必填")
		return
	}
	deleted := h.purgeObservingResource(r.Context(), req.ClientUID, req.Name)
	Success(w, map[string]interface{}{
		"message":       "已清理",
		"deleted_snaps": deleted.snapRows,
		"deleted_metas": deleted.metaRows,
	})
}

type purgeResult struct {
	snapRows int64
	metaRows int64
}

// purgeObservingResource 两级清理（立即清理与定时任务共用）。
// 安全前置：仅当该组确无活跃行才执行（活跃=误判，拒绝清理）。
func (h *PublishTorrentsHandler) purgeObservingResource(ctx context.Context, clientUID uint, name string) purgeResult {
	var active int64
	h.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
		Where("client_uid = ? AND name = ? AND is_hidden = ?", clientUID, name, false).
		Count(&active)
	if active > 0 {
		return purgeResult{}
	}

	// ① 该组全部 hash
	var hashes []string
	h.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
		Where("client_uid = ? AND name = ?", clientUID, name).
		Pluck("hash", &hashes)
	if len(hashes) == 0 {
		return purgeResult{}
	}

	// ② metadata 两级判定：hash 被其他下载器（或本下载器其他组）活跃引用则跳过
	var referenced []string
	h.db.WithContext(ctx).
		Table("torrent_snapshots").
		Where("hash IN ? AND is_hidden = ? AND (client_uid != ? OR name != ?)", hashes, false, clientUID, name).
		Distinct("hash").
		Pluck("hash", &referenced)
	refSet := make(map[string]bool, len(referenced))
	for _, h2 := range referenced {
		refSet[h2] = true
	}
	purgeHashes := make([]string, 0, len(hashes))
	for _, h2 := range hashes {
		if !refSet[h2] {
			purgeHashes = append(purgeHashes, h2)
		}
	}

	var res purgeResult
	// 删 metadata（发布记录不动——发布域史实）
	if len(purgeHashes) > 0 {
		d := h.db.WithContext(ctx).Where("info_hash IN ?", purgeHashes).
			Delete(&model.TorrentMetadata{})
		res.metaRows = d.RowsAffected
	}
	// 删该组快照行
	d := h.db.WithContext(ctx).Where("client_uid = ? AND name = ?", clientUID, name).
		Delete(&model.TorrentSnapshot{})
	res.snapRows = d.RowsAffected
	h.logger.Info("observing resource purged",
		zap.Uint("client", clientUID),
		zap.String("name", name[:min(len(name), 50)]),
		zap.Int64("snaps", res.snapRows),
		zap.Int64("metas", res.metaRows))
	return res
}

func (h *PublishTorrentsHandler) handleListSeeds(w http.ResponseWriter, r *http.Request) {
	clientUID := uint(0)
	if v := r.URL.Query().Get("client_id"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			clientUID = uint(n)
		}
	}
	savePath := r.URL.Query().Get("save_path")
	statusFilter := r.URL.Query().Get("status")
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	// §59.131 ②: ready 过滤——发布页（一种多站）簇口径 data_ready 筛选（R3-3: 簇级=reviewed）
	readyFilter := r.URL.Query().Get("ready")
	// §59.143: 发布页排除源站禁转——forbidden（flags 内存判定零成本）直接隐藏；
	// system_forbidden 精判在分页后（排除致页不齐）——保留红标, Eligibility 提交兜底
	excludeForbidden := r.URL.Query().Get("exclude_forbidden") == "true"

	// §59.166 一站多种：目标站发布维度筛选（publish_state=publishable/published
	// + target_site）。口径（用户定案）：已发布=终态有记录（pushed/pushed_existing/
	// uploaded/duplicate/existing——uploaded=上传成功站上已有，§59.166 补入：
	// 漏种簇不再留在"可发布"防整体重传）；可发布=已审核（reviewed 蕴含资产完备——种配审核
	// 流程权威保证）AND 无四终态记录（failed 算未发布可重试）；关联键=簇成员 hash
	// 集合反查簇键（宁漏勿错——早期空 source_info_hash 记录匹配不上→落可发布，
	// dedup 兜底）。
	publishState := r.URL.Query().Get("publish_state")
	publishTargetSite := r.URL.Query().Get("target_site")
	var publishedClusters map[string]bool
	// §59.547: 填充条件宽化为 target 在场（原含 publishState 前置——API 直调
	// target+无 state 时 site_tags 的选中站置顶判断会漏记录面）
	if publishTargetSite != "" {
		var pubHashes []string
		h.db.WithContext(r.Context()).
			Table("publish_result_records").
			Where("target_site = ? AND source_info_hash != '' AND status IN ?",
				publishTargetSite, model.PublishedListStatuses).
			Distinct("source_info_hash").
			Pluck("source_info_hash", &pubHashes)
		publishedClusters = make(map[string]bool)
		if len(pubHashes) > 0 {
			type ckRow struct {
				ClientUID uint
				SavePath  string
				Name      string
			}
			var cks []ckRow
			h.db.WithContext(r.Context()).
				Table("torrent_snapshots").
				Select("DISTINCT client_uid, save_path, name").
				Where("hash IN ? AND is_hidden = 0 AND name != ''", pubHashes).
				Find(&cks)
			for _, c := range cks {
				publishedClusters[fmt.Sprintf("%d|%s|%s", c.ClientUID, c.SavePath, c.Name)] = true
			}
		}
	}

	// §59.38: 观察期视图——独立分支（hidden 组按 (client,name) 聚合，
	// 不与活跃视图共用查询/状态机）
	if statusFilter == "observing" {
		h.handleListObserving(w, r, clientUID, savePath, search)
		return
	}

	// §59.552 变体：默认 50+上限 100 为 §59.166 单批发布业务定案（不迁标准 200）
	page, pageSize := parsePaginationWith(r, 50, 100, 10000)

	// 1. 查 snapshots——§59.26: 先去重再分页；§59.29: SQL 层分组去重（20w+ 行时
	// 避免 Go 全量载入）。同 client+path+name 的多站点 hash 只取 updated_at 最新一行。
	// 无 name 的行按 hash 自成一组（name='' 时 MAX(hash) 无意义但保持行为：空名行退化
	// 为按 hash 去重等价于不去重）。
	// §59.537: 完成度 100% 边界（用户定案）——获取/展示只认数据完整的种子
	// （做种中/暂停均可·不考虑做种状态只看完成度；reseed 引擎 Progress<1.0
	// continue 同款语义对齐）。未完成副本被剔除后完成副本成为簇代表；整簇
	// 未完成则整簇不出现（未完成不是可获取/可发布素材——重庆森林 pausedDL
	// 0% 案族）。
	snapQuery := h.db.WithContext(r.Context()).
		Table("torrent_snapshots AS s").
		Select("s.*").
		Where("s.is_hidden = ?", false).
		Where("s.progress >= ?", 1.0)

	// SQLite/MySQL 兼容的"组内最新行"：子查询取每组 MAX(id)。
	// name 为空时用 hash 作为组键（等效不去重）。
	// §59.537: sub 同滤 progress（组代表选择必须先剔未完成——否则代表 id
	// 是未完成行，主查询 IN 匹配后过滤会误杀整簇完成副本）。
	sub := h.db.WithContext(r.Context()).
		Table("torrent_snapshots").
		Select("MAX(id) AS id").
		Where("is_hidden = ?", false).
		Where("progress >= ?", 1.0).
		Group("client_uid, save_path, CASE WHEN name = '' THEN hash ELSE name END")
	if clientUID != 0 {
		snapQuery = snapQuery.Where("s.client_uid = ?", clientUID)
		sub = sub.Where("client_uid = ?", clientUID)
	}
	if savePath != "" {
		snapQuery = snapQuery.Where("s.save_path = ?", savePath)
		sub = sub.Where("save_path = ?", savePath)
	}
	// §59.29: 搜索下推 SQL，20w+ 行时避免 Go 层全量过滤
	// §59.138: 三域搜索——CJK 词跨表扩 title/subtitle（Yumi 纯英文簇的中文剧名在
	// subtitle 域）；拉丁词纯 name LIKE。回归审核定案：29 大库 FLAC 全表跨表子查询
	// 69.8s 爆炸（大命中集 O(N×M)），CJK 门控消解（中文剧名命中集天然小）。
	// snapQuery 不再重复 search 过滤——代表行组键=name，sub 已筛组（§59.29 双保险收敛）。
	// sub/hashQ 两查询点必须同域——漏一处列表假阴性（§59.28 H1 同族）。
	searchHasCJK := false
	for _, r := range search {
		if unicode.Is(unicode.Han, r) {
			searchHasCJK = true
			break
		}
	}
	var searchConds []string
	var searchArgs []interface{}
	if search != "" {
		if searchHasCJK {
			searchConds = []string{"name LIKE ?", "OR name IN (SELECT s2.name FROM torrent_snapshots s2 JOIN torrent_metadata m ON m.info_hash = s2.hash WHERE m.title LIKE ? OR m.subtitle LIKE ?)"}
			searchArgs = []interface{}{"%" + search + "%", "%" + search + "%", "%" + search + "%"}
		} else {
			searchConds = []string{"name LIKE ?"}
			searchArgs = []interface{}{"%" + search + "%"}
		}
		sub = sub.Where(strings.Join(searchConds, " "), searchArgs...)
	}
	var rawSnapshots []model.TorrentSnapshot
	if err := snapQuery.
		Where("s.id IN (?)", sub).
		Order("s.updated_at DESC").
		Find(&rawSnapshots).Error; err != nil {
		Success(w, map[string]interface{}{"items": []interface{}{}, "total": 0})
		return
	}

	// §59.26: 按 name 去重（SQL 分组已去重，此处为双保险 + 保序语义）。
	// §59.29: 全部视图下 key 为 client+path+name（不同下载器/路径的同名资源是不同实体）
	snapshots := util.DedupByKey(rawSnapshots, func(s model.TorrentSnapshot) string {
		if s.Name != "" {
			return fmt.Sprintf("%d|%s", s.ClientUID, s.SavePath) + "|" + s.Name
		}
		return s.Hash
	})

	if len(snapshots) == 0 {
		Success(w, map[string]interface{}{"items": []interface{}{}, "total": 0})
		return
	}

	// 2. 收集 hash → 查 metadata（§59.29 性能：2.2w hash 的 IN 查询过慢，
	// 改为子查询 JOIN——同簇资源的所有站点 hash 的 metadata 一并取出，
	// 解决去重保留行 hash 与 metadata 行 hash 不一致时丢数据的问题（§59.28 H1））
	// §59.316 附 C: 域收紧 name → 紧键 (client,path,name)——与 §59.44 资源视图
	// 权威口径统一；同名跨下载器/跨路径簇不再互相借数据（假 ready 消除）。
	// 同簇内跨站 hash 三元组一致不受影响（H1/§59.38 语义保持）。
	// 回归审核修复: 行值 IN 用【值列表展开】在 2.4w 簇规模下 3 占位符/元组
	// 超 mattn SQLITE_MAX_VARIABLE_NUMBER(32766)——"too many SQL variables"
	// 且错误被吞=全表静默 unfetched（29 实证 rows:0）。改保留行 ID 子查询
	// （参数量=len(snapshots)，与旧 name IN 同量级同上限）。
	snapIDs := make([]uint, 0, len(snapshots))
	for _, s := range snapshots {
		snapIDs = append(snapIDs, s.ID)
	}
	keySub := h.db.WithContext(r.Context()).
		Table("torrent_snapshots").
		Select("client_uid, save_path, name").
		Where("id IN ?", snapIDs)
	var metas []model.TorrentMetadata
	if err := h.db.WithContext(r.Context()).
		Where("info_hash IN (SELECT hash FROM torrent_snapshots WHERE (client_uid, save_path, name) IN (?) AND is_hidden = 0)", keySub).
		Find(&metas).Error; err != nil {
		if h.logger != nil {
			h.logger.Error("listSeeds metas join failed", zap.Error(err))
		}
		metas = nil
	}

	// §59.469 附七: 音乐 hash 批量预查（音乐种审核门口径——classifySeedStatusLite
	// 的 isMusicSeed 输入）。LIKE 模式与 detailSourceHasMusic 理论等价：Music
	// 非 nil 序列化必有 "music":{ ·omitempty nil 不序列化 ·BBCode 文本出现该
	// 模式概率≈0。只回轻量列——7615 行零 JSON 载入（metas 查询轻量化
	// §59.29 不带 detail_source_json 列）。
	// §59.469 附九: 扩展 json_extract 取 artist/album（字段列音乐行三项红绿标
	// 消费·与审核门方案 B 同构）。
	type musicRow struct {
		InfoHash string
		Artist   string
		Album    string
	}
	var musicRows []musicRow
	musicMeta := make(map[string]musicRow)
	if len(metas) > 0 {
		hashes := make([]string, 0, len(metas))
		for _, m := range metas {
			hashes = append(hashes, m.InfoHash)
		}
		if err := h.db.WithContext(r.Context()).Model(&model.TorrentMetadata{}).
			Select("info_hash, json_extract(detail_source_json, '$.music.artist') AS artist, json_extract(detail_source_json, '$.music.album') AS album").
			Where("info_hash IN ? AND detail_source_json LIKE ?", hashes, `%"music":{%`).
			Scan(&musicRows).Error; err != nil && h.logger != nil {
			// 降级语义：集合空=音乐种全按影视 9 字段判（现状行为·可接受）——
			// 但不可静默（空态三问第 3 问）
			h.logger.Warn("listSeeds music hash precheck failed, fallback to video gate", zap.Error(err))
		}
		for _, row := range musicRows {
			musicMeta[row.InfoHash] = row
		}
	}

	// 3. 按紧键三元组关联 metadata（§59.29: 同簇所有 hash 的 metadata 行，
	// 解决去重保留行与 metadata 行 hash 不一致时丢数据 §59.28 H1）
	// §59.38 修复: tripleByHash 域放宽到 (client,path) 全部活跃行（原 rawSnapshots
	// 是分组去重保留行——同资源多变体场景 metadata 挂在非保留行上时映射丢失，
	// 列表假阴性标 unfetched。H1 只放宽了 metas 查询没放宽映射域，半修）
	// §59.316 附 C: 键值 name → triple key（与 copyCountByKey 同格式）
	tripleByHash := make(map[string]string, len(rawSnapshots)*2)
	// §59.521: 簇内 tracker 域名聚合——「已有站点」列换做种面（tracker 归属）。
	// 原 metadata 行 site_name 集合失真（簇行复制代表行 site_name·Flight 案
	// 14 站在种只显示 1 站）；tracker 域名是快照扫描持久化的真实归属
	// （243 实测填充率 100%）。零新增查询——hashQ 顺路多取一列。
	domainByCluster := make(map[string][]string, len(rawSnapshots)*2)
	{
		var hashKeyRows []struct {
			Hash          string
			ClientUID     uint
			SavePath      string
			Name          string
			TrackerDomain string
		}
		hashQ := h.db.WithContext(r.Context()).
			Table("torrent_snapshots").
			Select("hash, client_uid, save_path, name, tracker_domain").
			Where("is_hidden = ? AND name != ''", false)
		if clientUID != 0 {
			hashQ = hashQ.Where("client_uid = ?", clientUID)
		}
		if savePath != "" {
			hashQ = hashQ.Where("save_path = ?", savePath)
		}
		if search != "" {
			// §59.138: tripleByHash 域同步搜索域（CJK 三域/拉丁 name——漏则假阴性）
			hashQ = hashQ.Where(strings.Join(searchConds, " "), searchArgs...)
		}
		hashQ.Find(&hashKeyRows)
		for _, r := range hashKeyRows {
			key := fmt.Sprintf("%d|%s|%s", r.ClientUID, r.SavePath, r.Name)
			tripleByHash[r.Hash] = key
			if r.TrackerDomain != "" {
				domainByCluster[key] = append(domainByCluster[key], r.TrackerDomain)
			}
		}
	}
	metaByCluster := make(map[string][]model.TorrentMetadata, len(snapshots))
	for _, m := range metas {
		if key, ok := tripleByHash[m.InfoHash]; ok {
			metaByCluster[key] = append(metaByCluster[key], m)
		}
	}

	// §59.547 三态筛选·覆盖面预查 + 四色 tag 数据（选站上下文）：
	// 三面并集（记录∪覆盖∪tracker）与执行层 CoveredSitesOfCluster 同口径——
	// 列表说可发布的种子发布时永不撞跳过（列表即真相）。tracker 面零预查
	// （循环内 siteSet 现成）；覆盖面按当前域 hash 分批查覆盖表（500/批·
	// SQLite IN 变量上限）；ours/covered 为四色数据（簇级站点属性集合·
	// 用户四色定案：绿=我们发/蓝=选中站别人发/黑=有副本/黄=站上有无副本）。
	coverageTargetClusters := map[string]bool{}      // 簇键→目标站覆盖面命中（筛选面2）
	oursByCluster := map[string]map[string]bool{}    // 簇键→我们发过的站集合（四色绿）
	coveredByCluster := map[string]map[string]bool{} // 簇键→覆盖面站集合（四色黄）
	if publishTargetSite != "" {
		domainHashes := make([]string, 0, len(tripleByHash))
		for h := range tripleByHash {
			domainHashes = append(domainHashes, h)
		}
		if len(domainHashes) > 0 {
			type covRow struct {
				InfoHash string
				SiteName string
			}
			var covRows []covRow
			for _, batch := range util.ChunkStrings(domainHashes, 500) {
				var br []covRow
				h.db.WithContext(r.Context()).
					Table("site_coverage_cache").
					Select("info_hash, site_name").
					Where("info_hash IN ? AND status IN ?", batch,
						model.CoverageHasStatuses).
					Find(&br)
				covRows = append(covRows, br...)
			}
			for _, cr := range covRows {
				key := tripleByHash[cr.InfoHash]
				if key == "" {
					continue
				}
				if coveredByCluster[key] == nil {
					coveredByCluster[key] = map[string]bool{}
				}
				coveredByCluster[key][cr.SiteName] = true
				if cr.SiteName == publishTargetSite {
					coverageTargetClusters[key] = true
				}
			}
			// 四色绿：我们发过的站（发布事实口径=PublishSkipStatuses 五终态——
			// §59.547 回归审核修正：原用 PublishedListStatuses 六终态·existing
			// （文案信任·没发上去）会误标绿——绿语义=「我们发的」必须是发上去了）
			type oursRow struct {
				SourceInfoHash string
				TargetSite     string
			}
			var oursRows []oursRow
			for _, batch := range util.ChunkStrings(domainHashes, 500) {
				var br []oursRow
				h.db.WithContext(r.Context()).
					Table("publish_result_records").
					Select("source_info_hash, target_site").
					Where("source_info_hash IN ? AND target_site != '' AND status IN ?",
						batch, model.PublishSkipStatuses).
					Find(&br)
				oursRows = append(oursRows, br...)
			}
			for _, or := range oursRows {
				key := tripleByHash[or.SourceInfoHash]
				if key == "" {
					continue
				}
				if oursByCluster[key] == nil {
					oursByCluster[key] = map[string]bool{}
				}
				oursByCluster[key][or.TargetSite] = true
			}
		}
	}

	// §59.131 ②: 簇副本数——发布页（一种多站）增量字段。
	// 按 (client,path,name) 聚合活跃快照行数；不施加 search 过滤（簇成员数与
	// 搜索无关，search 只决定哪些簇可见）。name='' 行不入此表，副本数退化 1。
	type clusterCountRow struct {
		ClientUID uint
		SavePath  string
		Name      string
		Cnt       int
	}
	var countRows []clusterCountRow
	countQ := h.db.WithContext(r.Context()).
		Table("torrent_snapshots").
		Select("client_uid, save_path, name, COUNT(*) AS cnt").
		Where("is_hidden = ? AND name != ''", false)
	if clientUID != 0 {
		countQ = countQ.Where("client_uid = ?", clientUID)
	}
	if savePath != "" {
		countQ = countQ.Where("save_path = ?", savePath)
	}
	countQ.Group("client_uid, save_path, name").Find(&countRows)
	copyCountByKey := make(map[string]int, len(countRows))
	for _, cr := range countRows {
		copyCountByKey[fmt.Sprintf("%d|%s|%s", cr.ClientUID, cr.SavePath, cr.Name)] = cr.Cnt
	}

	// 4. 组装结果（§59.29 性能：轻量状态标注全量执行——flags/映射/reviewed 纯内存或
	// 带缓存查询；compliance（含 per-row DB 查询）延迟到分页后的当前页执行，
	// 未筛选 system_forbidden 时默认降级为轻量判定，筛该态时对全量补跑）
	// §59.489: 簇级音乐判定（compliance 分流输入——音乐不存在成人内容·
	// 用户定案）。musicMeta 批量预查（§59.469 附七）+metaByCluster 簇关联：
	// 簇内任一行有 music 即音乐种（代表行可能无 music 而簇内副本有——
	// §59.484 同型跨行残留防御）。
	clusterIsMusic := func(item map[string]interface{}) bool {
		key := fmt.Sprintf("%d|%s|%s", item["client_id"], item["save_path"], item["name"])
		for _, m := range metaByCluster[key] {
			if _, ok := musicMeta[m.InfoHash]; ok {
				return true
			}
		}
		return false
	}
	needFullCompliance := statusFilter == "system_forbidden"
	// §59.521: 「已有站点」tracker 匹配器单例（循环内逐行 New 会重查 sites 表）
	var seedTrackerMatcher *site.TrackerMatcher
	items := make([]map[string]interface{}, 0, len(snapshots))
	for _, snap := range snapshots {
		item := map[string]interface{}{
			"hash":      snap.Hash,
			"name":      snap.Name,
			"size":      snap.Size,
			"client_id": snap.ClientUID,
			"save_path": snap.SavePath,
		}

		// §59.316 附 C: 紧键三元组（map 键与 metas 子查询同口径）
		clusterKey := fmt.Sprintf("%d|%s|%s", snap.ClientUID, snap.SavePath, snap.Name)

		// 查找源站行 metadata（按紧键关联，含同簇兄弟 hash 的行）
		var meta *model.TorrentMetadata
		if metas, ok := metaByCluster[clusterKey]; ok && len(metas) > 0 {
			meta = h.selectSourceMeta(metas)
		}

		if meta != nil {
			item["site_name"] = meta.SiteName
			item["title"] = meta.Title
			item["subtitle"] = meta.Subtitle
			item["poster"] = meta.Poster
			item["reviewed"] = meta.Reviewed
			item["flags"] = meta.Flags
			item["source_category"] = meta.SourceCategory
			item["fetch_source"] = meta.FetchSource
			// §59.171 D: MI 口径对齐设计（本地优先+源站兜底）——双列任一非空即"有 MI"。
			// 原只查 media_info（本地列）："仅源站 MI"行 Tab5 能显示（消费侧兜底生效）、
			// 列表却报红叉——口径自相矛盾。
			// §59.319 P1: BDInfo 计入完备性（原盘行 media_info 已清空）+ is_disc
			// 供前端徽标显示 BD（D7 闭环：空 BDInfo 原盘红叉拦截）
			item["has_mediainfo"] = meta.MediaInfo != "" || meta.SourceMediaInfo != "" || meta.BDInfo != ""
			item["is_disc"] = meta.BDInfo != ""
			// §59.319 附十八: 扫盘中提示——原盘簇 bd_info 空+簇内有快照+本地
			// 下载器=扫描进行中（D6 自动入队后 5-11 分钟窗口；清除后重获/存量
			// 补扫期间用户可见"视频文件分析中"而非误以为系统停摆）。is_disc
			// false+disc_scan_pending true 并存是常态（is_disc 以 bd_info 落库
			// 为准，扫描完成前两者同时可见）
			// §59.361 A: 三态修正——有失败记录的是"失败"非"扫描中"（pt29 案：
			// 30min 超时失败 24h+ 后蓝圈仍永转——推断值不反映任务真实状态）；
			// pending 仅在无失败记录时为真（未扫/扫描中——精确区分需查队列，
			// 列表频查队列成本高，失败态与扫描中的界分已覆盖本案痛点）。
			discOnDisk := h.bdinfoScanner != nil &&
				h.bdinfoScanner.DetectDiscPath(snap.SavePath, snap.Name) != ""
			item["disc_scan_failed"] = meta.BDScanFailedAt != nil && meta.BDInfo == ""
			item["disc_scan_error"] = meta.BDScanError
			if meta.BDScanFailedAt != nil {
				item["disc_scan_failed_at"] = meta.BDScanFailedAt
			}
			item["disc_scan_pending"] = meta.BDInfo == "" && discOnDisk &&
				meta.BDScanFailedAt == nil
			item["has_description"] = meta.Description != ""
			item["has_screenshots"] = meta.Screenshots != ""
			item["fetched_at"] = meta.FetchedAt
			// 技术参数（列表展示用）
			item["resolution"] = meta.Resolution
			item["video_codec"] = meta.VideoCodec
			item["audio_codec"] = meta.AudioCodec
			item["audio_channels"] = meta.AudioChannels
			item["audio_tech"] = meta.AudioTech
			item["hdr"] = meta.HDR
			item["source_type"] = meta.SourceType
			item["specification"] = meta.Specification
			// §59.226 附七: Encode 派生单点化——列表页此前手工构造 mini profile
			// （只填 3 字段）MI 铁证输入恒 false（MIEncoded/MIHasVideo 零值）——
			// 与详情页（完整 profile 含 MI）同种子判定可能打架。改为：MI 本地列
			// 存在时解析出铁证输入，与详情页同源。
			// §59.319 P1: 原盘行 BDInfo 优先（Encoded 恒 false 铁证）
			encProfile := titleparser.TechProfile{
				SourceType:    meta.SourceType,
				Specification: meta.Specification,
				VideoCodec:    meta.VideoCodec,
				// §59.319 附三十二: RawTitle 必传（同 displayProfile——
				// IsEncode 重编码标记排除扫原始标题）
				RawTitle: meta.Title,
			}
			miText := meta.BDInfo
			if miText == "" {
				miText = meta.MediaInfo
			}
			if miText != "" {
				miTech := titleparser.ExtractMediaInfo(miText)
				encProfile.MIEncoded = miTech.Encoded
				encProfile.MIHasVideo = miTech.Encoded || miTech.Resolution != "" || miTech.VideoCodec != ""
				// §59.517: WEB 族引擎裁决同步（与 BuildTechProfile 同源——
				// IsEncode WEB 豁免消费）
				encProfile.MIWebVerdict = miTech.WEBVerdict
				encProfile.MIWebRule = miTech.WEBRule
				// §59.319 附十六: BDInfo 输入=原盘铁证（与 BuildTechProfile 同源判定）
				if strings.Contains(miText, "QUICK SUMMARY:") {
					encProfile.FromBDInfo = true
				}
			}
			item["encode"] = titleparser.IsEncode(encProfile)
			item["category"] = meta.Category
			item["form"] = meta.Form
		} else {
			item["reviewed"] = false
			item["fetched"] = false
			item["title"] = ""
		}

		// 5 态标注（轻量：跳过 compliance）
		// §59.469 附七: 音乐 hint（批量预查集·meta nil=unfetched 行恒 false）
		mr := musicRow{}
		if meta != nil {
			mr = musicMeta[meta.InfoHash]
		}
		isMusic := mr.InfoHash != ""
		status := h.classifySeedStatusLite(r.Context(), snap.Name, meta, isMusic)
		item["status"] = status
		// §59.469 附八: 音乐种行标记（SeedConfig 主入口按钮文案/分流消费）
		item["is_music"] = isMusic
		// §59.469 附九: artist/album（字段列音乐行红绿标·方案 1a）
		if isMusic {
			item["music_artist"] = mr.Artist
			item["music_album"] = mr.Album
		}

		// §59.131 ②: 副本数 + 站点列表（簇内已有站点——发布页"已存在"标注数据源）
		cc := copyCountByKey[fmt.Sprintf("%d|%s|%s", snap.ClientUID, snap.SavePath, snap.Name)]
		if cc == 0 {
			cc = 1
		}
		item["copy_count"] = cc
		// §59.521: 已有站点=簇内做种面（tracker 归属·trackerMatcher.Match）。
		// 原 metadata 行 site_name 集合失真（簇行建行复制代表行 site_name——
		// §59.519 Flight 案 14 站在种只显示「朋友」1 站）；tracker 域名=快照
		// 持久化的真实归属（与 .torrent comment 教训同源：归属看 tracker）。
		// trackerMatcher 函数级单例（循环内逐行 New 会每行重查 sites 表）。
		if seedTrackerMatcher == nil {
			seedTrackerMatcher = site.NewTrackerMatcher(h.db)
		}
		siteSet := make(map[string]bool)
		if seedTrackerMatcher != nil {
			for _, d := range domainByCluster[clusterKey] {
				if sn := seedTrackerMatcher.Match(d); sn != "" {
					siteSet[sn] = true
				}
			}
		}
		sites := make([]string, 0, len(siteSet))
		for s := range siteSet {
			sites = append(sites, s)
		}
		sort.Strings(sites)
		item["sites"] = sites

		// §59.547 四色 tag 数据（选站上下文·target_site 在场）——前端着色消费：
		//   选中站（置顶）：绿=ours（我们发的·永久） / 蓝=非 ours（别人发的）
		//   非选中站：     绿=ours / 黑=seeded（有副本·做种/已辅种） / 黄=covered
		//                 （站上有+无副本=辅种机会）
		// 三面任一命中才渲染选中站 tag（可发布行=三面 miss=选中站无 tag）；
		// 覆盖面发现的站（不在做种面）注入（黄 tag 场景——别人发的站我们
		// 通常无副本·sites 列天然无此站名）。
		if publishTargetSite != "" {
			type siteTagRow struct {
				Name    string `json:"name"`
				Ours    bool   `json:"ours"`
				Seeded  bool   `json:"seeded"`
				Covered bool   `json:"covered"`
			}
			tags := make([]siteTagRow, 0, len(sites)+2)
			seen := map[string]bool{}
			addTag := func(name string) {
				if name == "" || seen[name] {
					return
				}
				seen[name] = true
				tags = append(tags, siteTagRow{
					Name:    name,
					Ours:    oursByCluster[clusterKey][name],
					Seeded:  siteSet[name],
					Covered: coveredByCluster[clusterKey][name],
				})
			}
			if publishedClusters[clusterKey] || coverageTargetClusters[clusterKey] || siteSet[publishTargetSite] {
				addTag(publishTargetSite) // 三面命中→选中站置顶第一位
			}
			for _, s := range sites {
				addTag(s)
			}
			for s := range coveredByCluster[clusterKey] {
				addTag(s)
			}
			for s := range oursByCluster[clusterKey] {
				addTag(s)
			}
			item["site_tags"] = tags
		}

		// §59.143: 发布页隐藏源站禁转簇（种子配置页不传此参数——数据维护视角可见）
		if excludeForbidden && status == "forbidden" {
			continue
		}

		// §59.131 ②: ready 过滤（data_ready=reviewed 口径，R3-3）
		if readyFilter != "" {
			isReady := meta != nil && meta.Reviewed
			if readyFilter == "true" && !isReady {
				continue
			}
			if readyFilter == "false" && isReady {
				continue
			}
		}

		// §59.166 一站多种：发布维度过滤（publishable 自含 reviewed——三态筛选
		// 无独立 ready 选项；published=历史事实回溯）
		// §59.547: 三面并集（记录∪覆盖∪tracker·与执行层 CoveredSitesOfCluster
		// 同口径）——tracker 面循环内 siteSet 现成零预查；列表说可发布的种子
		// 发布时永不撞跳过（列表即真相·Hatfields 案教训：两层口径差制造惊吓）。
		if (publishState == "publishable" || publishState == "published") && publishTargetSite != "" {
			isHas := publishedClusters[clusterKey] ||
				coverageTargetClusters[clusterKey] ||
				siteSet[publishTargetSite]
			if publishState == "published" && !isHas {
				continue
			}
			if publishState == "publishable" {
				if isHas || meta == nil || !meta.Reviewed {
					continue
				}
			}
		}

		// §59.29: 后端过滤（状态 + 搜索），与分页解耦
		if statusFilter != "" && statusFilter != "system_forbidden" {
			if statusFilter == "issues" {
				// 复合态：禁转/系统禁转/无映射
				if status != "forbidden" && status != "no_mapping" {
					continue
				}
			} else if status != statusFilter {
				continue
			}
		}
		if search != "" {
			sLower := strings.ToLower(search)
			// §59.138: Go 层双保险同步三域（name/title/subtitle）
			subtitle := ""
			if meta != nil {
				subtitle = meta.Subtitle
			}
			if !strings.Contains(strings.ToLower(snap.Name), sLower) &&
				!strings.Contains(strings.ToLower(item["title"].(string)), sLower) &&
				!strings.Contains(strings.ToLower(subtitle), sLower) {
				continue
			}
		}

		items = append(items, item)
	}

	// §59.29: 筛 system_forbidden 时对全量补跑 compliance（结果集已缩小）
	if needFullCompliance {
		filtered := items[:0]
		for _, item := range items {
			name, _ := item["name"].(string)
			siteName, _ := item["site_name"].(string)
			if r2 := h.complianceChecker.CheckWithSite(r.Context(), name, siteName, clusterIsMusic(item)); r2 != nil && !r2.Passed {
				item["status"] = "system_forbidden"
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}

	total := int64(len(items))

	// 去重+过滤后再分页
	start := (page - 1) * pageSize
	if start > len(items) {
		items = nil
	} else {
		end := start + pageSize
		if end > len(items) {
			end = len(items)
		}
		items = items[start:end]
	}

	// §59.29: 当前页补跑 compliance（未筛选时全量循环已跳过）——
	// 只影响本页状态标注精度，50 行可接受
	if !needFullCompliance && h.complianceChecker != nil {
		for _, item := range items {
			name, _ := item["name"].(string)
			siteName, _ := item["site_name"].(string)
			status, _ := item["status"].(string)
			// 轻量态非禁转/无映射的行才需要 compliance 精化
			if status != "forbidden" && status != "no_mapping" {
				if r2 := h.complianceChecker.CheckWithSite(r.Context(), name, siteName, clusterIsMusic(item)); r2 != nil && !r2.Passed {
					item["status"] = "system_forbidden"
				}
			}
		}
	}

	Success(w, map[string]interface{}{
		"items": items,
		"total": total,
	})
}

// handleSeedUniquePaths §59.29: 返回有快照的 下载器→路径 树（筛选弹层数据源）。
func (h *PublishTorrentsHandler) handleSeedUniquePaths(w http.ResponseWriter, r *http.Request) {
	type pathRow struct {
		ClientUID uint
		SavePath  string
		Count     int64
	}
	var rows []pathRow
	h.db.WithContext(r.Context()).Model(&model.TorrentSnapshot{}).
		Select("client_uid, save_path, COUNT(*) as count").
		Where("is_hidden = ?", false).
		Where("progress >= ?", 1.0). // §59.537: 完成度边界同列表
		Group("client_uid, save_path").
		Order("client_uid, save_path").
		Find(&rows)

	var allClients []model.ClientConfig
	h.db.WithContext(r.Context()).Select("id, name").Find(&allClients)
	clientNames := make(map[uint]string, len(allClients))
	for _, c := range allClients {
		clientNames[c.ID] = c.Name
	}

	clients := make([]map[string]interface{}, 0, len(rows))
	byClient := map[uint]*map[string]interface{}{}
	for _, row := range rows {
		c, ok := byClient[row.ClientUID]
		if !ok {
			entry := map[string]interface{}{
				"client_id": row.ClientUID,
				"name":      clientNames[row.ClientUID],
				"paths":     make([]map[string]interface{}, 0, 4),
			}
			byClient[row.ClientUID] = &entry
			clients = append(clients, entry)
			c = &entry
		}
		(*c)["paths"] = append((*c)["paths"].([]map[string]interface{}), map[string]interface{}{
			"save_path": row.SavePath,
			"count":     row.Count,
		})
	}

	Success(w, map[string]interface{}{"clients": clients})
}

// selectSourceMeta 从同一 hash 的多行 metadata 中选出源站行（制作组映射命中优先，否则 updated_at 最新）。
func (h *PublishTorrentsHandler) selectSourceMeta(metas []model.TorrentMetadata) *model.TorrentMetadata {
	if len(metas) == 0 {
		return nil
	}
	// §59.171: 确定性排序——排序即选择（权威行=非 cluster 传播副本优先 → 新者优先
	// → 小 id 优先）。消除无序查询抽签（PT31 MI 红叉实锤：item hash 挂着有 MI 的
	// 源行，却选中 cluster 空行）。原 best=updated_at 全量扫描已被废弃：传播会
	// bump 副本 updated_at（reviewed/截图二波），"最新"常是副本行——权威优先级
	// 必须高于新旧。
	// §59.481: 排序前置到 music-first 之前——音乐行集合内仍按权威序选（原
	// 无序扫首个音乐行=簇传播副本与权威源行抽签·Ultimate Box Set 案展示
	// 家园副本行而非皇后源行）。
	model.SortMetasAuthoritative(metas)
	// §59.476: music 在场行优先（缺口④第二例实锤——12 环境 Tiesto 簇三行
	// 竞争·车站韩剧串种行（影视字段全）完爆音乐行·MDP/展示恒错）。
	// 音乐字段仅音乐链产出（皇后纯音乐站+验证层双保证）→ 影视簇 no-op。
	for i := range metas {
		if detailSourceHasMusic(metas[i].DetailSourceJSON) {
			return &metas[i]
		}
	}
	ctx := context.Background()
	best := &metas[0]
	// 尝试找制作组映射命中行（排序后首个命中=权威最新的匹配行）
	if h.sourceDetector != nil {
		for i := range metas {
			group := publish.ExtractGroupName(metas[i].Title)
			if group != "" {
				site := h.sourceDetector.LookupGroup(ctx, group)
				if site == metas[i].SiteName {
					return &metas[i]
				}
			}
		}
	}
	return best
}

// classifySeedStatusLite §59.29: 轻量状态标注（跳过 compliance 的 per-row DB 查询）。
// 全量循环用；compliance 由调用方对当前页（或筛选 system_forbidden 时对全量）补跑。
// §59.469 附七: isMusicSeed 由列表层批量预查传入（metas 查询轻量化不带
// detail_source_json 列·内部判定恒空——防列表/getSeed 审核门口径分裂）。
func (h *PublishTorrentsHandler) classifySeedStatusLite(ctx context.Context, name string, meta *model.TorrentMetadata, isMusicSeed bool) string {
	// ① flags 检查
	if meta != nil && meta.Flags != "" {
		var flags []string
		if err := json.Unmarshal([]byte(meta.Flags), &flags); err == nil {
			forbiddenSet := map[string]bool{
				"禁转": true, "禁止转载": true, "谢绝转载": true, "严禁转载": true,
				"谢绝搬运": true, "独占": true, "限转": true,
			}
			for _, f := range flags {
				if forbiddenSet[f] {
					return "forbidden"
				}
				// §59.162: 限时禁转两态——now<until 才拦（keepfrds 24h 窗到期自动放行）
				if f == "限时禁转" {
					if meta.NoTransferUntil == nil || meta.NoTransferUntil.After(time.Now()) {
						return "forbidden"
					}
				}
			}
		}
	}

	// ③ 制作组映射检查 §59.230: 仅对未获取种子有意义（无映射=获取通道提醒——
	// 已获取种子该状态双重过时：a)簇 comment/coverage 兜底已获取成功 b)站方标题
	// 可能带组名（PERFUME 案：tr 裸名无组名判 no_mapping，获取后织梦标题
	// 实为 -CMCT 官组种有映射——本地裸名误判被站方标题纠正）。
	// 已获取种子用站方标题（meta.Title）重判映射——站方标题是权威
	// （发布/审核消费的就是它）。
	// §59.469 附七: 音乐种跳过组映射——音乐标题通常无组名（§59.469 定案
	// 表述）·音乐链已定向音乐源站（coverage/簇判定）·组映射是影视获取通道
	// 语义；不跳过则已审核音乐种恒 no_mapping（许景淳案：③ 早退优先于
	// ④ reviewed——审核态被组映射遮蔽）。
	if h.sourceDetector != nil && !isMusicSeed {
		if meta == nil {
			group := publish.ExtractGroupName(name)
			if group == "" {
				return "no_mapping"
			}
			site := h.sourceDetector.LookupGroup(ctx, group)
			if site == "" {
				return "no_mapping"
			}
		} else if meta.Title != "" {
			group := publish.ExtractGroupName(meta.Title)
			if group != "" {
				site := h.sourceDetector.LookupGroup(ctx, group)
				if site == "" {
					// 站方标题有组名但无映射——真无映射（比裸名可信）
					return "no_mapping"
				}
			}
			// 站方标题无组名（电影等）或有映射——均按获取后状态流展示
		}
	}

	// 无 metadata → 未获取（映射检查已前置处理 no_mapping）
	if meta == nil {
		return "unfetched"
	}

	// ④ reviewed=true
	if meta.Reviewed {
		return "reviewed"
	}

	// ⑤⑥ 9 字段校验
	missing := h.checkRequiredFields(meta, isMusicSeed)
	if len(missing) > 0 {
		return "incomplete"
	}
	return "pending"
}

// handleAuditInfoHash §59.59 审计: 诊断端点——下载指定站点的 .torrent 并计算
// infohash 与期望值比对（获取链路正确性的确定性判据）。独立于获取链，不写库。
func (h *PublishTorrentsHandler) handleAuditInfoHash(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Site       string `json:"site"`
		TorrentID  string `json:"torrent_id"`
		ExpectHash string `json:"expect_hash"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.Site == "" || req.TorrentID == "" {
		Error(w, http.StatusBadRequest, 40001, "site/torrent_id 必填")
		return
	}
	if h.siteProvider == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "site provider 未配置")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	config, err := h.siteProvider.GetSiteConfig(ctx, req.Site)
	if err != nil || config == nil {
		Error(w, http.StatusBadGateway, 50201, fmt.Sprintf("获取站点配置失败: %v", err))
		return
	}
	adapter, err := h.siteProvider.GetAdapter(ctx, req.Site)
	if err != nil || adapter == nil {
		Error(w, http.StatusBadGateway, 50202, fmt.Sprintf("获取适配器失败: %v", err))
		return
	}
	data, err := adapter.DownloadTorrent(ctx, config, req.TorrentID)
	if err != nil {
		Error(w, http.StatusBadGateway, 50203, fmt.Sprintf("下载 .torrent 失败: %v", err))
		return
	}
	meta, err := fingerprint.ComputeFromTorrent(data)
	if err != nil || meta == nil {
		Error(w, http.StatusInternalServerError, 50002, fmt.Sprintf("解析 .torrent 失败: %v", err))
		return
	}
	noSrc, _ := fingerprint.ComputeNoSourceHash(data)
	Success(w, map[string]interface{}{
		"site":                req.Site,
		"torrent_id":          req.TorrentID,
		"info_hash":           meta.InfoHash,
		"expect_hash":         req.ExpectHash,
		"match":               strings.EqualFold(meta.InfoHash, req.ExpectHash),
		"info_hash_no_source": noSrc,
	})
}

// musicFetchArgs §59.474: 音乐链 FetchAndStoreBySearch 变参构造——
// 影视：[localMI, suffix]；音乐：[localMI, suffix, "music"]（isMusicSeed
// 约定值·fetcher 内解析）。
func (h *PublishTorrentsHandler) musicFetchArgs(musicRouting bool, args ...string) []string {
	if !musicRouting {
		return args
	}
	return append(args, "music")
}

// detailSourceHasMusic §59.469 附七: 音乐种判定（单颗场景精确通道——
// UnmarshalDetailSource 判 Music 非 nil）。列表批量场景走 listSeeds 的
// LIKE 预查（等价性见该处注释）。
func detailSourceHasMusic(detailSourceJSON string) bool {
	return metadata.HasMusic(detailSourceJSON) // §59.489 提升公共
}

// checkRequiredFields §59.20: 9 必需字段校验，返回缺失字段名列表。
// §59.469 附七: isMusicSeed=true 走音乐核心字段校验（title+artist+album
// 三非空——质量门·fetch 后必在不挡路）；format/poster/description 不进门
// （单源自足：乌托邦 desc 天然空）。免影视 9 字段（screenshots/mediainfo/
// resolution/video_codec/release_group 音乐种天然没有——生产实证音乐种
// 恒 review 不了→MDP 入口被挡死）。
func (h *PublishTorrentsHandler) checkRequiredFields(meta *model.TorrentMetadata, isMusicSeed bool) []string {
	var missing []string
	if meta.Title == "" {
		missing = append(missing, "title")
	}
	if isMusicSeed {
		if src, err := metadata.UnmarshalDetailSource(meta.DetailSourceJSON); err == nil && src != nil && src.Music != nil {
			if src.Music.Artist == "" {
				missing = append(missing, "artist")
			}
			if src.Music.Album == "" {
				missing = append(missing, "album")
			}
			return missing
		}
		// Music 解析意外失败：回退影视 9 字段（宁严勿松）
	}
	if meta.Poster == "" {
		missing = append(missing, "poster")
	}
	// §59.53: 门槛升级——非空 → 解析后 < MinScreenshots 即缺失（凑数+审核双保险）
	if len(model.ParseScreenshotColumn(meta.Screenshots)) < publish.MinScreenshots {
		missing = append(missing, "screenshots")
	}
	if meta.Description == "" {
		missing = append(missing, "description")
	}
	// §59.171 D: MI 校验口径对齐设计（本地优先+源站兜底）——双列任一非空即通过。
	// 原只查 MediaInfo："仅源站 MI"行九字段校验永缺 → reviewed 上不去（与列表
	// 红叉同族误判）。
	// §59.319 附五：BDInfo 计入——原盘行 D8 伪 MI 停产后双列恒空、仅 bd_info
	// 非空，漏此口径则原盘恒缺 mediainfo 审核失败（用户清除重获实测捕获；
	// 与列表 has_mediainfo 三列口径一致）。
	if meta.MediaInfo == "" && meta.SourceMediaInfo == "" && meta.BDInfo == "" {
		missing = append(missing, "mediainfo")
	}
	if meta.Resolution == "" {
		missing = append(missing, "resolution")
	}
	if meta.VideoCodec == "" {
		missing = append(missing, "video_codec")
	}
	if meta.AudioCodec == "" {
		missing = append(missing, "audio_codec")
	}
	// 制作组从标题解析
	if h.sourceDetector != nil {
		if g := publish.ExtractGroupName(meta.Title); g == "" {
			missing = append(missing, "release_group")
		}
	}
	return missing
}

// mainlineQueryKeys §59.285→§59.286: 主链 PTGen 三级查询键列表
// （DoubanURL→IMDbURL→Title，空键跳过——顺序链逐级尝试成功即短路）。
func mainlineQueryKeys(douban, imdb, title string) []string {
	var keys []string
	for _, k := range []string{douban, imdb, title} {
		if k != "" {
			keys = append(keys, k)
		}
	}
	return keys
}

// extractSeedHash 从 URL 路径 /api/v1/publish/seeds/:info_hash 提取 info_hash。
func extractSeedHash(r *http.Request) string {
	path := strings.TrimRight(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// handleGetSeed §59.20: 读取单个种子 metadata（GET /publish/seeds/:info_hash）。
// 返回 DB 14 平铺字段 + ParseTitleTech 解析 5 字段 = 完整 18 TechProfile + 编辑字段。
// runMainlinePTGen §59.236 ①: 主链必经 PTGen 查询——豆瓣链接 → 端点查询 →
// ptgen_source_json（唯一账本）+ desc=RawBBCode。失败全空（incomplete 承接）。
// 站方原文不保存（desc=我们的 RawBBCode 或空——§59.236 定案）。
// force=true 绕缓存（单条重获 §59.173）/false 普通缓存（批量——§59.236 决策点①）。
func (h *PublishTorrentsHandler) runMainlinePTGen(ctx context.Context, meta *model.TorrentMetadata, force bool) bool {
	if meta == nil || h.ptgen == nil {
		return false
	}
	// §59.490: 音乐种跳过——PTGen 是影视元数据语义（title 兜底搜索碰巧
	// 命中影视/动漫/豆瓣音乐条目时 poster/description 被"成功即权威"
	// 无条件覆盖——School Rumble 案 86 行污染实证：封面=doubaninfo 海报·
	// desc=豆瓣剧情简介）。音乐链数据单源自足（§59.466）·音乐 poster/
	// desc 由 adapter 权威提供。影视链零触碰（music 不在场照走）。
	if metadata.HasMusic(meta.DetailSourceJSON) {
		h.logger.Info("mainline ptgen: music seed, skip",
			zap.String("hash", meta.InfoHash[:min(10, len(meta.InfoHash))]),
			zap.String("site", meta.SiteName))
		// §59.490 附: 幂等清理 PTGen 影视语义残留列——upsert 零值语义
		// （store 的 Assign 只写非零值字段）致重获不清理旧行：12 九簇
		// 重获后消费行 ptgen/description 仍豆瓣残留实证。音乐语义两列
		// 恒空——Updates map 显式写空串（map 可写零值）·每次获取都清
		// （不只 force·防御性幂等）。影视链零触碰（music 不在场不进）。
		if h.db != nil {
			if uErr := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
				Where("info_hash = ? AND site_name = ?", meta.InfoHash, meta.SiteName).
				Updates(map[string]interface{}{
					"ptgen_source_json": "",
					"description":       "",
				}).Error; uErr != nil {
				h.logger.Warn("mainline ptgen: music residue cleanup failed",
					zap.String("hash", meta.InfoHash[:min(10, len(meta.InfoHash))]),
					zap.Error(uErr))
			}
		}
		return false
	}
	// §59.286 终版: 三级顺序链——douban →（失败）→ imdb →（失败）→ title；
	// 空键跳过；成功（RawBBCode 非空）即短路；写穿缓存（Provider 天然）。
	// 原 mainlineQueryKey 单键选取在键失效（豆瓣页删除/API 抖动）时整链失败。
	keys := mainlineQueryKeys(meta.DoubanURL, meta.IMDbURL, meta.Title)
	if len(keys) == 0 {
		h.logger.Info("mainline ptgen: no query key, skip",
			zap.String("hash", meta.InfoHash[:min(10, len(meta.InfoHash))]),
			zap.String("site", meta.SiteName))
		return false
	}
	var result *model.PTGenResult
	for _, k := range keys {
		var (
			r   *model.PTGenResult
			err error
		)
		if force {
			r, err = h.ptgen.AnalyzePTGenForce(ctx, k)
		} else {
			r, err = h.ptgen.AnalyzePTGen(ctx, k)
		}
		if err == nil && r != nil && r.RawBBCode != "" {
			result = r
			break // 短路
		}
		h.logger.Info("mainline ptgen: level failed, trying next",
			zap.String("query", k[:min(50, len(k))]),
			zap.Bool("force", force),
			zap.Error(err))
	}
	if result == nil {
		h.logger.Warn("mainline ptgen: all levels failed",
			zap.String("hash", meta.InfoHash[:min(10, len(meta.InfoHash))]),
			zap.Int("levels", len(keys)))
		return false // 全空——incomplete 状态机承接（§59.236 遗漏①定案）
	}
	// 唯一账本（§59.286: canonical 包装——PTGenToSource 含 fetched_at，
	// 原 bare json.Marshal 与②路径同列两种编码分裂）+ desc + poster
	// 无条件同源覆盖（PTGen 成功即权威——空/野图源/任意均覆盖，与 desc
	// 同页同源；PosterURL 空不覆盖）
	raw, mErr := json.Marshal(metadata.PTGenToSource(*result, time.Now()))
	if mErr != nil {
		h.logger.Warn("mainline ptgen: marshal failed", zap.Error(mErr))
		return false
	}
	updates := map[string]interface{}{
		"ptgen_source_json": string(raw),
		"description":       result.RawBBCode,
	}
	if result.PosterURL != "" {
		updates["poster"] = result.PosterURL
	}
	if uErr := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("info_hash = ? AND site_name = ?", meta.InfoHash, meta.SiteName).
		Updates(updates).Error; uErr != nil {
		h.logger.Warn("mainline ptgen: persist failed", zap.Error(uErr))
		return false
	}
	h.logger.Info("mainline ptgen: applied",
		zap.String("hash", meta.InfoHash[:min(10, len(meta.InfoHash))]),
		zap.String("site", meta.SiteName))
	return true
}

// canonicalGroupName §59.234 ①: 组名规范化到映射词条名——搜索附加词用
// 站方标题标准写法（CMCTf 笔误提取→CMCT 词条）。miss 返回原词。
func (h *PublishTorrentsHandler) canonicalGroupName(ctx context.Context, group string) string {
	if group == "" || h.db == nil {
		return group
	}
	var exact []string
	h.db.WithContext(ctx).Model(&model.ReleaseGroupMapping{}).
		Where("LOWER(group_name) = LOWER(?)", group).Limit(1).Pluck("group_name", &exact)
	if len(exact) > 0 {
		return exact[0]
	}
	// §59.550: 前缀回退退役（用户定案「官组必须显式映射·系统不推导」）——
	// miss 返回原词（诚实形态）。变体由映射表显式收编（DSWeb 先例）。
	return group
}

// syncGroupLexicon §59.233: mappings CRUD 后重建组名词表（识别+展示双层——
// RefreshCache 只刷 LookupGroup 缓存，lexicon 不联动则新词条不生效）。
func (h *PublishTorrentsHandler) syncGroupLexicon(ctx context.Context) {
	var words []string
	h.db.WithContext(ctx).Model(&model.ReleaseGroupMapping{}).Pluck("group_name", &words)
	// §59.343: 组名词表单点注册（titleparser 收敛到 util——双注册废止）
	util.SetGroupLexicon(words)
}

// effectiveSpecification §59.226 附七: 生效规格——SPEC 非空返回 SPEC；
// SPEC 空时 IsEncode 铁证派生 "Encode"（Encode 派生单点化：后端出值，
// 前端 specDisplay 拼接逻辑删除）。
// nonDefaultFrameRate §59.226 附二十一: 非默认帧率判定——默认帧率
// （23.976/24/25/29.970——电影母源标准）返回空（标题生态从不标默认）；
// 非默认（60/120/50 高帧率）返回整数形态（"60.000"→"60fps"）。
func nonDefaultFrameRate(fps string) string {
	if fps == "" {
		return ""
	}
	switch fps {
	case "23.976", "24", "24.000", "25", "25.000", "29.970", "29.97":
		return ""
	}
	// 去尾零："60.000"→"60"
	trimmed := strings.TrimRight(strings.TrimRight(fps, "0"), ".")
	return trimmed + "fps"
}

func effectiveSpecification(p titleparser.TechProfile) string {
	if p.Specification != "" {
		return p.Specification
	}
	if titleparser.IsEncode(p) {
		return "Encode"
	}
	return ""
}

func (h *PublishTorrentsHandler) handleGetSeed(w http.ResponseWriter, r *http.Request) {
	infoHash := extractSeedHash(r)
	if infoHash == "" || infoHash == "seeds" {
		Error(w, http.StatusBadRequest, 40001, "缺少 info_hash")
		return
	}

	// §59.44: 资源视图解析——按 (client,path,name) 圈全部活跃 hash 聚合 metadata，
	// 修复挂载 hash≠保留行 hash 的 404（49% 资源组）；hidden 兜底覆盖编辑期间删种
	rv := h.resourceResolver.ResolveResource(r.Context(), infoHash)
	if rv == nil || rv.Meta == nil {
		Error(w, http.StatusNotFound, 40401, "未找到 metadata")
		return
	}
	meta := h.selectSourceMeta(rv.AllMetas)
	if meta == nil {
		meta = rv.Meta
	}
	// 后续更新定位用资源视图的元数据行 hash（数据在谁名下就更新谁）——meta.InfoHash 即权威

	// §59.47: screenshots 列统一解析（JSON 数组优先/换行回退）——
	// 原 strings.Split("\n") 对 JSON 格式列拆出整个 JSON 串当 URL（1 张损坏图）
	screenshots := model.ParseScreenshotColumn(meta.Screenshots)

	// §59.26: BuildTechProfile 三源合并（标题 + MediaInfo + DOM），5 标题字段始终用 profile，
	// 14 平铺字段 DB 为空时 fallback 到 profile（兼容历史数据）
	// MI 优先 local（MediaInfo），fallback 源站（SourceMediaInfo）
	// §59.319 P1: 原盘优先 BDInfo（ExtractMediaInfo 格式分流自动解析）
	miForProfile := meta.BDInfo
	if miForProfile == "" {
		miForProfile = meta.MediaInfo
	}
	if miForProfile == "" {
		miForProfile = meta.SourceMediaInfo
	}
	// §59.28 D1: DOM 源接入（与 fetchSingleTorrent 同一套三源合并）
	domMedium, domRes, domVideo, domAudio := titleparser.DOMFieldsFromDetailSource(meta.DetailSourceJSON)
	profile := titleparser.BuildTechProfile(meta.Title, miForProfile, domMedium, domRes, domVideo, domAudio)
	components := titleparser.TechProfileToComponents(profile)
	inferredCategory := titleparser.InferCategory(components, meta.SourceCategory, "", "")

	// §59.34: Encode 派生（基于最终展示组合值：DB 优先 fallback profile）
	displayProfile := titleparser.TechProfile{
		SourceType:    pickNonEmpty(meta.SourceType, profile.SourceType),
		Specification: pickNonEmpty(meta.Specification, profile.Specification),
		VideoCodec:    pickNonEmpty(meta.VideoCodec, profile.VideoCodec),
		// §59.319 附二十八: Resolution 必传——MediumCanonicalOf 的 FromBDInfo
		// 分支需要 Resolution=2160p 判 UHD（缺失时 Mercy 2160p 盘误显示
		// "Blu-ray 原盘" 而非 "UHD Blu-ray 原盘"）；mini profile 构造点
		// 第四处漏传（displayProfile/encProfile/auditProfile/preAuditTitle 清单化）
		Resolution: pickNonEmpty(meta.Resolution, profile.Resolution),
		// §59.319 附三十二: RawTitle 必传——重编码标记排除扫原始标题
		// （雀起乡 Blu-ray.x265 误判原盘案：漏传致扫描恒空）
		RawTitle: meta.Title,
	}
	// §59.151: MI 铁证传递（displayProfile 原列值构造丢 MIEncoded——
	// WEB-DL 剧集 Encode 误判根因：IsEncode 走不进 MI 分支回落旧 spec 判定）
	displayProfile.MIEncoded = profile.MIEncoded
	displayProfile.MIHasVideo = profile.MIHasVideo
	// §59.319 附十六补: BDInfo 铁证传递（displayProfile 同样丢 FromBDInfo——
	// detail 端点 medium_canonical 仍走压制写法启发式→Encode 误标，243
	// Tab1 实测捕获；与列表 encProfile/BuildTechProfile 三处同源）
	displayProfile.FromBDInfo = profile.FromBDInfo

	// §59.46: 展示优先级兜底——description 列为空（源站无简介形态）时按
	// douban_url 查 ptgen_cache（format BBCode）；非空不覆盖（尊重源站/用户编辑）
	descriptionOut := meta.Description
	if descriptionOut == "" && meta.DoubanURL != "" {
		var cached model.PTGenCache
		if err := h.db.WithContext(r.Context()).
			Where("query_key = ?", meta.DoubanURL).
			Order("updated_at DESC").First(&cached).Error; err == nil {
			var pr struct {
				RawBBCode string `json:"raw_bbcode"`
			}
			if json.Unmarshal([]byte(cached.JSONData), &pr) == nil && pr.RawBBCode != "" {
				descriptionOut = pr.RawBBCode
			}
		}
	}

	result := map[string]interface{}{
		"info_hash": meta.InfoHash,
		"site_name": meta.SiteName,
		"title":     meta.Title,
		"subtitle":  meta.Subtitle,
		// §59.514 附八: 重组标题资产列透出（Tab1 ①区展示核对+tooltip notes）
		"reassembled_title": meta.ReassembledTitle,
		"reassembled_notes": meta.ReassembledNotes,
		"poster":            meta.Poster,
		"description":       descriptionOut,
		"screenshots":       screenshots,
		"mediainfo":         miForProfile,
		"bdinfo":            meta.BDInfo,
		"is_disc":           meta.BDInfo != "", // §59.319 附四：前端单框化/文案分流
		// §59.542 追记十二: 消费时合成——tab2 初见即完整形态（站方声明+
		// 官组致谢）·DB 保持净化态（数据与水印分离终态设计）。
		"statement":       description.ComposeStatement(meta.Statement, meta.SiteName, meta.Title),
		"imdb_url":        meta.IMDbURL,
		"douban_url":      meta.DoubanURL,
		"tmdb_url":        meta.TMDbURL,
		"flags":           meta.Flags,
		"source_category": meta.SourceCategory,
		"reviewed":        meta.Reviewed,
		"fetched_at":      meta.FetchedAt,
		"fetch_source":    meta.FetchSource,

		// 14 DB 平铺字段（DB 为空 → profile fallback）
		"category": pickNonEmpty(meta.Category, inferredCategory),
		"form":     meta.Form,
		// §59.108: 编辑表单媒介输入框数据源（titleComponents.medium 曾恒空——
		// GET 无 medium 键, source_type+specification 合成 TitleComponents.Medium 形态）
		// §59.235 P1 附: 公共单点（ComposeMedium——双实现漏修教训）
		"medium": titleparser.ComposeMedium(
			pickNonEmpty(meta.SourceType, profile.SourceType),
			pickNonEmpty(meta.Specification, profile.Specification)),
		"resolution":     pickNonEmpty(meta.Resolution, profile.Resolution),
		"video_codec":    pickNonEmpty(meta.VideoCodec, profile.VideoCodec),
		"audio_codec":    pickNonEmpty(meta.AudioCodec, profile.AudioCodec),
		"audio_channels": pickNonEmpty(meta.AudioChannels, profile.AudioChannels),
		"audio_tech":     pickNonEmpty(meta.AudioTech, profile.AudioTechnology),
		// §59.116: 展示层零计算回归——列值唯一真相（计算只在 t0 落库层，有完整
		// titleparser 管道与段上下文）；fallback 只裸 profile 补位（列 0=异常诚实显示，
		// 刷列治本）。§59.115 曾在 fallback 加扣减——展示层第二套计算是分裂之源。
		"audio_tracks": pickNonZero(meta.AudioTracks, profile.AudioTracks),
		"hdr":          pickNonEmpty(meta.HDR, profile.HDR),
		"bit_depth":    pickNonEmpty(meta.BitDepth, profile.BitDepth),
		"source_type":  displayProfile.SourceType,
		// §59.226 附七: 生效规格（effective specification）——SPEC 空 →
		// IsEncode 派生 "Encode"（后端单点出值，前端规格栏纯展示）
		"specification": effectiveSpecification(displayProfile),
		// §59.34: Encode 派生标识（前端规格展示用，不参与重组）
		"encode":          titleparser.IsEncode(displayProfile),
		"source_platform": pickNonEmpty(meta.SourcePlatform, profile.SourcePlatform),
		"edition_info":    pickNonEmpty(meta.EditionInfo, profile.EditionInfo),
		"region_code":     pickNonEmpty(meta.RegionCode, profile.RegionCode),

		// 5 标题解析字段（chinese_prefix fallback 到副标题【中文名】）
		"main_title":     profile.MainTitle,
		"season_episode": profile.SeasonEpisode,
		"year": func() string {
			if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil && src.Year != "" {
				return src.Year // §59.236 ④: ◎年代优先（#4 定案实施）
			}
			return profile.Year
		}(),
		"release_group":  profile.ReleaseGroup,
		"chinese_prefix": pickNonEmpty(profile.ChinesePrefix, extractChineseFromSubtitle(meta.Subtitle)),

		// §59.236 ④: PTGen 族键改 ptgen_source_json 唯一账本直读
		// （三列冻结——§59.236 定案；fallback 标题侧保留供 PTGen 缺失显示）
		"chinese_title": func() string {
			if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil && src.ChineseTitle != "" {
				return src.ChineseTitle
			}
			return pickNonEmpty(profile.ChinesePrefix, extractChineseFromSubtitle(meta.Subtitle))
		}(),
		"english_title": func() string {
			// §59.237: 读 TranslatedTitles（◎译名行完整串——唯一真源）；
			// ForeignTitle 是端点陷阱字段（恒=chinese_title——fnos 片名=译名故障根因）
			if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil && src.TranslatedTitles != "" {
				return src.TranslatedTitles
			}
			return profile.MainTitle
		}(),
		// §59.319 附二十一: 规格表忠实显示帧率（含默认值）——"默认不显示"
		// 仅适用于标题重组（§59.226 附二十一），技术规格表是信息展示应完整
		"frame_rate": profile.FrameRate,
		// §59.226 附四: 媒介 canonical 单点（后端出值——前端 siteMediumDisplay 副本废除）
		"medium_canonical": titleparser.MediumCanonicalOf(displayProfile),

		// 状态
		// §59.469 附七: 音乐种判定精确通道（meta 完整行·JSON 有值）
		"missing_fields": h.checkRequiredFields(meta, detailSourceHasMusic(meta.DetailSourceJSON)),
	}

	// §59.75: 产地/类型（PTGen 源归一——region.us/genre.drama + label 双形态，
	// 前端 Tab1 只读展示，发布映射消费 canonical）
	if src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON); err == nil && src != nil {
		result["region"] = normalizeDomainValues("region", src.Region)
		result["genre"] = normalizeDomainValues("genre", src.Genre)
	}

	// §59.469 附六: MDP 消费——music 结构化元数据透出（音乐种非 nil·§59.466
	// 落库资产；入口分流（music 在场→MDP）+MDP 22 字段渲染）
	if src, err := metadata.UnmarshalDetailSource(meta.DetailSourceJSON); err == nil && src != nil && src.Music != nil {
		result["music"] = src.Music
	}

	// §59.26: 返回 tags——双形态（§59.106: keys=canonical 供发布映射,
	// labels=dict 显示名——前端已选区不依赖 bundle 内 dict 新旧, 新词条上线
	// 旧缓存页面也不显示代码）
	if meta.Tags != "" {
		var tags []string
		if err := json.Unmarshal([]byte(meta.Tags), &tags); err == nil && len(tags) > 0 {
			result["tags"] = tags
			result["tag_labels"] = tagDisplayNames(tags)
		}
	}
	if result["tags"] == nil {
		result["tags"] = []string{}
	}

	// §59.21: 返回 is_local（从 client_id 查询）
	clientUID := uint(0)
	if v := r.URL.Query().Get("client_id"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			clientUID = uint(n)
		}
	}
	if clientUID != 0 {
		var client model.ClientConfig
		if err := h.db.WithContext(r.Context()).Where("id = ?", clientUID).First(&client).Error; err == nil {
			result["is_local"] = client.IsLocal
		}
	}

	Success(w, result)
}

// putSeedRequest §59.89: PUT /publish/seeds 请求体（提升命名——buildPutSeedUpdates 测试锚）。
type putSeedRequest struct {
	Force bool `json:"force"` // §59.484: 强制审核——MDP 详情页 missing_fields 非空时
	// 用户显式确认"站方源数据固有缺口"后旁路审核门（大合集老种 album 空案）
	Poster      string   `json:"poster"`
	Screenshots []string `json:"screenshots"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	SiteName    string   `json:"siteName"`
	Reviewed    bool     `json:"reviewed"` // §59.247: 显式审核通道（确认完成——带 9 字段门槛）
}

// buildPutSeedUpdates §59.89: PUT 更新构造（空值不覆盖）——部分保存场景（只改
// tags 不带 description）不得清空简介/海报/截图；v0.0.738 验证脚本 PUT {} 污染
// 预览实锤的根因修复。清空语义: 显式传空串 poster=""/description="" 视为清空
// 仍生效（需求方主动清空 vs 漏传的区分——JSON 里 nil 与 "" 无法区分，取保守:
// 一律不覆盖空值；主动清空走"重新获取"链）。
// §59.170: screenshots 空数组同 policy——[] 与 nil 一律不覆盖（直开预览自动
// 保存空表单实锤：[] 覆盖 + reviewed 合成降级 + 簇连坐三连击；空值清除无合法
// 消费者，主动清空走"重新获取"链）。
func buildPutSeedUpdates(req putSeedRequest, existingPoster, existingDesc, existingShots string) map[string]interface{} {
	updates := map[string]interface{}{}
	if req.Poster != "" {
		updates["poster"] = req.Poster
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if len(req.Screenshots) > 0 {
		updates["screenshots"] = model.FormatScreenshotColumn(req.Screenshots)
	}
	if req.Tags != nil {
		if data, err := json.Marshal(req.Tags); err == nil {
			updates["tags"] = string(data)
		}
	}
	return updates
}

// handlePutSeed §59.20: 保存种子配置（PUT /publish/seeds/:info_hash）。
// 写入编辑字段（poster/screenshots/description）→ 9 字段校验 → Reviewed → 预览渲染。
func (h *PublishTorrentsHandler) handlePutSeed(w http.ResponseWriter, r *http.Request) {
	infoHash := extractSeedHash(r)
	if infoHash == "" || infoHash == "seeds" {
		Error(w, http.StatusBadRequest, 40001, "缺少 info_hash")
		return
	}

	var req putSeedRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	// 找到目标行
	// §59.44: 资源视图解析——前端传的 hash 可能是列表保留行（无 metadata 挂靠），
	// 按 (client,path,name) 圈 hash 后用挂载行 hash 定位更新目标
	if rv := h.resourceResolver.ResolveResource(r.Context(), infoHash); rv != nil && rv.Meta != nil {
		infoHash = rv.Meta.InfoHash
	}
	query := h.db.WithContext(r.Context()).Where("info_hash = ?", infoHash)
	if req.SiteName != "" {
		query = query.Where("site_name = ?", req.SiteName)
	}
	var metas []model.TorrentMetadata
	query.Find(&metas)
	if len(metas) == 0 {
		Error(w, http.StatusNotFound, 40401, "未找到 metadata")
		return
	}
	meta := h.selectSourceMeta(metas)
	if meta == nil {
		meta = &metas[0]
	}
	existingShots := meta.Screenshots

	// 写入编辑字段
	updates := buildPutSeedUpdates(req, meta.Poster, meta.Description, meta.Screenshots)

	// 9 字段校验 → Reviewed（有效值合成——空请求不把已有值算成缺失）
	meta.Poster = pickNonEmpty(req.Poster, meta.Poster)
	// §59.170: 空 screenshots（nil 与 [] 同义）沿用库中已有——校验才诚实。
	// 原 guard 只挡 nil：空表单自动保存（直开预览）合成出"缺截图"→ reviewed
	// 降级 → 簇传播连坐（46 行实锤）。与 buildPutSeedUpdates 空值不覆盖同语义。
	if len(req.Screenshots) > 0 {
		meta.Screenshots = model.FormatScreenshotColumn(req.Screenshots)
	} else {
		meta.Screenshots = existingShots
	}
	meta.Description = pickNonEmpty(req.Description, meta.Description)
	// §59.469 附七: 音乐种判定精确通道（meta 完整行·JSON 有值）
	missing := h.checkRequiredFields(meta, detailSourceHasMusic(meta.DetailSourceJSON))
	// §59.247: PUT 默认不碰 reviewed（保存≠审核——浏览预览/取消退出不
	// 误审）；req.Reviewed=true 为显式审核通道（确认完成调用——带 9 字段
	// 门槛：缺字段拒绝审核）。§59.20 自动审核在 §59.141 自动保存后语义错位。
	if req.Reviewed {
		if len(missing) > 0 && !req.Force {
			Error(w, http.StatusBadRequest, 40001, fmt.Sprintf("审核失败：仍缺 %d 个字段: %s（如确认站方源数据未提供·可 force=true 强制审核）", len(missing), strings.Join(missing, ", ")))
			return
		}
		updates["reviewed"] = true
		// §59.93/94: 审核状态簇同步（显式审核触发）
		defer h.syncClusterReviewedByHashes(context.Background(), []string{infoHash}, true)
	}

	h.db.WithContext(r.Context()).Model(&model.TorrentMetadata{}).
		Where("id = ?", meta.ID).
		Updates(updates)

	// §59.328: PUT 编辑簇传播——编辑是用户意图写入，与 fetch 链同语义
	// 全簇生效（#7 挂账兑现：原只写选中行，副本不跟——展示层不一致；
	// 且编辑落在 cluster 行场景下次 fetch 回传会覆盖丢失）。路由已由
	// §59.44 ResolveResource 保证写权威行；此处复用 v0.0.1144
	// clusterInferredFields 传播（编辑字段全在白名单内）。幂等：只覆盖
	// fetch_source='cluster' 行，不碰其他站独立行。
	var editedSrc model.TorrentMetadata
	if err := h.db.WithContext(r.Context()).Where("id = ?", meta.ID).First(&editedSrc).Error; err == nil {
		h.propagateEditedClusterFields(r.Context(), editedSrc)
	}

	// 重新查询获取更新后的数据（§59.28 C：并发删除时返回明确错误而非零值）
	var updated model.TorrentMetadata
	if err := h.db.WithContext(r.Context()).Where("id = ?", meta.ID).First(&updated).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50001, "保存后读取失败: "+err.Error())
		return
	}

	// §59.104: description 展示兜底（与 GET §59.46 同款）——列空按 douban_url 查
	// ptgen_cache（format BBCode）；非空不覆盖
	descriptionOut := updated.Description
	if descriptionOut == "" && updated.DoubanURL != "" {
		var cached model.PTGenCache
		if err := h.db.WithContext(r.Context()).
			Where("query_key = ?", updated.DoubanURL).
			Order("updated_at DESC").First(&cached).Error; err == nil {
			var pr struct {
				RawBBCode string `json:"raw_bbcode"`
			}
			if json.Unmarshal([]byte(cached.JSONData), &pr) == nil && pr.RawBBCode != "" {
				descriptionOut = pr.RawBBCode
			}
		}
	}

	// §59.28 C（方案A ②）：ReassembleFromTechProfile 标准化重组标题（预览用，不落库——
	// 发布时按目标站 title_format 重组，这里用 v1.05 默认模板展示标准化效果）
	// §59.28 C（方案A ④）：Renderer.Render 生成完整描述（声明+致谢+海报+正文+截图）
	miForProfile := updated.MediaInfo
	if miForProfile == "" {
		miForProfile = updated.SourceMediaInfo
	}
	domMedium, domRes, domVideo, domAudio := titleparser.DOMFieldsFromDetailSource(updated.DetailSourceJSON)
	profile := titleparser.BuildTechProfile(updated.Title, miForProfile, domMedium, domRes, domVideo, domAudio)
	// §59.514 附八: PUT 保存后编辑素材可能变化——FinalizeTitle 共享单点
	// 重算刷列（第四消费方·与发布/t0/recompute 同源）。torrentData 尽力
	// 拉（无 clientUID 上下文→快照定位·同 recompute 模式）；空列降级
	// 弱版现算（titleIncomplete 判定用终值）。
	reassembledTitle := ""
	{
		var td []byte
		if h.clientMgr != nil && updated.InfoHash != "" {
			var snap model.TorrentSnapshot
			if err := h.db.WithContext(r.Context()).Where("hash = ?", updated.InfoHash).First(&snap).Error; err == nil {
				if cl, clErr := h.clientMgr.Get(snap.ClientUID); clErr == nil && cl != nil {
					fCtx, fCancel := context.WithTimeout(r.Context(), 30*time.Second)
					if b, bErr := cl.ExportTorrent(fCtx, updated.InfoHash); bErr == nil {
						td = b
					}
					fCancel()
				}
			}
		}
		rt, notes := h.executor.FinalizeTitle(r.Context(), &updated, &profile, td)
		if rt != "" {
			reassembledTitle = rt
			_ = h.db.WithContext(r.Context()).Model(&model.TorrentMetadata{}).
				Where("info_hash = ?", updated.InfoHash).
				Updates(map[string]interface{}{
					"reassembled_title": rt,
					"reassembled_notes": strings.Join(notes, "；"),
				}).Error
		}
	}
	if reassembledTitle == "" {
		reassembledTitle = titleparser.ReassembleFromTechProfile(profile, titleparser.V105TitleFormat())
	}
	// §59.254 项4 D：预览展示且警示不拒（预览本身即发布前检查工具）——
	// 重组空=数据不完整（发布链将拒发），此处展示原形态+警示标记
	titleIncomplete := strings.TrimSpace(reassembledTitle) == ""

	renderer := description.NewRenderer("")
	screenshots := model.ParseScreenshotColumn(updated.Screenshots) // §59.47
	descData := &model.DescriptionData{
		Statement:     updated.Statement,
		PosterURL:     updated.Poster,
		PTGenBody:     updated.Description,
		MediaInfoText: miForProfile,
		BDInfoText:    updated.BDInfo,
		Screenshots:   screenshots,
		SourceSite:    updated.SiteName,
		Title:         updated.Title,
	}
	renderedDesc, renderErr := renderer.Render(descData, model.SiteDescConfig{})

	result := map[string]interface{}{
		"info_hash":   updated.InfoHash,
		"site_name":   updated.SiteName,
		"title":       updated.Title,
		"subtitle":    updated.Subtitle,
		"poster":      updated.Poster,
		"description": descriptionOut, // §59.104: GET 同款兜底（列空按 douban_url 查 ptgen_cache——预览不漏缓存简介）
		"statement":   updated.Statement,
		"tags": func() interface{} {
			var tags []string
			if updated.Tags != "" {
				if err := json.Unmarshal([]byte(updated.Tags), &tags); err != nil {
					return []string{}
				}
			}
			return tags
		}(),
		"tag_labels": func() []string {
			// §59.110: 预览④与 Tab1 同源显示名（§59.106 双形态）
			var tags []string
			if updated.Tags != "" {
				if err := json.Unmarshal([]byte(updated.Tags), &tags); err != nil {
					return []string{}
				}
				return tagDisplayNames(tags)
			}
			return []string{}
		}(),
		"reviewed":       updated.Reviewed,
		"missing_fields": missing,

		// §59.103: v1.05 全字段——与 GET detail（Tab1 数据源）同款取值策略：
		// 列值优先（pickNonEmpty/pickNonZero），列空才 profile 现算补。预览=引用
		// Tab1 数据，不再独立计算（§59.102 音轨数分裂的结构性根除——两响应同源）。
		"season_episode":  profile.SeasonEpisode, // transient（不落列，两处同源现算）
		"year":            profile.Year,          // transient
		"resolution":      pickNonEmpty(updated.Resolution, profile.Resolution),
		"hdr":             pickNonEmpty(updated.HDR, profile.HDR),
		"bit_depth":       pickNonEmpty(updated.BitDepth, profile.BitDepth),
		"video_codec":     pickNonEmpty(updated.VideoCodec, profile.VideoCodec),
		"audio_codec":     pickNonEmpty(updated.AudioCodec, profile.AudioCodec),
		"audio_channels":  pickNonEmpty(updated.AudioChannels, profile.AudioChannels),
		"audio_tech":      pickNonEmpty(updated.AudioTech, profile.AudioTechnology),
		"audio_tracks":    pickNonZero(updated.AudioTracks, profile.AudioTracks), // §59.116: 展示层零计算（同 GET）
		"source_type":     pickNonEmpty(updated.SourceType, profile.SourceType),
		"specification":   pickNonEmpty(updated.Specification, profile.Specification),
		"source_platform": pickNonEmpty(updated.SourcePlatform, profile.SourcePlatform),
		"edition_info":    pickNonEmpty(updated.EditionInfo, profile.EditionInfo),
		"region_code":     pickNonEmpty(updated.RegionCode, profile.RegionCode),
		"chinese_prefix":  pickNonEmpty(profile.ChinesePrefix, extractChineseFromSubtitle(updated.Subtitle)),
		"encode":          titleparser.IsEncode(profile),
		// §59.90: 对齐 Tab1——剧名/制作组/类型(InferCategory)
		"main_title":    profile.MainTitle,
		"release_group": profile.ReleaseGroup,
		"category": func() string {
			comps := titleparser.TechProfileToComponents(profile)
			return titleparser.InferCategory(comps, updated.SourceCategory, "", "")
		}(),

		// §59.28 C（方案A ②④）：标准化重组标题 + 渲染后完整描述（预览）
		"reassembled_title":    reassembledTitle,
		"title_incomplete":     titleIncomplete,
		"rendered_description": renderedDesc,
	}
	// §59.81: 产地/类型 + 分段渲染素材（简介四段结构化展示）
	if src, err := metadata.UnmarshalPTGenSource(updated.PTGenSourceJSON); err == nil && src != nil {
		result["region"] = normalizeDomainValues("region", src.Region)
		result["genre"] = normalizeDomainValues("genre", src.Genre)
	}
	if renderErr != nil {
		result["render_error"] = renderErr.Error()
	}
	Success(w, result)
}

// handleFetchSingleSeed §59.26: 单种子获取（复用 fetchSingleTorrent + is_local 支持）
func (h *PublishTorrentsHandler) handleFetchSingleSeed(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(r.URL.Path, "/")
	infoHash := strings.TrimSuffix(path, "/fetch")
	parts := strings.Split(infoHash, "/")
	if len(parts) > 0 {
		infoHash = parts[len(parts)-1]
	}
	if infoHash == "" || infoHash == "seeds" {
		Error(w, http.StatusBadRequest, 40001, "缺少 info_hash")
		return
	}
	clientUID := uint(0)
	if v := r.URL.Query().Get("client_id"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			clientUID = uint(n)
		}
	}
	if clientUID == 0 {
		Error(w, http.StatusBadRequest, 40001, "client_id 为必填")
		return
	}

	// §59.537: 完成度边界——未 100% 不获取（数据不完整不是可获取素材；
	// 做种/暂停不限只看完成度·用户定案）。查代表快照行校验。
	var snapProg struct {
		Progress float64
		State    string
	}
	if err := h.db.WithContext(r.Context()).
		Table("torrent_snapshots").
		Select("progress, state").
		Where("hash = ? AND is_hidden = ?", infoHash, false).
		Order("updated_at DESC").First(&snapProg).Error; err == nil && snapProg.Progress < 1.0 {
		Error(w, http.StatusBadRequest, 40006, fmt.Sprintf("种子未下载完成（%.1f%%·%s）——数据不完整不获取，完成后重试", snapProg.Progress*100, snapProg.State))
		return
	}

	// §59.476 位置3: 单种入口同款换 hash——簇内音乐源站副本优先
	// （皇后副本）→ S1 天然命中音乐链。name/savePath 簇内恒同·快照
	// 从换后 hash 读（size 对齐该副本）。
	infoHash, _ = h.resolveMusicReplicaHash(r.Context(), clientUID, infoHash, 0)

	var snap model.TorrentSnapshot
	if err := h.db.WithContext(r.Context()).Where("hash = ? AND client_uid = ?", infoHash, clientUID).First(&snap).Error; err != nil {
		Error(w, http.StatusNotFound, 40401, "快照中未找到该种子")
		return
	}

	isLocal := false
	var client model.ClientConfig
	if h.db.WithContext(r.Context()).Where("id = ?", clientUID).First(&client).Error == nil {
		isLocal = client.IsLocal
	}

	// §59.28 nil 守卫（与 handleBatchFetch 对齐）
	if h.sourceDetector == nil || h.metadataFetcher == nil {
		Error(w, http.StatusInternalServerError, 50002, "服务未就绪（detector/fetcher 未注入）")
		return
	}

	if err := h.fetchSingleTorrent(r.Context(), clientUID, infoHash, snap.Name, snap.Size, snap.SavePath, isLocal, true); err != nil {
		Error(w, http.StatusInternalServerError, 50000, fmt.Sprintf("获取失败: %v", err))
		return
	}

	// §59.26: 获取（含重新获取）后 reviewed=false，必须重新走预览审核
	// §59.44: 资源视图圈 hash——获取可能落在资源键内其他 hash（tid 反查），全组重置
	resetHashes := []string{infoHash}
	if rv := h.resourceResolver.ResolveResource(r.Context(), infoHash); rv != nil && len(rv.Hashes) > 0 {
		resetHashes = rv.Hashes
	}
	h.db.WithContext(r.Context()).Model(&model.TorrentMetadata{}).
		Where("info_hash IN ?", resetHashes).
		Update("reviewed", false)

	Success(w, map[string]interface{}{"message": "获取成功"})
}

// handleDeleteSeed §59.26 §59.32: 清除种子 metadata（资源级——删同名全部 hash）。
// coverage 不清除。多站聚合下载器同资源多 hash 变体，metadata 只挂其一；
// "未配置"判定是资源级（任一 hash 有 = 已配置），故清除也必须资源级，
// 否则资源永远无法回到未获取态（20 清除 → 12 未配置 的根因）。
func (h *PublishTorrentsHandler) handleDeleteSeed(w http.ResponseWriter, r *http.Request) {
	infoHash := extractSeedHash(r)
	if infoHash == "" || infoHash == "seeds" {
		Error(w, http.StatusBadRequest, 40001, "缺少 info_hash")
		return
	}

	// 查该 hash 的紧键三元组 → 同簇（(client,path,name) 一致）全部 hash
	var ck struct {
		ClientUID uint
		SavePath  string
		Name      string
	}
	hasCluster := true
	if err := h.db.WithContext(r.Context()). //nolint:gosec // Row().Scan 容错——无行 → 走直删分支
							Table("torrent_snapshots").
							Select("client_uid, save_path, name").
							Where("hash = ? AND name != ''", infoHash).
							Limit(1).
							Row().Scan(&ck.ClientUID, &ck.SavePath, &ck.Name); err != nil { //nolint:errcheck // 同上
		hasCluster = false
	}

	result := h.db.WithContext(r.Context()).Where("info_hash = ?", infoHash).Delete(&model.TorrentMetadata{})
	if hasCluster {
		// 资源级（§59.26 语义保持）：删同簇全部 hash 的 metadata（含代表 hash 自身，幂等）。
		// §59.316 附 D: 域收紧 name → 紧键 (client,path,name)——同名跨下载器/跨路径
		// 簇不再被连坐清除（数据跟随紧键资源，与 §59.44 权威口径统一）
		var siblingHashes []string
		h.db.WithContext(r.Context()).
			Table("torrent_snapshots").
			Select("hash").
			Where("client_uid = ? AND save_path = ? AND name = ?", ck.ClientUID, ck.SavePath, ck.Name).
			Find(&siblingHashes)
		if len(siblingHashes) > 0 {
			result = h.db.WithContext(r.Context()).
				Where("info_hash IN ?", siblingHashes).
				Delete(&model.TorrentMetadata{})
		}
	}
	// §59.319 附七：清除审计——原 `_ = result` 零日志吞掉删除痕迹（243 Mercy
	// 簇消失调查一小时主因：单行"清除"删整簇 62 行 metadata 无任何可观测信号）
	h.logger.Info("seed metadata cleared",
		zap.String("hash", infoHash[:min(10, len(infoHash))]),
		zap.Bool("cluster_scope", hasCluster),
		zap.Int64("rows", result.RowsAffected),
		zap.Error(result.Error))
	Success(w, map[string]interface{}{"message": "已清除"})
}

// tagDisplayNames §59.106: tag canonical 键 → dict 显示名列表（miss 保留原文）。
func tagDisplayNames(tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if l := titleparser.ReverseLookup("tag." + t); l != "" {
			out = append(out, l)
		} else {
			out = append(out, t)
		}
	}
	return out
}

// normalizeDomainValues §59.75: 域值归一（PTGen 原词→canonical）+ label 映射。
// 返回 {"keys": ["region.us"], "labels": ["美国"]} 双形态——Tab1 显示 label，
// 发布映射消费 keys。miss（未收录国名/类型词）保留原文双形态。
func normalizeDomainValues(domain string, raws []string) map[string][]string {
	if len(raws) == 0 {
		return nil
	}
	keys, labels := make([]string, 0, len(raws)), make([]string, 0, len(raws))
	seen := map[string]bool{}
	for _, r := range raws {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		key := titleparser.LookupDictKey(domain, r)
		if key == "" {
			key = r // miss 保留原文
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
		if label := titleparser.ReverseLookup(key); label != "" {
			labels = append(labels, label)
		} else {
			labels = append(labels, r)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	return map[string][]string{"keys": keys, "labels": labels}
}

// persistPTGenSource §59.75: PTGen 源结构化持久化（ptgen_source_json）。
// Region/Genre（产地/类型）此前只在 querier 闭包里被取走 RawBBCode/PosterURL 后
// 丢弃——接入遗漏修复（列/Marshal/Unmarshal 机制 §56 时代已有）。
// Tab1 产地/类型展示与未来发布站点映射消费此列。
func (h *PublishTorrentsHandler) persistPTGenSource(ctx context.Context, infoHash, siteName string, r *model.PTGenResult) {
	if r == nil {
		return
	}
	data, err := json.Marshal(metadata.PTGenToSource(*r, time.Now()))
	if err != nil {
		return
	}
	if err := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("info_hash = ? AND site_name = ?", infoHash, siteName).
		Update("ptgen_source_json", string(data)).Error; err != nil {
		h.logger.Warn("ptgen source persist failed", zap.Error(err))
	}
}

// inferFinalTags §59.500 刀1: 终局推理单点——完整性闸门达标后单轮推理。
// 闸门=checkRequiredFields（形态条件化·与发布门同一语义：推理资格=发布
// 资格=完整性·三端统一·用户定案"数据不完整即不推理"）。截图/BDInfo 未
// 到或失败=missing=不可发布=不定型；幂等（推理纯函数+无变化不写）；
// 定型后簇回传（clusterInferredFields·§59.326 附四同款语义）。
func (h *PublishTorrentsHandler) inferFinalTags(ctx context.Context, clientUID uint, infoHash, siteName string) {
	var m model.TorrentMetadata
	if err := h.db.WithContext(ctx).
		Where("info_hash = ? AND site_name = ?", infoHash, siteName).
		First(&m).Error; err != nil {
		return
	}
	// 完整性闸门：missing 非空=数据不完整=不推理（等待后续数据源触发）
	if missing := h.checkRequiredFields(&m, detailSourceHasMusic(m.DetailSourceJSON)); len(missing) > 0 {
		h.logger.Debug("infer final gated: incomplete",
			zap.String("hash", infoHash[:min(10, len(infoHash))]),
			zap.Strings("missing", missing))
		return
	}
	var snapSize int64
	h.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
		Where("hash = ?", infoHash).Select("COALESCE(MAX(size),0)").Scan(&snapSize)
	// §59.500 刀3 MI 素材融合 a（用户定案）：local 优先→source 兜底——
	// source MI 亦采集素材（"标签=采集素材+推理融合产物"），任一含编码
	// 痕迹即铁证；远程下载器/本地分析未产出场景受同等版本族保护
	// （BDInfo 走 NFO 独立通道不进此链）
	miForInfer := m.MediaInfo
	if miForInfer == "" {
		miForInfer = m.SourceMediaInfo
	}
	inferred := publish.NewMediaTagInferer().InferFull(publish.TagInput{
		MediaInfo:   miForInfer,
		Title:       m.Title,
		Subtitle:    m.Subtitle,
		Description: m.Description,
		NFO:         m.BDInfo,
		Size:        snapSize,
		Region:      m.RegionCode,
		Statement:   m.Statement,
	})
	var existing []string
	if m.Tags != "" {
		if err := json.Unmarshal([]byte(m.Tags), &existing); err != nil {
			existing = nil
		}
	}
	// §59.496 附二 existing 铁证门（融合素材同源）+ §59.74 MergeTags 单点
	all := publish.MergeTags(publish.CleanExistingForMerge(existing, miForInfer), inferred)
	data, _ := json.Marshal(all)
	if string(data) == m.Tags {
		return
	}
	if err := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("info_hash = ? AND site_name = ?", infoHash, siteName).
		Update("tags", string(data)).Error; err != nil {
		h.logger.Warn("infer final tags persist failed", zap.Error(err))
		return
	}
	h.logger.Info("infer final tags applied",
		zap.String("hash", infoHash[:min(10, len(infoHash))]),
		zap.Int("tags_total", len(all)))
	// 簇回传（幂等）：定型产物同步副本行
	if clientUID != 0 {
		var snap model.TorrentSnapshot
		if err := h.db.WithContext(ctx).Where("hash = ?", infoHash).First(&snap).Error; err == nil {
			h.propagateClusterMetadata(ctx, clientUID, snap.SavePath, snap.Name, infoHash, siteName)
		}
	}
}

// purgeDeadScreenshots §59.49: 截图获取时探活——HEAD（含 1 次重试）逐张检查，
// 清死留活；全死全清（screenshots=[]）。只反映获取时点存活状态。
func (h *PublishTorrentsHandler) purgeDeadScreenshots(infoHash, siteName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var meta model.TorrentMetadata
	if err := h.db.WithContext(ctx).
		Where("info_hash = ? AND site_name = ?", infoHash, siteName).
		First(&meta).Error; err != nil {
		return
	}
	urls := model.ParseScreenshotColumn(meta.Screenshots)
	if len(urls) == 0 {
		return
	}

	// 并发探活（每张 HEAD + 失败重试 1 次）
	type result struct {
		idx   int
		alive bool
	}
	ch := make(chan result, len(urls))
	sem := make(chan struct{}, 6) // 并发上限 6
	for i, u := range urls {
		go func(idx int, u string) {
			sem <- struct{}{}
			defer func() { <-sem }()
			ch <- result{idx, publish.CheckPosterAlive(ctx, u)}
		}(i, u)
	}
	alive := make([]bool, len(urls))
	for range urls {
		r := <-ch
		alive[r.idx] = r.alive
	}

	var kept []string
	var dead []string
	for i, u := range urls {
		if alive[i] {
			kept = append(kept, u)
		} else {
			dead = append(dead, u)
		}
	}
	if len(dead) == 0 {
		return // 全活，无需更新
	}
	for _, d := range dead {
		h.logger.Warn("screenshot dead link purged",
			zap.String("hash", infoHash[:10]),
			zap.String("dead_url", d[:min(80, len(d))]))
	}
	data, _ := json.Marshal(kept) // 全死时 kept=nil → "null" 需转 "[]"
	if kept == nil {
		data = []byte("[]")
	}
	h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("info_hash = ? AND site_name = ?", infoHash, siteName).
		Update("screenshots", string(data))
	h.logger.Info("screenshot purge completed",
		zap.String("hash", infoHash[:10]),
		zap.Int("total", len(urls)),
		zap.Int("kept", len(kept)),
		zap.Int("purged", len(dead)))
}

// applyScreenshotStrategy §59.53: 采集链截图策略异步执行（goroutine 内）。
// screenshotStrategyFor §59.319 附十二：读当前截图策略（system_settings
// 动态值；空默认 auto——与 pipeline.ApplyScreenshotStrategy 口径一致）。
// 探活分流判定用（local_upload/force 无视源站图）。
func (h *PublishTorrentsHandler) screenshotStrategyFor(ctx context.Context) string {
	var v string
	h.db.WithContext(ctx).Table("system_settings").
		Where("key = ?", "image_host_strategy").
		Select("value").Scan(&v)
	v = strings.TrimSpace(v)
	if v == "" {
		return "auto"
	}
	return v
}

// §59.57 竞态修复: purgeDeadScreenshots 内联前序（串行）——原双 goroutine 并发时
// 本函数可能读到 purge 前的死链列表，rehost 失败保源后 final==source → same 早退
// → 永不触发 mpv 补图（243 实测 8 组 ptpimg.me 全死链复现）。探活先行，本函数
// 读到的必为活链集，再走策略（白名单/转存/差额补足/无图全量）→ 落库。
func (h *PublishTorrentsHandler) applyScreenshotStrategy(clientUID uint, infoHash, siteName, name, savePath string, isLocal bool, skipCache ...bool) {
	noCache := len(skipCache) > 0 && skipCache[0]
	// §59.58: 并发额度闸门——批量链 N 路 fire-and-forget 无界并发挤爆 CPU/代理（243 实测
	// >20 路时 mpv 摊薄 15 倍 → 撞 4min ctx → 差额补足作废）。排队在闸门外等，不烧 ctx 预算
	// （ctx 在获得额度后才创建）。CPU 总量守恒：总时长不变，换来每单稳定完成。
	sem := h.strategySem
	if sem == nil {
		sem = make(chan struct{}, 5) // 兜底（零值 handler，如测试构造）
	}
	sem <- struct{}{}
	defer func() { <-sem }()

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute) // §59.300 附十八：m2ts IDR 慢链 5 点 2-5min 预算
	defer cancel()

	// §59.63: 截图链接缓存（观察期）——簇键命中且未过期直接复用，跳过探活/转存/
	// mpv/上传全链（Q3 缓存优先：本批源站截图不消费）。锚点=最近一次成功写穿，
	// 手动捕获结果也刷新锚点（Q4）。过期=miss 走既有策略（Q2=A：源站充足仍转存
	// 源站，不足才 mpv——与 §59.53 语义一致）。
	if !noCache {
		if cached, ok := h.lookupScreenshotCache(clientUID, savePath, name); ok {
			data, _ := json.Marshal(cached)
			if err := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
				Where("info_hash = ? AND site_name = ?", infoHash, siteName).
				Update("screenshots", string(data)).Error; err == nil {
				h.propagateClusterScreenshots(ctx, clientUID, savePath, name, infoHash, string(data))
				h.logger.Info("screenshot cache hit",
					zap.String("hash", infoHash[:min(10, len(infoHash))]),
					zap.Int("shots", len(cached)))
				// §59.500 刀1: 截图落地=完整性最后到达者之一→终局推理触发
				h.inferFinalTags(ctx, clientUID, infoHash, siteName)
			}
			return
		}
	}
	if noCache {
		h.logger.Info("screenshot cache skipped (single re-fetch)",
			zap.String("hash", infoHash[:min(10, len(infoHash))]))
	}

	// §59.57: 探活内联前序（自带 90s 独立 ctx，读自身快照；HEAD 秒级不占策略预算）
	// §59.319 附十二：探活按策略分流——local_upload/force 链无视源站图
	// （mpv 全新截传，源图仅作失败回退），探活→清除死链纯多余且有害
	// （mpv 失败回退源图时已被 purge 清空 → 行空图）；auto（源图参与
	// 决策）保持探活。用户定案：始终本地截图=无条件本地截，不走
	// 探活→清除 的 auto 兜底动作。
	if !noCache && h.screenshotStrategyFor(ctx) != "local_upload" {
		h.purgeDeadScreenshots(infoHash, siteName)
	}

	strategyCtx, scancel := context.WithTimeout(ctx, 10*time.Minute) // §59.300 附十八
	defer scancel()

	var meta model.TorrentMetadata
	if err := h.db.WithContext(ctx).
		Where("info_hash = ? AND site_name = ?", infoHash, siteName).
		First(&meta).Error; err != nil {
		return
	}
	source := model.ParseScreenshotColumn(meta.Screenshots)
	// §59.319 附六：force（单种获取/重获）传硬编码策略——无条件 mpv 全新
	// 截传（与 skipCache 同一 force 语义闭合 §59.299 意图）
	// §59.319 附十五：bdSid 注入退役——原盘选轨单点下沉 FindSubtitleStreamID
	// 的 CLPI 快速回退（毫秒级现场解析，不依赖 BDInfo 报告落库时序——
	// 附十四的 api 注入在"清除后重获"场景截图先于扫描落库而失效）；
	// 三链（fetch/手动/任意时刻）经同一函数统一。forcedSid 恒 0（自动）。
	final := h.shotStrategy.ApplyScreenshotStrategy(strategyCtx, name, savePath, source, isLocal, noCache, 0)
	if len(final) == len(source) {
		// 无变化（无图且截图失败/全白名单保留/远程无图）——不覆盖
		same := true
		for i := range final {
			if final[i] != source[i] {
				same = false
				break
			}
		}
		if same {
			// §59.61 附4: 全失败可见性——source=0 且 final=0 是捕获/上传全灭
			// （243 实测: pixhost 早高峰 16 簇三层重试全打穿后静默退出，行永久
			// 空 12.5h 无任何日志）。打点 warn 让停摆可观测，人工/下轮重取补。
			if len(source) == 0 && isLocal {
				h.logger.Warn("screenshot strategy total failure",
					zap.String("hash", infoHash[:10]),
					zap.String("name", name))
			}
			return
		}
	}
	data, _ := json.Marshal(final)
	if final == nil {
		data = []byte("[]")
	}
	if err := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("info_hash = ? AND site_name = ?", infoHash, siteName).
		Update("screenshots", string(data)).Error; err != nil {
		h.logger.Error("screenshot strategy persist failed",
			zap.String("hash", infoHash[:10]), zap.Error(err))
		return
	}
	// §59.61 第 4 步: 截图二次传播（策略异步完成后补簇内空行——元数据传播时未就绪）
	h.propagateClusterScreenshots(ctx, clientUID, savePath, name, infoHash, string(data))
	// §59.500 刀1: 截图落地=完整性最后到达者之一→终局推理触发
	h.inferFinalTags(ctx, clientUID, infoHash, siteName)
	h.logger.Info("screenshot strategy applied",
		zap.String("hash", infoHash[:10]),
		zap.Bool("local", isLocal),
		zap.Int("source", len(source)),
		zap.Int("final", len(final)))

	// §59.63: 成功落库写穿缓存（final 非空才到达此处——same 早退/失败路径不写）
	upsertClusterScreenshotCache(h.db, h.logger, h.screenshotCacheDays, clientUID, savePath, name, final)
}

// handleExecutePublish §59.156 切片 2: 新发布执行器入口（DB 供给——切读 publish_form_config）。
// body: {info_hash, target_site, anonymous, tag_overrides: [], dry_run}
// handleBDInfoBackfill §59.319 P3/D6：存量原盘批量补扫（手动运维端点，
// §59.275 先例）——扫描指定本地下载器下全部簇：磁盘识别门命中且簇内
// 无 bd_info → 入队扫描队列。返回统计（enqueued/已有产物跳过/非原盘/
// 文件不可见）。
func (h *PublishTorrentsHandler) handleBDInfoBackfill(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientUID uint `json:"clientId"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.ClientUID == 0 {
		Error(w, http.StatusBadRequest, 40001, "clientId 必填")
		return
	}
	var client model.ClientConfig
	if err := h.db.WithContext(r.Context()).Where("id = ?", req.ClientUID).First(&client).Error; err != nil {
		Error(w, http.StatusBadRequest, 40001, "下载器不存在")
		return
	}
	if !client.IsLocal {
		Error(w, http.StatusBadRequest, 40001, "该下载器非本地（is_local=false 不支持 BD 扫描）")
		return
	}
	if h.bdinfoQueue == nil || h.bdinfoScanner == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "BDInfo 扫描队列未配置")
		return
	}

	// 簇枚举：distinct (save_path, name) + max(size)（§59.361 B: 簇内最大快照
	// size 供超时自适应——同簇同数据取代表值）
	type clusterRow struct {
		SavePath, Name string
		MaxSize        int64
	}
	var clusters []clusterRow
	if err := h.db.WithContext(r.Context()).Model(&model.TorrentSnapshot{}).
		Where("client_uid = ? AND is_hidden = 0", req.ClientUID).
		Distinct().Select("save_path, name, max(size) as max_size").Find(&clusters).Error; err != nil {
		Error(w, http.StatusInternalServerError, 50000, "簇枚举失败: "+err.Error())
		return
	}

	result := map[string]interface{}{
		"enqueued": 0, "has_bdinfo": 0, "not_disc": 0, "missing": 0,
		"enqueued_names": []string{},
	}
	enqNames := []string{}
	for _, c := range clusters {
		if c.SavePath == "" || c.Name == "" {
			continue
		}
		discPath := h.bdinfoScanner.DetectDiscPath(c.SavePath, c.Name)
		if discPath == "" {
			// 区分文件不可见与确定非原盘：种子目录存在但无盘结构=not_disc，
			// 目录不存在=missing（挂载/路径问题可观测）
			if _, err := os.Stat(filepath.Join(c.SavePath, c.Name)); err != nil {
				result["missing"] = result["missing"].(int) + 1
			} else {
				result["not_disc"] = result["not_disc"].(int) + 1
			}
			continue
		}
		if clusterHasBDInfoDB(h.db, r.Context(), req.ClientUID, c.SavePath, c.Name) {
			result["has_bdinfo"] = result["has_bdinfo"].(int) + 1
			continue
		}
		if h.bdinfoQueue.Enqueue(publish.BDInfoScanTask{
			ClientUID: req.ClientUID, SavePath: c.SavePath, Name: c.Name, DiscPath: discPath,
			Size: c.MaxSize, // §59.361 B: 超时自适应
		}) {
			result["enqueued"] = result["enqueued"].(int) + 1
			enqNames = append(enqNames, c.Name)
		}
	}
	result["enqueued_names"] = enqNames
	result["clusters_total"] = len(clusters)
	h.logger.Info("bdinfo backfill",
		zap.Uint("client", req.ClientUID),
		zap.Int("clusters", len(clusters)),
		zap.Int("enqueued", result["enqueued"].(int)))
	Success(w, result)
}

// handleBDInfoRescan §59.361 A: 单颗重扫——失败态行的手动入口（清失败
// 记录+重新入队；预算自适应 §59.361 B）。pt29 案：30min 超时失败后
// 无重试无入口，失败态永挂。
func (h *PublishTorrentsHandler) handleBDInfoRescan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientUID uint   `json:"clientId"`
		SavePath  string `json:"savePath"`
		Name      string `json:"name"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.ClientUID == 0 || req.SavePath == "" || req.Name == "" {
		Error(w, http.StatusBadRequest, 40001, "clientId/savePath/name 必填")
		return
	}
	if h.bdinfoQueue == nil || h.bdinfoScanner == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "BDInfo 扫描队列未配置")
		return
	}
	discPath := h.bdinfoScanner.DetectDiscPath(req.SavePath, req.Name)
	if discPath == "" {
		Error(w, http.StatusBadRequest, 40001, "磁盘未检测到原盘结构（BDMV/ISO）")
		return
	}
	// 簇内已有产物=无需重扫
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if h.clusterHasBDInfo(ctx, req.ClientUID, req.SavePath, req.Name) {
		Success(w, map[string]interface{}{"enqueued": false, "reason": "簇内已有 BDInfo"})
		return
	}
	// 快照 size（超时自适应）+ 清失败记录（重新进入扫描中）
	var size int64
	h.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
		Where("client_uid = ? AND save_path = ? AND name = ?", req.ClientUID, req.SavePath, req.Name).
		Select("COALESCE(MAX(size),0)").Scan(&size)
	hashes := h.clusterHashes(ctx, req.ClientUID, req.SavePath, req.Name)
	if len(hashes) > 0 {
		if err := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
			Where("info_hash IN ?", hashes).
			Updates(map[string]interface{}{
				"bd_scan_failed_at": nil,
				"bd_scan_error":     "",
			}).Error; err != nil {
			h.logger.Warn("bdinfo rescan clear-failed failed",
				zap.String("name", req.Name), zap.Error(err))
		}
	}
	ok := h.bdinfoQueue.Enqueue(publish.BDInfoScanTask{
		ClientUID: req.ClientUID, SavePath: req.SavePath, Name: req.Name,
		DiscPath: discPath, Size: size,
	})
	h.logger.Info("bdinfo rescan requested",
		zap.String("name", req.Name), zap.Bool("enqueued", ok))
	Success(w, map[string]interface{}{"enqueued": ok})
}

// handleBDInfoQueueStatus §59.319 P3/D6：扫描队列状态（running/pending——
// 前端/运维轮询补扫进度）。
func (h *PublishTorrentsHandler) handleBDInfoQueueStatus(w http.ResponseWriter, r *http.Request) {
	if h.bdinfoQueue == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "BDInfo 扫描队列未配置")
		return
	}
	Success(w, h.bdinfoQueue.Status())
}
