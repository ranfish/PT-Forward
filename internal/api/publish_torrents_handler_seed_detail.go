package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/ranfish/pt-forward/internal/description"
	"github.com/ranfish/pt-forward/internal/fingerprint"
	"github.com/ranfish/pt-forward/internal/metadata"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/site"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"github.com/ranfish/pt-forward/internal/util"
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
