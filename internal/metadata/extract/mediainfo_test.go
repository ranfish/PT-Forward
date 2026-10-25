package extract

import (
	"strings"
	"testing"
)

const sampleMediaInfo = `General
Complete name                : movie.mkv
Format                       : Matroska
File size                    : 8.5 GiB
Duration                     : 2h 15min

Video
ID                           : 1
Format                       : HEVC
Bit rate                     : 8000 kb/s

Audio
ID                           : 2
Format                       : DTS`

const sampleBDInfo = `DISC INFO:

Disc Title: MOVIE_BD
Disc Label: MOVIE.2020.COMPLETE.BLURAY-GRP
Disc Size: 25,000,000,000 bytes
Protection: AACS
BDInfo: 0.7.5.6

PLAYLIST REPORT:

Name: 00000.MPLS
Length: 1:26:21.217
Total Bitrate: 29.04 Mbps

VIDEO:

Codec: MPEG-4 AVC
Bitrate: 24480 kbps

AUDIO:

Codec: DTS-HD Master Audio
Language: English

FILES:
01: M2TS 12345
02: M2TS 67890`

func TestIsLikelyMediaInfoText(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"standard mediainfo", sampleMediaInfo, true},
		{"strong keywords only", "General info\nVideo stream\nAudio stream", true},
		{"weak + video", "Format: Matroska\nBit rate: 8000\nVideo: HEVC", true},
		{"complete name + format", "Complete name: movie.mkv\nFormat: Matroska", true},
		{"plain text", "这是一段普通简介文本", false},
		{"empty", "", false},
		{"only one keyword", "Format only", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := isLikelyMediaInfoText(c.text)
			if got != c.want {
				t.Errorf("isLikelyMediaInfoText(%q) = %v, want %v", c.name, got, c.want)
			}
		})
	}
}

func TestIsLikelyBDInfoText(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{"standard bdinfo", sampleBDInfo, true},
		// §59.348 v2 三重门实录锚：
		// 水手服案伪候选（制作组声明段——弱词 CHAPTERS: 旧版单命中伪纳）
		{"sailor suit statement (230B)", "Chapters:\n- JPN includes named chapters in Japanese. extracted the names and translated them into English, included both languages, and aligned the chapter markers to I/B-frames\n中文字幕基于Remux自带（999hk999@zimuku）制作，感谢原作者！\n感谢以上所有素材提供者！", false},
		// 织梦简报形态（Disc Title: 直接开头无大段头——锚=专属字段）
		{"quick summary digest form (dreamhd)", "Disc Title: 131313\nDisc Label: 13-13-13.-.Day.of.the.Demons.2013.Uncut.Dual.Complete.BluRay-MAMA\nDisc Size: 19,099,284,525 bytes\nProtection: AACS\nPlaylist: 01001.MPLS\nSize: 18,805,800,960 bytes\nLength: 1:26:21.217\nTotal Bitrate: 29.04 Mbps\nVideo: MPEG-4 AVC Video / 24480 kbps / 1080p / 23.976 fps / 16:9\nAudio: German / DTS-HD Master Audio / 5.1 / 48 kHz / 1452 kbps\nAudio: English / DTS-HD Master Audio / 5.1 / 48 kHz / 1524 kbps", true},
		// Remux 来源说明引用片段（有大段头词但字段行 <3——①门挡不住②门挡）
		{"remux source excerpt (few fields)", "DISC INFO:\nDisc Title: Some Movie\n来自某原盘", false},
		// 长度门（锚+字段够但物理过短）
		{"too short", "DISC INFO:\nDisc Title: X\nDisc Label: Y\nDisc Size: Z", false},
		// 旧版 keyword 形态在新版下：仅弱词不再判真
		{"disc info keyword alone", "DISC INFO\nsomething", false},
		{"quick summary keyword alone", "QUICK SUMMARY\n...", false},
		{"plain text", "普通文本", false},
		{"empty", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := IsLikelyBDInfoText(c.text)
			if got != c.want {
				t.Errorf("isLikelyBDInfoText(%q) = %v, want %v", c.name, got, c.want)
			}
		})
	}
}

func TestTrimMediaInfoLeadingNoise(t *testing.T) {
	// 有 General anchor
	text := "前缀污染\nBy ARDTU@FRDS\nGeneral\nUnique ID: 123\nVideo: HEVC"
	trimmed := trimMediaInfoLeadingNoise(text)
	if !strings.HasPrefix(strings.TrimSpace(trimmed), "General") {
		t.Errorf("should start from 'General', got %q", trimmed)
	}

	// 有分节标题（无 anchor）
	text = "前缀\nGeneral\nVideo\nAudio"
	trimmed = trimMediaInfoLeadingNoise(text)
	if strings.Contains(trimmed, "前缀") {
		t.Errorf("should trim prefix, got %q", trimmed)
	}

	// 无锚点（原样返回）
	text = "无法识别前缀的文本"
	trimmed = trimMediaInfoLeadingNoise(text)
	if trimmed != text {
		t.Errorf("should return as-is when no anchor, got %q", trimmed)
	}
}

func TestTrimBDInfoLeadingNoise(t *testing.T) {
	text := "前缀污染\nDISC INFO:\nDisc Title: X"
	trimmed := trimBDInfoLeadingNoise(text)
	if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(trimmed)), "DISC INFO") {
		t.Errorf("should start from 'DISC INFO', got %q", trimmed)
	}
}

func TestCompactBlankLinesForMediaReports(t *testing.T) {
	// 分节标题间的空行保留，其他压缩
	text := "General\n\n\nVideo\n\nAudio\n\n\nText"
	result := compactBlankLinesForMediaReports(text)
	// General 后的空行保留（后一行是 Video 标题）
	if !strings.Contains(result, "General\n\nVideo") {
		t.Errorf("blank line between sections should be kept, got %q", result)
	}
	// Audio 后的多空行压缩为 1 个
	if strings.Contains(result, "Audio\n\n\n") {
		t.Errorf("multiple blanks should be compressed, got %q", result)
	}
}

func TestPickLongest(t *testing.T) {
	candidates := []string{"short", "medium length text", "the longest text in the list"}
	got := pickLongest(candidates)
	if got != "the longest text in the list" {
		t.Errorf("pickLongest mismatch: %q", got)
	}
}

func TestExtractMediaInfo_FromPreBlock(t *testing.T) {
	// HTML 含 <pre> 块的 MediaInfo
	descrHTML := `<div>
<p>简介</p>
<pre>` + sampleMediaInfo + `</pre>
</div>`
	mi, bd := ExtractMediaInfo(descrHTML, "", "test")
	if mi == "" {
		t.Error("MediaInfo should be extracted from <pre>")
	}
	if !strings.Contains(mi, "General") {
		t.Errorf("MediaInfo should contain 'General', got %q", mi)
	}
	if bd != "" {
		t.Logf("BDInfo (expected empty): %q", bd)
	}
}

func TestExtractMediaInfo_FromBBCodeQuote(t *testing.T) {
	// BBCode 含 [quote] 块的 MediaInfo
	descrHTML := ""
	descrBBCode := `简介正文
[quote]` + sampleMediaInfo + `[/quote]
结尾`
	mi, _ := ExtractMediaInfo(descrHTML, descrBBCode, "test")
	if mi == "" {
		t.Error("MediaInfo should be extracted from [quote]")
	}
}

func TestExtractMediaInfo_BDInfo(t *testing.T) {
	descrHTML := `<div><pre>` + sampleBDInfo + `</pre></div>`
	_, bd := ExtractMediaInfo(descrHTML, "", "test")
	if bd == "" {
		t.Error("BDInfo should be extracted")
	}
	if !strings.Contains(strings.ToUpper(bd), "DISC INFO") {
		t.Errorf("BDInfo should contain 'DISC INFO', got %q", bd)
	}
}

func TestExtractMediaInfo_Mixed(t *testing.T) {
	// MediaInfo + BDInfo 共存
	descrHTML := `<div>
<pre>` + sampleMediaInfo + `</pre>
<pre>` + sampleBDInfo + `</pre>
</div>`
	mi, bd := ExtractMediaInfo(descrHTML, "", "test")
	if mi == "" {
		t.Error("MediaInfo should be extracted")
	}
	if bd == "" {
		t.Error("BDInfo should be extracted")
	}
}

func TestExtractMediaInfo_NoCandidates(t *testing.T) {
	mi, bd := ExtractMediaInfo("<p>无 mediainfo 的简介</p>", "纯文本", "test")
	if mi != "" {
		t.Errorf("expected empty MediaInfo, got %q", mi)
	}
	if bd != "" {
		t.Errorf("expected empty BDInfo, got %q", bd)
	}
}

func TestExtractMediaInfo_PicksLongest(t *testing.T) {
	// 多个 MediaInfo 候选，应选最长的
	short := "General\nVideo\nAudio"
	long := "General\nComplete name: movie.mkv\nFormat: Matroska\nFile size: 8 GiB\nDuration: 2h\nVideo\nAudio"
	descrHTML := "<pre>" + short + "</pre><pre>" + long + "</pre>"
	mi, _ := ExtractMediaInfo(descrHTML, "", "test")
	if len(mi) < len(long) {
		t.Errorf("should pick longest candidate, got len=%d (longest=%d)", len(mi), len(long))
	}
}

func TestExtractMediaInfo_PrefixNoiseRemoved(t *testing.T) {
	// 前缀污染应被去除
	descrHTML := `<pre>By ARDTU@FRDS
General
Unique ID: 123
Video: HEVC</pre>`
	mi, _ := ExtractMediaInfo(descrHTML, "", "test")
	if strings.Contains(mi, "By ARDTU") {
		t.Errorf("prefix noise should be removed, got %q", mi)
	}
}

func TestExtractMediaInfo_FromCodeMainDiv(t *testing.T) {
	descrHTML := `<div class="codemain">` + sampleMediaInfo + `</div>`
	mi, _ := ExtractMediaInfo(descrHTML, "", "test")
	if mi == "" {
		t.Error("MediaInfo should be extracted from div.codemain")
	}
}
