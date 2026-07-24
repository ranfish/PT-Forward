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
