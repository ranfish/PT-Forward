package adapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.269: 编辑表单 checkbox 数组采集与回放（缺失提交=清空勾选防护）
func TestGetEditForm_CheckboxArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><form method="post" action="takeedit.php">
		<input type="hidden" name="auth" value="abc123">
		<input type="text" name="name" value="Some.Title.2020.1080p.BluRay.x264-GRP">
		<select name="codec_sel[4]"><option value="1">H.264</option><option value="6" selected>H.265</option></select>
		<input type="checkbox" name="tags[4][]" value="5" checked>
		<input type="checkbox" name="tags[4][]" value="6" checked>
		<input type="checkbox" name="tags[4][]" value="7">
		<input type="checkbox" name="uplver" value="yes">
		<textarea name="descr">简介内容</textarea>
		<input type="submit" value="编辑">
		</form></body></html>`))
	}))
	defer srv.Close()

	a := NewNexusPHPAdapter(NewHTTPDoer(), nil)
	cfg := &model.SiteConfig{BaseURL: srv.URL, Cookie: "c=1"}
	form, err := a.GetEditForm(context.Background(), cfg, "123")
	if err != nil {
		t.Fatal(err)
	}
	if form.Title != "Some.Title.2020.1080p.BluRay.x264-GRP" {
		t.Errorf("title: %q", form.Title)
	}
	if form.Fields["codec_sel[4]"] != "6" {
		t.Errorf("codec selected: %q", form.Fields["codec_sel[4]"])
	}
	if form.Fields["auth"] != "abc123" {
		t.Errorf("hidden auth: %q", form.Fields["auth"])
	}
	// checked 2 项（5/6）；未勾选 7 不采集
	if len(form.ArrayFields) != 2 {
		t.Fatalf("array fields: %+v", form.ArrayFields)
	}
	for _, kv := range form.ArrayFields {
		if kv.Key != "tags[4][]" || (kv.Value != "5" && kv.Value != "6") {
			t.Errorf("unexpected kv: %+v", kv)
		}
	}
}
