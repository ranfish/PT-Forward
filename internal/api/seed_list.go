package api

// §59.558 PR-2: handleListSeeds 拆分产物——种子列表子域（编排+阶段 helper）。
// 原 682 行单函数按 12 阶段抽：参数解析/发布簇集/快照装载/元数据装载/
// 簇索引构建/覆盖四色装载/副本计数/行组装循环/尾段收尾。
// 行为锚：seed_list_filter_test.go 九态+observing（拆分前后同绿）。

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"unicode"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"github.com/ranfish/pt-forward/internal/util"
	"go.uber.org/zap"

	"context"

	"github.com/ranfish/pt-forward/internal/site"
)

// listSeedsQuery 种子列表筛选参数聚合（§59.558 PR-2 抽取自 handleListSeeds 头部）。
type listSeedsQuery struct {
	clientUID         uint
	savePath          string
	statusFilter      string
	search            string
	readyFilter       string // §59.131 ②: ready 过滤（簇级=reviewed）
	excludeForbidden  bool   // §59.143: 发布页排除源站禁转
	publishState      string // §59.166: publishable/published
	publishTargetSite string
	page, pageSize    int
}

// parseListSeedsQuery 参数解析与默认值。
func parseListSeedsQuery(r *http.Request) listSeedsQuery {
	q := listSeedsQuery{}
	q.clientUID = parseUintQuery(r, "client_id", 0) // §59.559 PR-D
	q.savePath = r.URL.Query().Get("save_path")
	q.statusFilter = r.URL.Query().Get("status")
	q.search = strings.TrimSpace(r.URL.Query().Get("search"))
	q.readyFilter = r.URL.Query().Get("ready")
	q.excludeForbidden = parseBoolQuery(r, "exclude_forbidden", false) // §59.559 PR-D
	q.publishState = r.URL.Query().Get("publish_state")
	q.publishTargetSite = r.URL.Query().Get("target_site")
	// §59.552 变体：默认 50+上限 100 为 §59.166 单批发布业务定案（不迁标准 200）
	q.page, q.pageSize = parsePaginationWith(r, 50, 100, 10000)
	return q
}

// loadPublishedClusterSet §59.166/§59.547: 目标站已发布簇集合（终态记录面）。
// 填充条件宽化为 target 在场（§59.547——target+无 state 时 site_tags 选中站
// 置顶判断不漏记录面）。
func (h *PublishTorrentsHandler) loadPublishedClusterSet(ctx context.Context, targetSite string) map[string]bool {
	if targetSite == "" {
		return nil
	}
	var pubHashes []string
	h.db.WithContext(ctx).
		Table("publish_result_records").
		Where("target_site = ? AND source_info_hash != '' AND status IN ?",
			targetSite, model.PublishedListStatuses).
		Distinct("source_info_hash").
		Pluck("source_info_hash", &pubHashes)
	publishedClusters := make(map[string]bool)
	if len(pubHashes) > 0 {
		type ckRow struct {
			ClientUID uint
			SavePath  string
			Name      string
		}
		var cks []ckRow
		h.db.WithContext(ctx).
			Table("torrent_snapshots").
			Select("DISTINCT client_uid, save_path, name").
			Where("hash IN ? AND is_hidden = 0 AND name != ''", pubHashes).
			Find(&cks)
		for _, c := range cks {
			publishedClusters[fmt.Sprintf("%d|%s|%s", c.ClientUID, c.SavePath, c.Name)] = true
		}
	}
	return publishedClusters
}

// loadSeedSnapshots §59.26/§59.29/§59.537: 快照主查询（SQL 分组去重+进度门+
// 搜索三域下推）+ Go 层双保险去重。
func (h *PublishTorrentsHandler) loadSeedSnapshots(ctx context.Context, q listSeedsQuery) []model.TorrentSnapshot {
	snapQuery := h.db.WithContext(ctx).
		Table("torrent_snapshots AS s").
		Select("s.*").
		Where("s.is_hidden = ?", false).
		Where("s.progress >= ?", 1.0)
	sub := h.db.WithContext(ctx).
		Table("torrent_snapshots").
		Select("MAX(id) AS id").
		Where("is_hidden = ?", false).
		Where("progress >= ?", 1.0).
		Group("client_uid, save_path, CASE WHEN name = '' THEN hash ELSE name END")
	if q.clientUID != 0 {
		snapQuery = snapQuery.Where("s.client_uid = ?", q.clientUID)
		sub = sub.Where("client_uid = ?", q.clientUID)
	}
	if q.savePath != "" {
		snapQuery = snapQuery.Where("s.save_path = ?", q.savePath)
		sub = sub.Where("save_path = ?", q.savePath)
	}
	// §59.138: 三域搜索——CJK 词跨表扩 title/subtitle；拉丁词纯 name LIKE。
	searchHasCJK := false
	for _, r := range q.search {
		if unicode.Is(unicode.Han, r) {
			searchHasCJK = true
			break
		}
	}
	var searchConds []string
	var searchArgs []interface{}
	if q.search != "" {
		// 13 刀1（v0.0.1656）: 用户输入 LIKE 转义收编（V-01——原裸拼
		// "%"+search+"%"·输入 %/_ 命中全表劫持过滤）。每个 LIKE ? 配
		// ESCAPE 子句（含子查询内两处）。
		if searchHasCJK {
			searchConds = []string{"name LIKE ?" + util.LikeEscapeSuffix, "OR name IN (SELECT s2.name FROM torrent_snapshots s2 JOIN torrent_metadata m ON m.info_hash = s2.hash WHERE m.title LIKE ?" + util.LikeEscapeSuffix + " OR m.subtitle LIKE ?" + util.LikeEscapeSuffix + ")"}
			esc := util.LikeContains(q.search)
			searchArgs = []interface{}{esc, esc, esc}
		} else {
			searchConds = []string{"name LIKE ?" + util.LikeEscapeSuffix}
			searchArgs = []interface{}{util.LikeContains(q.search)}
		}
		sub = sub.Where(strings.Join(searchConds, " "), searchArgs...)
	}
	var rawSnapshots []model.TorrentSnapshot
	if err := snapQuery.
		Where("s.id IN (?)", sub).
		Order("s.updated_at DESC").
		Find(&rawSnapshots).Error; err != nil {
		return nil
	}
	// §59.26/§59.29: Go 层去重双保险 + 保序。
	return util.DedupByKey(rawSnapshots, func(s model.TorrentSnapshot) string {
		if s.Name != "" {
			return fmt.Sprintf("%d|%s", s.ClientUID, s.SavePath) + "|" + s.Name
		}
		return s.Hash
	})
}

// seedMusicInfo §59.469 附七/附九: 音乐批量预查行（artist/album）。
type seedMusicInfo struct {
	InfoHash string
	Artist   string
	Album    string
}

// loadSeedMetas §59.29/§59.316 附 C/§59.469: 簇关联元数据（紧键三元组子查询）
// + 音乐 hash 批量预查。返回 metas 与 musicMeta（info_hash→artist/album）。
func (h *PublishTorrentsHandler) loadSeedMetas(ctx context.Context, snapshots []model.TorrentSnapshot) ([]model.TorrentMetadata, map[string]seedMusicInfo) {
	snapIDs := make([]uint, 0, len(snapshots))
	for _, s := range snapshots {
		snapIDs = append(snapIDs, s.ID)
	}
	keySub := h.db.WithContext(ctx).
		Table("torrent_snapshots").
		Select("client_uid, save_path, name").
		Where("id IN ?", snapIDs)
	var metas []model.TorrentMetadata
	if err := h.db.WithContext(ctx).
		Where("info_hash IN (SELECT hash FROM torrent_snapshots WHERE (client_uid, save_path, name) IN (?) AND is_hidden = 0)", keySub).
		Find(&metas).Error; err != nil {
		if h.logger != nil {
			h.logger.Error("listSeeds metas join failed", zap.Error(err))
		}
		metas = nil
	}
	var musicRows []seedMusicInfo
	musicMeta := make(map[string]seedMusicInfo)
	if len(metas) > 0 {
		hashes := make([]string, 0, len(metas))
		for _, m := range metas {
			hashes = append(hashes, m.InfoHash)
		}
		if err := h.db.WithContext(ctx).Model(&model.TorrentMetadata{}).
			Select("info_hash, json_extract(detail_source_json, '$.music.artist') AS artist, json_extract(detail_source_json, '$.music.album') AS album").
			Where("info_hash IN ? AND detail_source_json LIKE ?", hashes, `%"music":{%`).
			Scan(&musicRows).Error; err != nil && h.logger != nil {
			h.logger.Warn("listSeeds music hash precheck failed, fallback to video gate", zap.Error(err))
		}
		for _, row := range musicRows {
			musicMeta[row.InfoHash] = row
		}
	}
	return metas, musicMeta
}

// seedClusterIndex 簇索引三表（§59.38/§59.521）。
type seedClusterIndex struct {
	tripleByHash    map[string]string                  // hash→紧键（域放宽到 (client,path) 全部活跃行）
	domainByCluster map[string][]string                // 紧键→tracker 域名聚合（做种面）
	metaByCluster   map[string][]model.TorrentMetadata // 紧键→簇元数据行
}

// buildSeedClusterIndex §59.38/§59.521: hashQ 映射域构建 + meta 簇关联。
func (h *PublishTorrentsHandler) buildSeedClusterIndex(ctx context.Context, q listSeedsQuery, metas []model.TorrentMetadata) *seedClusterIndex {
	idx := &seedClusterIndex{
		tripleByHash:    make(map[string]string),
		domainByCluster: make(map[string][]string),
		metaByCluster:   make(map[string][]model.TorrentMetadata),
	}
	var hashKeyRows []struct {
		Hash          string
		ClientUID     uint
		SavePath      string
		Name          string
		TrackerDomain string
	}
	hashQ := h.db.WithContext(ctx).
		Table("torrent_snapshots").
		Select("hash, client_uid, save_path, name, tracker_domain").
		Where("is_hidden = ? AND name != ''", false)
	if q.clientUID != 0 {
		hashQ = hashQ.Where("client_uid = ?", q.clientUID)
	}
	if q.savePath != "" {
		hashQ = hashQ.Where("save_path = ?", q.savePath)
	}
	if q.search != "" {
		searchHasCJK := false
		for _, r := range q.search {
			if unicode.Is(unicode.Han, r) {
				searchHasCJK = true
				break
			}
		}
		var searchConds []string
		var searchArgs []interface{}
		if searchHasCJK {
			// 13 刀1（v0.0.1656）: 同上组——转义+ESCAPE 成对收编。
			searchConds = []string{"name LIKE ?" + util.LikeEscapeSuffix, "OR name IN (SELECT s2.name FROM torrent_snapshots s2 JOIN torrent_metadata m ON m.info_hash = s2.hash WHERE m.title LIKE ?" + util.LikeEscapeSuffix + " OR m.subtitle LIKE ?" + util.LikeEscapeSuffix + ")"}
			esc := util.LikeContains(q.search)
			searchArgs = []interface{}{esc, esc, esc}
		} else {
			searchConds = []string{"name LIKE ?" + util.LikeEscapeSuffix}
			searchArgs = []interface{}{util.LikeContains(q.search)}
		}
		hashQ = hashQ.Where(strings.Join(searchConds, " "), searchArgs...)
	}
	hashQ.Find(&hashKeyRows)
	for _, r := range hashKeyRows {
		key := fmt.Sprintf("%d|%s|%s", r.ClientUID, r.SavePath, r.Name)
		idx.tripleByHash[r.Hash] = key
		if r.TrackerDomain != "" {
			idx.domainByCluster[key] = append(idx.domainByCluster[key], r.TrackerDomain)
		}
	}
	for _, m := range metas {
		if key, ok := idx.tripleByHash[m.InfoHash]; ok {
			idx.metaByCluster[key] = append(idx.metaByCluster[key], m)
		}
	}
	return idx
}

// seedCoverageData §59.547 三态筛选覆盖面 + 四色数据。
type seedCoverageData struct {
	coverageTarget   map[string]bool            // 紧键→目标站覆盖面命中（筛选面2）
	oursByCluster    map[string]map[string]bool // 四色绿（我们发过的站）
	coveredByCluster map[string]map[string]bool // 四色黄（覆盖面站集合）
}

// loadSeedCoverage §59.547: 覆盖面预查（500/批）+ 四色绿预查。
func (h *PublishTorrentsHandler) loadSeedCoverage(ctx context.Context, idx *seedClusterIndex, targetSite string) *seedCoverageData {
	cd := &seedCoverageData{
		coverageTarget:   map[string]bool{},
		oursByCluster:    map[string]map[string]bool{},
		coveredByCluster: map[string]map[string]bool{},
	}
	if targetSite == "" {
		return cd
	}
	domainHashes := make([]string, 0, len(idx.tripleByHash))
	for hh := range idx.tripleByHash {
		domainHashes = append(domainHashes, hh)
	}
	if len(domainHashes) == 0 {
		return cd
	}
	type covRow struct {
		InfoHash string
		SiteName string
	}
	var covRows []covRow
	for _, batch := range util.ChunkStrings(domainHashes, 500) {
		var br []covRow
		h.db.WithContext(ctx).
			Table("site_coverage_cache").
			Select("info_hash, site_name").
			Where("info_hash IN ? AND status IN ?", batch, model.CoverageHasStatuses).
			Find(&br)
		covRows = append(covRows, br...)
	}
	for _, cr := range covRows {
		key := idx.tripleByHash[cr.InfoHash]
		if key == "" {
			continue
		}
		if cd.coveredByCluster[key] == nil {
			cd.coveredByCluster[key] = map[string]bool{}
		}
		cd.coveredByCluster[key][cr.SiteName] = true
		if cr.SiteName == targetSite {
			cd.coverageTarget[key] = true
		}
	}
	type oursRow struct {
		SourceInfoHash string
		TargetSite     string
	}
	var oursRows []oursRow
	for _, batch := range util.ChunkStrings(domainHashes, 500) {
		var br []oursRow
		h.db.WithContext(ctx).
			Table("publish_result_records").
			Select("source_info_hash, target_site").
			Where("source_info_hash IN ? AND target_site != '' AND status IN ?",
				batch, model.PublishSkipStatuses).
			Find(&br)
		oursRows = append(oursRows, br...)
	}
	for _, or := range oursRows {
		key := idx.tripleByHash[or.SourceInfoHash]
		if key == "" {
			continue
		}
		if cd.oursByCluster[key] == nil {
			cd.oursByCluster[key] = map[string]bool{}
		}
		cd.oursByCluster[key][or.TargetSite] = true
	}
	return cd
}

// loadClusterCopyCounts §59.131 ②: 簇副本数（簇成员数与搜索无关）。
func (h *PublishTorrentsHandler) loadClusterCopyCounts(ctx context.Context, q listSeedsQuery) map[string]int {
	type clusterCountRow struct {
		ClientUID uint
		SavePath  string
		Name      string
		Cnt       int
	}
	var countRows []clusterCountRow
	countQ := h.db.WithContext(ctx).
		Table("torrent_snapshots").
		Select("client_uid, save_path, name, COUNT(*) AS cnt").
		Where("is_hidden = ? AND name != ''", false)
	if q.clientUID != 0 {
		countQ = countQ.Where("client_uid = ?", q.clientUID)
	}
	if q.savePath != "" {
		countQ = countQ.Where("save_path = ?", q.savePath)
	}
	countQ.Group("client_uid, save_path, name").Find(&countRows)
	copyCountByKey := make(map[string]int, len(countRows))
	for _, cr := range countRows {
		copyCountByKey[fmt.Sprintf("%d|%s|%s", cr.ClientUID, cr.SavePath, cr.Name)] = cr.Cnt
	}
	return copyCountByKey
}

// seedListContext 行组装循环的全部上下文（阶段数据汇聚）。
type seedListContext struct {
	q                 listSeedsQuery
	publishedClusters map[string]bool
	snapshots         []model.TorrentSnapshot
	idx               *seedClusterIndex
	musicMeta         map[string]seedMusicInfo
	coverage          *seedCoverageData
	copyCountByKey    map[string]int
	trackerMatcher    *site.TrackerMatcher
}

// buildSeedListItems §59.29/§59.131/§59.143/§59.166/§59.547: 行组装 + 行级过滤
// （excludeForbidden/ready/publishState/status/search 五级·过滤即组装的一部分）。
// 返回过滤后 items 与簇音乐判定闭包（尾段 compliance 消费）。
func (h *PublishTorrentsHandler) buildSeedListItems(r *http.Request, sc *seedListContext) ([]map[string]interface{}, func(map[string]interface{}) bool) {
	q := sc.q
	clusterIsMusic := func(item map[string]interface{}) bool {
		key := fmt.Sprintf("%d|%s|%s", item["client_id"], item["save_path"], item["name"])
		for _, m := range sc.idx.metaByCluster[key] {
			if _, ok := sc.musicMeta[m.InfoHash]; ok {
				return true
			}
		}
		return false
	}
	items := make([]map[string]interface{}, 0, len(sc.snapshots))
	for _, snap := range sc.snapshots {
		item := map[string]interface{}{
			"hash":      snap.Hash,
			"name":      snap.Name,
			"size":      snap.Size,
			"client_id": snap.ClientUID,
			"save_path": snap.SavePath,
		}
		clusterKey := fmt.Sprintf("%d|%s|%s", snap.ClientUID, snap.SavePath, snap.Name)
		var meta *model.TorrentMetadata
		if metas, ok := sc.idx.metaByCluster[clusterKey]; ok && len(metas) > 0 {
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
			item["has_mediainfo"] = meta.MediaInfo != "" || meta.SourceMediaInfo != "" || meta.BDInfo != ""
			item["is_disc"] = meta.BDInfo != ""
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
			item["resolution"] = meta.Resolution
			item["video_codec"] = meta.VideoCodec
			item["audio_codec"] = meta.AudioCodec
			item["audio_channels"] = meta.AudioChannels
			item["audio_tech"] = meta.AudioTech
			item["hdr"] = meta.HDR
			item["source_type"] = meta.SourceType
			item["specification"] = meta.Specification
			encProfile := titleparser.TechProfile{
				SourceType:    meta.SourceType,
				Specification: meta.Specification,
				VideoCodec:    meta.VideoCodec,
				RawTitle:      meta.Title,
			}
			miText := meta.BDInfo
			if miText == "" {
				miText = meta.MediaInfo
			}
			if miText != "" {
				miTech := titleparser.ExtractMediaInfo(miText)
				encProfile.MIEncoded = miTech.Encoded
				encProfile.MIHasVideo = miTech.Encoded || miTech.Resolution != "" || miTech.VideoCodec != ""
				encProfile.MIWebVerdict = miTech.WEBVerdict
				encProfile.MIWebRule = miTech.WEBRule
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
		mr := seedMusicInfo{}
		if meta != nil {
			mr = sc.musicMeta[meta.InfoHash]
		}
		isMusic := mr.InfoHash != ""
		// §59.578 刀2: 烙印②（snap 循环变量直接可读——零额外查询）
		status := h.classifySeedStatusLite(r.Context(), snap.Name, meta, isMusic, snap.FetchFailedAt != nil)
		item["status"] = status
		// §59.578 刀3: 烙印原因透出（可解释性——unsupported② 颗用户须能
		// 查「为什么不支持」：tooltip 消费）
		if snap.FetchFailedAt != nil {
			item["fetch_failed_at"] = snap.FetchFailedAt
			item["fetch_error"] = snap.FetchError
		}
		// §59.577 刀2: 外站组身份并行字段（已获取态状态流后·身份从状态剥离）
		itemExternal := h.seedExternalGroupFlag(r.Context(), meta)
		item["external_group"] = itemExternal
		item["is_music"] = isMusic
		if isMusic {
			item["music_artist"] = mr.Artist
			item["music_album"] = mr.Album
		}
		cc := sc.copyCountByKey[clusterKey]
		if cc == 0 {
			cc = 1
		}
		item["copy_count"] = cc
		if sc.trackerMatcher == nil {
			sc.trackerMatcher = site.NewTrackerMatcher(h.db)
		}
		siteSet := make(map[string]bool)
		if sc.trackerMatcher != nil {
			for _, d := range sc.idx.domainByCluster[clusterKey] {
				if sn := sc.trackerMatcher.Match(d); sn != "" {
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

		if q.publishTargetSite != "" {
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
					Ours:    sc.coverage.oursByCluster[clusterKey][name],
					Seeded:  siteSet[name],
					Covered: sc.coverage.coveredByCluster[clusterKey][name],
				})
			}
			if sc.publishedClusters[clusterKey] || sc.coverage.coverageTarget[clusterKey] || siteSet[q.publishTargetSite] {
				addTag(q.publishTargetSite)
			}
			for _, s := range sites {
				addTag(s)
			}
			for s := range sc.coverage.coveredByCluster[clusterKey] {
				addTag(s)
			}
			for s := range sc.coverage.oursByCluster[clusterKey] {
				addTag(s)
			}
			item["site_tags"] = tags
		}

		// —— 行级过滤五级（§59.143/§59.131/§59.166/§59.29）——
		if q.excludeForbidden && status == "forbidden" {
			continue
		}
		if q.readyFilter != "" {
			isReady := meta != nil && meta.Reviewed
			if q.readyFilter == "true" && !isReady {
				continue
			}
			if q.readyFilter == "false" && isReady {
				continue
			}
		}
		if (q.publishState == "publishable" || q.publishState == "published") && q.publishTargetSite != "" {
			isHas := sc.publishedClusters[clusterKey] ||
				sc.coverage.coverageTarget[clusterKey] ||
				siteSet[q.publishTargetSite]
			if q.publishState == "published" && !isHas {
				continue
			}
			if q.publishState == "publishable" {
				if isHas || meta == nil || !meta.Reviewed {
					continue
				}
			}
		}
		if q.statusFilter != "" && q.statusFilter != "system_forbidden" {
			// §59.578: issues/external_group 特判删除——前端 value 改
			// forbidden/unsupported 直通 status === filter（前后端同版本
			// embed 部署·无兼容负担）。已获取外站组 tag 颗不入筛选桶
			// （状态流桶消费——外站组概念收回系统内部）。
			if status != q.statusFilter {
				continue
			}
		}
		if q.search != "" {
			sLower := strings.ToLower(q.search)
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
	return items, clusterIsMusic
}

// finalizeSeedList §59.29 尾段：system_forbidden 全量补跑（筛选该态时）+
// total+分页+当前页 compliance 精化（未筛选时）。
func (h *PublishTorrentsHandler) finalizeSeedList(r *http.Request, items []map[string]interface{}, clusterIsMusic func(map[string]interface{}) bool, q listSeedsQuery) []map[string]interface{} {
	if q.statusFilter == "system_forbidden" {
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
	start := (q.page - 1) * q.pageSize
	if start > len(items) {
		items = nil
	} else {
		end := start + q.pageSize
		if end > len(items) {
			end = len(items)
		}
		items = items[start:end]
	}
	if q.statusFilter != "system_forbidden" && h.complianceChecker != nil {
		for _, item := range items {
			name, _ := item["name"].(string)
			siteName, _ := item["site_name"].(string)
			status, _ := item["status"].(string)
			// §59.578: 跳过清单=禁转+不支持（原 no_mapping 位——
			// compliance 精化跳过未获取族·防双重覆盖）
			if status != "forbidden" && status != "unsupported" {
				if r2 := h.complianceChecker.CheckWithSite(r.Context(), name, siteName, clusterIsMusic(item)); r2 != nil && !r2.Passed {
					item["status"] = "system_forbidden"
				}
			}
		}
	}
	return items
}

// handleListSeeds §59.558 PR-2 拆分后编排（1-2 屏）：
// 解析→observing 短路→阶段装载→行组装→尾段收尾→Success。
func (h *PublishTorrentsHandler) handleListSeeds(w http.ResponseWriter, r *http.Request) {
	q := parseListSeedsQuery(r)

	// §59.38: 观察期视图独立分支短路
	if q.statusFilter == "observing" {
		h.handleListObserving(w, r, q.clientUID, q.savePath, q.search)
		return
	}

	sc := &seedListContext{q: q}
	sc.publishedClusters = h.loadPublishedClusterSet(r.Context(), q.publishTargetSite)
	sc.snapshots = h.loadSeedSnapshots(r.Context(), q)
	if len(sc.snapshots) == 0 {
		Success(w, map[string]interface{}{"items": []interface{}{}, "total": 0})
		return
	}
	metas, musicMeta := h.loadSeedMetas(r.Context(), sc.snapshots)
	sc.musicMeta = musicMeta
	sc.idx = h.buildSeedClusterIndex(r.Context(), q, metas)
	sc.coverage = h.loadSeedCoverage(r.Context(), sc.idx, q.publishTargetSite)
	sc.copyCountByKey = h.loadClusterCopyCounts(r.Context(), q)

	items, clusterIsMusic := h.buildSeedListItems(r, sc)
	total := int64(len(items))
	items = h.finalizeSeedList(r, items, clusterIsMusic, q)

	Success(w, map[string]interface{}{
		"items": items,
		"total": total,
	})
}
