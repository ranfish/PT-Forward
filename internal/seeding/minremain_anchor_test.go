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
// 锚② 端到端字段保真（Add→同 DB RecoverOnStartup→恢复值=原值+保护仍生效
//      ——直调生产方法不 mock·专补 TestFreeWaitMonitor_RecoverOnStartup
//      只验条数不验字段的覆盖维度缺失）
// 锚③ 静态双构造块（Add 的 dbEntry 与 RecoverOnStartup 的 freeWaitEntry
//      都必须出现 MinRemainMin——沿 16/17/18/19 篇静态锚惯例）
//
// 阴性对照（刀 3）：
// A=删 model 字段 → 锚①+锚② FAIL
// B=只改 model+Add 不补 Recover 回填 → 锚② FAIL（恢复仍归零）
// C=删 dbEntry 写入行 → 锚③ 点名 FAIL

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

// 锚②：端到端字段保真——Add(30)→同 DB RecoverOnStartup→恢复 30+保护生效。
func TestFreeWaitMinRemainEndToEnd(t *testing.T) {
	db := setupFreeWaitDB(t)
	m := NewFreeWaitMonitor(db, zap.NewNop())
	m.Add("s", "anchor1", "h", "T", 1, nil, 1, "sub", false, 0, 60, 30)

	// A 未重启：保护生效（不推送）
	m.CheckOnce(context.Background(), &r20AnchorChecker{}, func(ctx context.Context, e *freeWaitEntry) error {
		t.Fatal("A: 未重启不应推送")
		return nil
	})

	// B 模拟重启：恢复后字段保真（30 ≠ 0）+保护仍生效
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
