package adapter

import (
	"bytes"
	"context"
	"encoding/json"
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
	defer func() { _ = resp.Body.Close() }()

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

	return io.ReadAll(resp.Body)
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("HTTP %d", resp.StatusCode), nil)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, networkError("读取响应失败", err)
	}

	html := string(body)
	detail := &model.TorrentDetail{}

	if m := regexp.MustCompile(`<title>([^<]+)</title>`).FindStringSubmatch(html); len(m) > 1 {
		detail.Title = strings.TrimSpace(m[1])
	}

	rowRe := regexp.MustCompile(`(?s)<tr[^>]*>.*?</tr>`)
	for _, row := range rowRe.FindAllString(html, -1) {
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
		ddBlockRe := regexp.MustCompile(`(?s)基本信息</dt>\s*<dd>(.*)</dd>`)
		if m := ddBlockRe.FindStringSubmatch(html); len(m) > 1 {
			extractBasicInfoFields(m[1], detail)
		}
		fallbackDivRe := regexp.MustCompile(`(?s)基本信息</div>(.*?)种子文件`)
		if m := fallbackDivRe.FindStringSubmatch(html); len(m) > 1 {
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
		if m := regexp.MustCompile(`(?i)info_hash.*?([a-fA-F0-9]{40})`).FindStringSubmatch(html); len(m) > 1 {
			detail.InfoHash = strings.ToLower(m[1])
		}
	}

	if m := regexp.MustCompile(`(?s)<div[^>]*id=['"]kdescr['"][^>]*>([\s\S]*?)</div>`).FindStringSubmatch(html); len(m) > 1 {
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("HTTP %d", resp.StatusCode), nil)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, networkError("读取响应失败", err)
	}

	html := string(body)
	sl := &model.SLData{}

	seedersRe := regexp.MustCompile(`(?i)(?:做种数|Seeders?|S[^<]*<[^>]*>)(\d+)`)
	if m := seedersRe.FindStringSubmatch(html); len(m) > 1 {
		sl.Seeders, _ = strconv.Atoi(m[1])
	}

	leechersRe := regexp.MustCompile(`(?i)(?:下载数|Leechers?|L[^<]*<[^>]*>)(\d+)`)
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
		return &model.DiscountResult{Level: model.DiscountNone}, nil
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

func (a *GenericAdapter) detectDiscountGenericPage(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.DiscountResult, error) {
	u := buildGenericURL(config, config.Paths.Detail, torrentID)
	if u == "" {
		return &model.DiscountResult{Level: model.DiscountNone}, nil
	}

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

	result := DetectDiscountFromHTML(html, &config.DiscountDetection)
	if result.Level != model.DiscountNone {
		return result, nil
	}

	return &model.DiscountResult{Level: model.DiscountNone}, nil
}

func (a *GenericAdapter) DetectHR(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
	if len(config.HR.Selectors) == 0 {
		return &model.HRResult{HasHR: false}, nil
	}

	u := buildGenericURL(config, config.Paths.Detail, torrentID)
	if u == "" {
		return &model.HRResult{HasHR: false}, nil
	}

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
		strings.Contains(html, "hitandrun")

	result := &model.HRResult{HasHR: hasHR}
	if hasHR {
		result.SeedTimeH = config.HR.SeedTimeH()
	}

	return result, nil
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

	fileWriter, err := writer.CreateFormFile("file", "upload.torrent")
	if err != nil {
		return nil, uploadError("创建表单文件字段失败", err)
	}
	if _, err := fileWriter.Write(req.TorrentData); err != nil {
		return nil, networkError("写入种子数据失败", err)
	}

	if req.Title != "" {
		_ = writer.WriteField("name", req.Title)
	}
	if req.Description != "" {
		_ = writer.WriteField("descr", req.Description)
	}
	for k, v := range req.FormFields {
		_ = writer.WriteField(k, v)
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
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if resp.StatusCode == http.StatusForbidden {
		return nil, &model.AppError{Code: 14003, Message: "403 Forbidden: 权限不足"}
	}

	if idMatch := regexp.MustCompile(`(?:details|detail|torrent)\.php\?id=(\d+)`).FindStringSubmatch(html); len(idMatch) > 1 {
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
	if m := regexp.MustCompile(`class="error"[^>]*>([^<]+)`).FindStringSubmatch(html); len(m) > 1 {
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
		u += "&search=" + keyword
	} else {
		u += "?search=" + keyword
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("HTTP %d", resp.StatusCode), nil)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, networkError("读取响应失败", err)
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
		return false, nil
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
		u = strings.ReplaceAll(u, "{id}", torrentID)
	}
	u = strings.ReplaceAll(u, "{passkey}", config.Passkey)
	u = strings.ReplaceAll(u, "{authkey}", config.AuthKey)
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
		return base + "/download.php?id=" + torrentID + "&passkey=" + config.Passkey
	}
	return base + "/download.php?id=" + torrentID
}

func stripTags(s string) string {
	return regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, "")
}

func parseGenericBrowse(html string, config *model.SiteConfig) []*model.SeedingSearchResult {
	var results []*model.SeedingSearchResult

	rowRe := regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	detailLinkRe := regexp.MustCompile(`(?s)href="[^"]*(?:details?|torrent)[^"]*id=(\d+)[^"]*"[^>]*>(.*?)</a>`)
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

	fileWriter, err := writer.CreateFormFile("file", "upload.torrent")
	if err != nil {
		return nil, uploadError("创建表单文件字段失败", err)
	}
	if _, err := fileWriter.Write(req.TorrentData); err != nil {
		return nil, networkError("写入种子数据失败", err)
	}

	if req.Title != "" {
		_ = writer.WriteField("name", req.Title)
	}
	if req.Subtitle != "" {
		_ = writer.WriteField("subtitle", req.Subtitle)
	}
	if req.Description != "" {
		_ = writer.WriteField("descr", req.Description)
	}
	if req.IMDbLink != "" {
		_ = writer.WriteField("imdb_c", req.IMDbLink)
	}
	if req.DoubanLink != "" {
		m := regexp.MustCompile(`(\d+)`).FindString(req.DoubanLink)
		if m != "" {
			_ = writer.WriteField("douban_id", m)
		}
	}
	if req.Anonymous {
		_ = writer.WriteField("anonymity", "yes")
	}

	if v, ok := req.FormFields["cat"]; ok {
		_ = writer.WriteField("type", v)
	}
	if v, ok := req.FormFields["tags"]; ok {
		if strings.Contains(v, "禁转") {
			_ = writer.WriteField("nodistr", "yes")
		}
	}

	_ = writer.WriteField("MAX_FILE_SIZE", "4000000")
	_ = writer.WriteField("team", "")
	_ = writer.WriteField("hr", "no")

	for k, v := range req.FormFields {
		switch k {
		case "cat", "tags":
			continue
		}
		_ = writer.WriteField(k, v)
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
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if resp.StatusCode == http.StatusForbidden {
		return nil, &model.AppError{Code: 14003, Message: "403 Forbidden: 权限不足"}
	}

	if idMatch := regexp.MustCompile(`/t/(\d+)`).FindStringSubmatch(html); len(idMatch) > 1 {
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
	if m := regexp.MustCompile(`class="error"[^>]*>([^<]+)`).FindStringSubmatch(html); len(m) > 1 {
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

	fileWriter, err := writer.CreateFormFile("file", "upload.torrent")
	if err != nil {
		return nil, uploadError("创建表单文件字段失败", err)
	}
	if _, err := fileWriter.Write(req.TorrentData); err != nil {
		return nil, networkError("写入种子数据失败", err)
	}

	_ = writer.WriteField("tid", "0")

	if req.Title != "" {
		_ = writer.WriteField("name", req.Title)
	}
	if req.Subtitle != "" {
		_ = writer.WriteField("small_desc", req.Subtitle)
	}
	if req.Description != "" {
		_ = writer.WriteField("descr", req.Description)
	}
	if req.DoubanLink != "" {
		_ = writer.WriteField("douban_url", req.DoubanLink)
	}
	if req.IMDbLink != "" {
		_ = writer.WriteField("imdb_url", req.IMDbLink)
	}
	if len(req.Screenshots) > 0 {
		_ = writer.WriteField("screen", strings.Join(req.Screenshots, "\n"))
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
			_ = writer.WriteField(targetName, v)
		}
	}

	if hdr, ok := req.ExtraFields["tr_hdr"]; ok {
		_ = writer.WriteField("tr_hdr", hdr)
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
				_ = writer.WriteField(tagName, "on")
			}
		}
	}

	for k, v := range req.FormFields {
		switch k {
		case "cat", "medium_sel", "codec_sel", "audiocodec_sel", "standard_sel", "team_sel", "tags":
			continue
		}
		_ = writer.WriteField(k, v)
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

	fileWriter, err := writer.CreateFormFile("file", "upload.torrent")
	if err != nil {
		return nil, uploadError("创建表单文件字段失败", err)
	}
	if _, err := fileWriter.Write(req.TorrentData); err != nil {
		return nil, networkError("写入种子数据失败", err)
	}

	_ = writer.WriteField("tid", "0")

	if v, ok := req.ExtraFields["artist"]; ok {
		_ = writer.WriteField("artist", v)
	}
	if req.Title != "" {
		_ = writer.WriteField("title", req.Title)
	}
	if v, ok := req.ExtraFields["year"]; ok {
		_ = writer.WriteField("year", v)
	}
	if v, ok := req.ExtraFields["image"]; ok {
		_ = writer.WriteField("image", v)
	}
	if req.Description != "" {
		_ = writer.WriteField("release_desc", req.Description)
	}

	if v, ok := req.FormFields["cat"]; ok {
		_ = writer.WriteField("release_type", v)
	}
	if v := resolveField(req.FormFields, "category", "cat"); v != "" {
		_ = writer.WriteField("release_type", v)
	}
	if v := resolveField(req.FormFields, "codec_sel", "codec"); v != "" {
		_ = writer.WriteField("format", v)
	}
	if v := resolveField(req.FormFields, "audiocodec_sel", "audioCodec"); v != "" {
		_ = writer.WriteField("bitrate", v)
	}
	if v := resolveField(req.FormFields, "medium_sel", "medium"); v != "" {
		_ = writer.WriteField("media", v)
	}

	if v, ok := req.ExtraFields["remaster_year"]; ok {
		_ = writer.WriteField("remaster_year", v)
	}
	if v, ok := req.ExtraFields["remaster_title"]; ok {
		_ = writer.WriteField("remaster_title", v)
	}
	if v, ok := req.ExtraFields["remaster_record_label"]; ok {
		_ = writer.WriteField("remaster_record_label", v)
	}
	if v, ok := req.ExtraFields["remaster_catalogue_number"]; ok {
		_ = writer.WriteField("remaster_catalogue_number", v)
	}

	for k, v := range req.ExtraFields {
		switch k {
		case "artist", "year", "image", "remaster_year", "remaster_title",
			"remaster_record_label", "remaster_catalogue_number":
			continue
		}
		_ = writer.WriteField(k, v)
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
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if resp.StatusCode == http.StatusForbidden {
		return nil, &model.AppError{Code: 14003, Message: "403 Forbidden"}
	}

	if idMatch := regexp.MustCompile(`(?:details|torrent)\.php\?id=(\d+)`).FindStringSubmatch(html); len(idMatch) > 1 {
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
	if m := regexp.MustCompile(`class="error"[^>]*>([^<]+)`).FindStringSubmatch(html); len(m) > 1 {
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

	fileWriter, err := writer.CreateFormFile("files", "upload.torrent")
	if err != nil {
		return nil, uploadError("创建表单文件字段失败", err)
	}
	if _, err := fileWriter.Write(req.TorrentData); err != nil {
		return nil, networkError("写入种子数据失败", err)
	}

	if req.Title != "" {
		_ = writer.WriteField("showName", req.Title)
	}
	if req.Subtitle != "" {
		_ = writer.WriteField("shortDesc", req.Subtitle)
	}
	if req.Description != "" {
		_ = writer.WriteField("descr", req.Description)
	}
	if req.Anonymous {
		_ = writer.WriteField("uploadUserAnonymous", "是")
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
			_ = writer.WriteField(targetName, v)
		}
	}

	if tags, ok := req.FormFields["tags"]; ok {
		tagList := strings.Split(tags, ",")
		for _, tag := range tagList {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				_ = writer.WriteField("tagList", tag)
			}
		}
	}

	if imdbID := extractIMDbIDGeneric(req.IMDbLink); imdbID != "" {
		_ = writer.WriteField("imdb", imdbID)
	}
	if req.DoubanLink != "" {
		m := regexp.MustCompile(`(\d+)`).FindString(req.DoubanLink)
		if m != "" {
			_ = writer.WriteField("douban", m)
		}
	}
	if v, ok := req.ExtraFields["picture"]; ok {
		_ = writer.WriteField("picture", v)
	}

	for k, v := range req.FormFields {
		switch k {
		case "cat", "medium_sel", "codec_sel", "audiocodec_sel", "standard_sel",
			"processing_sel", "team_sel", "tags":
			continue
		}
		_ = writer.WriteField(k, v)
	}
	for k, v := range req.ExtraFields {
		if k == "picture" {
			continue
		}
		_ = writer.WriteField(k, v)
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
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

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
	m := regexp.MustCompile(`tt\d+`).FindString(link)
	return m
}
