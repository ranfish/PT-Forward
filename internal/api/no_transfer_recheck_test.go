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
	"fmt"
	"strings"
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
	calls             int
}

func (f *fakeRecheckAdapter) GetTorrentDetail(_ context.Context, _ *model.SiteConfig, _ string) (*model.TorrentDetail, error) {
	f.calls++
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
	if err := db.AutoMigrate(&model.TorrentMetadata{}, &model.TorrentSnapshot{}, &model.PublishSetting{}, &model.Site{}); err != nil {
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
	// §59.548: 复检查询含活跃快照校验——seed 行顺路建快照（种子在）
	seedRecheckSnapshot(t, h, row.InfoHash)
}

// seedRecheckSnapshot §59.548: 复检对象的活跃快照（种子在下载器）。
func seedRecheckSnapshot(t *testing.T, h *PublishTorrentsHandler, hash string) {
	t.Helper()
	if err := h.db.Create(&model.TorrentSnapshot{Hash: hash, ClientUID: 1,
		SavePath: "/v", Name: "n", Progress: 1}).Error; err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}
}

// seedRecheckSite §59.542 追记十一: 复检链先经 sites 表解析站名→domain。
func seedRecheckSite(t *testing.T, h *PublishTorrentsHandler, name, domain string) {
	t.Helper()
	if err := h.db.Create(&model.Site{Name: name, Domain: domain, Enabled: true}).Error; err != nil {
		t.Fatalf("seed site: %v", err)
	}
}

// recheckRowFull 新两段组合（模拟旧单行接口·resolve+apply）。
func recheckRowFull(h *PublishTorrentsHandler, meta *model.TorrentMetadata) string {
	nf, nu, ok := h.resolveNoTransferState(context.Background(), meta.SiteName, meta.TorrentID)
	if !ok {
		return "failed"
	}
	return h.applyNoTransferResult(context.Background(), meta, nf, nu)
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
	seedRecheckSite(t, h, "憨憨", "hhanclub")
	h.siteProvider = &fakeRecheckProvider{
		config:  &model.SiteConfig{Domain: "hhanclub"},
		adapter: &fakeRecheckAdapter{detail: &model.TorrentDetail{Title: "干净标题"}},
	}
	if got := recheckRowFull(h, &model.TorrentMetadata{InfoHash: "ntrhash00000000000000000000000000000000", SiteName: "憨憨", TorrentID: "1001", Flags: `["禁转","分集"]`}); got != "cleared" {
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
	seedRecheckSite(t, h, "憨憨", "hhanclub")
	h.siteProvider = &fakeRecheckProvider{
		config:  &model.SiteConfig{Domain: "hhanclub"},
		adapter: &fakeRecheckAdapter{detail: &model.TorrentDetail{Title: "某片 2000 1080p", Description: "本资源禁转"}},
	}
	if got := recheckRowFull(h, &model.TorrentMetadata{InfoHash: "ntrhash00000000000000000000000000000000", SiteName: "憨憨", TorrentID: "1001", Flags: `["禁转"]`}); got != "kept" {
		t.Fatalf("want kept, got %s", got)
	}
}

// 场景 3：keepfrds 限时禁转标记还在+until 已过（站方延长窗）→滚动续期 now+24h。
func TestNoTransferRecheckRenewed(t *testing.T) {
	h := newRecheckTestHandler(t)
	past := time.Now().Add(-48 * time.Hour)
	seedRecheckRow(t, h, "朋友", []string{"限时禁转"}, &past)
	seedRecheckSite(t, h, "朋友", "pt.keepfrds.com")
	h.siteProvider = &fakeRecheckProvider{
		config: &model.SiteConfig{Domain: "pt.keepfrds.com"},
		adapter: &fakeRecheckAdapter{detail: &model.TorrentDetail{
			RawListRow: "<td>某片 2000 1080p [ 限时禁转 ]</td>", AddedAtText: "3天前",
		}},
	}
	if got := recheckRowFull(h, &model.TorrentMetadata{InfoHash: "ntrhash00000000000000000000000000000000", SiteName: "朋友", TorrentID: "1001", Flags: `["限时禁转"]`}); got != "renewed" {
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
	seedRecheckSite(t, h, "朋友", "pt.keepfrds.com")
	h.siteProvider = &fakeRecheckProvider{
		config: &model.SiteConfig{Domain: "pt.keepfrds.com"},
		adapter: &fakeRecheckAdapter{detail: &model.TorrentDetail{
			RawListRow: "<td>某片 2000 1080p [禁转]</td>", AddedAtText: "3天前",
		}},
	}
	if got := recheckRowFull(h, &model.TorrentMetadata{InfoHash: "ntrhash00000000000000000000000000000000", SiteName: "朋友", TorrentID: "1001", Flags: `["限时禁转"]`}); got != "changed" {
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
	seedRecheckSite(t, h, "朋友", "pt.keepfrds.com")
	h.siteProvider = &fakeRecheckProvider{
		config:  &model.SiteConfig{Domain: "pt.keepfrds.com"},
		adapter: &fakeRecheckAdapter{detail: &model.TorrentDetail{Title: "某片"}},
	}
	if got := recheckRowFull(h, &model.TorrentMetadata{InfoHash: "ntrhash00000000000000000000000000000000", SiteName: "朋友", TorrentID: "1001", Flags: `["禁转"]`}); got != "failed" {
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
	seedRecheckSite(t, h, "朋友", "pt.keepfrds.com")
	h.siteProvider = &fakeRecheckProvider{
		config: &model.SiteConfig{Domain: "pt.keepfrds.com"},
		adapter: &fakeRecheckAdapter{detail: &model.TorrentDetail{
			RawListRow: "<td>某片 2000 1080p</td>", AddedAtText: "20时30分前",
		}},
	}
	if got := recheckRowFull(h, &model.TorrentMetadata{InfoHash: "ntrhash00000000000000000000000000000000", SiteName: "朋友", TorrentID: "1001", Flags: `["限时禁转"]`}); got != "cleared" {
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

// §59.542 追记十一锚: 簇脱钩——同 (site,tid) 一组一次判定（详情只拉一次）·
// 组内行统一应用·计数按组（16 副本行=检测 1 颗）。
func TestNoTransferRecheckClusterDedup(t *testing.T) {
	h := newRecheckTestHandler(t)
	seedRecheckSite(t, h, "朋友", "pt.keepfrds.com")
	for i := 0; i < 16; i++ {
		fj, _ := json.Marshal([]string{"禁转"})
		hash := fmt.Sprintf("dedup%02dhash00000000000000000000000000", i)
		row := model.TorrentMetadata{
			InfoHash: hash,
			SiteName: "朋友", TorrentID: "2782403", Flags: string(fj),
		}
		if err := h.db.Create(&row).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
		seedRecheckSnapshot(t, h, hash) // §59.548: 活跃快照（种子在）
	}
	adapter := &fakeRecheckAdapter{detail: &model.TorrentDetail{
		RawListRow: "<td>某片 2000 1080p [禁转]</td>", AddedAtText: "3天前",
	}}
	h.siteProvider = &fakeRecheckProvider{
		config:  &model.SiteConfig{Domain: "pt.keepfrds.com"},
		adapter: adapter,
	}
	h.RunNoTransferRecheck(context.Background())

	if adapter.calls != 1 {
		t.Errorf("详情请求次数=%d（同 tid 簇副本应去重为 1 次）", adapter.calls)
	}
	// §59.548: 文案括号闭合到结尾（整段明细被包）
	if !strings.Contains(h.noTransferRecheck.lastStats, "检测 1 颗（行 16：") ||
		!strings.HasSuffix(h.noTransferRecheck.lastStats, "）") {
		t.Errorf("lastStats=%q（计数应按组：1 颗·行 16·结尾闭合）", h.noTransferRecheck.lastStats)
	}
	var keptN int64
	h.db.Model(&model.TorrentMetadata{}).Where("flags LIKE ?", `%"禁转"%`).Count(&keptN)
	if keptN != 16 {
		t.Errorf("kept 行数=%d（组内 16 行应统一应用 kept）", keptN)
	}
}

// §59.548 锚: 孤儿行过滤——下载器删种（快照行消失）后 metadata 残留行
// 不再进入复检名单（「行 N」=实时实态·243 案：簇删部分种子后仍「行 16」）。
func TestNoTransferRecheckOrphanFiltered(t *testing.T) {
	h := newRecheckTestHandler(t)
	seedRecheckSite(t, h, "朋友", "pt.keepfrds.com")
	// 两行同 tid：一行活跃（有快照）·一行孤儿（快照已随删种消失）
	for _, hash := range []string{"orphAhash000000000000000000000000000", "orphBhash000000000000000000000000000"} {
		fj, _ := json.Marshal([]string{"禁转"})
		if err := h.db.Create(&model.TorrentMetadata{InfoHash: hash, SiteName: "朋友",
			TorrentID: "2782403", Flags: string(fj)}).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	seedRecheckSnapshot(t, h, "orphAhash000000000000000000000000000") // 仅 A 活跃
	adapter := &fakeRecheckAdapter{detail: &model.TorrentDetail{
		RawListRow: "<td>某片 2000 1080p [禁转]</td>", AddedAtText: "3天前",
	}}
	h.siteProvider = &fakeRecheckProvider{
		config:  &model.SiteConfig{Domain: "pt.keepfrds.com"},
		adapter: adapter,
	}
	h.RunNoTransferRecheck(context.Background())

	if !strings.Contains(h.noTransferRecheck.lastStats, "行 1：") {
		t.Errorf("lastStats=%q（孤儿行应被过滤·只计活跃 1 行）", h.noTransferRecheck.lastStats)
	}
	// 孤儿行的 flags 保持原样（历史资产·不被无效 apply 触碰）
	var orphanRow model.TorrentMetadata
	if err := h.db.Where("info_hash = ?", "orphBhash000000000000000000000000000").First(&orphanRow).Error; err != nil {
		t.Fatalf("load orphan: %v", err)
	}
	if !strings.Contains(orphanRow.Flags, "禁转") {
		t.Errorf("孤儿行 flags 应保持原样, got %q", orphanRow.Flags)
	}
}

// §59.542 追记十一锚: 站名→域名解析——provider 收到 domain（非站名）·
// sites 表无该站名时 failed（243 案形态：GetByDomain("朋友") 必 miss）。
func TestNoTransferRecheckNameToDomain(t *testing.T) {
	h := newRecheckTestHandler(t)
	seedRecheckRow(t, h, "憨憨", []string{"禁转"}, nil)
	seedRecheckSite(t, h, "憨憨", "hhanclub")
	var gotDomain string
	wrap := &fakeRecheckProvider{config: &model.SiteConfig{}, adapter: &fakeRecheckAdapter{}}
	h.siteProvider = &nameCaptureProvider{inner: wrap, got: &gotDomain}
	// resolve 阶段：sites 解析成功即收到 domain（后续 detail 空导致 failed
	// 也算解析过关——断言点在 gotDomain 收到的值）。
	_ = recheckRowFull(h, &model.TorrentMetadata{InfoHash: "ntrhash00000000000000000000000000000000", SiteName: "憨憨", TorrentID: "1001", Flags: `["禁转"]`})
	if gotDomain == "" {
		t.Fatal("provider 未收到任何 key——sites 解析未发生")
	}
	if gotDomain == "憨憨" {
		t.Errorf("provider 收到站名而非 domain: %q", gotDomain)
	}
	// 无站点行→failed
	h2 := newRecheckTestHandler(t)
	seedRecheckRow(t, h2, "幽灵站", []string{"禁转"}, nil)
	h2.siteProvider = &fakeRecheckProvider{config: &model.SiteConfig{}, adapter: &fakeRecheckAdapter{}}
	if got := recheckRowFull(h2, &model.TorrentMetadata{InfoHash: "ntrhash00000000000000000000000000000000", SiteName: "幽灵站", TorrentID: "1001", Flags: `["禁转"]`}); got != "failed" {
		t.Errorf("无站点行应 failed, got %s", got)
	}
}

type nameCaptureProvider struct {
	inner *fakeRecheckProvider
	got   *string
}

func (p *nameCaptureProvider) GetAdapter(ctx context.Context, key string) (model.SiteAdapter, error) {
	*p.got = key
	return p.inner.adapter, nil
}
func (p *nameCaptureProvider) GetSiteConfig(ctx context.Context, key string) (*model.SiteConfig, error) {
	*p.got = key
	return p.inner.config, nil
}
