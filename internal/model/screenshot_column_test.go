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
