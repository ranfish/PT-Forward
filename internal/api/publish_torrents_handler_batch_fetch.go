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
		Force bool `json:"force"`
		Items []struct {
			Hash     string `json:"hash"`
			Name     string `json:"name"`
			Size     int64  `json:"size"`
			SavePath string `json:"savePath"`
		} `json:"items"`
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
