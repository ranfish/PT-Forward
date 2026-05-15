package rss

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type Fetcher struct {
	client *http.Client
	logger *zap.Logger
}

func NewFetcher(logger *zap.Logger) *Fetcher {
	return &Fetcher{
		client: httpclient.NewSiteHTTPClient(httpclient.SiteHTTPConfig{}),
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
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, rssError(ErrRSSNetwork, "构造请求失败", err)
	}
	req.Header.Set("User-Agent", "PT-Forward/1.0")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, rssError(ErrRSSNetwork, "请求失败", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
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
			Metadata:  map[string]any{},
		}

		event.TorrentID = extractTorrentID(item, site)
		event.DownloadURL = extractDownloadURL(item, site)
		event.InfoHash = extractInfoHash(item, site, event.TorrentID)
		event.Size = extractSize(item, site)

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

	Enclosure RSSEnclosure `xml:"enclosure"`

	CustomHash string `xml:"hash"`
	CustomSize string `xml:"size"`
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
