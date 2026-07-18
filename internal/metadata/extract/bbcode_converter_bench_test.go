package extract

import (
	"strings"
	"testing"
)

// BenchmarkHTMLToBBCode_Simple 简单 HTML（< 1KB）。
func BenchmarkHTMLToBBCode_Simple(b *testing.B) {
	html := `<p>段落 <b>粗体</b> <i>斜体</i> <a href="https://example.com">链接</a></p>`
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := HTMLToBBCode(html)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkHTMLToBBCode_100KB §56.9 要求 < 50ms / 100KB HTML。
func BenchmarkHTMLToBBCode_100KB(b *testing.B) {
	var sb strings.Builder
	// 重复 1000 次复杂段落，约 100KB
	for i := 0; i < 1000; i++ {
		sb.WriteString(`<p><span style="color: red; font-weight: bold">红色粗体</span>`)
		sb.WriteString(`<span style="font-size: 18px">大字</span>`)
		sb.WriteString(`<a href="https://example.com/page-`)
		sb.WriteString("x")
		sb.WriteString(`">链接文字</a>`)
		sb.WriteString(`<img src="https://example.com/shot-`)
		sb.WriteString("x")
		sb.WriteString(`.jpg">`)
		sb.WriteString(`<blockquote>引用块文字内容</blockquote></p>`)
	}
	html := sb.String()
	b.Logf("HTML size: %d bytes", len(html))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := HTMLToBBCode(html)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkHTMLToBBCode_Realistic 模拟真实 PT 站简介（图片+段落+引用）。
func BenchmarkHTMLToBBCode_Realistic(b *testing.B) {
	var sb strings.Builder
	sb.WriteString(`<p>简介正文段落，包含多个 <b>加粗</b> 和 <i>斜体</i>。</p>`)
	for i := 0; i < 10; i++ {
		sb.WriteString(`<p>段落正文示例 `)
		sb.WriteString(strings.Repeat("填充", 20))
		sb.WriteString(`</p>`)
	}
	for i := 0; i < 20; i++ {
		sb.WriteString(`<img src="https://example.com/shot-`)
		sb.WriteString("x")
		sb.WriteString(`.jpg"/><br>`)
	}
	sb.WriteString(`<fieldset><legend>MediaInfo</legend><pre><code>`)
	sb.WriteString(strings.Repeat("Complete name : movie.mkv\n", 30))
	sb.WriteString(`</code></pre></fieldset>`)
	html := sb.String()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := HTMLToBBCode(html)
		if err != nil {
			b.Fatal(err)
		}
	}
}
