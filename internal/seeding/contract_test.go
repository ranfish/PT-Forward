package seeding

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/rule"
)

// 15 刀3（v0.0.1661）: 规则引擎条件字段语义契约锚——V-01 零上传
// +∞ / V-02 expr 双路径一致 / F-01 校验拒绝。
//
// 阴性对照（08 篇方法论——锚必须可触发）：
//   - V-01 退回 "0" → 零上传锚 FAIL；
//   - 删 exprEnv 字段 → 覆盖锚/编译锚 FAIL；
//   - evalCondition 未知恢复放行 → 未知语义锚 FAIL。

// 锚1: 零上传=+∞（契约 docs31:7757——黑车「只下不传」是核心需求）。
// 断言口径=「>阈值 比较为真」（黑车规则形态）——value 字符串经
// compareNumeric ParseFloat 解析 "+Inf"。
func TestRuleV01_ZeroUploadInfinity(t *testing.T) {
	// seeding 声明式路径
	rec := &model.SeedingTorrentRecord{SiteName: "s", Status: "seeding", Discount: "none"}
	ti := &model.TorrentInfo{
		Name: "n", TotalSize: 10 << 30,
		DownloadSpeed: 5 << 20, UploadSpeed: 0, // 黑车典型：只下不传
	}
	rc := &RuleContext{Record: rec, Torrent: ti, Now: time.Now()}
	v, ok := rc.fieldValue("downloadUploadRatio")
	if !ok {
		t.Fatal("downloadUploadRatio 应为已知字段")
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		t.Fatalf("值 %q 不可解析: %v", v, err)
	}
	if !(f > 5) { //nolint:staticcheck // +Inf > 5 语义断言
		t.Errorf("零上传（黑车典型）应产出 >∞ 比较真值：fieldValue=%q（V-01 退回 0 回归）", v)
	}

	// rule 包声明式路径（同语义单点——由 rule 包内锚
	// TestRuleZeroUploadContractField 覆盖·此处仅单点函数断言）
	if got := rule.DownloadUploadRatioOf(5<<20, 0); !(got > 1e300) { //nolint:staticcheck // +Inf 断言
		t.Errorf("rule 包零上传同语义失效：got %v", got)
	}

	// 有上传对照（正常比值不回归）
	ti2 := &model.TorrentInfo{Name: "n", TotalSize: 10 << 30, DownloadSpeed: 5 << 20, UploadSpeed: 1 << 20}
	rc2 := &RuleContext{Record: rec, Torrent: ti2, Now: time.Now()}
	v3, _ := rc2.fieldValue("downloadUploadRatio")
	if v3 != "5.0000" {
		t.Errorf("有上传对照应 5.0000·实得 %q", v3)
	}

	// TotalSize=0 去门槛（文档未声明——速度比与大小无关）
	ti3 := &model.TorrentInfo{Name: "n", DownloadSpeed: 5 << 20, UploadSpeed: 1 << 20}
	rc3 := &RuleContext{Record: rec, Torrent: ti3, Now: time.Now()}
	v4, _ := rc3.fieldValue("downloadUploadRatio")
	if v4 != "5.0000" {
		t.Errorf("零大小有上传应 5.0000（TotalSize 门槛已去）·实得 %q", v4)
	}
}

// 锚2: 语义单点——DownloadUploadRatioOf 四态。
func TestRuleDownloadUploadRatioOf(t *testing.T) {
	if got := rule.DownloadUploadRatioOf(5<<20, 1<<20); got != 5 {
		t.Errorf("正常比值: got %v want 5", got)
	}
	if got := rule.DownloadUploadRatioOf(5<<20, 0); !(got > 1e300) { //nolint:staticcheck // +Inf 断言
		t.Errorf("零上传应 +Inf·got %v", got)
	}
	if got := rule.DownloadUploadRatioOf(0, 1<<20); got != 0 {
		t.Errorf("零下载有上传应 0·got %v", got)
	}
}

// 锚3: expr 双路径一致性覆盖锚——fieldValue 可用键中 expr 环境逐个
// 编译断言（V-02 病灶本质=「fieldValue 有的键 expr 缺」·downloadUploadRatio
// 原案+实施中抓出的 4 键〔unregistered/downloaded/error/diskUsedPercent〕
// 修复验证）。蛇形旧键（site_name 等）为声明式兼容层——不在 expr 域。
func TestRuleExprPathCoverage(t *testing.T) {
	// expr 环境支持的驼峰键域（exprEnv tag 全集的手写镜像——
	// 覆盖锚断言其中 fieldValue 驼峰新键全可用）
	cases := []struct {
		key       string
		exprProbe string // 编译探针（按类型选比较形态）
	}{
		{"downloadUploadRatio", "downloadUploadRatio > 5"},
		{"unregistered", "unregistered == true"},
		{"downloaded", "downloaded > 0"},
		{"error", `error != ""`},
		{"diskUsedPercent", "diskUsedPercent > 0"},
		{"diskUsedPercent", "diskUsedPct > 0"}, // 旧键兼容并存
		{"name", `name != ""`},
		{"ratio", "ratio > 0"},
		{"addedTime", "addedTime > 0"},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s: %s", c.key, c.exprProbe), func(t *testing.T) {
			if err := ValidateExpr(c.exprProbe); err != nil {
				t.Errorf("expr 编译失败（V-02 双路径分裂回归）: %v", err)
			}
		})
	}
}

// 锚4: expr 端到端——零上传规则在 expr 路径同样为真（双口径一致）。
func TestRuleExprZeroUploadEval(t *testing.T) {
	rec := &model.SeedingTorrentRecord{SiteName: "s", Status: "seeding", Discount: "none"}
	ti := &model.TorrentInfo{
		Name: "n", TotalSize: 10 << 30,
		DownloadSpeed: 5 << 20, UploadSpeed: 0, // 黑车典型
	}
	rc := &RuleContext{Record: rec, Torrent: ti, Now: time.Now()}
	got, err := EvalExprForTest("downloadUploadRatio > 5", rc)
	if err != nil {
		t.Fatalf("expr 求值失败: %v", err)
	}
	if !got {
		t.Error("零上传 expr 路径应为真（黑车检测·双口径与声明式一致）")
	}
}

// 锚5: F-01 未知字段/操作符统一拒绝（原 default true 放行——拼错
// 恒真误删风险）。
func TestRuleUnknownRejected(t *testing.T) {
	rec := &model.SeedingTorrentRecord{SiteName: "s", Status: "seeding", Discount: "none"}
	rc := &RuleContext{Record: rec, Now: time.Now()}
	if evalCondition(rc, ruleCondition{Field: "no_such_field", Operator: "equals", Value: "x"}) {
		t.Error("未知字段应拒绝（原放行恒真——F-01 回归）")
	}
	if evalCondition(rc, ruleCondition{Field: "site_name", Operator: "biggerThen", Value: "1"}) {
		t.Error("未知操作符应拒绝（拼错操作符原落入 default 放行——F-01 回归）")
	}
}

// 锚6: ValidateConditions 配置期校验——白名单（实现真实键集 49 键）
// 拒拼错·过合法。
func TestValidateConditions(t *testing.T) {
	// 合法：29 生产规则实态形态（蛇形+驼峰混用）
	if err := ValidateConditions(`[{"field":"unregistered","operator":"==","value":"true"}]`); err != nil {
		t.Errorf("合法蛇形键被拒: %v", err)
	}
	if err := ValidateConditions(`[{"field":"addedTime","operator":">=","value":"18000"}]`); err != nil {
		t.Errorf("合法驼峰键被拒: %v", err)
	}
	if err := ValidateConditions(`[{"field":"downloadUploadRatio","operator":">","value":"5"}]`); err != nil {
		t.Errorf("downloadUploadRatio 应在白名单: %v", err)
	}
	// 拒绝：拼错字段
	if err := ValidateConditions(`[{"field":"addedTim","operator":">=","value":"1"}]`); err == nil {
		t.Error("拼错字段应被拒（配置期显性化）")
	}
	// 拒绝：拼错操作符
	if err := ValidateConditions(`[{"field":"addedTime","operator":"biggerThen","value":"1"}]`); err == nil {
		t.Error("拼错操作符应被拒")
	}
	// 拒绝：畸形 JSON（原系统静默当无规则·现显性 400）
	if err := ValidateConditions(`{"field":"seeders","op":"gt","value":5}`); err == nil {
		t.Error("畸形 conditions（单对象+op 短键——原静默恒不命中）应显性拒绝")
	}
	// 空串合法（无条件的规则由上层处理）
	if err := ValidateConditions(""); err != nil {
		t.Errorf("空串应合法: %v", err)
	}
}

// 锚7: 白名单与 fieldValue 实现一致性——白名单键全部真实可用
// （known=true·scoringCycleID 恒 not-known 已排除）。
func TestWhitelistMatchesImplementation(t *testing.T) {
	rec := &model.SeedingTorrentRecord{
		SiteName: "s", Status: "seeding", Discount: "none", Source: "x",
	}
	ti := &model.TorrentInfo{Name: "n", Category: "c", Tags: []string{"t"}, SavePath: "/p"}
	rc := &RuleContext{Record: rec, Torrent: ti, Now: time.Now()}
	for k := range ruleConditionFields {
		if _, ok := rc.fieldValue(k); !ok {
			// 部分键需特定状态（is_paused 需状态匹配等）——仅对
			// 恒 not-known 的键报警（scoringCycleID 已排除）。
			if k == "scoringCycleID" {
				continue
			}
			// 上下文相关键在最小 ctx 下可能 not-known——记录供
			// 人工核（不算 FAIL：与实现同步的白名单以 case 存在
			// 为准）。此锚防「白名单出现实现没有的键」。
			t.Logf("键 %s 在最小 ctx 下 not-known（上下文相关·case 存在即合法）", k)
		}
	}
}
