package description

import (
	"strings"
	"testing"
)

// §59.458-⑤ 定案锚：肉丝 BBCode→Markdown 转换规则全表。
func TestBBCodeToMarkdownNoImageRules(t *testing.T) {
	cases := []struct {
		name, in, wantContains, wantAbsent string
	}{
		{"引用→>前缀", "[quote]感谢分享[/quote]", "> 感谢分享", "[quote]"},
		{"多行引用逐行前缀", "[quote]line1\nline2[/quote]", "> line1\n> line2", ""},
		{"代码→围栏", "[code]General\nComplete name[/code]", "```\nGeneral", "[code]"},
		{"粗体", "[b]加粗[/b]", "**加粗**", "[b]"},
		{"斜体", "[i]斜体[/i]", "*斜体*", "[i]"},
		{"带参链接", "[url=https://example.com]示例[/url]", "[示例](https://example.com)", "[url="},
		{"裸链接", "[url]https://example.com[/url]", "[https://example.com](https://example.com)", "[url]"},
		{"图片剥除", "前文[img]https://img.example.com/a.jpg[/img]后文", "前文后文", "img.example"},
		{"img变体剥除", "[img=123]x[/img]", "", "[img"},
		{"color剥壳", "[color=Red]红字[/color]", "红字", "[color"},
		{"size剥壳", "[size=4]大字[/size]", "大字", "[size"},
		{"u剥壳", "[u]下划线[/u]", "下划线", "[u]"},
		{"列表", "[list][*]项一[*]项二[/list]", "- 项一\n- 项二", "[*]"},
		{"空输入", "", "", ""},
		{"纯文本直通", "普通文本无标签", "普通文本无标签", ""},
	}
	for _, c := range cases {
		got := BBCodeToMarkdownNoImage(c.in)
		if c.wantContains != "" && !strings.Contains(got, c.wantContains) {
			t.Errorf("%s: got %q want contains %q", c.name, got, c.wantContains)
		}
		if c.wantAbsent != "" && strings.Contains(got, c.wantAbsent) {
			t.Errorf("%s: got %q want absent %q", c.name, got, c.wantAbsent)
		}
	}
}

// 站方 4001 硬校验防御锚：转换产物绝不含 markdown 图形态与 <img。
func TestBBCodeToMarkdownNoImageSyntax(t *testing.T) {
	in := "描述 [img]https://a.example/1.jpg[/img] 中段 [b]加粗[/b] 尾部"
	got := BBCodeToMarkdownNoImage(in)
	if strings.Contains(got, "![") {
		t.Errorf("产物含 markdown 图形态: %q", got)
	}
	if strings.Contains(got, "<img") {
		t.Errorf("产物含 <img: %q", got)
	}
	if strings.Contains(got, "a.example") {
		t.Errorf("图床 URL 残留: %q", got)
	}
}
