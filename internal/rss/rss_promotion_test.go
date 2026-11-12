package rss

import (
	"encoding/xml"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.515 刀3 D1: 好大增强 RSS 免费截止解析锚。
// 夹具=2026-11-12 实拉 spstate=9 真实 feed 消毒形态（§59.1339 教训：
// 验证样例必须真实数据源——首版夹具用官方文档样例 promotion_until
// 而生产实态是 free_until/freeuntil attr·D1 核心价值落空·此锚锁定实态）。
const hdareaRSSXML = `<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0" xmlns:torznab="http://torznab.com/schemas/2015/feed" xmlns:nexusphp="https://nexusphp.org/ns">
<channel>
<title>HDArea Torrents</title>
<link>https://hdarea.club</link>
<item>
<title>[免费]Wan Qiu Ji Qing 2026 S01 E09-E10 2160p WEB-DL H265 AAC-PTerWEB</title>
<link>https://hdarea.club/details.php?id=460812</link>
<description><![CDATA[test]]></description>
<guid isPermaLink="false">528053f7092d2ac826cea6ed221bee0edcd7112c</guid>
<pubDate>Thu, 12 Nov 2026 12:58:02 +0800</pubDate>
<enclosure url="https://hdarea.club/download.php?id=460812&amp;apikey=SANITIZED" length="557968249" type="application/x-bittorrent"/>
<torznab:attr name="size" value="557968249"/>
<torznab:attr name="seeders" value="2"/>
<torznab:attr name="leechers" value="6"/>
<torznab:attr name="peers" value="8"/>
<torznab:attr name="grabs" value="0"/>
<torznab:attr name="downloadvolumefactor" value="0"/>
<torznab:attr name="uploadvolumefactor" value="1"/>
<torznab:attr name="infohash" value="528053f7092d2ac826cea6ed221bee0edcd7112c"/>
<torznab:attr name="freeuntil" value="2026-11-13 12:58:02"/>
<nexusphp:sp_state>2</nexusphp:sp_state>
<nexusphp:sp_label>免费</nexusphp:sp_label>
<nexusphp:downloadvolumefactor>0</nexusphp:downloadvolumefactor>
<nexusphp:uploadvolumefactor>1</nexusphp:uploadvolumefactor>
<nexusphp:free_until>2026-11-13 12:58:02</nexusphp:free_until>
<nexusphp:seeders>2</nexusphp:seeders>
<nexusphp:leechers>6</nexusphp:leechers>
<nexusphp:snatched>0</nexusphp:snatched>
</item>
<item>
<title>Only Attr Freeuntil Channel</title>
<link>https://hdarea.club/details.php?id=460813</link>
<guid isPermaLink="false">aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa</guid>
<enclosure url="https://hdarea.club/download.php?id=460813" length="12345" type="application/x-bittorrent"/>
<torznab:attr name="downloadvolumefactor" value="0"/>
<torznab:attr name="freeuntil" value="2026-11-14 09:30:00"/>
</item>
<item>
<title>Only PromotionUntil Doc Form</title>
<link>https://hdarea.club/details.php?id=460814</link>
<guid isPermaLink="false">bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb</guid>
<enclosure url="https://hdarea.club/download.php?id=460814" length="12345" type="application/x-bittorrent"/>
<nexusphp:promotion_until>2026-11-15 08:00:00</nexusphp:promotion_until>
</item>
<item>
<title>Non-Free Torrent Without Extensions</title>
<link>https://hdarea.club/details.php?id=12345</link>
<guid isPermaLink="false">cccccccccccccccccccccccccccccccccccccccc</guid>
<enclosure url="https://hdarea.club/download.php?id=12345" length="12345" type="application/x-bittorrent"/>
</item>
</channel>
</rss>`

func parseFeedForTest(data []byte) (*RSSFeed, error) {
	var feed RSSFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, err
	}
	return &feed, nil
}

// TestRSSPromotionUntil 真实形态 XML 结构解析（free_until 主通道）。
func TestRSSPromotionUntil(t *testing.T) {
	feed, err := parseFeedForTest([]byte(hdareaRSSXML))
	if err != nil {
		t.Fatalf("parseFeed: %v", err)
	}
	if len(feed.Channel.Items) != 4 {
		t.Fatalf("items=%d want 4", len(feed.Channel.Items))
	}

	// ① 真实形态条目：free_until 元素+torznab:attr 全家
	item1 := feed.Channel.Items[0]
	if item1.FreeUntil != "2026-11-13 12:58:02" {
		t.Errorf("FreeUntil=%q（nexusphp:free_until 主通道应解析）", item1.FreeUntil)
	}
	if len(item1.TorznabAttrs) == 0 {
		t.Fatal("TorznabAttrs 应非空（torznab:attr 解析）")
	}

	// ④ 非好大条目：无扩展节点→零值
	item4 := feed.Channel.Items[3]
	if item4.FreeUntil != "" || item4.PromotionUntil != "" || len(item4.TorznabAttrs) != 0 {
		t.Errorf("非好大条目应零值: FreeUntil=%q PromotionUntil=%q attrs=%v",
			item4.FreeUntil, item4.PromotionUntil, item4.TorznabAttrs)
	}
}

// TestRSSPromotionUntilParseItems 消费链：三通道 FreeEndAt+attr IsFree。
func TestRSSPromotionUntilParseItems(t *testing.T) {
	feed, err := parseFeedForTest([]byte(hdareaRSSXML))
	if err != nil {
		t.Fatalf("parseFeed: %v", err)
	}

	sub := &model.RSSSubscription{SiteName: "好大"}
	site := &model.Site{Domain: "hdarea.club", Name: "好大"}
	events := (&Fetcher{}).ParseItems(feed, sub, site)
	if len(events) != 4 {
		t.Fatalf("events=%d want 4", len(events))
	}

	// ① 真实形态：FreeUntil 主通道 + IsFree
	ev1 := events[0]
	if ev1.FreeEndAt == nil {
		t.Fatal("FreeEndAt 应非空（free_until 主通道消费）")
	}
	expected := time.Date(2026, 11, 13, 12, 58, 2, 0, time.Local)
	if !ev1.FreeEndAt.Equal(expected) {
		t.Errorf("FreeEndAt=%v want %v", ev1.FreeEndAt, expected)
	}
	if !ev1.IsFree {
		t.Error("IsFree 应 true（downloadvolumefactor=0）")
	}

	// ② 纯 attr freeuntil 通道（元素缺失时 attr 兜底）
	ev2 := events[1]
	if ev2.FreeEndAt == nil {
		t.Fatal("FreeEndAt 应非空（attr freeuntil 通道）")
	}
	exp2 := time.Date(2026, 11, 14, 9, 30, 0, 0, time.Local)
	if !ev2.FreeEndAt.Equal(exp2) {
		t.Errorf("attr 通道 FreeEndAt=%v want %v", ev2.FreeEndAt, exp2)
	}

	// ③ 纯 promotion_until 文档形态（兜底通道）
	ev3 := events[2]
	if ev3.FreeEndAt == nil {
		t.Fatal("FreeEndAt 应非空（promotion_until 文档形态兜底）")
	}
	exp3 := time.Date(2026, 11, 15, 8, 0, 0, 0, time.Local)
	if !ev3.FreeEndAt.Equal(exp3) {
		t.Errorf("兜底通道 FreeEndAt=%v want %v", ev3.FreeEndAt, exp3)
	}

	// ④ 非好大零影响
	ev4 := events[3]
	if ev4.FreeEndAt != nil || ev4.IsFree {
		t.Errorf("非好大条目应零值: FreeEndAt=%v IsFree=%v", ev4.FreeEndAt, ev4.IsFree)
	}
}
