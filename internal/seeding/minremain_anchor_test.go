package seeding

import (
	"context"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// §59.594（审计 20 刀2）: MinRemainMin 持久化保真三层锚——V-01 防回归。
//
// 锚① 模型字段（GORM schema 含 MinRemainMin——防字段被删）
// 锚② 端到端字段保真（A 未重启真走闸门/B 经 DB 恢复字段保真+保护存续
//      ——直调生产方法不 mock·专补 TestFreeWaitMonitor_RecoverOnStartup
//      只验条数不验字段的覆盖维度缺失；两态独立条目防 Remove 污染）
// 锚③ 静态三构造块（Add 内存 entry/Add 的 dbEntry/Recover 的 freeWaitEntry
//      都必须出现 MinRemainMin——沿 16/17/18/19 篇静态锚惯例）
//
// 阴性对照（刀 3·复审补强）：
// A=删 model 字段（全清） → 锚①+锚② FAIL
// B=只改 model+Add 不补 Recover 回填 → 锚② FAIL（恢复仍归零）
// C=删 dbEntry 写入行 → 锚③ 点名 FAIL
// D=删 CheckOnce 保护闸门 → 锚② A 态 FAIL（推送+移出断言双路径）
//    〔复审教训：原 A 态未 age 被间隔门跳过=恒绿假锚——「移出」断言
//    是走到闸门的处理证据，间隔门跳过不移出〕

// 锚①：模型字段锚——schema 解析含 MinRemainMin。
func TestFreeWaitModelHasMinRemainMin(t *testing.T) {
	db := setupFreeWaitDB(t) // AutoMigrate 含新列——表结构级验证
	schemaFields := freeWaitSchemaFields(db, t)
	if !schemaFields["min_remain_min"] {
		t.Fatal("FreeWaitEntry schema 缺 min_remain_min 列（字段被删——锚① FAIL）")
	}
}

func freeWaitSchemaFields(db *gorm.DB, t *testing.T) map[string]bool {
	t.Helper()
	rows, err := db.Raw("PRAGMA table_info(free_wait_entries)").Rows()
	if err != nil {
		t.Fatalf("pragma: %v", err)
	}
	defer rows.Close()
	fields := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt *string
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		fields[name] = true
	}
	return fields
}

// r20Checker 端到端锚用 checker（免费+5 分钟剩余——保护触发形态）。
type r20AnchorChecker struct{}

func (c *r20AnchorChecker) CheckDiscount(ctx context.Context, siteName, torrentID string) (model.DiscountLevel, *time.Time, error) {
	end := time.Now().Add(5 * time.Minute)
	return model.DiscountFree, &end, nil
}

// 锚②：端到端字段保真——A/B 两态用独立条目（保护触发会 Remove 连带删 DB 行，
// 同一条目先走 A 态会让 B 恢复 0 条——两态必须互不污染）。
//
// A 态（未重启·真走闸门）：age 绕间隔门 → CheckOnce 真进 discount 检查 →
// 保护跳过=不推送+条目被移出。「移出」是走到闸门的处理证据（间隔门跳过
// 不移出——防假绿：仅断言不推送时，间隔门跳过也恒绿〔用户复审发现的
// 原形态假绿〕）。
// B 态（模拟重启）：独立条目不经 A 的 CheckOnce（DB 行保留）→ 同 DB
// RecoverOnStartup → 恢复值=30（字段保真）+ age 后不推送（保护经 DB 存续）。
func TestFreeWaitMinRemainEndToEnd(t *testing.T) {
	db := setupFreeWaitDB(t)
	m := NewFreeWaitMonitor(db, zap.NewNop())

	// A 条目：未重启——age 真走保护闸门
	m.Add("s", "anchorA", "h", "T", 1, nil, 1, "sub", false, 0, 60, 30)
	ageEntries(m)
	m.CheckOnce(context.Background(), &r20AnchorChecker{}, func(ctx context.Context, e *freeWaitEntry) error {
		t.Fatal("A: 未重启不应推送（保护应跳过）")
		return nil
	})
	if _, ok := func() (e *freeWaitEntry, ok bool) {
		m.mu.Lock()
		defer m.mu.Unlock()
		e, ok = m.pending["s|anchorA"]
		return
	}(); ok {
		t.Fatal("A: 条目未被移出——间隔门跳过（假绿形态）或保护 Remove 未执行")
	}

	// B 条目：模拟重启——DB 行必须保留（A 的 Remove 不触及 B）
	m.Add("s", "anchorB", "h", "T", 1, nil, 1, "sub", false, 0, 60, 30)
	m2 := NewFreeWaitMonitor(db, zap.NewNop())
	m2.RecoverOnStartup(context.Background())
	m2.mu.Lock()
	var got int
	for _, e := range m2.pending {
		got = e.MinRemainMin
	}
	m2.mu.Unlock()
	if got != 30 {
		t.Fatalf("锚②: 恢复后 MinRemainMin=%d want 30（字段保真断裂——回填漏/字段删）", got)
	}
	ageEntries(m2)
	m2.CheckOnce(context.Background(), &r20AnchorChecker{}, func(ctx context.Context, e *freeWaitEntry) error {
		t.Fatal("B: 修复后重启保护应仍生效（不推送）")
		return nil
	})
}

// 永久免费（freeEndAt==nil）不受保护影响——回归保障。
type r20PermChecker struct{}

func (c *r20PermChecker) CheckDiscount(ctx context.Context, siteName, torrentID string) (model.DiscountLevel, *time.Time, error) {
	return model.DiscountFree, nil, nil // 永久免费
}

func TestFreeWaitPermFreeUnaffected(t *testing.T) {
	db := setupFreeWaitDB(t)
	m := NewFreeWaitMonitor(db, zap.NewNop())
	m.Add("s", "perm1", "h", "T", 1, nil, 1, "sub", false, 0, 60, 30)
	ageEntries(m)
	pushed := 0
	m.CheckOnce(context.Background(), &r20PermChecker{}, func(ctx context.Context, e *freeWaitEntry) error {
		pushed++
		return nil
	})
	if pushed != 1 {
		t.Fatalf("永久免费应直接推（设计 P2: FreeEndAt==nil 跳过检查）, pushed=%d", pushed)
	}
}
