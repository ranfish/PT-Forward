package api

// §59.558 PR-3: fetchSingleTorrent 拆分产物——单条获取管线子域。
// 五阶段：coverage 保证/路由判定/源站选择/获取降级链/后处理。
// 原 537 行单函数（goto fetched 跨越获取与后处理）——获取链函数化后
// goto 语义等价消除（成功返回 meta·失败返回 err·主函数顺序衔接后处理）。
// 行为约束（报告 §4.2）：不改日志 key/调用顺序/截图-BDInfo-标签时序。

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/comment"
	"github.com/ranfish/pt-forward/internal/metadata"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"github.com/ranfish/pt-forward/internal/site"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"go.uber.org/zap"
)

// ensureFetchCoverage §59.337/§59.469 附五: coverage 读取 + tracker 行保证
// （未过期 tracker 行才算已有；无则单颗补拉 GetTrackers+MatchAll+写缓存）。
func (h *PublishTorrentsHandler) ensureFetchCoverage(ctx context.Context, clientUID uint, hash string) []model.SiteCoverageCache {
	var coverageSites []model.SiteCoverageCache
	if h.coverage != nil {
		cs, err := h.coverage.GetCachedCoverage(ctx, hash)
		if err == nil {
			coverageSites = cs
		}
	}
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
	return coverageSites
}

// fetchRouting §59.469/§59.476 路由判定产物。
type fetchRouting struct {
	musicRouting   bool
	coverageSites  []model.SiteCoverageCache
	clusterTargets []comment.DirectTarget
}

// resolveFetchRouting §59.61/§59.469: 簇 comment 候选构建 + 音乐链分流判定
// （S1 coverage∩音乐源站 ∨ S2+ 标题音乐官组词 → 输入过滤音乐源站内）。
func (h *PublishTorrentsHandler) resolveFetchRouting(ctx context.Context, clientUID uint, savePath, name, hash string, coverageSites []model.SiteCoverageCache) fetchRouting {
	clusterTargets := h.buildClusterTargets(ctx, clientUID, savePath, name, hash)
	fr := fetchRouting{coverageSites: coverageSites, clusterTargets: clusterTargets}
	if h.isMusicSeedForRouting(ctx, name, coverageSites) {
		fr.musicRouting = true
		fr.coverageSites = filterToMusicSources(coverageSites)
		fr.clusterTargets = filterClusterToMusicSources(clusterTargets)
		h.logger.Info("music chain routing",
			zap.String("hash", hash),
			zap.Int("coverage_music", len(fr.coverageSites)),
			zap.Int("cluster_music", len(fr.clusterTargets)))
	}
	return fr
}

// selectFetchSourceSite §59.476: 源站选择 + 音乐链兜底站 synth
// （纯流转种五层全 miss 时 synth 首选音乐源站走 ①b 搜索分支）。
func (h *PublishTorrentsHandler) selectFetchSourceSite(ctx context.Context, name string, fr fetchRouting, hash string) *publish.SourceDetectResult {
	result := h.sourceDetector.SelectFetchSite(ctx, name, fr.coverageSites, fr.clusterTargets, hash)
	if result.SourceSite == "" && fr.musicRouting {
		if ms := h.musicFallbackSites(ctx); len(ms) > 0 {
			result.SourceSite = ms[0]
			result.AutoDetected = true
			h.logger.Info("music fallback site synthesized",
				zap.String("hash", hash[:min(10, len(hash))]),
				zap.String("site", ms[0]))
		}
	}
	if result.SourceSite == "" {
		return nil
	}
	return &result
}

// fetchMetadataChain §59.65/§59.330/§59.337/§59.231: 获取降级链（原 goto
// fetched 双分支函数化）。直达分支：直达 90s→簇 comment 逐个→IYUU coverage
// →音乐桥→同站搜索；①b 搜索分支：搜索 25s→簇 comment→IYUU→音乐桥。
func (h *PublishTorrentsHandler) fetchMetadataChain(fetchCtx context.Context, result *publish.SourceDetectResult, fr fetchRouting, hash, name string, size int64, clientUID uint, savePath, localMI string) (*model.TorrentMetadata, error) {
	var meta *model.TorrentMetadata
	var err error
	if result.TorrentID != "" {
		// §59.338: 直达预算 25s→90s——fallback 独立预算不烧本 ctx·但 enrich/store
		// 继续用本 ctx（243 实证 90s 真实工作被 25s 绞杀在 store 最后一步）。
		directCtx, directCancel := context.WithTimeout(fetchCtx, 90*time.Second)
		meta, err = h.metadataFetcher.FetchAndStoreDirect(directCtx, hash, result.SourceSite, result.TorrentID, name)
		directCancel()
		if err == nil {
			return meta, nil
		}
		transientSeen := isTransientFetchErr(err)
		triedSites := []string{result.SourceSite}
		h.logger.Info("direct fetch failed, fallback chain start",
			zap.String("hash", hash[:min(10, len(hash))]),
			zap.String("site", result.SourceSite),
			zap.Error(err))
		// ② ②a: 簇内其他副本 comment 凭证逐个直达（§59.341 PTGen 不可行换下一凭证）
		for _, tgt := range fr.clusterTargets {
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
					return meta, nil
				}
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
		// ③ IYUU coverage 直达（本地缓存挑未试站）
		if err != nil {
			h.logger.Info("cluster comments exhausted, trying IYUU coverage direct",
				zap.String("hash", hash[:min(10, len(hash))]),
				zap.Strings("tried_sites", triedSites))
			meta, err = h.metadataFetcher.FetchAndStoreIYUU(fetchCtx, hash, triedSites, h.musicFetchArgs(fr.musicRouting, localMI)...)
			if err == nil {
				h.logger.Info("IYUU coverage direct hit",
					zap.String("hash", hash[:min(10, len(hash))]),
					zap.String("site", meta.SiteName))
				return meta, nil
			}
			// §59.482 刀7: 音乐链第三兜底
			if fr.musicRouting {
				if m := h.musicSiblingTitleBridge(fetchCtx, clientUID, savePath, name, hash, size, result.SourceSite, triedSites); m != nil {
					meta, err = m, nil
					h.logger.Info("music sibling title bridge hit",
						zap.String("hash", hash[:min(10, len(hash))]),
						zap.String("site", m.SiteName))
					return meta, nil
				}
			}
			if isTransientFetchErr(err) {
				transientSeen = true
			}
		}
		// ④ 同站搜索（五闸门验证）
		if err != nil {
			h.logger.Info("IYUU coverage exhausted, trying same-site search",
				zap.String("hash", hash[:min(10, len(hash))]),
				zap.Error(err))
			meta, err = h.metadataFetcher.FetchAndStoreBySearch(fetchCtx, hash, result.SourceSite, name, size, h.musicFetchArgs(fr.musicRouting, localMI)...)
			if err != nil {
				if transientSeen {
					if !isTransientFetchErr(err) {
						return nil, fmt.Errorf("transient upstream (direct/cluster/iyuu/search chain): %w", err)
					}
				}
				return nil, err
			}
		}
		return meta, nil
	}
	// §59.231 ①b: 组映射_搜索模式（tid 空）——搜索失败→簇 comment→IYUU。
	searchSuffix := ""
	if !fr.musicRouting {
		searchSuffix = h.canonicalGroupName(fetchCtx, result.GroupName)
	}
	searchCtx, searchCancel := context.WithTimeout(fetchCtx, 25*time.Second)
	meta, err = h.metadataFetcher.FetchAndStoreBySearch(searchCtx, hash, result.SourceSite, name, size, h.musicFetchArgs(fr.musicRouting, localMI, searchSuffix)...)
	searchCancel()
	if err == nil {
		return meta, nil
	}
	h.logger.Info("mapped-site search failed, trying cluster comments",
		zap.String("hash", hash[:min(10, len(hash))]),
		zap.String("site", result.SourceSite),
		zap.Error(err))
	triedSites := []string{result.SourceSite}
	for _, tgt := range fr.clusterTargets {
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
			err = fmt.Errorf("ptgen not viable on %s (data quality fallback)", tgt.SiteName)
		}
		triedSites = append(triedSites, tgt.SiteName)
	}
	if err != nil {
		h.logger.Info("cluster comments exhausted, IYUU fallback (mapped-site search path)",
			zap.String("hash", hash[:min(10, len(hash))]),
			zap.Strings("tried_sites", triedSites),
			zap.Error(err))
		meta, err = h.metadataFetcher.FetchAndStoreIYUU(fetchCtx, hash, triedSites, h.musicFetchArgs(fr.musicRouting, localMI)...)
		if err != nil && fr.musicRouting {
			if m := h.musicSiblingTitleBridge(fetchCtx, clientUID, savePath, name, hash, size, result.SourceSite, triedSites); m != nil {
				meta, err = m, nil
				h.logger.Info("music sibling title bridge hit",
					zap.String("hash", hash[:min(10, len(hash))]),
					zap.String("site", m.SiteName))
			}
		}
	}
	if err != nil {
		return nil, err
	}
	return meta, nil
}

// preFetchLocalArtifacts §59.36/§59.319 P1: 原盘识别门（isDisc+BDInfo 入队）
// + 本地 MI 预取（90s 限时·音频仲裁源侧证据·失败不阻断）。
// 返回 (isDisc, localMI)。
func (h *PublishTorrentsHandler) preFetchLocalArtifacts(ctx context.Context, clientUID uint, hash, name, savePath string, size int64, isLocal bool) (bool, string) {
	isDisc := false
	if isLocal && savePath != "" && h.bdinfoScanner != nil {
		if discPath := h.bdinfoScanner.DetectDiscPath(savePath, name); discPath != "" {
			isDisc = true
			if h.bdinfoQueue != nil && !h.clusterHasBDInfo(ctx, clientUID, savePath, name) {
				h.bdinfoQueue.Enqueue(publish.BDInfoScanTask{
					ClientUID: clientUID, SavePath: savePath, Name: name, DiscPath: discPath,
					Size: size,
				})
			}
		}
	}
	var localMI string
	if isLocal && !isDisc && savePath != "" && h.seedPipeline != nil {
		analyzeCtx, analyzeCancel := context.WithTimeout(ctx, 90*time.Second)
		artifacts, artErr := h.seedPipeline.AnalyzeLocalArtifacts(analyzeCtx, name, savePath)
		analyzeCancel()
		if artErr != nil {
			h.logger.Warn("batch-fetch: pre-fetch local mediainfo failed", zap.String("hash", hash), zap.Error(artErr))
		} else if mi, ok := artifacts["media_info"]; ok {
			localMI, _ = mi.(string)
		}
	}
	return isDisc, localMI
}

// runFetchPostArtifacts §59.49/§59.490/§59.21/§59.236: 后处理三件——
// 截图策略（音乐种跳过）+ localMI 落库簇传播 + 主链 PTGen。
func (h *PublishTorrentsHandler) runFetchPostArtifacts(ctx context.Context, meta *model.TorrentMetadata, clientUID uint, savePath, name string, isLocal, force bool, localMI string) {
	// §59.49+§59.57+§59.490: 截图探活与策略
	if meta != nil && !metadata.HasMusic(meta.DetailSourceJSON) {
		if h.shotStrategy != nil && savePath != "" {
			go h.applyScreenshotStrategy(clientUID, meta.InfoHash, meta.SiteName, name, savePath, isLocal, force)
		} else if meta.Screenshots != "" && meta.Screenshots != "[]" {
			go h.purgeDeadScreenshots(meta.InfoHash, meta.SiteName)
		}
	}
	// §59.21/§59.56: 本地 MI 落库 + §59.171 B 簇传播
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
			propagateClusterMediainfoDB(h.db, h.logger, ctx, clientUID, savePath, name, meta.InfoHash, localMI)
		}
	}
	// §59.236 ①/§59.286: 主链 PTGen 单点单责
	_ = h.runMainlinePTGen(ctx, meta, force)
}

// finalizeFetchProfile §59.26/§59.77/§59.514 附八/§59.500 刀1: 终态 Profile
// 合并+分类推断+声明过滤+flags+updates 落库+重组标题簇传播+inferFinalTags。
func (h *PublishTorrentsHandler) finalizeFetchProfile(ctx context.Context, meta *model.TorrentMetadata, clientUID uint, savePath, name string, isLocal bool) {
	if meta == nil {
		return
	}
	var finalMeta model.TorrentMetadata
	h.db.WithContext(ctx).Where("info_hash = ? AND site_name = ?", meta.InfoHash, meta.SiteName).First(&finalMeta)
	if finalMeta.Title == "" {
		return
	}
	// §59.26 三源合并：MI 优先 local·fallback 源站；§59.319 P1 原盘优先 BDInfo
	miForProfile := finalMeta.BDInfo
	if miForProfile == "" {
		miForProfile = finalMeta.MediaInfo
	}
	if miForProfile == "" {
		miForProfile = finalMeta.SourceMediaInfo
	}
	domMedium, domRes, domVideo, domAudio := titleparser.DOMFieldsFromDetailSource(finalMeta.DetailSourceJSON)
	profile := titleparser.BuildTechProfile(finalMeta.Title, miForProfile, domMedium, domRes, domVideo, domAudio)
	profile.AudioTracks = titleparser.AdjustCommentaryTracks(profile.AudioTracks, finalMeta.Subtitle, miForProfile)
	// §59.514 附八: 重组标题资产化（ExportTorrent 顺路拉·isLocal 场景毫秒级）
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
		"audio_tracks":      profile.AudioTracks,
		"hdr":               profile.HDR,
		"bit_depth":         profile.BitDepth,
		"source_type":       profile.SourceType,
		"specification":     profile.Specification,
		"source_platform":   profile.SourcePlatform,
		"edition_info":      profile.EditionInfo,
		"region_code":       profile.RegionCode,
	}
	// §59.265 附: statement 列同步过滤（先滤后接致谢基线）
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
	// §59.27: flags 以本次 detail 为准（map 显式覆盖空值）
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
	_ = profile
	h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
		Where("info_hash = ? AND site_name = ?", meta.InfoHash, meta.SiteName).
		Updates(updates)
	// §59.514 附八: 重组标题簇传播 + §59.500 刀1: inferFinalTags 单点
	if reassembledTitle != "" {
		propagateClusterTitleDB(h.db, h.logger, ctx, clientUID, savePath, name, meta.InfoHash,
			reassembledTitle, strings.Join(reassembledNotes, "；"))
	}
	h.inferFinalTags(ctx, clientUID, meta.InfoHash, meta.SiteName)
}

// fetchSingleTorrent §59.558 PR-3 拆分后编排（1 屏）：
// 准备（coverage/路由/源站/预算/本地工件）→ 获取降级链 → 后处理 → 终态 Profile。
func (h *PublishTorrentsHandler) fetchSingleTorrent(ctx context.Context, clientUID uint, hash, name string, size int64, savePath string, isLocal bool, forcePTGen ...bool) error {
	force := len(forcePTGen) > 0 && forcePTGen[0]

	// §59.578 刀2: 烙印基线——首获（无 metadata）才可能烙印（重获失败
	// 数据在手可发布·不烙——设计定案边界）。查询在链前（获取链自身
	// INSERT/UPDATE metadata·链后查会误判）。
	var metaCount int64
	if h.db != nil {
		h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
			Where("info_hash = ?", hash).Count(&metaCount)
	}

	coverageSites := h.ensureFetchCoverage(ctx, clientUID, hash)
	fr := h.resolveFetchRouting(ctx, clientUID, savePath, name, hash, coverageSites)
	result := h.selectFetchSourceSite(ctx, name, fr, hash)
	if result == nil {
		h.markFetchFailed(ctx, clientUID, hash, metaCount == 0, fmt.Errorf("无可用源站（制作组未映射 + 无覆盖）"))
		return fmt.Errorf("无可用源站（制作组未映射 + 无覆盖）")
	}

	fetchCtx, cancel := context.WithTimeout(ctx, 100*time.Second)
	defer cancel()

	isDisc, localMI := h.preFetchLocalArtifacts(ctx, clientUID, hash, name, savePath, size, isLocal)
	_ = isDisc

	meta, err := h.fetchMetadataChain(fetchCtx, result, fr, hash, name, size, clientUID, savePath, localMI)
	if err != nil {
		h.markFetchFailed(ctx, clientUID, hash, metaCount == 0, err)
		return err
	}

	// §59.578 刀2: 成功清印（簇级）——获取成功=走过且成功的语义终结
	// （残留烙印会让「清除 metadata→弹窗重现」通道断裂）。
	h.clearFetchBrand(ctx, clientUID, savePath, name)

	h.runFetchPostArtifacts(ctx, meta, clientUID, savePath, name, isLocal, force, localMI)
	h.finalizeFetchProfile(ctx, meta, clientUID, savePath, name, isLocal)
	return nil
}

// clearFetchBrandByHash §59.578 刀2: 单 hash 清烙印（deleteSeed 无簇
// 直删分支——快照 name 为空的兜底路径）。
func (h *PublishTorrentsHandler) clearFetchBrandByHash(ctx context.Context, hash string) {
	if h.db == nil || hash == "" {
		return
	}
	if uerr := h.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
		Where("hash = ? AND fetch_failed_at IS NOT NULL", hash).
		Updates(map[string]interface{}{
			"fetch_failed_at": nil,
			"fetch_error":     "",
		}).Error; uerr != nil && h.logger != nil {
		h.logger.Warn("fetch brand clear failed", zap.String("hash", hash), zap.Error(uerr))
	}
}

// markFetchFailed §59.578 刀2: 获取失败烙印（口径 A·用户定案）——
// 瞬态（isTransientFetchErr 含扩四形态）外全烙。三不烙边界：
// ① 重获失败（fresh=false·数据在手）② 瞬态失败 ③ DB 更新自身失败
// （Warn 不静默·不反转获取终态）。写=当前 hash（弹窗 name 级排除靠
// 「任一 hash 烙印」语义覆盖·清除簇级全清）。
func (h *PublishTorrentsHandler) markFetchFailed(ctx context.Context, clientUID uint, hash string, fresh bool, err error) {
	if h.db == nil || !fresh || err == nil || isTransientFetchErr(err) {
		return
	}
	fetchErr := err.Error()
	if len(fetchErr) > 500 { // 列宽 size:500
		fetchErr = fetchErr[:500]
	}
	if uerr := h.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
		Where("hash = ? AND client_uid = ?", hash, clientUID).
		Updates(map[string]interface{}{
			"fetch_failed_at": time.Now(),
			"fetch_error":     fetchErr,
		}).Error; uerr != nil && h.logger != nil {
		h.logger.Warn("fetch brand update failed", zap.String("hash", hash), zap.Error(uerr))
	}
}

// clearFetchBrand §59.578 刀2: 清烙印（簇级·紧键域 client+path+name）
// ——获取成功与 handleDeleteSeed 共用。幂等（无烙印行零变化零噪音）。
func (h *PublishTorrentsHandler) clearFetchBrand(ctx context.Context, clientUID uint, savePath, name string) {
	if h.db == nil || name == "" {
		return
	}
	if uerr := h.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
		Where("client_uid = ? AND save_path = ? AND name = ? AND fetch_failed_at IS NOT NULL",
			clientUID, savePath, name).
		Updates(map[string]interface{}{
			"fetch_failed_at": nil,
			"fetch_error":     "",
		}).Error; uerr != nil && h.logger != nil {
		h.logger.Warn("fetch brand clear failed", zap.String("name", name), zap.Error(uerr))
	}
}
