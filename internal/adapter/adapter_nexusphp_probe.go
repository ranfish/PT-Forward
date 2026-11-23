package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

func (a *NexusPHPAdapter) GetBatchSLData(ctx context.Context, config *model.SiteConfig, torrentIDs []string) (map[string]*model.SLData, error) {
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

func (a *NexusPHPAdapter) GetPreciseSLData(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	u := buildDetailsURL(config.Domain, torrentID, config.DetailsURLTemplate)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("请求详情页失败", err)
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("HTTP %d", resp.StatusCode), nil)
	}

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	return parseNexusSLFromHTML(string(body)), nil
}

// parseNexusSLFromHTML 从 nexusphp 详情页 HTML 提取 seeders/leechers。
// §55.19 抽出复用：DetectHRDiscountAndSL 也调用本函数，避免重复抓 HTML。
func parseNexusSLFromHTML(html string) *model.SLData {
	sl := &model.SLData{}

	seedersRe := reNexusSeedersCount
	leechersRe := reNexusLeechersCount

	rowRe := reNexusRow
	for _, row := range rowRe.FindAllString(html, -1) {
		rowText := cleanRowText(row)

		if strings.Contains(rowText, "同伴") {
			if m := seedersRe.FindStringSubmatch(rowText); len(m) > 1 {
				sl.Seeders, _ = strconv.Atoi(m[1])
			}
			if m := leechersRe.FindStringSubmatch(rowText); len(m) > 1 {
				sl.Leechers, _ = strconv.Atoi(m[1])
			}
			if sl.Seeders > 0 || sl.Leechers > 0 {
				return sl
			}
		}

		if strings.Contains(rowText, "基本信息") {
			if m := reNexusSeedersID.FindStringSubmatch(row); len(m) > 1 {
				sl.Seeders, _ = strconv.Atoi(m[1])
			}
			if m := reNexusLeechersID.FindStringSubmatch(row); len(m) > 1 {
				sl.Leechers, _ = strconv.Atoi(m[1])
			}
		}
	}

	if sl.Seeders == 0 && sl.Leechers == 0 {
		if m := seedersRe.FindStringSubmatch(html); len(m) > 1 {
			sl.Seeders, _ = strconv.Atoi(m[1])
		}
		if m := leechersRe.FindStringSubmatch(html); len(m) > 1 {
			sl.Leechers, _ = strconv.Atoi(m[1])
		}
	}

	return sl
}

func (a *NexusPHPAdapter) DetectDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	// §59.510 hdwing（瓷器）：优惠状态 JS 动态填充（列表页静态零信息）——
	// POST ajax_promotion.php（ids[]+csrf·csrf 取详情页 meta x-csrf）
	if strings.Contains(config.Domain, "hdwing") {
		return a.detectDiscountHdwing(ctx, config, torrentID)
	}
	if config.Discount.HasAPI && config.Discount.APIURL != "" {
		return a.detectDiscountAPI(ctx, config, torrentID)
	}
	return a.detectDiscountPage(ctx, config, torrentID)
}

// reHdwingCsrfMeta 详情页 CSRF token（NP 全页 meta——ajax_promotion POST 凭证）
var reHdwingCsrfMeta = regexp.MustCompile(`name="x-csrf" content="([a-f0-9]+)"`)

// detectDiscountHdwing §59.510 hdwing 优惠检测：
// ① GET 详情页提 meta x-csrf
// ② POST ajax_promotion.php（ids[]={tid}&csrf={token}·X-Requested-With）
// ③ 响应 {"status":200,"message":{"{tid}":{"sp_state":"<img class=\"pro_free\" …
//
//	title=\"免费\"/>","timeout":"…"}}}——class 词映射优惠档（pro_free=Free/
//	pro_2free=2xFree/pro_50percent=50%/pro_30percent=30%/pro_2x=2xUp；
//	timeout=限时截止·空=不限时——当前消费方 Level 即可·限时语义后续按需）。
//
// 实测（2026-11-10）：三 tid 全 pro_free 命中。
func (a *NexusPHPAdapter) detectDiscountHdwing(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	base := config.Domain
	if !strings.HasPrefix(base, "http") {
		base = "https://" + base
	}
	base = strings.TrimRight(base, "/")

	// ① 详情页提 csrf
	detailURL := buildDetailsURL(config.Domain, torrentID, config.DetailsURLTemplate)
	req, err := http.NewRequestWithContext(ctx, "GET", detailURL, nil)
	if err != nil {
		return nil, err
	}
	setCommonHeaders(req, config.Cookie)
	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("获取详情页失败", err)
	}
	body, _ := readBody(resp)
	drainBody(resp)
	csrfM := reHdwingCsrfMeta.FindStringSubmatch(string(body))
	if csrfM == nil {
		return nil, configError("未在详情页找到 csrf token")
	}

	// ② POST ajax_promotion
	form := url.Values{}
	form.Set("ids[]", torrentID)
	form.Set("csrf", csrfM[1])
	promoReq, err := http.NewRequestWithContext(ctx, "POST", base+"/ajax_promotion.php", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	setCommonHeaders(promoReq, config.Cookie)
	promoReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	promoReq.Header.Set("X-Requested-With", "XMLHttpRequest")
	promoReq.Header.Set("Referer", base+"/torrents.php")
	promoResp, err := a.doer.Client.Do(promoReq)
	if err != nil {
		return nil, networkError("请求优惠状态失败", err)
	}
	defer func() { drainBody(promoResp) }()
	promoBody, err := readBody(promoResp)
	if err != nil {
		return nil, err
	}

	// ③ 解析 {message:{tid:{sp_state:…}}}
	var payload struct {
		Status  int `json:"status"`
		Message map[string]struct {
			SpState string `json:"sp_state"`
			Timeout string `json:"timeout"`
		} `json:"message"`
	}
	if err := json.Unmarshal(promoBody, &payload); err != nil {
		return nil, parseError("解析优惠响应失败", err)
	}
	entry, ok := payload.Message[torrentID]
	if !ok {
		return &model.DiscountResult{Level: model.DiscountNone}, nil
	}
	lower := strings.ToLower(entry.SpState)
	switch {
	case strings.Contains(lower, "pro_2free"):
		return &model.DiscountResult{Level: model.Discount2xFree, Multiplier: 2.0}, nil
	case strings.Contains(lower, "pro_free"):
		return &model.DiscountResult{Level: model.DiscountFree}, nil
	case strings.Contains(lower, "pro_50percent"), strings.Contains(lower, "pro_50down"):
		return &model.DiscountResult{Level: model.DiscountPercent50}, nil
	case strings.Contains(lower, "pro_30percent"), strings.Contains(lower, "pro_30down"):
		return &model.DiscountResult{Level: model.DiscountPercent30}, nil
	case strings.Contains(lower, "pro_2x"):
		return &model.DiscountResult{Level: model.Discount2xUp, Multiplier: 2.0}, nil
	}
	return &model.DiscountResult{Level: model.DiscountNone}, nil
}

func (a *NexusPHPAdapter) detectDiscountAPI(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	u := config.Domain
	if !strings.HasPrefix(u, "http") {
		u = "https://" + u
	}
	u += strings.Replace(config.Discount.APIURL, "{id}", torrentID, 1)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("请求优惠API失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := strings.ToLower(string(body))

	for _, sel := range config.Discount.Selectors {
		if !strings.Contains(html, strings.ToLower(sel)) {
			continue
		}
		lower := strings.ToLower(sel)
		switch {
		case strings.Contains(lower, "2xfree"):
			return &model.DiscountResult{Level: model.Discount2xFree, Multiplier: 2.0}, nil
		case strings.Contains(lower, "2xup") || strings.Contains(lower, "2upload"):
			return &model.DiscountResult{Level: model.Discount2xUp, Multiplier: 2.0}, nil
		case strings.Contains(lower, "free"):
			return &model.DiscountResult{Level: model.DiscountFree}, nil
		case strings.Contains(lower, "50%"):
			return &model.DiscountResult{Level: model.DiscountPercent50}, nil
		case strings.Contains(lower, "30%"):
			return &model.DiscountResult{Level: model.DiscountPercent30}, nil
		}
	}

	return &model.DiscountResult{Level: model.DiscountNone}, nil
}

func (a *NexusPHPAdapter) detectDiscountPage(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	u := buildDetailsURL(config.Domain, torrentID, config.DetailsURLTemplate)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造优惠检测请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("请求优惠详情页失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := strings.ToLower(string(body))

	result := DetectDiscountFromDetailsPage(html, &config.DiscountDetection)
	if result.Level != model.DiscountNone {
		return result, nil
	}

	return &model.DiscountResult{Level: model.DiscountNone}, nil
}

func (a *NexusPHPAdapter) DetectHR(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
	u := buildDetailsURL(config.Domain, torrentID, config.DetailsURLTemplate)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造HR检测请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("请求HR详情页失败", err)
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

func (a *NexusPHPAdapter) DetectHRAndDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, *model.DiscountResult, error) {
	if config.Discount.HasAPI && config.Discount.APIURL != "" {
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

	u := buildDetailsURL(config.Domain, torrentID, config.DetailsURLTemplate)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, networkError("构造请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		a.logger.Warn("detectHRAndDiscount request failed",
			zap.String("url", u),
			zap.String("torrent", torrentID),
			zap.Error(err))
		return nil, nil, networkError("请求详情页失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, nil, err
	}
	html := strings.ToLower(string(body))

	hrResult := &model.HRResult{HasHR: detectHRFromHTML(html)}
	if hrResult.HasHR {
		hrResult.SeedTimeH = config.HR.SeedTimeH()
	}

	discResult := DetectDiscountFromDetailsPage(html, &config.DiscountDetection)

	return hrResult, discResult, nil
}

// §55.19 根本修复：DetectHRDiscountAndSL 在 detect 阶段顺便提取 SL 数据。
// 复用 DetectHRAndDiscount 已抓取的详情页 HTML，避免评分阶段 GetBatchSLData 重复请求同一 URL。
// 实现 CombinedHRDiscountSLDetector 接口（包装扩展，rss/engine.go 用类型断言优先调用）。
func (a *NexusPHPAdapter) DetectHRDiscountAndSL(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, *model.DiscountResult, *model.SLData, error) {
	if config.Discount.HasAPI && config.Discount.APIURL != "" {
		// API 模式：HR/Discount 走 API，SL 必须抓详情页（无法合并，单独抓）
		hr, err := a.DetectHR(ctx, config, torrentID)
		if err != nil {
			return nil, nil, nil, err
		}
		disc, err := a.DetectDiscount(ctx, config, torrentID)
		if err != nil {
			return hr, nil, nil, err
		}
		sl, slErr := a.GetPreciseSLData(ctx, config, torrentID)
		if slErr != nil {
			a.logger.Warn("DetectHRDiscountAndSL: get SL failed",
				zap.String("torrent", torrentID), zap.Error(slErr))
			sl = &model.SLData{}
		}
		return hr, disc, sl, nil
	}

	// 非 API 模式：抓一次 HTML，提取 HR + Discount + SL
	u := buildDetailsURL(config.Domain, torrentID, config.DetailsURLTemplate)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, nil, networkError("构造请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		a.logger.Warn("DetectHRDiscountAndSL request failed",
			zap.String("url", u),
			zap.String("torrent", torrentID),
			zap.Error(err))
		return nil, nil, nil, networkError("请求详情页失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, nil, nil, err
	}
	rawHTML := string(body)
	lowerHTML := strings.ToLower(rawHTML)

	hrResult := &model.HRResult{HasHR: detectHRFromHTML(lowerHTML)}
	if hrResult.HasHR {
		hrResult.SeedTimeH = config.HR.SeedTimeH()
	}
	discResult := DetectDiscountFromDetailsPage(lowerHTML, &config.DiscountDetection)
	sl := parseNexusSLFromHTML(rawHTML)

	return hrResult, discResult, sl, nil
}

func (a *NexusPHPAdapter) FetchUserStats(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	stats, err := a.fetchUserStatsAPI(ctx, config)
	if err == nil && stats != nil {
		if stats.SeedingCount > 0 && stats.SeedingSize > 0 {
			a.scrapePasskey(ctx, config, stats)
			if cuhashEnabled(config) {
				a.scrapeCuhash(ctx, config, stats)
			}
			return stats, nil
		}
		htmlStats, htmlErr := a.fetchUserStatsHTML(ctx, config)
		if htmlErr == nil && htmlStats != nil {
			if stats.SeedingCount == 0 && htmlStats.SeedingCount > 0 {
				stats.SeedingCount = htmlStats.SeedingCount
			}
			if stats.SeedingSize == 0 && htmlStats.SeedingSize > 0 {
				stats.SeedingSize = htmlStats.SeedingSize
			}
			if stats.Username == "" || strings.HasPrefix(stats.Username, "UID:") {
				if htmlStats.Username != "" {
					stats.Username = htmlStats.Username
				}
			}
			if (stats.UserClass == "" || strings.HasPrefix(stats.UserClass, "UID:")) && htmlStats.UserClass != "" {
				stats.UserClass = htmlStats.UserClass
			}
			if htmlStats.Passkey != "" {
				stats.Passkey = htmlStats.Passkey
			}
		}
		a.scrapePasskey(ctx, config, stats)
		if cuhashEnabled(config) {
			a.scrapeCuhash(ctx, config, stats)
		}
		return stats, nil
	}
	stats, err = a.fetchUserStatsHTML(ctx, config)
	if err == nil && stats != nil {
		a.scrapePasskey(ctx, config, stats)
		if cuhashEnabled(config) {
			a.scrapeCuhash(ctx, config, stats)
		}
	}
	return stats, err
}

func (a *NexusPHPAdapter) fetchUserStatsAPI(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	apiURL := config.Domain + "/api/user"
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
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
		return nil, fmt.Errorf("API returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if !reNexusApiUser.Match(body) {
		return nil, fmt.Errorf("not a valid user API response")
	}

	var raw map[string]interface{}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&raw); err != nil {
		return nil, err
	}

	data := raw
	if d, ok := raw["data"].(map[string]interface{}); ok {
		data = d
	}

	result := &model.UserStatsResult{}
	result.Username = jsonStr(data, "username", "name")
	result.UploadBytes = jsonInt(data, "uploaded", "upload")
	result.DownloadBytes = jsonInt(data, "downloaded", "download")
	result.Ratio = jsonFloat(data, "ratio")
	result.BonusPoints = jsonFloat(data, "bonus", "seedbonus")
	if result.Username == "" {
		if uid := jsonInt(data, "uid", "id", "user_id"); uid > 0 {
			result.Username = fmt.Sprintf("UID: %d", uid)
		}
	}
	if result.Ratio == 0 && result.DownloadBytes > 0 && result.UploadBytes > 0 {
		result.Ratio = float64(result.UploadBytes) / float64(result.DownloadBytes)
	}
	return result, nil
}

var rePasskeyRow = regexp.MustCompile(`(?s)(?:密钥|密匙|PASSKEY).{0,500}?([a-f0-9]{32})`)
var reCuhashFromLink = regexp.MustCompile(`cuhash=([a-f0-9]{32})`)

func (a *NexusPHPAdapter) scrapePasskey(ctx context.Context, config *model.SiteConfig, stats *model.UserStatsResult) {
	if stats.Passkey != "" {
		return
	}
	if config.Cookie == "" {
		return
	}
	pageURL := config.Domain + "/usercp.php"
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return
	}
	setCommonHeaders(req, config.Cookie)
	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return
	}
	defer httpclient.DrainBody(resp)
	if resp.StatusCode != http.StatusOK {
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	if m := rePasskeyRow.FindStringSubmatch(string(body)); len(m) > 1 {
		stats.Passkey = m[1]
	}
}

// cuhashEnabled §59.185 ②: cuhash 依赖判定——mode 显式声明或下载模板含 cuhash=
// （如城市 hdcity "download?id={id}&cuhash={passkey}"）。此前仅判 mode，
// 城市实际 mode=template → 统计同步的 cuhash 抓取链路整条空转（下载必败根因）。
func cuhashEnabled(config *model.SiteConfig) bool {
	return config.DownloadMode == "cuhash" || strings.Contains(config.DownloadURLTemplate, "cuhash=")
}

// ScrapeCuhash §59.185 ②: 公开形态——首页任意下载链接提取当前 32hex cuhash
// （无则空串）。供下载失败懒刷新（CuhashScraper 可选接口）。
func (a *NexusPHPAdapter) ScrapeCuhash(ctx context.Context, config *model.SiteConfig) string {
	if config.Cookie == "" {
		return ""
	}
	pageURL := config.Domain + "/index.php"
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return ""
	}
	setCommonHeaders(req, config.Cookie)
	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return ""
	}
	defer httpclient.DrainBody(resp)
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return ""
	}
	if m := reCuhashFromLink.FindStringSubmatch(string(body)); len(m) > 1 {
		return m[1]
	}
	return ""
}

func (a *NexusPHPAdapter) scrapeCuhash(ctx context.Context, config *model.SiteConfig, stats *model.UserStatsResult) {
	if cu := a.ScrapeCuhash(ctx, config); cu != "" {
		stats.Passkey = cu
	}
}

func (a *NexusPHPAdapter) fetchUserStatsHTML(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	html := string(body)

	if strings.Contains(html, "login.php") && !strings.Contains(html, "userdetails") && !strings.Contains(html, "userdetail.") {
		return nil, fmt.Errorf("cookie 无效或已过期")
	}

	searchHTML := html
	if m := reNexusInfoBlock.FindStringSubmatch(html); len(m) > 1 {
		searchHTML = m[1]
	}

	result := &model.UserStatsResult{}

	var extractedUID string
	if m := reNexusWelcomeBack.FindStringSubmatch(html); len(m) > 2 {
		text := stripHTMLTags(m[2])
		text = strings.TrimSpace(text)
		if text != "" && len(text) < 30 {
			result.Username = cleanUsername(text)
			extractedUID = m[1]
		}
	}
	if result.Username == "" {
		if m := reNexusUsernameByName.FindStringSubmatch(html); len(m) > 2 {
			result.Username = cleanUsername(m[2])
			extractedUID = m[1]
		} else if m := reNexusUsernameByName2.FindStringSubmatch(html); len(m) > 2 {
			result.Username = cleanUsername(m[2])
			extractedUID = m[1]
		} else if m := reNexusUsernameUUID.FindStringSubmatch(html); len(m) > 1 {
			result.Username = cleanUsername(m[1])
		} else if m := reNexusUsername.FindStringSubmatch(searchHTML); len(m) > 2 {
			result.Username = cleanUsername(m[2])
			extractedUID = m[1]
		} else if m := reNexusUsernameSpan.FindStringSubmatch(searchHTML); len(m) > 2 {
			result.Username = cleanUsername(m[2])
			extractedUID = m[1]
		} else if m := reNexusUsername.FindStringSubmatch(html); len(m) > 2 {
			result.Username = cleanUsername(m[2])
			extractedUID = m[1]
		} else if m := reNexusUsernameAlt.FindStringSubmatch(html); len(m) > 2 {
			un := strings.TrimSpace(m[2])
			if !strings.Contains(un, "<") && len(un) > 0 && len(un) < 30 {
				result.Username = cleanUsername(un)
			}
			if extractedUID == "" {
				extractedUID = m[1]
			}
		}
	}

	if result.Username == "" && extractedUID == "" {
		if m := reNexusUsernameDeep.FindStringSubmatch(html); len(m) > 2 {
			text := cleanText(m[2])
			if text != "" && len(text) < 30 {
				result.Username = cleanUsername(text)
				extractedUID = m[1]
			}
		}
	}

	if result.Username == "" {
		if m := reNexusUsernameSimple.FindStringSubmatch(html); len(m) > 1 {
			text := strings.TrimSpace(m[1])
			if text != "" && len(text) < 30 {
				result.Username = cleanUsername(text)
			}
		}
	}

	if result.Username == "" {
		if extractedUID != "" {
			result.Username = "UID:" + extractedUID
		} else if m := reNexusUserID.FindStringSubmatch(searchHTML); len(m) > 1 {
			result.Username = "UID:" + m[1]
		}
	}

	if m := reNexusUserNameClass.FindStringSubmatch(searchHTML); len(m) > 1 {
		className := m[1]
		if idx := strings.LastIndex(className, "_"); idx >= 0 {
			className = className[:idx]
		}
		if vc := isValidUserClass(className); vc != "" {
			result.UserClass = vc
		}
	}
	if result.UserClass == "" {
		if m := reNexusUserClassSpan.FindStringSubmatch(html); len(m) > 1 {
			result.UserClass = strings.TrimSpace(m[1])
		}
	}
	if result.UserClass == "" {
		if m := reNexusUserClassStrong.FindStringSubmatch(html); len(m) > 1 {
			result.UserClass = strings.TrimSpace(m[1])
		}
	}

	// §59.332: keepfrds 新版形态最优先（span.uploaded / span.downloaded）
	if m := reKfNewUploaded.FindStringSubmatch(html); len(m) > 1 {
		result.UploadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reNexusFontUploaded.FindStringSubmatch(searchHTML); len(m) > 1 {
		result.UploadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reNexusLabelUpload.FindStringSubmatch(searchHTML); len(m) > 1 && !isSiteTotalMatch(searchHTML, m) {
		result.UploadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reNexusLabelUploadRelaxed.FindStringSubmatch(searchHTML); len(m) > 1 {
		result.UploadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reNexusLabelUploadRelaxed.FindStringSubmatch(html); len(m) > 1 {
		result.UploadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reNexusDataStats.FindStringSubmatch(html); len(m) > 1 {
		result.UploadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reNexusIconTitleUpload.FindStringSubmatch(html); len(m) > 1 {
		result.UploadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reNexusJsonUpload.FindStringSubmatch(html); len(m) > 1 {
		result.UploadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reAltUpload.FindStringSubmatch(html); len(m) > 1 {
		result.UploadBytes = parseSizeString(m[1] + " " + m[2])
	} else if m := reFontLabelUpload.FindStringSubmatch(html); len(m) > 1 {
		result.UploadBytes = parseSizeString(m[1] + " " + m[2])
	} else if m := reNexusLabelUpload.FindStringSubmatch(searchHTML); len(m) > 1 {
		result.UploadBytes = parseSizeString(cleanText(m[1]))
	}

	if m := reNexusFontDownloaded.FindStringSubmatch(searchHTML); len(m) > 1 {
		result.DownloadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reNexusLabelDownload.FindStringSubmatch(searchHTML); len(m) > 1 && !isSiteTotalMatch(searchHTML, m) {
		result.DownloadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reNexusLabelDownloadRelaxed.FindStringSubmatch(searchHTML); len(m) > 1 {
		result.DownloadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reNexusLabelDownloadRelaxed.FindStringSubmatch(html); len(m) > 1 {
		result.DownloadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reNexusIconTitleDown.FindStringSubmatch(html); len(m) > 1 {
		result.DownloadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reNexusJsonDownload.FindStringSubmatch(html); len(m) > 1 {
		result.DownloadBytes = parseSizeString(cleanText(m[1]))
	} else if m := reAltDownload.FindStringSubmatch(html); len(m) > 1 {
		result.DownloadBytes = parseSizeString(m[1] + " " + m[2])
	} else if m := reFontLabelDownload.FindStringSubmatch(html); len(m) > 1 {
		result.DownloadBytes = parseSizeString(m[1] + " " + m[2])
	} else if m := reNexusLabelDownload.FindStringSubmatch(searchHTML); len(m) > 1 {
		result.DownloadBytes = parseSizeString(cleanText(m[1]))
	}

	if m := reNexusFontRatio.FindStringSubmatch(searchHTML); len(m) > 1 {
		result.Ratio, _ = strconv.ParseFloat(cleanText(m[1]), 64)
	} else if m := reNexusLabelRatio.FindStringSubmatch(searchHTML); len(m) > 1 {
		result.Ratio, _ = strconv.ParseFloat(cleanText(m[1]), 64)
	} else if m := reNexusIconTitleRatio.FindStringSubmatch(html); len(m) > 1 {
		result.Ratio, _ = strconv.ParseFloat(cleanText(m[1]), 64)
	} else if m := reNexusJsonRatio.FindStringSubmatch(html); len(m) > 1 {
		result.Ratio, _ = strconv.ParseFloat(cleanText(m[1]), 64)
	} else if m := reAltRatio.FindStringSubmatch(html); len(m) > 1 {
		result.Ratio, _ = strconv.ParseFloat(m[1], 64)
	} else if m := reFontLabelRatio.FindStringSubmatch(html); len(m) > 1 {
		result.Ratio, _ = strconv.ParseFloat(m[1], 64)
	}
	if result.Ratio == 0 && result.DownloadBytes > 0 && result.UploadBytes > 0 {
		result.Ratio = float64(result.UploadBytes) / float64(result.DownloadBytes)
	}

	if m := reNexusFontBonus.FindStringSubmatch(searchHTML); len(m) > 1 {
		bonusStr := strings.ReplaceAll(cleanText(m[1]), ",", "")
		result.BonusPoints, _ = strconv.ParseFloat(bonusStr, 64)
	} else if m := reNexusFontBonusInline.FindStringSubmatch(searchHTML); len(m) > 1 {
		bonusStr := strings.ReplaceAll(cleanText(m[1]), ",", "")
		result.BonusPoints, _ = strconv.ParseFloat(bonusStr, 64)
	} else if m := reAltBonus.FindStringSubmatch(html); len(m) > 1 {
		bonusStr := strings.ReplaceAll(m[1], ",", "")
		result.BonusPoints, _ = strconv.ParseFloat(bonusStr, 64)
	} else if m := reNexusBonusBeforeLabel.FindStringSubmatch(html); len(m) > 1 {
		bonusStr := strings.ReplaceAll(m[1], ",", "")
		result.BonusPoints, _ = strconv.ParseFloat(bonusStr, 64)
	} else if m := reNexusLabelBonus.FindStringSubmatch(searchHTML); len(m) > 1 {
		bonusStr := strings.ReplaceAll(cleanText(m[1]), ",", "")
		result.BonusPoints, _ = strconv.ParseFloat(bonusStr, 64)
	} else if m := reNexusDataBonus.FindStringSubmatch(html); len(m) > 1 {
		bonusStr := strings.ReplaceAll(cleanText(m[1]), ",", "")
		result.BonusPoints, _ = strconv.ParseFloat(bonusStr, 64)
	} else if m := reNexusJsonBonus.FindStringSubmatch(html); len(m) > 1 {
		bonusStr := strings.ReplaceAll(cleanText(m[1]), ",", "")
		result.BonusPoints, _ = strconv.ParseFloat(bonusStr, 64)
	} else if m := reFontLabelBonus.FindStringSubmatch(html); len(m) > 1 {
		bonusStr := strings.ReplaceAll(m[1], ",", "")
		result.BonusPoints, _ = strconv.ParseFloat(bonusStr, 64)
	} else if m := reFontLabelBonusSpan.FindStringSubmatch(html); len(m) > 1 {
		bonusStr := strings.ReplaceAll(m[1], ",", "")
		result.BonusPoints, _ = strconv.ParseFloat(bonusStr, 64)
	} else if m := reLinkBonus.FindStringSubmatch(html); len(m) > 1 {
		bonusStr := strings.ReplaceAll(m[1], ",", "")
		result.BonusPoints, _ = strconv.ParseFloat(bonusStr, 64)
	} else if m := reYuanbaoBonus.FindStringSubmatch(html); len(m) > 1 {
		bonusStr := strings.ReplaceAll(m[1], ",", "")
		result.BonusPoints, _ = strconv.ParseFloat(bonusStr, 64)
	}

	if m := reSpriteArrowSeeding.FindStringSubmatch(html); len(m) > 1 {
		if n, err := strconv.Atoi(strings.TrimSpace(m[1])); err == nil && n > 0 {
			result.SeedingCount = n
		}
		if sz := parseSizeString(strings.ReplaceAll(m[2], "&nbsp;", " ")); sz > 0 {
			result.SeedingSize = sz
		}
	} else if m := reNexusArrowUp.FindStringSubmatch(searchHTML); len(m) > 1 {
		arrowText := strings.TrimSpace(m[1])
		if n, err := strconv.Atoi(arrowText); err == nil && n > 0 {
			result.SeedingCount = n
		} else if sm := reNexusSeedLabel.FindStringSubmatch(arrowText); len(sm) > 1 {
			result.SeedingCount, _ = strconv.Atoi(sm[1])
		}
	} else if m := reNexusArrowUp.FindStringSubmatch(html); len(m) > 1 {
		arrowText := strings.TrimSpace(m[1])
		if n, err := strconv.Atoi(arrowText); err == nil && n > 0 {
			result.SeedingCount = n
		} else if sm := reNexusSeedLabel.FindStringSubmatch(arrowText); len(sm) > 1 {
			result.SeedingCount, _ = strconv.Atoi(sm[1])
		}
	}
	if result.SeedingCount == 0 {
		if m := reMaterialPlayArrow.FindStringSubmatch(html); len(m) > 1 {
			result.SeedingCount, _ = strconv.Atoi(m[1])
		}
	}
	if result.SeedingCount == 0 {
		if m := reAltSeedingCount.FindStringSubmatch(html); len(m) > 1 {
			result.SeedingCount, _ = strconv.Atoi(m[1])
		}
	}
	if result.SeedingCount == 0 {
		// §59.332: 新版形态最优先（title="当前做种" 图标后数字）
		if m := reKfNewSeedingCount.FindStringSubmatch(html); len(m) > 1 {
			result.SeedingCount, _ = strconv.Atoi(m[1])
		}
	}
	if result.SeedingCount == 0 {
		if m := reFontTitleSeeding.FindStringSubmatch(html); len(m) > 1 {
			result.SeedingCount, _ = strconv.Atoi(m[1])
		}
	}
	if result.SeedingCount == 0 {
		if m := reNexusArrowUpImg.FindStringSubmatch(html); len(m) > 1 {
			result.SeedingCount, _ = strconv.Atoi(m[1])
		}
	}
	if result.SeedingCount == 0 {
		if m := reFontArrowSeeding.FindStringSubmatch(html); len(m) > 1 {
			result.SeedingCount, _ = strconv.Atoi(m[1])
		}
	}

	uid := extractedUID
	if uid == "" {
		if m := reNexusUserID.FindStringSubmatch(searchHTML); len(m) > 1 {
			uid = m[1]
		}
	}
	if uid == "" {
		if m := reNexusUserID.FindStringSubmatch(html); len(m) > 1 {
			uid = m[1]
		}
	}
	if uid == "" {
		if m := reNexusUserUUID.FindStringSubmatch(html); len(m) > 1 {
			uid = "uuid:" + m[1]
		}
	}
	if uid == "" {
		uid = a.resolveUIDFromSelfProfile(ctx, config)
	}
	if uid != "" {
		a.enrichFromUserDetails(ctx, config, uid, result)
	}

	return result, nil
}

func (a *NexusPHPAdapter) enrichFromUserDetails(ctx context.Context, config *model.SiteConfig, uid string, result *model.UserStatsResult) {
	var pageURL string
	if strings.HasPrefix(uid, "uuid:") {
		pageURL = config.Domain + "/userdetails.php?uuid=" + strings.TrimPrefix(uid, "uuid:")
	} else {
		pageURL = config.Domain + "/userdetails.php?id=" + uid
	}
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return
	}
	defer httpclient.DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	html := string(body)

	if m := reDetailTransfer.FindStringSubmatch(html); len(m) > 1 {
		transferText := m[1]
		if idx := strings.LastIndex(transferText, "</table>"); idx > 0 {
			transferText = transferText[:idx]
		}
		transferText = cleanText(transferText)
		if up := reDetailUpload.FindStringSubmatch(transferText); len(up) > 1 {
			result.UploadBytes = parseSizeString(cleanText(up[1]))
		}
		if dn := reDetailDownload.FindStringSubmatch(transferText); len(dn) > 1 {
			result.DownloadBytes = parseSizeString(cleanText(dn[1]))
		}
	}
	if result.UploadBytes < 1048576 {
		if m := reDetailRowheadUpload.FindStringSubmatch(html); len(m) > 1 {
			val := parseSizeString(cleanText(m[1]))
			if val > result.UploadBytes {
				result.UploadBytes = val
			}
		}
	}
	if result.DownloadBytes < 1048576 {
		if m := reDetailRowheadDownload.FindStringSubmatch(html); len(m) > 1 {
			val := parseSizeString(cleanText(m[1]))
			if val > result.DownloadBytes {
				result.DownloadBytes = val
			}
		}
	}
	if result.Ratio == 0 {
		if m := reDetailRowheadRatio.FindStringSubmatch(html); len(m) > 1 {
			result.Ratio, _ = strconv.ParseFloat(cleanText(m[1]), 64)
		}
	}
	if result.Ratio == 0 && result.DownloadBytes > 0 && result.UploadBytes > 0 {
		result.Ratio = float64(result.UploadBytes) / float64(result.DownloadBytes)
	}

	if result.UserClass == "" {
		if m := reDetailClass.FindStringSubmatch(html); len(m) > 1 {
			classHTML := m[1]
			if img := reDetailClassImg.FindStringSubmatch(classHTML); len(img) > 1 {
				result.UserClass = img[1]
			} else {
				uc := cleanText(classHTML)
				if len(uc) > 0 && len(uc) < 20 {
					result.UserClass = uc
				}
			}
		}
	}

	if m := reDetailBonus.FindStringSubmatch(html); len(m) > 1 {
		raw := cleanText(m[1])
		raw = strings.ReplaceAll(raw, ",", "")
		for _, part := range strings.Fields(raw) {
			if v, err := strconv.ParseFloat(part, 64); err == nil && v > result.BonusPoints {
				result.BonusPoints = v
				break
			}
		}
	}

	if result.BonusPoints == 0 {
		if m := reUcoinValue.FindStringSubmatch(html); len(m) > 1 {
			bonusStr := strings.ReplaceAll(m[1], ",", "")
			if v, err := strconv.ParseFloat(bonusStr, 64); err == nil {
				result.BonusPoints = v
			}
		}
	}

	if result.SeedingPoints == 0 {
		if m := reDetailSeedingPoints.FindStringSubmatch(html); len(m) > 1 {
			spStr := strings.ReplaceAll(cleanText(m[1]), ",", "")
			spStr = strings.Fields(spStr)[0]
			result.SeedingPoints, _ = strconv.ParseFloat(spStr, 64)
		}
	}
	if result.SeedingPoints == 0 {
		if m := reDetailSeedingPointsB.FindStringSubmatch(html); len(m) > 1 {
			spStr := strings.ReplaceAll(m[1], ",", "")
			result.SeedingPoints, _ = strconv.ParseFloat(spStr, 64)
		}
	}

	if result.SeedingCount == 0 || result.SeedingSize == 0 {
		if m := reDetailSeeding.FindStringSubmatch(html); len(m) > 1 {
			tdHTML := m[1]
			seedStr := cleanText(tdHTML)
			parts := strings.Split(seedStr, " ")
			for _, p := range parts {
				if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil && n > 0 && result.SeedingCount == 0 {
					result.SeedingCount = n
					break
				}
			}
			if result.SeedingSize == 0 {
				if sv := reDetailSeedingVol.FindStringSubmatch(tdHTML); len(sv) > 1 {
					if sz := parseSizeString(sv[1]); sz > 0 {
						result.SeedingSize = sz
					}
				}

			}
		}
	}
	if result.SeedingCount == 0 {
		if m := reDetailSeedingSimple.FindStringSubmatch(html); len(m) > 1 {
			seedStr := cleanText(m[1])
			parts := strings.Split(seedStr, " ")
			for _, p := range parts {
				if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil && n > 0 {
					result.SeedingCount = n
					break
				}
			}
		}
	}
	if result.SeedingCount == 0 {
		if m := reNexusJsonSeeding.FindStringSubmatch(html); len(m) > 1 {
			result.SeedingCount, _ = strconv.Atoi(m[1])
		}
	}

	if result.Username == "" || strings.HasPrefix(result.Username, "UID:") {
		if m := reDetailUsername.FindStringSubmatch(html); len(m) > 1 {
			un := cleanText(m[1])
			un = cleanUsername(un)
			if un != "" {
				result.Username = un
			}
		}
	}

	if strings.HasPrefix(result.Username, "UID:") {
		if m := reNexusUsername.FindStringSubmatch(html); len(m) > 2 {
			if un := cleanUsername(m[2]); un != "" {
				result.Username = un
			}
		} else if m := reNexusUsernameAlt.FindStringSubmatch(html); len(m) > 2 {
			un := strings.TrimSpace(m[2])
			if !strings.Contains(un, "<") && len(un) > 0 && len(un) < 30 {
				if cleaned := cleanUsername(un); cleaned != "" {
					result.Username = cleaned
				}
			}
		}
	}

	if strings.HasPrefix(result.Username, "UID:") {
		if m := reDetailH1.FindStringSubmatch(html); len(m) > 1 {
			h1Text := cleanText(m[1])
			h1Text = strings.TrimSpace(h1Text)
			if h1Text != "" && len(h1Text) < 50 && !strings.Contains(h1Text, "匿名") {
				if !strings.Contains(h1Text, "详情") && !strings.Contains(h1Text, "Home") {
					result.Username = h1Text
				}
			}
		}
	}

	if result.Ratio == 0 && result.DownloadBytes > 0 && result.UploadBytes > 0 {
		result.Ratio = float64(result.UploadBytes) / float64(result.DownloadBytes)
	}

	if result.SeedingSize == 0 {
		if m := reDetailSeedingSize.FindStringSubmatch(html); len(m) > 1 {
			if sz := parseSizeString(cleanText(m[1])); sz > 0 {
				result.SeedingSize = sz
			}
		}
	}
	if result.SeedingSize == 0 {
		if m := reDetailArrowUpSize.FindStringSubmatch(html); len(m) > 1 {
			if sz := parseSizeString(m[1]); sz > 0 {
				result.SeedingSize = sz
			}
		}
	}

	if result.SeedingCount == 0 || result.SeedingSize == 0 {
		a.fetchSeedingFromAJAX(ctx, config, uid, result)
	}
	if result.SeedingSize == 0 {
		a.fetchSeedingSizeFromBonus(ctx, config, result)
	}
}

func (a *NexusPHPAdapter) fetchSeedingFromAJAX(ctx context.Context, config *model.SiteConfig, uid string, result *model.UserStatsResult) {
	var ajaxURL string
	if strings.HasPrefix(uid, "uuid:") {
		uuid := strings.TrimPrefix(uid, "uuid:")
		ajaxURL = config.Domain + "/getusertorrentlistajax.php?useruuid=" + uuid + "&type=seeding"
	} else {
		ajaxURL = config.Domain + "/getusertorrentlistajax.php?userid=" + uid + "&type=seeding"
	}
	req, err := http.NewRequestWithContext(ctx, "GET", ajaxURL, nil)
	if err != nil {
		return
	}
	setCommonHeaders(req, config.Cookie)
	if strings.HasPrefix(uid, "uuid:") {
		req.Header.Set("Referer", config.Domain+"/userdetails.php?uuid="+strings.TrimPrefix(uid, "uuid:"))
	} else {
		req.Header.Set("Referer", config.Domain+"/userdetails.php?id="+uid)
	}
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return
	}
	defer httpclient.DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	html := string(body)
	if !strings.Contains(html, "<table") && !strings.Contains(html, "条记录") {
		return
	}

	ajaxText := strings.ReplaceAll(html, "&nbsp;", " ")
	ajaxText = strings.ReplaceAll(ajaxText, "&amp;", "&")

	if m := reAJAXSummary.FindStringSubmatch(ajaxText); len(m) > 2 {
		if summaryCount, _ := strconv.Atoi(m[1]); summaryCount > 0 && result.SeedingCount == 0 {
			result.SeedingCount = summaryCount
		}
		if sz := parseSizeString(m[2]); sz > 0 && result.SeedingSize == 0 {
			result.SeedingSize = sz
			return
		}
	}

	if !strings.Contains(html, "<table") {
		return
	}

	reTR := regexp.MustCompile(`(?is)<tr[^>]*>(.*?)</tr>`)
	reTD := regexp.MustCompile(`(?is)<td[^>]*>(.*?)</td>`)
	reSizeInTD := regexp.MustCompile(`(?i)([\d.,]+\s*(?:PiB|TiB|GiB|MiB|KiB|TB|GB|MB|KB))`)

	trMatches := reTR.FindAllStringSubmatch(html, -1)
	if len(trMatches) <= 1 {
		return
	}

	count := 0
	var totalSize int64
	for _, tr := range trMatches[1:] {
		tdMatches := reTD.FindAllStringSubmatch(tr[1], -1)
		count++
		for _, td := range tdMatches {
			tdText := cleanText(td[1])
			if sm := reSizeInTD.FindStringSubmatch(tdText); len(sm) > 1 {
				totalSize += parseSizeString(sm[1])
				break
			}
		}
	}

	if count > 0 && result.SeedingCount == 0 {
		result.SeedingCount = count
	}
	if totalSize > 0 && result.SeedingSize == 0 {
		result.SeedingSize = totalSize
	}
}

func isSiteTotalMatch(search string, m []string) bool {
	if len(m) == 0 {
		return false
	}
	idx := strings.Index(search, m[0])
	if idx < 0 {
		return false
	}
	prefix := search[:idx]
	return strings.HasSuffix(prefix, "总")
}

var reAnonSuffix = regexp.MustCompile(`\s*\(匿名[^)]*\)\s*$`)

func cleanUsername(name string) string {
	name = strings.TrimSpace(name)
	name = reAnonSuffix.ReplaceAllString(name, "")
	name = strings.TrimSpace(name)
	return name
}

func isValidUserClass(class string) string {
	class = strings.TrimSpace(class)
	if class == "" {
		return ""
	}
	if _, err := strconv.Atoi(class); err == nil {
		return ""
	}
	if strings.HasPrefix(class, "UID") {
		return ""
	}
	return class
}

func (a *NexusPHPAdapter) resolveUIDFromSelfProfile(ctx context.Context, config *model.SiteConfig) string {
	pageURL := config.Domain + "/userdetails"
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return ""
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return ""
	}
	defer httpclient.DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	html := string(body)
	if m := reUserIDLoose.FindStringSubmatch(html); len(m) > 1 {
		return m[1]
	}
	if m := reIDField.FindStringSubmatch(html); len(m) > 1 {
		return m[1]
	}
	return ""
}

func (a *NexusPHPAdapter) fetchSeedingSizeFromBonus(ctx context.Context, config *model.SiteConfig, result *model.UserStatsResult) {
	pageURL := config.Domain + "/mybonus.php"
	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return
	}
	defer httpclient.DrainBody(resp)

	if resp.StatusCode != http.StatusOK {
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	html := string(body)
	if m := reBonusSeedingSize.FindStringSubmatch(html); len(m) > 1 {
		if sz := parseSizeString(m[1]); sz > 0 {
			result.SeedingSize = sz
		}
	}
}
