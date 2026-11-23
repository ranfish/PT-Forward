package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
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
	// §59.460 附五（刀1）: 协议层经 mtorrentCore 组合
	// （searchViaAPI/downloadViaAPI/setAPIHeaders/fetchProfile 由 core
	// 提升供宿主调用）。doer/logger 显式声明遮蔽双 embed 同名提升
	// （GenericAdapter 与 mtorrentCore 各持同实例——三份同引用）。
	mtorrentCore
	doer   *HTTPDoer
	logger *zap.Logger
}

func NewMTeamAdapter(doer *HTTPDoer, logger *zap.Logger) *MTeamAdapter {
	return &MTeamAdapter{
		GenericAdapter: NewGenericAdapter("mteam", doer, logger),
		mtorrentCore:   mtorrentCore{doer: doer, logger: logger},
		doer:           doer,
		logger:         logger,
	}
}

func (a *MTeamAdapter) Framework() string { return "mteam" }

func (a *MTeamAdapter) SearchTorrents(ctx context.Context, config *model.SiteConfig, keyword string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
	if config.APIKey != "" {
		return a.searchViaAPI(ctx, config, keyword, []string{"normal", "adult"})
	}
	return a.GenericAdapter.SearchTorrents(ctx, config, keyword, opts)
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
