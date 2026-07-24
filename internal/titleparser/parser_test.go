package titleparser

import (
	"strings"
	"testing"
)

func TestParseTitle_Movie(t *testing.T) {
	c := ParseTitle("The.Dark.Knight.2008.1080p.BluRay.x264-DTS-CMCT")
	if c.MainTitle != "The Dark Knight" {
		t.Errorf("MainTitle = %q, want %q", c.MainTitle, "The Dark Knight")
	}
	if c.Year != "2008" {
		t.Errorf("Year = %q, want %q", c.Year, "2008")
	}
	if c.Resolution != "1080p" {
		t.Errorf("Resolution = %q, want %q", c.Resolution, "1080p")
	}
	if c.VideoCodec != "x264" {
		t.Errorf("VideoCodec = %q, want %q", c.VideoCodec, "x264")
	}
	if c.ReleaseGroup != "CMCT" {
		t.Errorf("ReleaseGroup = %q, want %q", c.ReleaseGroup, "CMCT")
	}
}

func TestParseTitle_TVSeries(t *testing.T) {
	c := ParseTitle("House.of.the.Dragon.S01E03.2024.2160p.MAX.WEB-DL.H265.DDP.5.1-CMCTV")
	if c.SeasonEpisode != "S01E03" {
		t.Errorf("SeasonEpisode = %q, want %q", c.SeasonEpisode, "S01E03")
	}
	if c.Resolution != "2160p" {
		t.Errorf("Resolution = %q, want %q", c.Resolution, "2160p")
	}
	if c.SourcePlatform != "MAX" {
		t.Errorf("SourcePlatform = %q, want %q", c.SourcePlatform, "MAX")
	}
	if c.VideoCodec != "H265" && c.VideoCodec != "HEVC" {
		t.Errorf("VideoCodec = %q, want H265 or HEVC", c.VideoCodec)
	}
	if c.AudioCodec != "DDP" {
		t.Errorf("AudioCodec = %q, want %q", c.AudioCodec, "DDP")
	}
	if c.ReleaseGroup != "CMCTV" {
		t.Errorf("ReleaseGroup = %q, want %q", c.ReleaseGroup, "CMCTV")
	}
}

func TestParseTitle_ChinesePrefix(t *testing.T) {
	c := ParseTitle("[蝙蝠侠] The Dark Knight 2008 1080p BluRay x264-CMCT")
	if c.ChinesePrefix != "蝙蝠侠" {
		t.Errorf("ChinesePrefix = %q, want %q", c.ChinesePrefix, "蝙蝠侠")
	}
	if c.MainTitle != "The Dark Knight" {
		t.Errorf("MainTitle = %q, want %q", c.MainTitle, "The Dark Knight")
	}
}

func TestParseTitle_HDR(t *testing.T) {
	c := ParseTitle("Dune.2024.2160p.UHD.BluRay.HDR.HEVC.TrueHD.7.1-FraMeSToR")
	if c.HDRFormat != "HDR" {
		t.Errorf("HDRFormat = %q, want %q", c.HDRFormat, "HDR")
	}
	if c.AudioCodec != "TrueHD" {
		t.Errorf("AudioCodec = %q, want %q", c.AudioCodec, "TrueHD")
	}
}

func TestParseTitle_DoVi(t *testing.T) {
	c := ParseTitle("Dune.2024.2160p.WEB-DL.DV.HDR10+.HEVC-PTer")
	if c.HDRFormat != "DoVi HDR10+" {
		t.Errorf("HDRFormat = %q, want %q", c.HDRFormat, "DoVi HDR10+")
	}
}

func TestParseTitle_Empty(t *testing.T) {
	c := ParseTitle("")
	if c.MainTitle != "" {
		t.Errorf("MainTitle = %q, want empty", c.MainTitle)
	}
}

func TestExtractGroup_NoGroup(t *testing.T) {
	g := extractGroup("Some.Title.Without.Group")
	if g != "" {
		t.Errorf("extractGroup = %q, want empty", g)
	}
}

func TestExtractResolution_4K(t *testing.T) {
	r := extractResolution("Title.4K.WEB-DL")
	if r != "4k" && r != "4K" {
		t.Errorf("resolution = %q, want 4k/4K", r)
	}
}

func TestParseTitle_H265TokenRemoved(t *testing.T) {
	c := ParseTitle("[温暖的抱抱].Warm.Hug.2020.2160p.WEB-DL.H265.AAC-UBWEB.mp4")
	if c.VideoCodec != "HEVC" {
		t.Errorf("VideoCodec = %q, want HEVC", c.VideoCodec)
	}
	if strings.Contains(strings.ToUpper(c.MainTitle), "265") {
		t.Errorf("MainTitle = %q, H265 should be removed", c.MainTitle)
	}
	if c.ReleaseGroup != "UBWEB" {
		t.Errorf("ReleaseGroup = %q, want UBWEB (without .mp4)", c.ReleaseGroup)
	}
}

func TestParseTitle_H264TokenRemoved(t *testing.T) {
	c := ParseTitle("Test.2024.1080p.NF.WEB-DL.H264.DDP-NTb")
	if c.VideoCodec != "AVC" {
		t.Errorf("VideoCodec = %q, want AVC", c.VideoCodec)
	}
	if strings.Contains(strings.ToUpper(c.MainTitle), "264") {
		t.Errorf("MainTitle = %q, H264 should be removed", c.MainTitle)
	}
}

func TestExtractGroup_FileExtension(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"Movie.2020.1080p.AAC-UBWEB.mp4", "UBWEB"},
		{"Movie.2020.1080p.AAC-FRDS.mkv", "FRDS"},
		{"Movie.2020.1080p.AAC-CMCT", "CMCT"},
	}
	for _, tt := range tests {
		got := extractGroup(tt.title)
		if got != tt.want {
			t.Errorf("extractGroup(%q) = %q, want %q", tt.title, got, tt.want)
		}
	}
}

func TestParseTitle_AC3TokenRemoved(t *testing.T) {
	c := ParseTitle("Go.Lala.Go.2.2015.BluRay.720p.x264.AC3-CMCT")
	if strings.Contains(strings.ToUpper(c.MainTitle), "AC3") {
		t.Errorf("MainTitle = %q, AC3 should be removed", c.MainTitle)
	}
	if c.AudioCodec != "DD" {
		t.Errorf("AudioCodec = %q, want DD", c.AudioCodec)
	}
}

func TestParseTitle_AudioTracksTokenRemoved(t *testing.T) {
	c := ParseTitle("Movie.2026.1080p.NF.WEB-DL.x264.DDP.5.1.Atmos.2Audios-CMCTV")
	if strings.Contains(strings.ToUpper(c.MainTitle), "2AUDIOS") {
		t.Errorf("MainTitle = %q, 2Audios should be removed", c.MainTitle)
	}
}

func TestParseTitle_SiteTagSuffixRemoved(t *testing.T) {
	c := ParseTitle("Movie.2024.BluRay.x264-CMCT [热门] [2X免费]")
	if strings.Contains(c.MainTitle, "[") || strings.Contains(c.MainTitle, "热门") {
		t.Errorf("MainTitle = %q, site tags should be removed", c.MainTitle)
	}
	if c.ReleaseGroup != "CMCT" {
		t.Errorf("ReleaseGroup = %q, want CMCT", c.ReleaseGroup)
	}
}

func TestParseTitle_SiteTagParenRemoved(t *testing.T) {
	c := ParseTitle("Movie.1975.BluRay.1080p.x264.FLAC-CMCT (已审) [2X免费]")
	if strings.Contains(c.MainTitle, "已审") || strings.Contains(c.MainTitle, "[") {
		t.Errorf("MainTitle = %q, site tags should be removed", c.MainTitle)
	}
}

func TestParseTitle_HDRTokenRemoved(t *testing.T) {
	tests := []string{
		"Movie.2025.2160p.WEB-DL.HDRVivid.H265.10",
		"Movie.2024.1080p.WEB-DL.HDR10+.H265",
		"Movie.2024.2160p.WEB-DL.DV.HDR.H265",
	}
	for _, title := range tests {
		c := ParseTitle(title)
		mtu := strings.ToUpper(c.MainTitle)
		if strings.Contains(mtu, "HDRVIVID") || strings.Contains(mtu, "HDR10") || strings.Contains(mtu, "VIVID") {
			t.Errorf("title %q: MainTitle = %q, HDR token should be removed", title, c.MainTitle)
		}
	}
}

func TestParseTitle_1440pResolution(t *testing.T) {
	c := ParseTitle("Movie.2025.1440p.WEB-DL.H265.DDP5.1-GRP")
	if !strings.Contains(strings.ToLower(c.Resolution), "1440") {
		t.Errorf("Resolution = %q, should contain 1440", c.Resolution)
	}
}

func TestParseTitle_DDPChannelsMerged(t *testing.T) {
	tests := []string{
		"Movie.2025.1080p.WEB-DL.DDP5.1.H265-GRP",
		"Movie.2025.1080p.WEB-DL.DDPA5.1.H265-GRP",
		"Movie.2024.1080p.WEB-DL.AAC2.0.H264-GRP",
	}
	for _, title := range tests {
		c := ParseTitle(title)
		// 编码+声道不应残留在 main_title
		mtu := strings.ToUpper(c.MainTitle)
		if strings.Contains(mtu, "DDP5") || strings.Contains(mtu, "DDPA") || strings.Contains(mtu, "AAC2") {
			t.Errorf("title %q: MainTitle = %q, codec+channels should be removed", title, c.MainTitle)
		}
	}
}
