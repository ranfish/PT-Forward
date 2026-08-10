package titleparser

import "testing"

func TestExtractMedium(t *testing.T) {
	cases := []struct {
		text   string
		name   string
		expect string
	}{
		{"WEB-DL", "Movie.2026.WEB-DL", "WEB-DL"},
		{"HDTV", "Show.HDTV", "HDTV"},
		{"Blu-ray", "Movie.Blu-ray", "Blu-ray"},
		{"Remux", "Movie.Remux", "Remux"},
		{"Encode", "Movie.x264", "Encode"},
		{"DVD", "Movie.DVDRip", "DVD"},
		{"UHD", "Movie.2026.UHD", "UHD"},
		{"", "Movie", ""},
		// webrip 排除：标题含 webrip 时不识别 WEB-DL
		{"WEB-DL", "Movie.webrip", ""},
	}
	for _, c := range cases {
		got := ExtractMedium(c.text, c.name)
		if got != c.expect {
			t.Errorf("ExtractMedium(%q, %q) = %q, want %q", c.text, c.name, got, c.expect)
		}
	}
}

func TestExtractCodec(t *testing.T) {
	cases := []struct {
		text   string
		expect string
	}{
		{"H.264", "H264"},
		{"H264", "H264"},
		{"AVC", "H264"},
		{"H.265", "H265"},
		{"HEVC", "H265"},
		{"x264", "X264"},
		{"x265", "X265"},
		{"VC-1", "VC-1"},
		{"MPEG-2", "MPEG-2"},
		{"VP9", "VP9"},
		{"", ""},
		{"unknown", ""},
	}
	for _, c := range cases {
		got := ExtractCodec(c.text)
		if got != c.expect {
			t.Errorf("ExtractCodec(%q) = %q, want %q", c.text, got, c.expect)
		}
	}
}

func TestExtractAudioCodec(t *testing.T) {
	cases := []struct {
		text   string
		expect string
	}{
		{"DTS-HDMA", "DTS-HDMA"},
		{"DTS-HD MA", "DTS-HDMA"},
		{"DTS:X 7.1", "DTS-HDMA:X 7.1"},
		{"TrueHD Atmos", "Atmos"},
		{"AC3", "AC3"},
		{" DDP 5.1", "AC3"}, // JS 正则 [ .]DD 要求 DD 前有空格或点
		{".DD", "AC3"},
		{"Dolby Digital", "AC3"},
		{"AAC", "AAC"},
		{"TrueHD", "TrueHD"},
		{"DTS", "DTS"},
		{"Flac", "Flac"},
		{"", ""},
	}
	for _, c := range cases {
		got := ExtractAudioCodec(c.text)
		if got != c.expect {
			t.Errorf("ExtractAudioCodec(%q) = %q, want %q", c.text, got, c.expect)
		}
	}
}

func TestExtractResolution(t *testing.T) {
	cases := []struct {
		text   string
		expect string
	}{
		{"1080p", "1080p"},
		{"2160p", "4K"},
		{"4K", "4K"},
		{"720p", "720p"},
		{"1080i", "1080i"},
		{"8K", "8K"},
		{"4320p", "8K"},
		{"480p", "SD"},
		{"", ""},
	}
	for _, c := range cases {
		got := ExtractResolution(c.text)
		if got != c.expect {
			t.Errorf("ExtractResolution(%q) = %q, want %q", c.text, got, c.expect)
		}
	}
}

func TestExtractSource(t *testing.T) {
	cases := []struct {
		text   string
		expect string
	}{
		{"大陆", "大陆"},
		{"China", "大陆"},
		{"欧美", "欧美"},
		{"日韩", "日韩"},
		{"香港", "香港"},
		{"台湾", "台湾"},
		{"日本", "日本"},
		{"韩国", "韩国"},
		{"印度", "印度"},
		{"", ""},
	}
	for _, c := range cases {
		got := ExtractSource(c.text)
		if got != c.expect {
			t.Errorf("ExtractSource(%q) = %q, want %q", c.text, got, c.expect)
		}
	}
}

// TestExtractFromBasicInfo 测试从 PTer/HHanClub 的"基本信息"聚合文本提取字段。
// 这是 text_extract 的主要应用场景。
func TestExtractFromBasicInfo(t *testing.T) {
	// PTer 基本信息格式（真实样本 torrent_id=859990）
	basicInfo := "大小：18.53 GB   类型: 电视剧 (TV Series)   质量: WEB-DL   地区: 欧美 (Western)"

	if v := ExtractMedium(basicInfo, ""); v != "WEB-DL" {
		t.Errorf("ExtractMedium(basicInfo) = %q, want WEB-DL", v)
	}
	if v := ExtractSource(basicInfo); v != "欧美" {
		t.Errorf("ExtractSource(basicInfo) = %q, want 欧美", v)
	}
}

// TestExtractFromTitle 测试从种子标题提取字段（与 ParseTitle 互补）。
func TestExtractFromTitle(t *testing.T) {
	title := "Ride or Die S01 2026 1080p AMZN WEB-DL H.264 DDP 5.1 Atmos-CMCTV"

	if v := ExtractMedium(title, title); v != "WEB-DL" {
		t.Errorf("ExtractMedium(title) = %q, want WEB-DL", v)
	}
	if v := ExtractCodec(title); v != "H264" {
		t.Errorf("ExtractCodec(title) = %q, want H264", v)
	}
	if v := ExtractResolution(title); v != "1080p" {
		t.Errorf("ExtractResolution(title) = %q, want 1080p", v)
	}
	// DDP → AC3（DD 匹配）
	if v := ExtractAudioCodec(title); v != "AC3" {
		t.Errorf("ExtractAudioCodec(title) = %q, want AC3", v)
	}
}
