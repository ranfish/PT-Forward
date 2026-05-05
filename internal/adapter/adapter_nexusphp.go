package adapter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type NexusPHPAdapter struct {
	doer   *HTTPDoer
	logger *zap.Logger
}

func NewNexusPHPAdapter(doer *HTTPDoer, logger *zap.Logger) *NexusPHPAdapter {
	return &NexusPHPAdapter{doer: doer, logger: logger}
}

func (a *NexusPHPAdapter) Framework() string { return "nexusphp" }

func (a *NexusPHPAdapter) ParseRSS(_ context.Context, _ string, _ *model.SiteConfig) ([]*model.RSSTorrentEvent, error) {
	return nil, fmt.Errorf("ParseRSS: use RSS fetcher instead")
}

func (a *NexusPHPAdapter) DownloadTorrent(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	u := buildURL(config.Domain, "/download.php", torrentID, config.Passkey)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("403 Forbidden: cookie 可能已过期")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		return nil, fmt.Errorf("返回了 HTML 页面而非种子文件，下载链接可能有误")
	}

	return io.ReadAll(resp.Body)
}

func (a *NexusPHPAdapter) GetTorrentDetail(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	u := buildURL(config.Domain, "/details.php", torrentID, "")

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求详情页失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	html := string(body)
	detail := &model.TorrentDetail{}

	if m := regexp.MustCompile(`<title>([^<]+)</title>`).FindStringSubmatch(html); len(m) > 1 {
		detail.Title = strings.TrimSpace(m[1])
	}

	if m := regexp.MustCompile(`(?i)info_hash.*?<td[^>]*>([a-fA-F0-9]{40})`).FindStringSubmatch(html); len(m) > 1 {
		detail.InfoHash = strings.ToLower(m[1])
	}

	if m := regexp.MustCompile(`(?i)(?:大小|Size)[^<]*<[^>]*>([^<]+)`).FindStringSubmatch(html); len(m) > 1 {
		detail.Size = parseSizeStr(m[1])
	}

	if m := regexp.MustCompile(`(?i)(?:分类|Category)[^<]*<[^>]*>([^<]+)`).FindStringSubmatch(html); len(m) > 1 {
		detail.Category = strings.TrimSpace(m[1])
	}

	detail.Tags = extractTags(html)

	return detail, nil
}

func (a *NexusPHPAdapter) GetBatchSLData(ctx context.Context, config *model.SiteConfig, torrentIDs []string) (map[string]*model.SLData, error) {
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

func (a *NexusPHPAdapter) GetPreciseSLData(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.SLData, error) {
	u := buildURL(config.Domain, "/details.php", torrentID, "")

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求详情页失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	html := string(body)
	sl := &model.SLData{}

	seedersRe := regexp.MustCompile(`(?i)(?:做种数|Seeders?)[^<]*<[^>]*>(\d+)`)
	if m := seedersRe.FindStringSubmatch(html); len(m) > 1 {
		sl.Seeders, _ = strconv.Atoi(m[1])
	}

	leechersRe := regexp.MustCompile(`(?i)(?:下载数|Leechers?)[^<]*<[^>]*>(\d+)`)
	if m := leechersRe.FindStringSubmatch(html); len(m) > 1 {
		sl.Leechers, _ = strconv.Atoi(m[1])
	}

	return sl, nil
}

func (a *NexusPHPAdapter) DetectDiscount(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	if config.Discount.HasAPI && config.Discount.APIURL != "" {
		return a.detectDiscountAPI(ctx, config, torrentID)
	}
	return a.detectDiscountPage(ctx, config, torrentID)
}

func (a *NexusPHPAdapter) detectDiscountAPI(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	u := config.Domain
	if !strings.HasPrefix(u, "http") {
		u = "https://" + u
	}
	u = u + strings.Replace(config.Discount.APIURL, "{id}", torrentID, 1)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return &model.DiscountResult{Level: model.DiscountNone}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	for _, sel := range config.Discount.Selectors {
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

	_ = html
	return &model.DiscountResult{Level: model.DiscountNone}, nil
}

func (a *NexusPHPAdapter) detectDiscountPage(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	u := buildURL(config.Domain, "/details.php", torrentID, "")

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return &model.DiscountResult{Level: model.DiscountNone}, nil
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return &model.DiscountResult{Level: model.DiscountNone}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	html := strings.ToLower(string(body))

	if strings.Contains(html, "class=\"pro_free2up\"") || strings.Contains(html, "pro_free2up") {
		return &model.DiscountResult{Level: model.Discount2xFree, Multiplier: 2.0}, nil
	}
	if strings.Contains(html, "class=\"pro_2up\"") || strings.Contains(html, "pro_2up") {
		return &model.DiscountResult{Level: model.Discount2xUp, Multiplier: 2.0}, nil
	}
	if strings.Contains(html, "class=\"pro_free\"") || strings.Contains(html, "pro_free") {
		return &model.DiscountResult{Level: model.DiscountFree}, nil
	}
	if strings.Contains(html, "class=\"pro_50p\"") || strings.Contains(html, "pro_50p") {
		return &model.DiscountResult{Level: model.DiscountPercent50}, nil
	}

	if strings.Contains(html, "免费") || strings.Contains(html, "free") {
		return &model.DiscountResult{Level: model.DiscountFree}, nil
	}

	return &model.DiscountResult{Level: model.DiscountNone}, nil
}

func (a *NexusPHPAdapter) DetectHR(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
	if len(config.HR.Selectors) == 0 {
		return &model.HRResult{HasHR: false}, nil
	}

	u := buildURL(config.Domain, "/details.php", torrentID, "")

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return &model.HRResult{HasHR: false}, nil
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return &model.HRResult{HasHR: false}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	html := strings.ToLower(string(body))

	hasHR := strings.Contains(html, "hit and run") ||
		strings.Contains(html, "hit&run") ||
		strings.Contains(html, "h&r") ||
		strings.Contains(html, "考核") ||
		strings.Contains(html, "class=\"hitandrun\"")

	result := &model.HRResult{HasHR: hasHR}
	if hasHR {
		result.SeedTimeH = 72
	}

	return result, nil
}

func (a *NexusPHPAdapter) UploadTorrent(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	if len(req.TorrentData) == 0 {
		return nil, fmt.Errorf("种子文件数据为空")
	}

	baseURL := config.Domain
	if !strings.HasPrefix(baseURL, "http") {
		baseURL = "https://" + baseURL
	}
	uploadPath := "/upload.php"
	if config.Paths.Upload != "" {
		uploadPath = config.Paths.Upload
	}

	if cat, ok := req.FormFields["category"]; ok && config.Paths.TakeUpload != "" {
		if isMusicCategory(cat) {
			uploadPath = config.Paths.TakeUpload
		}
	}

	uploadURL := baseURL + uploadPath

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	fileWriter, err := writer.CreateFormFile("torrent", "upload.torrent")
	if err != nil {
		return nil, fmt.Errorf("创建表单文件字段失败: %w", err)
	}
	if _, err := fileWriter.Write(req.TorrentData); err != nil {
		return nil, fmt.Errorf("写入种子数据失败: %w", err)
	}

	if req.Title != "" {
		_ = writer.WriteField("name", req.Title)
	}
	if req.Subtitle != "" {
		_ = writer.WriteField("small_descr", req.Subtitle)
	}
	if req.Description != "" {
		_ = writer.WriteField("descr", req.Description)
	}
	if req.IMDbLink != "" {
		_ = writer.WriteField("url", req.IMDbLink)
	}
	if req.DoubanLink != "" {
		_ = writer.WriteField("douban_url", req.DoubanLink)
	}
	if req.Anonymous {
		_ = writer.WriteField("anonymity", "1")
	}

	npFieldMap := map[string]string{
		"category":   "type",
		"source":     "source_sel",
		"resolution": "standard_sel",
		"codec":      "codec_sel",
		"audioCodec": "audiocodec_sel",
		"medium":     "medium_sel",
		"team":       "team_sel",
		"processing": "processing_sel",
	}

	for k, v := range req.FormFields {
		if mapped, ok := npFieldMap[k]; ok {
			_ = writer.WriteField(mapped, v)
		} else {
			_ = writer.WriteField(k, v)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("关闭 multipart writer 失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", uploadURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	setCommonHeaders(httpReq, config.Cookie)

	resp, err := a.doer.Client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("上传请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	html := string(body)

	if resp.StatusCode == http.StatusForbidden {
		return &model.PublishResponse{Success: false, ErrorMessage: "403 Forbidden: 权限不足或 cookie 过期"}, nil
	}

	if idMatch := regexp.MustCompile(`(?:details|detail)\.php\?id=(\d+)`).FindStringSubmatch(html); len(idMatch) > 1 {
		torrentID := idMatch[1]
		detailURL := baseURL + "/details.php?id=" + torrentID
		return &model.PublishResponse{
			Success:    true,
			TorrentID:  torrentID,
			DetailURL:  detailURL,
			TargetSite: config.Domain,
		}, nil
	}

	if strings.Contains(html, "uploaded") || strings.Contains(html, "成功") || strings.Contains(html, "Upload succeeded") {
		return &model.PublishResponse{Success: true, TargetSite: config.Domain}, nil
	}

	errMsg := "上传失败: 未知响应"
	if m := regexp.MustCompile(`class="error"[^>]*>([^<]+)`).FindStringSubmatch(html); len(m) > 1 {
		errMsg = strings.TrimSpace(m[1])
	} else if m := regexp.MustCompile(`<p[^>]*>([^<]*(?:失败|错误|error|fail|拒绝|duplicate|already)[^<]*)</p>`).FindStringSubmatch(html); len(m) > 1 {
		errMsg = strings.TrimSpace(m[1])
	}

	return &model.PublishResponse{Success: false, ErrorMessage: errMsg}, nil
}

func (a *NexusPHPAdapter) SearchTorrents(ctx context.Context, config *model.SiteConfig, keyword string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
	u := config.Domain
	if !strings.HasPrefix(u, "http") {
		u = "https://" + u
	}
	browsePath := "/browse.php"
	if config.Paths.Browse != "" {
		browsePath = config.Paths.Browse
	}
	searchURL := u + browsePath + "?search=" + keyword
	if opts != nil && opts.Category != "" {
		searchURL += "&cat=" + opts.Category
	}

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构造搜索请求失败: %w", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("搜索请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	return parseNexusPHPBrowse(string(body), config), nil
}

func (a *NexusPHPAdapter) GetTorrentInfoHash(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	detail, err := a.GetTorrentDetail(ctx, config, torrentID)
	if err != nil {
		return "", err
	}
	if detail.InfoHash == "" {
		return "", fmt.Errorf("未在详情页找到 info_hash")
	}
	return detail.InfoHash, nil
}

func (a *NexusPHPAdapter) SupportsSearchByPiecesHash() bool { return false }

func buildURL(domain, path, torrentID, passkey string) string {
	u := domain
	if !strings.HasPrefix(u, "http") {
		u = "https://" + u
	}
	u = u + path + "?id=" + torrentID
	if passkey != "" {
		u += "&passkey=" + passkey
	}
	return u
}

func setCommonHeaders(req *http.Request, cookie string) {
	req.Header.Set("User-Agent", "PT-Forward/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
}

func parseNexusPHPBrowse(html string, config *model.SiteConfig) []*model.SeedingSearchResult {
	var results []*model.SeedingSearchResult

	rowRe := regexp.MustCompile(`<tr[^>]*class="torrent-row[^"]*"[^>]*>(.*?)</tr>`)
	if !rowRe.MatchString(html) {
		rowRe = regexp.MustCompile(`<tr[^>]*>(.*?)</tr>`)
	}

	detailLinkRe := regexp.MustCompile(`href="(?:details|detail)\.php\?id=(\d+)[^"]*"[^>]*>([^<]+)`)
	sizeRe := regexp.MustCompile(`(?i)([\d.]+)\s*(TB|GB|MB|KB)`)
	seedersRe := regexp.MustCompile(`>(\d+)</a>\s*</td>\s*$`)
	leechersRe := regexp.MustCompile(`(\d+)\s*</td>\s*$`)

	rows := rowRe.FindAllString(html, -1)
	for _, row := range rows {
		linkMatch := detailLinkRe.FindStringSubmatch(row)
		if len(linkMatch) < 3 {
			continue
		}

		torrentID := linkMatch[1]
		title := strings.TrimSpace(linkMatch[2])

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

		if opts := len(results); opts >= 50 {
			break
		}
	}

	return results
}

func parseSizeStr(s string) int64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")

	valRe := regexp.MustCompile(`([\d.]+)\s*(TB|GB|MB|KB|B)`)
	m := valRe.FindStringSubmatch(s)
	if len(m) < 3 {
		return 0
	}

	val, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}

	switch strings.ToUpper(m[2]) {
	case "TB":
		return int64(val * 1024 * 1024 * 1024 * 1024)
	case "GB":
		return int64(val * 1024 * 1024 * 1024)
	case "MB":
		return int64(val * 1024 * 1024)
	case "KB":
		return int64(val * 1024)
	default:
		return int64(val)
	}
}

func extractTags(html string) []string {
	tagRe := regexp.MustCompile(`class="tag[^"]*"[^>]*>([^<]+)`)
	matches := tagRe.FindAllStringSubmatch(html, -1)
	tags := make([]string, 0, len(matches))
	for _, m := range matches {
		tags = append(tags, strings.TrimSpace(m[1]))
	}
	return tags
}

var musicCategoryIDs = map[string]bool{
	"406": true,
	"408": true,
	"409": true,
}

func isMusicCategory(cat string) bool {
	if musicCategoryIDs[cat] {
		return true
	}
	lower := strings.ToLower(cat)
	return strings.Contains(lower, "music") || strings.Contains(lower, "音频") || strings.Contains(lower, "hq audio")
}
