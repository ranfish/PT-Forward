package extract

import (
	"strings"
	"testing"
)

// BenchmarkParseHTML §56.7 要求 < 50ms/页（即 < 50,000,000 ns/op）。
func BenchmarkParseHTML(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := ParseHTML(sampleHTML)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseHTMLFromReader 流式解析（避免字符串拷贝）。
func BenchmarkParseHTMLFromReader(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := ParseHTMLFromReader(strings.NewReader(sampleHTML))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseHTML_LargePage 模拟真实 PT 站大页面（10KB+）。
func BenchmarkParseHTML_LargePage(b *testing.B) {
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html><html><head><meta charset="UTF-8"></head><body>`)
	// 模拟 200 张图片 + 50 个段落 + 10 个 MediaInfo 块
	for i := 0; i < 10; i++ {
		sb.WriteString(`<div class="codemain"><pre><code>`)
		sb.WriteString(strings.Repeat("General info line\n", 30))
		sb.WriteString(`</code></pre></div>`)
	}
	for i := 0; i < 50; i++ {
		sb.WriteString(`<p>段落正文示例 `)
		sb.WriteString(strings.Repeat("填充", 10))
		sb.WriteString(`</p>`)
	}
	for i := 0; i < 200; i++ {
		sb.WriteString(`<img src="https://example.com/shot-`)
		sb.WriteString("x")
		sb.WriteString(`.jpg"/>`)
	}
	sb.WriteString(`</body></html>`)
	large := sb.String()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseHTML(large)
		if err != nil {
			b.Fatal(err)
		}
	}
}
