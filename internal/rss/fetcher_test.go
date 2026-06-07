package rss

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

func newTestLogger() *zap.Logger {
	return zap.NewNop()
}

const sampleRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Site RSS</title>
    <link>https://example.com</link>
    <item>
      <title>Movie.2024.1080p.BluRay.x264</title>
      <link>https://example.com/download.php?id=42&amp;passkey=abc</link>
      <guid>abc123def456abc123def456abc123de</guid>
      <enclosure url="https://example.com/download.php?id=42&amp;passkey=abc" length="2147483648" type="application/x-bittorrent"/>
    </item>
    <item>
      <title>Show.S02E05.720p.WEB-DL.x265</title>
      <link>https://example.com/download.php?id=99&amp;passkey=abc</link>
      <guid>f47ac10b58cc4372a5670e02b2c3d479</guid>
      <enclosure url="https://example.com/download.php?id=99&amp;passkey=abc" length="1073741824" type="application/x-bittorrent"/>
    </item>
  </channel>
</rss>`

func TestFetch_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("User-Agent header should be set")
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(sampleRSS))
	}))
	defer srv.Close()

	f := NewFetcher(newTestLogger())
	feed, err := f.Fetch(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(feed.Channel.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(feed.Channel.Items))
	}
	if feed.Channel.Items[0].Title != "Movie.2024.1080p.BluRay.x264" {
		t.Errorf("unexpected title: %s", feed.Channel.Items[0].Title)
	}
}

func TestFetch_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	f := NewFetcher(newTestLogger())
	_, err := f.Fetch(t.Context(), srv.URL)
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestFetch_InvalidXML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not xml at all"))
	}))
	defer srv.Close()

	f := NewFetcher(newTestLogger())
	_, err := f.Fetch(t.Context(), srv.URL)
	if err == nil {
		t.Fatal("expected error for invalid XML")
	}
}

func TestFetchWithProxy_DefaultsToFallbackClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(sampleRSS))
	}))
	defer srv.Close()

	f := NewFetcher(newTestLogger())
	feed, err := f.FetchWithProxy(t.Context(), srv.URL, "", false)
	if err != nil {
		t.Fatalf("FetchWithProxy with no proxy: %v", err)
	}
	if len(feed.Channel.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(feed.Channel.Items))
	}
}

func TestFetchWithProxy_CacheReusesClient(t *testing.T) {
	f := NewFetcher(newTestLogger())
	c1 := f.clientFor("http://example.proxy:9999", false)
	c2 := f.clientFor("http://example.proxy:9999", false)
	if c1 != c2 {
		t.Fatal("clientFor should return cached client for same key")
	}
	c3 := f.clientFor("", true)
	if c3 == c1 {
		t.Fatal("clientFor should return different client for different config")
	}
	c4 := f.clientFor("", false)
	if c4 != f.client {
		t.Fatal("clientFor with empty proxy and skipSSL=false should return default client")
	}
}

func TestParseItems_QueryParamID(t *testing.T) {
	var feed RSSFeed
	if err := xml.Unmarshal([]byte(sampleRSS), &feed); err != nil {
		t.Fatal(err)
	}

	site := &model.Site{
		IDStrategy: "query_param",
		IDPattern:  "id",
	}
	sub := &model.RSSSubscription{
		Name:     "test-sub",
		SiteName: "example",
	}

	items := (&Fetcher{}).ParseItems(&feed, sub, site)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].TorrentID != "42" {
		t.Errorf("expected torrent_id 42, got %s", items[0].TorrentID)
	}
	if items[1].TorrentID != "99" {
		t.Errorf("expected torrent_id 99, got %s", items[1].TorrentID)
	}
}

func TestParseItems_GuidTextID(t *testing.T) {
	rss := `<?xml version="1.0"?>
<rss version="2.0"><channel><title>T</title><link>https://x.com</link>
<item>
  <title>Test Torrent</title>
  <link>https://x.com/torrent/123</link>
  <guid>123</guid>
  <enclosure url="https://x.com/dl/123" length="500000000" type="application/x-bittorrent"/>
</item>
</channel></rss>`

	var feed RSSFeed
	if err := xml.Unmarshal([]byte(rss), &feed); err != nil {
		t.Fatal(err)
	}

	site := &model.Site{IDStrategy: "guid_text"}
	sub := &model.RSSSubscription{Name: "s", SiteName: "x"}

	items := (&Fetcher{}).ParseItems(&feed, sub, site)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].TorrentID != "123" {
		t.Errorf("expected torrent_id 123, got %s", items[0].TorrentID)
	}
}

func TestParseItems_SkipEmpty(t *testing.T) {
	rss := `<?xml version="1.0"?>
<rss version="2.0"><channel><title>T</title><link>https://x.com</link>
<item><title>No Link Item</title></item>
</channel></rss>`

	var feed RSSFeed
	if err := xml.Unmarshal([]byte(rss), &feed); err != nil {
		t.Fatal(err)
	}

	site := &model.Site{IDStrategy: "query_param", IDPattern: "id"}
	sub := &model.RSSSubscription{Name: "s", SiteName: "x"}

	items := (&Fetcher{}).ParseItems(&feed, sub, site)
	if len(items) != 0 {
		t.Fatalf("expected 0 items for empty link/guid, got %d", len(items))
	}
}

func TestExtractQueryParam(t *testing.T) {
	tests := []struct {
		url    string
		param  string
		expect string
	}{
		{"https://x.com/dl?id=42&key=abc", "id", "42"},
		{"https://x.com/dl?id=99", "id", "99"},
		{"https://x.com/dl?id=", "id", ""},
		{"https://x.com/dl", "id", ""},
		{"https://x.com/dl?tid=55#section", "tid", "55"},
	}

	for _, tt := range tests {
		got := extractQueryParam(tt.url, tt.param)
		if got != tt.expect {
			t.Errorf("extractQueryParam(%q, %q) = %q, want %q", tt.url, tt.param, got, tt.expect)
		}
	}
}

func TestExtractPathSegment(t *testing.T) {
	tests := []struct {
		url    string
		expect string
	}{
		{"https://x.com/torrent/777", "777"},
		{"https://x.com/torrent/888/", "888"},
		{"https://x.com/", "x.com"},
	}

	for _, tt := range tests {
		got := extractPathSegment(tt.url, "")
		if got != tt.expect {
			t.Errorf("extractPathSegment(%q) = %q, want %q", tt.url, got, tt.expect)
		}
	}
}

func TestExtractByRegex(t *testing.T) {
	got := extractByRegex("https://x.com/t/12345/details", `/t/(\d+)/`)
	if got != "12345" {
		t.Errorf("got %q, want 12345", got)
	}

	got2 := extractByRegex("no match here", `/t/(\d+)/`)
	if got2 != "" {
		t.Errorf("expected empty for no match, got %q", got2)
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		input  string
		expect int64
	}{
		{"2147483648", 2147483648},
		{"0", 0},
		{"", 0},
		{"abc", 0},
	}

	for _, tt := range tests {
		got := parseSize(tt.input)
		if got != tt.expect {
			t.Errorf("parseSize(%q) = %d, want %d", tt.input, got, tt.expect)
		}
	}
}

func TestExtractInfoHash_Guid(t *testing.T) {
	item := RSSItem{GUID: "ABC123DEF456ABC123DEF456ABC123DEF456AB12"}
	site := &model.Site{HashStrategy: "guid"}

	h := extractInfoHash(item, site, "")
	if h != "abc123def456abc123def456abc123def456ab12" {
		t.Errorf("expected lowercase hash, got %s", h)
	}
}

func TestExtractInfoHash_XMLTag(t *testing.T) {
	item := RSSItem{CustomHash: "DEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEF"}
	site := &model.Site{HashStrategy: "xml_tag"}

	h := extractInfoHash(item, site, "")
	if h != "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef" {
		t.Errorf("expected lowercase hash, got %s", h)
	}
}

func TestExtractSize_Enclosure(t *testing.T) {
	item := RSSItem{Enclosure: RSSEnclosure{Length: "1073741824"}}
	site := &model.Site{SizeStrategy: "enclosure"}

	s := extractSize(item, site)
	if s != 1073741824 {
		t.Errorf("expected 1073741824, got %d", s)
	}
}

func TestExtractSize_XMLTag(t *testing.T) {
	item := RSSItem{CustomSize: "536870912"}
	site := &model.Site{SizeStrategy: "xml_tag"}

	s := extractSize(item, site)
	if s != 536870912 {
		t.Errorf("expected 536870912, got %d", s)
	}
}
