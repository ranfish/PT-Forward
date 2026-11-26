package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

// HxptAdapter 好学（framework=hxpt·§59.533 刀2→§59.570 Token 全切）。
//
// 站方 2026-11 改版离开 NP·2026-11-17/18 上线第三方 API 体系。
// §59.570 全切 Token（用户定案 2026-11-25）：
//   - 鉴权：Authorization: Token fxo_...（sites.APIKey·api_key 列·auth_type=apikey——馒头先例同款字段）
//   - 搜索：GET /api/v1/compat/nexusphp/torrents.json（NP 兼容层·Token）
//   - 详情：GET /api/v1/open/torrents/{id}（开放 API·深数据）
//   - 下载：POST /api/v1/downloads/keys → GET /api/v1/downloads/{id}?token=fxk_...
//   - 用户：GET /api/v1/plugins/ptppUserInfo（PT-Plugin-Plus 口径）
//   - 发种：POST /api/v1/open/torrents（Token+multipart·info_hash 幂等·
//     元数据走查询参数〔v0.0.1618 契约对齐——descr/mediainfo/tags 无投递通道〕）
//   - ⚠ §59.572 关闸（v0.0.1618）：站方 open API 上传不注入 source（web 路径
//     注入）→ 种子与源站同 info_hash → 下载器去重无法做种——等站方修后重启
//   - CSRF：Token 免·全链移除
//   - 限流：60/min·429+Retry-After 退避
//
// 统一信封 {code, message, data}；code===0 成功（旧 SPA 同款——兼容层/
// 开放 API 均沿用此信封·openapi.json 为权威）。
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

// hxptCompatItem NP 兼容层 torrents.json 条目形态。
type hxptCompatItem struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	SmallDescr   string `json:"small_descr"`
	Seeders      int    `json:"seeders"`
	Leechers     int    `json:"leechers"`
	Completed    int    `json:"completed"`
	Size         int64  `json:"size"`  // 字节
	Added        int64  `json:"added"` // Unix 秒
	Category     int    `json:"category"`
	CategoryName string `json:"category_name"`
	CategoryNP   int    `json:"category_np"`
	InfoHash     string `json:"info_hash"`
	PiecesHash   string `json:"pieces_hash"`
	Free         bool   `json:"free"`
	Official     bool   `json:"official"`
	Promotion    string `json:"promotion"` // free/x2/half/none
}

// hxptOpenTorrent 开放 API /open/torrents/{id} 深详情形态。
type hxptOpenTorrent struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	InfoHash string `json:"info_hash"`
	// NP 兼容层与开放 API 共有的核心字段
	Seeders    int   `json:"seeders"`
	Leechers   int   `json:"leechers"`
	Size       int64 `json:"size"`
	Added      int64 `json:"added"`
	CategoryID int   `json:"category_id"`
	// 开放 API 独有深数据
	SmallDescr string          `json:"small_descr"`
	Descr      string          `json:"descr"`
	MediaInfo  string          `json:"mediainfo"`
	NumFiles   int             `json:"numfiles"`
	Poster     string          `json:"poster"`
	IMDbID     string          `json:"imdb_id"`
	Sections   json.RawMessage `json:"sections"`
	Promotion  string          `json:"promotion"`
	HRPolicy   *hxptHRPolicy   `json:"hr_policy"`
}

type hxptHRPolicy struct {
	Days      int  `json:"days"`
	Enabled   bool `json:"enabled"`
	SeedHours int  `json:"seed_hours"`
}

// hxptCategoryNames 分类 id→名（NP 兼容层 category_name 原生带——备用）。
var hxptCategoryNames = map[int]string{
	1: "电影", 2: "电视剧", 3: "短剧", 4: "音乐", 5: "游戏", 6: "软件",
	7: "动漫", 8: "纪录片", 9: "电子书", 10: "有声书", 11: "体育",
	12: "综艺", 13: "教育资源",
}

// ── 内部请求骨架 ──

func (a *HxptAdapter) base(config *model.SiteConfig) string {
	// §59.575: 复用公共 siteBaseURL（三态：BaseURL 优先/带 scheme 的 Domain/
	// 裸 Domain 补 https）。原手搓 "https://"+Domain 在统计同步链的
	// Domain=site.BaseURL（带 scheme）形态下产出 "https://https//..." 双
	// scheme（12:00 同步 EOF 实证·§59.570 Token 修复前被 token 检查掩盖）。
	return siteBaseURL(config)
}

// apiCall §59.570 Token 鉴权请求。Authorization: Token fxo_...（APIKey 字段·api_key 列）。
// 统一信封 {code, message, data}；code!==0 返回错误。
// 429 限流：读取 Retry-After 并返回带提示的错误（调用方退避）。
func (a *HxptAdapter) apiCall(ctx context.Context, config *model.SiteConfig, method, url string, body io.Reader, out any) error {
	token := strings.TrimSpace(config.APIKey)
	if token == "" {
		return &model.AppError{Code: 40001, Message: "好学 API 需要 Token（站点配置认证凭证·fxo_ 开头）"}
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return networkError("构造请求失败: "+err.Error(), err)
	}
	// §59.570: Token 鉴权（免 cookie/CSRF）
	req.Header.Set("Authorization", "Token "+token)
	req.Header.Set("User-Agent", httpclient.BrowserUserAgent())
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
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := resp.Header.Get("Retry-After")
		return &model.AppError{Code: 42900, Message: fmt.Sprintf("好学 API 限流 429（Retry-After: %ss·60/min）", retryAfter)}
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

// ── 搜索（NP 兼容层） ──

func (a *HxptAdapter) SearchTorrents(ctx context.Context, config *model.SiteConfig, keyword string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
	// §59.570: NP 兼容层 torrents.json（Token 鉴权·page/pagesize 翻页）
	u := fmt.Sprintf("%s/api/v1/compat/nexusphp/torrents.json?page=1&pagesize=50", a.base(config))
	if keyword != "" {
		u += "&keyword=" + url.QueryEscape(keyword)
	}
	var resp struct {
		Page     int              `json:"page"`
		PageSize int              `json:"page_size"`
		Total    int              `json:"total_estimate"`
		HasMore  bool             `json:"has_more"`
		Items    []hxptCompatItem `json:"items"`
	}
	if err := a.apiCall(ctx, config, http.MethodGet, u, nil, &resp); err != nil {
		return nil, err
	}
	results := make([]*model.SeedingSearchResult, 0, len(resp.Items))
	for i := range resp.Items {
		results = append(results, hxptCompatToResult(resp.Items[i]))
	}
	return results, nil
}

func hxptCompatToResult(it hxptCompatItem) *model.SeedingSearchResult {
	catName := it.CategoryName
	if catName == "" {
		catName = hxptCategoryNames[it.Category]
	}
	_ = catName // Category 不在 SeedingSearchResult（仅 TorrentDetail）
	return &model.SeedingSearchResult{
		TorrentID: strconv.FormatInt(it.ID, 10),
		Title:     it.Name,
		Size:      it.Size,
		Seeders:   it.Seeders,
		Leechers:  it.Leechers,
		PublishAt: time.Unix(it.Added, 0),
	}
}

// ── 详情（开放 API 深数据） ──

func (a *HxptAdapter) GetTorrentDetail(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	// §59.570: 开放 API 深详情（单端点·含 descr/mediainfo/sections）
	u := fmt.Sprintf("%s/api/v1/open/torrents/%s", a.base(config), urlQueryEscape(torrentID))
	var t hxptOpenTorrent
	if err := a.apiCall(ctx, config, http.MethodGet, u, nil, &t); err != nil {
		return nil, err
	}
	d := &model.TorrentDetail{
		InfoHash:     t.InfoHash,
		Title:        t.Name,
		Subtitle:     t.SmallDescr,
		Category:     hxptCategoryNames[t.CategoryID],
		Description:  t.Descr,
		MediaInfo:    t.MediaInfo,
		IMDbID:       t.IMDbID,
		PosterURL:    t.Poster,
		Size:         t.Size,
		ReleaseGroup: hxptGroupOfName(t.Name),
	}
	if t.Added > 0 {
		ts := time.Unix(t.Added, 0)
		d.PublishedAt = &ts
	}
	if strings.Contains(t.IMDbID, "tt") && len(t.IMDbID) >= 2 {
		d.IMDbURL = "https://www.imdb.com/title/" + t.IMDbID + "/"
	}
	if err := CheckDetailVacuum(d); err != nil {
		return nil, err
	}
	return d, nil
}

// hxptGroupOfName 标题尾 -GROUP 提取。
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

// ── 下载（临时凭证两步） ──

// DownloadTorrent §59.570: Token 两步下载——POST /downloads/keys 换
// fxk_ 临时凭证（30min 泄露窗口·推荐）→ GET /downloads/{id}?token=fxk_...。
// 也可走 fxr_ 直下（compat/nexusphp/download.php?token=fxr_）但需要
// 额外一种 token·我们统一用 API Token 换临时凭证。
func (a *HxptAdapter) DownloadTorrent(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	domain := config.BaseURL
	if domain == "" {
		domain = config.Domain
	}
	if err := httpclient.GlobalDownloadLimiter.AcquireWithLimit(domain, config.DownloadHourlyLimit); err != nil {
		return nil, downloadError("下载限流: "+err.Error(), nil)
	}
	token := strings.TrimSpace(config.APIKey)
	if token == "" {
		return nil, &model.AppError{Code: 40001, Message: "好学下载需要 API Token（站点配置认证凭证）"}
	}

	// ① POST /downloads/keys 换 fxk_ 临时凭证
	id, err := strconv.ParseInt(torrentID, 10, 64)
	if err != nil {
		return nil, parseError("种子 ID 解析失败: "+torrentID, err)
	}
	keyReq, _ := json.Marshal(map[string]int64{"torrent_id": id})
	keyURL := fmt.Sprintf("%s/api/v1/downloads/keys", a.base(config))
	keyHTTPReq, err := http.NewRequestWithContext(ctx, http.MethodPost, keyURL, bytes.NewReader(keyReq))
	if err != nil {
		return nil, networkError("构造凭证请求失败: "+err.Error(), err)
	}
	keyHTTPReq.Header.Set("Authorization", "Token "+token)
	keyHTTPReq.Header.Set("Content-Type", "application/json")
	keyHTTPReq.Header.Set("User-Agent", httpclient.BrowserUserAgent())
	keyResp, err := a.doer.Client.Do(keyHTTPReq)
	if err != nil {
		return nil, networkError("凭证请求失败: "+err.Error(), err)
	}
	defer func() { drainBody(keyResp) }()
	keyBody, _ := io.ReadAll(io.LimitReader(keyResp.Body, 1<<20))
	if keyResp.StatusCode != http.StatusOK {
		return nil, httpError(fmt.Sprintf("好学凭证 HTTP %d: %s", keyResp.StatusCode, truncate(string(keyBody), 200)), nil)
	}
	var keyEnv hxptEnvelope
	if err := json.Unmarshal(keyBody, &keyEnv); err != nil {
		return nil, parseError("凭证响应解析失败: "+err.Error(), err)
	}
	if keyEnv.Code != 0 {
		return nil, &model.AppError{Code: 40001, Message: fmt.Sprintf("好学凭证 code=%d: %s", keyEnv.Code, keyEnv.Message)}
	}
	var keyData struct {
		Key         string `json:"key"`
		DownloadURL string `json:"download_url"`
	}
	if err := json.Unmarshal(keyEnv.Data, &keyData); err != nil {
		return nil, parseError("凭证 data 解析失败: "+err.Error(), err)
	}
	if keyData.Key == "" {
		return nil, &model.AppError{Code: 40001, Message: "好学凭证返回空 key"}
	}

	// ② GET /downloads/{id}?token=fxk_...（免 Authorization）
	dlURL := keyData.DownloadURL
	if dlURL == "" {
		dlURL = fmt.Sprintf("%s/api/v1/downloads/%s?token=%s", a.base(config), urlQueryEscape(torrentID), keyData.Key)
	}
	dlReq, err := http.NewRequestWithContext(ctx, http.MethodGet, dlURL, nil)
	if err != nil {
		return nil, networkError("构造下载请求失败: "+err.Error(), err)
	}
	dlReq.Header.Set("User-Agent", httpclient.BrowserUserAgent())
	dlResp, err := a.doer.Client.Do(dlReq)
	if err != nil {
		return nil, networkError("下载请求失败: "+err.Error(), err)
	}
	defer func() { drainBody(dlResp) }()
	rb, err := io.ReadAll(io.LimitReader(dlResp.Body, 32<<20))
	if err != nil {
		return nil, parseError("读取种子数据失败", err)
	}
	if dlResp.StatusCode != http.StatusOK {
		return nil, httpError(fmt.Sprintf("好学下载 HTTP %d: %s", dlResp.StatusCode, truncate(string(rb), 200)), nil)
	}
	return rb, nil
}

// ── 促销检测 ──

func (a *HxptAdapter) DetectDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	// §59.570: 开放 API 深详情 promotion 字段
	u := fmt.Sprintf("%s/api/v1/open/torrents/%s", a.base(config), urlQueryEscape(torrentID))
	var t hxptOpenTorrent
	if err := a.apiCall(ctx, config, http.MethodGet, u, nil, &t); err != nil {
		return nil, err
	}
	d := &model.DiscountResult{}
	switch strings.ToLower(t.Promotion) {
	case "free":
		d.Level = model.DiscountFree
	case "x2":
		d.Level = model.Discount2xUp
		d.Multiplier = 2.0
	case "half":
		d.Level = model.DiscountPercent50
		d.Multiplier = 0.5
	}
	return d, nil
}

// ── HR 检测 ──

func (a *HxptAdapter) DetectHR(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
	u := fmt.Sprintf("%s/api/v1/open/torrents/%s", a.base(config), urlQueryEscape(torrentID))
	var t hxptOpenTorrent
	if err := a.apiCall(ctx, config, http.MethodGet, u, nil, &t); err != nil {
		return nil, err
	}
	r := &model.HRResult{HasHR: t.HRPolicy != nil && t.HRPolicy.Enabled}
	if t.HRPolicy != nil && t.HRPolicy.Enabled {
		r.SeedTimeH = t.HRPolicy.SeedHours
	}
	return r, nil
}

// ── 用户统计（PT-Plugin-Plus 口径） ──

type hxptUserInfo struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Bonus        float64 `json:"bonus"`
	Uploaded     int64   `json:"uploaded"`
	Downloaded   int64   `json:"downloaded"`
	Seeding      int     `json:"seeding"`
	Leeching     int     `json:"leeching"`
	SeedingSize  int64   `json:"seedingSize"`
	Invites      int     `json:"invites"`
	LevelName    string  `json:"levelName"`
	JoinTime     string  `json:"joinTime"`
	MessageCount int     `json:"messageCount"`
}

func (a *HxptAdapter) FetchUserStats(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	u := fmt.Sprintf("%s/api/v1/plugins/ptppUserInfo", a.base(config))
	var info hxptUserInfo
	if err := a.apiCall(ctx, config, http.MethodGet, u, nil, &info); err != nil {
		return nil, err
	}
	r := &model.UserStatsResult{
		Username:      info.Name,
		UploadBytes:   info.Uploaded,
		DownloadBytes: info.Downloaded,
		SeedingCount:  info.Seeding,
		BonusPoints:   info.Bonus,
		UserClass:     info.LevelName,
	}
	if info.Downloaded > 0 {
		r.Ratio = float64(info.Uploaded) / float64(info.Downloaded)
	}
	return r, nil
}

// ── InfoHash ──

func (a *HxptAdapter) GetTorrentInfoHash(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	u := fmt.Sprintf("%s/api/v1/open/torrents/%s", a.base(config), urlQueryEscape(torrentID))
	var t hxptOpenTorrent
	if err := a.apiCall(ctx, config, http.MethodGet, u, nil, &t); err != nil {
		return "", err
	}
	return t.InfoHash, nil
}

func (a *HxptAdapter) SupportsSearchByPiecesHash() bool { return false }

func (a *HxptAdapter) VerifyExists(ctx context.Context, config *model.SiteConfig, torrentID string) (bool, error) {
	u := fmt.Sprintf("%s/api/v1/open/torrents/%s", a.base(config), urlQueryEscape(torrentID))
	var t hxptOpenTorrent
	if err := a.apiCall(ctx, config, http.MethodGet, u, nil, &t); err != nil {
		return false, nil // 404 → 不存在
	}
	return t.ID > 0, nil
}

func (a *HxptAdapter) GetPreciseSLData(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	detail, err := a.GetTorrentDetail(ctx, config, torrentID)
	if err != nil {
		return nil, err
	}
	_ = detail // TorrentDetail 无 Seeders/Leechers 字段——SLData 从开放 API 独立拉
	u := fmt.Sprintf("%s/api/v1/open/torrents/%s", a.base(config), urlQueryEscape(torrentID))
	var t hxptOpenTorrent
	if err := a.apiCall(ctx, config, http.MethodGet, u, nil, &t); err != nil {
		return nil, err
	}
	return &model.SLData{
		Seeders:  t.Seeders,
		Leechers: t.Leechers,
	}, nil
}

func (a *HxptAdapter) GetBatchSLData(ctx context.Context, config *model.SiteConfig, torrentIDs []string) (map[string]*model.SLData, error) {
	m := make(map[string]*model.SLData, len(torrentIDs))
	for _, id := range torrentIDs {
		if d, err := a.GetPreciseSLData(ctx, config, id); err == nil {
			m[id] = d
		}
	}
	return m, nil
}

// ── 发布（§59.570 刀2 待重写——当前保留旧 cookie+CSRF 形态·刀2 切换） ──
// ⚠ 以下 UploadTorrent 为 §59.533 旧版（cookie+CSRF）——刀2 全切 Token 后重写

var hxptSectionKinds = []string{"media", "codec", "source", "team"}

// UploadTorrent POST /api/v1/open/torrents（Token+multipart·info_hash 幂等）。
// §59.570 刀2 v2（v0.0.1618 契约对齐）：元数据走查询参数·multipart 只留 file。
// 站方当前契约（openapi.json 实拉·2026-11-25）：category_id/name/small_descr/
// anonymous/price 走 query 参数·multipart 只收 file+nfo（文本）。
// descr/mediainfo/tags/sections/imdb 无投递通道（API 未完成——§59.572 定案
// 好学关闸等站方）。price 恒不传（§59.533 定案：免费发布·23941 实证缺省=0）。
func (a *HxptAdapter) UploadTorrent(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	if len(req.TorrentData) == 0 {
		return nil, &model.AppError{Code: 40001, Message: "种子文件数据为空"}
	}
	token := strings.TrimSpace(config.APIKey)
	if token == "" {
		return nil, &model.AppError{Code: 40001, Message: "好学发布需要 API Token（站点配置认证凭证·fxo_ 开头）"}
	}

	categoryID := strings.TrimSpace(req.FormFields["category_id"])
	if categoryID == "" {
		return nil, &model.AppError{Code: 40001, Message: "好学发布需要 category_id"}
	}

	// 元数据走查询参数（url.Values 自动编码·中文/空格安全）
	q := url.Values{}
	q.Set("category_id", categoryID)
	if v := strings.TrimSpace(req.FormFields["name"]); v != "" {
		q.Set("name", v)
	}
	if v := strings.TrimSpace(req.FormFields["small_descr"]); v != "" {
		q.Set("small_descr", v)
	}
	// anonymous "1"/"0"（openapi 形态——非旧 v2 的 "true"/"false"）：
	// 缺省 "0"（23941 实证站方默认非匿名）
	anonymous := "0"
	if v, ok := req.FormFields["anonymous"]; ok && (v == "true" || v == "yes" || v == "1") {
		anonymous = "1"
	}
	q.Set("anonymous", anonymous)

	// multipart 只留 file（站方契约：nfo 可选文本域——§59.570 附二刀3 未定案·不投递）
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	fw, err := w.CreateFormFile("file", "torrent.torrent")
	if err != nil {
		return nil, parseError("创建 file 字段失败", err)
	}
	if _, err := fw.Write(req.TorrentData); err != nil {
		return nil, networkError("写入种子数据失败", err)
	}
	if err := w.Close(); err != nil {
		return nil, parseError("关闭 multipart writer 失败", err)
	}

	// POST /api/v1/open/torrents?category_id=...（Token 鉴权·multipart）
	uploadURL := fmt.Sprintf("%s/api/v1/open/torrents?%s", a.base(config), q.Encode())
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &buf)
	if err != nil {
		return nil, networkError("构造上传请求失败: "+err.Error(), err)
	}
	httpReq.Header.Set("Authorization", "Token "+token)
	httpReq.Header.Set("Content-Type", w.FormDataContentType())
	httpReq.Header.Set("User-Agent", httpclient.BrowserUserAgent())

	resp, err := a.doer.Client.Do(httpReq)
	if err != nil {
		return nil, networkError("上传请求失败: "+err.Error(), err)
	}
	defer func() { drainBody(resp) }()
	rb, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, parseError("读取上传响应失败", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmt.Sprintf("好学上传 HTTP %d: %s", resp.StatusCode, truncate(string(rb), 300)), nil)
	}

	// 解析响应：{code, message, data: {id, duplicate, detail_url}}
	var env hxptEnvelope
	if err := json.Unmarshal(rb, &env); err != nil {
		return nil, parseError("上传响应解析失败: "+err.Error(), err)
	}
	if env.Code == 1002 {
		// 字段校验错误
		var errs map[string][]string
		_ = json.Unmarshal(env.Data, &errs)
		msgs := make([]string, 0, len(errs))
		for field, fieldMsgs := range errs {
			for _, m := range fieldMsgs {
				msgs = append(msgs, field+": "+m)
			}
		}
		return nil, &model.AppError{Code: 40001, Message: "好学上传字段校验失败: " + strings.Join(msgs, "; ")}
	}
	if env.Code != 0 {
		return nil, &model.AppError{Code: 40001, Message: fmt.Sprintf("好学上传 code=%d: %s", env.Code, env.Message)}
	}

	var data struct {
		ID        int64  `json:"id"`
		Duplicate bool   `json:"duplicate"`
		DetailURL string `json:"detail_url"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return nil, parseError("上传 data 解析失败: "+err.Error(), err)
	}

	// §59.570 附二刀1：Success 标志必设——executor_upload.go:223 判
	// !resp.Success →「上传未成功」→ failed 终态+不推种（243 首颗 5310
	// 实证：站方 code=0 已建种 tid=23941·我们判 failed）。
	result := &model.PublishResponse{
		Success:   true,
		TorrentID: strconv.FormatInt(data.ID, 10),
		DetailURL: fmt.Sprintf("%s/torrent/%d", a.base(config), data.ID),
	}
	if data.Duplicate {
		result.IsExisting = true
	}
	return result, nil
}
