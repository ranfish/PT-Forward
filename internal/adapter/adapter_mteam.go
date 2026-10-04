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
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

var (
	reMTeamTitle      = regexp.MustCompile(`<title>([^<]+)</title>`)
	reMTeamInfoHash   = regexp.MustCompile(`(?i)info_hash.*?<td[^>]*>([a-fA-F0-9]{40})`)
	reMTeamSize       = regexp.MustCompile(`(?i)(?:大小|Size)[^<]*<[^>]*>([^<]+)`)
	reMTeamCategory   = regexp.MustCompile(`(?i)(?:分类|Category)[^<]*<[^>]*>([^<]+)`)
	reMTeamSeeders    = regexp.MustCompile(`(?i)(?:做种|Seeders?)[^<]*<[^>]*>(\d+)`)
	reMTeamLeechers   = regexp.MustCompile(`(?i)(?:下载|Leechers?)[^<]*<[^>]*>(\d+)`)
	reMTeamDetailID   = regexp.MustCompile(`(?:details|detail)\.php\?id=(\d+)`)
	reMTeamErrorClass = regexp.MustCompile(`class="error"[^>]*>([^<]+)`)
)

const mteamUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

type flexInt int64

func (f *flexInt) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		*f = 0
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			*f = 0
			return nil
		}
		*f = flexInt(n)
		return nil
	}
	var n int64
	if err := json.Unmarshal(data, &n); err != nil {
		*f = 0
		return nil
	}
	*f = flexInt(n)
	return nil
}

type MTeamAdapter struct {
	*GenericAdapter
	doer   *HTTPDoer
	logger *zap.Logger
}

func NewMTeamAdapter(doer *HTTPDoer, logger *zap.Logger) *MTeamAdapter {
	return &MTeamAdapter{
		GenericAdapter: NewGenericAdapter("mteam", doer, logger),
		doer:           doer,
		logger:         logger,
	}
}

func (a *MTeamAdapter) Framework() string { return "mteam" }

func (a *MTeamAdapter) SearchTorrents(ctx context.Context, config *model.SiteConfig, keyword string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
	if config.APIKey != "" {
		return a.searchViaAPI(ctx, config, keyword)
	}
	return a.GenericAdapter.SearchTorrents(ctx, config, keyword, opts)
}

// searchViaAPI §59.222: 双模式搜索（normal + adult）——成人区不再被搜索层
// 一刀切（3dsvr 六连案：mode:"normal" 硬编码致全站 97 站 0 结果）。
// 成人内容限制移至发布业务层（SelectFetchSite/publishing 过滤），
// 孤儿恢复/辅种不受限。adult 结果带 Adult 标记供下游消费。
func (a *MTeamAdapter) searchViaAPI(ctx context.Context, config *model.SiteConfig, keyword string) ([]*model.SeedingSearchResult, error) {
	u := resolveBaseURL(config) + "/api/torrent/search"

	var all []*model.SeedingSearchResult
	for _, mode := range []string{"normal", "adult"} {
		payload, _ := json.Marshal(map[string]interface{}{
			"keyword":    keyword,
			"mode":       mode,
			"pageNumber": 1,
			"pageSize":   20,
		})

		req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(payload))
		if err != nil {
			return nil, searchError("构造搜索请求失败", err)
		}
		req.Header.Set("Content-Type", "application/json")
		a.setAPIHeaders(req, config.APIKey)

		resp, err := a.doer.Client.Do(req)
		if err != nil {
			if mode == "normal" {
				return nil, searchError("搜索请求失败", err)
			}
			continue // adult 轮失败不阻断——降级 normal-only
		}

		body, readErr := readBody(resp)
		drainBody(resp)
		if readErr != nil && mode == "normal" {
			return nil, readErr
		}
		if readErr != nil {
			continue
		}

		var result struct {
			Code json.Number `json:"code"`
			Data struct {
				Data []struct {
					ID     string  `json:"id"`
					Name   string  `json:"name"`
					Size   flexInt `json:"size"`
					Status struct {
						Seeders  flexInt `json:"seeders"`
						Leechers flexInt `json:"leechers"`
						Discount string  `json:"discount"`
					} `json:"status"`
				} `json:"data"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			if mode == "normal" {
				return nil, parseError("解析搜索结果失败", err)
			}
			continue
		}

		// §59.224: API 业务错误码（code≠0）——返回错误进入退避重试链
		// （savr 六连案：并发批量期 code=4 瞬时限流，单独重试即恢复——
		//  此前返回空结果=合法空页，§59.219 空页重试偶尔能兜住但不可靠）。
		// message 提取供日志+错误消息。
		if result.Code.String() != "0" {
			var errBody struct {
				Message string `json:"message"`
			}
			_ = json.Unmarshal(body, &errBody)
			if a.logger != nil {
				a.logger.Warn("mteam search API error",
					zap.String("mode", mode),
					zap.String("keyword", keyword),
					zap.String("code", result.Code.String()),
					zap.String("message", errBody.Message))
			}
			if mode == "normal" {
				return nil, &model.AppError{Code: 15002, Message: fmt.Sprintf("MTeam API error code=%s: %s", result.Code.String(), errBody.Message)}
			}
			// adult 轮 code≠0：记日志不阻断（降级 normal-only 结果）
			continue
		}
		if a.logger != nil {
			a.logger.Debug("mteam search resp",
				zap.String("mode", mode),
				zap.String("keyword", keyword),
				zap.Int("body_bytes", len(body)),
				zap.Int("parsed", len(result.Data.Data)))
		}

		for _, item := range result.Data.Data {
			r := &model.SeedingSearchResult{
				TorrentID: item.ID,
				Title:     item.Name,
				Size:      int64(item.Size),
				Seeders:   int(item.Status.Seeders),
				Leechers:  int(item.Status.Leechers),
			}
			if mode == "adult" {
				r.Adult = true
			}
			all = append(all, r)
		}
	}
	return all, nil
}

func (a *MTeamAdapter) setAPIHeaders(req *http.Request, apiKey string) {
	req.Header.Set("User-Agent", mteamUserAgent)
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("Accept", "application/json")
}

func (a *MTeamAdapter) DownloadTorrent(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	domain := config.BaseURL
	if domain == "" {
		domain = config.Domain
	}
	if err := httpclient.GlobalDownloadLimiter.AcquireWithLimit(domain, config.DownloadHourlyLimit); err != nil {
		return nil, downloadError("下载限流: "+err.Error(), nil)
	}

	if config.APIKey != "" {
		return a.downloadViaAPI(ctx, config, torrentID)
	}
	return a.downloadViaWeb(ctx, config, torrentID)
}

func (a *MTeamAdapter) downloadViaAPI(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	tokenURL := resolveBaseURL(config) + "/api/torrent/genDlToken"
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader("id="+torrentID))
	if err != nil {
		return nil, networkError("构造 genDlToken 请求失败", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	a.setAPIHeaders(req, config.APIKey)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("genDlToken 请求失败", err)
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, notFoundError("种子不存在或已被删除")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("genDlToken HTTP %d", resp.StatusCode), nil)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return nil, networkError("读取 genDlToken 响应失败", err)
	}
	var tokenResp struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, parseError("解析 genDlToken 响应失败", err)
	}
	if tokenResp.Data == "" {
		return nil, downloadError("genDlToken 返回空下载链接", nil)
	}

	dlReq, err := http.NewRequestWithContext(ctx, "GET", tokenResp.Data, nil)
	if err != nil {
		return nil, networkError("构造下载请求失败", err)
	}
	a.setAPIHeaders(dlReq, config.APIKey)

	dlResp, err := a.doer.Client.Do(dlReq)
	if err != nil {
		return nil, downloadError("下载种子失败", err)
	}
	defer func() { drainBody(dlResp) }()

	if dlResp.StatusCode == http.StatusNotFound {
		return nil, notFoundError("种子不存在或已被删除")
	}
	if dlResp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("下载种子 HTTP %d", dlResp.StatusCode), nil)
	}

	data, err := io.ReadAll(io.LimitReader(dlResp.Body, 50*1024*1024))
	if err != nil {
		return nil, downloadError("读取种子数据失败", err)
	}
	if len(data) == 0 {
		return nil, &model.AppError{Code: 15001, Message: "种子数据为空"}
	}
	return data, nil
}

func (a *MTeamAdapter) downloadViaWeb(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	u := resolveBaseURL(config) + "/download.php?id=" + torrentID
	if config.Passkey != "" {
		u += "&passkey=" + config.Passkey
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, downloadError("下载失败", err)
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, notFoundError("种子不存在或已被删除")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("HTTP %d", resp.StatusCode), nil)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return nil, downloadError("读取种子数据失败", err)
	}
	if len(data) == 0 {
		return nil, &model.AppError{Code: 15001, Message: "种子数据为空"}
	}
	return data, nil
}

func (a *MTeamAdapter) GetTorrentDetail(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	if config.APIKey != "" {
		return a.detailViaAPI(ctx, config, torrentID)
	}
	return a.detailViaWeb(ctx, config, torrentID)
}

// mteamAdultCategories 馒头成人区 category 编码（mode=adult）。
// §59.317 修订：按 categoryList 权威树补全——AV(无码)含 AV(网站)/0Day(436) 与
// AV(Gay)/HD(440)；主类 ID（115 AV有码/120 AV无码/445 IV/446 H-ACG）保险入集。
var mteamAdultCategories = map[string]bool{
	"410": true, "411": true, "412": true, "413": true,
	"424": true, "425": true, "426": true,
	"429": true, "430": true, "431": true, "432": true, "433": true,
	"436": true, "437": true, "440": true,
	"115": true, "120": true, "445": true, "446": true,
}

// mteamCategoryMap MTeam API category ID → 标准 canonical（§59.317 权威重建）。
// 数据源：/api/torrent/categoryList 实测两级树（2026-10-03 经系统解密链拉取）。
// 【历史教训】v0.0.255 版误用 sites.json 发布表单 ID 快照——馒头 2024-03 重建
// 分类树为两级结构（主类>媒介子类）后 ID 全部错位：421 实为电影/BluRay 而旧表
// 译 category.cartoon（243 全库 456 行 HDT 原盘电影全标"动漫"案）。
// 映射原则：①主类级 canonical（媒介子类 BluRay/HD 与 medium 资产重叠，不入
// category）②键对齐 dict canonical 键族（documentary 单数/comic/education 等）
// ③主类 ID（100/105/110/447/449/450）一并映射（防御 detail 返回主类形态）。
var mteamCategoryMap = map[string]string{
	// 电影（100 主类 + SD/HD/DVDiSo/BluRay/Remux 子类）
	"100": "category.movie",
	"401": "category.movie",
	"419": "category.movie",
	"420": "category.movie",
	"421": "category.movie",
	"439": "category.movie",
	// 影剧/综艺（105 主类 + HD/SD/DVDiSo/BluRay 子类）
	"105": "category.tv_series",
	"402": "category.tv_series",
	"403": "category.tv_series",
	"435": "category.tv_series",
	"438": "category.tv_series",
	// 紀錄（444 主类 + 纪录子类）
	"444": "category.documentary",
	"404": "category.documentary",
	// 動漫（449 主类 + 动画/动画BluRay 子类——453 为 2026-07 新增）
	"449": "category.animation",
	"405": "category.animation",
	"453": "category.animation",
	// Music（110 主类：演唱 406→演唱会；Music(无损) 434→无损音乐）
	"110": "category.music",
	"406": "category.concert",
	"434": "category.lossless_music",
	// 遊戲（447 主类 + PC游戏/TV遊戲）
	"447": "category.game",
	"423": "category.game",
	"448": "category.game",
	// 其他（450 主类）
	"450": "category.other",
	"407": "category.sports",
	"409": "category.other",
	"422": "category.software",
	"427": "category.ebook",
	"442": "category.audiobook",
	"451": "category.education", // 2025-05 新增：教育影片
}

// normalizeMTeamCategoryID §59.317 P0: 馒头 API 原始分类 ID → 标准 canonical（静态表）。
// 成人区命中 → category.adult；map 命中 → canonical；未收录 → ""（调用方回退）。
// detail 与 RSS（FetchItemsByAPI）两链统一走此口径；动态缓存（§59.317 P3）
// 未命中时回退本表。
func normalizeMTeamCategoryID(raw string) string {
	if raw == "" {
		return ""
	}
	if mteamAdultCategories[raw] {
		return "category.adult"
	}
	return mteamCategoryMap[raw]
}

// ===== §59.317 P3: categoryList 动态映射（防腐——站方持续加分类 451/453 实证）=====
//
// 架构：动态缓存（TTL 24h，singleflight 式锁合并并发首拉）→ 静态表兜底（本文件
// P0 权威快照）→ 双 miss Warn 告警。映射生成走"主类继承 + 叶子例外"两张稳定
// 小表——新叶子挂已知主类下自动继承正确 canonical（防腐机理），仅主类重排
// （2024-03 级改版）才需人工更新，且彼时 miss 告警必响。

const (
	mteamCategoryTTL          = 24 * time.Hour
	mteamCategoryRetryBackoff = 10 * time.Minute // 拉取失败退避（防风暴）
	mteamCategoryPullTimeout  = 5 * time.Second
)

// 主类 ID → canonical（7 主类，权威树稳定层——叶子默认继承）
var mteamParentCanonical = map[string]string{
	"100": "category.movie",
	"105": "category.tv_series",
	"444": "category.documentary",
	"449": "category.animation",
	"110": "category.music",
	"447": "category.game",
	"450": "category.other",
}

// 叶子例外（与主类 canonical 不同的叶子——主类 other 下各专类 + Music 特化）
var mteamLeafOverride = map[string]string{
	"406": "category.concert",
	"434": "category.lossless_music",
	"442": "category.audiobook",
	"427": "category.ebook",
	"422": "category.software",
	"407": "category.sports",
	"451": "category.education",
}

type mteamCategoryCache struct {
	mu        sync.RWMutex
	idMap     map[string]string // ID → canonical（含主类自身）
	adult     map[string]bool   // 动态成人集（与静态集并集使用）
	fetchedAt time.Time
	lastErr   time.Time
}

// 进程级单例（馒头站点配置进程内唯一；缓存不含凭证）
var mteamCatCache = &mteamCategoryCache{}

func (c *mteamCategoryCache) reset() {
	c.mu.Lock()
	c.idMap = nil
	c.adult = nil
	c.fetchedAt = time.Time{}
	c.lastErr = time.Time{}
	c.mu.Unlock()
}

// resolveCategory 动态→静态两级解析（detail/RSS 两链入口）。
func (a *MTeamAdapter) resolveCategory(ctx context.Context, config *model.SiteConfig, raw string) string {
	if raw == "" {
		return ""
	}
	a.refreshCategoryCache(ctx, config)
	mteamCatCache.mu.RLock()
	isAdult, adultOK := mteamCatCache.adult[raw]
	canonical, ok := mteamCatCache.idMap[raw]
	mteamCatCache.mu.RUnlock()
	if adultOK && isAdult {
		return "category.adult"
	}
	if ok {
		return canonical
	}
	// 静态兜底（动态未加载/未收录）
	return normalizeMTeamCategoryID(raw)
}

// refreshCategoryCache TTL 内零开销（RLock 快路径）；过期/未加载时锁内单飞拉取，
// 失败退避后不重试风暴。
func (a *MTeamAdapter) refreshCategoryCache(ctx context.Context, config *model.SiteConfig) {
	c := mteamCatCache
	c.mu.RLock()
	fresh := time.Since(c.fetchedAt) < mteamCategoryTTL
	backoff := time.Since(c.lastErr) < mteamCategoryRetryBackoff
	c.mu.RUnlock()
	if fresh || backoff {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.fetchedAt) < mteamCategoryTTL || time.Since(c.lastErr) < mteamCategoryRetryBackoff {
		return // 双检（并发首拉合并）
	}
	if err := a.fetchCategoryList(ctx, config); err != nil {
		c.lastErr = time.Now()
		if a.logger != nil {
			a.logger.Warn("mteam categoryList fetch failed, fallback static", zap.Error(err))
		}
		return
	}
	c.fetchedAt = time.Now()
	if a.logger != nil {
		a.logger.Info("mteam category cache refreshed",
			zap.Int("ids", len(c.idMap)), zap.Int("adult", len(c.adult)))
	}
}

// fetchCategoryList 拉取并生成映射（锁内调用）。
// 生成规则：叶子 = 例外表 ?? 主类继承；主类自身直接映射；归一失败的跳过
// （保持 miss 告警闭环——新语义分类需人工定案后入例外表）。
func (a *MTeamAdapter) fetchCategoryList(ctx context.Context, config *model.SiteConfig) error {
	pullCtx, cancel := context.WithTimeout(ctx, mteamCategoryPullTimeout)
	defer cancel()
	u := resolveBaseURL(config) + "/api/torrent/categoryList"
	req, err := http.NewRequestWithContext(pullCtx, http.MethodPost, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	a.setAPIHeaders(req, config.APIKey)
	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return err
	}
	defer func() { drainBody(resp) }()
	body, err := readBody(resp)
	if err != nil {
		return err
	}
	var result struct {
		Code json.Number `json:"code"`
		Data struct {
			List []struct {
				ID     string  `json:"id"`
				Parent *string `json:"parent"`
			} `json:"list"`
			Adult []string `json:"adult"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return parseError("解析 categoryList 失败", err)
	}
	if result.Code.String() != "0" {
		return parseError(fmt.Sprintf("categoryList API 失败: code=%s", result.Code.String()), nil)
	}
	adult := make(map[string]bool, len(result.Data.Adult))
	for _, id := range result.Data.Adult {
		adult[id] = true
	}
	idMap := make(map[string]string, len(result.Data.List))
	for _, item := range result.Data.List {
		if adult[item.ID] {
			continue // 成人叶子走 adult 集
		}
		if item.Parent == nil {
			// 主类自身
			if canonical, ok := mteamParentCanonical[item.ID]; ok {
				idMap[item.ID] = canonical
			}
			continue
		}
		if canonical, ok := mteamLeafOverride[item.ID]; ok {
			idMap[item.ID] = canonical
			continue
		}
		if canonical, ok := mteamParentCanonical[*item.Parent]; ok {
			idMap[item.ID] = canonical // 主类继承（新叶子防腐路径）
		}
		// 主类也未知（站方加新主类/重排）→ 跳过，miss 告警闭环
	}
	c := mteamCatCache
	c.idMap = idMap
	c.adult = adult
	return nil
}

func (a *MTeamAdapter) detailViaAPI(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	u := resolveBaseURL(config) + "/api/torrent/detail"
	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader("id="+torrentID))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	a.setAPIHeaders(req, config.APIKey)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code    json.Number `json:"code"`
		Message string      `json:"message"`
		Data    struct {
			Name        string   `json:"name"`
			SmallDescr  string   `json:"smallDescr"`
			Description string   `json:"description"`
			Size        flexInt  `json:"size"`
			InfoHash    string   `json:"infoHash"`
			Category    string   `json:"category"`
			Source      string   `json:"source"`
			Standard    string   `json:"standard"`
			VideoCodec  string   `json:"videoCodec"`
			AudioCodec  string   `json:"audioCodec"`
			Team        string   `json:"team"`
			Imdb        string   `json:"imdb"`
			Douban      string   `json:"douban"`
			MediaInfo   string   `json:"mediaInfo"`
			Tags        []string `json:"tags"`
			Status      struct {
				Discount        string  `json:"discount"`
				DiscountEndTime string  `json:"discountEndTime"`
				Seeders         flexInt `json:"seeders"`
				Leechers        flexInt `json:"leechers"`
				HR              bool    `json:"hr"`
			} `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, parseError("解析 API 响应失败", err)
	}
	// §59.60 B1: code≠0（无效 tid/限流/域名熔断）必须报错——原实现直接解空 data
	// 构造全空 detail 伪成功（243 实测空行落库且降级链不触发）。照抄 search :577 模式。
	if result.Code.String() != "0" {
		return nil, parseError(fmt.Sprintf("API 详情失败: %s", result.Message), nil)
	}

	d := result.Data
	// §59.317: detail 响应可观测——raw category 原始值落日志（对齐 §59.223 search
	// req/resp Debug 模式；243 全库 cartoon 案因 detail 无日志成盲区，原始值不可考）。
	if a.logger != nil {
		a.logger.Debug("mteam detail resp",
			zap.String("tid", torrentID),
			zap.String("raw_category", d.Category),
			zap.Int("body_bytes", len(body)))
	}
	detail := &model.TorrentDetail{
		Title:        d.Name,
		Subtitle:     d.SmallDescr,
		Description:  d.Description,
		Size:         int64(d.Size),
		InfoHash:     strings.ToLower(d.InfoHash),
		Category:     d.Category,
		Source:       d.Source,
		Resolution:   d.Standard,
		Codec:        d.VideoCodec,
		AudioCodec:   d.AudioCodec,
		ReleaseGroup: d.Team,
		MediaInfo:    d.MediaInfo,
		IMDbID:       d.Imdb,
		DoubanID:     d.Douban,
		Tags:         d.Tags,
	}
	if d.Imdb != "" {
		detail.IMDbURL = buildIMDbURL(d.Imdb) // §59.297 双形态
	}
	if d.Douban != "" {
		detail.DoubanURL = buildDoubanURL(d.Douban) // §59.297 双形态
	}
	if d.Description != "" {
		detail.Screenshots = extractScreenshotsFromBBCode(d.Description)
		if len(detail.Screenshots) > 0 {
			detail.PosterURL = detail.Screenshots[0]
			detail.Screenshots = detail.Screenshots[1:]
		}
	}
	// §59.317 P0+P3: 分类归一——动态缓存（categoryList）→ 静态表两级解析。
	// 成人区命中：flags 标记 adult（§56.40 合规增强）+ category.adult。
	if nc := a.resolveCategory(ctx, config, detail.Category); nc != "" {
		detail.Category = nc
		if nc == "category.adult" {
			detail.Flags = []string{"adult"}
			return detail, nil
		}
	} else {
		// map miss：站方新增分类早知道（Warn 可观测——本案若有此告警 09-30
		// 当天即暴露，无需等用户肉眼发现错标）
		if a.logger != nil && detail.Category != "" {
			a.logger.Warn("mteam category id unmapped",
				zap.String("tid", torrentID),
				zap.String("raw_category", detail.Category))
		}
		detail.Category = NormalizeCategory(detail.Category)
	}
	// 从结构化字段提取禁转标记（§56.37 合规修复）
	detail.Flags = extractFlagsFromStructured(detail.Title, detail.Subtitle, detail.Tags)
	return detail, nil
}

func (a *MTeamAdapter) detailViaWeb(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	u := resolveBaseURL(config) + "/details.php?id=" + torrentID
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := string(body)

	detail := &model.TorrentDetail{}
	if m := reMTeamTitle.FindStringSubmatch(html); len(m) > 1 {
		detail.Title = strings.TrimSpace(m[1])
	}
	if m := reMTeamInfoHash.FindStringSubmatch(html); len(m) > 1 {
		detail.InfoHash = strings.ToLower(m[1])
	}
	if m := reMTeamSize.FindStringSubmatch(html); len(m) > 1 {
		detail.Size = parseSizeStr(m[1])
	}
	if m := reMTeamCategory.FindStringSubmatch(html); len(m) > 1 {
		detail.Category = strings.TrimSpace(m[1])
	}
	detail.Tags = extractTags(html)

	return detail, nil
}

func (a *MTeamAdapter) DetectDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	if config.APIKey != "" {
		return a.discountViaAPI(ctx, config, torrentID)
	}
	return a.discountViaWeb(ctx, config, torrentID)
}

func (a *MTeamAdapter) discountViaAPI(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	u := resolveBaseURL(config) + "/api/torrent/detail"
	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader("id="+torrentID))
	if err != nil {
		return nil, networkError("构造优惠检测请求失败", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	a.setAPIHeaders(req, config.APIKey)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("优惠检测请求失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	var rawResult struct {
		Data struct {
			Status struct {
				Discount        string `json:"discount"`
				DiscountEndTime string `json:"discountEndTime"`
			} `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &rawResult); err != nil {
		return nil, parseError("解析优惠检测结果失败", err)
	}

	var dr *model.DiscountResult
	switch strings.ToUpper(rawResult.Data.Status.Discount) {
	case "FREE":
		dr = &model.DiscountResult{Level: model.DiscountFree}
	case "_2X_FREE", "FREE_2XUP", "TWOFREE":
		dr = &model.DiscountResult{Level: model.Discount2xFree, Multiplier: 2.0}
	case "_2X", "2XUP":
		dr = &model.DiscountResult{Level: model.Discount2xUp, Multiplier: 2.0}
	case "_2X_PERCENT_50":
		dr = &model.DiscountResult{Level: model.Discount2x50, Multiplier: 2.0}
	case "PERCENT_50":
		dr = &model.DiscountResult{Level: model.DiscountPercent50}
	case "PERCENT_70", "_2X_PERCENT_70":
		dr = &model.DiscountResult{Level: model.DiscountPercent70}
	case "PERCENT_30":
		dr = &model.DiscountResult{Level: model.DiscountPercent30}
	case "NORMAL":
		dr = &model.DiscountResult{Level: model.DiscountNone}
	default:
		dr = &model.DiscountResult{Level: model.DiscountNone}
	}
	if dr.Level != model.DiscountNone && rawResult.Data.Status.DiscountEndTime != "" {
		if t, err := time.Parse(time.RFC3339, rawResult.Data.Status.DiscountEndTime); err == nil {
			dr.FreeEndAt = &t
		}
	}
	return dr, nil
}

// FetchItemsByAPI queries the /api/torrent/search endpoint directly to bypass
// the page-size + sign-bound RSS feed. Used when site.framework == "mteam" and
// APIKey is configured. Categories/teams/pageSize/discounts are parsed from
// the original RSS URL's query string (so users keep using the same URL field).
//
// URL query params:
//
//	categories=401,419,...     -> API filter
//	teams=9,44                 -> API filter
//	pageSize=50                -> API request size (default 50, max 100)
//	discounts=FREE,_2X_FREE    -> client-side filter (default: FREE,_2X_FREE,FREE_2XUP,TWOFREE)
//
// Returns []*RSSTorrentEvent already populated with discount info (no need to
// run DetectDiscount again).
func (a *MTeamAdapter) FetchItemsByAPI(ctx context.Context, config *model.SiteConfig, feedURL, siteName string) ([]*model.RSSTorrentEvent, error) {
	if config.APIKey == "" {
		return nil, parseError("FetchItemsByAPI: API key 未配置", nil)
	}

	categories, teams, pageSize, wantDiscounts := parseMTeamFeedParams(feedURL)
	payload := map[string]interface{}{
		"mode":       "normal",
		"pageNumber": 1,
		"pageSize":   pageSize,
	}
	if len(categories) > 0 {
		payload["categories"] = categories
	}
	if len(teams) > 0 {
		payload["teams"] = teams
	}
	body, _ := json.Marshal(payload)

	u := resolveBaseURL(config) + "/api/torrent/search"
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, networkError("构造 API 搜索请求失败", err)
	}
	req.Header.Set("Content-Type", "application/json")
	a.setAPIHeaders(req, config.APIKey)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("API 搜索请求失败", err)
	}
	defer func() { drainBody(resp) }()

	respBody, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	var result struct {
		Code json.Number `json:"code"`
		Data struct {
			Data []mTeamSearchItem `json:"data"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, parseError("解析 API 搜索结果失败", err)
	}
	if result.Code.String() != "0" {
		return nil, parseError(fmt.Sprintf("API 搜索失败: %s", result.Message), nil)
	}

	events := make([]*model.RSSTorrentEvent, 0, len(result.Data.Data))
	for _, item := range result.Data.Data {
		dr := mTeamDiscountToResult(item.Status.Discount, item.Status.DiscountEndTime)
		if dr.Level == model.DiscountNone {
			continue
		}
		if !wantDiscounts[strings.ToUpper(item.Status.Discount)] {
			continue
		}
		ev := &model.RSSTorrentEvent{
			SourceRSS:     feedURL,
			SiteName:      siteName,
			TorrentID:     item.ID,
			Title:         item.Name,
			Size:          int64(item.Size),
			InfoHash:      "",
			DownloadURL:   resolveBaseURL(config) + "/download.php?id=" + item.ID,
			IsFree:        dr.Level == model.DiscountFree || dr.Level == model.Discount2xFree || dr.Level == model.Discount2x50,
			DiscountLevel: dr.Level,
			FreeEndAt:     dr.FreeEndAt,
			// §59.317: RSS 链同口径归一——原透传原始 ID（如 "421"）落
			// rss_torrent_seen.source_category，消费端（normalizeCategorySimple/
			// compliance lookupSourceCategory）均无法归一（回归审查遗漏 A）。
			Category: a.resolveCategory(ctx, config, item.Category),
			Metadata: map[string]any{},
		}
		events = append(events, ev)
	}
	return events, nil
}

// mTeamSearchItem mirrors the search API item shape (subset we care about).
type mTeamSearchItem struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Size     flexInt `json:"size"`
	Status   struct {
		Discount        string `json:"discount"`
		DiscountEndTime string `json:"discountEndTime"`
	} `json:"status"`
}

// parseMTeamFeedParams extracts categories/teams/pageSize/discounts from the
// RSS URL's query string. Returns parallel slices + a discount lookup set.
func parseMTeamFeedParams(feedURL string) (categories, teams []int, pageSize int, discounts map[string]bool) {
	discounts = defaultMTeamDiscounts()
	pageSize = 50
	if feedURL == "" {
		return
	}
	parsed, err := url.Parse(feedURL)
	if err != nil {
		return
	}
	q := parsed.Query()
	if v := q.Get("categories"); v != "" {
		categories = parseIntList(v)
	}
	if v := q.Get("teams"); v != "" {
		teams = parseIntList(v)
	}
	if v := q.Get("pageSize"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			pageSize = n
		}
	}
	if v := q.Get("discounts"); v != "" {
		custom := make(map[string]bool)
		for _, d := range strings.Split(v, ",") {
			d = strings.TrimSpace(strings.ToUpper(d))
			if d != "" {
				custom[d] = true
			}
		}
		if len(custom) > 0 {
			discounts = custom
		}
	}
	return
}

func defaultMTeamDiscounts() map[string]bool {
	return map[string]bool{
		"FREE":           true,
		"_2X_FREE":       true,
		"FREE_2XUP":      true,
		"TWOFREE":        true,
		"_2X":            true,
		"2XUP":           true,
		"_2X_PERCENT_50": true,
	}
}

func parseIntList(s string) []int {
	var out []int
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if n, err := strconv.Atoi(p); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// mTeamDiscountToResult converts API discount strings to DiscountResult.
// Mirrors discountViaAPI's switch but operates on a raw string.
func mTeamDiscountToResult(discount, endTime string) *model.DiscountResult {
	var dr *model.DiscountResult
	switch strings.ToUpper(discount) {
	case "FREE":
		dr = &model.DiscountResult{Level: model.DiscountFree}
	case "_2X_FREE", "FREE_2XUP", "TWOFREE":
		dr = &model.DiscountResult{Level: model.Discount2xFree, Multiplier: 2.0}
	case "_2X", "2XUP":
		dr = &model.DiscountResult{Level: model.Discount2xUp, Multiplier: 2.0}
	case "_2X_PERCENT_50":
		dr = &model.DiscountResult{Level: model.Discount2x50, Multiplier: 2.0}
	case "PERCENT_50":
		dr = &model.DiscountResult{Level: model.DiscountPercent50}
	case "PERCENT_70", "_2X_PERCENT_70":
		dr = &model.DiscountResult{Level: model.DiscountPercent70}
	case "PERCENT_30":
		dr = &model.DiscountResult{Level: model.DiscountPercent30}
	case "NORMAL", "":
		dr = &model.DiscountResult{Level: model.DiscountNone}
	default:
		dr = &model.DiscountResult{Level: model.DiscountNone}
	}
	if dr.Level != model.DiscountNone && endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			dr.FreeEndAt = &t
		}
	}
	return dr
}

func (a *MTeamAdapter) discountViaWeb(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	u := resolveBaseURL(config) + "/details.php?id=" + torrentID
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造优惠检测请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("优惠检测请求失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := strings.ToLower(string(body))

	if strings.Contains(html, "pro_free2up") {
		return &model.DiscountResult{Level: model.Discount2xFree, Multiplier: 2.0}, nil
	}
	if strings.Contains(html, "pro_2up") {
		return &model.DiscountResult{Level: model.Discount2xUp, Multiplier: 2.0}, nil
	}
	if strings.Contains(html, "pro_free") || strings.Contains(html, "免费") {
		return &model.DiscountResult{Level: model.DiscountFree}, nil
	}
	if strings.Contains(html, "pro_50p") {
		return &model.DiscountResult{Level: model.DiscountPercent50}, nil
	}

	return &model.DiscountResult{Level: model.DiscountNone}, nil
}

func (a *MTeamAdapter) DetectHRAndDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, *model.DiscountResult, error) {
	if config.APIKey != "" {
		return a.detectHRAndDiscountViaAPI(ctx, config, torrentID)
	}
	hr, err := a.DetectHR(ctx, config, torrentID)
	if err != nil {
		return nil, nil, err
	}
	disc, err := a.DetectDiscount(ctx, config, torrentID)
	if err != nil {
		return hr, nil, err
	}
	return hr, disc, nil
}

func (a *MTeamAdapter) detectHRAndDiscountViaAPI(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, *model.DiscountResult, error) {
	u := resolveBaseURL(config) + "/api/torrent/detail"
	a.logger.Debug("mteam combined detect: sending request",
		zap.String("url", u),
		zap.String("torrent", torrentID),
		zap.Bool("hasAPIKey", config.APIKey != ""))
	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader("id="+torrentID))
	if err != nil {
		return nil, nil, networkError("构造合并检测请求失败", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	a.setAPIHeaders(req, config.APIKey)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, nil, networkError("合并检测请求失败", err)
	}
	defer func() { drainBody(resp) }()
	a.logger.Debug("mteam combined detect: got response",
		zap.String("torrent", torrentID),
		zap.Int("status", resp.StatusCode))

	body, err := readBody(resp)
	if err != nil {
		return nil, nil, err
	}
	var rawResult struct {
		Data struct {
			Status struct {
				Discount        string `json:"discount"`
				DiscountEndTime string `json:"discountEndTime"`
				HR              bool   `json:"hr"`
			} `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &rawResult); err != nil {
		return nil, nil, parseError("解析合并检测结果失败", err)
	}

	hrResult := &model.HRResult{HasHR: rawResult.Data.Status.HR}
	if hrResult.HasHR {
		hrResult.SeedTimeH = config.HR.SeedTimeH()
	}

	var dr *model.DiscountResult
	switch strings.ToUpper(rawResult.Data.Status.Discount) {
	case "FREE":
		dr = &model.DiscountResult{Level: model.DiscountFree}
	case "_2X_FREE", "FREE_2XUP", "TWOFREE":
		dr = &model.DiscountResult{Level: model.Discount2xFree, Multiplier: 2.0}
	case "_2X", "2XUP":
		dr = &model.DiscountResult{Level: model.Discount2xUp, Multiplier: 2.0}
	case "_2X_PERCENT_50":
		dr = &model.DiscountResult{Level: model.Discount2x50, Multiplier: 2.0}
	case "PERCENT_50":
		dr = &model.DiscountResult{Level: model.DiscountPercent50}
	case "PERCENT_70", "_2X_PERCENT_70":
		dr = &model.DiscountResult{Level: model.DiscountPercent70}
	case "PERCENT_30":
		dr = &model.DiscountResult{Level: model.DiscountPercent30}
	case "NORMAL":
		dr = &model.DiscountResult{Level: model.DiscountNone}
	default:
		dr = &model.DiscountResult{Level: model.DiscountNone}
	}
	if dr.Level != model.DiscountNone && rawResult.Data.Status.DiscountEndTime != "" {
		if t, err := time.Parse(time.RFC3339, rawResult.Data.Status.DiscountEndTime); err == nil {
			dr.FreeEndAt = &t
		}
	}
	return hrResult, dr, nil
}

func (a *MTeamAdapter) DetectHR(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
	if config.APIKey != "" {
		u := resolveBaseURL(config) + "/api/torrent/detail"
		req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader("id="+torrentID))
		if err != nil {
			return nil, networkError("构造 HR 检测请求失败", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		a.setAPIHeaders(req, config.APIKey)

		resp, err := a.doer.Client.Do(req)
		if err != nil {
			return nil, networkError("HR 检测请求失败", err)
		}
		defer func() { drainBody(resp) }()

		body, err := readBody(resp)
		if err != nil {
			return nil, err
		}
		var rawResult struct {
			Data struct {
				Status struct {
					HR bool `json:"hr"`
				} `json:"status"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &rawResult); err != nil {
			return nil, parseError("解析 HR 检测结果失败", err)
		}

		if rawResult.Data.Status.HR {
			return &model.HRResult{HasHR: true, SeedTimeH: config.HR.SeedTimeH()}, nil
		}
		return &model.HRResult{HasHR: false}, nil
	}

	u := resolveBaseURL(config) + "/details.php?id=" + torrentID
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造 HR 检测请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("HR 检测请求失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := strings.ToLower(string(body))

	hasHR := detectHRFromHTML(html)

	result := &model.HRResult{HasHR: hasHR}
	if hasHR {
		result.SeedTimeH = config.HR.SeedTimeH()
	}
	return result, nil
}

func (a *MTeamAdapter) GetBatchSLData(ctx context.Context, config *model.SiteConfig, torrentIDs []string) (map[string]*model.SLData, error) {
	result := make(map[string]*model.SLData, len(torrentIDs))
	for _, id := range torrentIDs {
		sl, err := a.GetPreciseSLData(ctx, config, id)
		if err != nil {
			a.logger.Warn("failed to get SL data", zap.String("torrentID", id), zap.Error(err))
			continue
		}
		result[id] = sl
	}
	return result, nil
}

func (a *MTeamAdapter) GetPreciseSLData(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	if config.APIKey != "" {
		return a.slViaAPI(ctx, config, torrentID)
	}
	return a.slViaWeb(ctx, config, torrentID)
}

func (a *MTeamAdapter) slViaAPI(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	u := resolveBaseURL(config) + "/api/torrent/detail"
	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader("id="+torrentID))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	a.setAPIHeaders(req, config.APIKey)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	var result struct {
		Data struct {
			Status struct {
				Seeders  flexInt `json:"seeders"`
				Leechers flexInt `json:"leechers"`
			} `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, parseError("解析响应失败", err)
	}

	return &model.SLData{Seeders: int(result.Data.Status.Seeders), Leechers: int(result.Data.Status.Leechers)}, nil
}

func (a *MTeamAdapter) slViaWeb(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	u := resolveBaseURL(config) + "/details.php?id=" + torrentID
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := string(body)
	sl := &model.SLData{}

	if m := reMTeamSeeders.FindStringSubmatch(html); len(m) > 1 {
		sl.Seeders, _ = strconv.Atoi(m[1])
	}
	if m := reMTeamLeechers.FindStringSubmatch(html); len(m) > 1 {
		sl.Leechers, _ = strconv.Atoi(m[1])
	}

	return sl, nil
}

func (a *MTeamAdapter) UploadTorrent(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	if len(req.TorrentData) == 0 {
		return nil, &model.AppError{Code: 40001, Message: "种子文件数据为空"}
	}

	if config.APIKey != "" {
		return a.uploadViaAPI(ctx, config, req)
	}
	return a.uploadViaWeb(ctx, config, req)
}

func (a *MTeamAdapter) uploadViaAPI(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	baseURL := resolveBaseURL(config)
	uploadURL := baseURL + "/api/torrent/createOredit"

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
		fw.writeField("smallDescr", req.Subtitle)
	}
	if req.Description != "" {
		fw.writeField("descr", req.Description)
	}
	if req.Anonymous {
		fw.writeField("anonymous", "true")
	}
	if req.IMDbLink != "" {
		fw.writeField("imdb", req.IMDbLink)
	}
	if req.DoubanLink != "" {
		fw.writeField("douban", req.DoubanLink)
	}
	if req.MediaInfo != "" {
		fw.writeField("mediainfo", req.MediaInfo)
	}

	if v, ok := req.FormFields["category"]; ok {
		fw.writeField("category", v)
	}
	if v, ok := req.FormFields["source"]; ok {
		fw.writeField("source", v)
	}
	if v, ok := req.FormFields["medium"]; ok {
		fw.writeField("medium", v)
	}
	if v, ok := req.FormFields["standard"]; ok {
		fw.writeField("standard", v)
	}
	if v, ok := req.FormFields["videoCodec"]; ok {
		fw.writeField("videoCodec", v)
	}
	if v, ok := req.FormFields["audioCodec"]; ok {
		fw.writeField("audioCodec", v)
	}
	if v, ok := req.FormFields["team"]; ok {
		fw.writeField("team", v)
	}
	if v, ok := req.FormFields["processing"]; ok {
		fw.writeField("processing", v)
	}

	for k, v := range req.ExtraFields {
		fw.writeField(k, v)
	}

	if err := fw.hasError(); err != nil {
		return nil, fmt.Errorf("write form field: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, networkError("关闭 multipart writer 失败", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", uploadURL, &buf)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	a.setAPIHeaders(httpReq, config.APIKey)

	resp, err := a.doer.Client.Do(httpReq)
	if err != nil {
		return nil, uploadError("上传请求失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, classifySiteError(resp, "upload (API)")
	}

	var apiResp struct {
		Data struct {
			ID interface{} `json:"id"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &apiResp); err == nil {
		if apiResp.Data.ID != nil {
			torrentID := fmt.Sprintf("%v", apiResp.Data.ID)
			return &model.PublishResponse{
				Success:    true,
				TorrentID:  torrentID,
				DetailURL:  baseURL + "/detail/" + torrentID,
				TargetSite: config.Domain,
			}, nil
		}
		if apiResp.Message != "" {
			return &model.PublishResponse{Success: false, ErrorMessage: apiResp.Message}, nil
		}
	}

	html := string(body)
	if idMatch := reMTeamDetailID.FindStringSubmatch(html); len(idMatch) > 1 {
		torrentID := idMatch[1]
		return &model.PublishResponse{
			Success:    true,
			TorrentID:  torrentID,
			DetailURL:  baseURL + "/details.php?id=" + torrentID,
			TargetSite: config.Domain,
		}, nil
	}

	if strings.Contains(html, "成功") || strings.Contains(html, "succeeded") {
		return &model.PublishResponse{Success: true, TargetSite: config.Domain}, nil
	}

	return nil, &model.AppError{Code: 15001, Message: fmt.Sprintf("上传失败: HTTP %d", resp.StatusCode)}
}

func (a *MTeamAdapter) uploadViaWeb(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	baseURL := resolveBaseURL(config)
	uploadPath := "/upload.php"
	if config.Paths.Upload != "" {
		uploadPath = config.Paths.Upload
	}
	uploadURL := baseURL + uploadPath

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
	if req.Anonymous {
		fw.writeField("uplver", "1")
	}
	if req.DoubanLink != "" {
		fw.writeField("douban", req.DoubanLink)
	}
	if req.MediaInfo != "" {
		fw.writeField("mediainfo", req.MediaInfo)
	}
	for k, v := range req.FormFields {
		fw.writeField(k, v)
	}

	if err := fw.hasError(); err != nil {
		return nil, fmt.Errorf("write form field: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, networkError("关闭 multipart writer 失败", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", uploadURL, &buf)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	setCommonHeaders(httpReq, config.Cookie)

	resp, err := a.doer.Client.Do(httpReq)
	if err != nil {
		return nil, uploadError("上传请求失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := string(body)

	if resp.StatusCode != http.StatusOK {
		return nil, classifySiteError(resp, "upload (web)")
	}

	if idMatch := reMTeamDetailID.FindStringSubmatch(html); len(idMatch) > 1 {
		torrentID := idMatch[1]
		return &model.PublishResponse{
			Success:    true,
			TorrentID:  torrentID,
			DetailURL:  baseURL + "/details.php?id=" + torrentID,
			TargetSite: config.Domain,
		}, nil
	}

	if strings.Contains(html, "成功") || strings.Contains(html, "succeeded") {
		return &model.PublishResponse{Success: true, TargetSite: config.Domain}, nil
	}

	errMsg := "上传失败"
	if m := reMTeamErrorClass.FindStringSubmatch(html); len(m) > 1 {
		errMsg = strings.TrimSpace(m[1])
	}

	return nil, &model.AppError{Code: 15001, Message: errMsg}
}

func (a *MTeamAdapter) GetTorrentInfoHash(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	detail, err := a.GetTorrentDetail(ctx, config, torrentID)
	if err != nil {
		return "", err
	}
	if detail.InfoHash == "" {
		return "", notFoundError("未找到 info_hash")
	}
	return detail.InfoHash, nil
}

func resolveBaseURL(config *model.SiteConfig) string {
	u := config.Domain
	if !strings.HasPrefix(u, "http") {
		u = "https://" + u
	}
	return strings.TrimRight(u, "/")
}

func (a *MTeamAdapter) VerifyExists(ctx context.Context, config *model.SiteConfig, torrentID string) (bool, error) {
	results, err := a.SearchTorrents(ctx, config, torrentID, nil)
	if err != nil {
		return false, fmt.Errorf("search for verify exists %q: %w", torrentID, err)
	}
	for _, r := range results {
		if r.TorrentID == torrentID {
			return true, nil
		}
	}
	return false, nil
}

func (a *MTeamAdapter) FetchUserStats(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	if config.APIKey == "" {
		return a.fetchUserStatsHTML(ctx, config)
	}
	return a.fetchUserStatsAPI(ctx, config)
}

func (a *MTeamAdapter) fetchUserStatsAPI(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	u := resolveBaseURL(config) + "/api/member/profile"
	req, err := http.NewRequestWithContext(ctx, "POST", u, nil)
	if err != nil {
		return nil, err
	}
	a.setAPIHeaders(req, config.APIKey)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	var result struct {
		Message string `json:"message"`
		Data    struct {
			ID          flexInt `json:"id"`
			Username    string  `json:"username"`
			Role        flexInt `json:"role"`
			MemberCount struct {
				Uploaded   string `json:"uploaded"`
				Downloaded string `json:"downloaded"`
				Bonus      string `json:"bonus"`
				ShareRate  string `json:"shareRate"`
			} `json:"memberCount"`
			MemberStatus struct {
				LastLogin string `json:"lastLogin"`
			} `json:"memberStatus"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析用户信息失败: %w", err)
	}

	stats := &model.UserStatsResult{
		Username: result.Data.Username,
	}
	if className := a.resolveRoleName(ctx, config, int(result.Data.Role)); className != "" {
		stats.UserClass = className
	}
	stats.UploadBytes = parseSizeString(result.Data.MemberCount.Uploaded)
	stats.DownloadBytes = parseSizeString(result.Data.MemberCount.Downloaded)
	stats.BonusPoints, _ = strconv.ParseFloat(result.Data.MemberCount.Bonus, 64)
	stats.Ratio, _ = strconv.ParseFloat(result.Data.MemberCount.ShareRate, 64)
	if stats.Ratio == 0 && stats.DownloadBytes > 0 {
		stats.Ratio = float64(stats.UploadBytes) / float64(stats.DownloadBytes)
	}

	peerURL := resolveBaseURL(config) + "/api/tracker/myPeerStatistics"
	peerReq, err := http.NewRequestWithContext(ctx, "POST", peerURL, nil)
	if err != nil {
		a.logger.Warn("failed to construct myPeerStatistics request", zap.Error(err))
		return stats, nil
	}
	a.setAPIHeaders(peerReq, config.APIKey)

	peerResp, err := a.doer.Client.Do(peerReq)
	if err != nil {
		a.logger.Warn("myPeerStatistics request failed", zap.Error(err))
		return stats, nil
	}
	defer func() { drainBody(peerResp) }()

	peerBody, err := readBody(peerResp)
	if err != nil {
		a.logger.Warn("failed to read myPeerStatistics response", zap.Error(err))
		return stats, nil
	}

	var peerResult struct {
		Data struct {
			SeederCount  flexInt `json:"seederCount"`
			SeederSize   flexInt `json:"seederSize"`
			LeecherCount flexInt `json:"leecherCount"`
		} `json:"data"`
	}
	if err := json.Unmarshal(peerBody, &peerResult); err != nil {
		a.logger.Warn("failed to parse myPeerStatistics response", zap.Error(err))
		return stats, nil
	}

	stats.SeedingCount = int(peerResult.Data.SeederCount)
	stats.SeedingSize = int64(peerResult.Data.SeederSize)
	return stats, nil
}

var mteamRoleMap = map[int]string{
	1: "小卒", 2: "捕頭", 3: "知縣", 4: "通判", 5: "知州",
	6: "府丞", 7: "府尹", 8: "總督", 9: "大臣", 10: "VIP",
	11: "職人", 12: "巡查", 13: "總版", 14: "總管",
	15: "維護開發員", 16: "站長", 17: "候選管理", 18: "波菜管理", 19: "小組長",
}

func (a *MTeamAdapter) resolveRoleName(ctx context.Context, config *model.SiteConfig, role int) string {
	if role <= 0 {
		return ""
	}
	if name, ok := mteamRoleMap[role]; ok {
		return name
	}
	roleURL := resolveBaseURL(config) + "/api/member/sysRoleList"
	roleReq, err := http.NewRequestWithContext(ctx, "POST", roleURL, nil)
	if err != nil {
		return ""
	}
	a.setAPIHeaders(roleReq, config.APIKey)
	roleResp, err := a.doer.Client.Do(roleReq)
	if err != nil {
		return ""
	}
	defer func() { drainBody(roleResp) }()
	roleBody, err := readBody(roleResp)
	if err != nil {
		return ""
	}
	var roleResult struct {
		Data []struct {
			ID      flexInt `json:"id"`
			NameChs string  `json:"nameChs"`
		} `json:"data"`
	}
	if err := json.Unmarshal(roleBody, &roleResult); err != nil {
		return ""
	}
	for _, r := range roleResult.Data {
		if int(r.ID) == role {
			return r.NameChs
		}
	}
	return ""
}

func (a *MTeamAdapter) fetchUserStatsHTML(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	pageURL := config.Domain + "/index.php"
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return nil, err
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer httpclient.DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	htmlBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	html := string(htmlBytes)

	result := &model.UserStatsResult{}
	if m := reNexusUsername.FindStringSubmatch(html); len(m) > 2 {
		result.Username = strings.TrimSpace(m[2])
	} else if m := reNexusUsernameAlt.FindStringSubmatch(html); len(m) > 2 {
		result.Username = strings.TrimSpace(m[2])
	}
	if m := reNexusFontUploaded.FindStringSubmatch(html); len(m) > 1 {
		result.UploadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reNexusLabelUpload.FindStringSubmatch(html); len(m) > 1 {
		result.UploadBytes = parseSizeString(cleanText(m[1]))
	}
	if m := reNexusFontDownloaded.FindStringSubmatch(html); len(m) > 1 {
		result.DownloadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reNexusLabelDownload.FindStringSubmatch(html); len(m) > 1 {
		result.DownloadBytes = parseSizeString(cleanText(m[1]))
	}
	return result, nil
}

// buildIMDbURL §59.297: API 字段双形态——纯 ID 拼接 / 完整 URL 直用（防双拼）。
func buildIMDbURL(v string) string {
	if strings.HasPrefix(v, "http") {
		return v
	}
	return "https://www.imdb.com/title/" + v + "/"
}

// buildDoubanURL §59.297: 同族（Under Current 案 douban 双拼实证——API 返回完整 URL）。
func buildDoubanURL(v string) string {
	if strings.HasPrefix(v, "http") {
		return v
	}
	return "https://movie.douban.com/subject/" + v + "/"
}
