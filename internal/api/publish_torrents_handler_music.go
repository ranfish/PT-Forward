package api

import (
	"context"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/ranfish/pt-forward/internal/comment"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"github.com/ranfish/pt-forward/internal/reseed"
	"github.com/ranfish/pt-forward/internal/site"
	"github.com/ranfish/pt-forward/internal/titleparser"
	"go.uber.org/zap"
)

// musicTitleMatch §59.476 S2+ 标题官组词判定（纯函数·DB 无关可测）。
//
// 匹配序（用户定案"见到即判"+廉价否决）：
//  1. 组位恒过（权威形态·括号外）：@锚（最后 @ 后首词）∨ 尾段（最后 -
//     段首词·容忍 spec 后缀 "OpenCD 16bit 44.1kHz" 与装饰 "-Rosebudx@OpenCD🌱"）
//  2. 非组位 word-boundary 命中 ∧ 无影视技术词（ParseTitleTech
//     Resolution ∨ VideoCodec——仅视频专属词·Medium 排除：SACD/CD 是
//     音乐媒介）→ 音乐
//  3. 非组位命中 ∧ 影视技术词在场 → 否决（AI 纪录片类内容词 LLM 假阳性
//     拦截——PT 影视标题必带分辨率/编码·音乐标题必不带·两域无交集）
func musicTitleMatch(title string, words []string) bool {
	if title == "" || len(words) == 0 {
		return false
	}
	bare := stripBracketSegments(title)
	tp := titleparser.ParseTitleTech(title)
	veto := tp.Resolution != "" || tp.VideoCodec != ""
	for _, w := range words {
		if w == "" {
			continue
		}
		// ① @锚（括号外）
		if i := strings.LastIndex(bare, "@"); i >= 0 && i+1 < len(bare) {
			if firstWordToken(bare[i+1:]) == w {
				return true
			}
		}
		// ② 尾段（括号外·首词容忍 spec 后缀）
		if i := strings.LastIndex(bare, "-"); i >= 0 && i+1 < len(bare) {
			if firstWordToken(bare[i+1:]) == w {
				return true
			}
		}
		// ③ 非组位见到即判（否决闸：影视技术词在场 → 本词否决·续查下词）
		if veto {
			continue
		}
		if wordBoundaryHit(bare, w) {
			return true
		}
	}
	return false
}

// wordBoundaryHit §59.476: word-boundary 精确词命中（大小写敏感·
// 防 OpenCDX 类子串误命中——X 为词字符不裂词）。
func wordBoundaryHit(s, word string) bool {
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(word) + `\b`).MatchString(s)
}

// firstWordToken §59.476: 段首词提取——剥前导空白/连接符后取首个
// [A-Za-z0-9]+ 词（emoji/中文不在词字符类·"OpenCD🌱"→"OpenCD"）。
func firstWordToken(s string) string {
	s = strings.TrimLeft(s, " \t‑–—―﹣")
	if m := regexp.MustCompile(`^[A-Za-z0-9]+`).FindString(s); m != "" {
		return m
	}
	return ""
}

// stripBracketSegments §59.476: 剥全部括号段（组名"必定在括号外"——
// 用户权威定义）。栈式扫描：任一开括号入栈·配对闭括号出栈·栈空保留。
// 未配对开括号后内容全部丢弃（防御·罕见）。
func stripBracketSegments(s string) string {
	var b strings.Builder
	var stack []rune
	pairs := map[rune]rune{']': '[', '}': '{', ')': '(', '）': '（', '】': '【'}
	for _, r := range s {
		switch r {
		case '[', '{', '(', '（', '【':
			stack = append(stack, r)
		default:
			if closer, ok := pairs[r]; ok {
				if len(stack) > 0 && stack[len(stack)-1] == closer {
					stack = stack[:len(stack)-1]
				}
				continue // 闭括号（配对弹出/孤立）均不入输出
			}
			if len(stack) == 0 {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// filterToMusicSources coverage 过滤为音乐源站子集（保留全状态行——
// SelectFetchSite 内部按状态筛；S1 判定含 tracker-only 行）。
func filterToMusicSources(sites []model.SiteCoverageCache) []model.SiteCoverageCache {
	out := make([]model.SiteCoverageCache, 0, len(sites))
	for _, c := range sites {
		if site.IsMusicSourceSite(c.SiteName) {
			out = append(out, c)
		}
	}
	return out
}

// filterClusterToMusicSources 簇直达候选过滤为音乐源站子集
// （簇概念音乐链保留——限定音乐源站内直达）。
func filterClusterToMusicSources(targets []comment.DirectTarget) []comment.DirectTarget {
	out := make([]comment.DirectTarget, 0, len(targets))
	for _, t := range targets {
		if site.IsMusicSourceSite(t.SiteName) {
			out = append(out, t)
		}
	}
	return out
}

// musicFallbackSites §59.476 音乐链兜底站序：fetch_priority∩音乐源站
// （配置内按序）优先 → 其余按素材量序（皇后→海豚→乌托邦）。仅返回
// enabled 且有凭证（cookie/api_key）的站——与 SelectFetchSite 各层同门槛。
func (h *PublishTorrentsHandler) musicFallbackSites(ctx context.Context) []string {
	if h == nil || h.db == nil {
		return nil
	}
	var enabled []model.Site
	h.db.WithContext(ctx).
		Where("name IN ? AND enabled = ? AND (cookie != '' OR api_key != '')", site.MusicSourceSites(), true).
		Find(&enabled)
	if len(enabled) == 0 {
		return nil
	}
	enabledSet := map[string]bool{}
	for _, s := range enabled {
		enabledSet[s.Name] = true
	}
	var out []string
	for _, p := range publish.FetchPriorityList(ctx, h.db) {
		if enabledSet[p] && site.IsMusicSourceSite(p) && !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	for _, m := range site.MusicSourceSites() {
		if enabledSet[m] && !slices.Contains(out, m) {
			out = append(out, m)
		}
	}
	return out
}

// resolveMusicReplicaHash §59.476 位置3: 簇内音乐源站副本优先——批量/
// 单种取数入口把代表 hash 换成音乐源站 tracker 副本（素材量序皇后→
// 海豚→乌托邦）→ S1 天然命中·路由逻辑零改动（fetchSingleTorrent 不变）。
// 影视簇/无音乐源站副本/快照缺失 → 原样返回（零变化）。size 一并对齐
// （同簇同内容·verify 层微差容忍 §59.474；副本快照 size 缺失回退原值）。
//
// 案（12 环境实证）：Gwyneth 簇代表 b5967ea3（pterclub 辅种）→ 换
// a901d2bf（tracker.open.cd）→ 单种路径两次全对的音乐链成为批量默认。
func (h *PublishTorrentsHandler) resolveMusicReplicaHash(ctx context.Context, clientUID uint, hash string, size int64) (string, int64) {
	if h == nil || h.db == nil || clientUID == 0 || hash == "" {
		return hash, size
	}
	var ck struct{ SavePath, Name string }
	if err := h.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
		Select("save_path, name").
		Where("hash = ? AND client_uid = ? AND is_hidden = 0 AND name != ''", hash, clientUID).
		Row().Scan(&ck.SavePath, &ck.Name); err != nil || ck.Name == "" {
		return hash, size
	}
	type repRow struct {
		Hash          string
		Size          int64
		TrackerDomain string
	}
	var reps []repRow
	h.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
		Select("hash, size, tracker_domain").
		Where("client_uid = ? AND save_path = ? AND name = ? AND is_hidden = 0 AND tracker_domain != ''",
			clientUID, ck.SavePath, ck.Name).
		Scan(&reps)
	if len(reps) == 0 {
		return hash, size
	}
	// 音乐源站 tracker 匹配器（复用 TrackerMatcher 域名语义·含 alt 域）
	var musicSites []model.Site
	h.db.WithContext(ctx).Where("name IN ?", site.MusicSourceSites()).Find(&musicSites)
	if len(musicSites) == 0 {
		return hash, size
	}
	matcher := site.NewTrackerMatcherFromSites(musicSites)
	rank := map[string]int{}
	for i, s := range site.MusicSourceSites() {
		rank[s] = i
	}
	bestHash, bestSize, bestRank := "", size, 1<<30
	for _, r := range reps {
		hit := matcher.Match("http://" + r.TrackerDomain + "/")
		if hit == "" {
			continue
		}
		if rk, ok := rank[hit]; ok && rk < bestRank {
			bestRank, bestHash, bestSize = rk, r.Hash, r.Size
		}
	}
	if bestHash == "" || bestHash == hash {
		return hash, size
	}
	if bestSize <= 0 {
		bestSize = size
	}
	h.logger.Info("music replica hash swapped",
		zap.String("from", hash[:min(10, len(hash))]),
		zap.String("to", bestHash[:min(10, len(bestHash))]))
	return bestHash, bestSize
}

// siblingTitleCandidates §59.482 刀7/§59.483 双通道: 簇全域兄弟候选——
// 【不加音乐源站过滤】（与 §59.478 缓存路径的本质差异：桥只取兄弟站的
// 标题词·数据仍从音乐源站取——龙PT 给词·皇后给数据）。
// 通道① coverage tid 行（需 coverage 已丰富·按需触发非自动）
// 通道② 簇 comment 解析（零网络成本·快照同步即生效——School Rumble
//
//	转发案：新副本 coverage tid 未丰富但 comment 立即可用）
//
// 合并去重·限 4 候选（成本上界：≤4 兄弟详情+≤4 音乐源站搜索）。
// 输出轻量 struct（统一两通道类型——桥循环只消费 SiteName/TorrentID）。
type siblingCandidate struct {
	SiteName  string
	TorrentID string
}

func (h *PublishTorrentsHandler) siblingTitleCandidates(ctx context.Context, clientUID uint, savePath, name, selfHash string, excludeSites []string) []siblingCandidate {
	if h.db == nil || clientUID == 0 || savePath == "" || name == "" {
		return nil
	}
	seen := make(map[string]bool) // "site:tid" 去重
	excl := make(map[string]bool, len(excludeSites))
	for _, s := range excludeSites {
		excl[s] = true
	}
	var out []siblingCandidate

	// 通道② 簇 comment 解析（优先——零成本·新副本即生效）
	// §59.483: buildClusterTargets 自建（外层 clusterTargets 已被音乐
	// 过滤重赋值·原始未过滤版本不可达——桥内自建保持自洽）
	for _, tgt := range h.buildClusterTargets(ctx, clientUID, savePath, name, selfHash) {
		if tgt.TorrentID == "" || tgt.SiteName == "" || excl[tgt.SiteName] {
			continue
		}
		key := tgt.SiteName + ":" + tgt.TorrentID
		if !seen[key] {
			seen[key] = true
			out = append(out, siblingCandidate{SiteName: tgt.SiteName, TorrentID: tgt.TorrentID})
		}
	}

	// 通道① coverage tid 行
	var hashes []string
	h.db.WithContext(ctx).Model(&model.TorrentSnapshot{}).
		Where("client_uid = ? AND save_path = ? AND name = ? AND is_hidden = 0", clientUID, savePath, name).
		Pluck("hash", &hashes)
	if len(hashes) > 0 {
		q := h.db.WithContext(ctx).
			Where("info_hash IN ? AND status = ? AND torrent_id != ''",
				hashes, model.CoverageConfirmedHas)
		if len(excludeSites) > 0 {
			q = q.Where("site_name NOT IN ?", excludeSites)
		}
		var covs []model.SiteCoverageCache
		q.Order("confidence DESC").Limit(2).Find(&covs)
		for _, c := range covs {
			key := c.SiteName + ":" + c.TorrentID
			if !seen[key] {
				seen[key] = true
				out = append(out, siblingCandidate{SiteName: c.SiteName, TorrentID: c.TorrentID})
			}
		}
	}

	// 限 4（通道② ≤2 comment + 通道① ≤2 coverage）
	if len(out) > 4 {
		out = out[:4]
	}
	return out
}

// siblingKeywordUsable §59.482 刀7: 兄弟标题→音乐关键词+同词守卫——
// 与主搜已试关键词相同则跳过（同词再搜=白烧请求）。
func siblingKeywordUsable(originalName, siblingTitle string) (string, bool) {
	if siblingTitle == "" || siblingTitle == originalName {
		return "", false
	}
	kw := reseed.ExtractMusicSearchKeyword(siblingTitle, false)
	if kw == "" {
		return "", false
	}
	tried := reseed.ExtractMusicSearchKeyword(originalName, false)
	if kw == tried {
		return "", false
	}
	return kw, true
}

// musicSiblingTitleBridge §59.482 刀7: 簇兄弟标题桥——音乐链搜索 miss
// + IYUU 缓存空后的第三兜底（§59.480 实时段回退后的异形名救援通道）：
// 兄弟站仅取标题（GetTorrentDetail 直调零落库——红豆饭数据污染防御）
// → 音乐关键词管道 → 音乐源站搜索（全验证链把关）→ musicY 落库 +
// coverage 回写（未来缓存直达）。KC 50th 案实证：龙PT tid=416 标题
// "Various Artists - Heaven & Earth 2019" → 皇后 tid=165449。
func (h *PublishTorrentsHandler) musicSiblingTitleBridge(ctx context.Context, clientUID uint, savePath, name, hash string, size int64, musicSite string, excludeSites []string) *model.TorrentMetadata {
	if h.siteProvider == nil || h.metadataFetcher == nil || musicSite == "" {
		return nil
	}
	for _, cand := range h.siblingTitleCandidates(ctx, clientUID, savePath, name, hash, excludeSites) {
		config, err := h.siteProvider.GetSiteConfig(ctx, cand.SiteName)
		if err != nil || config == nil {
			continue
		}
		adapter, err := h.siteProvider.GetAdapter(ctx, cand.SiteName)
		if err != nil || adapter == nil {
			continue
		}
		detail, err := adapter.GetTorrentDetail(ctx, config, cand.TorrentID)
		if err != nil || detail == nil || detail.Title == "" {
			continue
		}
		kw, ok := siblingKeywordUsable(name, detail.Title)
		if !ok {
			continue
		}
		h.logger.Info("music sibling title bridge: searching with sibling title",
			zap.String("hash", hash[:min(10, len(hash))]),
			zap.String("sibling_site", cand.SiteName),
			zap.String("keyword", kw))
		meta, err := h.metadataFetcher.FetchAndStoreBySearch(ctx, hash, musicSite, detail.Title, size, "", "", "music")
		if err == nil && meta != nil {
			// coverage 回写：未来重获走缓存直达（§59.482 闭环 3）
			h.metadataFetcher.PersistCoverageHit(ctx, hash, musicSite, meta.TorrentID)
			return meta
		}
	}
	return nil
}

// handleCoveredSites §59.535 刀5: 簇已有站三面并集（GET /publish/seeds/covered-sites?hash=）
// ——选站发布弹窗置灰判据（单簇过滤·宁可多置灰：站方 dedup 兜底·不漏放）。
func (h *PublishTorrentsHandler) handleCoveredSites(w http.ResponseWriter, r *http.Request) {
	hash := strings.TrimSpace(r.URL.Query().Get("hash"))
	if hash == "" {
		Error(w, http.StatusBadRequest, 40001, "缺少 hash 参数")
		return
	}
	tm := site.NewTrackerMatcher(h.db)
	covered := publish.CoveredSitesOfCluster(r.Context(), h.db, tm, hash)
	names := make([]string, 0, len(covered))
	for name := range covered {
		names = append(names, name)
	}
	sort.Strings(names)
	Success(w, map[string]interface{}{"hash": hash, "sites": names})
}

// isMusicSeedForRouting §59.469+§59.476 音乐种分流判定：
// S1 = coverage 含音乐源站（纯音乐站无影视内容——逻辑必然）
// S2+ = 标题含音乐官组词（词表=release_group_mappings 音乐源站行推导·
//
//	§59.476 扩表 {OpenCD,LLM,TSxD,KHQ,Musopia}·见到即判·否决闸见
//	musicTitleMatch）。
//
// 宁漏勿误：漏判走影视链（music 空可接受）；误判影视种锁进音乐链不可
// 接受（误判成本=音乐链搜索无果·干净失败可重试）。
func (h *PublishTorrentsHandler) isMusicSeedForRouting(ctx context.Context, title string, coverageSites []model.SiteCoverageCache) bool {
	for _, c := range coverageSites {
		if site.IsMusicSourceSite(c.SiteName) {
			return true
		}
	}
	if h == nil || h.db == nil {
		return false
	}
	return musicTitleMatch(title, publish.MusicOfficialGroupWords(ctx, h.db))
}
