package publish

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// §59.593（语义重定义刀 2/3 锚）: 禁转权利族 gate 判定矩阵+发布门聚合
// +列表链口径——七轮定案语义的防回归锚。
//
// 判定矩阵（用户九轮定案）：
//   官组站行+官组种签（组映射命中打签站）→ 拦（源站签·有效）
//   转载站行+官组种签（越权）→ 不拦（权利模型·PT 圈共识）
//   描述域命中（署名不可判）→ 保守拦（R8·跟随现状）
//   组名空+官组站 → 拦（个人首发保守）
//   组名空+非官组站 → 不拦（第 2 点：无官组物理不可能源站）
//   外站组（映射无命中）→ 保守拦（权利在外站不可判）
//   nil detector → 保守拦（降级语义）
//
// 阴性对照（刀 3）：
//   A=gate 撤除（回退 forbiddenSet 直判）→ 越权签锚 FAIL
//   B=发布门回退单行读 → 聚合锚 FAIL（A 行禁转签被晾——发现 1 复现）

func newGateTestDetector(t *testing.T) *SourceSiteDetector {
	t.Helper()
	d := NewSourceSiteDetector(nil, zap.NewNop())
	// 直填 cache（免 db——组→官组站映射+官组站集合派生）
	d.mu.Lock()
	d.cache = map[string]string{
		"FRDS":   "朋友",
		"CHDWEB": "",
	}
	d.mu.Unlock()
	return d
}

func gateMeta(site, title, subtitle, tags, desc string, flags []string) *model.TorrentMetadata {
	data, _ := json.Marshal(flags)
	return &model.TorrentMetadata{
		SiteName: site, Title: title, Subtitle: subtitle,
		Tags: tags, Description: desc, Flags: string(data),
	}
}

func seedGateMeta(t *testing.T, db *gorm.DB, hash, site, title, tags, desc string, flags []string, at time.Time) {
	t.Helper()
	data, _ := json.Marshal(flags)
	m := &model.TorrentMetadata{
		InfoHash: hash, SiteName: site, Title: title,
		Tags: tags, Description: desc, Flags: string(data),
	}
	m.UpdatedAt = at
	if err := db.Create(m).Error; err != nil {
		t.Fatal(err)
	}
}

// 锚①：gate 判定矩阵八态。
func TestNoTransferGate_Matrix(t *testing.T) {
	d := newGateTestDetector(t)
	// 用例构造纪律：Flags 与域文本一致（真实数据 Flags 由 extractFlags 从
	// 四域提取——有签必有域命中；凭空塞签触发数据不一致防御拦截）
	cases := []struct {
		name  string
		meta  *model.TorrentMetadata
		block bool
	}{
		{"官组站行官组种签=源站签拦", gateMeta("朋友", "Title-FRDS [禁转]", "", "", "", []string{"禁转"}), true},
		{"转载站行官组种签=越权不拦", gateMeta("转载站C", "Title-FRDS [禁转]", "", "", "", []string{"禁转"}), false},
		{"描述域命中=保守拦", gateMeta("转载站C", "Title.FRDS", "", "", "本资源禁止转载", []string{"禁转"}), true},
		{"组名空+官组站=保守拦", gateMeta("朋友", "无组名标题 [禁转]", "", "", "", []string{"禁转"}), true},
		{"组名空+非官组站=不拦", gateMeta("转载站C", "无组名标题 [禁转]", "", "", "", []string{"禁转"}), false},
		{"外站组=保守拦", gateMeta("转载站C", "Title-CHDWEB [禁转]", "", "", "", []string{"禁转"}), true},
		{"无权利族签=不拦", gateMeta("朋友", "Title.FRDS", "", "", "", []string{"限时禁转"}), false},
		{"nil meta=不拦", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := NoTransferGateBlock(d, tc.meta)
			if got != tc.block {
				t.Errorf("gate=%v want %v", got, tc.block)
			}
		})
	}
	// nil detector 保守拦
	if ok, _ := NoTransferGateBlock(nil, gateMeta("朋友", "Title.FRDS", "", "", "", []string{"禁转"})); !ok {
		t.Error("nil detector 应保守拦")
	}
}

// 锚②：发布门聚合——发现 1 根修（旧单行口径晾掉旧站行禁转签）。
func TestCheckFlags_AggregateRows(t *testing.T) {
	p, db := newDirectedTestPipeline(t)
	d := newGateTestDetector(t)
	p.SetSourceDetector(d)

	hash := "gatehashaggregate001"
	// A 行（朋友站·官组种·禁转签·较旧）+ C 行（转载站·无签·最新）
	old := time.Now().Add(-24 * time.Hour)
	seedGateMeta(t, db, hash, "朋友", "Title.FRDS", "", "", []string{"禁转"}, old)
	seedGateMeta(t, db, hash, "转载站C", "Title.FRDS", "", "", nil, time.Now())

	// 聚合读：A 行源站签有效→拦（旧单行读最新 C 行无签→漏——发现 1 复现
	// 形态的修复验证）
	blocked, reason := p.checkFlagsFromMetadata(context.Background(), hash, "转载站C", "目标站")
	if !blocked {
		t.Fatal("聚合锚：A 行（源站签）应拦——发现 1 根修失效（回退单行读？）")
	}
	if reason == "" {
		t.Error("拦截原因应为非空")
	}
}

// 锚③：发布门越权签放行（gate 生效——旧 forbiddenSet 全拦的语义变更）。
func TestCheckFlags_OverreachSignPassthrough(t *testing.T) {
	p, db := newDirectedTestPipeline(t)
	d := newGateTestDetector(t)
	p.SetSourceDetector(d)

	hash := "gatehashoverreach001"
	// 转载站 C 行：FRDS 官组种+Tags 复查有标记（标签域署名确定）→ 越权不拦
	seedGateMeta(t, db, hash, "转载站C", "Title.FRDS", `["禁转"]`, "", []string{"禁转"}, time.Now())

	blocked, _ := p.checkFlagsFromMetadata(context.Background(), hash, "转载站C", "目标站")
	if blocked {
		t.Fatal("越权签锚：转载站行的官组种签应不拦（gate 语义——回退全拦？）")
	}
}
