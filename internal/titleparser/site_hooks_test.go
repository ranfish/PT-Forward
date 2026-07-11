package titleparser

import (
	"strings"
	"testing"
)

func TestPterclubHook_Disc(t *testing.T) {
	c := TitleComponents{
		MainTitle:    "Dune",
		Year:         "2024",
		Resolution:   "2160p",
		Medium:       "UHD Blu-ray",
		VideoCodec:   "HEVC",
		AudioCodec:   "TrueHD",
		ReleaseGroup: "FraMeSToR",
	}
	tf := TitleFormat{Hook: "pterclub"}
	got := Reassemble(c, tf)

	// 原盘: 编码在音频前
	codecIdx := strings.Index(got, "HEVC")
	audioIdx := strings.Index(got, "TrueHD")
	if codecIdx < 0 || audioIdx < 0 {
		t.Fatalf("missing codec or audio in: %q", got)
	}
	if codecIdx > audioIdx {
		t.Errorf("disc: codec should be BEFORE audio, got: %q", got)
	}
	if !strings.HasSuffix(got, "-FraMeSToR") {
		t.Errorf("missing group suffix: %q", got)
	}
}

func TestPterclubHook_Encode(t *testing.T) {
	c := TitleComponents{
		MainTitle:    "Dune",
		Year:         "2024",
		Resolution:   "1080p",
		Medium:       "WEB-DL",
		VideoCodec:   "x264",
		AudioCodec:   "AAC",
		ReleaseGroup: "PTer",
	}
	tf := TitleFormat{Hook: "pterclub"}
	got := Reassemble(c, tf)

	// Encode: 编码在音频后
	codecIdx := strings.Index(got, "x264")
	audioIdx := strings.Index(got, "AAC")
	if codecIdx < 0 || audioIdx < 0 {
		t.Fatalf("missing codec or audio in: %q", got)
	}
	if codecIdx < audioIdx {
		t.Errorf("encode: codec should be AFTER audio, got: %q", got)
	}
}

func TestPterclubHook_BDRipReplaced(t *testing.T) {
	c := TitleComponents{
		MainTitle:    "Test Movie",
		Year:         "2024",
		Resolution:   "1080p",
		Medium:       "BDRip",
		VideoCodec:   "x264",
		ReleaseGroup: "PTer",
	}
	tf := TitleFormat{Hook: "pterclub"}
	got := Reassemble(c, tf)
	if strings.Contains(got, "BDRip") {
		t.Errorf("BDRip should be replaced with BluRay: %q", got)
	}
}

func TestReassemble_NoHook_UsesTemplate(t *testing.T) {
	c := TitleComponents{
		MainTitle:    "Test",
		Year:         "2024",
		VideoCodec:   "x264",
		ReleaseGroup: "GRP",
	}
	tf := TitleFormat{
		Separator: ".",
		Order:     []string{"title", "year", "video_codec", "group"},
	}
	got := Reassemble(c, tf)
	if got != "Test.2024.x264-GRP" {
		t.Errorf("template mode: got %q, want Test.2024.x264-GRP", got)
	}
}
