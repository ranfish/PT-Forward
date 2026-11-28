package seeding

import (
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.592（审计 19 刀2）: 免费重检缓存复合键锚——V-01 防回归。
// 设计权威 docs/31:10067 决策 #401(4)「键为 (siteName, torrentID)」。
//
// 阴性对照（刀3）：
// A=discountCacheKey 回退裸 torrentID → 端到端锚 FAIL（串数据复现）
// B=读点退回裸 key 索引 → 端到端锚 FAIL（复合键写入读不到·缓存恒 miss）
// C=注入裸 e.discountCache[ 索引形态 → 静态锚点名 FAIL

// 单元锚①：键形态——跨站同号不同键·同站同号同键。
func TestDiscountCacheKey_Form(t *testing.T) {
	if discountCacheKey("siteA", "12345") == discountCacheKey("siteB", "12345") {
		t.Fatal("跨站同号键必须不同（V-01 根修核心语义）")
	}
	if discountCacheKey("siteA", "12345") != discountCacheKey("siteA", "12345") {
		t.Fatal("同站同号键必须相同")
	}
	if got := discountCacheKey("siteA", "12345"); got != "siteA:12345" {
		t.Fatalf("键形态=siteName:torrentID（rss 正例同构）, got %q", got)
	}
}

// 端到端锚②：两站同号记录依次写入/读取（直调生产方法）——站 B 不得
// 命中站 A 条目（离线复现 19 篇双方向串数据的反向验证）。
func TestDiscountCache_CrossSiteIsolation(t *testing.T) {
	e := &Engine{discountCache: make(map[string]*discountCacheEntry)}

	// 站 A 写 FREE（同号 12345）
	e.setDiscountCache("siteA", "12345", &model.DiscountResult{Level: model.DiscountFree})

	// 站 B 同号读——复合键下必须 miss（不再命中站 A 的 FREE）
	if cached, ok := e.getDiscountFromCache("siteB", "12345", 600); ok {
		t.Fatalf("V-01 回归：站 B 命中站 A 缓存（Level=%v）——复合键隔离失效",
			cached.Level)
	}

	// 站 B 自己写 NONE 后可读自己的
	e.setDiscountCache("siteB", "12345", &model.DiscountResult{Level: model.DiscountNone})
	got, ok := e.getDiscountFromCache("siteB", "12345", 600)
	if !ok || got == nil || got.Level.IsFree() {
		t.Fatalf("站 B 自读失败（ok=%v）——复合键下本站读写应正常", ok)
	}

	// 站 A 仍读得到自己的 FREE（互不污染）
	gotA, okA := e.getDiscountFromCache("siteA", "12345", 600)
	if !okA || !gotA.Level.IsFree() {
		t.Fatalf("站 A 自读被站 B 污染（复合键下应独立）")
	}
}

// 端到端锚③：TTL 过期语义不回归（命中刷新边界——18 篇已核健康面）。
func TestDiscountCache_TTLBoundary(t *testing.T) {
	e := &Engine{discountCache: make(map[string]*discountCacheEntry)}
	e.setDiscountCache("s", "1", &model.DiscountResult{Level: model.DiscountFree})
	if _, ok := e.getDiscountFromCache("s", "1", 600); !ok {
		t.Fatal("TTL 内应命中")
	}
	// 人工过期（CheckedAt 拨回）
	e.discountCacheMu.Lock()
	for _, v := range e.discountCache {
		v.CheckedAt = time.Now().Add(-2 * time.Hour)
	}
	e.discountCacheMu.Unlock()
	if _, ok := e.getDiscountFromCache("s", "1", 600); ok {
		t.Fatal("过期后不应命中")
	}
}
