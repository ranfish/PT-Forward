package adapter

import (
	"context"
	htmllib "html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/browser"
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
	reNexusTitle         = regexp.MustCompile(`<title>([^<]+)</title>`)
	reNexusRow           = regexp.MustCompile(`(?s)<tr[^>]*>.*?</tr>`)
	reNexusDDBlock       = regexp.MustCompile(`(?s)基本信息</dt>\s*<dd>(.*)</dd>`)
	reNexusFallbackDiv   = regexp.MustCompile(`(?s)基本信息</div>(.*?)种子文件`)
	reNexusSubtitleDD    = regexp.MustCompile(`(?s)副标题</dt>\s*<dd>(.*?)</dd>`)
	reNexusWhitespace    = regexp.MustCompile(`\s+`)
	reNexusHashDD        = regexp.MustCompile(`(?i)Hash码</dt>\s*<dd>\s*([a-fA-F0-9]{40})`)
	reNexusInfoHash      = regexp.MustCompile(`(?i)info_hash.*?([a-fA-F0-9]{40})`)
	reNexusKdescr        = regexp.MustCompile(`(?s)<div[^>]*id=['"]kdescr['"][^>]*>([\s\S]*?)</div>`)
	reNexusDTDDBoundary  = regexp.MustCompile(`</d[t]>\s*<d[d]>`)
	reNexusSizeValue     = regexp.MustCompile(`([\d.]+)\s*(TB|GB|MB|KB|TiB|GiB|MiB|KiB)`)
	reNexusImgAlt        = regexp.MustCompile(`<img[^>]*alt="([^"]+)"`)
	reNexusCatLink       = regexp.MustCompile(`cat=\d+[^>]*>([^<]+)`)
	reNexusRowFollow     = regexp.MustCompile(`(?s)class="rowfollow"[^>]*>(.*?)</td>`)
	reNexusSeedersCount  = regexp.MustCompile(`(\d+)个做种者`)
	reNexusLeechersCount = regexp.MustCompile(`(\d+)个下载者`)
	reNexusSeedersID     = regexp.MustCompile(`id=['"]seeders['"][^>]*>(?:<[^>]*>)*(\d+)`)
	reNexusLeechersID    = regexp.MustCompile(`id=['"]leechers['"][^>]*>(?:<[^>]*>)*(\d+)`)
	// §59.159 词边界：防 userdetails.php 误命中（实战四次假成功真因——失败页用户导航链接）
	reNexusErrorClass    = regexp.MustCompile(`class="error"[^>]*>([^<]+)`)
	reNexusErrorP        = regexp.MustCompile(`<p[^>]*>([^<]*(?:失败|错误|error|fail|拒绝|duplicate|already)[^<]*)</p>`)
	reNexusBrowseLink    = regexp.MustCompile(`(?s)href=["'](?:[^"']*/)?(?:plugin_)?details\.php\?id=(\d+)["'][^>]*><b>\s*&nbsp;([^<]+)</b>`)
	reNexusBrowseAltLink = regexp.MustCompile(`(?s)href=["'](?:[^"']*/)?(?:plugin_)?details?\.php\?(?:id|group_id)=(\d+)[^"']*["'][^>]*>(.*?)</a>`)
	// §59.473: 海胆组行双层链接（group_id=N&torrent_id=M）——优先于
	// altLink（altLink 捕组 id 会错投 tid·海胆搜索行实证）。仅含
	// torrent_id 参数的行命中·其它站零影响。
	reNexusGroupTorrentLink = regexp.MustCompile(`(?s)href=["'](?:[^"']*/)?details?\.php\?group_id=\d+&(?:amp;)?torrent_id=(\d+)["'][^>]*>(.*?)</a>`)
	// §59.510: t_size 分支——hdwing（瓷器）列表 size 单元格 class="t_size"
	// （`799.12&nbsp;GB`·&nbsp; 分隔），原 rowfollow/torrent-info-text-size
	// 两分支 miss → size 恒 0；加分支后用户样本 100/100 全中。
	reNexusBrowseSize     = regexp.MustCompile(`(?i)class=["']?(?:rowfollow|t_size|[^"']*torrent-info-text-size)["']?[^>]*>\s*([\d.]+)\s*(?:&nbsp;)?\s*(?:<br\s*/?>)?\s*(?:&nbsp;)?\s*(TiB|GiB|MiB|KiB|TB|GB|MB|KB|T|G|M)\b`)
	reNexusBrowseSeeders  = regexp.MustCompile(`dllist=1#seeders">\s*(\d+)\s*</a>`)
	reNexusBrowseLeechers = regexp.MustCompile(`dllist=1#leechers">\s*(\d+)\s*</a>`)
	reNexusSizeStr        = regexp.MustCompile(`([\d.]+)\s*(TiB|GiB|MiB|KiB|TB|GB|MB|KB|T|G|M|B)`)
	reNexusDomTTTitle     = regexp.MustCompile(`(?s)onmouseover="domTT_activate\([^,]+,\s*event,\s*'content',\s*'(.*?)'\s*,\s*'trail'`)
	// §59.369 附二: torrent-content-tag 前缀（深度站 NP 新版前端标签形态）
	reNexusTag = regexp.MustCompile(`class="(?:tag|torrent-content-tag)[^"]*"[^>]*>([^<]+)`)

	reBBCodeImg     = regexp.MustCompile(`(?i)\[img\](.*?)\[/img\]`)
	reBBCodeHTMLImg = regexp.MustCompile(`(?i)<img[^>]+src=["']([^"']+)["']`)
	reBBCodeQuote   = regexp.MustCompile(`(?is)\[quote(?:=([^\]]*))?\](.*?)\[/quote\]`)
	reIMDbURL       = regexp.MustCompile(`https?://(?:www\.)?imdb\.com/title/(?:tt\d+)`)
	reDoubanURL     = regexp.MustCompile(`https?://(?:www\.)?(?:movie\.)?douban\.com/(?:subject|movie)/(?:\d+)`)
)

type NexusPHPAdapter struct {
	doer      *HTTPDoer
	logger    *zap.Logger
	engine    *extract.Engine   // §56.13 方案 B: 可选 Engine 引用，注入后 GetTorrentDetail 优先走 Engine
	browser   *browser.Fetcher  // §59.336: 浏览器 fallback（详情页 HTTP 失败时 Chromium 兜底）
	listCache keepfrdsListCache // §59.340 B: 列表页批次缓存（page=0 同 URL 批量去重）
	hdwCache  keepfrdsListCache // §59.510: hdwing 列表缓存（NP adapter 单例——与 keepfrds HTML 不同防互踩）
}

func NewNexusPHPAdapter(doer *HTTPDoer, logger *zap.Logger) *NexusPHPAdapter {
	return &NexusPHPAdapter{doer: doer, logger: logger}
}

// SetBrowserFetcher 注入浏览器获取器（§59.336）。
// nil 时不启用 fallback（默认）——容器无 Chromium 时优雅降级。
func (a *NexusPHPAdapter) SetBrowserFetcher(f *browser.Fetcher) { a.browser = f }

// SetEngine 注入 Engine（§56.13 方案 B）。
// 启动时由 adapter.Factory.Create 调用。engine == nil 时退化为 legacy regexp 提取。
func (a *NexusPHPAdapter) SetEngine(e *extract.Engine) { a.engine = e }

func (a *NexusPHPAdapter) Framework() string { return "nexusphp" }

func (a *NexusPHPAdapter) ParseRSS(_ context.Context, _ string, _ *model.SiteConfig) ([]*model.RSSTorrentEvent, error) {
	return nil, parseError("ParseRSS: use RSS fetcher instead", nil)
}

func buildTemplateURL(config *model.SiteConfig, tpl, torrentID string) string {
	u := tpl
	if torrentID != "" {
		u = strings.ReplaceAll(u, "{id}", url.PathEscape(torrentID))
	}
	u = strings.ReplaceAll(u, "{passkey}", url.QueryEscape(config.Passkey))
	u = strings.ReplaceAll(u, "{apikey}", url.QueryEscape(config.APIKey))
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
	httpclient.SetBrowserHeaders(req.Header, cookie)
}

func parseNexusPHPBrowse(html string, config *model.SiteConfig) []*model.SeedingSearchResult {
	return parseNexusPHPBrowseLimit(html, config, 0)
}

// parseNexusPHPBrowseLimit §59.486: limit>0 覆盖默认 50 上限——皇后等
// 单页全量站（61 条无分页）在"同体积真身排 60+ 位"场景（Monstercat
// 77.8GB 案 tid=72218 位次 61）被默认 50 截断丢弃——音乐第三轮大上限
// （150）覆盖。
func parseNexusPHPBrowseLimit(html string, config *model.SiteConfig, limit int) []*model.SeedingSearchResult {
	if limit <= 0 {
		limit = 50
	}

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
		// §59.473: 海胆组行双层（group_id&torrent_id）优先于 altLink——
		// altLink 对该形态捕组 id 错投 tid
		detailMatches = reNexusGroupTorrentLink.FindAllStringSubmatchIndex(html, -1)
	}
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
			// 判定信号：分辨率词（完整显示）或 年份+大写开头（站点 CSS 截断显示，如 "216.."）
			if containsResolutionKeyword(afterText) || looksLikeSceneName(afterText) {
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
			textResults = append(textResults, result)
		}

		if len(textResults)+len(imgResults) >= limit {
			break
		}
	}

	// Prefer text-link results (list table with sizes), fall back to image-only (poster grid)
	if len(textResults) > 0 {
		return textResults
	}
	return imgResults
}

// searchLimitOf §59.486: opts.MaxResults>0 → 解析上限（默认 50）。
func searchLimitOf(opts *model.SearchOptions) int {
	if opts != nil && opts.MaxResults > 0 {
		return opts.MaxResults
	}
	return 0
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

// looksLikeSceneName §59.26: 判定 </a> 后文本是否像 scene 命名的原始种子名。
// 用于站点 CSS 截断显示（如 "216.." 替代 "2160p"）时分辨率词缺失的兜底：
// 要求首词大写字母开头 + 含年份（19xx/20xx）+ 总长 ≥ 15。
func looksLikeSceneName(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 15 {
		return false
	}
	first := rune(s[0])
	if first < 'A' || first > 'Z' {
		return false
	}
	return regexp.MustCompile(`\b(19|20)\d{2}\b`).MatchString(s)
}

// hasGroupSuffix §59.26: 检查标题末尾是否有制作组后缀模式（-XXXXX 至少 2 个字母数字）
// §59.182: 字符类加 @——@ 子组署名（-DIY@UBits / -cXcY@FRDS，§59.179 同族）不含 @ 时
// 完整英文标题被误判"无后缀"→ §59.26 补全用 </a> 后中文副标题覆盖英文主标题。
func hasGroupSuffix(s string) bool {
	return regexp.MustCompile(`-[A-Za-z0-9@]{2,}$`).MatchString(strings.TrimSpace(s))
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
	// §59.332: keepfrds 新版（NexusPHP tailwind/htmx——2026-10-05 改版，
	// /api/user 下线+DOM 全换）。做种体积站方已不提供（页面体系全无展示——
	// 首页/userdetails/mybonus/列表页汇总均无），SeedingSize 保持 0。
	reKfNewSeedingCount = regexp.MustCompile(`(?is)title=["']当前做种["'][^>]*>\s*</i>\s*(\d+)`)
	reKfNewUploaded     = regexp.MustCompile(`(?i)class=["']?uploaded["']?>([\d.,]+\s*(?:PiB|TiB|GiB|MiB))`)
	reKfNewDownloaded   = regexp.MustCompile(`(?i)class=["']?downloaded["']?>([\d.,]+\s*(?:PiB|TiB|GiB|MiB))`)
	reKfNewRatio        = regexp.MustCompile(`(?is)color_ratio["']?>\s*分享率：?</span>\s*([\d.]+)`)
	reKfNewBonus        = regexp.MustCompile(`(?i)id=["']?totalBonus["']?>([\d,.]+)`)
	reLinkBonus         = regexp.MustCompile(`(?i)魔力值\s*\(([\d,.]+)\)`)
	reYuanbaoBonus      = regexp.MustCompile(`(?s)元宝.*?mybonus\.php[^>]*>([\d,.]+)`)
)

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
