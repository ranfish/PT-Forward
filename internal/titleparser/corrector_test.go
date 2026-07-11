package titleparser

import (
	"strings"
	"testing"
)

const sampleMediaInfo = `General
Complete name                            : movie.mkv
Format                                   : Matroska

Video
ID                                       : 1
Format                                   : AVC
Format/Info                              : Advanced Video Codec
Format profile                           : High@L4
Format settings                          : CABAC / 4 Ref Frames
Bit depth                                : 8 bits
Writing library                          : x264 core 164
Width                                    : 1 920 pixels
Height                                   : 1 080 pixels

Audio
ID                                       : 2
Format                                   : E-AC-3
Format/Info                              : Enhanced AC-3 with Dolby Atmos
Bit rate                                 : 768 kb/s
Channel(s)                               : 6 channels
`

const sampleMediaInfoHEVC = `General
Complete name                            : movie.mkv

Video
ID                                       : 1
Format                                   : HEVC
Format/Info                              : High Efficiency Video Coding
Format profile                           : Main 10@L5@High
Bit depth                                : 10 bits
HDR format                               : Dolby Vision, Version 1.0, dvhe.07.06, BL+EL+RPU
HDR format                               : SMPTE ST 2086, HDR10 compatible
Width                                    : 3 840 pixels
Height                                   : 2 160 pixels

Audio
ID                                       : 2
Format                                   : DTS-HD MA
Format/Info                              : DTS-HD Master Audio
Channel(s)                               : 8 channels
`

func TestCorrectWithMediaInfo_Empty(t *testing.T) {
	c := &TitleComponents{}
	err := CorrectWithMediaInfo(c, "")
	if err == nil {
		t.Error("expected error for empty MediaInfo")
	}
}

func TestCorrectWithMediaInfo_CodecFromWritingLibrary(t *testing.T) {
	c := &TitleComponents{VideoCodec: "H.264"}
	err := CorrectWithMediaInfo(c, sampleMediaInfo)
	if err != nil {
		t.Fatal(err)
	}
	// Writing library says x264 → should be x264
	if c.VideoCodec != "x264" {
		t.Errorf("VideoCodec = %q, want x264 (from Writing library)", c.VideoCodec)
	}
}

func TestCorrectWithMediaInfo_Height(t *testing.T) {
	c := &TitleComponents{Resolution: "720p"}
	err := CorrectWithMediaInfo(c, sampleMediaInfo)
	if err != nil {
		t.Fatal(err)
	}
	if c.Resolution != "1080p" {
		t.Errorf("Resolution = %q, want 1080p (from Height 1080)", c.Resolution)
	}
}

func TestCorrectWithMediaInfo_AudioFormat(t *testing.T) {
	c := &TitleComponents{AudioCodec: ""}
	err := CorrectWithMediaInfo(c, sampleMediaInfo)
	if err != nil {
		t.Fatal(err)
	}
	if c.AudioCodec != "DDP" {
		t.Errorf("AudioCodec = %q, want DDP (from E-AC-3)", c.AudioCodec)
	}
}

func TestCorrectWithMediaInfo_HEVC_Disc(t *testing.T) {
	c := &TitleComponents{VideoCodec: "HEVC", Medium: "UHD Blu-ray"}
	err := CorrectWithMediaInfo(c, sampleMediaInfoHEVC)
	if err != nil {
		t.Fatal(err)
	}
	// Disc + HEVC → should stay HEVC
	if c.VideoCodec != "HEVC" {
		t.Errorf("VideoCodec = %q, want HEVC (disc)", c.VideoCodec)
	}
	// Height 2160 → 2160p
	if c.Resolution != "2160p" {
		t.Errorf("Resolution = %q, want 2160p", c.Resolution)
	}
	// Bit depth 10
	if !strings.Contains(c.BitDepth, "10") {
		t.Errorf("BitDepth = %q, want 10bit", c.BitDepth)
	}
	// HDR: Dolby Vision → DoVi
	if c.HDRFormat != "DoVi" && c.HDRFormat != "DoVi HDR" {
		t.Errorf("HDRFormat = %q, want DoVi or DoVi HDR", c.HDRFormat)
	}
	// Audio: DTS-HD MA
	if c.AudioCodec != "DTS-HD MA" {
		t.Errorf("AudioCodec = %q, want DTS-HD MA", c.AudioCodec)
	}
}

func TestParseMediaInfo_HeightParsing(t *testing.T) {
	mi := parseMediaInfo(sampleMediaInfo)
	if mi.height != 1080 {
		t.Errorf("height = %d, want 1080", mi.height)
	}
	mi2 := parseMediaInfo(sampleMediaInfoHEVC)
	if mi2.height != 2160 {
		t.Errorf("height = %d, want 2160", mi2.height)
	}
}
