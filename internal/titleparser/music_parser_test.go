package titleparser

import "testing"

func TestParseMusicTitle_ChinesePrefixed(t *testing.T) {
	p := ParseMusicTitle("[恩雅 - 冬季降临] Enya - And Winter Came 2008 FLAC-CMCT")
	assertEquals(t, "Artist", "Enya", p.Artist)
	assertEquals(t, "Album", "And Winter Came", p.Album)
	assertEquals(t, "Year", "2008", p.Year)
	assertEquals(t, "AudioCodec", "FLAC", p.AudioCodec)
	assertEquals(t, "ReleaseGroup", "CMCT", p.ReleaseGroup)
	assertEquals(t, "ChineseArtist", "恩雅", p.ChineseArtist)
	assertEquals(t, "ChineseAlbum", "冬季降临", p.ChineseAlbum)
}

func TestParseMusicTitle_PlainEnglish(t *testing.T) {
	p := ParseMusicTitle("Taylor Swift - 1989 2023 FLAC-FRDS")
	assertEquals(t, "Artist", "Taylor Swift", p.Artist)
	assertEquals(t, "Album", "1989", p.Album)
	assertEquals(t, "Year", "2023", p.Year)
	assertEquals(t, "AudioCodec", "FLAC", p.AudioCodec)
	assertEquals(t, "ReleaseGroup", "FRDS", p.ReleaseGroup)
}

func TestParseMusicTitle_APE(t *testing.T) {
	p := ParseMusicTitle("Adele - 25 2015 APE-GROUP")
	assertEquals(t, "AudioCodec", "APE", p.AudioCodec)
	assertEquals(t, "Album", "25", p.Album)
}

func TestReassembleMusic_ChinesePrefixed(t *testing.T) {
	p := TechProfile{
		Artist:        "Enya",
		Album:         "And Winter Came",
		Year:          "2008",
		AudioCodec:    "FLAC",
		ReleaseGroup:  "CMCT",
		ChineseArtist: "恩雅",
		ChineseAlbum:  "冬季降临",
	}
	tf := TitleFormat{Separator: " "}
	r := ReassembleFromTechProfile(p, tf)
	want := "[恩雅 - 冬季降临] Enya - And Winter Came 2008 FLAC-CMCT"
	if r != want {
		t.Errorf("got %q, want %q", r, want)
	}
}

func TestReassembleMusic_PlainEnglish(t *testing.T) {
	p := TechProfile{
		Artist:       "Taylor Swift",
		Album:        "1989",
		Year:         "2023",
		AudioCodec:   "FLAC",
		ReleaseGroup: "FRDS",
	}
	tf := TitleFormat{Separator: " "}
	r := ReassembleFromTechProfile(p, tf)
	want := "Taylor Swift - 1989 2023 FLAC-FRDS"
	if r != want {
		t.Errorf("got %q, want %q", r, want)
	}
}

func TestReassembleMusic_WithBitDepthSampleRate(t *testing.T) {
	p := TechProfile{
		Artist:       "Adele",
		Album:        "25",
		Year:         "2015",
		AudioCodec:   "FLAC",
		AudioBitDepth: "24",
		SampleRate:   "96.0",
		ReleaseGroup: "GRP",
	}
	tf := TitleFormat{Separator: " "}
	r := ReassembleFromTechProfile(p, tf)
	if !contains(r, "24bit 96.0kHz") {
		t.Errorf("should contain bit depth + sample rate, got %q", r)
	}
}

func TestReassembleMusic_DotSeparator(t *testing.T) {
	p := TechProfile{
		Artist:       "Enya",
		Album:        "Watermark",
		Year:         "1988",
		AudioCodec:   "FLAC",
		ReleaseGroup: "GRP",
	}
	tf := TitleFormat{Separator: "."}
	r := ReassembleFromTechProfile(p, tf)
	// 点号分隔时 " - " → "."（HDArea 格式）
	if !contains(r, "Enya.Watermark") {
		t.Errorf("dot separator should use . for artist-album, got %q", r)
	}
}

func TestReassembleMusic_AutoDetect(t *testing.T) {
	// Artist 非空 → 自动用音乐范式（而非视频范式）
	p := TechProfile{
		Artist:       "Test",
		Album:        "Album",
		Resolution:   "1080p", // 视频字段，不应出现在音乐标题中
		AudioCodec:   "FLAC",
		ReleaseGroup: "GRP",
	}
	tf := TitleFormat{
		Separator: " ",
		Order:     []string{"title", "resolution", "audio_codec", "group"},
	}
	r := ReassembleFromTechProfile(p, tf)
	if contains(r, "1080p") {
		t.Errorf("music should not contain video fields, got %q", r)
	}
	if !contains(r, "Test - Album") {
		t.Errorf("should contain artist - album, got %q", r)
	}
}
