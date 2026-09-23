package publish

import (
	"strings"
	"testing"
)


// §59.265: 站点固定资源声明整块剥除——包子/农场真实形态（243 QHstudIo 案）
func TestDeclarationFilter_SiteDisclaimers(t *testing.T) {
	text := "[quote][b]BaoziPT · 资源声明[/b]\n[list]本站提供的所有资源，不得下载用于商业盈利[/list][/quote]\n\n" +
		"正文描述保留\n\n" +
		"[quote][color=#000][b] 自由农场  - 资源声明 [/b][/color]\n1、本站提供的所有资源[/quote]\n\n" +
		"[quote][b][color=#0000ff]HDDolby官组作品，源站种子ID：245576，感谢原制作者发布。[/color][/b][/quote]\n\n" +
		"[quote][b][color=blue][size=5]QHstudIo官组作品，感谢原制作者发布。[/size][/color][/b][/quote]\n" +
		"[quote][b][color=red][size=5]互珍互重，禁转PTT[/size][/color][/b][/quote]"

	f := NewDeclarationFilter(nil, nil)
	got := f.Filter(text, defaultDeclarationPatterns)

	if strings.Contains(got.CleanedText, "BaoziPT") {
		t.Error("包子声明块未剥除")
	}
	if strings.Contains(got.CleanedText, "自由农场") {
		t.Error("农场声明块未剥除（双空格头形态）")
	}
	if !strings.Contains(got.CleanedText, "正文描述保留") {
		t.Error("正文误伤")
	}
	if !strings.Contains(got.CleanedText, "QHstudIo官组作品，感谢原制作者发布。") {
		t.Error("我方追加致谢块被误剥")
	}
	if !strings.Contains(got.CleanedText, "互珍互重，禁转PTT") {
		t.Error("禁转PTT块被误剥")
	}
	if len(got.RemovedDecls) != 2 {
		t.Errorf("应剥除恰好 2 块，实际 %d", len(got.RemovedDecls))
	}
}
