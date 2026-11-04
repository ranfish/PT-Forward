// ColaAdapter §59.460 附五（刀2）: 可乐（reelcore）独立架构 adapter——
// 与 MTeamAdapter 平级（两站各自独立架构各自演进），共享 mtorrentCore
// 协议层（M-Team 兼容面契约的请求构造/解析）。可乐特有行为全部在本
// adapter：Web 面详情（/api/v1/torrents/{id}——兼容面无 detail 端点）/
// promotion 优惠结构/peer 简名端点 myPeerStatus/搜索单轮（可乐无成人
// 区语义——M-Team 双轮对可乐=同果双倍配额浪费）。
// 三列分层见 docs/31 §59.460 附五。
package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

// ColaAdapter 组合：GenericAdapter（接口默认实现+HTML fallback）+
// mtorrentCore（协议层）+ 可乐特化方法。
type ColaAdapter struct {
	*GenericAdapter
	mtorrentCore
	doer   *HTTPDoer
	logger *zap.Logger
}

func NewColaAdapter(doer *HTTPDoer, logger *zap.Logger) *ColaAdapter {
	return &ColaAdapter{
		GenericAdapter: NewGenericAdapter("cola", doer, logger),
		mtorrentCore:   mtorrentCore{doer: doer, logger: logger},
		doer:           doer,
		logger:         logger,
	}
}

func (a *ColaAdapter) Framework() string { return "cola" }

// SearchTorrents §59.460 附五: 可乐单轮 normal——可乐无 M-Team 式成人区
// 语义（实测双轮返回同结果=同果双倍 Key 配额浪费）。
func (a *ColaAdapter) SearchTorrents(ctx context.Context, config *model.SiteConfig, keyword string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
	if config.APIKey != "" {
		return a.searchViaAPI(ctx, config, keyword, []string{"normal"})
	}
	return a.GenericAdapter.SearchTorrents(ctx, config, keyword, opts)
}

// DownloadTorrent 分发壳（限流+APIKey 判定——与 mTeamAdapter 同构，
// 下载协议两段在 mtorrentCore）。
func (a *ColaAdapter) DownloadTorrent(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
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
	return a.GenericAdapter.DownloadTorrent(ctx, config, torrentID)
}

// GetTorrentDetail §59.460 附五: 直连 Web 面详情（cookie 通道）——可乐
// 兼容面无 detail 端点（scope_denied 403），Web 面是正确第一路径（非降级）。
func (a *ColaAdapter) GetTorrentDetail(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	if config.Cookie == "" {
		return a.GenericAdapter.GetTorrentDetail(ctx, config, torrentID)
	}
	return a.detailViaWebJSON(ctx, config, torrentID)
}

// DetectDiscount §59.460 附五: 直连 Web 面 promotion 结构（可乐全站
// 免费站——免费语义在 promotion 不在 feed/兼容面）。
func (a *ColaAdapter) DetectDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	if config.Cookie == "" {
		return a.GenericAdapter.DetectDiscount(ctx, config, torrentID)
	}
	return a.discountViaWebPromotion(ctx, config, torrentID)
}

// FetchUserStats §59.460 附五: profile（core）+ peer 两层（可乐简名
// myPeerStatus 直连 → getUserTorrentList 翻页兜底）。
func (a *ColaAdapter) FetchUserStats(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	if config.APIKey == "" {
		return a.GenericAdapter.FetchUserStats(ctx, config)
	}
	prof, err := a.fetchProfile(ctx, config)
	if err != nil {
		return nil, err
	}
	stats := &model.UserStatsResult{
		Username:      prof.Username,
		UploadBytes:   prof.UploadBytes,
		DownloadBytes: prof.DownloadBytes,
		BonusPoints:   prof.Bonus,
		Ratio:         prof.Ratio,
	}
	// 可乐 Role→等级名映射待挖（wiki site-levels 7 级中文等级体系）
	if count, ok := a.seedingViaPeerStatus(ctx, config); ok {
		stats.SeedingCount = count
		return stats, nil
	}
	count, size := a.seedingViaUserTorrentList(ctx, config)
	stats.SeedingCount = count
	stats.SeedingSize = size
	return stats, nil
}

// discountViaWebPromotion §59.460 附三: Web 面详情 promotion 结构优惠
// 解析——可乐实测 {accounting_applied, upload:{num,den}, download:{num,den}}：
// download.num==0 → FREE；否则按 num/den 比例归 PERCENT 档（不认识的
// 比例回 NONE——站方全站免费期恒 FREE）。
func (a *ColaAdapter) discountViaWebPromotion(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	host := config.Domain
	if !strings.HasPrefix(host, "http") {
		host = "https://" + host
	}
	u := strings.TrimRight(host, "/") + "/api/v1/torrents/" + url.PathEscape(torrentID)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", httpclient.BrowserUserAgent())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cookie", config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { drainBody(resp) }()
	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("web promotion HTTP %d", resp.StatusCode), nil)
	}
	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	var d struct {
		Promotion struct {
			Download struct {
				Num json.Number `json:"num"`
				Den json.Number `json:"den"`
			} `json:"download"`
		} `json:"promotion"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, parseError("解析 Web 面优惠失败", err)
	}
	num, _ := d.Promotion.Download.Num.Int64()
	den, _ := d.Promotion.Download.Den.Int64()
	if den <= 0 {
		return &model.DiscountResult{Level: model.DiscountNone}, nil
	}
	switch {
	case num == 0:
		return &model.DiscountResult{Level: model.DiscountFree}, nil
	case num*2 == den:
		return &model.DiscountResult{Level: model.DiscountPercent50}, nil
	case num*10 == den*7:
		return &model.DiscountResult{Level: model.DiscountPercent70}, nil
	default:
		return &model.DiscountResult{Level: model.DiscountNone}, nil
	}
}


// detailViaWebJSON §59.460 附五: Web 面详情端点直连——GET
// https://<domain>/api/v1/torrents/{id}（cookie 通道；可乐实测 12KB/45+
// 字段完整覆盖 detail 需求：release_name/subtitle/size_bytes 字符串数字/
// info_hash+source_info_hash/技术规格全套/完整 MI/screenshots/tags）。
// Web 面判浏览器请求：带 cookie+浏览器 UA，禁止 x-api-key 混带。
func (a *ColaAdapter) detailViaWebJSON(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	host := config.Domain
	if !strings.HasPrefix(host, "http") {
		host = "https://" + host
	}
	u := strings.TrimRight(host, "/") + "/api/v1/torrents/" + url.PathEscape(torrentID)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", httpclient.BrowserUserAgent())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cookie", config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { drainBody(resp) }()
	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("web detail HTTP %d", resp.StatusCode), nil)
	}
	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	var d struct {
		ReleaseName  string      `json:"release_name"`
		Subtitle     string      `json:"subtitle"`
		SizeBytes    json.Number `json:"size_bytes"`
		InfoHash     string      `json:"info_hash"`
		VideoCodec   string      `json:"video_codec"`
		Audio        []string    `json:"audio"`
		Resolution   string      `json:"resolution"`
		Source       string      `json:"source"`
		ReleaseGroup string      `json:"release_group"`
		MediaInfo    string      `json:"media_info"`
		Screenshots  []string    `json:"screenshots"`
		Tags         []string    `json:"tags"`
	}
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, parseError("解析 Web 面详情失败", err)
	}
	size, _ := d.SizeBytes.Int64()
	audioCodec := ""
	if len(d.Audio) > 0 {
		audioCodec = d.Audio[0]
	}
	return &model.TorrentDetail{
		Title:        d.ReleaseName,
		Subtitle:     d.Subtitle,
		Size:         size,
		InfoHash:     strings.ToLower(d.InfoHash),
		Codec:        d.VideoCodec,
		AudioCodec:   audioCodec,
		Resolution:   d.Resolution,
		Source:       d.Source,
		ReleaseGroup: d.ReleaseGroup,
		MediaInfo:    d.MediaInfo,
		Screenshots:  d.Screenshots,
		Tags:         d.Tags,
	}, nil
}

// seedingViaPeerStatus §59.460 附四: 可乐简名端点 POST /api/tracker/
// myPeerStatus——响应 {code,data:{seeder:N}}（仅做种数无 size）。
// 返回 ok=false 时调用方继续 getUserTorrentList 兜底。
func (a *ColaAdapter) seedingViaPeerStatus(ctx context.Context, config *model.SiteConfig) (int, bool) {
	u := resolveBaseURL(config) + "/api/tracker/myPeerStatus"
	req, err := http.NewRequestWithContext(ctx, "POST", u, nil)
	if err != nil {
		return 0, false
	}
	a.setAPIHeaders(req, config.APIKey)
	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return 0, false
	}
	body, readErr := readBody(resp)
	drainBody(resp)
	if readErr != nil || resp.StatusCode != http.StatusOK {
		return 0, false
	}
	var result struct {
		Data struct {
			Seeder flexInt `json:"seeder"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, false
	}
	return int(result.Data.Seeder), true
}

// seedingViaUserTorrentList §59.460: myPeerStatistics 不可用时的做种数
// 降级通道——getUserTorrentList 翻页计数（可乐兼容面实测响应条目为
// {torrent:{size,source}} 嵌套形态；M-Team 主端点健康不触发本函数）。
func (a *ColaAdapter) seedingViaUserTorrentList(ctx context.Context, config *model.SiteConfig) (int, int64) {
	total, sizeSum := 0, int64(0)
	const pageSize = 100
	for page := 1; page <= 20; page++ {
		payload, _ := json.Marshal(map[string]interface{}{
			"pageNumber": page,
			"pageSize":   pageSize,
		})
		u := resolveBaseURL(config) + "/api/member/getUserTorrentList"
		req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(payload))
		if err != nil {
			return total, sizeSum
		}
		req.Header.Set("Content-Type", "application/json")
		a.setAPIHeaders(req, config.APIKey)
		resp, err := a.doer.Client.Do(req)
		if err != nil {
			return total, sizeSum
		}
		body, readErr := readBody(resp)
		drainBody(resp)
		// §59.460 附四: 非 200 静默零值缺陷修正（错误 JSON Unmarshal
		// 成功空结构 → rows=0 被当"翻完了"返回——真实状态码显式短路）
		if readErr != nil || resp.StatusCode != http.StatusOK {
			return total, sizeSum
		}
		var result struct {
			Data struct {
				Data []struct {
					Torrent struct {
						Size flexInt `json:"size"`
					} `json:"torrent"`
				} `json:"data"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return total, sizeSum
		}
		rows := result.Data.Data
		for _, r := range rows {
			total++
			sizeSum += int64(r.Torrent.Size)
		}
		if len(rows) < pageSize {
			return total, sizeSum
		}
	}
	return total, sizeSum
}

