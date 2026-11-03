package rss

import (
	"encoding/xml"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

func TestParseItemsRoushDiscountMetadata(t *testing.T) {
	raw := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>PeerGo · 免费种子</title>
<item><title>Verity 2026 1080p</title><link>https://rousi.pro/torrents/10253</link>
<description>不存在的罪人 Verity (2026)&#xA;大小：8.29 GB&#xA;优惠：50% 下载（至 2026-11-05 03:28）&#xA;做种 11</description>
<guid>peergo:torrent:10253</guid><pubDate>Sat, 31 Oct 2026 10:31:47 +0000</pubDate><category>电影</category>
<enclosure url="https://rousi.pro/rss/TOKEN/torrents/10253/download" length="8904462411" type="application/x-bittorrent"/></item>
<item><title>Free Movie 2026</title><link>https://rousi.pro/torrents/10254</link>
<description>某片&#xA;优惠：免费（至 2026-11-05 03:28）</description>
<guid>peergo:torrent:10254</guid>
<enclosure url="https://rousi.pro/rss/TOKEN/torrents/10254/download" length="100" type="application/x-bittorrent"/></item>
</channel></rss>`
	var feed RSSFeed
	if err := xml.Unmarshal([]byte(raw), &feed); err != nil {
		t.Fatal(err)
	}
	f := &Fetcher{}
	site := &model.Site{Name: "肉丝", Domain: "rousi.pro", IDStrategy: "path_segment", HashStrategy: "guid", SizeStrategy: "enclosure"}
	sub := &model.RSSSubscription{Name: "t", SiteName: "肉丝"}
	events := f.ParseItems(&feed, sub, site)
	if len(events) != 2 {
		t.Fatalf("events = %d want 2", len(events))
	}
	for _, ev := range events {
		lv, _ := ev.Metadata["rss_discount_level"].(string)
		t.Logf("id=%s level=%q free_end=%v size=%d dl=%s", ev.TorrentID, lv, ev.Metadata["rss_free_end"], ev.Size, ev.DownloadURL[:40])
	}
	if lv, _ := events[0].Metadata["rss_discount_level"].(string); lv != string(model.DiscountNone) {
		t.Errorf("item0 50%%下载应 NONE got %q", lv)
	}
	if lv, _ := events[1].Metadata["rss_discount_level"].(string); lv != string(model.DiscountFree) {
		t.Errorf("item1 免费应 FREE got %q", lv)
	}
}
