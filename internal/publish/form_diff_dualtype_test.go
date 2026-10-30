package publish

import (
	"testing"
)

// §59.372: 双区互斥站（壹吧）双 type select——specialcat（成人区禁发布）
// 不得覆盖 browsecat（种子区）；Diff 工具曾把 16 项成人分类冒充表单现状
// （用户上传 HTML Diff 假阳性实证）。
func TestParsePublishFormHTMLDualTypeZones(t *testing.T) {
	html := `<html><body><form>
	<tr><td>类型</td><td>
	发布至种子区 <select name="type" id="browsecat" data-mode='4'>
		<option value="0">請選擇</option>
		<option value="401">Movie(電影)</option>
		<option value="402">TV Series(電視影劇)</option>
	</select>
	或特别区 <select name="type" id="specialcat" data-mode='9'>
		<option value="0">請選擇</option>
		<option value="610">AV(有碼)/HD Censored</option>
		<option value="624">Adult film(成人電影)</option>
	</select>
	</td></tr>
	<select name="medium_sel[4]">
		<option value="1">Blu-ray(原盘)</option>
		<option value="23">WEB-DL</option>
	</select>
	</form></body></html>`
	draft := ParsePublishFormHTML(html, nil)
	if draft == nil {
		t.Fatal("应解析出表单")
	}
	vm := draft.ValueMappings["type"]
	if len(vm) != 2 {
		t.Fatalf("type 域应只收录 browsecat 2 项（specialcat 2 项不得覆盖/合并），got %d: %v", len(vm), vm)
	}
	for _, m := range vm {
		if m.Value == "610" || m.Value == "624" {
			t.Fatalf("成人区选项不得进入 type 域: %v", m)
		}
	}
	if vm[0].Value != "401" || vm[0].Label != "Movie(電影)" {
		t.Fatalf("首项应为种子区 401: %v", vm[0])
	}
	// 占位项过滤回归（value=0 繁体占位）
	if draft.ValueMappings["medium"][0].Value != "1" {
		t.Fatalf("medium 域解析异常: %v", draft.ValueMappings["medium"])
	}
}

// 单 type 标准站回归（NP 标准——无双区不受影响）。
func TestParsePublishFormHTMLSingleType(t *testing.T) {
	html := `<select name="type">
		<option value="401">电影</option>
		<option value="402">电视剧</option>
	</select>`
	draft := ParsePublishFormHTML(html, nil)
	if draft == nil || len(draft.ValueMappings["type"]) != 2 {
		t.Fatalf("标准单 type 解析异常: %v", draft)
	}
}
