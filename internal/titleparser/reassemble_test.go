package titleparser

import (
	"strings"
	"testing"
)

func TestReassemble_DotSeparated(t *testing.T) {
	c := TitleComponents{
		MainTitle:  "The Dark Knight",
		Year:       "2008",
		Resolution: "1080p",
		Medium:     "Blu-ray",
		VideoCodec: "x264",
		ReleaseGroup: "CMCT",
	}
	tf := TitleFormat{
		Separator: ".",
		Order:     []string{"title", "year", "resolution", "medium", "video_codec", "group"},
	}
	got := Reassemble(c, tf)
	// 点分隔时标题内空格替换为点
	want := "The.Dark.Knight.2008.1080p.Blu-ray.x264-CMCT"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReassemble_SpaceSeparated(t *testing.T) {
	c := TitleComponents{
		MainTitle:    "The Dark Knight",
		Year:         "2008",
		Resolution:   "1080p",
		Medium:       "BluRay",
		AudioCodec:   "DTS",
		VideoCodec:   "x264",
		ReleaseGroup: "CMCT",
	}
	tf := TitleFormat{
		Separator: " ",
		Order:     []string{"title", "year", "resolution", "medium", "audio_codec", "video_codec", "group"},
	}
	got := Reassemble(c, tf)
	want := "The Dark Knight 2008 1080p BluRay DTS x264-CMCT"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReassemble_ChinesePrefix(t *testing.T) {
	c := TitleComponents{
		MainTitle:     "The Dark Knight",
		Year:          "2008",
		Resolution:    "1080p",
		Medium:        "BluRay",
		VideoCodec:    "x264",
		ReleaseGroup:  "SiNNERS",
		ChinesePrefix: "蝙蝠侠：黑暗骑士",
	}
	tf := TitleFormat{
		Separator:     " ",
		Order:         []string{"title", "year", "resolution", "medium", "video_codec", "group"},
		ChinesePrefix: true,
	}
	got := Reassemble(c, tf)
	if !strings.HasPrefix(got, "[蝙蝠侠：黑暗骑士]") {
		t.Errorf("expected Chinese prefix, got %q", got)
	}
	if !strings.Contains(got, "The Dark Knight") {
		t.Errorf("missing main title in %q", got)
	}
}

func TestReassemble_StripForbidden(t *testing.T) {
	c := TitleComponents{
		MainTitle: "Test.BDMV.Title",
		Year:      "2024",
	}
	tf := TitleFormat{
		Separator: " ",
		Order:     []string{"title", "year"},
		Forbidden: []string{"BDMV"},
	}
	got := Reassemble(c, tf)
	if strings.Contains(got, "BDMV") {
		t.Errorf("BDMV should be stripped, got %q", got)
	}
}

func TestReassemble_SkipEmptyFields(t *testing.T) {
	c := TitleComponents{
		MainTitle:  "Test Movie",
		Year:       "2024",
		Resolution: "",
		Medium:     "",
		VideoCodec: "x264",
	}
	tf := TitleFormat{
		Separator: " ",
		Order:     []string{"title", "year", "resolution", "medium", "video_codec"},
	}
	got := Reassemble(c, tf)
	// 空字段不应产生多余空格
	if strings.Contains(got, "  ") {
		t.Errorf("double space in result: %q", got)
	}
}

func TestReassemble_DefaultFormat(t *testing.T) {
	c := TitleComponents{
		MainTitle:    "Dune",
		Year:         "2024",
		SeasonEpisode: "",
		Resolution:   "2160p",
		Medium:       "WEB-DL",
		AudioCodec:   "TrueHD",
		VideoCodec:   "HEVC",
		ReleaseGroup: "PTer",
	}
	tf := DefaultTitleFormat()
	got := Reassemble(c, tf)
	want := "Dune 2024 2160p WEB-DL TrueHD HEVC-PTer"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReassemble_TVSeries(t *testing.T) {
	c := TitleComponents{
		MainTitle:     "House of the Dragon",
		Year:          "2024",
		SeasonEpisode: "S01E03",
		Resolution:    "2160p",
		SourcePlatform: "MAX",
		Medium:        "WEB-DL",
		VideoCodec:    "HEVC",
		AudioCodec:    "DDP",
		ReleaseGroup:  "CMCTV",
	}
	tf := TitleFormat{
		Separator: ".",
		Order:     []string{"title", "season", "year", "resolution", "platform", "medium", "video_codec", "audio_codec", "group"},
	}
	got := Reassemble(c, tf)
	// 点分隔时标题内空格替换为点
	if !strings.Contains(got, "House.of.the.Dragon.S01E03") {
		t.Errorf("missing title+season in %q", got)
	}
	if !strings.Contains(got, "MAX") {
		t.Errorf("missing platform in %q", got)
	}
	if !strings.HasSuffix(got, "-CMCTV") {
		t.Errorf("missing group suffix in %q", got)
	}
}
