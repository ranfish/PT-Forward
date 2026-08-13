package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	htmllib "html"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/metadata/extract"
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

var (
	reNexusTitle          = regexp.MustCompile(`<title>([^<]+)</title>`)
	reNexusRow            = regexp.MustCompile(`(?s)<tr[^>]*>.*?</tr>`)
	reNexusDDBlock        = regexp.MustCompile(`(?s)基本信息</dt>\s*<dd>(.*)</dd>`)
	reNexusFallbackDiv    = regexp.MustCompile(`(?s)基本信息</div>(.*?)种子文件`)
	reNexusSubtitleDD     = regexp.MustCompile(`(?s)副标题</dt>\s*<dd>(.*?)</dd>`)
	reNexusWhitespace     = regexp.MustCompile(`\s+`)
	reNexusHashDD         = regexp.MustCompile(`(?i)Hash码</dt>\s*<dd>\s*([a-fA-F0-9]{40})`)
	reNexusInfoHash       = regexp.MustCompile(`(?i)info_hash.*?([a-fA-F0-9]{40})`)
	reNexusKdescr         = regexp.MustCompile(`(?s)<div[^>]*id=['"]kdescr['"][^>]*>([\s\S]*?)</div>`)
	reNexusDTDDBoundary   = regexp.MustCompile(`</d[t]>\s*<d[d]>`)
	reNexusSizeValue      = regexp.MustCompile(`([\d.]+)\s*(TB|GB|MB|KB|TiB|GiB|MiB|KiB)`)
	reNexusImgAlt         = regexp.MustCompile(`<img[^>]*alt="([^"]+)"`)
	reNexusCatLink        = regexp.MustCompile(`cat=\d+[^>]*>([^<]+)`)
	reNexusRowFollow      = regexp.MustCompile(`(?s)class="rowfollow"[^>]*>(.*?)</td>`)
	reNexusSeedersCount   = regexp.MustCompile(`(\d+)个做种者`)
	reNexusLeechersCount  = regexp.MustCompile(`(\d+)个下载者`)
	reNexusSeedersID      = regexp.MustCompile(`id=['"]seeders['"][^>]*>(?:<[^>]*>)*(\d+)`)
	reNexusLeechersID     = regexp.MustCompile(`id=['"]leechers['"][^>]*>(?:<[^>]*>)*(\d+)`)
	reNexusDetailID       = regexp.MustCompile(`(?:details|detail)\.php\?id=(\d+)`)
	reNexusErrorClass     = regexp.MustCompile(`class="error"[^>]*>([^<]+)`)
	reNexusErrorP         = regexp.MustCompile(`<p[^>]*>([^<]*(?:失败|错误|error|fail|拒绝|duplicate|already)[^<]*)</p>`)
	reNexusBrowseLink     = regexp.MustCompile(`(?s)href=["'](?:[^"']*/)?(?:plugin_)?details\.php\?id=(\d+)["'][^>]*><b>\s*&nbsp;([^<]+)</b>`)
	reNexusBrowseAltLink  = regexp.MustCompile(`(?s)href=["'](?:[^"']*/)?(?:plugin_)?details?\.php\?(?:id|group_id)=(\d+)[^"']*["'][^>]*>(.*?)</a>`)
	reNexusBrowseSize     = regexp.MustCompile(`(?i)class=["']?(?:rowfollow|[^"']*torrent-info-text-size)["']?[^>]*>\s*([\d.]+)\s*(?:&nbsp;)?\s*(?:<br\s*/?>)?\s*(?:&nbsp;)?\s*(TiB|GiB|MiB|KiB|TB|GB|MB|KB|T|G|M)\b`)
	reNexusBrowseSeeders  = regexp.MustCompile(`dllist=1#seeders">\s*(\d+)\s*</a>`)
	reNexusBrowseLeechers = regexp.MustCompile(`dllist=1#leechers">\s*(\d+)\s*</a>`)
	reNexusSizeStr        = regexp.MustCompile(`([\d.]+)\s*(TiB|GiB|MiB|KiB|TB|GB|MB|KB|T|G|M|B)`)
	reNexusDomTTTitle     = regexp.MustCompile(`(?s)onmouseover="domTT_activate\([^,]+,\s*event,\s*'content',\s*'(.*?)'\s*,\s*'trail'`)
	reNexusTag            = regexp.MustCompile(`class="tag[^"]*"[^>]*>([^<]+)`)

	reBBCodeImg     = regexp.MustCompile(`(?i)\[img\](.*?)\[/img\]`)
	reBBCodeHTMLImg = regexp.MustCompile(`(?i)<img[^>]+src=["']([^"']+)["']`)
	reBBCodeQuote   = regexp.MustCompile(`(?is)\[quote(?:=([^\]]*))?\](.*?)\[/quote\]`)
	reIMDbURL       = regexp.MustCompile(`https?://(?:www\.)?imdb\.com/title/(?:tt\d+)`)
	reDoubanURL     = regexp.MustCompile(`https?://(?:www\.)?(?:movie\.)?douban\.com/(?:subject|movie)/(?:\d+)`)
)

type NexusPHPAdapter struct {
	doer   *HTTPDoer
	logger *zap.Logger
	engine *extract.Engine // §56.13 方案 B: 可选 Engine 引用，注入后 GetTorrentDetail 优先走 Engine
}

func NewNexusPHPAdapter(doer *HTTPDoer, logger *zap.Logger) *NexusPHPAdapter {
	return &NexusPHPAdapter{doer: doer, logger: logger}
}

// SetEngine 注入 Engine（§56.13 方案 B）。
// 启动时由 adapter.Factory.Create 调用。engine == nil 时退化为 legacy regexp 提取。
func (a *NexusPHPAdapter) SetEngine(e *extract.Engine) { a.engine = e }

func (a *NexusPHPAdapter) Framework() string { return "nexusphp" }

func (a *NexusPHPAdapter) ParseRSS(_ context.Context, _ string, _ *model.SiteConfig) ([]*model.RSSTorrentEvent, error) {
	return nil, parseError("ParseRSS: use RSS fetcher instead", nil)
}

func (a *NexusPHPAdapter) DownloadTorrent(ctx context.Context, config *model.SiteConfig, torrentID string) ([]byte, error) {
	domain := config.BaseURL
	if domain == "" {
		domain = config.Domain
	}
	if err := httpclient.GlobalDownloadLimiter.Acquire(domain); err != nil {
		return nil, downloadError("下载限流: "+err.Error(), nil)
	}

	var u string
	if config.DownloadMode == "signed" {
		signedURL, err := a.resolveSignedDownloadURL(ctx, config, torrentID)
		if err != nil {
			return nil, err
		}
		u = signedURL
	} else if config.DownloadURLTemplate != "" {
		u = buildTemplateURL(config, config.DownloadURLTemplate, torrentID)
	} else {
		u = buildURL(config.Domain, "/download.php", torrentID, config.Passkey)
	}

	method := "GET"
	if config.DownloadMode == "signed" {
		method = "POST"
	}

	req, err := http.NewRequestWithContext(ctx, method, u, nil)
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
	return data, nil
}

var reSignedDownloadURL = regexp.MustCompile(`download\.php\?id=` + regexp.QuoteMeta("{id}") + `&t=\d+&sign=[a-f0-9]+`)

func (a *NexusPHPAdapter) resolveSignedDownloadURL(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	detailURL := buildDetailsURL(config.Domain, torrentID, config.DetailsURLTemplate)

	req, err := http.NewRequestWithContext(ctx, "GET", detailURL, nil)
	if err != nil {
		return "", networkError("构造详情页请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return "", downloadError("获取详情页失败", err)
	}
	defer func() { drainBody(resp) }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return "", downloadError("读取详情页失败", err)
	}

	idEscaped := regexp.QuoteMeta(torrentID)
	pattern := regexp.MustCompile(`download\.php\?id=` + idEscaped + `&t=\d+&sign=[a-f0-9]+`)
	match := pattern.FindString(string(body))
	if match == "" {
		return "", configError("未在详情页找到签名下载链接")
	}

	if !strings.HasPrefix(match, "http") {
		base := config.Domain
		if !strings.HasPrefix(base, "http") {
			base = "https://" + base
		}
		match = base + "/" + strings.TrimLeft(match, "/")
	}

	return match, nil
}

func (a *NexusPHPAdapter) GetTorrentDetail(ctx context.Context, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, error) {
	html, err := a.fetchDetailsHTML(ctx, config, torrentID)
	if err != nil {
		return nil, err
	}

	// §56.34 OpenCD 专用提取（音乐站 DOM 与标准 NexusPHP 差异大）
	if isOpenCDDomain(config.Domain) {
		return extractOpenCDDetail(html), nil
	}

	// §56.13 方案 B: 优先用 Engine 提取（更精准：goquery + 站点 hook + 标准化键映射）
	if a.engine != nil {
		detail, ok := a.extractWithEngine(html, config, torrentID)
		if ok {
			return detail, nil
		}
		if a.logger != nil {
			a.logger.Warn("nexusphp: engine extraction not meaningful, fallback to legacy regexp")
		}
	}

	// fallback: 老 regexp 提取（保留作 Engine 失败时的兜底）
	return a.legacyExtractDetail(html), nil
}

// fetchDetailsHTML 负责详情页 HTTP GET，返回原始 HTML。
func (a *NexusPHPAdapter) fetchDetailsHTML(ctx context.Context, config *model.SiteConfig, torrentID string) (string, error) {
	u := buildDetailsURL(config.Domain, torrentID, config.DetailsURLTemplate)

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return "", networkError("构造请求失败", err)
	}
	setCommonHeaders(req, config.Cookie)

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return "", networkError("请求详情页失败", err)
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode != http.StatusOK {
		return "", httpError(fmtES("HTTP %d", resp.StatusCode), nil)
	}

	body, err := readBody(resp)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// extractWithEngine 用 Engine 提取详情页字段。
// 返回 (detail, true) 表示提取有意义；返回 (nil, false) 表示 Engine 失败需 fallback。
func (a *NexusPHPAdapter) extractWithEngine(html string, config *model.SiteConfig, torrentID string) (*model.TorrentDetail, bool) {
	input := extract.Input{
		SiteCode:     deriveSiteCode(config.Domain),
		Domain:       config.Domain,
		SiteNickname: config.SiteName,
		BaseURL:      config.BaseURL,
		TorrentID:    torrentID,
		PageHTML:     html,
	}
	seed, meta := a.engine.Extract(input)
	if !seed.IsMeaningful() {
		return nil, false
	}
	detail := extract.SeedToDetail(seed)
	if detail == nil {
		return nil, false
	}
	detail.RawHTML = html
	if meta.ExtractorName != "" {
		detail.EngineExtractorName = meta.ExtractorName
	}
	return detail, true
}

// legacyExtractDetail 老 regexp 提取逻辑（原 GetTorrentDetail 主体）。
// 保留作 Engine 失败时的 fallback；Engine 成功时不会调用。
func (a *NexusPHPAdapter) legacyExtractDetail(html string) *model.TorrentDetail {
	detail := &model.TorrentDetail{}

	if m := reNexusTitle.FindStringSubmatch(html); len(m) > 1 {
		detail.Title = strings.TrimSpace(m[1])
	}

	for _, row := range reNexusRow.FindAllString(html, -1) {
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
		ddBlockRe := reNexusDDBlock
		if m := ddBlockRe.FindStringSubmatch(html); len(m) > 1 {
			extractBasicInfoFields(m[1], detail)
		}
		fallbackDivRe := reNexusFallbackDiv
		if m := fallbackDivRe.FindStringSubmatch(html); len(m) > 1 {
			extractBasicInfoFields(m[1], detail)
		}
	}

	if detail.Subtitle == "" {
		ddSubRe := reNexusSubtitleDD
		if m := ddSubRe.FindStringSubmatch(html); len(m) > 1 {
			text := stripTags(m[1])
			text = reNexusWhitespace.ReplaceAllString(text, " ")
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
		if m := reNexusHashDD.FindStringSubmatch(html); len(m) > 1 {
			detail.InfoHash = strings.ToLower(m[1])
		}
	}

	if detail.InfoHash == "" {
		if m := reNexusInfoHash.FindStringSubmatch(html); len(m) > 1 {
			detail.InfoHash = strings.ToLower(m[1])
		}
	}

	if m := reNexusKdescr.FindStringSubmatch(html); len(m) > 1 {
		detail.Description = strings.TrimSpace(m[1])
	}

	detail.Tags = extractTags(html)
	detail.Category = NormalizeCategory(detail.Category)

	if detail.Description != "" {
		detail.Screenshots = extractScreenshotsFromBBCode(detail.Description)
		detail.MediaInfo = extractMediaInfoFromBBCode(detail.Description)
	}

	if len(detail.Screenshots) > 0 {
		detail.PosterURL = detail.Screenshots[0]
		detail.Screenshots = detail.Screenshots[1:]
	}

	// 只从结构化字段提取禁转标记（不扫描简介正文，避免误判）
	detail.Flags = extractFlagsFromStructured(detail.Title, detail.Subtitle, detail.Tags)
	if imdbURL := reIMDbURL.FindString(html); imdbURL != "" {
		detail.IMDbURL = imdbURL
	}
	if doubanURL := reDoubanURL.FindString(html); doubanURL != "" {
		detail.DoubanURL = doubanURL
	}

	detail.RawHTML = html
	detail.EngineExtractorName = "legacy_regexp"
	return detail
}

func cleanRowText(row string) string {
	text := stripTags(row)
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = reNexusWhitespace.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func extractBasicInfoFields(row string, detail *model.TorrentDetail) {
	preprocessed := reNexusDTDDBoundary.ReplaceAllString(row, "：")
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
				if sm := reNexusSizeValue.FindStringSubmatch(val); len(sm) > 2 {
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
	imgRe := reNexusImgAlt
	for _, m := range imgRe.FindAllStringSubmatch(row, -1) {
		alt := strings.TrimSpace(m[1])
		if alt == "" || alt == "Show/Hide" || alt == "显示/隐藏" {
			continue
		}
		detail.Category = alt
		return
	}
	if m := reNexusCatLink.FindStringSubmatch(row); len(m) > 1 {
		detail.Category = strings.TrimSpace(m[1])
	}
}

func extractSubtitleFromRow(row string, detail *model.TorrentDetail) {
	tdRe := reNexusRowFollow
	m := tdRe.FindStringSubmatch(row)
	if len(m) < 2 {
		return
	}
	text := stripTags(m[1])
	text = reNexusWhitespace.ReplaceAllString(text, " ")
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
	defer func() { drainBody(resp) }()

	body, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	html := string(body)

	if resp.StatusCode != http.StatusOK {
		return nil, classifySiteError(resp, "upload")
	}

	if idMatch := reNexusDetailID.FindStringSubmatch(html); len(idMatch) > 1 {
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
	if m := reNexusErrorClass.FindStringSubmatch(html); len(m) > 1 {
		errMsg = strings.TrimSpace(m[1])
	} else if m := reNexusErrorP.FindStringSubmatch(html); len(m) > 1 {
		errMsg = strings.TrimSpace(m[1])
	}

	return nil, &model.AppError{Code: 15001, Message: errMsg}
}

func (a *NexusPHPAdapter) SearchTorrents(ctx context.Context, config *model.SiteConfig, keyword string, opts *model.SearchOptions) ([]*model.SeedingSearchResult, error) {
	u := config.Domain
	if !strings.HasPrefix(u, "http") {
		u = "https://" + u
	}

	browsePaths := []string{"/browse.php", "/torrents.php"}
	searchParam := config.Paths.SearchParam

	if browsePaths[0] != config.Paths.Browse && config.Paths.Browse != "" {
		browsePaths = []string{config.Paths.Browse}
	} else if d, ok := getSiteSearchDefaults(config.Domain); ok {
		if d.Browse != "" {
			browsePaths = []string{d.Browse}
		}
		if searchParam == "" && d.Param != "" {
			searchParam = d.Param
		}
	}

	if searchParam == "" {
		searchParam = "search"
	}

	catParam := ""
	if opts != nil && opts.Category != "" {
		catParam = "&cat=" + opts.Category
	}

	var lastErr error
	for _, bp := range browsePaths {
		searchURL := u + bp + "?" + searchParam + "=" + url.QueryEscape(keyword) + "&search_area=0&search_mode=0&incldead=1" + catParam

		if strings.Contains(config.Domain, "audiences") {
			a.logger.Info("nexusphp search debug", zap.String("url", searchURL), zap.String("bp", bp))
		}

		req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
		if err != nil {
			return nil, searchError("构造搜索请求失败", err)
		}
		setCommonHeaders(req, config.Cookie)

		resp, err := a.doer.Client.Do(req)
		if err != nil {
			lastErr = searchError(fmtES("search %s failed: %v", bp, err), nil)
			continue
		}

		if resp.StatusCode == http.StatusNotFound {
			drainBody(resp)
			lastErr = httpError(fmtES("HTTP %d on %s", resp.StatusCode, bp), nil)
			continue
		}

		body, readErr := readBody(resp)
		if readErr != nil {
			lastErr = readErr
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = classifySiteError(resp, "search")
			continue
		}

		parsed := parseNexusPHPBrowse(string(body), config)
		if strings.Contains(config.Domain, "tey.cc") {
		}
		if len(parsed) == 0 && (strings.Contains(config.Domain, "haidan") || strings.Contains(config.Domain, "hdcity")) {
			a.logger.Info("haidan debug", zap.String("bp", bp), zap.Int("body_len", len(body)))
		}
		if len(parsed) == 0 && strings.Contains(config.Domain, "audiences") {
			titleStr := "no title"
			if m := regexp.MustCompile(`<title>([^<]+)`).FindStringSubmatch(string(body)); len(m) > 1 {
				titleStr = m[1]
				if len(titleStr) > 50 {
					titleStr = titleStr[:50]
				}
			}
			a.logger.Info("audiences parse result",
				zap.String("bp", bp),
				zap.Int("body_len", len(body)),
				zap.String("title", titleStr))
		}
		return parsed, nil
	}

	return nil, lastErr
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

func (a *NexusPHPAdapter) SupportsSearchByPiecesHash() bool { return true }

// §33.32 §33.34 — SearchByPiecesHash calls NexusPHP pieces-hash API
//
// Authentication modes:
//   - passkey (standard NP, 52 sites): passkey in body, cookie optional
//   - cookie-only (keepfrds): cookie required, passkey ignored
//
// API paths (auto-fallback):
//   - /api/pieces-hash (standard, 52 sites)
//   - /api/torrents/pieces-hash (keepfrds variant)
//
// Response formats:
//   - standard NP: {"ret":0,"msg":"...","data":{"hash":tid}}
//   - keepfrds:    {"data":{"hash":tid}}  (no ret/msg fields)
//
// Max 100 hashes per request (NexusPHP TorrentRepository::getPiecesHashCache limit).
func (a *NexusPHPAdapter) SearchByPiecesHash(ctx context.Context, config *model.SiteConfig, piecesHashes []string) (map[string]int, error) {
	if len(piecesHashes) == 0 {
		return nil, nil
	}

	hasPasskey := config.Passkey != "" && len(config.Passkey) == 32
	hasCookie := config.Cookie != ""
	if !hasPasskey && !hasCookie {
		return nil, configError("pieces_hash API 需要 passkey 或 cookie 认证")
	}

	u := config.Domain
	if config.APIDomain != "" {
		u = config.APIDomain
	}
	if !strings.HasPrefix(u, "http") {
		u = "https://" + u
	}

	const maxBatch = 100
	hashes := piecesHashes
	if len(hashes) > maxBatch {
		hashes = hashes[:maxBatch]
	}

	bodyMap := map[string]any{
		"pieces_hash": hashes,
	}
	if hasPasskey {
		bodyMap["passkey"] = config.Passkey
	}
	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, parseError("构造 pieces_hash 请求失败", err)
	}

	paths := []string{"/api/pieces-hash", "/api/torrents/pieces-hash"}
	var lastErr error
	for _, path := range paths {
		result, err := a.doPiecesHashRequest(ctx, u+path, bodyBytes, config.Cookie)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !isPiecesHash404(err) {
			return nil, err
		}
	}
	return nil, lastErr
}

func isPiecesHash404(err error) bool {
	appErr, ok := err.(*model.AppError)
	if !ok {
		return false
	}
	return strings.Contains(appErr.Message, "HTTP 404")
}

func (a *NexusPHPAdapter) doPiecesHashRequest(ctx context.Context, apiURL string, body []byte, cookie string) (map[string]int, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, networkError("构造 pieces_hash 请求失败", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	resp, err := a.doer.Client.Do(req)
	if err != nil {
		return nil, networkError("pieces_hash API 请求失败", err)
	}
	defer func() { drainBody(resp) }()

	if resp.StatusCode != http.StatusOK {
		return nil, httpError(fmtES("pieces_hash API HTTP %d", resp.StatusCode), nil)
	}

	respBody, err := readBody(resp)
	if err != nil {
		return nil, err
	}

	var apiResp struct {
		Ret  int            `json:"ret"`
		Msg  string         `json:"msg"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, parseError("解析 pieces_hash 响应失败", err)
	}

	if apiResp.Ret != 0 {
		return nil, authError(fmtES("pieces_hash API 返回错误: %s", apiResp.Msg), nil)
	}

	result := make(map[string]int, len(apiResp.Data))
	for ph, v := range apiResp.Data {
		switch n := v.(type) {
		case float64:
			result[ph] = int(n)
		case string:
			if id, err := strconv.Atoi(n); err == nil {
				result[ph] = id
			}
		}
	}
	return result, nil
}

func (a *NexusPHPAdapter) VerifyExists(ctx context.Context, config *model.SiteConfig, torrentID string) (bool, error) {
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

func buildTemplateURL(config *model.SiteConfig, tpl, torrentID string) string {
	u := tpl
	if torrentID != "" {
		u = strings.ReplaceAll(u, "{id}", url.PathEscape(torrentID))
	}
	u = strings.ReplaceAll(u, "{passkey}", url.QueryEscape(config.Passkey))
	u = strings.ReplaceAll(u, "{rsskey}", url.QueryEscape(config.RSSKey))
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

func buildURL(domain, path, torrentID, passkey string) string {
	u := domain
	if !strings.HasPrefix(u, "http") {
		u = "https://" + u
	}
	u = u + path + "?id=" + url.QueryEscape(torrentID)
	if passkey != "" {
		u += "&passkey=" + url.QueryEscape(passkey)
	}
	return u
}

// buildDetailsURL constructs the details page URL. If detailsURLTemplate is
// provided (e.g. "/t-{id}"), it takes precedence over the default /details.php?id={id}.
func buildDetailsURL(domain, torrentID, detailsURLTemplate string) string {
	if detailsURLTemplate != "" {
		u := strings.ReplaceAll(detailsURLTemplate, "{id}", url.QueryEscape(torrentID))
		if !strings.HasPrefix(u, "http") {
			base := domain
			if !strings.HasPrefix(base, "http") {
				base = "https://" + base
			}
			u = base + u
		}
		return u
	}
	return buildURL(domain, "/details.php", torrentID, "")
}

func setCommonHeaders(req *http.Request, cookie string) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
}

func parseNexusPHPBrowse(html string, config *model.SiteConfig) []*model.SeedingSearchResult {

	detailLinkRe := reNexusBrowseLink
	altDetailLinkRe := reNexusBrowseAltLink
	sizeRe := reNexusBrowseSize
	seedersRe := reNexusBrowseSeeders
	leechersRe := reNexusBrowseLeechers

	seen := map[string]bool{}

	// Check for "no results" page (poster grid shows latest torrents, not search results)
	if strings.Contains(html, "没有种子") || strings.Contains(html, "Nothing found") {
		return nil
	}

	detailMatches := detailLinkRe.FindAllStringSubmatchIndex(html, -1)
	if len(detailMatches) == 0 {
		detailMatches = altDetailLinkRe.FindAllStringSubmatchIndex(html, -1)
	}

	// Fallback: HDCiTY uses href="t-{id}" format (no .php, no details)
	if len(detailMatches) == 0 {
		return parseHDCiTYBrowse(html, config)
	}

	// Two-pass strategy: first collect text-link entries (list table with sizes),
	// then if not enough, fall back to image-only/domTT entries (poster grid).
	var textResults, imgResults []*model.SeedingSearchResult

	for _, loc := range detailMatches {
		torrentID := html[loc[2]:loc[3]]

		rawContent := html[loc[4]:loc[5]]
		title := strings.TrimSpace(stripTags(strings.ReplaceAll(rawContent, "&nbsp;", " ")))
		isImageOnly := strings.Contains(rawContent, "<img") || strings.Contains(html[loc[0]:loc[1]], "onmouseover")

		// domTT fallback
		if title == "" {
			linkTag := html[loc[0]:min(loc[0]+1000, len(html))]
			if dm := reNexusDomTTTitle.FindStringSubmatch(linkTag); len(dm) > 1 {
				title = strings.TrimSpace(stripTags(htmllib.UnescapeString(dm[1])))
				title = strings.ReplaceAll(title, "\\'", "'")
				title = strings.ReplaceAll(title, "\\\"", "\"")
				if idx := strings.IndexAny(title, "["); idx > 10 {
					title = strings.TrimSpace(title[:idx])
				}
				if idx := strings.Index(title, "\n"); idx > 0 {
					title = title[:idx]
				}
			}
		}

		// Check if entry is in a list table row (has rowfollow nearby)
		// vs poster grid (no rowfollow)
		trStart := strings.LastIndex(html[:loc[0]], "<tr")
		trEnd := strings.Index(html[loc[0]:], "</tr>")
		if trStart >= 0 && trEnd > 0 {
			trContent := html[trStart : loc[0]+trEnd+5]
			if strings.Contains(trContent, "rowfollow") {
				isImageOnly = false
			}
		}

		if title == "" {
			seen[torrentID] = true
			continue
		}

		// Short titles (<=3 chars) without image = seeders/leechers count links
		// Use them to update existing entry's size, then skip
		if len(title) <= 3 && !isImageOnly && seen[torrentID] {
			// Search both directions from the link for rowfollow size cell
			wideStart := loc[0] - 500
			if wideStart < 0 {
				wideStart = 0
			}
			wideEnd := loc[1] + 500
			if wideEnd > len(html) {
				wideEnd = len(html)
			}
			wideChunk := html[wideStart:wideEnd]
			for _, list := range [][]*model.SeedingSearchResult{textResults, imgResults} {
				for _, r := range list {
					if r.TorrentID == torrentID && r.Size == 0 {
						if m := sizeRe.FindStringSubmatch(wideChunk); len(m) > 2 {
							r.Size = parseSizeStr(m[1] + " " + m[2])
						} else if m := reNexusSizeStr.FindStringSubmatch(wideChunk); len(m) > 2 {
							r.Size = parseSizeStr(m[1] + " " + m[2])
						}
					}
				}
			}
			continue
		}

		// Allow list table entries (non-image) to override poster grid entries
		if !isImageOnly && seen[torrentID] {
			for i := len(imgResults) - 1; i >= 0; i-- {
				if imgResults[i].TorrentID == torrentID {
					imgResults = append(imgResults[:i], imgResults[i+1:]...)
				}
			}
			alreadyInText := false
			for _, r := range textResults {
				if r.TorrentID == torrentID {
					alreadyInText = true
					break
				}
			}
			if alreadyInText {
				continue
			}
		}
		if seen[torrentID] && isImageOnly {
			continue
		}
		seen[torrentID] = true

		start := loc[0]
		end := loc[1]
		if end+20000 <= len(html) {
			end += 20000
		} else {
			end = len(html)
		}
		chunk := html[start:end]

		// §59.26: 标题补全——部分 NexusPHP 站点（如 keepfrds）在 <a> 内只放中文格式化标题，
		// 原始英文名（含分辨率+制作组后缀）在 </a> 后的同 <td> 内。
		// 如果提取的标题不含制作组后缀模式（-XXXXX），在 </a> 后搜索原始种子名。
		if title != "" && !hasGroupSuffix(title) {
			afterLink := html[loc[1]:min(loc[1]+2000, len(html))]
			afterText := stripTags(strings.ReplaceAll(afterLink, "&nbsp;", " "))
			afterText = strings.TrimSpace(afterText)
			if containsResolutionKeyword(afterText) {
				if idx := strings.IndexAny(afterText, "\n<"); idx > 10 {
					afterText = strings.TrimSpace(afterText[:idx])
				}
				// 清理尾部 []（stripTags 残留）
				afterText = strings.TrimRight(afterText, "[]")
				afterText = strings.TrimSpace(afterText)
				if afterText != "" {
					title = afterText
				}
			}
		}

		result := &model.SeedingSearchResult{
			TorrentID: torrentID,
			Title:     title,
			DetailURL: config.Domain + "/details.php?id=" + torrentID,
		}

		// Size: search chunk for rowfollow size cell
		if m := sizeRe.FindStringSubmatch(chunk); len(m) > 2 {
			result.Size = parseSizeStr(m[1] + " " + m[2])
		} else if m := reNexusSizeStr.FindStringSubmatch(chunk); len(m) > 2 {
			result.Size = parseSizeStr(m[1] + " " + m[2])
		}

		if m := seedersRe.FindStringSubmatch(chunk); len(m) > 1 {
			result.Seeders, _ = strconv.Atoi(m[1])
		}

		if m := leechersRe.FindStringSubmatch(chunk); len(m) > 1 {
			result.Leechers, _ = strconv.Atoi(m[1])
		}

		result.PublishAt = time.Now()

		if isImageOnly {
			imgResults = append(imgResults, result)
		} else {
			if torrentID == "582" && strings.Contains(config.Domain, "tey") {
			}
			textResults = append(textResults, result)
		}

		if len(textResults)+len(imgResults) >= 50 {
			break
		}
	}

	// Prefer text-link results (list table with sizes), fall back to image-only (poster grid)
	if len(textResults) > 0 {
		return textResults
	}
	return imgResults
}

var reHDCiTYEntry = regexp.MustCompile(`href="t-(\d+)"[^>]*>([^<]+)`)
var reHDCiTYSize = regexp.MustCompile(`sd_storage[^>]*>\s*([\d.]+)\s*(GB|MB|TB|GiB|MiB)`)

func parseHDCiTYBrowse(html string, config *model.SiteConfig) []*model.SeedingSearchResult {
	matches := reHDCiTYEntry.FindAllStringSubmatchIndex(html, -1)
	var results []*model.SeedingSearchResult
	for _, loc := range matches {
		tid := html[loc[2]:loc[3]]
		title := strings.TrimSpace(html[loc[4]:loc[5]])
		title = strings.ReplaceAll(title, "&nbsp;", " ")
		if len(title) < 5 || strings.HasPrefix(title, "[") {
			continue
		}
		result := &model.SeedingSearchResult{
			TorrentID: tid,
			Title:     title,
			DetailURL: config.Domain + "/t-" + tid,
		}
		chunk := html[loc[0]:min(loc[0]+5000, len(html))]
		if sm := reHDCiTYSize.FindStringSubmatch(chunk); len(sm) > 2 {
			result.Size = parseSizeStr(sm[1] + " " + sm[2])
		}
		results = append(results, result)
		if len(results) >= 50 {
			break
		}
	}
	return results
}

// containsResolutionKeyword §59.26: 检查标题是否含分辨率关键词（2160p/1080p 等）
func containsResolutionKeyword(s string) bool {
	lower := strings.ToLower(s)
	for _, kw := range []string{"2160p", "1080p", "1080i", "720p", "576p", "480p", "1440p", "4320p", "4k"} {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// hasGroupSuffix §59.26: 检查标题末尾是否有制作组后缀模式（-XXXXX 至少 2 个字母数字）
func hasGroupSuffix(s string) bool {
	return regexp.MustCompile(`-[A-Za-z0-9]{2,}$`).MatchString(strings.TrimSpace(s))
}

func parseSizeStr(s string) int64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")

	m := reNexusSizeStr.FindStringSubmatch(s)
	if len(m) < 3 {
		return 0
	}

	val, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}

	switch strings.ToUpper(m[2]) {
	case "TB", "TIB", "T":
		return int64(val * 1024 * 1024 * 1024 * 1024)
	case "GB", "GIB", "G":
		return int64(val * 1024 * 1024 * 1024)
	case "MB", "MIB", "M":
		return int64(val * 1024 * 1024)
	case "KB", "KIB", "K":
		return int64(val * 1024)
	default:
		return int64(val)
	}
}

func extractTags(html string) []string {
	tagRe := reNexusTag
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

var (
	reNexusUsername             = regexp.MustCompile(`(?i)<a[^>]*href="[^"]*userdetails?\.php\?id=(\d+)"[^>]*><b[^>]*>([^<]+)`)
	reNexusUsernameAlt          = regexp.MustCompile(`(?i)<a[^>]*href="[^"]*userdetails?\.php\?id=(\d+)"[^>]*>([^<]+)</a>`)
	reNexusUsernameSpan         = regexp.MustCompile(`(?i)<a[^>]*href="[^"]*userdetails?\.php\?id=(\d+)"[^>]*><[^>]*><span[^>]*>([^<]+)</span>`)
	reNexusUserID               = regexp.MustCompile(`userdetails?\.php\?id=(\d+)`)
	reNexusUserNameClass        = regexp.MustCompile(`(?i)class='([^']+_Name)'`)
	reNexusUserClassSpan        = regexp.MustCompile(`(?i)<a[^>]*href="[^"]*userdetails?\.php\?id=\d+"[^>]*>[^<]*</a></span>\s*<span>\s*[\[(]([^)\]]+)[\])]\s*</span>`)
	reNexusUserClassStrong      = regexp.MustCompile(`(?i)</strong>\[([^\]]+)\]`)
	reNexusBonusBeforeLabel     = regexp.MustCompile(`(?i)(\d[\d,.]+)\s*<[^>]*>\s*(?:魅力值?|魔力|火花|UCoin|Bonus|憨豆|元宝|蝌蚪)\s*</[^>]+>`)
	reNexusUsernameByName       = regexp.MustCompile(`(?i)<a[^>]*class='[^']*_Name'[^>]*href="[^"]*userdetails?\.php\?id=(\d+)"[^>]*><b[^>]*>([^<]+)`)
	reNexusUsernameByName2      = regexp.MustCompile(`(?i)<a[^>]*href="[^"]*userdetails?\.php\?id=(\d+)"[^>]*class='[^']*_Name'[^>]*><b[^>]*>([^<]+)`)
	reNexusUsernameUUID         = regexp.MustCompile(`(?i)<a\s[^>]*class='[^']*_Name'[^>]*><b[^>]*>([^<]+)`)
	reNexusUserUUID             = regexp.MustCompile(`userdetails\.php\?uuid=([a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})`)
	reNexusUsernameDeep         = regexp.MustCompile(`(?is)<a[^>]*href=["'][^"']*userdetails?\.php\?id=(\d+)["'][^>]*>(.*?)</a>`)
	reNexusInfoBlock            = regexp.MustCompile(`(?s)id="info_block"(.*?)</table>`)
	reNexusWelcomeBack          = regexp.MustCompile(`(?is)(?:欢迎[回来]|Welcome\s+back|你好).{0,200}?<a[^>]*href="[^"]*userdetails?\.php\?id=(\d+)"[^>]*>(.*?)</a>`)
	reNexusUsernameSimple       = regexp.MustCompile(`(?is)<a[^>]*href="userdetails?"[^>]*>.*?<strong>([^<]+)</strong>`)
	reNexusFontUploaded         = regexp.MustCompile(`(?i)class\s*=\s*['"]?color_uploaded['"]?[^>]*>[^<]*</(?:font|span)>(?:\s*<[^>]*>)*\s*([\d.,]+\s*(?:PiB|TiB|GiB|MiB|KiB|TB|GB|MB|KB|B)?)`)
	reNexusFontDownloaded       = regexp.MustCompile(`(?i)class\s*=\s*['"]?color_downloaded['"]?[^>]*>[^<]*</(?:font|span)>(?:\s*<[^>]*>)*\s*([\d.,]+\s*(?:PiB|TiB|GiB|MiB|KiB|TB|GB|MB|KB|B)?)`)
	reNexusFontRatio            = regexp.MustCompile(`(?i)class\s*=\s*['"]?color_ratio['"]?>[^<]*</(?:font|span)>\s*([\d.,]+)`)
	reNexusFontBonus            = regexp.MustCompile(`(?i)class\s*=\s*['"]?color_bonus['"]?>[^<]*</(?:font|span)>\s*(?:<[^>]*>)?\s*(?:\[[^\]]*\]\s*)?:?\s*([\d,]+\.\d+|[\d,]+\d)`)
	reNexusFontBonusInline      = regexp.MustCompile(`(?i)class\s*=\s*['"]?color_bonus['"]?>(?:魔力|时魔|啤酒|茉莉|火花|魅力值?|UCoin|Bonus|憨豆|元宝|蝌蚪)[^:<]*:\s*([\d,]+\.\d+)\s*</(?:font|span)>`)
	reNexusArrowUp              = regexp.MustCompile(`(?i)class="arrowup"[^>]*(?:/>|>)([^<]*)`)
	reNexusArrowUpImg           = regexp.MustCompile(`(?i)(?:arrowup|做种中)[^>]*>(?:\s|&nbsp;)*(?:<(?:font|span)[^>]*>)*\s*(\d+)\s*(?:</(?:font|span)>)?`)
	reSpriteArrowSeeding        = regexp.MustCompile(`(?is)sprite_arrowup[^>]*>(?:</figure>)?\s*(\d+)\s*</a>\s*(?:&nbsp;)*\s*<span[^>]*class="peering-size"[^>]*>\(([\d.,]+(?:&nbsp;|\s)*(?:PiB|TiB|GiB|MiB|KiB|TB|GB|MB|KB|B))\)`)
	reNexusJsonSeeding          = regexp.MustCompile(`(?i)&quot;(?:活跃|做种数|做种|seeding|active)[：:]?&quot;,&quot;value&quot;:&quot;[↑↑\s]*(\d+)`)
	reNexusSeedLabel            = regexp.MustCompile(`(?i)(?:做种数|seeding)\s*[：:]\s*(\d+)`)
	reMaterialPlayArrow         = regexp.MustCompile(`(?i)play_arrow</i>\s*<span[^>]*>(\d+)\s*</span>`)
	reAltSeedingCount           = regexp.MustCompile(`(?i)alt=["']做种数["'][^>]*>(?:\s|&nbsp;)*(\d+)`)
	reFontTitleSeeding          = regexp.MustCompile(`(?i)title=["']当前做种["'][^>]*>(?:<[^>]*>)?\s*⬆(?:</[^>]*>)?\s*(\d+)`)
	reNexusLabelUpload          = regexp.MustCompile(`(?i)(?:上传量|Uploaded|上傳量)\s*(?:<[^>]*>)*\s*[:：]?\s*([\d.,]+(?:\s|&nbsp;)*(?:PiB|TiB|GiB|MiB|KiB|TB|GB|MB|KB|B)?)`)
	reNexusLabelUploadRelaxed   = regexp.MustCompile(`(?i)(?:上传量|Uploaded|上傳量)\s*(?:：|:)?\s*(?:</?(?:font|span|a|b|i|div|img)[^>]*>)*(?:\s|&nbsp;)*(?:<[^>]*>)*\s*([\d.,]+(?:\s|&nbsp;)*(?:PiB|TiB|GiB|MiB|KiB|TB|GB|MB|KB|B))`)
	reNexusLabelDownload        = regexp.MustCompile(`(?i)(?:下载量|Downloaded|下載量)\s*(?:<[^>]*>)*\s*[:：]?\s*([\d.,]+(?:\s|&nbsp;)*(?:PiB|TiB|GiB|MiB|KiB|TB|GB|MB|KB|B)?)`)
	reNexusLabelDownloadRelaxed = regexp.MustCompile(`(?i)(?:下载量|Downloaded|下載量)\s*(?:：|:)?\s*(?:</?(?:font|span|a|b|i|div|img)[^>]*>)*(?:\s|&nbsp;)*(?:<[^>]*>)*\s*([\d.,]+(?:\s|&nbsp;)*(?:PiB|TiB|GiB|MiB|KiB|TB|GB|MB|KB|B))`)
	reNexusLabelRatio           = regexp.MustCompile(`(?i)(?:分享率|Share\s*Ratio|Ratio)\s*(?:<[^>]*>)+\s*([\d.,]+)`)
	reNexusLabelBonus           = regexp.MustCompile(`(?i)(?:魔力|茉莉|火花|魅力值?|UCoin|Bonus|憨豆|元宝|蝌蚪)\s*(?:<[^>]*>)+\s*(?:.*?:\s*)?([\d,.]+)`)
	reNexusDataStats            = regexp.MustCompile(`(?i)&quot;label&quot;:&quot;上传量：&quot;,&quot;value&quot;:&quot;([\d.,]+\s*(?:PB|TB|GB|MB|KB|B))&quot;`)
	reNexusIconTitleUpload      = regexp.MustCompile(`(?i)title=["']上传量[：:]["'][^>]*>[^<]*</[a-z]>\s*([\d.,]+\s*(?:PiB|TiB|GiB|MiB|KiB|TB|GB|MB|KB|B))`)
	reNexusIconTitleDown        = regexp.MustCompile(`(?i)title=["']下载量[：:]["'][^>]*>[^<]*</[a-z]>\s*([\d.,]+\s*(?:PiB|TiB|GiB|MiB|KiB|TB|GB|MB|KB|B))`)
	reNexusIconTitleRatio       = regexp.MustCompile(`(?i)title=["']分享率[：:]["'][^>]*>[^<]*</[a-z]>\s*([\d.,]+)`)
	reNexusDataBonus            = regexp.MustCompile(`(?i)&quot;label&quot;:&quot;(?:魔力|爆米花|茉莉|火花|魅力值?|UCoin|Bonus|憨豆|元宝|蝌蚪)[^&]*&quot;,&quot;value&quot;:&quot;([\d,.]+)&quot;`)
	reNexusApiUser              = regexp.MustCompile(`"username"`)
	reDetailTransfer            = regexp.MustCompile(`(?is)class=["']?rowhead["']?[^>]*>(?:传输|傳送|Transfer)[^<]*</td>\s*<td[^>]*>(.+)`)
	reDetailUpload              = regexp.MustCompile(`(?i)(?:上[传傳]量|Uploaded)\s*[:：]?\s*([\d.,]+\s*(?:PB|TB|GB|MB|KB|B))`)
	reDetailDownload            = regexp.MustCompile(`(?i)(?:下[载載]量|Downloaded)\s*[:：]?\s*([\d.,]+\s*(?:PB|TB|GB|MB|KB|B))`)
	reDetailClass               = regexp.MustCompile(`(?is)class=["']?rowhead["']?[^>]*>(?:等级|等級|Class)\s*</td>\s*<td[^>]*>(.*?)</td>`)
	reDetailClassImg            = regexp.MustCompile(`<img[^>]*title=["']([^"']+)["']`)
	reDetailBonus               = regexp.MustCompile(`(?is)class=["']?rowhead["']?[^>]*>(?:魔力|Bonus|Karma|积分|茉莉|火花|魅力值?|UCoin|憨豆|元宝|蝌蚪)[^<]*</td>\s*<td[^>]*>(.*?)</td>`)
	reUcoinValue                = regexp.MustCompile(`(?is)class=["']?rowhead["']?[^>]*>UCoin.*?<td[^>]*>.*?title="([\d,.]+)"`)
	reDetailSeedingPoints       = regexp.MustCompile(`(?is)class=["']?rowhead["']?[^>]*>做种积分[^<]*</td>\s*<td[^>]*>(.*?)</td>`)
	reDetailSeedingPointsB      = regexp.MustCompile(`(?i)做种积分[^<]*</b>[\s:]*([\d,]+\.?\d*)`)
	reDetailSeeding             = regexp.MustCompile(`(?is)class=["']?rowhead["']?[^>]*>(?:做种活动|当前做种|做种体积|做种数)\s*</td>\s*<td[^>]*>(.*?)</td>`)
	reDetailSeedingSimple       = regexp.MustCompile(`(?is)class=["']?rowhead["']?[^>]*>做种\s*</td>\s*<td[^>]*>(.*?)</td>`)
	reDetailSeedingSize         = regexp.MustCompile(`(?is)class=["']?rowhead["']?[^>]*>做种大小\s*</td>\s*<td[^>]*>(.*?)</td>`)
	reDetailArrowUpSize         = regexp.MustCompile(`(?is)class=["']?arrowup["']?[^>]*>\s*([\d,.]+\s*(?:TB|GB|MB|TiB|GiB|MiB|PB|PiB))`)
	reDetailUsername            = regexp.MustCompile(`(?is)class=["']?rowhead["']?[^>]*>(?:用户名|用戶名|Username)\s*</td>\s*<td[^>]*>(.*?)</td>`)
	reDetailH1                  = regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)
	reDetailRowheadUpload       = regexp.MustCompile(`(?is)class=["']?rowhead["']?[^>]*>(?:上[传傳]量|Uploaded)\s*</td>\s*<td[^>]*>(.*?)</td>`)
	reUserIDLoose               = regexp.MustCompile(`userdetails?(?:\.php)?\?id=(\d+)`)
	reIDField                   = regexp.MustCompile(`(?i)<b>ID</b>\s*(?:&nbsp;|\s)*(\d+)`)
	reAJAXSummary               = regexp.MustCompile(`(?i)(?:<[^>]*>)?(\d+)(?:<[^>]*>)?\s*条记录.*?大小[：:]?\s*([\d.,]+\s*(?:PiB|TiB|GiB|MiB|KiB|TB|GB|MB|KB))`)
	reBonusSeedingSize          = regexp.MustCompile(`(?i)当前做种体积[^<]*<[^>]*>([\d.,]+\s*(?:PiB|TiB|GiB|MiB|KiB|TB|GB|MB|KB))`)
	reDetailSeedingVol          = regexp.MustCompile(`([\d.,]+\s*(?:PiB|TiB|GiB|MiB|KiB|TB|GB|MB|KB))`)
	reDetailRowheadDownload     = regexp.MustCompile(`(?is)class=["']?rowhead["']?[^>]*>(?:下[载載]量|Downloaded)\s*</td>\s*<td[^>]*>(.*?)</td>`)
	reDetailRowheadRatio        = regexp.MustCompile(`(?is)class=["']?rowhead["']?[^>]*>(?:分享率|Ratio)\s*</td>\s*<td[^>]*>(.*?)</td>`)
	reNexusJsonUpload           = regexp.MustCompile(`(?i)&quot;上[传傳]量[：:]&quot;,&quot;value&quot;:&quot;(\d[\d.,]+\s*(?:PB|TB|GB|MB|KB))&quot;`)
	reNexusJsonDownload         = regexp.MustCompile(`(?i)&quot;下[载載]量[：:]&quot;,&quot;value&quot;:&quot;(\d[\d.,]+\s*(?:PB|TB|GB|MB|KB))&quot;`)
	reNexusJsonRatio            = regexp.MustCompile(`(?i)&quot;分享率[：:]&quot;,&quot;value&quot;:&quot;([\d.]+)&quot;`)
	reNexusJsonBonus            = regexp.MustCompile(`(?i)&quot;(?:魔力|积分|茉莉|火花|魅力值?|UCoin|Bonus|憨豆|元宝|蝌蚪)[：:]&quot;,&quot;value&quot;:&quot;([\d,.]+)&quot;`)
	reAltUpload                 = regexp.MustCompile(`(?i)alt=["']上传["'][^>]*>(?:\s|&nbsp;)*([\d.,]+)\s*(PiB|TB|GB|MB|KB)`)
	reAltDownload               = regexp.MustCompile(`(?i)alt=["']下载["'][^>]*>(?:\s|&nbsp;)*([\d.,]+)\s*(PiB|TB|GB|MB|KB)`)
	reAltRatio                  = regexp.MustCompile(`(?i)\[分享率\]\s*[:：]\s*(?:&nbsp;|\s)*([\d.,]+)`)
	reAltBonus                  = regexp.MustCompile(`(?is)alt=["']憨豆["'][^>]*>.*?<div[^>]*>([\d,.]+)\s*</div>`)
	reFontLabelUpload           = regexp.MustCompile(`(?i)(?:上传|上传量)\s*[:：]\s*</(?:font|span)>\s*([\d.,]+)\s*(PiB|TB|GB|MB|KB)`)
	reFontLabelDownload         = regexp.MustCompile(`(?i)(?:下载|下载量)\s*[:：]\s*</(?:font|span)>\s*([\d.,]+)\s*(PiB|TB|GB|MB|KB)`)
	reFontLabelRatio            = regexp.MustCompile(`(?i)分享率\s*[:：]\s*</(?:font|span)>\s*≥?\s*([\d.,]+)`)
	reFontLabelBonus            = regexp.MustCompile(`(?is)魔力值[^<]*</(?:font|span)>.*?]:\s*([\d,.]+)`)
	reFontLabelBonusSpan        = regexp.MustCompile(`(?is)魔力值[^<]*</(?:font|span)>.*?]:\s*(?:<[^>]*>)*\s*([\d,.]+)`)
	reFontArrowSeeding          = regexp.MustCompile(`(?i)当前做种[^>]*>[^⬆]*⬆</(?:font|span)>\s*(\d+)`)
	reLinkBonus                 = regexp.MustCompile(`(?i)魔力值\s*\(([\d,.]+)\)`)
	reYuanbaoBonus              = regexp.MustCompile(`(?s)元宝.*?mybonus\.php[^>]*>([\d,.]+)`)
)

func (a *NexusPHPAdapter) FetchUserStats(ctx context.Context, config *model.SiteConfig) (*model.UserStatsResult, error) {
	stats, err := a.fetchUserStatsAPI(ctx, config)
	if err == nil && stats != nil {
		if stats.SeedingCount > 0 && stats.SeedingSize > 0 {
			a.scrapePasskey(ctx, config, stats)
			if config.DownloadMode == "cuhash" {
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
		if config.DownloadMode == "cuhash" {
			a.scrapeCuhash(ctx, config, stats)
		}
		return stats, nil
	}
	stats, err = a.fetchUserStatsHTML(ctx, config)
	if err == nil && stats != nil {
		a.scrapePasskey(ctx, config, stats)
		if config.DownloadMode == "cuhash" {
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

func (a *NexusPHPAdapter) scrapeCuhash(ctx context.Context, config *model.SiteConfig, stats *model.UserStatsResult) {
	if config.Cookie == "" {
		return
	}
	pageURL := config.Domain + "/index.php"
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
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return
	}
	if m := reCuhashFromLink.FindStringSubmatch(string(body)); len(m) > 1 {
		stats.Passkey = m[1]
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

	if m := reNexusFontUploaded.FindStringSubmatch(searchHTML); len(m) > 1 {
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

func jsonStr(data map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := data[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

func jsonInt(data map[string]interface{}, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := data[k]; ok {
			switch n := v.(type) {
			case float64:
				return int64(n)
			case string:
				if i, err := strconv.ParseInt(n, 10, 64); err == nil {
					return i
				}
			}
		}
	}
	return 0
}

func jsonFloat(data map[string]interface{}, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := data[k]; ok {
			switch n := v.(type) {
			case float64:
				return n
			case string:
				if f, err := strconv.ParseFloat(n, 64); err == nil {
					return f
				}
			}
		}
	}
	return 0
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

func extractScreenshotsFromBBCode(text string) []string {
	seen := make(map[string]bool)
	var urls []string
	for _, m := range reBBCodeImg.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			u := strings.TrimSpace(m[1])
			if u != "" && !seen[u] && !isUnwantedImage(u) {
				seen[u] = true
				urls = append(urls, u)
			}
		}
	}
	for _, m := range reBBCodeHTMLImg.FindAllStringSubmatch(text, -1) {
		if len(m) > 1 {
			u := strings.TrimSpace(m[1])
			if u != "" && !seen[u] && !isUnwantedImage(u) {
				seen[u] = true
				urls = append(urls, u)
			}
		}
	}
	return urls
}

func extractMediaInfoFromBBCode(text string) string {
	for _, m := range reBBCodeQuote.FindAllStringSubmatch(text, -1) {
		label := ""
		if len(m) > 1 {
			label = strings.ToLower(m[1])
		}
		content := ""
		if len(m) > 2 {
			content = strings.TrimSpace(m[2])
		}
		if content == "" {
			continue
		}
		preview := content
		if len(preview) > 200 {
			preview = preview[:200]
		}
		if strings.Contains(label, "mediainfo") ||
			strings.Contains(label, "bdinfo") ||
			strings.Contains(label, "编码信息") ||
			strings.Contains(strings.ToLower(preview), "general") {
			return content
		}
	}
	return ""
}

var flagKeywords = []string{"禁转", "禁止转载", "谢绝转载", "严禁转载", "谢绝搬运", "独占", "限时禁转"}

// extractFlagsFromStructured 只从结构化字段（标题/副标题/标签）提取禁转标记。
// 不扫描简介正文，避免"禁转PTT"等误判（§56.37 合规修复）。
func extractFlagsFromStructured(title, subtitle string, tags []string) []string {
	combined := title + " " + subtitle
	for _, t := range tags {
		combined += " " + t
	}
	return extractFlagsFromText(combined)
}

func extractFlagsFromText(text string) []string {
	var flags []string
	seen := make(map[string]bool)
	for _, kw := range flagKeywords {
		if strings.Contains(text, kw) && !seen[kw] {
			seen[kw] = true
			flags = append(flags, kw)
		}
	}
	return flags
}

var unwantedImagePatterns = []string{"ico", "logo", "banner", "icon", "emoji", "smiley", "rank_", "badge"}

func isUnwantedImage(url string) bool {
	lower := strings.ToLower(url)
	for _, pattern := range unwantedImagePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// deriveSiteCode 从 site.Domain 推导 site_code（如 pterclub.net → "pterclub"）。
// Engine.specialByCode 用此 key 匹配站点特殊提取器（如 pterclub_special）。
// 推导失败返回空串（Engine 会自动 fallback 到 public extractor）。
func deriveSiteCode(domain string) string {
	if domain == "" {
		return ""
	}
	host := domain
	if u, err := url.Parse(domain); err == nil && u.Hostname() != "" {
		host = u.Hostname()
	}
	if i := strings.IndexByte(host, ':'); i > 0 {
		host = host[:i]
	}
	// 剥掉 www. 前缀（v0.0.252：之前 www.xxx.com 会返回 "www"，导致 19 站 siteCode 都是 "www"）
	host = strings.TrimPrefix(host, "www.")
	parts := strings.Split(host, ".")
	if len(parts) == 0 {
		return ""
	}
	return strings.ToLower(parts[0])
}
