package adapter

import (
	"testing"
)


// §59.473: 海胆式 200 直渲组详情页成功页锚——h1 发布成功门+download 尾取
// （多版本组升序末位=新种）。
func TestClassifyHaidanSuccessRender(t *testing.T) {
	single := `<h1 align="center"> 发布成功！ </h1><p>你必须重新下载种子</p>
		<a href="download.php?id=80385">下载</a><title>HAIDAN :: 详情</title>`
	resp := classifyUploadHTML(nil, "海胆", "https://www.haidan.cc", single, "", "")
	if resp == nil || !resp.Success || resp.TorrentID != "80385" {
		t.Errorf("单版本组: %+v", resp)
	}
	// 多版本组：download×3·尾取=最新（升序实证）
	multi := `<h1 align="center"> 发布成功！ </h1>
		<a href="download.php?id=72211">v1</a><a href="download.php?id=73990">v2</a><a href="download.php?id=74264">v3</a>`
	resp2 := classifyUploadHTML(nil, "海胆", "https://www.haidan.cc", multi, "", "")
	if resp2 == nil || !resp2.Success || resp2.TorrentID != "74264" {
		t.Errorf("多版本组尾取: %+v", resp2)
	}
	// h1 标签门：非 h1 形态的"发布成功"文案不误判
	notH1 := `<p>发布成功后请耐心等待审核</p><a href="download.php?id=1">x</a>`
	resp3 := classifyUploadHTML(nil, "海胆", "https://www.haidan.cc", notH1, "", "")
	if resp3 != nil {
		t.Errorf("非 h1 文案应不判成功: %+v", resp3)
	}
}
