package adapter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

var basicInfoKeyList = []string{"大小", "类型", "媒介", "编码", "分辨率", "音频编码", "制作组", "地区", "质量", "来源", "发布时间", "发布者", "季", "处理"}

var hashRowPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)种子Hash[：:](?:<[^>]*>|&nbsp;|\s)*([a-fA-F0-9]{40})`),
	regexp.MustCompile(`(?i)种子散列值[：:](?:<[^>]*>|&nbsp;|\s)*([a-fA-F0-9]{40})`),
	regexp.MustCompile(`(?i)Hash码(?:<[^>]*>)?[：:]?(?:<[^>]*>|&nbsp;|\s)*([a-fA-F0-9]{40})`),
	regexp.MustCompile(`data-hash="([a-fA-F0-9]{40})"`),
}

type NexusPHPAdapter struct {
	doer   *HTTPDoer
	logger *zap.Logger
}

func NewNexusPHPAdapter(doer *HTTPDoer, logger *zap.Logger) *NexusPHPAdapter {
	return &NexusPHPAdapter{doer: doer, logger: logger}
}

func (a *NexusPHPAdapter) Framework() string { return "nexusphp" }

func (a *NexusPHPAdapter) ParseRSS(_ context.Context, _ string, _ *model.SiteConfig) ([]*model.RSSTorrentEvent, error) {
	return nil, parseError("ParseRSS: use RSS fetcher instead", nil)
}

func (a *NexusPHPAdapter) DownloadTorrent(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	u := buildURL(config.Domain, "/download.php", torrentID, config.Passkey)

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

	return io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
}

func (a *NexusPHPAdapter) GetTorrentDetail(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	u := buildURL(config.Domain, "/details.php", torrentID, "")

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

	if detail.Subtitle == "" {
		ddSubRe := regexp.MustCompile(`(?s)副标题</dt>\s*<dd>(.*?)</dd>`)
		if m := ddSubRe.FindStringSubmatch(html); len(m) > 1 {
			text := stripTags(m[1])
			text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
			text = strings.TrimSpace(text)
			if text != "" {
				detail.Subtitle = text
			}
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
		if m := regexp.MustCompile(`(?i)Hash码</dt>\s*<dd>\s*([a-fA-F0-9]{40})`).FindStringSubmatch(html); len(m) > 1 {
			detail.InfoHash = strings.ToLower(m[1])
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

func cleanRowText(row string) string {
	text := stripTags(row)
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func extractBasicInfoFields(row string, detail *model.TorrentDetail) {
	preprocessed := regexp.MustCompile(`</d[t]>\s*<d[d]>`).ReplaceAllString(row, "：")
	text := cleanRowText(preprocessed)

	type keyMatch struct {
		key      string
		valStart int
		pos      int
	}
	var matches []keyMatch
	for _, key := range basicInfoKeyList {
		re := regexp.MustCompile(key + `[：:]`)
		loc := re.FindStringIndex(text)
		if loc != nil {
			matches = append(matches, keyMatch{key: key, valStart: loc[1], pos: loc[0]})
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].pos < matches[j].pos })

	for i, m := range matches {
		valEnd := len(text)
		if i+1 < len(matches) {
			valEnd = matches[i+1].pos
		}
		val := strings.TrimSpace(text[m.valStart:valEnd])

		switch m.key {
		case "大小":
			if detail.Size == 0 {
				if sm := regexp.MustCompile(`([\d.]+)\s*(TB|GB|MB|KB|TiB|GiB|MiB|KiB)`).FindStringSubmatch(val); len(sm) > 2 {
					detail.Size = parseSizeStr(sm[1] + " " + sm[2])
				}
			}
		case "类型":
			if detail.Category == "" {
				detail.Category = val
			}
		case "编码":
			detail.Codec = val
		case "分辨率":
			detail.Resolution = val
		case "媒介", "来源", "质量":
			if detail.Source == "" {
				detail.Source = val
			}
		case "音频编码":
			detail.AudioCodec = val
		case "制作组":
			detail.ReleaseGroup = val
		case "地区":
			detail.Region = val
		case "处理":
			detail.Processing = val
		}
	}
}

func extractCategoryFromImg(row string, detail *model.TorrentDetail) {
	imgRe := regexp.MustCompile(`<img[^>]*alt="([^"]+)"`)
	for _, m := range imgRe.FindAllStringSubmatch(row, -1) {
		alt := strings.TrimSpace(m[1])
		if alt == "" || alt == "Show/Hide" || alt == "显示/隐藏" {
			continue
		}
		detail.Category = alt
		return
	}
	if m := regexp.MustCompile(`cat=\d+[^>]*>([^<]+)`).FindStringSubmatch(row); len(m) > 1 {
		detail.Category = strings.TrimSpace(m[1])
	}
}

func extractSubtitleFromRow(row string, detail *model.TorrentDetail) {
	tdRe := regexp.MustCompile(`(?s)class="rowfollow"[^>]*>(.*?)</td>`)
	m := tdRe.FindStringSubmatch(row)
	if len(m) < 2 {
		return
	}
	text := stripTags(m[1])
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)
	if text != "" {
		detail.Subtitle = text
	}
}

func extractInfoHashFromRow(row string) string {
	for _, re := range hashRowPatterns {
		if m := re.FindStringSubmatch(row); len(m) > 1 {
			return strings.ToLower(m[1])
		}
	}
	return ""
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

	seedersRe := regexp.MustCompile(`(\d+)个做种者`)
	leechersRe := regexp.MustCompile(`(\d+)个下载者`)

	rowRe := regexp.MustCompile(`(?s)<tr[^>]*>.*?</tr>`)
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
				return sl, nil
			}
		}

		if strings.Contains(rowText, "基本信息") {
			if m := regexp.MustCompile(`id=['"]seeders['"][^>]*>(?:<[^>]*>)*(\d+)`).FindStringSubmatch(row); len(m) > 1 {
				sl.Seeders, _ = strconv.Atoi(m[1])
			}
			if m := regexp.MustCompile(`id=['"]leechers['"][^>]*>(?:<[^>]*>)*(\d+)`).FindStringSubmatch(row); len(m) > 1 {
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
	defer func() { _ = resp.Body.Close() }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
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
		return nil, networkError("构造优惠检测请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("请求优惠详情页失败", err)
	}
	defer func() { _ = resp.Body.Close() }()

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

func (a *NexusPHPAdapter) DetectHR(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.HRResult, error) {
	if len(config.HR.Selectors) == 0 {
		return &model.HRResult{HasHR: false}, nil
	}

	u := buildURL(config.Domain, "/details.php", torrentID, "")

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, networkError("构造HR检测请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("请求HR详情页失败", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := strings.ToLower(string(body))

	hasHR := strings.Contains(html, "hit and run") ||
		strings.Contains(html, "hit&run") ||
		strings.Contains(html, "h&r") ||
		strings.Contains(html, "考核") ||
		strings.Contains(html, "class=\"hitandrun\"")

	result := &model.HRResult{HasHR: hasHR}
	if hasHR {
		result.SeedTimeH = config.HR.SeedTimeH()
	}

	return result, nil
}

func (a *NexusPHPAdapter) UploadTorrent(ctx context.Context, config *model.SiteConfig, req *model.PublishRequest) (*model.PublishResponse, error) {
	if len(req.TorrentData) == 0 {
		return nil, &model.AppError{Code: 40001, Message: "种子文件数据为空"}
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

	npFieldMap := map[string]string{
		"category":   "type",
		"source":     "source_sel",
		"resolution": "standard_sel",
		"codec":      "codec_sel",
		"audioCodec": "audiocodec_sel",
		"medium":     "medium_sel",
		"team":       "team_sel",
		"processing": "processing_sel",
		"douban":     "douban",
	}

	for k, v := range config.Publish.FormFields {
		if _, exists := npFieldMap[k]; exists {
			npFieldMap[k] = v
		}
	}

	musicCat := ""
	if cat, ok := req.FormFields["category"]; ok {
		musicCat = cat
	}
	isMusic := isMusicCategory(musicCat)

	for k, v := range req.FormFields {
		if isMusic {
			if mapped, ok := npMusicFieldMap[k]; ok {
				fw.writeField(mapped, v)
			} else if mapped, ok := npFieldMap[k]; ok && k == "category" {
				fw.writeField(mapped, v)
			} else {
				fw.writeField(k, v)
			}
		} else {
			if mapped, ok := npFieldMap[k]; ok {
				fw.writeField(mapped, v)
			} else {
				fw.writeField(k, v)
			}
		}
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
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, networkError("读取响应失败", err)
	}

	html := string(body)

	if resp.StatusCode == http.StatusForbidden {
		return nil, &model.AppError{Code: 14003, Message: "403 Forbidden: cookie 可能已过期"}
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

	return nil, &model.AppError{Code: 15001, Message: errMsg}
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

	return parseNexusPHPBrowse(string(body), config), nil
}

func (a *NexusPHPAdapter) GetTorrentInfoHash(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	detail, err := a.GetTorrentDetail(ctx, config, torrentID)
	if err != nil {
		return "", err
	}
	if detail.InfoHash == "" {
		return "", notFoundError("未在详情页找到 info_hash")
	}
	return detail.InfoHash, nil
}

func (a *NexusPHPAdapter) SupportsSearchByPiecesHash() bool { return false }

func (a *NexusPHPAdapter) VerifyExists(ctx context.Context, config *model.SiteConfig, torrentID string) (bool, error) {
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

	detailLinkRe := regexp.MustCompile(`(?s)href="details\.php\?id=(\d+)"[^>]*><b>\s*&nbsp;([^<]+)</b>`)
	altDetailLinkRe := regexp.MustCompile(`(?s)href="details\.php\?id=(\d+)[^"]*"[^>]*>(.*?)</a>`)
	sizeRe := regexp.MustCompile(`(?i)class="rowfollow">([\d.]+)\s*<br\s*/?>\s*(TB|GB|MB|KB)`)
	seedersRe := regexp.MustCompile(`dllist=1#seeders">\s*(\d+)\s*</a>`)
	leechersRe := regexp.MustCompile(`dllist=1#leechers">\s*(\d+)\s*</a>`)

	seen := map[string]bool{}
	detailMatches := detailLinkRe.FindAllStringSubmatchIndex(html, -1)
	if len(detailMatches) == 0 {
		detailMatches = altDetailLinkRe.FindAllStringSubmatchIndex(html, -1)
	}

	for _, loc := range detailMatches {
		torrentID := html[loc[2]:loc[3]]
		if seen[torrentID] {
			continue
		}
		seen[torrentID] = true

		title := html[loc[4]:loc[5]]
		title = strings.TrimSpace(stripTags(strings.ReplaceAll(title, "&nbsp;", " ")))

		start := loc[0]
		end := loc[1]
		if end+5000 <= len(html) {
			end += 5000
		} else {
			end = len(html)
		}
		chunk := html[start:end]

		result := &model.SeedingSearchResult{
			TorrentID: torrentID,
			Title:     title,
			DetailURL: config.Domain + "/details.php?id=" + torrentID,
		}

		if m := sizeRe.FindStringSubmatch(chunk); len(m) > 2 {
			result.Size = parseSizeStr(m[1] + " " + m[2])
		}

		if m := seedersRe.FindStringSubmatch(chunk); len(m) > 1 {
			result.Seeders, _ = strconv.Atoi(m[1])
		}

		if m := leechersRe.FindStringSubmatch(chunk); len(m) > 1 {
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

func parseSizeStr(s string) int64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")

	valRe := regexp.MustCompile(`([\d.]+)\s*(TiB|GiB|MiB|KiB|TB|GB|MB|KB|B)`)
	m := valRe.FindStringSubmatch(s)
	if len(m) < 3 {
		return 0
	}

	val, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}

	switch strings.ToUpper(m[2]) {
	case "TB", "TIB":
		return int64(val * 1024 * 1024 * 1024 * 1024)
	case "GB", "GIB":
		return int64(val * 1024 * 1024 * 1024)
	case "MB", "MIB":
		return int64(val * 1024 * 1024)
	case "KB", "KIB":
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

var npMusicFieldMap = map[string]string{
	"music_artist":  "artists",
	"music_album":   "album",
	"music_year":    "year",
	"music_format":  "format_type",
	"music_medium":  "medium_type",
	"music_publish": "publish_type",
	"cover_url":     "cover_url",
}

func isMusicCategory(cat string) bool {
	if musicCategoryIDs[cat] {
		return true
	}
	lower := strings.ToLower(cat)
	return strings.Contains(lower, "music") || strings.Contains(lower, "音频") || strings.Contains(lower, "hq audio")
}

var categoryNormRules = []struct {
	pattern string
	target  string
}{
	{`(?i)^Movies`, "电影"},
	{`电影`, "电影"},
	{`高清电影`, "电影"},
	{`(?i)^TV\s*Series`, "电视剧"},
	{`电视剧`, "电视剧"},
	{`剧集`, "电视剧"},
	{`短剧`, "电视剧"},
	{`(?i)^TV\s*Shows`, "综艺"},
	{`综艺`, "综艺"},
	{`(?i)^Anime`, "动漫"},
	{`(?i)^Animations`, "动漫"},
	{`动漫`, "动漫"},
	{`动画`, "动漫"},
	{`(?i)^Documentar`, "纪录片"},
	{`纪录片`, "纪录片"},
	{`(?i)^Music`, "音乐"},
	{`音乐`, "音乐"},
	{`(?i)^Lossless`, "音乐"},
	{`(?i)^Book`, "书籍"},
	{`(?i)^Program`, "软件"},
	{`学习`, "教育"},
	{`文档`, "文档"},
	{`漫画`, "漫画"},
	{`(?i)^Nature`, "纪录片"},
	{`Doc\s`, "纪录片"},
}

var categoryNormRe = func() []struct {
	re     *regexp.Regexp
	target string
} {
	var compiled []struct {
		re     *regexp.Regexp
		target string
	}
	for _, r := range categoryNormRules {
		compiled = append(compiled, struct {
			re     *regexp.Regexp
			target string
		}{regexp.MustCompile(r.pattern), r.target})
	}
	return compiled
}()

func NormalizeCategory(raw string) string {
	if raw == "" {
		return ""
	}
	for _, rule := range categoryNormRe {
		if rule.re.MatchString(raw) {
			return rule.target
		}
	}
	return raw
}
