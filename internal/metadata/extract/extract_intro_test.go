package extract

import (
	"strings"
	"testing"
)

func TestIsToolSignatureQuote(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"CSAUTO 压制", true},
		{"Encoder by FRDS", true},
		{"Ripped by xxx", true},
		{"NEXUS 发布器生成", true},
		{"感谢 ARDTU 工具压制", true}, // 含 ARDTU
		// 声明文本不再归工具签名（由 IsAcknowledgmentQuote 处理为 Statement）
		{"郑重声明：本资源仅供...", false},
		{"感谢原制作者", false},
		{"普通正文内容", false},
		{"这是一段简介", false},
		{"", false},
	}
	for _, c := range cases {
		got := IsToolSignatureQuote(c.text)
		if got != c.want {
			t.Errorf("IsToolSignatureQuote(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}

func TestIsTechParamsQuote(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"Format: Matroska\nBit rate: 8000\nWriting library: x264", true},
		{"Unique ID: 123\nComplete name: movie.mkv\nFile size: 8 GiB", true},
		{"Format profile: High\nCodec ID: V_MPEGH", true},
		{"普通文本不含技术参数", false},
		{"Format only", false}, // 只有一个关键字
		{"", false},
	}
	for _, c := range cases {
		got := IsTechParamsQuote(c.text)
		if got != c.want {
			t.Errorf("IsTechParamsQuote(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}

func TestIsByARDTUWithGroupInfo(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"By ARDTU@FRDS: 正文内容", true},
		{"by ARDTU@PTer", true},
		{"BY ARDTU 正文", true},
		{"By ARDTU", true},
		{"普通文本 By ARDTU", false}, // 不在开头
		{"", false},
	}
	for _, c := range cases {
		got := IsByARDTUWithGroupInfo(c.text)
		if got != c.want {
			t.Errorf("IsByARDTUWithGroupInfo(%q) = %v, want %v", c.text, got, c.want)
		}
	}
}

func TestStripByARDTUPrefix(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"By ARDTU@FRDS: 正文内容", "正文内容"},
		{"by ARDTU@PTer 正文", "正文"},
		{"By ARDTU 正文", "正文"},
		{"By ARDTU@FRDS", ""}, // 只有前缀
		{"无前缀的正文", "无前缀的正文"},
	}
	for _, c := range cases {
		got := StripByARDTUPrefix(c.in)
		if got != c.want {
			t.Errorf("StripByARDTUPrefix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsAcknowledgmentQuote(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"短文本", true}, // < 200 rune
		{"官组声明：仅供本站", true},
		{"感谢原制作者提供资源", true},
		{strings.Repeat("长文本", 150), false}, // > 200 rune 且不含声明关键词
		{"", false},
	}
	for _, c := range cases {
		got := IsAcknowledgmentQuote(c.text)
		if got != c.want {
			t.Errorf("IsAcknowledgmentQuote(text len=%d) = %v, want %v", len([]rune(c.text)), got, c.want)
		}
	}
}

func TestExtractQuoteBlocks(t *testing.T) {
	bbcode := `前言[quote]第一个 quote[/quote]中间[quote=有标签]第二个[/quote]结尾`
	blocks := extractQuoteBlocks(bbcode)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 quote blocks, got %d", len(blocks))
	}
	if strings.TrimSpace(blocks[0].Inner) != "第一个 quote" {
		t.Errorf("block 0 inner mismatch: %q", blocks[0].Inner)
	}
	if strings.TrimSpace(blocks[1].Inner) != "第二个" {
		t.Errorf("block 1 inner mismatch: %q", blocks[1].Inner)
	}
	if blocks[0].Start >= blocks[1].Start {
		t.Errorf("block positions wrong: 0=%d, 1=%d", blocks[0].Start, blocks[1].Start)
	}
}

func TestExtractQuoteBlocks_Empty(t *testing.T) {
	blocks := extractQuoteBlocks("无 quote 文本")
	if blocks != nil {
		t.Errorf("expected nil for no quotes, got %v", blocks)
	}
}

func TestSplitPosterAndScreenshots_FirstImage(t *testing.T) {
	urls := []string{"shot1.jpg", "shot2.jpg", "shot3.jpg"}
	poster, screenshots := ClassifyPosterAndScreenshots(urls, "")
	if poster != "shot1.jpg" {
		t.Errorf("expected first image as poster, got %q", poster)
	}
	if len(screenshots) != 2 {
		t.Errorf("expected 2 screenshots, got %d", len(screenshots))
	}
}

func TestSplitPosterAndScreenshots_KeywordPriority(t *testing.T) {
	urls := []string{"shot1.jpg", "https://img9.doubanio.com/poster.jpg", "shot2.jpg"}
	poster, screenshots := ClassifyPosterAndScreenshots(urls, "")
	if !strings.Contains(poster, "doubanio") {
		t.Errorf("expected doubanio URL as poster, got %q", poster)
	}
	if len(screenshots) != 2 {
		t.Errorf("expected 2 screenshots, got %d", len(screenshots))
	}
}

func TestSplitPosterAndScreenshots_Empty(t *testing.T) {
	poster, screenshots := ClassifyPosterAndScreenshots(nil, "")
	if poster != "" {
		t.Errorf("expected empty poster, got %q", poster)
	}
	if screenshots != nil {
		t.Errorf("expected nil screenshots, got %v", screenshots)
	}
}

func TestFilterUnwantedImages(t *testing.T) {
	urls := []string{
		"https://example.com/poster.jpg",
		"https://example.com/logo.png",    // 黑名单
		"https://example.com/icon.png",    // 黑名单
		"https://example.com/shot1.jpg",
		"https://example.com/banner.gif",  // 黑名单
	}
	valid := make([]string, 0, len(urls))
	for _, u := range urls {
		if !IsUnwantedImage(u) {
			valid = append(valid, u)
		}
	}
	if len(valid) != 2 {
		t.Errorf("expected 2 valid images, got %d: %v", len(valid), valid)
	}
}

func TestSplitIntroSections_Statement(t *testing.T) {
	// 声明 quote 在首图前（实际 PT 站常见结构：声明 → 海报 → 正文）
	descrHTML := `<div>
<blockquote>官组声明：仅供本站</blockquote>
<img src="https://example.com/poster.jpg"/>
<p>正文内容</p>
</div>`
	descrBBCode := `[quote]官组声明：仅供本站[/quote]
[img]https://example.com/poster.jpg[/img]
正文内容`

	p := NewPublicExtractor("test", "测试")
	intro := p.splitIntroSections(descrHTML, descrBBCode, false)

	if intro.Poster != "https://example.com/poster.jpg" {
		t.Errorf("Poster mismatch: %q", intro.Poster)
	}
	if !strings.Contains(intro.Statement, "官组声明") {
		t.Errorf("Statement should contain '官组声明', got %q", intro.Statement)
	}
	if strings.Contains(intro.Body, "官组声明") {
		t.Errorf("Body should not contain Statement, got %q", intro.Body)
	}
}

func TestSplitIntroSections_ARDTUStripped(t *testing.T) {
	// ARDTU 签名在首图前
	descrHTML := `<div>
<blockquote>By ARDTU@FRDS: 压制声明</blockquote>
<img src="https://example.com/poster.jpg"/>
<p>正文</p>
</div>`
	descrBBCode := `[quote]By ARDTU@FRDS: 压制声明[/quote]
[img]https://example.com/poster.jpg[/img]
正文`

	p := NewPublicExtractor("test", "测试")
	intro := p.splitIntroSections(descrHTML, descrBBCode, false)

	if len(intro.RemovedARDTUDeclarations) == 0 {
		t.Error("should have ARDTU declarations removed")
	}
	if strings.Contains(intro.Body, "By ARDTU") {
		t.Errorf("Body should not contain ARDTU signature, got %q", intro.Body)
	}
}

func TestSplitIntroSections_NoPoster(t *testing.T) {
	descrHTML := `<div><p>无图片的简介</p></div>`
	descrBBCode := `无图片的简介`

	p := NewPublicExtractor("test", "测试")
	intro := p.splitIntroSections(descrHTML, descrBBCode, false)

	if intro.Poster != "" {
		t.Errorf("expected empty poster, got %q", intro.Poster)
	}
	if intro.Body != "无图片的简介" {
		t.Errorf("Body should be the full bbcode, got %q", intro.Body)
	}
}

func TestSplitIntroSections_LogoFiltered(t *testing.T) {
	descrHTML := `<div>
<img src="https://example.com/logo.png"/>
<img src="https://example.com/poster.jpg"/>
<img src="https://example.com/shot1.jpg"/>
</div>`
	p := NewPublicExtractor("test", "测试")
	intro := p.splitIntroSections(descrHTML, "", false)

	// logo 应被过滤，poster.jpg 成为首图
	if strings.Contains(intro.Poster, "logo") {
		t.Errorf("logo should be filtered, got poster=%q", intro.Poster)
	}
	if !strings.Contains(intro.Poster, "poster.jpg") {
		t.Errorf("expected poster.jpg as poster, got %q", intro.Poster)
	}
	urls := intro.ScreenshotURLs()
	if len(urls) != 1 || urls[0] != "https://example.com/shot1.jpg" {
		t.Errorf("expected 1 screenshot, got %v", urls)
	}
}

// §59.66: quote 引用内容剥离站内相对路径布局图（trans.gif 类）——保留绝对 URL 内容图。
func TestStripQuoteLayoutImages(t *testing.T) {
	cases := []struct{ in, want string }{
		// 站内相对路径（NexusPHP 布局 hack）——剥
		{"[img]/static/pic/trans.gif[/img][b]Source #1:[/b]", "[b]Source #1:[/b]"},
		{"[img]pic/trans.gif[/img]文本", "文本"},
		// 已知布局图文件名（绝对 URL 形态的站点道具）——剥
		{"[img]https://pt.keepfrds.com/static/pic/trans.gif[/img][url=https://x.com]链接[/url]", "[url=https://x.com]链接[/url]"},
		// 绝对 URL 内容图——保留（方案 A）
		{"前[img]https://img.example.com/a.jpg[/img]后", "前[img]https://img.example.com/a.jpg[/img]后"},
		// 文本/格式/链接不动
		{"[b]加粗[/b] [i]斜体[/i] [url=https://b.com]链接[/url]", "[b]加粗[/b] [i]斜体[/i] [url=https://b.com]链接[/url]"},
		// 多个布局图混排
		{"[img]/static/pic/trans.gif[/img]A\n[img]/static/pic/trans.gif[/img]B", "A\nB"},
	}
	for _, c := range cases {
		if got := stripQuoteLayoutImages(c.in); got != c.want {
			t.Errorf("stripQuoteLayoutImages(%q)\n = %q\n want %q", c.in, got, c.want)
		}
	}
}

// §59.67: 引用三细节——多引用全采集/嵌套原样/MI 引用不入声明。
func TestSplitIntroSections_MultipleQuotesAllCollected(t *testing.T) {
	bb := `[quote]第一个声明：感谢原制作者[/quote]
中间文本
[quote]第二个声明：仅供学习交流[/quote]
[img]https://img.example.com/poster.jpg[/img]
正文与截图`
	d := (&PublicExtractor{}).splitIntroSections("", bb, false)
	if !strings.Contains(d.Statement, "第一个声明") || !strings.Contains(d.Statement, "第二个声明") {
		t.Errorf("多个声明应全部依次采集: %q", d.Statement)
	}
	if strings.Index(d.Statement, "第一个") > strings.Index(d.Statement, "第二个") {
		t.Errorf("声明应保持原序: %q", d.Statement)
	}
}

// 嵌套引用: 外层承载内层原样——内层不得重复采入 statement
func TestSplitIntroSections_NestedQuoteNoDuplicate(t *testing.T) {
	bb := `[quote]Remux来自X
[quote]内层引用：感谢字幕组[/quote]
外层尾[/quote]
[img]https://img.example.com/p.jpg[/img]
正文`
	d := (&PublicExtractor{}).splitIntroSections("", bb, false)
	n := strings.Count(d.Statement, "内层引用：感谢字幕组")
	if n != 1 {
		t.Errorf("嵌套内容应仅随外层原样出现一次(实得 %d 次): %q", n, d.Statement)
	}
	if !strings.Contains(d.Statement, "[quote]内层引用：感谢字幕组[/quote]") {
		t.Errorf("嵌套 quote 标签应原样保留: %q", d.Statement)
	}
}

// MediaInfo 以 quote 形式放简介区: 不得入 statement; MI 文本仍应可被 MI 提取层捕获
func TestSplitIntroSections_MediaInfoQuoteNotStatement(t *testing.T) {
	mi := "General\nUnique ID : 123\nComplete name : /x/a.mkv\nFormat : Matroska\nFile size : 50 GiB\nVideo\nFormat : HEVC\nBit rate : 50 Mbps\nAudio\nFormat : TrueHD\nChannel(s) : 8\n"
	bb := `[quote]声明：感谢原制作者[/quote]
[img]https://img.example.com/p.jpg[/img]
正文描述
[quote]` + mi + `[/quote]`
	d := (&PublicExtractor{}).splitIntroSections("", bb, false)
	if strings.Contains(d.Statement, "Unique ID") {
		t.Errorf("MI quote 不得归入声明: %q", d.Statement)
	}
	gotMI, _ := ExtractMediaInfo("", bb, "test")
	if gotMI == "" {
		t.Error("quote 形态 MI 应被 MI 提取层捕获(发布需要)")
	}
}

// §59.78: MI 碎片引用（mUHD 制作者把 MI 摘要拆成多个小 quote）不得入 Statement。
// "短文本即声明"启发式对 [quote]General[/quote] 类 MI 段名碎片失效（墓碑镇实锤）。
func TestSplitIntroSections_MIFragmentQuotes(t *testing.T) {
	bb := `[quote]
原盘来自Tombstone.1993
[quote]Source 内层[/quote]
[/quote]
感谢素材提供者！
[quote]General[/quote]
[quote]Video (1)[/quote]
[quote]Audio (2)[/quote]
[quote]Subtitles (5)[/quote]
[quote]Container: Matroska
Runtime: 2 h 9 min
Size: 21.6 GiB
[/quote]
[img]https://img.example.com/p.jpg[/img]
正文`
	d := (&PublicExtractor{}).splitIntroSections("", bb, false)
	if strings.Contains(d.Statement, "General") || strings.Contains(d.Statement, "Video (1)") {
		t.Errorf("MI 碎片不得入 Statement: %q", d.Statement[:min(200, len(d.Statement))])
	}
	if strings.Contains(d.Statement, "Container: Matroska") {
		t.Errorf("MI 元数据块不得入 Statement")
	}
	if !strings.Contains(d.Statement, "原盘来自") {
		t.Errorf("真声明保留: %q", d.Statement[:min(100, len(d.Statement))])
	}
	if strings.Contains(d.Body, "[quote]General[/quote]") {
		t.Errorf("MI 碎片也应从 Body 剥离")
	}
}

// §59.78 附: MI 碎片变体——音轨明细块（#1:5.1 Surround Remix 6 channels, DTS XLL@4... GiB (%)）。
// mUHD 制作者的音轨摘要行形态: "#N:<名称> <X> channels, <编码>@<码率>, <体积> (<百分比>%)"。
func TestMISectionQuote_AudioTrackDetail(t *testing.T) {
	inner := "[b]#1:5.1 Surround Remix 6 channels, DTS XLL@4 195 kb/s, 3.80 GiB (18%)[/b]\n#2:Original Dolby Stereo SR Matrixed 2 channels, FLAC@577 kb/s, 535 MiB (2%)"
	if !isMISectionQuote(inner) {
		t.Errorf("音轨明细块应判 MI 碎片: %q", inner[:60])
	}
	// 真声明防御: 含 MI 词但结构不符的多行块不判
	if isMISectionQuote("5.1音轨及镭射碟杜比立体声均保留；\n中文字幕来自 TLF字幕组") {
		t.Error("真声明不应误判")
	}
	// 边界: #N 行 + 普通文本行混合 → 不判
	if isMISectionQuote("#1:FLAC 2 channels, 577 kb/s\n普通说明文字") {
		t.Error("混合块不应判 MI 碎片")
	}
}

// §59.172: keepfrds 头区引用归一——dash 族包装 + 分类域拓宽。
// 四真页形态（PT31 Top250 合集爬取实证）：
//   ---- 形态（tid=4554 模仿游戏）海报→<b>----鸣谢----</b>→◎行
//   —— 形态（tid=7025）全角破折号变体
//   fieldset 海报后（tid=9073）——已 [quote] 化，仅靠域拓宽捕获
//   域纪律：无 ◎ 行不处理；◎ 后的 ---- 不包装（防正文噪声）
func TestKFHeadDashQuotesNormalize(t *testing.T) {
	// ① dash 包装：头区内 ---- 段获得 [quote] 壳
	in := "[img]https://x/p.jpg[/img]\n[b]----国配由优伶@CMCT授权frds发布，感谢！严禁提取转载！----\n----国配字幕来自realyinxu@CMCT，特此鸣谢！----[/b]\n◎片　　名　模仿游戏\n◎译　　名　The Imitation Game"
	out := normalizeKFHeadDashQuotes(in)
	if strings.Count(out, "[quote]") != 2 {
		t.Errorf("dash 段应包装 2 个 quote: %q", out)
	}
	if !strings.Contains(out, "[quote]----国配由优伶") {
		t.Errorf("包装应保留原文（诚实透传）: %q", out)
	}
	// ◎ 后的 ---- 不包装
	if strings.Contains(out[strings.Index(out, "◎"):], "[quote]") {
		t.Errorf("◎ 行之后不应包装: %q", out)
	}

	// ② 全角破折号变体
	in2 := "[img]https://x/p.jpg[/img]\n——韩版原盘DIY来自HDHome ，感谢！！！——\n◎片　　名　X"
	out2 := normalizeKFHeadDashQuotes(in2)
	if !strings.Contains(out2, "[quote]——韩版原盘DIY") {
		t.Errorf("—— 变体应包装: %q", out2)
	}

	// ③ 无 ◎ 行（kdouban 框架页回退语义）——原样返回
	in3 := "[img]https://x/p.jpg[/img]\n----whatever----"
	if out3 := normalizeKFHeadDashQuotes(in3); out3 != in3 {
		t.Errorf("无 ◎ 行不应处理: %q", out3)
	}

	// ④ 分隔线噪声（空内容/超长）不包装
	in4 := "[img]https://x/p.jpg[/img]\n----------------------------------------------------------------\n◎片　　名　X"
	if out4 := normalizeKFHeadDashQuotes(in4); strings.Contains(out4, "[quote]") {
		t.Errorf("纯分隔线不应包装: %q", out4)
	}
}

// §59.172: keepfrds 域拓宽——海报→◎行 之间的 [quote]（fieldset/dash 归一后）
// 分类入 Statement（原海报前域漏采，tid=9073/4554 实证）。
func TestSplitIntroSectionsKFWiden(t *testing.T) {
	p := &PublicExtractor{}
	// 海报 → quote（鸣谢短句）→ ◎正文
	bb := "[img]https://x/p.jpg[/img]\n[quote]----国配由优伶@CMCT授权，感谢！----[/quote]\n◎片　　名　模仿游戏\n正文"
	d := p.splitIntroSections("", bb, true)
	if !strings.Contains(d.Statement, "国配由优伶") {
		t.Errorf("海报→◎行 间 quote 应入 Statement（拓宽）: %q", d.Statement)
	}
	if strings.Contains(d.Body, "国配由优伶") {
		t.Errorf("入 Statement 后应从 Body 移除: %q", d.Body)
	}
	// 非 keepfrds（widen=false）：维持原海报前域——不采
	d2 := p.splitIntroSections("", bb, false)
	if strings.Contains(d2.Statement, "国配由优伶") {
		t.Errorf("非 keepfrds 不应拓宽: %q", d2.Statement)
	}
}

// §59.172 附: 行首单侧形态——[b]——文字[/b]（tid=7025 实证：分隔符仅行首无闭合）。
func TestKFHeadLeadingDashNormalize(t *testing.T) {
	in := "[img]https://x/p.jpg[/img]\n[b]——韩版原盘DIY来自HDHome ，感谢！！！[/b]\n[b]——提高视频码率，增加简,繁,韩,英字幕，音轨为ac3-5.1@640kbps[/b]\n\n◎片　　名　熔炉"
	out := normalizeKFHeadDashQuotes(in)
	if strings.Count(out, "[quote]") != 2 {
		t.Errorf("行首单侧 —— 应包装 2 行: %q", out)
	}
	if !strings.Contains(out, "[quote][b]——韩版原盘DIY") {
		t.Errorf("应整行包装（含 [b] 壳原样）: %q", out)
	}
	// 纯分隔线行（行首 ---- 但余文全 dash）不包装
	in2 := "[img]https://x/p.jpg[/img]\n--------------------------------\n◎片　　名　X"
	if out2 := normalizeKFHeadDashQuotes(in2); strings.Contains(out2, "[quote]") {
		t.Errorf("行首纯分隔线不应包装: %q", out2)
	}
	// 双侧形态行不被行首规则重复包装（已含 [quote] 跳过）
	in3 := "[img]https://x/p.jpg[/img]\n[quote]----已有壳----[/quote]\n[b]——单独行[/b]\n◎片　　名　X"
	out3 := normalizeKFHeadDashQuotes(in3)
	if strings.Count(out3, "[quote]") != 2 {
		t.Errorf("已包装行不重复，新行包装一次: %q", out3)
	}
}
