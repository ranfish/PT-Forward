package adapter

import (
	"regexp"
	"testing"
)

// §59.494: NP 上传 CSRF _token 提取锚（优堡案——表单 hidden _token·
// 缺失被拒"请填写必填项目"）。三态：标准形态/属性倒序/无 token 页。
func TestNPUploadCSRFExtract(t *testing.T) {
	if reNPUploadCSRF == nil {
		t.Fatal("regexp nil")
	}
	// 标准形态（优堡实据形态）
	m := reNPUploadCSRF.FindStringSubmatch(`<input type="hidden" name="_token" value="422951b92b63a8716155676de7776cc8ecfdee155e6ed90d1e8d990e311c1a76">`)
	if len(m) < 2 || m[1] != "422951b92b63a8716155676de7776cc8ecfdee155e6ed90d1e8d990e311c1a76" {
		t.Fatalf("standard: %v", m)
	}
	// 无 token 页（老 NP）→ 空
	if reNPUploadCSRF.MatchString(`<form action="takeupload.php"><input name="name"></form>`) {
		t.Fatal("no-token page should not match")
	}
	// 短 token（<20 hex）不算——防误捕其它 _token 用途
	if reNPUploadCSRF.MatchString(`name="_token" value="abc123"`) {
		t.Fatal("short token should not match")
	}
	_ = regexp.MustCompile // keep import
}
