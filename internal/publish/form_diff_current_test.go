package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.373: 尊重配置槽位映射——三态兼容性锚。
const baHTML = `<html><body><form>
	<select name="type" id="browsecat"><option value="401">Movie</option></select>
	<select name="source_sel[4]"><option value="23">WEB-DL</option><option value="1">Blu-ray(原盘)</option></select>
	<select name="medium_sel[4]"><option value="9">Track</option><option value="7">Encode</option></select>
	<textarea name="b"></textarea>
	<textarea name="small_descr"></textarea>
	</form></body></html>`

// ① 壹吧特化：current medium=source_sel[4]/description=b → draft 尊重配置
// （medium 值域从 source_sel 抓；medium_sel 不再冒充；简介字段名=b）。
func TestParseRespectsCurrentSlotMapping(t *testing.T) {
	current := map[string]string{
		"medium":      "source_sel[4]",
		"description": "b",
		"type":        "type",
	}
	draft := ParsePublishFormHTML(baHTML, current)
	if draft == nil {
		t.Fatal("应解析")
	}
	if draft.FormFields["medium"] != "source_sel[4]" {
		t.Fatalf("medium 槽应尊重配置 source_sel[4]，got %q", draft.FormFields["medium"])
	}
	vm := draft.ValueMappings["medium"]
	if len(vm) != 2 || vm[0].Label != "WEB-DL" {
		t.Fatalf("medium 值域应从 source_sel 抓（WEB-DL/Blu-ray），got %v", vm)
	}
	if draft.FormFields["description"] != "b" {
		t.Fatalf("description 应尊重配置 b，got %q", draft.FormFields["description"])
	}
}

// ② 幸运/龙之家标准站兼容：current 名=标准启发名 → 与 nil（现状）结果一致。
func TestParseCurrentEqualsStandardNoChange(t *testing.T) {
	current := map[string]string{
		"medium":      "medium_sel[4]",
		"description": "descr",
		"type":        "type",
	}
	withCur := ParsePublishFormHTML(baHTML, current)
	withNil := ParsePublishFormHTML(baHTML, nil)
	if withCur == nil || withNil == nil {
		t.Fatal("均应解析")
	}
	if withCur.FormFields["medium"] != withNil.FormFields["medium"] {
		t.Fatalf("标准名 current 与 nil 应同结果: %q vs %q",
			withCur.FormFields["medium"], withNil.FormFields["medium"])
	}
	if len(withCur.ValueMappings["medium"]) != len(withNil.ValueMappings["medium"]) {
		t.Fatalf("medium 值域应一致: %d vs %d",
			len(withCur.ValueMappings["medium"]), len(withNil.ValueMappings["medium"]))
	}
}

// ③ 真实改名检测保留：current medium=source_sel[4] 但页面无此字段
// （站方改名）→ 标准启发抓 medium_sel[4] → diff 可报字段名改版。
func TestParseRenamedFieldStillDetected(t *testing.T) {
	html := `<select name="medium_sel[4]"><option value="1">Blu-ray</option></select>`
	current := map[string]string{"medium": "source_sel[4]"}
	draft := ParsePublishFormHTML(html, current)
	if draft == nil {
		t.Fatal("应解析")
	}
	if draft.FormFields["medium"] != "medium_sel[4]" {
		t.Fatalf("current 名消失应 fallback 标准启发 medium_sel[4]，got %q", draft.FormFields["medium"])
	}
	if draft.FormFields[model.FieldDomainMedium] == "source_sel[4]" {
		t.Fatal("不得虚构 current 名")
	}
}
