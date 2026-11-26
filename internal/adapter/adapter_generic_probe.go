package adapter

import (
	"context"
	"net/http"
	"strings"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
)

func (a *GenericAdapter) DetectDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	if config.Discount.HasAPI && config.Discount.APIURL != "" {
		return a.detectDiscountGenericAPI(ctx, config, torrentID)
	}
	return a.detectDiscountGenericPage(ctx, config, torrentID)
}

func (a *GenericAdapter) detectDiscountGenericAPI(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	u := buildGenericURL(config, config.Discount.APIURL, torrentID)
	if u == "" {
		u = config.Domain + strings.Replace(config.Discount.APIURL, "{id}", torrentID, 1)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造请求失败", err)
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

	if strings.Contains(html, "pro_free2up") || strings.Contains(html, "2xfree") {
		return &model.DiscountResult{Level: model.Discount2xFree, Multiplier: 2.0}, nil
	}
	if strings.Contains(html, "pro_2up") || strings.Contains(html, "2xup") {
		return &model.DiscountResult{Level: model.Discount2xUp, Multiplier: 2.0}, nil
	}
	if strings.Contains(html, "pro_free") || strings.Contains(html, "免费") || strings.Contains(html, "free") {
		return &model.DiscountResult{Level: model.DiscountFree}, nil
	}
	if strings.Contains(html, "pro_50p") || strings.Contains(html, "50%") {
		return &model.DiscountResult{Level: model.DiscountPercent50}, nil
	}
	if strings.Contains(html, "pro_30p") || strings.Contains(html, "30%") {
		return &model.DiscountResult{Level: model.DiscountPercent30}, nil
	}

	return &model.DiscountResult{Level: model.DiscountNone}, nil
}

func (a *GenericAdapter) detectDiscountGenericPage(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	u := buildGenericURL(config, config.Paths.Detail, torrentID)
	if u == "" {
		return &model.DiscountResult{Level: model.DiscountNone}, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("请求优惠信息失败", err)
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

func (a *GenericAdapter) fetchSeedingSizeFromBonusHour(ctx context.Context, config *model.SiteConfig, result *model.UserStatsResult) {
	pageURL := config.Domain + "/p_bonus/bonus_hour.php"
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

	body, err := httpclient.ReadBody(resp)
	if err != nil {
		return
	}

	html := string(body)
	if m := reBonusHourTotalSize.FindStringSubmatch(html); len(m) > 1 {
		sizeStr := strings.TrimSpace(m[1])
		if strings.HasSuffix(sizeStr, " T") {
			sizeStr = sizeStr[:len(sizeStr)-2] + " TB"
		} else if strings.HasSuffix(sizeStr, " G") {
			sizeStr = sizeStr[:len(sizeStr)-2] + " GB"
		} else if strings.HasSuffix(sizeStr, " M") {
			sizeStr = sizeStr[:len(sizeStr)-2] + " MB"
		}
		if sz := parseSizeString(sizeStr); sz > 0 {
			result.SeedingSize = sz
		}
	}
}

func (a *GenericAdapter) DetectHR(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
	u := buildGenericURL(config, config.Paths.Detail, torrentID)
	if u == "" {
		return &model.HRResult{HasHR: false}, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("请求HR信息失败", err)
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

func (a *GenericAdapter) DetectHRAndDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, *model.DiscountResult, error) {
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

	u := buildGenericURL(config, config.Paths.Detail, torrentID)
	if u == "" {
		return &model.HRResult{HasHR: false}, &model.DiscountResult{Level: model.DiscountNone}, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, networkError("构造请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
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
