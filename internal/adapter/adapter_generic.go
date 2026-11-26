package adapter

import (
	"bytes"
	"context"
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
	reGenericTitle       = regexp.MustCompile(`<title>([^<]+)</title>`)
	reGenericRow         = regexp.MustCompile(`(?s)<tr[^>]*>.*?</tr>`)
	reGenericDDBlock     = regexp.MustCompile(`(?s)基本信息</dt>\s*<dd>(.*)</dd>`)
	reGenericFallbackDiv = regexp.MustCompile(`(?s)基本信息</div>(.*?)种子文件`)
	reGenericInfoHash    = regexp.MustCompile(`(?i)info_hash.*?([a-fA-F0-9]{40})`)
	reGenericKdescr      = regexp.MustCompile(`(?s)<div[^>]*id=['"]kdescr['"][^>]*>([\s\S]*?)</div>`)
	reGenericSeeders     = regexp.MustCompile(`(?i)(?:做种数|Seeders?|S[^<]*<[^>]*>)(\d+)`)
	reGenericLeechers    = regexp.MustCompile(`(?i)(?:下载数|Leechers?|L[^<]*<[^>]*>)(\d+)`)
	// §59.159 词边界修复：前置 [^a-zA-Z0-9_] 防 userdetails.php/user torrent 误命中
	// （实战：四次发布 tid=10136 恒等于用户自身 ID——上传失败页的导航链接被误抓判"成功"）
	reGenericErrorClass        = regexp.MustCompile(`class="error"[^>]*>([^<]+)`)
	reGenericStripTags         = regexp.MustCompile(`<[^>]+>`)
	reGenericBrowseRow         = regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	reGenericBrowseDetailLink  = regexp.MustCompile(`(?s)href=["'](?:/t/(\d+)/|[^"']*\bdetails?\.php\?id=(\d+)[^"']*|[^"']*torrent[s]?\.php\?(?:torrent)?id=(\d+)[^"']*|[^"']*video_detail\.php\?tid=(\d+)[^"']*)["'][^>]*>(.*?)</a>`)
	reGenericBrowseSize        = regexp.MustCompile(`(?i)([\d.]+)\s*(?:<br\s*/?>)?\s*(TB|GB|MB|KB|T|G|M)\b`)
	reGenericBrowseSeeders     = regexp.MustCompile(`>(\d+)</a>\s*</td>\s*$`)
	reGenericBrowseLeechers    = regexp.MustCompile(`(\d+)\s*</td>\s*$`)
	reGenericDigits            = regexp.MustCompile(`(\d+)`)
	reGenericTTGID             = regexp.MustCompile(`/t/(\d+)`)
	reGenericStarSpaceDetailID = regexp.MustCompile(`(?:details|torrent)\.php\?id=(\d+)`)
	reGenericIMDbID            = regexp.MustCompile(`tt\d+`)

	reGenericWelcomeUsername = regexp.MustCompile(`(?i)欢迎[：:]\s*<a[^>]*><span[^>]*>([^<]+)</span>`)
	reGenericLabelUpload     = regexp.MustCompile(`(?i)上传[：:]\s*([\d.]+)\s*(TB|GB|MB|KB|T|G|M|K)`)
	reGenericLabelDownload   = regexp.MustCompile(`(?i)下载[：:]\s*([\d.]+)\s*(TB|GB|MB|KB|T|G|M|K)`)
	reGenericBonus           = regexp.MustCompile(`(?i)魔力[：:]\s*([\d,.]+)`)
	reGenericSeedingCount    = regexp.MustCompile(`(?i)alt=['"]做种数['"][^>]*>\s*(\d+)`)
	reGenericUserClass       = regexp.MustCompile(`(?i)class='uc(\d+)'`)
	reBonusHourTotalSize     = regexp.MustCompile(`合计体积</td>\s*<td>\s*</td>\s*<td[^>]*>\s*([\d.,]+\s*(?:T|G|M|TB|GB|MB))`)

	// hdroute.org patterns (class="header-user-data" structure)
	reHDRouteRatio     = regexp.MustCompile(`(?i)分享率[：:]?\s*<span class="header-user-data">([\d.]+)</span>`)
	reHDRouteUpload    = regexp.MustCompile(`(?i)上传量[：:]?\s*<span class="header-user-data">([\d.]+)&nbsp;(TB|GB|MB|KB|B)</span>`)
	reHDRouteDownload  = regexp.MustCompile(`(?i)下载量[：:]?\s*<span class="header-user-data">([\d.]+)&nbsp;(TB|GB|MB|KB|B)</span>`)
	reHDRouteSeeding   = regexp.MustCompile(`(?i)title="seeding"></figure>\s*(\d+)`)
	reHDRouteSeedSize  = regexp.MustCompile(`(?i)class="peering-size">\(([\d.]+)&nbsp;(TB|GB|MB|KB|B)\)`)
	reHDRouteUserClass = regexp.MustCompile(`(?i)</a></span>\s*<span>\(([A-Za-z0-9]+)\)</span>`)

	// totheglory.im patterns (<font color=xxx> tag structure, colon inside font tag)
	reTTGRatio     = regexp.MustCompile(`(?i)分享率\s*:?\s*</font>\s*<font[^>]*>([\d.]+)</font>`)
	reTTGUpload    = regexp.MustCompile(`(?i)上传量\s*:?\s*</font>\s*<font[^>]*>\s*(?:<a[^>]*>)?\s*([\d.]+)\s*(TB|GB|MB|KB|B)`)
	reTTGDownload  = regexp.MustCompile(`(?i)下载量\s*:?\s*</font>\s*<font[^>]*>\s*(?:<a[^>]*>)?\s*([\d.]+)\s*(TB|GB|MB|KB|B)`)
	reTTGSeeding   = regexp.MustCompile(`(?i)alt=['"]做种中['"][^>]*>(?:\s|&nbsp;)*(?:<[^>]*>)*\s*(\d+)`)
	reTTGBonus     = regexp.MustCompile(`(?i)积分\s*:?\s*<a[^>]*>([\d.]+)</a>`)
	reTTGUserID    = regexp.MustCompile(`(?i)/userdetails\.php\?id=(\d+)`)
	reTTGUserClass = regexp.MustCompile(`(?i)等级\s*</td>\s*<td[^>]*>([^<]+)</td>`)

	starSpaceUserClassMap = map[string]string{
		"0": "未激活", "1": "User", "2": "Power User", "3": "Elite User",
		"4": "Crazy User", "5": "Insane User", "6": "Veteran User",
		"7": "Extreme User", "8": "Ultimate User", "9": "Master User",
		"10": "Star User", "11": "God User",
	}
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
	domain := config.BaseURL
	if domain == "" {
		domain = config.Domain
	}
	if err := httpclient.GlobalDownloadLimiter.AcquireWithLimit(domain, config.DownloadHourlyLimit); err != nil {
		return nil, downloadError("下载限流: "+err.Error(), nil)
	}

	var u string
	if config.DownloadURLTemplate != "" {
		u = buildGenericURL(config, config.DownloadURLTemplate, torrentID)
	} else {
		u = buildGenericURL(config, config.Paths.Detail, torrentID)
		if u == "" {
			// §59.159: NP 族公共单点（download_url.go——双副本收敛）
			u = BuildNexusDownloadURL(baseURLOf(config), torrentID, config.Passkey)
		}
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

	if resp.StatusCode == http.StatusNotFound {
		return nil, notFoundError("种子不存在或已被删除")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, classifySiteError(resp, "download")
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		return nil, &model.AppError{Code: 15001, Message: "返回了 HTML 页面而非种子文件，下载链接可能有误"}
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return nil, downloadError("读取种子数据失败", err)
	}
	if len(data) == 0 {
		return nil, &model.AppError{Code: 15001, Message: "种子数据为空"}
	}
	// §59.159: bencode 嗅探——错误页伪装（Content-Type 非 html 但 body 是页面文本，
	// 如 text/plain 的 stderr 消息页）不当种子数据返回（pusher 加种 HTML 必失败且难排查）
	if data[0] != 'd' && data[0] != 'l' && data[0] != 'i' {
		return nil, &model.AppError{Code: 15002, Message: fmt.Sprintf("响应非 bencode 种子数据（首字节 %q）——疑似错误页", data[0])}
	}
	return data, nil
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

	// §59.263/266: 凭证拦截页检测（公共化——interceptor.go 单点）
	if err := CheckCredentialRedirect(resp); err != nil {
		return nil, err
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

	// §59.263/264/266: 解析真空双守卫（公共化——interceptor.go 单点）
	if err := CheckDetailVacuum(detail); err != nil {
		return nil, err
	}

	detail.Category = NormalizeCategory(detail.Category)

	// 从结构化字段提取禁转标记（标题/副标题/标签）
	detail.Flags = extractFlagsFromStructured(detail.Title, detail.Subtitle, detail.Tags)

	// TTG 特殊检测：页面结构中的禁转标记（非标签体系）
	if strings.Contains(html, "本种子是禁转资源") || strings.Contains(html, `nodistr" value="yes"`) {
		already := false
		for _, f := range detail.Flags {
			if f == "禁转" {
				already = true
				break
			}
		}
		if !already {
			detail.Flags = append(detail.Flags, "禁转")
		}
	}

	return detail, nil
}

func (a *GenericAdapter) GetBatchSLData(ctx context.Context, config *model.SiteConfig, torrentIDs []string) (map[string]*model.SLData, error) {
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
	// §59.156: checkbox 数组标签——同名字段重复写（multipart 数组语义）
	for _, kv := range req.TagArrayFields {
		fw.writeField(kv.Key, kv.Value)
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

	// §59.159: 重定向检查——NP 家族重复上传 302 → details.php?id=N&existed=1
	// （PTNexus 实战佐证/织梦日志）。默认跟随会把落点详情页当成功页，
	// 已存在种 ID 被误判为新发布——CheckRedirect 捕获 existed 参数（transport 继承代理配置）。
	existingID, finalID := "", ""
	redirectClient := &http.Client{
		Timeout:   a.doer.Client.Timeout,
		Transport: a.doer.Client.Transport,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			id := req.URL.Query().Get("id")
			if req.URL.Query().Get("existed") == "1" && id != "" {
				existingID = id
			}
			if id != "" && strings.Contains(req.URL.Path, "details.php") {
				finalID = id
			}
			return nil
		},
	}
	resp, err := redirectClient.Do(httpReq)
	if err != nil {
		return nil, uploadError("上传请求失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}
	html := string(body)

	if resp.StatusCode != http.StatusOK {
		return nil, classifySiteError(resp, "upload")
	}

	// §59.159: 公共判定单点（upload_classify.go——双副本教训后收敛）
	if r := classifyUploadHTML(a.logger, config.Domain, config.Domain, html, existingID, finalID); r != nil {
		return r, nil
	}

	errMsg := "上传失败: 未知响应"
	if m := reGenericErrorClass.FindStringSubmatch(html); len(m) > 1 {
		errMsg = strings.TrimSpace(m[1])
	} else if e := ExtractUploadError(html); e != "" {
		// §59.289: h1+p 组合增强（与 nexusphp 单点共享）
		errMsg = e
	} else if a.logger != nil {
		a.logger.Error("upload unknown response body",
			zap.String("site", config.Domain),
			zap.String("html_head", html[:min(300, len(html))]))
	}

	return nil, &model.AppError{Code: 15001, Message: errMsg}
}

func (a *GenericAdapter) SearchTorrents(ctx context.Context, config *model.SiteConfig, keyword string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
	u := buildSearchURL(config, a.framework, keyword)
	if u == "" {
		return nil, configError("未配置浏览页路径 (paths.browse)")
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
	u = strings.ReplaceAll(u, "{apikey}", url.QueryEscape(config.APIKey))
	u = strings.ReplaceAll(u, "{authkey}", url.QueryEscape(config.AuthKey))
	if !strings.HasPrefix(u, "http") {
		base := config.BaseURL
		if base == "" {
			base = config.Domain
		}
		if !strings.HasPrefix(base, "http") {
			base = "https://" + base
		}
		u = base + "/" + strings.TrimLeft(u, "/")
	}
	return u
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

	// HDRoute: no <tr> rows, use div-based parsing
	if len(rows) == 0 {
		return parseGenericBrowseDiv(html, config)
	}

	rowIdx := 0
	for ; rowIdx < len(rows); rowIdx++ {
		row := rows[rowIdx]
		linkMatch := detailLinkRe.FindStringSubmatch(row)
		if len(linkMatch) < 6 {
			continue
		}

		torrentID := linkMatch[1]
		if torrentID == "" {
			torrentID = linkMatch[2]
		}
		if torrentID == "" {
			torrentID = linkMatch[3]
		}
		if torrentID == "" {
			torrentID = linkMatch[4]
		}
		title := stripTags(strings.TrimSpace(linkMatch[5]))

		if len(results) > 0 && results[len(results)-1].TorrentID == torrentID {
			if results[len(results)-1].Size == 0 {
				if m := sizeRe.FindStringSubmatch(row); len(m) > 2 {
					results[len(results)-1].Size = parseSizeStr(m[1] + " " + m[2])
				}
			}
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
			// Gazelle multi-row: last result may be movieInfo (no size),
			// rowTitle with size was never processed. Scan remaining rows.
			if results[len(results)-1].Size == 0 {
				lastTid := results[len(results)-1].TorrentID
				for j := rowIdx + 1; j < len(rows); j++ {
					lm := detailLinkRe.FindStringSubmatch(rows[j])
					if len(lm) < 6 {
						continue
					}
					tid2 := lm[1]
					if tid2 == "" {
						tid2 = lm[2]
					}
					if tid2 == "" {
						tid2 = lm[3]
					}
					if tid2 == "" {
						tid2 = lm[4]
					}
					if tid2 != lastTid {
						break
					}
					if m := sizeRe.FindStringSubmatch(rows[j]); len(m) > 2 {
						results[len(results)-1].Size = parseSizeStr(m[1] + " " + m[2])
						break
					}
				}
			}
			break
		}
	}

	return results
}

var reHDRouteEntry = regexp.MustCompile(`(?s)title_eng[^>]*>([^<]+)</p>.*?details\.php\?id=(\d+).*?torrent_size[^>]*>([\d.]+)\s*(?:<br\s*/?>)?\s*(GB|MB|TB)`)

func parseGenericBrowseDiv(html string, config *model.SiteConfig) []*model.SeedingSearchResult {
	var results []*model.SeedingSearchResult

	matches := reHDRouteEntry.FindAllStringSubmatch(html, -1)
	for _, m := range matches {
		title := strings.TrimSpace(m[1])
		torrentID := m[2]

		result := &model.SeedingSearchResult{
			TorrentID: torrentID,
			Title:     title,
			DetailURL: config.Domain + "/details.php?id=" + torrentID,
		}

		if len(m) > 4 {
			result.Size = parseSizeStr(m[3] + " " + m[4])
		}

		results = append(results, result)
		if len(results) >= 50 {
			break
		}
	}

	return results
}

func extractIMDbIDGeneric(link string) string {
	if link == "" {
		return ""
	}
	m := reGenericIMDbID.FindString(link)
	return m
}

// baseURLOf BaseURL 优先回落 Domain（§59.159 下载 URL 公共单点配套）。
// baseURLOf 08 刀2：委托 resolveBaseURL（消费点端到端等价——两个调用点
// generic:109/nexusphp_upload:57 全部经 BuildNexusDownloadURL 的幂等
// 补协议+去斜杠兜底·helper 层差异被吞掉·最终 URL 四态一致。
// 本体曾是「裸取」旧形态（不补协议不去斜杠）——等价性依赖下游
// 幂等兜底这一外部前提（附录G §2.2/§2.3：新增不经 BuildNexus 的
// 消费点前必须重新评估）。
func baseURLOf(config *model.SiteConfig) string {
	return resolveBaseURL(config)
}
