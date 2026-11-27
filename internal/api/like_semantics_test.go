package api

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/util"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// 13 刀3（v0.0.1656）: 查询语义端到端锚——生产同栈（GORM+SQLite）
// 验证「字面 %/_ 输入仅命中字面行」。对应 13 篇 V-01 复现矩阵
// （evidence/like-wildcard-repro.txt·修复前裸拼形态全表命中）。
//
// 阴性对照（08 篇方法论——锚必须可触发）：
//   - 退 LikeEscape 为恒等 → 全命中用例 FAIL；
//   - 删 ESCAPE 子句（Where 串去掉 util.LikeEscapeSuffix）→ 字面 % 用例
//     漏匹配 FAIL（V-02 形态：0 行）。
func TestLikeSemanticsEndToEnd(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:like_semantics_anchor?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	type row struct {
		ID    int64  `gorm:"primaryKey"`
		Title string `gorm:"column:title"`
	}
	if err := db.AutoMigrate(&row{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db.Where("1=0").Delete(&row{})
	db.Create(&[]row{{1, "ProjectA"}, {2, "ProjectB"}, {3, "Foo_Bar"}, {4, "100%done"}, {5, "abc"}})

	cases := []struct {
		name     string
		input    string
		wantHits []string // 期望命中的 Title 列表
	}{
		{"字面百分号（V-01 原案）", "%", []string{"100%done"}},
		{"字写下划线", "_", []string{"Foo_Bar"}},
		{"普通词", "Project", []string{"ProjectA", "ProjectB"}},
		{"精确字面", "abc", []string{"abc"}},
		{"普通词无命中", "zzz", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got []row
			like := util.LikeContains(c.input)
			if err := db.Where("title LIKE ?"+util.LikeEscapeSuffix, like).Find(&got).Error; err != nil {
				t.Fatalf("query: %v", err)
			}
			gotTitles := make([]string, 0, len(got))
			for _, r := range got {
				gotTitles = append(gotTitles, r.Title)
			}
			if len(gotTitles) != len(c.wantHits) {
				t.Errorf("input=%q: got %v, want %v（转义/ESCAPE 失效即全命中或漏匹配）", c.input, gotTitles, c.wantHits)
				return
			}
			for i := range gotTitles {
				if gotTitles[i] != c.wantHits[i] {
					t.Errorf("input=%q: got %v, want %v", c.input, gotTitles, c.wantHits)
				}
			}
		})
	}

	// 对照组：无转义裸拼形态（V-01 病灶形态）——证明转义前确实全表命中
	// （攻击面真实存在·非虚拟威胁）。
	var bare []row
	db.Where("title LIKE ?", "%"+"%"+"%").Find(&bare)
	if len(bare) != 5 {
		t.Errorf("裸拼对照：输入 %% 应命中全表 5 行·实得 %d（若非 5 说明 SQLite 语义漂移·锚需复核）", len(bare))
	}
}
