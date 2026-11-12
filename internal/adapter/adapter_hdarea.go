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
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

// HDAreaAdapter 好大统一 API 网关 adapter（framework=hdarea·2026-11-12 刀1）。
//
// 架构边界（用户定案·docs/32/hdarea-api.md §10）：
//   - 独立方言层——不 import mtorrentCore（M-Team 兼容=路由别名营销级非契约）
//   - 共享仅机制层（apihelper：X-API-Key/短效凭证流程）
//   - API-only 零 web 双路——回退阀=sites.framework 字段（可编辑·应急回 nexusphp）
//
// 实测契约（2026-11-12 活体探针·夹具 testdata/hdarea/）：
//   - 统一信封 {success,code,http_code,message,data}；code===0 成功
//   - genDlToken 独立信封 {ok,dl_token,expires_in,download_url}
//   - GET /api/v1/torrents?search=&pageSize= → data.items[]（分类 ID 全家/
//     sp_state 促销/info_hash/imdb 豆瓣 poster/forbid_transfer）
//   - detail 增 save_as/descr_bbcode/descr_html/nfo/files/other_copies
//   - RSS 走引擎标准 NP 解析（guid=infohash 真身·零侧载）——ParseRSS 委托
//     Generic stub 即可
//
// 待刀2：check 预查重接入/断种顶替语义/409↔四终态细化/source_sel+
// processing_sel 值集补配/return_torrent_base64 直推。
type HDAreaAdapter struct {
	*GenericAdapter
	doer   *HTTPDoer
	logger *zap.Logger
}

func NewHDAreaAdapter(doer *HTTPDoer, logger *zap.Logger) *HDAreaAdapter {
	return &HDAreaAdapter{GenericAdapter: NewGenericAdapter("hdarea", doer, logger), doer: doer, logger: logger}
}

func (a *HDAreaAdapter) Framework() string { return "hdarea" }

// ── 响应形态 ──

type hdareaEnvelope struct {
	Success bool            `json:"success"`
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type hdareaPagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

type hdareaItem struct {
	ID                    int64  `json:"id"`
	Name                  string `json:"name"`
	SmallDescr            string `json:"small_descr"`
	CategoryID            int    `json:"category_id"`
	CategoryName          string `json:"category_name"`
	SourceID              int    `json:"source_id"`
	SourceName            string `json:"source_name"`
	MediumID              int    `json:"medium_id"`
	MediumName            string `json:"medium_name"`
	CodecID               int    `json:"codec_id"`
	CodecName             string `json:"codec_name"`
	AudioCodecID          int    `json:"audiocodec_id"`
	AudioCodecName        string `json:"audiocodec_name"`
	StandardID            int    `json:"standard_id"`
	StandardName          string `json:"standard_name"`
	ProcessingID          int    `json:"processing_id"`
	ProcessingName        string `json:"processing_name"`
	TeamID                int    `json:"team_id"`
	TeamName              string `json:"team_name"`
	Size                  int64  `json:"size"`
	Seeders               int    `json:"seeders"`
	Leechers              int    `json:"leechers"`
	Added                 string `json:"added"`
	AddedTs               int64  `json:"added_ts"`
	SpState               int    `json:"sp_state"`
	SpStateName           string `json:"sp_state_name"`
	DownloadVolumeFactor  float64 `json:"download_volume_factor"`
	UploadVolumeFactor    float64 `json:"upload_volume_factor"`
	IsFree                bool   `json:"is_free"`
	PromotionUntil        string `json:"promotion_until"`
	IMDbID                string `json:"imdb_id"`
	IMDbURL               string `json:"imdb_url"`
	DoubanID              string `json:"douban_id"`
	DoubanURL             string `json:"douban_url"`
	PosterURL             string `json:"poster_url"`
	InfoHash              string `json:"info_hash"`
	DetailURL             string `json:"detail_url"`
	DownloadURL           string `json:"download_url"`
	ForbidTransfer        bool   `json:"forbid_transfer"`
	Anonymous             bool   `json:"anonymous"`
	UploaderName          string `json:"uploader_name"`
	// detail 独有
	SaveAs        string          `json:"save_as"`
	DescrBBCode   string          `json:"descr_bbcode"`
	DescrHTML     string          `json:"descr_html"`
	NFO           string          `json:"nfo"`
	Files         json.RawMessage `json:"files"`
	OtherCopies   json.RawMessage `json:"other_copies"`
}

// ── 内部请求骨架 ──

func (a *HDAreaAdapter) base(config *model.SiteConfig) string {
	if config.BaseURL != "" {
		return strings.TrimRight(config.BaseURL, "/")
	}
	return "https://" + config.Domain
}

// apiCall 统一请求：X-API-Key + JSON 信封解包。rawURL=true 时 url 为完整地址。
func (a *HDAreaAdapter) apiCall(ctx context.Context, config *model.SiteConfig, method, url string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return networkError("构造请求失败: "+err.Error(), err)
	}
	setXAPIKey(req, config.APIKey)
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
		return httpError(fmt.Sprintf("HDArea API HTTP %d: %s", resp.StatusCode, truncate(string(rb), 200)), nil)
	}
	if out == nil {
		return nil
	}
	var env hdareaEnvelope
	if err := json.Unmarshal(rb, &env); err != nil {
		return parseError("响应 JSON 解析失败: "+err.Error(), err)
	}
	if env.Code != 0 {
		return &model.AppError{Code: 40001, Message: fmt.Sprintf("HDArea API code=%d: %s", env.Code, env.Message)}
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return parseError("data 解析失败: "+err.Error(), err)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// ── 获取五方法 ──

func (a *HDAreaAdapter) SearchTorrents(ctx context.Context, config *model.SiteConfig, keyword string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
	pageSize := 100
	if opts != nil && opts.MaxResults > 0 {
		pageSize = opts.MaxResults
	}
	if pageSize > 100 {
		pageSize = 100
	}
	page := 1
	if opts != nil && opts.Page > 0 {
		page = opts.Page
	}
	url := fmt.Sprintf("%s/api/v1/torrents?search=%s&pageNumber=%d&pageSize=%d",
		a.base(config), urlQueryEscape(keyword), page, pageSize)
	var payload struct {
		Items      []hdareaItem   `json:"items"`
		Pagination hdareaPagination `json:"pagination"`
	}
	if err := a.apiCall(ctx, config, http.MethodGet, url, nil, &payload); err != nil {
		return nil, err
	}
	results := make([]*model.SeedingSearchResult, 0, len(payload.Items))
	for _, it := range payload.Items {
		results = append(results, hdareaItemToResult(it))
	}
	return results, nil
}

func hdareaItemToResult(it hdareaItem) *model.SeedingSearchResult {
	r := &model.SeedingSearchResult{
		TorrentID:   strconv.FormatInt(it.ID, 10),
		Title:       it.Name,
		Size:        it.Size,
		Seeders:     it.Seeders,
		Leechers:    it.Leechers,
		DetailURL:   it.DetailURL,
		DownloadURL: it.DownloadURL,
	}
	if t := hdareaParseTime(it.Added, it.AddedTs); !t.IsZero() {
		r.PublishAt = t
	}
	r.Discount = hdareaDiscountLevel(it.DownloadVolumeFactor, it.UploadVolumeFactor, it.SpState)
	return r
}

func hdareaDiscountLevel(dl, up float64, spState int) model.DiscountLevel {
	switch {
	case dl == 0 && up == 2:
		return model.Discount2xFree
	case dl == 0:
		return model.DiscountFree
	case up == 2:
		return model.Discount2xUp
	case spState == 5:
		return model.DiscountPercent50
	default:
		return model.DiscountNone
	}
}

func hdareaParseTime(s string, ts int64) time.Time {
	if ts > 0 {
		return time.Unix(ts, 0)
	}
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t
		}
	}
	return time.Time{}
}

func (a *HDAreaAdapter) fetchItem(ctx context.Context, config *model.SiteConfig, torrentID string) (*hdareaItem, error) {
	url := fmt.Sprintf("%s/api/v1/torrents/%s", a.base(config), urlQueryEscape(torrentID))
	var it hdareaItem
	if err := a.apiCall(ctx, config, http.MethodGet, url, nil, &it); err != nil {
		return nil, err
	}
	return &it, nil
}

func (a *HDAreaAdapter) GetTorrentDetail(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	it, err := a.fetchItem(ctx, config, torrentID)
	if err != nil {
		return nil, err
	}
	d := &model.TorrentDetail{
		InfoHash:     it.InfoHash,
		Title:        it.Name,
		Subtitle:     it.SmallDescr,
		Description:  it.DescrBBCode,
		Category:     it.CategoryName,
		Source:       it.SourceName,
		Resolution:   it.StandardName,
		Codec:        it.CodecName,
		AudioCodec:   it.AudioCodecName,
		ReleaseGroup: it.TeamName,
		NFO:          it.NFO,
		IMDbID:       it.IMDbID,
		IMDbURL:      it.IMDbURL,
		DoubanID:     it.DoubanID,
		DoubanURL:    it.DoubanURL,
		PosterURL:    it.PosterURL,
		Size:         it.Size,
	}
	if t := hdareaParseTime(it.Added, it.AddedTs); !t.IsZero() {
		d.PublishedAt = &t
	}
	d.MediaInfo = hdareaExtractMI(it.DescrBBCode)
	// forbid_transfer（API 独有禁转声明位——web 面无标记·探针⑨ 反转发现）：
	// 无限时态布尔 → Flags["禁转"]（keepfrds 采集同型·compliance R7 消费）
	if it.ForbidTransfer {
		d.Flags = append(d.Flags, "禁转")
	}
	if err := CheckDetailVacuum(d); err != nil {
		return nil, err
	}
	return d, nil
}

// hdareaExtractMI 从 descr BBCode 提取 MediaInfo 段（API 无独立 MI 字段——
// descr 内 [quote]General[/quote] 惯例块）。
var hdareaMIQuoteRe = regexp.MustCompile(`(?s)\[quote\]\s*(General\s*\n.*?)\[/quote\]`)

func hdareaExtractMI(descr string) string {
	for _, m := range hdareaMIQuoteRe.FindAllStringSubmatch(descr, -1) {
		if strings.Contains(m[1], "Complete name") || strings.Contains(m[1], "Video") {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

func (a *HDAreaAdapter) DownloadTorrent(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	domain := config.BaseURL
	if domain == "" {
		domain = config.Domain
	}
	if err := httpclient.GlobalDownloadLimiter.AcquireWithLimit(domain, config.DownloadHourlyLimit); err != nil {
		return nil, downloadError("下载限流: "+err.Error(), nil)
	}
	body := fmt.Sprintf(`{"id":%s}`, mustAtoiString(torrentID))
	buildReq := func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			a.base(config)+"/api/v1/torrents/genDlToken", strings.NewReader(body))
		if err != nil {
			return nil, err
		}
		setXAPIKey(req, config.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
		return req, nil
	}
	req, err := buildReq()
	if err != nil {
		return nil, networkError("构造 genDlToken 请求失败", err)
	}
	// 短效凭证流程骨架（apihelper）——重签时重建请求（ctx/body 复用）。
	// 实测修正（探针字符串搜索曾掩盖包裹）：genDlToken=统一信封双层
	// {success,code,data:{ok,dl_token,download_url}}——先解信封再取内层。
	return shortTokenDownload(a.doer.Client, req, func(b []byte) (string, error) {
		var env hdareaEnvelope
		if err := json.Unmarshal(b, &env); err != nil {
			return "", err
		}
		var gt struct {
			DownloadURL string `json:"download_url"`
		}
		if err := json.Unmarshal(env.Data, &gt); err != nil {
			return "", err
		}
		return gt.DownloadURL, nil
	})
}

func mustAtoiString(s string) string {
	if _, err := strconv.ParseInt(s, 10, 64); err == nil {
		return s
	}
	return "0"
}

func (a *HDAreaAdapter) DetectDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	it, err := a.fetchItem(ctx, config, torrentID)
	if err != nil {
		return nil, err
	}
	res := &model.DiscountResult{
		Level:      hdareaDiscountLevel(it.DownloadVolumeFactor, it.UploadVolumeFactor, it.SpState),
		Multiplier: it.DownloadVolumeFactor,
	}
	if it.PromotionUntil != "" && it.PromotionUntil != "None" {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", it.PromotionUntil, time.Local); err == nil {
			res.FreeEndAt = &t
		}
	}
	return res, nil
}

func (a *HDAreaAdapter) FetchUserStats(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	var p struct {
		Username     string  `json:"username"`
		Class        int     `json:"class"`
		ClassName    string  `json:"class_name"`
		Uploaded     int64   `json:"uploaded"`
		Downloaded   int64   `json:"downloaded"`
		ShareRatio   float64 `json:"share_ratio"`
		SeedBonus    float64 `json:"seedbonus"`
		SeedingCount int     `json:"seeding_count"`
		SeedingSize  int64   `json:"seeding_size"`
	}
	if err := a.apiCall(ctx, config, http.MethodGet, a.base(config)+"/api/v1/user/profile", nil, &p); err != nil {
		return nil, err
	}
	return &model.UserStatsResult{
		Username:      p.Username,
		UserClass:     p.ClassName,
		UploadBytes:   p.Uploaded,
		DownloadBytes: p.Downloaded,
		Ratio:         p.ShareRatio,
		BonusPoints:   p.SeedBonus,
		SeedingCount:  p.SeedingCount,
		SeedingSize:   p.SeedingSize,
	}, nil
}

// ── 薄实现（好大无 HR/SL 实证·探针定案）──

func (a *HDAreaAdapter) DetectHR(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
	return &model.HRResult{HasHR: false}, nil
}

func (a *HDAreaAdapter) GetBatchSLData(ctx context.Context, config *model.SiteConfig, torrentIDs []string) (map[string]*model.SLData, error) {
	return map[string]*model.SLData{}, nil
}

func (a *HDAreaAdapter) GetPreciseSLData(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	return nil, nil
}

func (a *HDAreaAdapter) GetTorrentInfoHash(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	it, err := a.fetchItem(ctx, config, torrentID)
	if err != nil {
		return "", err
	}
	return it.InfoHash, nil
}

func (a *HDAreaAdapter) SupportsSearchByPiecesHash() bool { return false }

func (a *HDAreaAdapter) VerifyExists(ctx context.Context, config *model.SiteConfig, torrentID string) (bool, error) {
	_, err := a.fetchItem(ctx, config, torrentID)
	if err == nil {
		return true, nil
	}
	if isHTTP404(err) {
		return false, nil
	}
	return false, err
}

func isHTTP404(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "404")
}

// ── 发布（刀1 基础版：表单驱动直写——API 域名与 NP 表单同名·
//     migration90 值集直接复用；深度语义 check/顶替/409 留刀2）──

// HashMatcher 刀3：站方批量 hash 匹配能力接口（好大 API 独有·
// PreChecker 同款类型断言消费——辅种引擎独立信息源）。
type HashMatcher interface {
	BatchMatchHashes(ctx context.Context, config *model.SiteConfig, hashes []string) ([]*HashMatchResult, error)
}

// HashMatchResult 刀3：hash 匹配结果（数学身份·无需标题验证）。
type HashMatchResult struct {
	InfoHash    string // 查询的 40 位 hex
	TorrentID   string // 本站 tid
	Title       string
	Size        int64
	Seeders     int
	DownloadURL string // 短效直链（dl_token）
}

// BatchMatchHashes 刀3 D2+D3：批量 hash 匹配（POST /api/v1/torrents/hashes·
// ≤200/批自动切分）。read scope·独立信息源（与 IYUU 并行·不 fallback）。
func (a *HDAreaAdapter) BatchMatchHashes(ctx context.Context, config *model.SiteConfig, hashes []string) ([]*HashMatchResult, error) {
	if len(hashes) == 0 {
		return nil, nil
	}
	if config.APIKey == "" {
		return nil, &model.AppError{Code: 40001, Message: "好大 hash 匹配需要 API Key"}
	}
	var all []*HashMatchResult
	for start := 0; start < len(hashes); start += 200 {
		end := start + 200
		if end > len(hashes) {
			end = len(hashes)
		}
		batch := hashes[start:end]
		payload, err := json.Marshal(map[string][]string{"hashes": batch})
		if err != nil {
			return nil, parseError("构造 hashes 请求失败: "+err.Error(), err)
		}
		var data struct {
			Items []struct {
				InfoHash    string `json:"info_hash"`
				ID          int    `json:"id"`
				Name        string `json:"name"`
				Size        int64  `json:"size"`
				Seeders     int    `json:"seeders"`
				DownloadURL string `json:"download_url"`
			} `json:"items"`
		}
		if err := a.apiCall(ctx, config, http.MethodPost,
			a.base(config)+"/api/v1/torrents/hashes",
			strings.NewReader(string(payload)), &data); err != nil {
			return nil, err
		}
		for _, it := range data.Items {
			all = append(all, &HashMatchResult{
				InfoHash:    it.InfoHash,
				TorrentID:   strconv.Itoa(it.ID),
				Title:       it.Name,
				Size:        it.Size,
				Seeders:     it.Seeders,
				DownloadURL: it.DownloadURL,
			})
		}
	}
	return all, nil
}

// PreChecker 刀2：站方预查重能力接口（好大 API 独有·类型断言消费）。
type PreChecker interface {
	CheckBeforeUpload(ctx context.Context, config *model.SiteConfig, torrentData []byte) (*PreCheckResult, error)
}

// PreCheckResult 刀2：站方 check 预查重结果（好大 API 独有能力·
// 类型断言消费——不在 14 方法接口内）。
type PreCheckResult struct {
	CanUpload       bool     // 站方判定可上传
	Status          string   // exists_active / clean / banned_duplicate 等
	Reason          string   // 站方文案（透出给用户/日志）
	ExistingID      string   // 已存在种 tid（can_upload=false 时）
	ExistingName    string   // 已存在种标题
	ReplaceableDead []string // 可顶替断种 tid 列表（can_upload=true 时）
	NormalizedHash  string   // 站方清洗重算 info_hash（file 通道）
	IsBanned        bool     // banned_duplicate 判定
}

// CheckBeforeUpload 刀2：站方预查重（POST /api/v1/torrents/check·
// read scope 零副作用·file 通道 multipart——才有 normalized_infohash）。
// 调用方：executor dedup 第三路 + DryRun。
func (a *HDAreaAdapter) CheckBeforeUpload(ctx context.Context, config *model.SiteConfig, torrentData []byte) (*PreCheckResult, error) {
	if len(torrentData) == 0 {
		return nil, &model.AppError{Code: 40001, Message: "种子数据为空"}
	}
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	fw, err := writer.CreateFormFile("file", "check.torrent")
	if err != nil {
		return nil, uploadError("创建 check 表单字段失败", err)
	}
	if _, err := fw.Write(torrentData); err != nil {
		return nil, networkError("写入种子数据失败", err)
	}
	if err := writer.Close(); err != nil {
		return nil, uploadError("关闭 multipart 失败", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.base(config)+"/api/v1/torrents/check", &buf)
	if err != nil {
		return nil, networkError("构造 check 请求失败", err)
	}
	setXAPIKey(req, config.APIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("check 请求失败: "+err.Error(), err)
	}
	defer func() { drainBody(resp) }()
	rb, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmt.Sprintf("check HTTP %d: %s", resp.StatusCode, truncate(string(rb), 200)), nil)
	}
	var env hdareaEnvelope
	if err := json.Unmarshal(rb, &env); err != nil {
		return nil, parseError("check 响应解析失败: "+err.Error(), err)
	}
	if env.Code != 0 {
		return nil, &model.AppError{Code: 40001, Message: fmt.Sprintf("check code=%d: %s", env.Code, env.Message)}
	}
	var data struct {
		CanUpload       bool   `json:"can_upload"`
		Status          string `json:"status"`
		Reason          string `json:"reason"`
		NormalizedHash  string `json:"normalized_infohash"`
		ExistingTorrent *struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"existing_torrent"`
		ReplaceableDead []struct {
			ID int `json:"id"`
		} `json:"replaceable_dead_torrents"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return nil, parseError("check data 解析失败: "+err.Error(), err)
	}
	result := &PreCheckResult{
		CanUpload:       data.CanUpload,
		Status:          data.Status,
		Reason:          data.Reason,
		NormalizedHash:  data.NormalizedHash,
		IsBanned:        strings.Contains(data.Status, "banned") || strings.Contains(data.Reason, "banned"),
	}
	if data.ExistingTorrent != nil {
		result.ExistingID = strconv.Itoa(data.ExistingTorrent.ID)
		result.ExistingName = data.ExistingTorrent.Name
	}
	for _, d := range data.ReplaceableDead {
		result.ReplaceableDead = append(result.ReplaceableDead, strconv.Itoa(d.ID))
	}
	return result, nil
}

func (a *HDAreaAdapter) UploadTorrent(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	if len(req.TorrentData) == 0 {
		return nil, &model.AppError{Code: 40001, Message: "种子文件数据为空"}
	}
	if config.APIKey == "" {
		return nil, &model.AppError{Code: 40001, Message: "好大 API 发布需要 API Key（控制面板→安全设定生成后填入站点配置）"}
	}
	_ = ctx // ctx 由 httpReq 携带

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	fw := newFieldWriter(writer)
	fileWriter, err := writer.CreateFormFile("file", "upload.torrent")
	if err != nil {
		return nil, uploadError("创建表单文件字段失败", err)
	}
	if _, err := fileWriter.Write(req.TorrentData); err != nil {
		return nil, networkError("写入种子数据失败", err)
	}
	if req.Title != "" {
		fw.writeField("name", req.Title)
	}
	if req.Subtitle != "" {
		fw.writeField("small_descr", req.Subtitle)
	}
	if req.Description != "" {
		fw.writeField("descr", req.Description)
	}
	if req.IMDbLink != "" {
		fw.writeField("url", req.IMDbLink)
	}
	if req.DoubanLink != "" {
		fw.writeField("dburl", req.DoubanLink)
	}
	if req.Anonymous {
		fw.writeField("uplver", "yes")
	}
	// 表单驱动槽位：FormFields 键即 API 域名（type/medium_sel/codec_sel/
	// audiocodec_sel/standard_sel/team_sel——migration90 配置直产）
	for k, v := range req.FormFields {
		fw.writeField(k, v)
	}
	if err := writer.Close(); err != nil {
		return nil, uploadError("关闭 multipart 失败", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.base(config)+"/api/v1/torrents/upload", &buf)
	if err != nil {
		return nil, networkError("构造上传请求失败", err)
	}
	setXAPIKey(httpReq, config.APIKey)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")

	resp, err := a.doer.Client.Do(httpReq)
	if err != nil {
		return nil, networkError("上传请求失败: "+err.Error(), err)
	}
	defer func() { drainBody(resp) }()
	rb, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode == http.StatusConflict {
		// 409 重复发种：existing_id 弹性解析（刀2 B1——顶层+data 双查·
		// 站方文档只说「响应体中」·生产首颗自然验证具体位置）
		var raw map[string]interface{}
		_ = json.Unmarshal(rb, &raw)
		exID := ""
		// 先查 data.existing_id
		if data, ok := raw["data"].(map[string]interface{}); ok {
			if v, ok := data["existing_id"]; ok {
				exID = fmt.Sprintf("%v", v)
			}
		}
		// 再查顶层 existing_id
		if exID == "" || exID == "<nil>" {
			if v, ok := raw["existing_id"]; ok {
				exID = fmt.Sprintf("%v", v)
			}
		}
		if exID == "<nil>" {
			exID = ""
		}
		return &model.PublishResponse{
			IsExisting: true,
			ExistingID: exID,
			TargetSite: config.SiteName,
		}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmt.Sprintf("HDArea 上传 HTTP %d: %s", resp.StatusCode, truncate(string(rb), 200)), nil)
	}
	var env hdareaEnvelope
	if err := json.Unmarshal(rb, &env); err != nil {
		return nil, parseError("上传响应解析失败: "+err.Error(), err)
	}
	if env.Code != 0 {
		return nil, &model.AppError{Code: 40001, Message: fmt.Sprintf("HDArea 上传 code=%d: %s", env.Code, env.Message)}
	}
	var data struct {
		TorrentID any    `json:"id"`
		DetailURL string `json:"detail_url"`
	}
	_ = json.Unmarshal(env.Data, &data)
	tid := ""
	switch v := data.TorrentID.(type) {
	case float64:
		tid = strconv.FormatInt(int64(v), 10)
	case string:
		tid = v
	}
	return &model.PublishResponse{
		Success:    true,
		TorrentID:  tid,
		DetailURL:  data.DetailURL,
		IDSource:   "api_response", // JSON 信封直读（302 权威同级的 API 通道）
		TargetSite: config.SiteName,
	}, nil
}

// urlQueryEscape P1 修（回归审核）：真 net/url.QueryEscape——首版只
// 换空格的假实现让中文关键词/含 &/= 字符原样进 URL（获取链中文
// 关键词+§59.192 & 分隔符场景错发）。
func urlQueryEscape(s string) string {
	return url.QueryEscape(strings.TrimSpace(s))
}
