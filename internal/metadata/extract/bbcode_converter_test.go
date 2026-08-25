package extract

import (
	"strings"
	"testing"
)

func TestHTMLToBBCode_Empty(t *testing.T) {
	out, err := HTMLToBBCode("")
	if err != nil {
		t.Errorf("empty input should not error: %v", err)
	}
	if out != "" {
		t.Errorf("empty input → empty output, got %q", out)
	}
	out, _ = HTMLToBBCode("   ")
	if out != "" {
		t.Errorf("whitespace input → empty output, got %q", out)
	}
}

func TestHTMLToBBCode_PlainText(t *testing.T) {
	out, err := HTMLToBBCode("纯文本")
	if err != nil {
		t.Fatal(err)
	}
	if out != "纯文本" {
		t.Errorf("plain text mismatch: %q", out)
	}
}

func TestHTMLToBBCode_Bold(t *testing.T) {
	out := HTMLToBBCodeMust("<b>bold</b>")
	if out != "[b]bold[/b]" {
		t.Errorf("bold mismatch: %q", out)
	}
	out = HTMLToBBCodeMust("<strong>strong</strong>")
	if out != "[b]strong[/b]" {
		t.Errorf("strong mismatch: %q", out)
	}
}

func TestHTMLToBBCode_Italic(t *testing.T) {
	out := HTMLToBBCodeMust("<i>italic</i>")
	if out != "[i]italic[/i]" {
		t.Errorf("italic mismatch: %q", out)
	}
	out = HTMLToBBCodeMust("<em>em</em>")
	if out != "[i]em[/i]" {
		t.Errorf("em mismatch: %q", out)
	}
}

func TestHTMLToBBCode_Underline(t *testing.T) {
	out := HTMLToBBCodeMust("<u>u</u>")
	if out != "[u]u[/u]" {
		t.Errorf("underline mismatch: %q", out)
	}
}

func TestHTMLToBBCode_Strike(t *testing.T) {
	for _, tag := range []string{"s", "del", "strike"} {
		out := HTMLToBBCodeMust("<" + tag + ">x</" + tag + ">")
		if out != "[s]x[/s]" {
			t.Errorf("%s mismatch: %q", tag, out)
		}
	}
}

func TestHTMLToBBCode_NestedStyles(t *testing.T) {
	// 多重样式叠加（验证双栈 accumulator 自动正确嵌套）
	out := HTMLToBBCodeMust("<b><i><u>biu</u></i></b>")
	if out != "[b][i][u]biu[/u][/i][/b]" {
		t.Errorf("nested mismatch: %q", out)
	}
}

func TestHTMLToBBCode_BR(t *testing.T) {
	out := HTMLToBBCodeMust("line1<br>line2")
	if !strings.Contains(out, "line1") || !strings.Contains(out, "line2") {
		t.Errorf("br mismatch: %q", out)
	}
	if !strings.Contains(out, "\n") {
		t.Errorf("br should produce newline: %q", out)
	}
}

func TestHTMLToBBCode_Paragraph(t *testing.T) {
	out := HTMLToBBCodeMust("<p>p1</p><p>p2</p>")
	if !strings.Contains(out, "p1") || !strings.Contains(out, "p2") {
		t.Errorf("paragraph mismatch: %q", out)
	}
	// 段落之间应有换行分隔
	if !strings.Contains(out, "\n\n") {
		t.Errorf("paragraph should have double newline: %q", out)
	}
}

func TestHTMLToBBCode_Quote(t *testing.T) {
	for _, tag := range []string{"fieldset", "blockquote", "pre", "code"} {
		out := HTMLToBBCodeMust("<" + tag + ">quoted</" + tag + ">")
		if !strings.Contains(out, "[quote]") || !strings.Contains(out, "quoted") || !strings.Contains(out, "[/quote]") {
			t.Errorf("%s quote mismatch: %q", tag, out)
		}
	}
}

func TestHTMLToBBCode_Image(t *testing.T) {
	out := HTMLToBBCodeMust(`<img src="https://example.com/x.jpg">`)
	if out != "[img]https://example.com/x.jpg[/img]" {
		t.Errorf("image mismatch: %q", out)
	}

	// data-src 兜底
	out = HTMLToBBCodeMust(`<img data-src="https://example.com/lazy.jpg">`)
	if out != "[img]https://example.com/lazy.jpg[/img]" {
		t.Errorf("data-src image mismatch: %q", out)
	}

	// src 为空 → 不输出
	out = HTMLToBBCodeMust(`<img alt="no src">`)
	if out != "" {
		t.Errorf("empty src should produce empty, got %q", out)
	}
}

func TestHTMLToBBCode_Anchor(t *testing.T) {
	out := HTMLToBBCodeMust(`<a href="https://example.com">链接</a>`)
	if out != `[url=https://example.com]链接[/url]` {
		t.Errorf("anchor mismatch: %q", out)
	}

	// href 为空 → 仅输出 text
	out = HTMLToBBCodeMust(`<a>无href</a>`)
	if out != "无href" {
		t.Errorf("empty href anchor mismatch: %q", out)
	}
}

func TestHTMLToBBCode_Center(t *testing.T) {
	out := HTMLToBBCodeMust("<center>居中</center>")
	if out != "[center]居中[/center]" {
		t.Errorf("center mismatch: %q", out)
	}
}

func TestHTMLToBBCode_Heading(t *testing.T) {
	out := HTMLToBBCodeMust("<h1>title</h1>")
	if !strings.Contains(out, `[b][size="7"]`) || !strings.Contains(out, "[/size][/b]") {
		t.Errorf("h1 mismatch: %q", out)
	}

	out = HTMLToBBCodeMust("<h3>sub</h3>")
	if !strings.Contains(out, `[size="5"]`) {
		t.Errorf("h3 size mismatch: %q", out)
	}
}

func TestHTMLToBBCode_HR(t *testing.T) {
	out := HTMLToBBCodeMust("<p>before</p><hr><p>after</p>")
	if !strings.Contains(out, "[hr]") {
		t.Errorf("hr missing: %q", out)
	}
}

func TestHTMLToBBCode_UnorderedList(t *testing.T) {
	html := `<ul><li>item1</li><li>item2</li></ul>`
	out := HTMLToBBCodeMust(html)
	if !strings.Contains(out, "- item1") {
		t.Errorf("ul/li item1 mismatch: %q", out)
	}
	if !strings.Contains(out, "- item2") {
		t.Errorf("ul/li item2 mismatch: %q", out)
	}
}

func TestHTMLToBBCode_OrderedList(t *testing.T) {
	html := `<ol><li>a</li><li>b</li></ol>`
	out := HTMLToBBCodeMust(html)
	if !strings.Contains(out, "[list=1]") || !strings.Contains(out, "[/list]") {
		t.Errorf("ol list tag mismatch: %q", out)
	}
	if !strings.Contains(out, "[*]a") || !strings.Contains(out, "[*]b") {
		t.Errorf("ol/li item mismatch: %q", out)
	}
}

func TestHTMLToBBCode_UnknownTag(t *testing.T) {
	// 未知标签应渲染子节点（不丢内容）
	out := HTMLToBBCodeMust(`<foobar>content preserved</foobar>`)
	if !strings.Contains(out, "content preserved") {
		t.Errorf("unknown tag should preserve content: %q", out)
	}
}

func TestHTMLToBBCode_ScriptStyleDropped(t *testing.T) {
	out := HTMLToBBCodeMust(`<p>text</p><script>alert("x")</script><style>.a{}</style>`)
	if strings.Contains(out, "alert") {
		t.Errorf("script content should be dropped: %q", out)
	}
	if strings.Contains(out, ".a{}") {
		t.Errorf("style content should be dropped: %q", out)
	}
	if !strings.Contains(out, "text") {
		t.Errorf("text should be kept: %q", out)
	}
}

func TestHTMLToBBCode_SpanStyleColor(t *testing.T) {
	html := `<span style="color: red">红</span>`
	out := HTMLToBBCodeMust(html)
	if !strings.Contains(out, "[color=#ff0000]") || !strings.Contains(out, "红") || !strings.Contains(out, "[/color]") {
		t.Errorf("span color mismatch: %q", out)
	}
}

func TestHTMLToBBCode_SpanStyleSize(t *testing.T) {
	html := `<span style="font-size: 18px">大</span>`
	out := HTMLToBBCodeMust(html)
	if !strings.Contains(out, "[size=4]") || !strings.Contains(out, "[/size]") {
		t.Errorf("span size mismatch: %q", out)
	}
}

func TestHTMLToBBCode_SpanStyleMultiple(t *testing.T) {
	html := `<span style="color: blue; font-weight: bold; font-style: italic">复合</span>`
	out := HTMLToBBCodeMust(html)
	if !strings.Contains(out, "[color=#0000ff]") {
		t.Errorf("multi-style missing color: %q", out)
	}
	if !strings.Contains(out, "[b]") {
		t.Errorf("multi-style missing bold: %q", out)
	}
	if !strings.Contains(out, "[i]") {
		t.Errorf("multi-style missing italic: %q", out)
	}
	if !strings.Contains(out, "复合") {
		t.Errorf("multi-style missing content: %q", out)
	}
}

func TestHTMLToBBCode_FontTag(t *testing.T) {
	html := `<font color="red" size="3">legacy</font>`
	out := HTMLToBBCodeMust(html)
	if !strings.Contains(out, "[color=#ff0000]") || !strings.Contains(out, "[size=3]") {
		t.Errorf("font tag mismatch: %q", out)
	}
}

func TestHTMLToBBCode_TextAlign(t *testing.T) {
	cases := map[string]string{
		`<div style="text-align: left">x</div>`:   "[left]",
		`<div style="text-align: right">x</div>`:  "[right]",
		`<div style="text-align: center">x</div>`: "[center]",
	}
	for html, want := range cases {
		out := HTMLToBBCodeMust(html)
		if !strings.Contains(out, want) {
			t.Errorf("align mismatch for %q: got %q, want contains %q", html, out, want)
		}
	}
}

func TestHTMLToBBCode_Table(t *testing.T) {
	// 2b.3 默认行为：table 仅换行，th/td 用 [quote]
	html := `<table><tr><th>标题</th><td>内容</td></tr></table>`
	out := HTMLToBBCodeMust(html)
	if !strings.Contains(out, "标题") || !strings.Contains(out, "内容") {
		t.Errorf("table content lost: %q", out)
	}
}

func TestHTMLToBBCode_DeepNesting(t *testing.T) {
	// 50 层内嵌套不崩溃，内容保留
	var sb strings.Builder
	for i := 0; i < 40; i++ {
		sb.WriteString("<div>")
	}
	sb.WriteString("deep")
	for i := 0; i < 40; i++ {
		sb.WriteString("</div>")
	}
	out, err := HTMLToBBCode(sb.String())
	if err != nil {
		t.Errorf("deep nesting should not error: %v", err)
	}
	if !strings.Contains(out, "deep") {
		t.Errorf("deep nesting content lost: %q", out)
	}
}

func TestHTMLToBBCode_TooLarge(t *testing.T) {
	big := strings.Repeat("a", maxHTMLBytes+1)
	_, err := HTMLToBBCode(big)
	if err == nil {
		t.Error("expected error for >1MB HTML")
	}
}

// §59.79: div.mediainfo（站方 MI 独立字段渲染区）整块剔除——147 页实测其内
// td.mi_head/codemain/pre 被转成伪 quote（[quote]General[/quote]）混入声明分类。
func TestConvert_SkipsMediaInfoDiv(t *testing.T) {
	html := `<fieldset>真声明：感谢原制作者</fieldset>
<div class='mediainfo'><a class="codetop"><span>MediaInfo: x.mkv</span></a><div class="codemain"><pre>
General
Complete name : x.mkv
</pre></div><div class="codemain"><table><tr><td class='mi_head'>General</td><td class='mi_head'>Video (1)</td></tr></table></div></div>
正文`
	out, err := HTMLToBBCode(html)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "[quote]General[/quote]") || strings.Contains(out, "Complete name") {
		t.Errorf("mediainfo 区应整块剔除: %q", out)
	}
	if !strings.Contains(out, "真声明") || !strings.Contains(out, "正文") {
		t.Errorf("区外内容保留: %q", out)
	}
}
