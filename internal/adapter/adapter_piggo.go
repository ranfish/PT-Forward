package adapter

// PiggoAdapter §59.538/§59.540: 猪猪（piggo.me）API 适配器。
// 站方 2026-11 搬家改版 Vite SPA + 令牌 API（/v1/api·Bearer）。
//
// 契约权威源：docs/32-站点适配器设计/piggo.md（用户供文档 v3 625 行 +
// 规则三章 + 2026-11-16 九端点实测）：
//   - 统一信封 {code, data, msg, time}·code=0 成功
//   - 查询类 POST JSON 正文（无参发 {}）
//   - 公告门特殊形态：HTTP 403 但外层 code=0·data.code="notice_unread"
//     （前台读公告后恢复——透出明确文案）
//   - 条目：id/name/small_descr/cover/size/section_id/category_ids/
//     taxonomy_ids/tag_ids/media{douban,imdb,tmdb}/seeders/leechers/
//     times_completed/added_at/page_url
//   - 发布：multipart（数组=[3,5] JSON 串）·201 成功·422 errors 字典
//     （dedup「种子已存在 #tid」→ data.torrent_id）
//   - CF 按 TLS 指纹判罚：sites.tls_pretend='chrome'（azuretls 通道）
//   - HR 恒 false（用户定案：辅种不考核·RSS 用户自担·只消费不判断）
//   - 促销 API 不透出→DetectDiscount 恒 Free
//   - download-link 签发 24h URL（不含 passkey·announce 后作废）

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/description"
	"github.com/ranfish/pt-forward/internal/fingerprint"
	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type PiggoAdapter struct {
	*GenericAdapter
	doer   *HTTPDoer
	logger *zap.Logger

	// catalog 缓存：tag/taxonomy id→名称（detail.Tags 名称化——§59.539 复检
	// 通用路径消费「禁转」等名称；TTL 1h）
	catMu     sync.Mutex
	catExpiry time.Time
	tagNames  map[int]string
}

func NewPiggoAdapter(doer *HTTPDoer, logger *zap.Logger) *PiggoAdapter {
	return &PiggoAdapter{GenericAdapter: NewGenericAdapter("piggo", doer, logger), doer: doer, logger: logger}
}

func (a *PiggoAdapter) Framework() string { return "piggo" }

// ── 响应形态 ──

type piggoEnvelope struct {
	Code int             `json:"code"`
	Data json.RawMessage `json:"data"`
	Msg  string          `json:"msg"`
}

// piggoNoticeGate 公告门特殊形态（§59.538 契约：403+code=0+data.code）。
type piggoNoticeGate struct {
	Code string `json:"code"`
}

type piggoItem struct {
	ID             int64      `json:"id"`
	Name           string     `json:"name"`
	SmallDescr     string     `json:"small_descr"`
	Cover          string     `json:"cover"`
	Size           int64      `json:"size"`
	SectionID      int        `json:"section_id"`
	CategoryIDs    []int      `json:"category_ids"`
	TaxonomyIDs    []int      `json:"taxonomy_ids"`
	TagIDs         []int      `json:"tag_ids"`
	Media          piggoMedia `json:"media"`
	Seeders        int        `json:"seeders"`
	Leechers       int        `json:"leechers"`
	TimesCompleted int        `json:"times_completed"`
	AddedAt        string     `json:"added_at"`
	PageURL        string     `json:"page_url"`
}

type piggoMedia struct {
	Douban   string `json:"douban"`
	IMDb     string `json:"imdb"`
	TMDB     string `json:"tmdb"`
	TMDBKind string `json:"tmdb_kind"`
}

type piggoSearchResp struct {
	Items       []piggoItem `json:"items"`
	Page        int         `json:"page"`
	Pages       int         `json:"pages"`
	PerPage     int         `json:"per_page"`
	Total       int         `json:"total"`
	TotalCapped bool        `json:"total_capped"`
}

type piggoMeResp struct {
	User struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		Class    struct {
			Name string `json:"name"`
		} `json:"class"`
		Stats struct {
			Bonus      json.Number `json:"bonus"`
			Uploaded   string      `json:"uploaded"`
			Downloaded string      `json:"downloaded"`
		} `json:"stats"`
	} `json:"user"`
}

// ── 请求骨架 ──

func (a *PiggoAdapter) base(config *model.SiteConfig) string {
	return siteBaseURL(config)
}

// apiCall POST JSON + Bearer + 信封处理。
func (a *PiggoAdapter) apiCall(ctx context.Context, config *model.SiteConfig, path string, body any, out any) (int, []byte, error) {
	url := a.base(config) + path
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, parseError("请求序列化失败: "+err.Error(), err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, rd)
	if err != nil {
		return 0, nil, networkError("构造请求失败: "+err.Error(), err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+config.APIKey)
	}
	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return 0, nil, networkError("piggo API 请求失败: "+err.Error(), err)
	}
	defer func() { drainBody(resp) }()
	rb, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return resp.StatusCode, nil, parseError("读取响应失败", err)
	}
	return resp.StatusCode, rb, nil
}

// apiJSON apiCall + 信封解包（code=0 → data 反序列化）。
// 公告门（403+code=0+data.code=notice_unread）透出明确文案。
func (a *PiggoAdapter) apiJSON(ctx context.Context, config *model.SiteConfig, path string, body, out any) error {
	status, rb, err := a.apiCall(ctx, config, path, body, nil)
	if err != nil {
		return err
	}
	var env piggoEnvelope
	if jerr := json.Unmarshal(rb, &env); jerr == nil {
		// 公告门特殊形态：HTTP 非 200 但外层 code=0——判 data.code
		if status != http.StatusOK && env.Code == 0 {
			var ng piggoNoticeGate
			if json.Unmarshal(env.Data, &ng) == nil && ng.Code == "notice_unread" {
				return &model.AppError{Code: 40301, Message: "猪猪有未读必读公告——请到网站前台读完公告后重试"}
			}
		}
	}
	if status != http.StatusOK {
		return httpError(fmt.Sprintf("piggo API HTTP %d: %s", status, truncate(string(rb), 200)), nil)
	}
	if env.Code != 0 {
		return &model.AppError{Code: 40001, Message: fmt.Sprintf("piggo API code=%d: %s", env.Code, env.Msg)}
	}
	if out == nil || len(env.Data) == 0 || string(env.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return parseError("data 解析失败: "+err.Error(), err)
	}
	return nil
}

// ── catalog 缓存（tag 名称化）──

type piggoCatalogResp struct {
	Sections   []piggoCatSection `json:"sections"`
	Categories []piggoCatSection `json:"categories"`
	Taxonomies []piggoTaxonomy   `json:"taxonomies"`
	Tags       []piggoTag        `json:"tags"`
}

type piggoCatSection struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	SectionID int    `json:"section_id,omitempty"`
}

type piggoTaxonomy struct {
	ID    int               `json:"id"`
	Name  string            `json:"name"`
	Items []piggoCatSection `json:"items"`
}

type piggoTag struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func (a *PiggoAdapter) tagNamesOf(ctx context.Context, config *model.SiteConfig) map[int]string {
	a.catMu.Lock()
	defer a.catMu.Unlock()
	if a.tagNames != nil && time.Now().Before(a.catExpiry) {
		return a.tagNames
	}
	var cat piggoCatalogResp
	if err := a.apiJSON(ctx, config, "/v1/api/catalog", map[string]any{}, &cat); err != nil {
		a.logger.Warn("piggo catalog 拉取失败（tag 名称化降级空）", zap.Error(err))
		return nil
	}
	m := make(map[int]string, len(cat.Tags))
	for _, t := range cat.Tags {
		m[t.ID] = t.Name
	}
	a.tagNames = m
	a.catExpiry = time.Now().Add(time.Hour)
	return m
}

// ── 获取方法 ──

func (a *PiggoAdapter) SearchTorrents(ctx context.Context, config *model.SiteConfig, keyword string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
	perPage := 100
	if opts != nil && opts.MaxResults > 0 && opts.MaxResults < 100 {
		perPage = opts.MaxResults
	}
	var resp piggoSearchResp
	body := map[string]any{"q": keyword, "page": 1, "per_page": perPage}
	if err := a.apiJSON(ctx, config, "/v1/api/torrents", body, &resp); err != nil {
		return nil, err
	}
	out := make([]*model.SeedingSearchResult, 0, len(resp.Items))
	for i := range resp.Items {
		it := resp.Items[i]
		out = append(out, &model.SeedingSearchResult{
			TorrentID: strconv.FormatInt(it.ID, 10),
			Title:     it.Name,
			Size:      it.Size,
			Seeders:   it.Seeders,
			Leechers:  it.Leechers,
			DetailURL: it.PageURL,
			// SubTitle 不在 SeedingSearchResult——SmallDescr 由验证层经详情补；
			// tags 名称化在 GetTorrentDetail 供 §59.539 复检通用路径消费。
		})
	}
	return out, nil
}

func (a *PiggoAdapter) tagNamesFor(ctx context.Context, config *model.SiteConfig, ids []int) []string {
	if len(ids) == 0 {
		return nil
	}
	m := a.tagNamesOf(ctx, config)
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if n, ok := m[id]; ok {
			out = append(out, n)
		}
	}
	return out
}

func (a *PiggoAdapter) fetchItem(ctx context.Context, config *model.SiteConfig, torrentID string) (*piggoItem, error) {
	var it piggoItem
	if err := a.apiJSON(ctx, config, "/v1/api/torrents/"+torrentID, map[string]any{}, &it); err != nil {
		return nil, err
	}
	return &it, nil
}

func (a *PiggoAdapter) GetTorrentDetail(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	it, err := a.fetchItem(ctx, config, torrentID)
	if err != nil {
		return nil, err
	}
	detail := &model.TorrentDetail{
		Title:     it.Name,
		Subtitle:  it.SmallDescr,
		Size:      it.Size,
		PosterURL: it.Cover,
		IMDbID:    it.Media.IMDb,
		DoubanID:  it.Media.Douban,
		Tags:      a.tagNamesFor(ctx, config, it.TagIDs),
	}
	if it.Media.IMDb != "" {
		detail.IMDbURL = "https://www.imdb.com/title/" + it.Media.IMDb
	}
	if it.Media.Douban != "" {
		detail.DoubanURL = "https://movie.douban.com/subject/" + it.Media.Douban
	}
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", it.AddedAt, time.Local); err == nil {
		detail.UploadTime = t
		detail.PublishedAt = &t
	}
	return detail, nil
}

// DownloadTorrent POST /torrents/{id}/download-link 签发 → GET url 下载。
func (a *PiggoAdapter) DownloadTorrent(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	var dl struct {
		URL string `json:"url"`
	}
	if err := a.apiJSON(ctx, config, "/v1/api/torrents/"+torrentID+"/download-link", map[string]any{}, &dl); err != nil {
		return nil, err
	}
	if dl.URL == "" {
		return nil, parseError("download-link 签发为空", nil)
	}
	// §59.543 追记四（2026-11-20 辅种送种链 403 实证修复）：下载 GET 改
	// chrome 栈优先（a.doer.Client·azuretls——主域 piggo.me 的 CF 按 TLS
	// 指纹拦截 Go 原生栈〔matches 10605-10607 HTTP 403+CF HTML·普通栈
	// 三连 403 两连超时〕·API 域同 CF 边缘对 azuretls 放行实证）。普通栈
	// 保留 fallback（§59.541 附九旧定案的栈·当时 curl-impersonate 时代
	// chrome 挂死的结论不适用 azuretls——架构反转后未在主域实测过）。
	// 无 Bearer/chrome 请求头——签名 URL 自含鉴权（§59.1323 好大签名 URL
	// 无头 GET 先例：带头反而触发 Web 面路由 scope 校验）。
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dl.URL, nil)
	if err != nil {
		return nil, networkError("构造下载请求失败: "+err.Error(), err)
	}
	req.Header.Set("User-Agent", httpclient.BrowserUserAgent())
	resp, err := a.doer.Client.Do(req)
	if err == nil {
		defer func() { drainBody(resp) }()
		if resp.StatusCode == http.StatusOK {
			return io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		}
		rb, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		a.logger.Warn("piggo download chrome 栈非 200·回退普通栈",
			zap.Int("status", resp.StatusCode),
			zap.String("body", truncate(string(rb), 100)))
		resp.Body.Close()
	} else {
		a.logger.Warn("piggo download chrome 栈失败·回退普通栈", zap.Error(err))
	}

	// fallback：§59.541 附九普通 GET 栈（Go 原生+代理）。
	req2, err := http.NewRequestWithContext(ctx, http.MethodGet, dl.URL, nil)
	if err != nil {
		return nil, networkError("构造下载请求失败: "+err.Error(), err)
	}
	req2.Header.Set("User-Agent", httpclient.BrowserUserAgent())
	plainClient := &http.Client{Timeout: 60 * time.Second}
	if config != nil && config.ProxyURL != "" {
		plainClient.Transport = &http.Transport{Proxy: func(*http.Request) (*url.URL, error) {
			return url.Parse(config.ProxyURL)
		}}
	}
	resp, err = plainClient.Do(req2)
	if err != nil {
		return nil, networkError("下载种子失败: "+err.Error(), err)
	}
	defer func() { drainBody(resp) }()
	if resp.StatusCode != http.StatusOK {
		rb, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, httpError(fmt.Sprintf("下载种子 HTTP %d: %s", resp.StatusCode, truncate(string(rb), 150)), nil)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 16<<20))
}

// GetTorrentInfoHash API 不透出 hash——下载 .torrent 本地计算（复用
// fingerprint.ComputeFromTorrent——§59.540 复用优先铁律）。
func (a *PiggoAdapter) GetTorrentInfoHash(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	data, err := a.DownloadTorrent(ctx, config, torrentID)
	if err != nil {
		return "", err
	}
	meta, err := fingerprint.ComputeFromTorrent(data)
	if err != nil || meta == nil {
		return "", parseError("info_hash 计算失败: "+err.Error(), err)
	}
	return meta.InfoHash, nil
}

func (a *PiggoAdapter) GetPreciseSLData(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	it, err := a.fetchItem(ctx, config, torrentID)
	if err != nil {
		return nil, err
	}
	return &model.SLData{Seeders: it.Seeders, Leechers: it.Leechers}, nil
}

func (a *PiggoAdapter) GetBatchSLData(ctx context.Context, config *model.SiteConfig, torrentIDs []string) (map[string]*model.SLData, error) {
	out := make(map[string]*model.SLData, len(torrentIDs))
	for _, id := range torrentIDs {
		sl, err := a.GetPreciseSLData(ctx, config, id)
		if err != nil {
			continue // 单颗失败跳过（批量容错）
		}
		out[id] = sl
	}
	return out, nil
}

// DetectHR 用户定案：系统零处理（辅种不考核·RSS 用户自担·只消费不判断）。
func (a *PiggoAdapter) DetectHR(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
	return &model.HRResult{HasHR: false}, nil
}

// DetectDiscount API 不透出促销字段→恒 Free（下载链不设促销预期）。
func (a *PiggoAdapter) DetectDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	return &model.DiscountResult{Level: model.DiscountFree}, nil
}

// VerifyExists detail 404=不存在（无权也 404 不区分——保守按不存在）。
func (a *PiggoAdapter) VerifyExists(ctx context.Context, config *model.SiteConfig, torrentID string) (bool, error) {
	_, err := a.fetchItem(ctx, config, torrentID)
	if err != nil {
		if strings.Contains(err.Error(), "HTTP 404") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// FetchUserStats /me 基础 + /me/torrents kind=seeding 做种数（total 一发）。
func (a *PiggoAdapter) FetchUserStats(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	var me piggoMeResp
	if err := a.apiJSON(ctx, config, "/v1/api/me", map[string]any{}, &me); err != nil {
		return nil, err
	}
	bonus, _ := me.User.Stats.Bonus.Float64()
	up, _ := parseHumanSize(me.User.Stats.Uploaded)
	down, _ := parseHumanSize(me.User.Stats.Downloaded)
	out := &model.UserStatsResult{
		Username:      me.User.Username,
		UserClass:     me.User.Class.Name,
		BonusPoints:   bonus,
		UploadBytes:   int64(up),
		DownloadBytes: int64(down),
	}
	// 做种数（kind=seeding total——首版口径：做种体积分页成本高挂起 §59.540）
	var seed piggoSearchResp
	if err := a.apiJSON(ctx, config, "/v1/api/me/torrents", map[string]any{"kind": "seeding", "page": 1, "per_page": 1}, &seed); err == nil {
		out.SeedingCount = seed.Total
	}
	return out, nil
}

// parseHumanSize "1.00 GB"/"11.50 TB" → 字节。
func parseHumanSize(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	units := []struct {
		suf string
		mul uint64
	}{
		{"TB", 1 << 40}, {"GB", 1 << 30}, {"MB", 1 << 20}, {"KB", 1 << 10}, {"B", 1},
	}
	for _, u := range units {
		if strings.HasSuffix(s, u.suf) {
			num := strings.TrimSpace(strings.TrimSuffix(s, u.suf))
			f, err := strconv.ParseFloat(num, 64)
			if err != nil {
				return 0, err
			}
			return uint64(f * float64(u.mul)), nil
		}
	}
	return strconv.ParseUint(s, 10, 64)
}

// SupportsSearchByPiecesHash piggo L0 通道可用（§59.538 pieces-hash 实测命中）。
// §59.543: SearchByPiecesHash 方法已实现（此前声明 true 却缺方法=断言失败
// 被特征批量通道排除——二师兄辅种零结果根因之一）。
func (a *PiggoAdapter) SupportsSearchByPiecesHash() bool { return true }

// SearchByPiecesHash §59.543: 特征辅种主通道——POST /torrents/pieces-hash
// 传真 pieces-hash（真指纹·piggo.md :514 实证精确命中）·返回
// map[piecesHash]tid。与 NP 站同管线（引擎 piecesHashSearcher 接口·
// 批 ≤100 与站方限流 30/min 匹配——825 源=9 批/轮远低于限流）。
func (a *PiggoAdapter) SearchByPiecesHash(ctx context.Context, config *model.SiteConfig, piecesHashes []string) (map[string]int, error) {
	out := make(map[string]int)
	if len(piecesHashes) == 0 {
		return out, nil
	}
	const max = 100
	for start := 0; start < len(piecesHashes); start += max {
		end := start + max
		if end > len(piecesHashes) {
			end = len(piecesHashes)
		}
		var resp struct {
			Items []struct {
				Hash string `json:"hash"`
				ID   int64  `json:"id"`
			} `json:"items"`
		}
		if err := a.apiJSON(ctx, config, "/v1/api/torrents/pieces-hash", map[string]any{"hashes": piecesHashes[start:end]}, &resp); err != nil {
			return out, err
		}
		for _, it := range resp.Items {
			out[it.Hash] = int(it.ID)
		}
	}
	return out, nil
}

// §59.543 项 2（用户定案·2026-11-20）：BatchMatchHashes 已删除——
// piggo 站方唯一的 hash 查询端点 /torrents/pieces-hash 收【真 pieces-hash
// （内容指纹）】（piggo.md :514 实证·当时验证即拿真指纹命中）·无 info-hash
// 反查端点——L0（info_hash 数学身份）通道对 piggo 物理不适用。此前该方法
// 把真指纹端点包装成 L0 接口：调用方传 info_hash 进收内容指纹的窗口→
// 必然零命中白跑（每轮 9 批·占 30/min 配额·243 实证 825 查询 0 命中）。
// 端点现由 SearchByPiecesHash（特征通道·传真指纹）独占正确使用。
// 引擎侧 model.HashBatchMatcher 断言不过→L0 预查对 piggo 静默跳过（零请求）。

// ── 发布 ──

// UploadTorrent multipart publish。
// piggoTaxonomyFallback §59.541: taxonomy 八维空位兜底（「八维全填」定案——
// 素材 miss 时填 Other/SDR，站方 MI 纠正盖住手填·Ronin 八维全填站方实态）。
var piggoTaxonomyFallback = map[string]string{
	"source_id":     "36", // Other（来源）
	"medium_id":     "31", // Other（媒介）
	"codec_id":      "26", // Other（编码）
	"audio_id":      "19", // Other（音频）
	"resolution_id": "10", // other（分辨率）
	"processing_id": "6",  // SDR（处理）
	"team_id":       "5",  // Other（制作组）
	"region_id":     "53", // 其他（地区）
}

// piggoTaxonomyIDs §59.541: 八维虚拟字段（form_config 值映射产物）合并为
// taxonomy_ids 扁平 JSON 数组（站方契约——id 段区分维度归属·每维一项）。
// ExtraFields 优先于 FormFields（与 get() 同序）。
func piggoTaxonomyIDs(req *model.PublishRequest) string {
	order := []string{
		"source_id", "medium_id", "codec_id", "audio_id",
		"resolution_id", "processing_id", "team_id", "region_id",
	}
	seen := map[string]bool{}
	var ids []string
	for _, k := range order {
		v := ""
		if vv, ok := req.ExtraFields[k]; ok && vv != "" {
			v = vv
		} else if vv, ok := req.FormFields[k]; ok {
			v = vv
		}
		if v == "" {
			v = piggoTaxonomyFallback[k]
		}
		if v == "" || seen[v] {
			continue // 维度 id 空间互斥——去重纯防御
		}
		seen[v] = true
		ids = append(ids, v)
	}
	if len(ids) == 0 {
		return ""
	}
	// 站方契约数字数组（detail 实测 [35,28,...] 无引号）——手工拼形
	var b strings.Builder
	b.WriteByte('[')
	for i, id := range ids {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(id)
	}
	b.WriteByte(']')
	return b.String()
}

// 契约：file/name/small_descr/descr/media_info/section_id/category_ids 必填；
// 数组字段 JSON 串形态（category_ids=[3,5]）；成功 201；422 errors 字典
// 透出（dedup「种子已存在 #tid」→ data.torrent_id → duplicate 语义）。
func (a *PiggoAdapter) UploadTorrent(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	form := req.FormFields
	get := func(k string) string {
		if v, ok := req.ExtraFields[k]; ok && v != "" {
			return v
		}
		return form[k]
	}
	name := req.Title
	if v := get("name"); v != "" {
		name = v
	}
	smallDescr := req.Subtitle
	if v := get("small_descr"); v != "" {
		smallDescr = v
	}
	sectionID := get("section_id")
	if sectionID == "" {
		return nil, &model.AppError{Code: 40001, Message: "piggo 发布缺 section_id（分区映射未配置——form_config 检查）"}
	}
	categoryIDs := get("category_ids")
	if categoryIDs == "" {
		return nil, &model.AppError{Code: 40001, Message: "piggo 发布缺 category_ids（分类映射未配置）"}
	}
	// §59.540: Markdown 简介（禁 BBCode）—— descr 必填
	descr := req.Description
	mediaInfo := req.MediaInfo

	var buf bytes.Buffer
	w := newPiggoMultipartWriter(&buf)
	w.addField("name", name)
	w.addField("small_descr", smallDescr)
	// §59.541: Markdown 翻译层（§59.538 定案 MD only 禁 BBCode——
	// BBCodeToMarkdown 复用·截图内嵌保留 [img]→![]()）
	w.addField("descr", description.BBCodeToMarkdown(descr))
	w.addField("media_info", mediaInfo)
	w.addField("section_id", sectionID)
	w.addField("category_ids", categoryIDs)
	// §59.541: taxonomy 八维合并——8 个虚拟字段（form_config 值映射各维 id）
	// 合并扁平 taxonomy_ids JSON 数组；空维兜 Other/SDR（「八维全填」定案——
	// Ronin 站方实态·站方 MI 纠正盖住手填）
	if tx := piggoTaxonomyIDs(req); tx != "" {
		w.addField("taxonomy_ids", tx)
	}
	// §59.541: tag_ids 聚合——TagArrayFields 同名 KV（executor 通用链真相源·
	// TNode 聚合先例）→ JSON 数组串；兜底 FormFields 单值。
	// §59.541 回归审核 P1: taglist applier 硬编码 Key="tagList"（系统约定·
	// TNode/好学/朱雀同款 §59.1331/§59.1481）——聚合条件双 key 收（站方
	// 字段名 tag_ids 由本层翻译）。
	// §59.540 附六·二轮回归 P2: 非数值项蒸发——applier 无映射标准键直出
	// （write("tagList", tag) else 分支）会混入数字数组致 422；好学先例
	// ParseInt 过滤（§59.1481 蒸发路径——第三站接入漏继承）。
	tagVals := make([]string, 0, 4)
	for _, kv := range req.TagArrayFields {
		if (kv.Key == "tag_ids" || kv.Key == "tagList") && kv.Value != "" {
			if n, perr := strconv.ParseInt(strings.TrimSpace(kv.Value), 10, 64); perr == nil && n > 0 {
				tagVals = append(tagVals, strconv.FormatInt(n, 10))
			}
		}
	}
	if len(tagVals) > 0 {
		// 站方契约数字数组（与 piggoTaxonomyIDs 同款——json.Marshal 字符串
		// 数组首版错被锚抓出·§59.1328 修一处 grep 另一处纪律）
		var tb strings.Builder
		tb.WriteByte('[')
		for i, v := range tagVals {
			if i > 0 {
				tb.WriteByte(',')
			}
			tb.WriteString(v)
		}
		tb.WriteByte(']')
		w.addField("tag_ids", tb.String())
	} else if v := get("tag_ids"); v != "" {
		w.addField("tag_ids", v)
	}
	if req.DoubanLink != "" {
		w.addField("douban", req.DoubanLink)
	}
	if req.IMDbLink != "" {
		w.addField("imdb", req.IMDbLink)
	}
	anon := "0"
	if req.Anonymous {
		anon = "1"
	}
	w.addField("anonymous", anon)
	// §59.541 附八: screenshots[] 独立截图字段（URL 列表——站方页面展示
	// 走独立输入行·descr 内嵌不渲染）。executor 经 __piggo_screenshots
	// 传递（newline 分隔 URL 列表）。
	if ss := get("__piggo_screenshots"); ss != "" {
		for _, line := range strings.Split(ss, "\n") {
			if u := strings.TrimSpace(line); u != "" {
				w.addField("screenshots[]", u)
			}
		}
	}
	w.addFile("file", "a.torrent", req.TorrentData)
	w.close()

	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.base(config)+"/v1/api/torrents/publish", &buf)
	if err != nil {
		return nil, networkError("构造发布请求失败: "+err.Error(), err)
	}
	hreq.Header.Set("Content-Type", w.contentType())
	hreq.Header.Set("Accept", "application/json")
	if config.APIKey != "" {
		hreq.Header.Set("Authorization", "Bearer "+config.APIKey)
	}
	resp, err := a.doer.Client.Do(hreq)
	if err != nil {
		return nil, networkError("piggo 发布请求失败: "+err.Error(), err)
	}
	defer func() { drainBody(resp) }()
	rb, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))

	var env piggoEnvelope
	var perr struct {
		Message   string              `json:"message"`
		Errors    map[string][]string `json:"errors"`
		TorrentID int64               `json:"torrent_id"`
	}
	_ = json.Unmarshal(rb, &env)
	_ = json.Unmarshal(env.Data, &perr)

	switch {
	case resp.StatusCode == http.StatusCreated:
		var it piggoItem
		if err := json.Unmarshal(env.Data, &it); err == nil && it.ID > 0 {
			return &model.PublishResponse{
				Success: true, TorrentID: strconv.FormatInt(it.ID, 10),
				DetailURL: it.PageURL, IDSource: "api_201",
				// §59.541 附七: 不回填 TorrentData——piggo 上传后会重新生成
				// .torrent（加 source 字段·hash 变化），本地数据≠站方终版。
				// 回填会触发 pusher §59.515 D2 直推通道跳过站方下载→TR 同
				// hash 去重→站方 tracker 永远挂不上（首战五连发实证）。
			}, nil
		}
		return nil, parseError("piggo 201 响应解析失败: "+truncate(string(rb), 200), nil)
	case resp.StatusCode == http.StatusUnprocessableEntity:
		// dedup 形态：errors.file 含「种子已存在 #tid」→ duplicate
		for _, es := range perr.Errors {
			for _, e := range es {
				if strings.Contains(e, "已存在") {
					return &model.PublishResponse{
						IsExisting: true, ExistingID: strconv.FormatInt(perr.TorrentID, 10),
						ErrorMessage: e, TorrentID: strconv.FormatInt(perr.TorrentID, 10),
					}, nil
				}
			}
		}
		detail := perr.Message
		if detail == "" {
			for k, es := range perr.Errors {
				for _, e := range es {
					detail += k + ": " + e + "; "
				}
			}
		}
		return nil, &model.AppError{Code: 40002, Message: "piggo 发布 422: " + truncate(detail, 300)}
	case resp.StatusCode == http.StatusForbidden && env.Code == 0:
		return nil, &model.AppError{Code: 40301, Message: "猪猪有未读必读公告——请到网站前台读完公告后重试"}
	default:
		return nil, httpError(fmt.Sprintf("piggo 发布 HTTP %d: %s", resp.StatusCode, truncate(string(rb), 300)), nil)
	}
}

// piggoMultipartWriter 轻量 multipart（数组字段=[3,5] JSON 串形态——
// 非同名字段重复·§59.538 cURL --form 权威形态）。
type piggoMultipartWriter struct {
	buf   *bytes.Buffer
	bound string
}

func newPiggoMultipartWriter(buf *bytes.Buffer) *piggoMultipartWriter {
	return &piggoMultipartWriter{buf: buf, bound: "piggo" + strconv.FormatInt(time.Now().UnixNano(), 36)}
}

func (w *piggoMultipartWriter) contentType() string {
	return "multipart/form-data; boundary=" + w.bound
}

func (w *piggoMultipartWriter) addField(name, value string) {
	fmt.Fprintf(w.buf, "--%s\r\nContent-Disposition: form-data; name=\"%s\"\r\n\r\n%s\r\n", w.bound, name, value)
}

func (w *piggoMultipartWriter) addFile(field, filename string, data []byte) {
	fmt.Fprintf(w.buf, "--%s\r\nContent-Disposition: form-data; name=\"%s\"; filename=\"%s\"\r\nContent-Type: application/x-bittorrent\r\n\r\n", w.bound, field, filename)
	w.buf.Write(data)
	w.buf.WriteString("\r\n")
}

func (w *piggoMultipartWriter) close() {
	fmt.Fprintf(w.buf, "--%s--\r\n", w.bound)
}
