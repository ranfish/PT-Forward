package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

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

// DetectHRDiscountAndSL 在 detect 阶段顺便提取 SL 数据（§55.19 根本修复）。
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
