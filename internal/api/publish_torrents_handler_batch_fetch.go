package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/description"
	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/metadata"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
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

func (h *PublishTorrentsHandler) runBatchFetch(clientUID uint, items []batchFetchItem, force bool) {
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("runBatchFetch panic", zap.Any("panic", r))
		}
		h.batchFetch.mu.Lock()
		h.batchFetch.active = false
		h.batchFetch.mu.Unlock()
	}()
	// §59.562 刀3: 颗内操作沿用 Background——graceful 语义（§59.495 用户定案：
	// 正在执行的颗完整跑完）·停机信号只作用于循环顶「不再开始新颗」·
	// 不中断当前颗的网络/落库（见循环顶 baseCtx.Done 检查点注释）。
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
		// §59.562 刀3: baseCtx（service ctx）Done 同语义位接入——停机信号
		// 与手动 stopRequested 等价：不开始新颗·当前颗完整跑完（graceful
		// 保留——审计 01 §3.1「跑完当前颗再停」）；颗内操作沿用 Background
		// 不被停机中断（现状语义零变化）。
		select {
		case <-h.baseCtx.Done():
			h.logger.Info("batch-fetch shutdown: remaining items skipped",
				zap.Int("remaining", len(items)-i))
			return // defer 释放 active
		default:
		}
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
		strings.Contains(s, "transient upstream") ||
		// §59.578 刀2: 瞬态扩四形态（站方临时宕机/DNS 抖动/连接重置——
		// 口径 A 烙印策略的配套：明显瞬态不烙·防误烙埋颗。注意本函数
		// 同源 retryDeferredBatch 的 deferred 判定——扩清单=更多 deferred
		// 二轮重试（行为变好·既有消费面有意识变更）。
		strings.Contains(s, "connection refused") ||
		strings.Contains(s, "no such host") || // DNS 解析失败（Go net 错误文案）
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "EOF")
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
func (h *PublishTorrentsHandler) classifySeedStatusLite(ctx context.Context, name string, meta *model.TorrentMetadata, isMusicSeed bool, fetchFailed bool) string {
	// ① flags 检查
	if meta != nil && meta.Flags != "" {
		var flags []string
		if err := json.Unmarshal([]byte(meta.Flags), &flags); err == nil {
			// §59.162: 限时禁转两态——now<until 才拦（keepfrds 24h 窗到期自动放行）
			for _, f := range flags {
				if f == "限时禁转" {
					if meta.NoTransferUntil == nil || meta.NoTransferUntil.After(time.Now()) {
						return "forbidden"
					}
				}
				// §59.539: 限转站规族（标记在=拦·站方解除后自动放行）——
				// 不进 gate（站规体系非权利体系·与发布门同口径）
				if f == "限转" {
					return "forbidden"
				}
			}
			// §59.593（语义重定义刀 3）: 权利族 gate（公共单点
			// NoTransferGateBlock——与发布门 checkFlagsFromMetadata 同口径
			// 防口径分裂）：组映射身份判定/描述域保守拦/退化二分支。
			// 列表判定=选出行（selectSourceMeta 组映射优先）+该行 gate——
			// 发布门聚合更严（多行全扫）——两层防御口径单向包含（发布门⊇
			// 列表·§59.548 复检实态化同款哲学）。
			if blocked, _ := publish.NoTransferGateBlock(h.sourceDetector, meta); blocked {
				return "forbidden"
			}
		}
	}

	// §59.578 刀2: 烙印②优先判定——认识但走链终态失败 → unsupported
	// （优先于身份判定·省注册表查询·且烙印是「已试过」的实证信号）。
	if fetchFailed {
		return "unsupported"
	}

	// ③ 制作组映射检查 §59.578（状态模型重构·用户定案）：
	// 未获取族二元身份——收录列表（官组∪外站注册表）命中 → unfetched
	// （可获取候选）；miss（含无组名）→ unsupported①（不认识·静态终态
	// ·隐含不支持转载发布）。原 no_mapping/external_group 状态消亡。
	// §59.230 已获取重判保留（PERFUME 案·站方标题权威）。
	// §59.469 附七: 音乐种跳过组映射（音乐标题通常无组名·音乐链定向
	// 音乐源站——组映射是影视获取通道语义）→ 落穿 unfetched。
	// §59.577 刀1: 三级回退链（站方发布页组名选择→标题→文件名笔误）
	// + 外站注册表第二查询保持。
	if h.sourceDetector != nil && !isMusicSeed {
		if meta == nil {
			// Ⅲ 未获取：快照名组名（发布者手写文件名·笔误容忍靠容错行）
			group := publish.ExtractGroupName(name)
			if group == "" {
				return "unsupported"
			}
			if h.sourceDetector.LookupGroup(ctx, group) == "" {
				if h.sourceDetector.LookupExternalGroup(ctx, group) {
					// §59.578 A 扩展：收录列表命中（注册表）→ unfetched
					// （可获取候选·簇/覆盖通道——SPHD 先例）
				} else {
					return "unsupported"
				}
			}
			// 官组映射命中 ∨ 注册表命中 → 落穿 unfetched
		} else if meta.Title != "" {
			// Ⅱ 标题组名（站方标题权威——§59.230 PERFUME 先例）
			group := publish.ExtractGroupName(meta.Title)
			// Ⅰ 站方发布页组名选择（release_group 列·标准键 team.xxx）：
			// 标题无组名时的救回通道（Ⅰ级只定组身份·不定源站——13city 型
			// 站点发布组下拉含外站组·PT-depiler 站定义实证）
			if group == "" {
				group = groupFromReleaseGroupSlot(meta.ReleaseGroup)
			}
			if group != "" {
				if h.sourceDetector.LookupGroup(ctx, group) == "" {
					if h.sourceDetector.LookupExternalGroup(ctx, group) {
						// §59.577 刀2 形态修正（§59.578 保留）：已获取外站组
						// 走状态流（④⑤⑥ reviewed/pending/incomplete·§59.230
						// 原则）。外站组身份并行字段 external_group（tag 消费）。
						// 未知组（双 miss）→ 不合成声明（诚实）。
					} else {
						// 站方标题有组名但不在收录列表 → unsupported①
						// （站方标题比裸名可信）
						return "unsupported"
					}
				}
			}
			// 站方标题无组名（电影等）或有映射（或外站组落穿）——均按获取后状态流展示
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
	// §59.577 刀1: 外部注册表归一查询（词条名标准写法·同 §59.550 显式
	// 收编语义——变体由注册表显式收编）
	h.db.WithContext(ctx).Model(&model.ExternalReleaseGroup{}).
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
// §59.577 刀1: 外部注册表词条并入（识别+展示共享·LookupGroup 语义不沾染）。
func (h *PublishTorrentsHandler) syncGroupLexicon(ctx context.Context) {
	var words []string
	h.db.WithContext(ctx).Model(&model.ReleaseGroupMapping{}).Pluck("group_name", &words)
	var extWords []string
	h.db.WithContext(ctx).Model(&model.ExternalReleaseGroup{}).Pluck("group_name", &extWords)
	words = append(words, extWords...)
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

// handleBatchFetch §59.20: 异步批量获取种子 metadata（"获取数据"按钮）。
func (h *PublishTorrentsHandler) handleBatchFetch(w http.ResponseWriter, r *http.Request) {
	if h.metadataFetcher == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "metadata fetcher 未初始化")
		return
	}
	if h.sourceDetector == nil {
		Error(w, http.StatusServiceUnavailable, 50001, "source detector 未初始化")
		return
	}

	var req struct {
		ClientUID uint `json:"clientId"`
		// §59.479: force=true 批量重获——绕过 hasCompleteMetadata 簇跳过
		// + forcePTGen 强刷（§59.173）。来源：禁转/无映射筛选视图「批量重获」
		// 按钮（该视图行多为已获取簇·常规批量获取会被簇跳过全部 no-op）。
		Force bool             `json:"force"`
		Items []batchFetchItem `json:"items"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if len(req.Items) == 0 {
		Error(w, http.StatusBadRequest, 40001, "items 为空")
		return
	}

	h.batchFetch.mu.Lock()
	if h.batchFetch.active {
		h.batchFetch.mu.Unlock()
		Error(w, http.StatusConflict, 40901, "已有批量获取任务在运行")
		return
	}
	h.batchFetch.active = true
	h.batchFetch.stopRequested = false // §59.495: 新批次重置停止请求
	h.batchFetch.total = len(req.Items)
	h.batchFetch.done = 0
	h.batchFetch.failed = 0
	h.batchFetch.items = make([]batchFetchItem, len(req.Items))
	for i, item := range req.Items {
		h.batchFetch.items[i] = batchFetchItem{Hash: item.Hash, Name: item.Name, Size: item.Size, SavePath: item.SavePath, Status: "pending"}
	}
	h.batchFetch.mu.Unlock()

	go h.runBatchFetch(req.ClientUID, req.Items, req.Force)

	Success(w, map[string]interface{}{"message": "已开始", "total": len(req.Items)})
}

// groupFromReleaseGroupSlot §59.577 刀1: Ⅰ级槽位值→组名。
// 标准键形态 team.xxx 剥前缀得组名（LOWER 查询兼容 HHWEB 大小写）；
// team.other（站方声明非本站官组）/ NOGROUP / 空 → 空（无组身份·落Ⅱ级）。
func groupFromReleaseGroupSlot(slot string) string {
	s := strings.TrimSpace(slot)
	if s == "" || s == "team.other" || s == "NOGROUP" {
		return ""
	}
	return strings.TrimPrefix(s, "team.")
}

// seedExternalGroupFlag §59.577 刀2: 外站组身份并行字段（非状态——
// 已获取外站组走状态流后·身份从状态剥离·tag 展示与「外站组」筛选桶消费）。
// 已获取种子：Ⅱ标题组名/Ⅰ级槽位救回 → LookupGroup miss ∧ 注册表命中。
// 未获取种子：status=external_group 本身即身份（tag 冗余·false）。
func (h *PublishTorrentsHandler) seedExternalGroupFlag(ctx context.Context, meta *model.TorrentMetadata) bool {
	if meta == nil || meta.Title == "" || h.sourceDetector == nil {
		return false
	}
	group := publish.ExtractGroupName(meta.Title)
	if group == "" {
		group = groupFromReleaseGroupSlot(meta.ReleaseGroup)
	}
	if group == "" || group == "NOGROUP" || group == "Other" {
		return false
	}
	if h.sourceDetector.LookupGroup(ctx, group) != "" {
		return false // 官组——非外站组
	}
	return h.sourceDetector.LookupExternalGroup(ctx, group)
}
