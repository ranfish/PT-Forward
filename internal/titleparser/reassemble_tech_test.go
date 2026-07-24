package titleparser

import "testing"

func makeTestProfile() TechProfile {
	return TechProfile{
		MainTitle:       "The Matrix",
		Year:            "1999",
		Resolution:      "2160p",
		SourceType:      "UHD Blu-ray",
		Specification:   "Remux",
		VideoCodec:      "HEVC",
		AudioCodec:      "TrueHD",
		AudioChannels:   "7.1",
		AudioTechnology: "Atmos",
		ReleaseGroup:    "FRDS",
		HDR:             "DoVi",
		BitDepth:        "10bit",
	}
}

func TestReassembleFromTechProfile_Basic(t *testing.T) {
	p := makeTestProfile()
	tf := TitleFormat{
		Separator:      " ",
		Order:          []string{"title", "year", "resolution", "medium", "video_codec", "audio_codec", "group"},
		GroupConnector: "-",
		StripChinese:   true,
	}
	result := ReassembleFromTechProfile(p, tf)
	want := "The Matrix 1999 2160p UHD Blu-ray Remux HEVC TrueHD-FRDS"
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestReassembleFromTechProfile_V105(t *testing.T) {
	p := makeTestProfile()
	p.EditionInfo = "IMAX"
	tf := V105TitleFormat()
	result := ReassembleFromTechProfile(p, tf)
	if result == "" {
		t.Error("V105 reassemble 不应返回空")
	}
	if !contains(result, "The Matrix") {
		t.Errorf("结果应含标题，got %q", result)
	}
	if !contains(result, "IMAX") {
		t.Errorf("结果应含 edition_info，got %q", result)
	}
	if !contains(result, "DoVi") {
		t.Errorf("结果应含 hdr，got %q", result)
	}
	if !contains(result, "TrueHD") {
		t.Errorf("结果应含 audio_codec，got %q", result)
	}
	if !contains(result, "7.1") {
		t.Errorf("结果应含 audio_channels，got %q", result)
	}
	if !contains(result, "-FRDS") {
		t.Errorf("结果应含 group，got %q", result)
	}
}

func TestReassembleFromTechProfile_AudioFull(t *testing.T) {
	p := TechProfile{
		MainTitle:       "Movie",
		AudioCodec:      "TrueHD",
		AudioChannels:   "7.1",
		AudioTechnology: "Atmos",
		AudioTracks:     2,
	}
	tf := TitleFormat{
		Separator: " ",
		Order:     []string{"title", "audio_full"},
	}
	result := ReassembleFromTechProfile(p, tf)
	want := "Movie TrueHD Atmos 7.1 2Audios"
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestComposeMedium(t *testing.T) {
	tests := []struct {
		p    TechProfile
		want string
	}{
		{TechProfile{SourceType: "UHD Blu-ray", Specification: "Remux"}, "UHD Blu-ray Remux"},
		{TechProfile{SourceType: "Blu-ray"}, "Blu-ray"},
		{TechProfile{Specification: "WEB-DL"}, "WEB-DL"},
		{TechProfile{}, ""},
	}
	for _, tt := range tests {
		got := composeMedium(tt.p)
		if got != tt.want {
			t.Errorf("composeMedium() = %q, want %q", got, tt.want)
		}
	}
}

func TestComposeAudio(t *testing.T) {
	tests := []struct {
		p    TechProfile
		want string
	}{
		{TechProfile{AudioCodec: "TrueHD", AudioTechnology: "Atmos", AudioChannels: "7.1"}, "TrueHD Atmos 7.1"},
		{TechProfile{AudioCodec: "DDP", AudioChannels: "5.1"}, "DDP 5.1"},
		{TechProfile{AudioCodec: "AAC", AudioChannels: "2.0"}, "AAC"},
		{TechProfile{AudioCodec: "AAC", AudioChannels: "5.1"}, "AAC 5.1"},
		{TechProfile{AudioCodec: "MP3", AudioChannels: "2.0"}, "MP3"},
		{TechProfile{AudioCodec: "DTS-HD MA", AudioChannels: "5.1", AudioTracks: 2}, "DTS-HD MA 5.1 2Audios"},
		{TechProfile{AudioCodec: "DTS-HD MA", AudioChannels: "5.1", AudioTracks: 1}, "DTS-HD MA 5.1"},
		{TechProfile{}, ""},
	}
	for _, tt := range tests {
		got := composeAudio(tt.p)
		if got != tt.want {
			t.Errorf("composeAudio(codec=%q ch=%q tech=%q tracks=%d) = %q, want %q",
				tt.p.AudioCodec, tt.p.AudioChannels, tt.p.AudioTechnology, tt.p.AudioTracks, got, tt.want)
		}
	}
}

func TestFormatBitDepthForTitle(t *testing.T) {
	tests := []struct {
		codec string
		want  string
	}{
		{"x264", "Hi10P"},
		{"AVC", "Hi10P"},
		{"H.264", "Hi10P"},
		{"HEVC", ""},
		{"x265", ""},
	}
	for _, tt := range tests {
		p := TechProfile{BitDepth: "10bit", VideoCodec: tt.codec}
		got := formatBitDepthForTitle(p)
		if got != tt.want {
			t.Errorf("formatBitDepthForTitle(codec=%q) = %q, want %q", tt.codec, got, tt.want)
		}
	}
	p8 := TechProfile{BitDepth: "8bit", VideoCodec: "x264"}
	if got := formatBitDepthForTitle(p8); got != "" {
		t.Errorf("8bit 应返回空，got %q", got)
	}
}

func TestStandardizeTechProfile(t *testing.T) {
	p := TechProfile{
		Resolution:    "2160p",
		VideoCodec:    "HEVC",
		AudioCodec:    "DTS-HD MA",
		HDR:           "DoVi",
		SourceType:    "UHD Blu-ray",
		Specification: "Remux",
	}
	sp, err := StandardizeTechProfile(p)
	if err != nil {
		t.Fatalf("StandardizeTechProfile error: %v", err)
	}
	if sp.Resolution == "" {
		t.Error("Resolution 映射不应为空")
	}
	if sp.VideoCodec == "" {
		t.Error("VideoCodec 映射不应为空")
	}
	if sp.AudioCodec == "" {
		t.Error("AudioCodec 映射不应为空")
	}
	if sp.HDR == "" {
		t.Error("HDR 映射不应为空")
	}
	if sp.Medium == "" {
		t.Error("Medium 映射不应为空")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestCodecStyle(t *testing.T) {
	tests := []struct {
		codec   string
		medium  string
		want    string
	}{
		// 原盘/Remux → AVC/HEVC
		{"HEVC", "UHD Blu-ray", "HEVC"},
		{"AVC", "Blu-ray", "AVC"},
		{"HEVC", "Blu-ray Remux", "HEVC"},
		// WEB-DL → H.264/H.265
		{"HEVC", "WEB-DL", "H.265"},
		{"AVC", "WEB-DL", "H.264"},
		// HDTV → H264/H265（无点）
		{"HEVC", "HDTV", "H265"},
		{"AVC", "HDTV", "H264"},
		// 压制编码器 x264/x265 直接返回
		{"x264", "Encode", "x264"},
		{"x265", "BDRip", "x265"},
		{"x265", "WEB-DL", "x265"},
		// 非 H.264/H.265 族原样返回
		{"AV1", "WEB-DL", "AV1"},
		{"VP9", "WEB-DL", "VP9"},
		// 无法判断媒介
		{"HEVC", "", "HEVC"},
	}
	for _, tt := range tests {
		got := codecStyle(tt.codec, tt.medium)
		if got != tt.want {
			t.Errorf("codecStyle(%q, %q) = %q, want %q", tt.codec, tt.medium, got, tt.want)
		}
	}
}

func TestNormalizeResolution(t *testing.T) {
	assertEquals(t, "lower", "1080p", normalizeResolution("1080p", ""))
	assertEquals(t, "lower2", "1080p", normalizeResolution("1080p", "lower"))
	assertEquals(t, "upper", "1080P", normalizeResolution("1080p", "upper"))
}

func TestNormalizeAudio(t *testing.T) {
	assertEquals(t, "AC3", "DD", normalizeAudio("AC3"))
	assertEquals(t, "AC-3", "DD", normalizeAudio("AC-3"))
	assertEquals(t, "E-AC-3", "DDP", normalizeAudio("E-AC-3"))
	assertEquals(t, "EAC3", "DDP", normalizeAudio("EAC3"))
	assertEquals(t, "DDP_keep", "DDP", normalizeAudio("DDP"))
	assertEquals(t, "DTS_keep", "DTS", normalizeAudio("DTS"))
}

func TestNormalizeHDR(t *testing.T) {
	assertEquals(t, "hdr10_default", "HDR10", normalizeHDR("HDR10", false))
	assertEquals(t, "hdr10_to_hdr", "HDR", normalizeHDR("HDR10", true))
	assertEquals(t, "dvi_keep", "DoVi", normalizeHDR("DoVi", true))
}

func TestAudioCountWord(t *testing.T) {
	assertEquals(t, "0", "", audioCountWord(0, ""))
	assertEquals(t, "1", "", audioCountWord(1, ""))
	assertEquals(t, "2s", "2Audios", audioCountWord(2, ""))
	assertEquals(t, "2none", "2Audio", audioCountWord(2, "none"))
}

func TestReassembleFromTechProfile_FuncOverrides(t *testing.T) {
	p := TechProfile{
		MainTitle:    "Test",
		Resolution:   "1080p",
		VideoCodec:   "HEVC",
		Specification: "WEB-DL",
		AudioCodec:   "DDP",
		ReleaseGroup: "GRP",
	}
	// 默认 func_overrides
	tf1 := TitleFormat{
		Separator: " ",
		Order:     []string{"title", "resolution", "video_codec", "group"},
	}
	r1 := ReassembleFromTechProfile(p, tf1)
	// WEB-DL + HEVC → H.265（codecStyle）
	if !contains(r1, "H.265") {
		t.Errorf("默认 codecStyle: got %q, should contain H.265", r1)
	}

	// DuckBooBee 大写 P
	tf2 := tf1
	tf2.ResolutionCase = "upper"
	r2 := ReassembleFromTechProfile(p, tf2)
	if !contains(r2, "1080P") {
		t.Errorf("upper resolution: got %q, should contain 1080P", r2)
	}
}
