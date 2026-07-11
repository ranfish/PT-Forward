package titleparser

import (
	"testing"
)

func TestStandardize_Movie(t *testing.T) {
	c := TitleComponents{
		Resolution: "1080p",
		Medium:     "Blu-ray",
		VideoCodec: "x264",
		AudioCodec: "DTS-HD MA",
	}
	sp, err := Standardize(c)
	if err != nil {
		t.Fatal(err)
	}
	if sp.Resolution != "resolution.r1080p" {
		t.Errorf("Resolution = %q, want resolution.r1080p", sp.Resolution)
	}
	if sp.Medium != "medium.bluray" {
		t.Errorf("Medium = %q, want medium.bluray", sp.Medium)
	}
	if sp.VideoCodec != "video.x264" {
		t.Errorf("VideoCodec = %q, want video.x264", sp.VideoCodec)
	}
	if sp.AudioCodec != "audio.dts_hd_ma" {
		t.Errorf("AudioCodec = %q, want audio.dts_hd_ma", sp.AudioCodec)
	}
}

func TestStandardize_4K_HDR(t *testing.T) {
	c := TitleComponents{
		Resolution: "2160p",
		Medium:     "UHD Blu-ray",
		VideoCodec: "HEVC",
		HDRFormat:  "DoVi",
	}
	sp, err := Standardize(c)
	if err != nil {
		t.Fatal(err)
	}
	if sp.Resolution != "resolution.r2160p" {
		t.Errorf("Resolution = %q, want resolution.r2160p", sp.Resolution)
	}
	if sp.Medium != "medium.uhd_bluray" {
		t.Errorf("Medium = %q, want medium.uhd_bluray", sp.Medium)
	}
	if sp.VideoCodec != "video.h265" {
		t.Errorf("VideoCodec = %q, want video.h265 (HEVC→h265)", sp.VideoCodec)
	}
	if sp.HDR != "hdr.dv" {
		t.Errorf("HDR = %q, want hdr.dv", sp.HDR)
	}
}

func TestStandardize_ComboMedium(t *testing.T) {
	c := TitleComponents{Medium: "UHD Blu-ray Remux"}
	sp, err := Standardize(c)
	if err != nil {
		t.Fatal(err)
	}
	// 组合媒介应匹配到 uhd_bluray 或 uhd_remux
	if sp.Medium == "" {
		t.Error("Medium empty for combo medium")
	}
}

func TestStandardize_EmptyComponents(t *testing.T) {
	c := TitleComponents{}
	sp, err := Standardize(c)
	if err != nil {
		t.Fatal(err)
	}
	if sp.Resolution != "" {
		t.Errorf("Resolution = %q, want empty", sp.Resolution)
	}
}

func TestReverseLookup(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"resolution.r1080p", "1080p"},
		{"video.x264", "x264"},
		{"audio.dts_hd_ma", "DTS-HD MA"},
		{"hdr.dv", "DoVi"},
		{"medium.bluray", "Blu-ray"},
		{"nonexistent", ""},
	}
	for _, tt := range tests {
		got := ReverseLookup(tt.key)
		if got != tt.want {
			t.Errorf("ReverseLookup(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestStandardize_4K_Uppercase(t *testing.T) {
	sp, _ := Standardize(TitleComponents{Resolution: "4K"})
	if sp.Resolution != "resolution.r2160p" {
		t.Errorf("4K Resolution = %q, want resolution.r2160p", sp.Resolution)
	}
}

func TestStandardize_DDP(t *testing.T) {
	sp, _ := Standardize(TitleComponents{AudioCodec: "DDP"})
	if sp.AudioCodec != "audio.ddp" {
		t.Errorf("DDP = %q, want audio.ddp", sp.AudioCodec)
	}
}
