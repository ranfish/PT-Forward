package publish

import (
	"strings"
	"testing"

)

// §59.301 view-source 另存包装页自动兼容回归：真源码被转义包裹在
// td.line-content（Chrome/Edge 查看源代码标签页 Ctrl+S 形态——修道院 422 案）
func TestUnwrapViewSourceSave(t *testing.T) {
	inner := `<html><body><form action="takeupload.php"><select name="type"><option value="401">电影</option></select></form></body></html>`
	// 构造包装页：真源码转义进 line-content 单元格
	escaped := strings.ReplaceAll(strings.ReplaceAll(inner, "<", "&lt;"), ">", "&gt;")
	wrapped := `<!DOCTYPE html><html><head></head><body><div class="line-gutter-backdrop"></div><table><tbody><tr><td class="line-number" value="1"></td><td class="line-content"><span class="html-tag">` + escaped + `</span></td></tr></tbody></table></body></html>`

	got := unwrapViewSourceSave(wrapped)
	if !strings.Contains(got, "<select") {
		t.Fatalf("包装页未还原: %.120s", got)
	}
	// 非包装页原样返回
	if got := unwrapViewSourceSave(inner); got != inner {
		t.Fatalf("正常页面被误改")
	}
	// 有 line-content 但还原失败 → 原样返回
	fake := `<html><table><td class="line-content">plain text no tags</td></table></html>`
	if got := unwrapViewSourceSave(fake); got != fake {
		t.Fatalf("还原失败未回退原样")
	}
}

// 端到端：包装页直接进 ParsePublishFormHTML 应产出 draft
func TestParsePublishFormHTML_ViewSourceWrapped(t *testing.T) {
	inner := `<html><body><form action="takeupload.php">
<select name="type"><option value="0">请选择</option><option value="401">电影</option><option value="402">剧集</option></select>
<select name="medium_sel[4]"><option value="1">Blu-ray</option><option value="2">WEB-DL</option></select>
</form></body></html>`
	escaped := strings.ReplaceAll(strings.ReplaceAll(inner, "<", "&lt;"), ">", "&gt;")
	wrapped := `<html><body><table><tr><td class="line-number" value="1"></td><td class="line-content"><span class="html-tag">` + escaped + `</span></td></tr></table></body></html>`

	draft := ParsePublishFormHTML(wrapped)
	if draft == nil {
		t.Fatal("包装页解析返回 nil（422 复现）")
	}
	if draft.FormFields["type"] != "type" || draft.FormFields["medium"] != "medium_sel[4]" {
		t.Fatalf("域识别错误: %v", draft.FormFields)
	}
	if len(draft.ValueMappings["type"]) != 2 {
		t.Fatalf("type 映射数错误: %d", len(draft.ValueMappings["type"]))
	}
}
