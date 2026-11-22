package api

import (
	"net/http"
	"net/url"
	"testing"
)

// §59.552 契约锁定：三键名支持+优先级（size > pageSize > page_size·
// 取第一个有效值）+非法回落+截断。修改 parsePagination 行为前先改本测试
// 并在提交说明中写明契约变化。
func TestParsePagination(t *testing.T) {
	cases := []struct {
		name     string
		query    url.Values
		wantPage int
		wantSize int
	}{
		{"空参用默认", url.Values{}, 1, 20},
		{"size 键有效", url.Values{"page": {"2"}, "size": {"50"}}, 2, 50},
		{"pageSize 键有效", url.Values{"page": {"2"}, "pageSize": {"50"}}, 2, 50},
		{"page_size 键有效", url.Values{"page": {"2"}, "page_size": {"50"}}, 2, 50},
		{"优先级 size 最先", url.Values{"size": {"10"}, "pageSize": {"20"}, "page_size": {"30"}}, 1, 10},
		{"优先级 pageSize 次之", url.Values{"pageSize": {"20"}, "page_size": {"30"}}, 1, 20},
		{"size 无效时取下一有效键", url.Values{"size": {"999"}, "page_size": {"30"}}, 1, 30},
		{"size 负数回落默认", url.Values{"size": {"-5"}}, 1, 20},
		{"size 零回落默认", url.Values{"size": {"0"}}, 1, 20},
		{"size 超上限回落默认", url.Values{"size": {"201"}}, 1, 20},
		{"size 上限边界 200", url.Values{"size": {"200"}}, 1, 200},
		{"size 非数字回落默认", url.Values{"size": {"abc"}}, 1, 20},
		{"page 非法回落 1", url.Values{"page": {"abc"}}, 1, 20},
		{"page 零回落 1", url.Values{"page": {"0"}}, 1, 20},
		{"page 负数回落 1", url.Values{"page": {"-3"}}, 1, 20},
		{"page 超大截断 10000", url.Values{"page": {"20000"}}, 10000, 20},
	}
	for _, c := range cases {
		r := &http.Request{URL: &url.URL{RawQuery: c.query.Encode()}}
		page, size := parsePagination(r)
		if page != c.wantPage || size != c.wantSize {
			t.Errorf("%s: got (%d,%d), want (%d,%d)", c.name, page, size, c.wantPage, c.wantSize)
		}
	}
}

// §59.552 变体契约：特殊默认值端点（默认 50）·边界与标准版一致。
func TestParsePaginationWith(t *testing.T) {
	r := &http.Request{URL: &url.URL{RawQuery: "page=2"}}
	page, size := parsePaginationWith(r, 50, 200, 10000)
	if page != 2 || size != 50 {
		t.Errorf("默认 50 变体: got (%d,%d), want (2,50)", page, size)
	}
	r = &http.Request{URL: &url.URL{RawQuery: "page_size=-5"}}
	page, size = parsePaginationWith(r, 50, 200, 10000)
	if size != 50 {
		t.Errorf("负数回落特殊默认: got size %d, want 50", size)
	}
}
