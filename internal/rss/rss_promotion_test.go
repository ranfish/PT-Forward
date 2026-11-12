package rss

import (
	"encoding/xml"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
)

// parseFeedForTest XML 解析（测试用——直接 Unmarshal 与生产同款）。
func parseFeedForTest(data []byte) (*RSSFeed, error) {
	var feed RSSFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, err
	}
	return &feed, nil
}

// §59.515 刀3 D1: 好大增强 RSS promotion_until+torznab:attr 解析锚。
// 用好大真实 RSS 片段（消毒·2026-11-12 实测）。
const hdareaRSSXML = `<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0" xmlns:torznab="http://torznab.com/schemas/2015/feed" xmlns:nexusphp="https://nexusphp.org/ns">
<channel>
<title>HDArea Torrents</title>
<link>https://hdarea.club</link>
<item>
<title>[免费]Alaska State Troopers S10E02 1080p HULU WEB-DL AAC 2.0 H.264-RAWR</title>
<link>https://hdarea.club/details.php?id=460596</link>
<description><![CDATA[test]]></description>
<guid isPermaLink="false">27ef67aa6ccd1d9a73ea1c77e63d9d257761c97b</guid>
<pubDate>Thu, 12 Nov 2026 08:15:02 +0800</pubDate>
<enclosure url="https://hdarea.club/download.php?id=460596&amp;apikey=SANITIZED" length="2340828138" type="application/x-bittorrent"/>
<torznab:attr name="size" value="2340828138"/>
<torznab:attr name="seeders" value="42"/>
<torznab:attr name="leechers" value="5"/>
<torznab:attr name="infohash" value="27ef67aa6ccd1d9a73ea1c77e63d9d257761c97b"/>
<torznab:attr name="downloadvolumefactor" value="0"/>
<torznab:attr name="uploadvolumefactor" value="1"/>
<nexusphp:sp_state>free</nexusphp:sp_state>
<nexusphp:promotion_until>2026-11-12 18:00:00</nexusphp:promotion_until>
</item>
<item>
<title>Non-Free Torrent Without Extensions</title>
<link>https://hdarea.club/details.php?id=12345</link>
<guid isPermaLink="false">aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa</guid>
<enclosure url="https://hdarea.club/download.php?id=12345" length="12345" type="application/x-bittorrent"/>
</item>
</channel>
</rss>`

func TestRSSPromotionUntil(t *testing.T) {
	
	feed, err := parseFeedForTest([]byte(hdareaRSSXML))
	if err != nil {
		t.Fatalf("parseFeed: %v", err)
	}
	if len(feed.Channel.Items) != 2 {
		t.Fatalf("items=%d want 2", len(feed.Channel.Items))
	}

	// ① 好大条目：promotion_until → FreeEndAt + attr → IsFree
	item1 := feed.Channel.Items[0]
	if item1.PromotionUntil == "" {
		t.Fatal("PromotionUntil 应非空（nexusphp:promotion_until 解析）")
	}
	if len(item1.TorznabAttrs) == 0 {
		t.Fatal("TorznabAttrs 应非空（torznab:attr 解析）")
	}
	// 验证 attr 内容
	foundDvf := false
	for _, attr := range item1.TorznabAttrs {
		if attr.Name == "downloadvolumefactor" && attr.Value == "0" {
			foundDvf = true
		}
	}
	if !foundDvf {
		t.Error("应解析到 downloadvolumefactor=0")
	}

	// ② 非好大条目：无扩展节点→零值
	item2 := feed.Channel.Items[1]
	if item2.PromotionUntil != "" {
		t.Errorf("非好大条目 PromotionUntil 应空: %q", item2.PromotionUntil)
	}
	if len(item2.TorznabAttrs) != 0 {
		t.Errorf("非好大条目 TorznabAttrs 应空: %v", item2.TorznabAttrs)
	}
}

func TestRSSPromotionUntilParseItems(t *testing.T) {
	
	feed, err := parseFeedForTest([]byte(hdareaRSSXML))
	if err != nil {
		t.Fatalf("parseFeed: %v", err)
	}

	// 构造最小 sub/site 消费 ParseItems
	sub := &model.RSSSubscription{SiteName: "好大"}
	site := &model.Site{Domain: "hdarea.club", Name: "好大"}
	events := (&Fetcher{}).ParseItems(feed, sub, site)
	if len(events) != 2 {
		t.Fatalf("events=%d want 2", len(events))
	}

	// ① 好大条目：FreeEndAt + IsFree 从扩展节点
	ev1 := events[0]
	if ev1.FreeEndAt == nil {
		t.Fatal("FreeEndAt 应非空（promotion_until 消费）")
	}
	expected := time.Date(2026, 11, 12, 18, 0, 0, 0, time.Local)
	if !ev1.FreeEndAt.Equal(expected) {
		t.Errorf("FreeEndAt=%v want %v", ev1.FreeEndAt, expected)
	}
	if !ev1.IsFree {
		t.Error("IsFree 应 true（downloadvolumefactor=0）")
	}

	// ② 非好大条目：FreeEndAt nil + IsFree false
	ev2 := events[1]
	if ev2.FreeEndAt != nil {
		t.Errorf("非好大条目 FreeEndAt 应 nil: %v", ev2.FreeEndAt)
	}
	if ev2.IsFree {
		t.Error("非好大条目 IsFree 应 false")
	}
}
