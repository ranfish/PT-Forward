package model

import (
	"encoding/json"
	"testing"
)

// §59.47: screenshots 列解析四态锚定
func TestParseScreenshotColumn(t *testing.T) {
	// JSON 数组（现行格式）
	jsonArr, _ := json.Marshal([]string{"https://a/1.jpg", "https://a/2.jpg", "https://a/3.jpg"})
	got := ParseScreenshotColumn(string(jsonArr))
	if len(got) != 3 || got[0] != "https://a/1.jpg" {
		t.Errorf("JSON 格式: got %v", got)
	}

	// 换行分隔（老格式兼容）
	got2 := ParseScreenshotColumn("https://a/1.jpg\nhttps://a/2.jpg\n")
	if len(got2) != 2 {
		t.Errorf("换行格式: got %v", got2)
	}

	// 空态
	if got3 := ParseScreenshotColumn(""); got3 != nil {
		t.Errorf("空串应返回 nil: %v", got3)
	}
	if got4 := ParseScreenshotColumn("[]"); got4 != nil {
		t.Errorf("空数组应返回 nil: %v", got4)
	}

	// 站点实际案例锚定（243 Way.We.Talk，5 张 keepfrds 图）
	sample := `["https://img.keepfrds.com/u1/x/1","https://img.keepfrds.com/u2/x/2","https://img.keepfrds.com/u3/x/3","https://img.keepfrds.com/u4/x/4","https://img.keepfrds.com/u5/x/5"]`
	got5 := ParseScreenshotColumn(sample)
	if len(got5) != 5 {
		t.Errorf("Way.We.Talk 案例: got %d 张, want 5", len(got5))
	}

	// 带空白元素的 JSON
	got6 := ParseScreenshotColumn(`["https://a/1.jpg", "", "https://a/2.jpg"]`)
	if len(got6) != 2 {
		t.Errorf("空白元素应过滤: %v", got6)
	}
}

// §59.47 同族第五处修复: 写侧单点写手锚定——列格式权威 = JSON 数组字符串
func TestFormatScreenshotColumn(t *testing.T) {
	// 事实锚定（fetcher.buildMetadata 现行格式，字面量而非 Marshal 重算——反同义反复）
	if got := FormatScreenshotColumn([]string{"https://a/1.jpg", "https://a/2.jpg"}); got != `["https://a/1.jpg","https://a/2.jpg"]` {
		t.Errorf("格式: got %s", got)
	}
	if got := FormatScreenshotColumn(nil); got != "[]" {
		t.Errorf("空列表应写 []: got %s", got)
	}
	if got := FormatScreenshotColumn([]string{}); got != "[]" {
		t.Errorf("空切片应写 []: got %s", got)
	}
	// 往返等价
	urls := []string{"https://a/1.jpg", "https://a/2.jpg", "https://a/3.jpg"}
	if rt := ParseScreenshotColumn(FormatScreenshotColumn(urls)); len(rt) != 3 || rt[2] != "https://a/3.jpg" {
		t.Errorf("往返: got %v", rt)
	}
}

// NormalizeScreenshotColumn: 透传写点归一——历史换行/裸 URL 一律转 JSON，JSON 幂等
func TestNormalizeScreenshotColumn(t *testing.T) {
	// e96ec7d0e1 实锤形态（243，换行 5 张）→ JSON
	nl := "https://img3.pixhost.cc/images/5143/761625249_shot_000.jpg\nhttps://img3.pixhost.cc/images/5143/761625263_shot_001.jpg"
	want := `["https://img3.pixhost.cc/images/5143/761625249_shot_000.jpg","https://img3.pixhost.cc/images/5143/761625263_shot_001.jpg"]`
	if got := NormalizeScreenshotColumn(nl); got != want {
		t.Errorf("换行归一: got %s", got)
	}
	// JSON 幂等
	j := `["https://a/1.jpg"]`
	if got := NormalizeScreenshotColumn(j); got != j {
		t.Errorf("JSON 幂等: got %s", got)
	}
	// 空归一
	if got := NormalizeScreenshotColumn(""); got != "[]" {
		t.Errorf("空归一: got %s", got)
	}
	// 裸单 URL
	if got := NormalizeScreenshotColumn("https://a/1.jpg"); got != `["https://a/1.jpg"]` {
		t.Errorf("裸 URL: got %s", got)
	}
}
