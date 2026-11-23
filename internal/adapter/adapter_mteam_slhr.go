package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

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
			TorrentID:     item.ID.String(),
			Title:         item.Name,
			Size:          int64(item.Size),
			InfoHash:      "",
			DownloadURL:   resolveBaseURL(config) + "/download.php?id=" + item.ID.String(),
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
	// §59.460 附四回归审核: json.Number 双兼容（searchViaAPI 同族第二处
	// ——可乐 RSS API 链数字 ID 对纯 string unmarshal 失败白烧配额）
	ID       json.Number `json:"id"`
	Name     string      `json:"name"`
	Category string      `json:"category"`
	Size     flexInt     `json:"size"`
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
