package api

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ranfish/pt-forward/internal/metadata"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"go.uber.org/zap"
)

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
		if cached, ok := h.lookupScreenshotCache(ctx, clientUID, savePath, name); ok {
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
