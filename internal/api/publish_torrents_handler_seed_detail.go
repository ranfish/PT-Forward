package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/description"
	"github.com/ranfish/pt-forward/internal/fingerprint"
	"github.com/ranfish/pt-forward/internal/metadata"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"go.uber.org/zap"
)

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
		// §59.565 刀D1: 5xx 透传收敛
		h.logger.Error("get site config failed", zap.String("op", "get_site_config"), zap.String("site", req.Site), zap.Error(err))
		Error(w, http.StatusBadGateway, 50201, "获取站点配置失败，请查看服务端日志")
		return
	}
	adapter, err := h.siteProvider.GetAdapter(ctx, req.Site)
	if err != nil || adapter == nil {
		// §59.565 刀D1
		h.logger.Error("get adapter failed", zap.String("op", "get_adapter"), zap.String("site", req.Site), zap.Error(err))
		Error(w, http.StatusBadGateway, 50202, "获取适配器失败，请查看服务端日志")
		return
	}
	data, err := adapter.DownloadTorrent(ctx, config, req.TorrentID)
	if err != nil {
		// §59.565 刀D1
		h.logger.Error("download torrent failed", zap.String("op", "download_torrent"), zap.String("site", req.Site), zap.String("torrent_id", req.TorrentID), zap.Error(err))
		Error(w, http.StatusBadGateway, 50203, "下载种子文件失败，请查看服务端日志")
		return
	}
	meta, err := fingerprint.ComputeFromTorrent(data)
	if err != nil || meta == nil {
		// §59.565 刀D1
		h.logger.Error("parse torrent failed", zap.String("op", "parse_torrent"), zap.String("torrent_id", req.TorrentID), zap.Error(err))
		Error(w, http.StatusInternalServerError, 50002, "解析种子文件失败，请查看服务端日志")
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
		// §59.565 刀D1
		h.logger.Error("fetch single torrent failed", zap.String("op", "fetch_single"), zap.Uint("client_id", clientUID), zap.String("info_hash", infoHash), zap.Error(err))
		Error(w, http.StatusInternalServerError, 50000, "获取种子失败，请查看服务端日志")
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
