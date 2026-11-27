package rule

import (
	"strconv"
	"testing"
)

// 15 刀1（v0.0.1661）: rule 包零上传契约锚（V-01 声明式路径）——
// docs31:7757 契约「上传=0 时为 +∞」·黑车「只下不传」核心需求。
// 断言口径=fieldValue 值经 compareNumeric 的 ParseFloat 解析后在
// >阈值 比较为真（黑车规则形态）。
//
// 阴性对照：退回 `return "0", true` → 本锚 FAIL。
func TestRuleZeroUploadContractField(t *testing.T) {
	c := &Context{TotalSize: 10 << 30, DownloadSpeed: 5 << 20, UploadSpeed: 0}
	v, ok := c.fieldValue("downloadUploadRatio")
	if !ok {
		t.Fatal("downloadUploadRatio 应为已知字段")
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		t.Fatalf("值 %q 不可解析（compareNumeric 兼容性）: %v", v, err)
	}
	if !(f > 5) { //nolint:staticcheck // +Inf > 5 语义断言
		t.Errorf("零上传（黑车典型）应为 +∞（>5 真）：fieldValue=%q——V-01 退回 0 回归", v)
	}

	// compareNumeric 端到端（黑车规则的实际比较路径）
	if cmp := compareNumeric(v, "5"); cmp <= 0 {
		t.Errorf("compareNumeric(%q, \"5\") = %d·应为正（黑车 > 阈值命中）", v, cmp)
	}

	// 有上传对照
	c2 := &Context{TotalSize: 10 << 30, DownloadSpeed: 5 << 20, UploadSpeed: 1 << 20}
	v2, _ := c2.fieldValue("downloadUploadRatio")
	if v2 != "5.0000" {
		t.Errorf("有上传对照应 5.0000·实得 %q", v2)
	}

	// 零大小去门槛（文档未声明 TotalSize 门槛）
	c3 := &Context{DownloadSpeed: 5 << 20, UploadSpeed: 1 << 20}
	v3, _ := c3.fieldValue("downloadUploadRatio")
	if v3 != "5.0000" {
		t.Errorf("零大小有上传应 5.0000（门槛已去）·实得 %q", v3)
	}
}

// 15 刀2（v0.0.1661）: rule 包 expr 编译锚（V-02）——
// downloadUploadRatio 在 ExprEnv 编译+语义单点填充。
func TestRuleExprDownloadUploadRatioCompiles(t *testing.T) {
	if err := ValidateExprForTest("downloadUploadRatio > 5"); err != nil {
		t.Errorf("rule 包 expr 编译失败（V-02 回归）: %v", err)
	}
}
