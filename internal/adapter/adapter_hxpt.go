package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

// HxptAdapter 好学（framework=hxpt·§59.533 刀2·2026-11-15）。
//
// 站方 2026-11 改版彻底离开 NexusPHP（Next.js SPA + /api/v1 REST）——
// 旧 NP 适配（§59.453 槽位错位特化）全部作废。实测契约（活体探针
// 2026-11-15·cookie 同域直用）：
//   - 统一信封 {code, message, data}；code===0 成功
//   - 列表 GET /api/v1/torrents?search=&limit=（cursor 分页 next_cursor；
//     唯一搜索参数 search——keyword/q/name 阴性对照实证被忽略）
//   - 元数据 GET /api/v1/torrents/{id}（category_id/info_hash/pieces_hash/
//     hr_policy{days,enabled,seed_hours}/promotion/created_at）
//   - 内容 GET /api/v1/torrents/{id}/detail（descr BBCode+mediainfo 独立
//     字段+sections 实例+numfiles）
//   - 下载 GET /api/v1/torrents/{id}/download（真 bencode·tracker.hxpt）
//   - 用户 GET /api/v1/me（uploaded/downloaded/seeding/class_name/uploads）
//   - 详情页路由 /torrent/{id}；RSS 无（404）
//   - CSRF double-submit（cookie flux_csrf ↔ header x-csrf-token）——
//     POST/PUT 必带·GET 免（刀3 发布链接入）
type HxptAdapter struct {
	*GenericAdapter
	doer   *HTTPDoer
	logger *zap.Logger
}

func NewHxptAdapter(doer *HTTPDoer, logger *zap.Logger) *HxptAdapter {
	return &HxptAdapter{GenericAdapter: NewGenericAdapter("hxpt", doer, logger), doer: doer, logger: logger}
}

func (a *HxptAdapter) Framework() string { return "hxpt" }

// ── 响应形态 ──

type hxptEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// hxptItem 列表条目（元数据端点同构——两处字段并集在此声明）。
type hxptItem struct {
	ID              int64           `json:"id"`
	Name            string          `json:"name"`
	SmallDescr      string          `json:"small_descr"`
	CategoryID      int             `json:"category_id"`
	Size            int64           `json:"size"`
	Seeders         int             `json:"seeders"`
	Leechers        int             `json:"leechers"`
	TimesCompleted  int             `json:"times_completed"`
	CreatedAt       string          `json:"created_at"`
	InfoHash        string          `json:"info_hash"`
	PiecesHash      string          `json:"pieces_hash"`
	Poster          string          `json:"poster"`
	IMDbID          string          `json:"imdb_id"`
	Promotion       json.RawMessage `json:"promotion"`        // null=无促销；非 null 形态未实证（宽容透传）
	PromotionEndsAt *time.Time      `json:"promotion_ends_at"` // RFC3339
	HRPolicy        *hxptHRPolicy   `json:"hr_policy"`
	OwnerName       string          `json:"owner_name"`
	Anonymous       bool            `json:"anonymous"`
	Official        bool            `json:"official"`
	Tags            json.RawMessage `json:"tags"`
	SecNames        []string        `json:"sec_names"`
}

type hxptHRPolicy struct {
	Days      int  `json:"days"`
	Enabled   bool `json:"enabled"`
	SeedHours int  `json:"seed_hours"`
}

// hxptCategoryNames 分类 id→名（site-profile.categories 实测 2026-11-15；
// 站方分类树稳定时同步——新分类 miss 走空串不阻塞）。
var hxptCategoryNames = map[int]string{
	1: "电影", 2: "电视剧", 3: "短剧", 4: "音乐", 5: "游戏", 6: "软件",
	7: "动漫", 8: "纪录片", 9: "电子书", 10: "有声书", 11: "体育",
	12: "综艺", 13: "教育资源",
}

// ── 内部请求骨架 ──

func (a *HxptAdapter) base(config *model.SiteConfig) string {
	return siteBaseURL(config)
}

// apiCall 统一请求：cookie 同域直用 + JSON 信封解包（GET 免 CSRF）。
func (a *HxptAdapter) apiCall(ctx context.Context, config *model.SiteConfig, method, url string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return networkError("构造请求失败: "+err.Error(), err)
	}
	if config.Cookie != "" {
		req.Header.Set("Cookie", config.Cookie)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return networkError("API 请求失败: "+err.Error(), err)
	}
	defer func() { drainBody(resp) }()
	rb, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return parseError("读取响应失败", err)
	}
	if resp.StatusCode != http.StatusOK {
		return httpError(fmt.Sprintf("好学 API HTTP %d: %s", resp.StatusCode, truncate(string(rb), 200)), nil)
	}
	if out == nil {
		return nil
	}
	var env hxptEnvelope
	if err := json.Unmarshal(rb, &env); err != nil {
		return parseError("响应 JSON 解析失败: "+err.Error(), err)
	}
	if env.Code != 0 {
		return &model.AppError{Code: 40001, Message: fmt.Sprintf("好学 API code=%d: %s", env.Code, env.Message)}
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return parseError("data 解析失败: "+err.Error(), err)
	}
	return nil
}

// hxptCSRFToken 从 cookie 提取 flux_csrf 值（静态 fallback——好学主通道
// 是 freshCSRF 动态获取·见 UploadTorrent 注释）。
func hxptCSRFToken(config *model.SiteConfig) string {
	for _, part := range strings.Split(config.Cookie, ";") {
		kv := strings.TrimSpace(part)
		if v, ok := strings.CutPrefix(kv, "flux_csrf="); ok {
			return v
		}
	}
	return ""
}

// freshCSRF GET /api/v1/me/csrf 获取新鲜 token（web 端 csrf() 机制对齐）。
// 响应=裸形态 {"csrf_token":"..."}（无统一信封 code/data 包装——不能走
// apiCall 信封解包）。
func (a *HxptAdapter) freshCSRF(ctx context.Context, config *model.SiteConfig) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.base(config)+"/api/v1/me/csrf", nil)
	if err != nil {
		return "", networkError("构造 csrf 请求失败: "+err.Error(), err)
	}
	if config.Cookie != "" {
		req.Header.Set("Cookie", config.Cookie)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return "", networkError("csrf 请求失败: "+err.Error(), err)
	}
	defer func() { drainBody(resp) }()
	rb, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", parseError("读取 csrf 响应失败", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", httpError(fmt.Sprintf("好学 csrf HTTP %d: %s", resp.StatusCode, truncate(string(rb), 120)), nil)
	}
	var out struct {
		CSRFToken string `json:"csrf_token"`
	}
	if err := json.Unmarshal(rb, &out); err != nil {
		return "", parseError("csrf 响应解析失败: "+err.Error(), err)
	}
	return out.CSRFToken, nil
}

// ── 获取方法 ──

func (a *HxptAdapter) SearchTorrents(ctx context.Context, config *model.SiteConfig, keyword string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
	limit := 100
	if opts != nil && opts.MaxResults > 0 {
		limit = opts.MaxResults
	}
	if limit > 100 {
		limit = 100
	}
	// page 从 1 起（cursor 分页 next_cursor；页码形态生产够用——
	// 引擎搜索单轮消费不翻页）。
	page := 1
	if opts != nil && opts.Page > 0 {
		page = opts.Page
	}
	u := fmt.Sprintf("%s/api/v1/torrents?search=%s&page=%d&limit=%d",
		a.base(config), urlQueryEscape(keyword), page, limit)
	var payload struct {
		Items      []hxptItem `json:"items"`
		NextCursor string     `json:"next_cursor"`
	}
	if err := a.apiCall(ctx, config, http.MethodGet, u, nil, &payload); err != nil {
		return nil, err
	}
	results := make([]*model.SeedingSearchResult, 0, len(payload.Items))
	for _, it := range payload.Items {
		results = append(results, hxptItemToResult(it))
	}
	return results, nil
}

func hxptItemToResult(it hxptItem) *model.SeedingSearchResult {
	r := &model.SeedingSearchResult{
		TorrentID: strconv.FormatInt(it.ID, 10),
		Title:     it.Name,
		Size:      it.Size,
		Seeders:   it.Seeders,
		Leechers:  it.Leechers,
	}
	if t := hxptParseTime(it.CreatedAt); !t.IsZero() {
		r.PublishAt = t
	}
	return r
}

func hxptParseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

// fetchMeta GET /api/v1/torrents/{id}——元数据（hr_policy/info_hash/
// category_id/seeders）。404 映射 AppError（VerifyExists 消费）。
func (a *HxptAdapter) fetchMeta(ctx context.Context, config *model.SiteConfig, torrentID string) (*hxptItem, error) {
	u := fmt.Sprintf("%s/api/v1/torrents/%s", a.base(config), urlQueryEscape(torrentID))
	var it hxptItem
	if err := a.apiCall(ctx, config, http.MethodGet, u, nil, &it); err != nil {
		return nil, err
	}
	return &it, nil
}

// hxptDetail /detail 端点（内容面：descr/mediainfo/sections）。
type hxptDetail struct {
	ID         int64           `json:"id"`
	Name       string          `json:"name"`
	Size       int64           `json:"size"`
	Seeders    int             `json:"seeders"`
	Descr      string          `json:"descr"`
	NumFiles   int             `json:"numfiles"`
	MediaInfo  string          `json:"mediainfo"`
	Sections   json.RawMessage `json:"sections"`
	PosState   int             `json:"pos_state"`
	PickType   int             `json:"pick_type"`
	IsOwner    bool            `json:"is_owner"`
}

func (a *HxptAdapter) fetchDetail(ctx context.Context, config *model.SiteConfig, torrentID string) (*hxptDetail, error) {
	u := fmt.Sprintf("%s/api/v1/torrents/%s/detail", a.base(config), urlQueryEscape(torrentID))
	var d hxptDetail
	if err := a.apiCall(ctx, config, http.MethodGet, u, nil, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

func (a *HxptAdapter) GetTorrentDetail(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	// 双端点合并：/{id} 元数据 + /{id}/detail 内容（站方内容与元数据分离）。
	meta, err := a.fetchMeta(ctx, config, torrentID)
	if err != nil {
		return nil, err
	}
	d := &model.TorrentDetail{
		InfoHash:    meta.InfoHash,
		Title:       meta.Name,
		Subtitle:    meta.SmallDescr,
		Category:    hxptCategoryNames[meta.CategoryID],
		IMDbID:      meta.IMDbID,
		PosterURL:   meta.Poster,
		Size:        meta.Size,
		ReleaseGroup: hxptGroupOfName(meta.Name),
	}
	if t := hxptParseTime(meta.CreatedAt); !t.IsZero() {
		d.PublishedAt = &t
	}
	if strings.Contains(meta.IMDbID, "tt") && len(meta.IMDbID) >= 2 {
		d.IMDbURL = "https://www.imdb.com/title/" + meta.IMDbID + "/"
	}
	detail, err := a.fetchDetail(ctx, config, torrentID)
	if err == nil && detail != nil {
		d.Description = detail.Descr
		d.MediaInfo = detail.MediaInfo
	}
	if err := CheckDetailVacuum(d); err != nil {
		return nil, err
	}
	return d, nil
}

// hxptGroupOfName 标题尾 -GROUP 提取（API 无独立组字段——sections 的
// team 维度在 /detail 才有·组名标题尾部惯用形态解析）。
func hxptGroupOfName(name string) string {
	i := strings.LastIndex(name, "-")
	if i <= 0 || i == len(name)-1 {
		return ""
	}
	tail := name[i+1:]
	if tail == "" || strings.ContainsAny(tail, " []") {
		return ""
	}
	return tail
}

func (a *HxptAdapter) DownloadTorrent(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	domain := config.BaseURL
	if domain == "" {
		domain = config.Domain
	}
	if err := httpclient.GlobalDownloadLimiter.AcquireWithLimit(domain, config.DownloadHourlyLimit); err != nil {
		return nil, downloadError("下载限流: "+err.Error(), nil)
	}
	u := fmt.Sprintf("%s/api/v1/torrents/%s/download", a.base(config), urlQueryEscape(torrentID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, networkError("构造下载请求失败", err)
	}
	if config.Cookie != "" {
		req.Header.Set("Cookie", config.Cookie)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("下载请求失败: "+err.Error(), err)
	}
	defer func() { drainBody(resp) }()
	rb, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, parseError("读取种子数据失败", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmt.Sprintf("好学下载 HTTP %d: %s", resp.StatusCode, truncate(string(rb), 200)), nil)
	}
	if len(rb) < 20 || !strings.HasPrefix(string(rb[:11]), "d8:announce") && !strings.HasPrefix(string(rb[:7]), "d4:info") {
		if a.logger != nil {
			a.logger.Warn("hxpt download: 非 bencode 响应", zap.Int("size", len(rb)))
		}
	}
	return rb, nil
}

// DetectDiscount promotion 字段（列表全 null 实证=无促销；非 null 值形态
// 未实证——宽容字符串匹配+日志留痕·形态生产首见后修正映射）。
func (a *HxptAdapter) DetectDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	meta, err := a.fetchMeta(ctx, config, torrentID)
	if err != nil {
		return nil, err
	}
	res := &model.DiscountResult{Level: model.DiscountNone}
	if meta.PromotionEndsAt != nil {
		res.FreeEndAt = meta.PromotionEndsAt
	}
	if len(meta.Promotion) == 0 || string(meta.Promotion) == "null" {
		return res, nil
	}
	if a.logger != nil {
		a.logger.Info("hxpt promotion 形态首见（映射待实证修正）",
			zap.String("site", config.SiteName), zap.String("torrent_id", torrentID),
			zap.String("promotion", string(meta.Promotion)))
	}
	p := strings.ToLower(string(meta.Promotion))
	switch {
	case strings.Contains(p, "2x") && strings.Contains(p, "free"):
		res.Level = model.Discount2xFree
	case strings.Contains(p, "free"):
		res.Level = model.DiscountFree
	case strings.Contains(p, "2x"):
		res.Level = model.Discount2xUp
	case strings.Contains(p, "50"):
		res.Level = model.DiscountPercent50
	case strings.Contains(p, "30"):
		res.Level = model.DiscountPercent30
	case strings.Contains(p, "25"):
		res.Level = model.DiscountPercent25
	}
	res.Multiplier = -1 // 形态未知（站方无倍率字段实证）——恒 -1 不投假值
	return res, nil
}

// DetectHR hr_policy 直读（/{id} 端点·{days,enabled,seed_hours}）。
func (a *HxptAdapter) DetectHR(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
	meta, err := a.fetchMeta(ctx, config, torrentID)
	if err != nil {
		return nil, err
	}
	res := &model.HRResult{HasHR: false}
	if meta.HRPolicy != nil && meta.HRPolicy.Enabled {
		res.HasHR = true
		res.SeedTimeH = meta.HRPolicy.SeedHours
	}
	return res, nil
}

func (a *HxptAdapter) FetchUserStats(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	var me struct {
		ID          int64  `json:"id"`
		Username    string `json:"username"`
		ClassName   string `json:"class_name"`
		Uploaded    int64  `json:"uploaded"`
		Downloaded  int64  `json:"downloaded"`
		Seeding     int    `json:"seeding"`
		Leeching    int    `json:"leeching"`
		Uploads     int    `json:"uploads"`
	}
	if err := a.apiCall(ctx, config, http.MethodGet, a.base(config)+"/api/v1/me", nil, &me); err != nil {
		return nil, err
	}
	res := &model.UserStatsResult{
		Username:     me.Username,
		UserClass:    me.ClassName,
		UploadBytes:  me.Uploaded,
		DownloadBytes: me.Downloaded,
		SeedingCount: me.Seeding,
	}
	if me.Downloaded > 0 {
		res.Ratio = float64(me.Uploaded) / float64(me.Downloaded)
	}
	// ② /api/v1/users/{id}（个人主页——用户实测指引）：seeding_size 全量
	// 做种体积（me/spark 只有官种子集）+spark_balance 魔力单点拿全。
	// me.id 提供用户 id（users/10056 页面 SSR+API 实测）。
	// UploadBytes/Downloaded 保持 me 端点显示值（users 的 real_* 是考核
	// 真实账本·两套语义——顶栏展示口径与 me 一致）。
	var uid int64
	if id, err := strconv.ParseInt(strings.TrimSpace(fmt.Sprint(me.ID)), 10, 64); err == nil && id > 0 {
		uid = id
	}
	if uid > 0 {
		var up struct {
			SeedingSize  int64   `json:"seeding_size"`
			SparkBalance float64 `json:"spark_balance"`
		}
		u := fmt.Sprintf("%s/api/v1/users/%d", a.base(config), uid)
		if err := a.apiCall(ctx, config, http.MethodGet, u, nil, &up); err == nil {
			res.SeedingSize = up.SeedingSize
			res.BonusPoints = up.SparkBalance
		} else if a.logger != nil {
			a.logger.Warn("hxpt users/{id} 拉取失败（做种体积/魔力降级）",
				zap.String("site", config.SiteName), zap.Error(err))
		}
	}
	return res, nil
}

func (a *HxptAdapter) GetTorrentInfoHash(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	meta, err := a.fetchMeta(ctx, config, torrentID)
	if err != nil {
		return "", err
	}
	return meta.InfoHash, nil
}

func (a *HxptAdapter) SupportsSearchByPiecesHash() bool { return false }

func (a *HxptAdapter) VerifyExists(ctx context.Context, config *model.SiteConfig, torrentID string) (bool, error) {
	_, err := a.fetchMeta(ctx, config, torrentID)
	if err == nil {
		return true, nil
	}
	if isHTTP404(err) {
		return false, nil
	}
	return false, err
}

// GetPreciseSLData /{id} 的 seeders/leechers 直读（SL 消费面最小实现；
// FreeEndAt 复用 promotion_ends_at）。
func (a *HxptAdapter) GetPreciseSLData(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	meta, err := a.fetchMeta(ctx, config, torrentID)
	if err != nil {
		return nil, err
	}
	return &model.SLData{
		Seeders:   meta.Seeders,
		Leechers: meta.Leechers,
		FreeEndAt: meta.PromotionEndsAt,
	}, nil
}

func (a *HxptAdapter) GetBatchSLData(ctx context.Context, config *model.SiteConfig, torrentIDs []string) (map[string]*model.SLData, error) {
	out := make(map[string]*model.SLData, len(torrentIDs))
	for _, id := range torrentIDs {
		sl, err := a.GetPreciseSLData(ctx, config, id)
		if err != nil {
			continue // 单条失败不废批次（对齐 GetBatchSLData 宽容语义）
		}
		out[id] = sl
	}
	return out, nil
}

// ── 发布（§59.533 刀3）──

// hxptSectionKinds 好学 sections 维度（全站统一实证——五分类对照
// category 1/2/4/7/8/12 kinds 恒为四维全 select·值集数量全同；
// section-dict 参数化只是形式）。提交形态=纯数字 dict_id（select
// 类型·UploadForm JS 逆向 a[n]=Number(r) 不包装；multiselect 等
// field_type 分支站内无实例不实现——按需生长）。
var hxptSectionKinds = []string{"media", "codec", "source", "team"}

// UploadTorrent POST /api/v1/torrents（FormData·cookie+CSRF double-submit）。
//
// 契约（UploadForm chunk 逆向+tags-dict 实测·2026-11-15）：
//   - 恒发：category_id/anonymous/file
//   - 条件发（非空才发·对齐站方 l() 语义）：name/imdb/douban/tmdb/
//     small_descr/descr/poster/mediainfo/sections/tags
//   - 恒不发：price（用户定案免费发布）/pos_state/pick_type（站方
//     管理功能）/hr_punish（HR 自愿）/log/nfo（音乐向）
//   - sections：{"media":443,...} 纯数字 dict_id JSON
//   - tags：[9,10] 数值 id 数组 JSON（TagArrayFields 聚合）
//   - 响应：code=0 → data.id（detail=/torrent/{id}）；1002=字段校验
//     errors map；其他 code=message
//
// 形态未实证项（DryRun/首颗暴露·不臆造）：重复发种 code/HTTP 形态
// （JS 无 409 分支）；上传权限 403 文案。
func (a *HxptAdapter) UploadTorrent(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	if len(req.TorrentData) == 0 {
		return nil, &model.AppError{Code: 40001, Message: "种子文件数据为空"}
	}
	if config.Cookie == "" {
		return nil, &model.AppError{Code: 40001, Message: "好学 API 发布需要 cookie（站点配置认证凭证）"}
	}
	// §59.533 附三 403 破案：好学上传 403 code 2003「无权限」=账号上传权限
	// 未开（全分类实证·web 端同端点同凭证——浏览器手动传同样 403·联系站方）。
	// CSRF 通道澄清：web 端 csrf() **cookie 静态值优先**（有值直接用·me/csrf
	// 仅缺 cookie 兜底——4710 chunk p() 函数逆向）——cookie 缺失时动态获取。
	csrf := hxptCSRFToken(config)
	if csrf == "" {
		if fresh, err := a.freshCSRF(ctx, config); err == nil && fresh != "" {
			csrf = fresh
		} else if a.logger != nil {
			a.logger.Warn("hxpt upload: cookie 无 flux_csrf 且动态获取失败（POST 可能被站方 403）",
				zap.String("site", config.SiteName), zap.Error(err))
		}
	}
	categoryID := strings.TrimSpace(req.FormFields["category_id"])
	if categoryID == "" {
		return nil, &model.AppError{Code: 40001, Message: "好学发布缺 category_id（刀4 值集映射产出——form_fields.category_id）"}
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	fw, err := writer.CreateFormFile("file", "upload.torrent")
	if err != nil {
		return nil, uploadError("创建表单文件字段失败", err)
	}
	if _, err := fw.Write(req.TorrentData); err != nil {
		return nil, networkError("写入种子数据失败", err)
	}
	if err := writer.WriteField("category_id", categoryID); err != nil {
		return nil, uploadError("写入 category_id 失败", err)
	}
	if err := writer.WriteField("anonymous", strconv.FormatBool(req.Anonymous)); err != nil {
		return nil, uploadError("写入 anonymous 失败", err)
	}
	writeIf := func(key, val string) {
		if val == "" || err != nil {
			return
		}
		err = writer.WriteField(key, val)
	}
	writeIf("name", strings.TrimSpace(req.Title))
	writeIf("imdb", strings.TrimSpace(req.IMDbLink))
	writeIf("douban", strings.TrimSpace(req.DoubanLink))
	writeIf("tmdb", strings.TrimSpace(req.FormFields["tmdb"]))
	writeIf("small_descr", strings.TrimSpace(req.Subtitle))
	writeIf("descr", req.Description)
	writeIf("poster", strings.TrimSpace(req.FormFields["poster"]))
	writeIf("mediainfo", strings.TrimSpace(req.MediaInfo))
	if err != nil {
		return nil, uploadError("写入文本字段失败", err)
	}

	// sections：四维 dict_id 数值化（值=值集词条 id 字符串）。
	sections := make(map[string]int64, len(hxptSectionKinds))
	for _, kind := range hxptSectionKinds {
		v := strings.TrimSpace(req.FormFields[kind])
		if v == "" {
			continue
		}
		n, perr := strconv.ParseInt(v, 10, 64)
		if perr != nil || n <= 0 {
			return nil, &model.AppError{Code: 40001,
				Message: fmt.Sprintf("好学 sections[%s]=%q 非 dict_id 数值（值集映射应产出词条 id）", kind, v)}
		}
		sections[kind] = n
	}
	if len(sections) > 0 {
		if b, merr := json.Marshal(sections); merr == nil {
			if werr := writer.WriteField("sections", string(b)); werr != nil {
				return nil, uploadError("写入 sections 失败", werr)
			}
		}
	}

	// tags：TagArrayFields 聚合数值 id 数组（系统 tag 组装单点
	// AssembleTagArrayForSite 产物·Taglist applier 硬编码 Key="tagList"——
	// TNode/Yemapt 同款匹配约定；非数值项（站方值集 miss 的标准键直出）
	// ParseInt 跳过自然蒸发——好学只收数值 id）。
	var tagIDs []int64
	for _, kv := range req.TagArrayFields {
		if kv.Key != "tagList" {
			continue
		}
		if n, perr := strconv.ParseInt(strings.TrimSpace(kv.Value), 10, 64); perr == nil && n > 0 {
			tagIDs = append(tagIDs, n)
		}
	}
	if len(tagIDs) > 0 {
		if b, merr := json.Marshal(tagIDs); merr == nil {
			if werr := writer.WriteField("tags", string(b)); werr != nil {
				return nil, uploadError("写入 tags 失败", werr)
			}
		}
	}
	if err := writer.Close(); err != nil {
		return nil, uploadError("关闭 multipart 失败", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.base(config)+"/api/v1/torrents", &buf)
	if err != nil {
		return nil, networkError("构造上传请求失败", err)
	}
	httpReq.Header.Set("Cookie", config.Cookie)
	if csrf != "" {
		httpReq.Header.Set("x-csrf-token", csrf)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")

	resp, err := a.doer.Client.Do(httpReq)
	if err != nil {
		return nil, networkError("上传请求失败: "+err.Error(), err)
	}
	defer func() { drainBody(resp) }()
	rb, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmt.Sprintf("好学上传 HTTP %d: %s", resp.StatusCode, truncate(string(rb), 300)), nil)
	}
	var env struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rb, &env); err != nil {
		return nil, parseError("上传响应解析失败: "+err.Error(), err)
	}
	if env.Code == 0 {
		var data struct {
			ID any `json:"id"`
		}
		_ = json.Unmarshal(env.Data, &data)
		tid := ""
		switch v := data.ID.(type) {
		case float64:
			tid = strconv.FormatInt(int64(v), 10)
		case string:
			tid = v
		}
		pubResp := &model.PublishResponse{
			Success:    true,
			TorrentID:  tid,
			IDSource:   "api_response", // JSON 信封直读（与 302 权威同级的 API 通道）
			TargetSite: config.SiteName,
		}
		if tid != "" {
			pubResp.DetailURL = a.base(config) + "/torrent/" + tid
		}
		return pubResp, nil
	}
	if env.Code == 1002 {
		// 字段校验错误：data={field: [msgs]} errors map（UploadForm JS
		// Object.entries 逆向）——拼接透出给用户/DryRun。
		var errs map[string]any
		msgs := make([]string, 0, 4)
		if jerr := json.Unmarshal(env.Data, &errs); jerr == nil {
			for field, v := range errs {
				switch vv := v.(type) {
				case []any:
					for _, m := range vv {
						msgs = append(msgs, fmt.Sprintf("%s: %v", field, m))
					}
				default:
					msgs = append(msgs, fmt.Sprintf("%s: %v", field, v))
				}
			}
		}
		if len(msgs) > 0 {
			return nil, &model.AppError{Code: 40001, Message: "好学字段校验失败: " + strings.Join(msgs, "; ")}
		}
	}
	return nil, &model.AppError{Code: 40001, Message: fmt.Sprintf("好学上传 code=%d: %s", env.Code, env.Message)}
}
