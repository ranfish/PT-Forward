package rss

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type Fetcher struct {
	client      *http.Client
	clientCache sync.Map
	logger      *zap.Logger
}

func NewFetcher(logger *zap.Logger, tlsPretend ...string) *Fetcher {
	cfg := httpclient.SiteHTTPConfig{}
	if len(tlsPretend) > 0 {
		cfg.TLSPretend = tlsPretend[0] // §59.333
	}
	return &Fetcher{
		client: httpclient.NewSiteHTTPClient(cfg),
		logger: logger,
	}
}

func NewFetcherWithClient(client *http.Client, logger *zap.Logger) *Fetcher {
	return &Fetcher{
		client: client,
		logger: logger,
	}
}

func (f *Fetcher) Fetch(ctx context.Context, url string) (*RSSFeed, error) {
	return f.fetchWithClient(ctx, url, f.client)
}

func (f *Fetcher) FetchWithProxy(ctx context.Context, url, proxyURL string, skipSSLVerify bool, tlsPretend ...string) (*RSSFeed, error) {
	return f.fetchWithClient(ctx, url, f.clientFor(proxyURL, skipSSLVerify, tlsPretend...))
}

func (f *Fetcher) clientFor(proxyURL string, skipSSLVerify bool, tlsPretend ...string) *http.Client {
	tp := ""
	if len(tlsPretend) > 0 {
		tp = tlsPretend[0]
	}
	if proxyURL == "" && !skipSSLVerify && tp == "" {
		return f.client
	}
	key := proxyURL + "|" + strconv.FormatBool(skipSSLVerify) + "|" + tp
	if cached, ok := f.clientCache.Load(key); ok {
		return cached.(*http.Client)
	}
	c := httpclient.NewSiteHTTPClient(httpclient.SiteHTTPConfig{
		ProxyURL:      proxyURL,
		SkipSSLVerify: skipSSLVerify,
		TLSPretend:    tp, // §59.333
	})
	actual, _ := f.clientCache.LoadOrStore(key, c)
	return actual.(*http.Client)
}

func (f *Fetcher) fetchWithClient(ctx context.Context, url string, client *http.Client) (*RSSFeed, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, rssError(ErrRSSNetwork, "构造请求失败", err)
	}
	httpclient.SetBrowserHeaders(req.Header, "")

	resp, err := client.Do(req)
	if err != nil {
		return nil, rssError(ErrRSSNetwork, "请求失败", err)
	}
	defer func() { httpclient.DrainBody(resp) }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, rssError(ErrRSSNetwork, fmt.Sprintf("RSS 返回非成功状态码: %d", resp.StatusCode), nil)
	}

	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if !strings.Contains(ct, "xml") && !strings.Contains(ct, "rss") && !strings.Contains(ct, "octet-stream") {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		preview := strings.TrimSpace(string(data))
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return nil, rssError(ErrRSSParse, fmt.Sprintf("RSS 返回非 XML 内容 (Content-Type: %s): %s", ct, preview), nil)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, rssError(ErrRSSNetwork, "读取响应失败", err)
	}

	var feed RSSFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, rssError(ErrRSSParse, "解析 XML 失败", err)
	}

	return &feed, nil
}

func (f *Fetcher) ParseItems(feed *RSSFeed, sub *model.RSSSubscription, site *model.Site) []*model.RSSTorrentEvent {
	items := make([]*model.RSSTorrentEvent, 0, len(feed.Channel.Items))

	for _, item := range feed.Channel.Items {
		event := &model.RSSTorrentEvent{
			SourceRSS: sub.Name,
			SiteName:  sub.SiteName,
			Title:     item.Title,
			Category:  item.Category,
			Metadata:  map[string]any{},
		}

		event.TorrentID = extractTorrentID(item, site)
		event.DownloadURL = extractDownloadURL(item, site)
		event.InfoHash = extractInfoHash(item, site, event.TorrentID)
		event.Size = extractSize(item, site)
		// §59.456 附: RSS 内优惠证据（PeerGo 系 feed）双通道：
		//   ① description "优惠：xxx（至 …）" 行——promotionLabel 枚举
		//     精确集（未知形态不干预——NP 站自然语言误短路防线）
		//   ② GUID 前缀 "peergo:torrent:" 且无优惠行=站方权威 NONE
		//     （回归审核补：首版仅①——49 颗无优惠种每轮全量打 SPA
		//     详情页 8s×N 超时/骨架 HTML 假检测；feed 无优惠即无折扣）
		if level, freeEnd := discountFromRSSDescription(item.Description); level != "" {
			event.Metadata["rss_discount_level"] = level
			if freeEnd != nil {
				event.Metadata["rss_free_end"] = freeEnd.Format(time.RFC3339)
			}
		} else if strings.HasPrefix(item.GUID, "peergo:torrent:") {
			event.Metadata["rss_discount_level"] = string(model.DiscountNone)
		}

		// §59.515 刀3 D1: 好大增强 RSS 扩展节点消费——
		// ① 免费截止时间 → FreeEndAt（唯一改变行为的新信息：
		//   刷流规则可设「免费剩余>N 才下」防过期误下）。三通道：
		//   nexusphp:free_until（生产实态主通道）> promotion_until
		//   （官方文档形态兜底）> torznab:attr freeuntil（第二通道）
		// ② torznab:attr downloadvolumefactor=0 → IsFree（好大 feed 内
		//   唯一 IsFree 证据源——回归审核实证 RSS 链路无 [免费] 前缀
		//   判定通道；downloadvolumefactor 是 Torznab 行业标准权威数据）
		// 局部名匹配——其他 NP 站无此元素→零值·零影响。
		freeUntil := strings.TrimSpace(item.FreeUntil)
		if freeUntil == "" {
			freeUntil = strings.TrimSpace(item.PromotionUntil)
		}
		if freeUntil == "" {
			for _, attr := range item.TorznabAttrs {
				if attr.Name == "freeuntil" && attr.Value != "" {
					freeUntil = strings.TrimSpace(attr.Value)
					break
				}
			}
		}
		if freeUntil != "" {
			if t, err := time.ParseInLocation("2006-01-02 15:04:05", freeUntil, time.Local); err == nil {
				event.FreeEndAt = &t
			}
		}
		for _, attr := range item.TorznabAttrs {
			if attr.Name == "downloadvolumefactor" && attr.Value == "0" {
				event.IsFree = true
			}
		}

		if event.TorrentID == "" && event.DownloadURL == "" {
			continue
		}

		if event.TorrentID == "" {
			event.TorrentID = event.InfoHash
		}

		items = append(items, event)
	}

	return items
}

type RSSFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel RSSChannel `xml:"channel"`
}

type RSSChannel struct {
	Title string    `xml:"title"`
	Link  string    `xml:"link"`
	Items []RSSItem `xml:"item"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate"`
	Category    string `xml:"category"`

	Enclosure RSSEnclosure `xml:"enclosure"`

	CustomHash string `xml:"hash"`
	CustomSize string `xml:"size"`

	// §59.515 刀3 D1: 好大增强 RSS 扩展节点（Go encoding/xml
	// 按局部名匹配——其他 NP 站无此元素→零值·全站解析器零影响）
	// 生产实态（2026-11-12 实拉 spstate=9 五条实证）：免费截止
	// 元素=free_until（官方文档样例为 promotion_until——文档形态
	// 保留兜底·两形态同解析）。
	PromotionUntil string        `xml:"promotion_until"`
	FreeUntil      string        `xml:"free_until"`
	TorznabAttrs   []TorznabAttr `xml:"attr"`
}

// TorznabAttr 好大 RSS 的 torznab:attr 键值对（name/value 属性）。
type TorznabAttr struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type RSSEnclosure struct {
	URL    string `xml:"url,attr"`
	Length string `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}

func extractTorrentID(item RSSItem, site *model.Site) string {
	strategy := site.IDStrategy
	if strategy == "" {
		strategy = "query_param"
	}

	link := item.Link
	if link == "" {
		link = item.Enclosure.URL
	}
	if link == "" {
		link = item.GUID
	}

	switch strategy {
	case "query_param":
		return extractQueryParam(link, site.IDPattern)
	case "path_segment":
		return extractPathSegment(link, site.IDPattern)
	case "link_regex":
		return extractByRegex(link, site.IDPattern)
	case "guid_regex":
		return extractByRegex(item.GUID, site.IDPattern)
	case "guid_text":
		return item.GUID
	default:
		return ""
	}
}

func extractDownloadURL(item RSSItem, site *model.Site) string {
	if item.Enclosure.URL != "" {
		return item.Enclosure.URL
	}
	if item.Link != "" && strings.Contains(item.Link, "download") {
		return item.Link
	}
	return item.Link
}

func extractInfoHash(item RSSItem, site *model.Site, torrentID string) string {
	strategy := site.HashStrategy
	if strategy == "" {
		strategy = "guid"
	}

	switch strategy {
	case "guid":
		h := strings.TrimPrefix(item.GUID, "magnet:?xt=urn:btih:")
		if len(h) == 40 {
			return strings.ToLower(h)
		}
		return strings.ToLower(item.GUID)
	case "xml_tag":
		return strings.ToLower(item.CustomHash)
	case "fake_from_id":
		if torrentID != "" {
			return "fakehash" + torrentID + "fakehash"
		}
		return strings.ToLower(item.CustomHash)
	default:
		return strings.ToLower(item.CustomHash)
	}
}

func extractSize(item RSSItem, site *model.Site) int64 {
	strategy := site.SizeStrategy
	if strategy == "" {
		strategy = "enclosure"
	}

	switch strategy {
	case "enclosure":
		return parseSize(item.Enclosure.Length)
	case "xml_tag":
		return parseSize(item.CustomSize)
	default:
		return parseSize(item.Enclosure.Length)
	}
}

func extractQueryParam(urlStr, param string) string {
	if param == "" {
		param = "id"
	}
	lower := strings.ToLower(urlStr)
	target := strings.ToLower(param) + "="
	idx := strings.Index(lower, target)
	if idx < 0 {
		return ""
	}
	start := idx + len(target)
	rest := urlStr[start:]
	end := strings.IndexAny(rest, "&#?")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

func extractPathSegment(urlStr, pattern string) string {
	parts := strings.Split(strings.TrimRight(urlStr, "/"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func extractByRegex(s, pattern string) string {
	if pattern == "" || s == "" {
		return ""
	}
	re, err := compileRegex(pattern)
	if err != nil {
		return ""
	}
	matches := re.FindStringSubmatch(s)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func parseSize(s string) int64 {
	if s == "" {
		return 0
	}
	var n int64
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}

const regexCacheMaxSize = 256

var regexCache = map[string]*regexp.Regexp{}
var regexCacheOrder []string
var regexCacheMu sync.RWMutex

func compileRegex(pattern string) (*regexp.Regexp, error) {
	regexCacheMu.RLock()
	if re, ok := regexCache[pattern]; ok {
		regexCacheMu.RUnlock()
		return re, nil
	}
	regexCacheMu.RUnlock()
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	regexCacheMu.Lock()
	if _, exists := regexCache[pattern]; !exists {
		if len(regexCache) >= regexCacheMaxSize {
			evict := regexCacheOrder[0]
			delete(regexCache, evict)
			regexCacheOrder = regexCacheOrder[1:]
		}
		regexCache[pattern] = re
		regexCacheOrder = append(regexCacheOrder, pattern)
	}
	regexCacheMu.Unlock()
	return re, nil
}


// rssDiscountLineRe §59.456: PeerGo 系 feed 的优惠行（站方
// promotionLabel 中文集：免费/2× 上传/2× 上传 / 免费/50% 下载/
// 2× 上传 / 50% 下载/30% 下载——rss/service.go promotionLabel 权威）。
// 判定语义对齐 IsFree：免费/2× 上传·免费=免费；纯上传优惠与折扣下载
// 不算免费（§59.367 PERCENT_50 不算同源）。
var rssDiscountLineRe = regexp.MustCompile(`优惠[:：]\s*([^\n（(]+)(?:[（(]\s*至\s*([0-9]{4}-[0-9]{2}-[0-9]{2}\s+[0-9]{2}:[0-9]{2})\s*[）)])?`)

// discountFromRSSDescription 从 RSS description 提取优惠等级与截止时间。
// 返回 ("", nil) 表示无证据（非 PeerGo 站常规形态——不干预检测链）。
func discountFromRSSDescription(desc string) (string, *time.Time) {
	if !strings.Contains(desc, "优惠") {
		return "", nil
	}
	m := rssDiscountLineRe.FindStringSubmatch(desc)
	if len(m) < 2 {
		return "", nil
	}
	label := strings.TrimSpace(m[1])
	var level string
	switch label {
	case "免费":
		level = string(model.DiscountFree)
	case "2× 上传 / 免费", "2x 上传 / 免费":
		level = string(model.Discount2xFree)
	case "2× 上传", "2x 上传", "50% 下载", "30% 下载",
		"2× 上传 / 50% 下载", "2x 上传 / 50% 下载":
		// 已知不免费形态（promotionLabel 精确集）——显式 NONE
		// 短路（feed 是站方权威数据，无需再拉详情页确认）
		level = string(model.DiscountNone)
	default:
		// §59.456 附: 未知形态不干预——NP 站 description 含自然语言
		// "优惠：xxx"（非 PeerGo 形态）时走原检测链，防误短路漏检
		// 真实免费（回归审核收紧：首版 default→NONE 是反向 fail-open）
		return "", nil
	}
	var freeEnd *time.Time
	if len(m) >= 3 && m[2] != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04", m[2], time.Local); err == nil {
			freeEnd = &t
		}
	}
	return level, freeEnd
}
