package adapter

import (
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.369 附二: 深度站禁转采集链——标签区 torrent-content-tag 形态
// （tid=5073 实录：rowhead"标签"→rowfollow 内 spans，官方/限转/原生）。
func TestExtractTagsTorrentContentTag(t *testing.T) {
	html := `<td class="rowhead nowrap">标签</td><td class="rowfollow">
	<span class="torrent-content-tag" style="color:#fff">官方</span>
	<span class="torrent-content-tag" style="color:#800080">限转</span>
	<span class="torrent-content-tag" style="color:#f90">原生</span></td>`
	tags := extractTags(html)
	found := false
	for _, tag := range tags {
		if tag == "限转" {
			found = true
		}
	}
	if !found {
		t.Fatalf("torrent-content-tag 形态应提取到'限转'，got %v", tags)
	}

	// 标准 NP tag class 回归（既有形态不受影响）
	tags2 := extractTags(`<span class="tag">禁转</span><span class="tag">首发</span>`)
	if len(tags2) != 2 || tags2[0] != "禁转" {
		t.Fatalf("标准 tag class 回归失败，got %v", tags2)
	}
}

// §59.369 附二: 发布时间绝对戳——NP 新版 details-inline-time title 属性
// （tid=5073 实录："发布于<span title='2026-10-29 19:35:30'>4时6分钟前</span>"）。
func TestFillPublishedAt(t *testing.T) {
	a := &NexusPHPAdapter{}
	html := `<span class="details-inline-time">发布于<span title="2026-10-29 19:35:30">4时6分钟前</span></span>`

	d := &model.TorrentDetail{}
	a.fillPublishedAt(html, d)
	if d.PublishedAt == nil {
		t.Fatal("发布时间 title 应提取成功")
	}
	if d.PublishedAt.Format("2006-01-02 15:04") != "2026-10-29 19:35" {
		t.Fatalf("提取异常: %v", d.PublishedAt)
	}

	// 已有值不覆盖
	earlier := d.PublishedAt.Add(-time.Hour)
	d2 := &model.TorrentDetail{PublishedAt: &earlier}
	a.fillPublishedAt(html, d2)
	if !d2.PublishedAt.Equal(earlier) {
		t.Fatal("已有值不应覆盖")
	}

	// 无形态零动作 + nil 防御
	d3 := &model.TorrentDetail{}
	a.fillPublishedAt(`<div>no time</div>`, d3)
	if d3.PublishedAt != nil {
		t.Fatal("无形态不应提取")
	}
	a.fillPublishedAt(html, nil) // 不 panic 即可

	// 老站相对时间形态（无 title）——不命中（keepfrds 走 ParseRelativeAge 既有通道）
	if rePublishedAtTitle.FindStringSubmatch(`由 xx 发布于20时46分前`) != nil {
		t.Fatal("无 title 的相对时间形态不应命中")
	}
}
