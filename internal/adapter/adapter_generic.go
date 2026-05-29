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

var (
	reGenericTitle             = regexp.MustCompile(`<title>([^<]+)</title>`)
	reGenericRow               = regexp.MustCompile(`(?s)<tr[^>]*>.*?</tr>`)
	reGenericDDBlock           = regexp.MustCompile(`(?s)基本信息</dt>\s*<dd>(.*)</dd>`)
	reGenericFallbackDiv       = regexp.MustCompile(`(?s)基本信息</div>(.*?)种子文件`)
	reGenericInfoHash          = regexp.MustCompile(`(?i)info_hash.*?([a-fA-F0-9]{40})`)
	reGenericKdescr            = regexp.MustCompile(`(?s)<div[^>]*id=['"]kdescr['"][^>]*>([\s\S]*?)</div>`)
	reGenericSeeders           = regexp.MustCompile(`(?i)(?:做种数|Seeders?|S[^<]*<[^>]*>)(\d+)`)
	reGenericLeechers          = regexp.MustCompile(`(?i)(?:下载数|Leechers?|L[^<]*<[^>]*>)(\d+)`)
	reGenericDetailID          = regexp.MustCompile(`(?:details|detail|torrent)\.php\?id=(\d+)`)
	reGenericErrorClass        = regexp.MustCompile(`class="error"[^>]*>([^<]+)`)
	reGenericStripTags         = regexp.MustCompile(`<[^>]+>`)
	reGenericBrowseRow         = regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	reGenericBrowseDetailLink  = regexp.MustCompile(`(?s)href="[^"]*(?:details?|torrent)[^"]*id=(\d+)[^"]*"[^>]*>(.*?)</a>`)
	reGenericBrowseSize        = regexp.MustCompile(`(?i)([\d.]+)\s*(TB|GB|MB|KB)`)
	reGenericBrowseSeeders     = regexp.MustCompile(`>(\d+)</a>\s*</td>\s*$`)
	reGenericBrowseLeechers    = regexp.MustCompile(`(\d+)\s*</td>\s*$`)
	reGenericDigits            = regexp.MustCompile(`(\d+)`)
	reGenericTTGID             = regexp.MustCompile(`/t/(\d+)`)
	reGenericStarSpaceDetailID = regexp.MustCompile(`(?:details|torrent)\.php\?id=(\d+)`)
	reGenericIMDbID            = regexp.MustCompile(`tt\d+`)
)

type GenericAdapter struct {
	framework string
	doer      *HTTPDoer
	logger    *zap.Logger
}

func NewGenericAdapter(framework string, doer *HTTPDoer, logger *zap.Logger) *GenericAdapter {
	return &GenericAdapter{framework: framework, doer: doer, logger: logger}
}

func (a *GenericAdapter) Framework() string { return a.framework }

func (a *GenericAdapter) ParseRSS(_ context.Context, _ string, _ *model.SiteConfig) ([]*model.RSSTorrentEvent, error) {
	return nil, parseError("ParseRSS: use RSS fetcher instead", nil)
}

func (a *GenericAdapter) DownloadTorrent(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	u := buildGenericURL(config, config.Paths.Detail, torrentID)
	if u == "" {
		u = buildGenericDownloadURL(config, torrentID)
	}
	if u == "" {
		return nil, configError(fmtES("no download URL configured for %s", a.framework))
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

	if resp.StatusCode == http.StatusForbidden {
		return nil, &model.AppError{Code: 14003, Message: "403 Forbidden: cookie 可能已过期"}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("HTTP %d", resp.StatusCode), nil)
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		return nil, &model.AppError{Code: 15001, Message: "返回了 HTML 页面而非种子文件，下载链接可能有误"}
	}

	return io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
}

func (a *GenericAdapter) GetTorrentDetail(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	u := buildGenericURL(config, config.Paths.Detail, torrentID)
	if u == "" {
		return nil, configError("未配置详情页路径 (paths.detail)")
	}

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

	html := string(body)
	detail := &model.TorrentDetail{}

	if m := reGenericTitle.FindStringSubmatch(html); len(m) > 1 {
		detail.Title = strings.TrimSpace(m[1])
	}

	for _, row := range reGenericRow.FindAllString(html, -1) {
		rowText := cleanRowText(row)

		if (strings.Contains(rowText, "类别") || strings.Contains(rowText, "Category")) && !strings.Contains(rowText, "基本信息") {
			extractCategoryFromImg(row, detail)
		}

		if strings.Contains(rowText, "基本信息") || strings.Contains(rowText, "基本") {
			extractBasicInfoFields(row, detail)
		}

		if strings.Contains(rowText, "副标题") && !strings.Contains(rowText, "基本信息") {
			extractSubtitleFromRow(row, detail)
		}

		if (strings.Contains(rowText, "种子文件") || strings.Contains(rowText, "种子信息")) && detail.InfoHash == "" {
			if h := extractInfoHashFromRow(row); h != "" {
				detail.InfoHash = h
			}
		}
	}

	if detail.Size == 0 || detail.Category == "" {
		if m := reGenericDDBlock.FindStringSubmatch(html); len(m) > 1 {
			extractBasicInfoFields(m[1], detail)
		}
		if m := reGenericFallbackDiv.FindStringSubmatch(html); len(m) > 1 {
			extractBasicInfoFields(m[1], detail)
		}
	}

	if detail.InfoHash == "" {
		for _, re := range hashRowPatterns {
			if m := re.FindStringSubmatch(html); len(m) > 1 {
				detail.InfoHash = strings.ToLower(m[1])
				break
			}
		}
	}

	if detail.InfoHash == "" {
		if m := reGenericInfoHash.FindStringSubmatch(html); len(m) > 1 {
			detail.InfoHash = strings.ToLower(m[1])
		}
	}

	if m := reGenericKdescr.FindStringSubmatch(html); len(m) > 1 {
		detail.Description = strings.TrimSpace(m[1])
	}

	detail.Tags = extractTags(html)

	detail.Category = NormalizeCategory(detail.Category)

	return detail, nil
}

func (a *GenericAdapter) GetBatchSLData(ctx context.Context, config *model.SiteConfig, torrentIDs []string) (map[string]*model.SLData, error) {
	result := make(map[string]*model.SLData, len(torrentIDs))
	for _, id := range torrentIDs {
		sl, err := a.GetPreciseSLData(ctx, config, id)
		if err != nil {
			a.logger.Warn("获取SL数据失败", zap.String("torrentID", id), zap.Error(err))
			continue
		}
		result[id] = sl
	}
	return result, nil
}

func (a *GenericAdapter) GetPreciseSLData(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	u := buildGenericURL(config, config.Paths.Detail, torrentID)
	if u == "" {
		return nil, configError("未配置详情页路径 (paths.detail)")
	}

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

	html := string(body)
	sl := &model.SLData{}

	seedersRe := reGenericSeeders
	if m := seedersRe.FindStringSubmatch(html); len(m) > 1 {
		sl.Seeders, _ = strconv.Atoi(m[1])
	}

	leechersRe := reGenericLeechers
	if m := leechersRe.FindStringSubmatch(html); len(m) > 1 {
		sl.Leechers, _ = strconv.Atoi(m[1])
	}

	return sl, nil
}

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

	result := DetectDiscountFromHTML(html, &config.DiscountDetection)
	if result.Level != model.DiscountNone {
		return result, nil
	}

	return &model.DiscountResult{Level: model.DiscountNone}, nil
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

	discResult := DetectDiscountFromHTML(html, &config.DiscountDetection)

	return hrResult, discResult, nil
}

func (a *GenericAdapter) UploadTorrent(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	domain := strings.ToLower(config.Domain)

	switch {
	case strings.Contains(domain, "totheglory"):
		return a.uploadTTG(ctx, config, req)
	case strings.Contains(domain, "star-space"):
		return a.uploadStarSpace(ctx, config, req)
	case strings.Contains(domain, "yemapt"):
		return a.uploadYemaPT(ctx, config, req)
	}

	return a.uploadGeneric(ctx, config, req)
}

func (a *GenericAdapter) uploadGeneric(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	uploadURL := buildGenericURL(config, config.Paths.Upload, "")
	if uploadURL == "" {
		return nil, configError("未配置上传路径 (paths.upload)")
	}

	if len(req.TorrentData) == 0 {
		return nil, &model.AppError{Code: 40001, Message: "种子文件数据为空"}
	}

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
	if req.Description != "" {
		fw.writeField("descr", req.Description)
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

	if resp.StatusCode == http.StatusForbidden {
		return nil, &model.AppError{Code: 14003, Message: "403 Forbidden: 权限不足"}
	}

	if idMatch := reGenericDetailID.FindStringSubmatch(html); len(idMatch) > 1 {
		torrentID := idMatch[1]
		return &model.PublishResponse{
			Success:    true,
			TorrentID:  torrentID,
			DetailURL:  config.Domain + "/details.php?id=" + torrentID,
			TargetSite: config.Domain,
		}, nil
	}

	if strings.Contains(html, "成功") || strings.Contains(html, "succeeded") || strings.Contains(html, "uploaded") {
		return &model.PublishResponse{Success: true, TargetSite: config.Domain}, nil
	}

	errMsg := "上传失败: 未知响应"
	if m := reGenericErrorClass.FindStringSubmatch(html); len(m) > 1 {
		errMsg = strings.TrimSpace(m[1])
	}

	return nil, &model.AppError{Code: 15001, Message: errMsg}
}

func (a *GenericAdapter) SearchTorrents(ctx context.Context, config *model.SiteConfig, keyword string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
	u := buildGenericURL(config, config.Paths.Browse, "")
	if u == "" {
		return nil, configError("未配置浏览页路径 (paths.browse)")
	}

	if strings.Contains(u, "?") {
		u += "&search=" + url.QueryEscape(keyword)
	} else {
		u += "?search=" + url.QueryEscape(keyword)
	}
	if opts != nil && opts.Category != "" {
		u += "&cat=" + opts.Category
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, searchError("构造搜索请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, searchError("搜索请求失败", err)
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("HTTP %d", resp.StatusCode), nil)
	}

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	return parseGenericBrowse(string(body), config), nil
}

func (a *GenericAdapter) GetTorrentInfoHash(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	detail, err := a.GetTorrentDetail(ctx, config, torrentID)
	if err != nil {
		return "", err
	}
	if detail.InfoHash == "" {
		return "", notFoundError("未在详情页找到 info_hash")
	}
	return detail.InfoHash, nil
}

func (a *GenericAdapter) SupportsSearchByPiecesHash() bool { return false }

func (a *GenericAdapter) VerifyExists(ctx context.Context, config *model.SiteConfig, torrentID string) (bool, error) {
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

func buildGenericURL(config *model.SiteConfig, pathTpl, torrentID string) string {
	if pathTpl == "" {
		return ""
	}
	u := pathTpl
	if torrentID != "" {
		u = strings.ReplaceAll(u, "{id}", url.PathEscape(torrentID))
	}
	u = strings.ReplaceAll(u, "{passkey}", url.QueryEscape(config.Passkey))
	u = strings.ReplaceAll(u, "{authkey}", url.QueryEscape(config.AuthKey))
	if !strings.HasPrefix(u, "http") {
		base := config.Domain
		if !strings.HasPrefix(base, "http") {
			base = "https://" + base
		}
		u = base + "/" + strings.TrimLeft(u, "/")
	}
	return u
}

func buildGenericDownloadURL(config *model.SiteConfig, torrentID string) string {
	base := config.Domain
	if !strings.HasPrefix(base, "http") {
		base = "https://" + base
	}
	if config.Passkey != "" {
		return base + "/download.php?id=" + url.QueryEscape(torrentID) + "&passkey=" + url.QueryEscape(config.Passkey)
	}
	return base + "/download.php?id=" + url.QueryEscape(torrentID)
}

func stripTags(s string) string {
	return reGenericStripTags.ReplaceAllString(s, "")
}

func parseGenericBrowse(html string, config *model.SiteConfig) []*model.SeedingSearchResult {
	var results []*model.SeedingSearchResult

	rowRe := reGenericBrowseRow
	detailLinkRe := reGenericBrowseDetailLink
	sizeRe := reGenericBrowseSize
	seedersRe := reGenericBrowseSeeders
	leechersRe := reGenericBrowseLeechers

	rows := rowRe.FindAllString(html, -1)
	for _, row := range rows {
		linkMatch := detailLinkRe.FindStringSubmatch(row)
		if len(linkMatch) < 3 {
			continue
		}

		torrentID := linkMatch[1]
		title := stripTags(strings.TrimSpace(linkMatch[2]))

		if len(results) > 0 && results[len(results)-1].TorrentID == torrentID {
			continue
		}

		result := &model.SeedingSearchResult{
			TorrentID: torrentID,
			Title:     title,
			DetailURL: config.Domain + "/details.php?id=" + torrentID,
		}

		if m := sizeRe.FindStringSubmatch(row); len(m) > 2 {
			result.Size = parseSizeStr(m[1] + " " + m[2])
		}

		if m := seedersRe.FindStringSubmatch(row); len(m) > 1 {
			result.Seeders, _ = strconv.Atoi(m[1])
		}

		if m := leechersRe.FindStringSubmatch(row); len(m) > 1 {
			result.Leechers, _ = strconv.Atoi(m[1])
		}

		result.PublishAt = time.Now()
		results = append(results, result)

		if len(results) >= 50 {
			break
		}
	}

	return results
}

func (a *GenericAdapter) uploadTTG(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	if len(req.TorrentData) == 0 {
		return nil, &model.AppError{Code: 40001, Message: "种子文件数据为空"}
	}

	baseURL := resolveBase(config)
	uploadURL := baseURL + "/upload.php"
	if config.Paths.Upload != "" {
		uploadURL = baseURL + config.Paths.Upload
	}

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
		fw.writeField("subtitle", req.Subtitle)
	}
	if req.Description != "" {
		fw.writeField("descr", req.Description)
	}
	if req.IMDbLink != "" {
		fw.writeField("imdb_c", req.IMDbLink)
	}
	if req.DoubanLink != "" {
		m := reGenericDigits.FindString(req.DoubanLink)
		if m != "" {
			fw.writeField("douban_id", m)
		}
	}
	if req.Anonymous {
		fw.writeField("anonymity", "yes")
	}

	if v, ok := req.FormFields["cat"]; ok {
		fw.writeField("type", v)
	}
	if v, ok := req.FormFields["tags"]; ok {
		if strings.Contains(v, "禁转") {
			fw.writeField("nodistr", "yes")
		}
	}

	fw.writeField("MAX_FILE_SIZE", "4000000")
	fw.writeField("team", "")
	fw.writeField("hr", "no")

	for k, v := range req.FormFields {
		switch k {
		case "cat", "tags":
			continue
		}
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

	if resp.StatusCode == http.StatusForbidden {
		return nil, &model.AppError{Code: 14003, Message: "403 Forbidden: 权限不足"}
	}

	if idMatch := reGenericTTGID.FindStringSubmatch(html); len(idMatch) > 1 {
		torrentID := idMatch[1]
		return &model.PublishResponse{
			Success:    true,
			TorrentID:  torrentID,
			DetailURL:  baseURL + "/t/" + torrentID + "/",
			TargetSite: config.Domain,
		}, nil
	}

	if strings.Contains(html, "成功") || strings.Contains(html, "uploaded") {
		return &model.PublishResponse{Success: true, TargetSite: config.Domain}, nil
	}

	errMsg := "上传失败"
	if m := reGenericErrorClass.FindStringSubmatch(html); len(m) > 1 {
		errMsg = strings.TrimSpace(m[1])
	}

	return nil, &model.AppError{Code: 15001, Message: errMsg}
}

func (a *GenericAdapter) uploadStarSpace(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	if len(req.TorrentData) == 0 {
		return nil, &model.AppError{Code: 40001, Message: "种子文件数据为空"}
	}

	isMusic := false
	if cat, ok := req.FormFields["cat"]; ok {
		if cat == "music" {
			isMusic = true
		}
	}
	if musicFlag, ok := req.ExtraFields["is_music"]; ok && musicFlag == "true" {
		isMusic = true
	}

	if isMusic {
		return a.uploadStarSpaceMusic(ctx, config, req)
	}
	return a.uploadStarSpaceVideo(ctx, config, req)
}

func (a *GenericAdapter) uploadStarSpaceVideo(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	baseURL := resolveBase(config)
	uploadURL := baseURL + "/p_torrent/video_upload.php"
	if config.Paths.Upload != "" {
		uploadURL = baseURL + config.Paths.Upload
	}

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

	fw.writeField("tid", "0")

	if req.Title != "" {
		fw.writeField("name", req.Title)
	}
	if req.Subtitle != "" {
		fw.writeField("small_desc", req.Subtitle)
	}
	if req.Description != "" {
		fw.writeField("descr", req.Description)
	}
	if req.DoubanLink != "" {
		fw.writeField("douban_url", req.DoubanLink)
	}
	if req.IMDbLink != "" {
		fw.writeField("imdb_url", req.IMDbLink)
	}
	if len(req.Screenshots) > 0 {
		fw.writeField("screen", strings.Join(req.Screenshots, "\n"))
	}

	fireflyFieldMap := map[string]string{
		"cat":            "tr_category",
		"medium_sel":     "tr_source",
		"codec_sel":      "tr_video_codec",
		"audiocodec_sel": "tr_audio_codec",
		"standard_sel":   "tr_resolution",
		"team_sel":       "tr_team",
	}
	for srcType, targetName := range fireflyFieldMap {
		if v, ok := req.FormFields[srcType]; ok {
			fw.writeField(targetName, v)
		}
	}

	if hdr, ok := req.ExtraFields["tr_hdr"]; ok {
		fw.writeField("tr_hdr", hdr)
	}

	if tags, ok := req.FormFields["tags"]; ok {
		tagMap := map[string]string{
			"官方": "tag_gf", "驻站组": "tag_xiaozu", "禁转": "tag_jz",
			"3D": "tag_3d", "中字": "tag_chs_sub", "国语": "tag_chs_lang",
			"粤语": "tag_yueyu", "英字": "tag_eng_sub", "英语": "tag_eng_lang",
			"分集": "tag_ep", "完结": "tag_complete",
		}
		for tagLabel, tagName := range tagMap {
			if strings.Contains(tags, tagLabel) {
				fw.writeField(tagName, "on")
			}
		}
	}

	for k, v := range req.FormFields {
		switch k {
		case "cat", "medium_sel", "codec_sel", "audiocodec_sel", "standard_sel", "team_sel", "tags":
			continue
		}
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

	return a.doStarSpaceUpload(httpReq, config)
}

func (a *GenericAdapter) uploadStarSpaceMusic(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	baseURL := resolveBase(config)
	uploadURL := baseURL + "/p_music/music_upload.php"

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

	fw.writeField("tid", "0")

	if v, ok := req.ExtraFields["artist"]; ok {
		fw.writeField("artist", v)
	}
	if req.Title != "" {
		fw.writeField("title", req.Title)
	}
	if v, ok := req.ExtraFields["year"]; ok {
		fw.writeField("year", v)
	}
	if v, ok := req.ExtraFields["image"]; ok {
		fw.writeField("image", v)
	}
	if req.Description != "" {
		fw.writeField("release_desc", req.Description)
	}

	if v, ok := req.FormFields["cat"]; ok {
		fw.writeField("release_type", v)
	}
	if v := resolveField(req.FormFields, "category", "cat"); v != "" {
		fw.writeField("release_type", v)
	}
	if v := resolveField(req.FormFields, "codec_sel", "codec"); v != "" {
		fw.writeField("format", v)
	}
	if v := resolveField(req.FormFields, "audiocodec_sel", "audioCodec"); v != "" {
		fw.writeField("bitrate", v)
	}
	if v := resolveField(req.FormFields, "medium_sel", "medium"); v != "" {
		fw.writeField("media", v)
	}

	if v, ok := req.ExtraFields["remaster_year"]; ok {
		fw.writeField("remaster_year", v)
	}
	if v, ok := req.ExtraFields["remaster_title"]; ok {
		fw.writeField("remaster_title", v)
	}
	if v, ok := req.ExtraFields["remaster_record_label"]; ok {
		fw.writeField("remaster_record_label", v)
	}
	if v, ok := req.ExtraFields["remaster_catalogue_number"]; ok {
		fw.writeField("remaster_catalogue_number", v)
	}

	for k, v := range req.ExtraFields {
		switch k {
		case "artist", "year", "image", "remaster_year", "remaster_title",
			"remaster_record_label", "remaster_catalogue_number":
			continue
		}
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

	return a.doStarSpaceUpload(httpReq, config)
}

func (a *GenericAdapter) doStarSpaceUpload(httpReq *http.Request, config *model.SiteConfig) (*model.PublishResponse, error) {
	resp, err := a.doer.Client.Do(httpReq) //nolint:gosec // URL from admin-configured site base URL
	if err != nil {
		return nil, uploadError("上传请求失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := string(body)

	if resp.StatusCode == http.StatusForbidden {
		return nil, &model.AppError{Code: 14003, Message: "403 Forbidden"}
	}

	if idMatch := reGenericStarSpaceDetailID.FindStringSubmatch(html); len(idMatch) > 1 {
		torrentID := idMatch[1]
		return &model.PublishResponse{
			Success:    true,
			TorrentID:  torrentID,
			DetailURL:  config.Domain + "/details.php?id=" + torrentID,
			TargetSite: config.Domain,
		}, nil
	}

	if strings.Contains(html, "成功") || strings.Contains(html, "succeeded") {
		return &model.PublishResponse{Success: true, TargetSite: config.Domain}, nil
	}

	errMsg := "上传失败"
	if m := reGenericErrorClass.FindStringSubmatch(html); len(m) > 1 {
		errMsg = strings.TrimSpace(m[1])
	}

	return nil, &model.AppError{Code: 15001, Message: errMsg}
}

func (a *GenericAdapter) uploadYemaPT(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	if len(req.TorrentData) == 0 {
		return nil, &model.AppError{Code: 40001, Message: "种子文件数据为空"}
	}

	baseURL := resolveBase(config)
	uploadURL := baseURL + "/api/torrent/add"
	if config.Paths.Upload != "" {
		uploadURL = baseURL + config.Paths.Upload
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	fw := newFieldWriter(writer)

	fileWriter, err := writer.CreateFormFile("files", "upload.torrent")
	if err != nil {
		return nil, uploadError("创建表单文件字段失败", err)
	}
	if _, err := fileWriter.Write(req.TorrentData); err != nil {
		return nil, networkError("写入种子数据失败", err)
	}

	if req.Title != "" {
		fw.writeField("showName", req.Title)
	}
	if req.Subtitle != "" {
		fw.writeField("shortDesc", req.Subtitle)
	}
	if req.Description != "" {
		fw.writeField("descr", req.Description)
	}
	if req.Anonymous {
		fw.writeField("uploadUserAnonymous", "是")
	}

	yemaFieldMap := map[string]string{
		"cat":            "categoryId",
		"medium_sel":     "medium",
		"codec_sel":      "codec",
		"audiocodec_sel": "audiocodec",
		"standard_sel":   "standard",
		"processing_sel": "region",
		"team_sel":       "team",
	}
	for srcType, targetName := range yemaFieldMap {
		if v, ok := req.FormFields[srcType]; ok {
			fw.writeField(targetName, v)
		}
	}

	if tags, ok := req.FormFields["tags"]; ok {
		tagList := strings.Split(tags, ",")
		for _, tag := range tagList {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				fw.writeField("tagList", tag)
			}
		}
	}

	if imdbID := extractIMDbIDGeneric(req.IMDbLink); imdbID != "" {
		fw.writeField("imdb", imdbID)
	}
	if req.DoubanLink != "" {
		m := reGenericDigits.FindString(req.DoubanLink)
		if m != "" {
			fw.writeField("douban", m)
		}
	}
	if v, ok := req.ExtraFields["picture"]; ok {
		fw.writeField("picture", v)
	}

	for k, v := range req.FormFields {
		switch k {
		case "cat", "medium_sel", "codec_sel", "audiocodec_sel", "standard_sel",
			"processing_sel", "team_sel", "tags":
			continue
		}
		fw.writeField(k, v)
	}
	for k, v := range req.ExtraFields {
		if k == "picture" {
			continue
		}
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

	var result struct {
		Success  bool        `json:"success"`
		ShowType int         `json:"showType"`
		Data     interface{} `json:"data"`
		Message  string      `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err == nil {
		if result.Success {
			dataMap, ok := result.Data.(map[string]interface{})
			torrentID := ""
			if ok {
				if id, ok := dataMap["id"]; ok {
					torrentID = fmt.Sprintf("%v", id)
				}
			}
			detailURL := ""
			if torrentID != "" {
				detailURL = baseURL + "/torrent/detail/" + torrentID
			}
			return &model.PublishResponse{
				Success:    true,
				TorrentID:  torrentID,
				DetailURL:  detailURL,
				TargetSite: config.Domain,
			}, nil
		}
		errMsg := result.Message
		if errMsg == "" {
			errMsg = fmt.Sprintf("上传失败: showType=%d", result.ShowType)
		}
		return nil, &model.AppError{Code: 15001, Message: errMsg}
	}

	html := string(body)
	if strings.Contains(html, "成功") || strings.Contains(html, "success") {
		return &model.PublishResponse{Success: true, TargetSite: config.Domain}, nil
	}

	return nil, &model.AppError{Code: 15001, Message: fmt.Sprintf("上传失败: HTTP %d", resp.StatusCode)}
}

func extractIMDbIDGeneric(link string) string {
	if link == "" {
		return ""
	}
	m := reGenericIMDbID.FindString(link)
	return m
}

func (a *GenericAdapter) FetchUserStats(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	pageURL := 	config.Domain + "/index.php"
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

	if strings.Contains(html, "login.php") && !strings.Contains(html, "userdetails") {
		return nil, fmt.Errorf("cookie 无效或已过期")
	}

	result := &model.UserStatsResult{}
	if m := reNexusUsername.FindStringSubmatch(html); len(m) > 2 {
		result.Username = strings.TrimSpace(m[2])
	} else if m := reNexusUsernameSpan.FindStringSubmatch(html); len(m) > 2 {
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
	if m := reNexusFontRatio.FindStringSubmatch(html); len(m) > 1 {
		result.Ratio, _ = strconv.ParseFloat(cleanText(m[1]), 64)
	} else if m := reNexusLabelRatio.FindStringSubmatch(html); len(m) > 1 {
		result.Ratio, _ = strconv.ParseFloat(cleanText(m[1]), 64)
	}
	if result.Ratio == 0 && result.DownloadBytes > 0 && result.UploadBytes > 0 {
		result.Ratio = float64(result.UploadBytes) / float64(result.DownloadBytes)
	}
	return result, nil
}
