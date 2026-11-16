package api

// §59.539 锚：禁转标记复检判定四态+子集替换+滚动续期+单向互斥。
//
// 四态语义（标记优先·忠实跟随站点标记）：
//   cleared=站方标记消失→直接放行（不看 until）
//   kept=标记还在无变化
//   renewed=限时禁转在+until 缺失/已过→滚动续期 now+24h
//   changed=标记变化（限时→永久）跟随站方
//   failed=数据不足/网络失败→原样保持
// 子集替换：分集（系统级禁发）不进复检名单、回写不触碰。

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fakeRecheckAdapter struct {
	model.SiteAdapter // 嵌入接口：未实现方法零调用
	detail            *model.TorrentDetail
	err               error
}

func (f *fakeRecheckAdapter) GetTorrentDetail(_ context.Context, _ *model.SiteConfig, _ string) (*model.TorrentDetail, error) {
	return f.detail, f.err
}

type fakeRecheckProvider struct {
	config  *model.SiteConfig
	adapter model.SiteAdapter
}

func (p *fakeRecheckProvider) GetAdapter(_ context.Context, _ string) (model.SiteAdapter, error) {
	return p.adapter, nil
}
func (p *fakeRecheckProvider) GetSiteConfig(_ context.Context, _ string) (*model.SiteConfig, error) {
	return p.config, nil
}

func newRecheckTestHandler(t *testing.T) *PublishTorrentsHandler {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.TorrentMetadata{}, &model.PublishSetting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
}

func seedRecheckRow(t *testing.T, h *PublishTorrentsHandler, siteName string, flags []string, until *time.Time) {
	t.Helper()
	fj, _ := json.Marshal(flags)
	row := model.TorrentMetadata{
		InfoHash: "ntrhash00000000000000000000000000000000", SiteName: siteName,
		TorrentID: "1001", Flags: string(fj), NoTransferUntil: until,
	}
	if err := h.db.Create(&row).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func recheckFlagsOf(t *testing.T, h *PublishTorrentsHandler) ([]string, *time.Time) {
	t.Helper()
	var row model.TorrentMetadata
	if err := h.db.Where("info_hash = ?", "ntrhash00000000000000000000000000000000").First(&row).Error; err != nil {
		t.Fatalf("load: %v", err)
	}
	var flags []string
	_ = json.Unmarshal([]byte(row.Flags), &flags)
	return flags, row.NoTransferUntil
}

// 场景 1：站方标记消失→cleared+子集替换保分集。
func TestNoTransferRecheckCleared(t *testing.T) {
	h := newRecheckTestHandler(t)
	seedRecheckRow(t, h, "憨憨", []string{"禁转", "分集"}, nil)
	h.siteProvider = &fakeRecheckProvider{
		config:  &model.SiteConfig{Domain: "hhanclub"},
		adapter: &fakeRecheckAdapter{detail: &model.TorrentDetail{Title: "干净标题"}},
	}
	if got := h.recheckOneNoTransferRow(context.Background(), &model.TorrentMetadata{InfoHash: "ntrhash00000000000000000000000000000000", SiteName: "憨憨", TorrentID: "1001", Flags: `["禁转","分集"]`}); got != "cleared" {
		t.Fatalf("want cleared, got %s", got)
	}
	flags, until := recheckFlagsOf(t, h)
	if len(flags) != 1 || flags[0] != "分集" {
		t.Fatalf("分集须保留, got %v", flags)
	}
	if until != nil {
		t.Fatalf("cleared 后 until 应清 nil")
	}
}

// 场景 2：标记还在→kept。
func TestNoTransferRecheckKept(t *testing.T) {
	h := newRecheckTestHandler(t)
	seedRecheckRow(t, h, "憨憨", []string{"禁转"}, nil)
	h.siteProvider = &fakeRecheckProvider{
		config:  &model.SiteConfig{Domain: "hhanclub"},
		adapter: &fakeRecheckAdapter{detail: &model.TorrentDetail{Title: "某片 2000 1080p", Description: "本资源禁转"}},
	}
	if got := h.recheckOneNoTransferRow(context.Background(), &model.TorrentMetadata{InfoHash: "ntrhash00000000000000000000000000000000", SiteName: "憨憨", TorrentID: "1001", Flags: `["禁转"]`}); got != "kept" {
		t.Fatalf("want kept, got %s", got)
	}
}

// 场景 3：keepfrds 限时禁转标记还在+until 已过（站方延长窗）→滚动续期 now+24h。
func TestNoTransferRecheckRenewed(t *testing.T) {
	h := newRecheckTestHandler(t)
	past := time.Now().Add(-48 * time.Hour)
	seedRecheckRow(t, h, "朋友", []string{"限时禁转"}, &past)
	h.siteProvider = &fakeRecheckProvider{
		config: &model.SiteConfig{Domain: "pt.keepfrds.com"},
		adapter: &fakeRecheckAdapter{detail: &model.TorrentDetail{
			RawListRow: "<td>某片 2000 1080p [ 限时禁转 ]</td>", AddedAtText: "3天前",
		}},
	}
	if got := h.recheckOneNoTransferRow(context.Background(), &model.TorrentMetadata{InfoHash: "ntrhash00000000000000000000000000000000", SiteName: "朋友", TorrentID: "1001", Flags: `["限时禁转"]`}); got != "renewed" {
		t.Fatalf("want renewed, got %s", got)
	}
	_, until := recheckFlagsOf(t, h)
	if until == nil || until.Before(time.Now().Add(23*time.Hour)) {
		t.Fatalf("滚动续期应≈now+24h, got %v", until)
	}
}

// 场景 4：keepfrds 限时→站方升级永久→changed 跟随。
func TestNoTransferRecheckChanged(t *testing.T) {
	h := newRecheckTestHandler(t)
	past := time.Now().Add(-48 * time.Hour)
	seedRecheckRow(t, h, "朋友", []string{"限时禁转"}, &past)
	h.siteProvider = &fakeRecheckProvider{
		config: &model.SiteConfig{Domain: "pt.keepfrds.com"},
		adapter: &fakeRecheckAdapter{detail: &model.TorrentDetail{
			RawListRow: "<td>某片 2000 1080p [禁转]</td>", AddedAtText: "3天前",
		}},
	}
	if got := h.recheckOneNoTransferRow(context.Background(), &model.TorrentMetadata{InfoHash: "ntrhash00000000000000000000000000000000", SiteName: "朋友", TorrentID: "1001", Flags: `["限时禁转"]`}); got != "changed" {
		t.Fatalf("want changed, got %s", got)
	}
	flags, until := recheckFlagsOf(t, h)
	if len(flags) != 1 || flags[0] != "禁转" {
		t.Fatalf("应跟随为禁转, got %v", flags)
	}
	if until != nil {
		t.Fatalf("永久禁转 until 应 nil")
	}
}

// 场景 5：keepfrds 列表行数据不足（RawListRow 空）→failed 保持原拦。
func TestNoTransferRecheckRowMissing(t *testing.T) {
	h := newRecheckTestHandler(t)
	seedRecheckRow(t, h, "朋友", []string{"禁转"}, nil)
	h.siteProvider = &fakeRecheckProvider{
		config:  &model.SiteConfig{Domain: "pt.keepfrds.com"},
		adapter: &fakeRecheckAdapter{detail: &model.TorrentDetail{Title: "某片"}},
	}
	if got := h.recheckOneNoTransferRow(context.Background(), &model.TorrentMetadata{InfoHash: "ntrhash00000000000000000000000000000000", SiteName: "朋友", TorrentID: "1001", Flags: `["禁转"]`}); got != "failed" {
		t.Fatalf("want failed, got %s", got)
	}
	flags, _ := recheckFlagsOf(t, h)
	if len(flags) != 1 || flags[0] != "禁转" {
		t.Fatalf("失败应保持原 flags, got %v", flags)
	}
}

// 场景 6：keepfrds 标记消失（限时窗到期站方摘标记）→cleared 直接放行不看 until。
func TestNoTransferRecheckClearedIgnoreUntil(t *testing.T) {
	h := newRecheckTestHandler(t)
	future := time.Now().Add(20 * time.Hour) // 闹钟还没响
	seedRecheckRow(t, h, "朋友", []string{"限时禁转"}, &future)
	h.siteProvider = &fakeRecheckProvider{
		config: &model.SiteConfig{Domain: "pt.keepfrds.com"},
		adapter: &fakeRecheckAdapter{detail: &model.TorrentDetail{
			RawListRow: "<td>某片 2000 1080p</td>", AddedAtText: "20时30分前",
		}},
	}
	if got := h.recheckOneNoTransferRow(context.Background(), &model.TorrentMetadata{InfoHash: "ntrhash00000000000000000000000000000000", SiteName: "朋友", TorrentID: "1001", Flags: `["限时禁转"]`}); got != "cleared" {
		t.Fatalf("标记消失应直接放行（不看 until）, got %s", got)
	}
	_, until := recheckFlagsOf(t, h)
	if until != nil {
		t.Fatalf("cleared 后 until 应 nil")
	}
}

// 单向互斥：深度→{时间,肉丝,家园} 单侧·反方向与无关站不拦。
func TestHardcodedOneWayExclusions(t *testing.T) {
	cases := []struct {
		src, tgt string
		want     bool
	}{
		{"深度", "时间", true},
		{"深度", "肉丝", true},
		{"深度", "家园", true},
		{"时间", "深度", false}, // 单向：反方向不拦
		{"深度", "优堡", false},
		{"朋友", "时间", false},
	}
	for _, c := range cases {
		if got := publish.IsHardcodedOneWayExclusion(c.src, c.tgt); got != c.want {
			t.Fatalf("%s→%s want %v got %v", c.src, c.tgt, c.want, got)
		}
	}
}

// isNoTransferWord：七词在内·分集/adult 不在（提升一档不复检）。
func TestIsNoTransferWord(t *testing.T) {
	for _, w := range noTransferForbiddenWords {
		if !isNoTransferWord(w) {
			t.Fatalf("%s 应属禁转族", w)
		}
	}
	for _, w := range []string{"分集", "adult"} {
		if isNoTransferWord(w) {
			t.Fatalf("%s 不应属禁转族（系统级禁发）", w)
		}
	}
	// §59.539 P7: 禁止转载=禁转族第八词（子串兜不住·显式列名）
	if !isNoTransferWord("禁止转载") {
		t.Fatal("禁止转载应属禁转族（P7 补词）")
	}
}
