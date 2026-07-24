package titleparser

import (
	"testing"
)

func TestExtractMediaInfo_Empty(t *testing.T) {
	r := ExtractMediaInfo("")
	if r.Resolution != "" || r.VideoCodec != "" || r.AudioCodec != "" {
		t.Fatalf("空输入应返回零值，got %+v", r)
	}
}

func TestExtractMediaInfo_HEVC_HDR10_Atmos(t *testing.T) {
	mi := `General
Format                                   : Matroska

Video
ID                                       : 1
Format                                   : HEVC
HDR format                               : Dolby Vision, Version 1.0, dvhe.05.06, BL+RPU
Width                                    : 3 840 pixels
Height                                   : 2 160 pixels
Bit depth                                : 10 bits

Audio #1
ID                                       : 2
Format                                   : E-AC-3 JOC
Commercial name                          : Dolby Digital Plus with Dolby Atmos
Channel(s)                               : 6 channels
Channel layout                           : L R C LFE Ls Rs
`
	r := ExtractMediaInfo(mi)
	assertEquals(t, "Resolution", "2160p", r.Resolution)
	assertEquals(t, "VideoCodec", "HEVC", r.VideoCodec)
	assertEquals(t, "BitDepth", "10bit", r.BitDepth)
	assertEquals(t, "HDR", "DoVi", r.HDR)
	assertEquals(t, "AudioCodec", "DDP", r.AudioCodec)
	assertEquals(t, "AudioTechnology", "Atmos", r.AudioTechnology)
	assertEquals(t, "AudioChannels", "5.1", r.AudioChannels)
	assertEqualsInt(t, "AudioTracks", 1, r.AudioTracks)
}

func TestExtractMediaInfo_x264_DTSXLL_MultiAudio(t *testing.T) {
	mi := `General
Format                                   : Matroska

Video
Format                                   : AVC
Width                                    : 1 920 pixels
Height                                   : 800 pixels
Bit depth                                : 8 bits
Writing library                          : x264 core 115

Audio #1
Format                                   : DTS XLL
Commercial name                          : DTS-HD Master Audio
Channel(s)                               : 6 channels
Channel layout                           : C L R Ls Rs LFE

Audio #2
Format                                   : AC-3
Commercial name                          : Dolby Digital
Channel(s)                               : 6 channels
Channel layout                           : L R C LFE Ls Rs

Audio #3
Format                                   : AC-3
Commercial name                          : Dolby Digital
Channel(s)                               : 2 channels
Channel layout                           : L R
`
	r := ExtractMediaInfo(mi)
	assertEquals(t, "VideoCodec", "x264", r.VideoCodec)
	assertEquals(t, "Resolution", "1080p", r.Resolution)
	assertEquals(t, "BitDepth", "8bit", r.BitDepth)
	assertEquals(t, "AudioCodec", "DTS-HD MA", r.AudioCodec)
	assertEquals(t, "AudioChannels", "5.1", r.AudioChannels)
	// Audio #1=DTS-HD MA(高清) + Audio #2=AC-3(兼容) → 排除 1 → 3-1=2
	assertEqualsInt(t, "AudioTracks", 2, r.AudioTracks)
}

func TestExtractMediaInfo_TrueHD_Atmos_Compat(t *testing.T) {
	mi := `Video
Format                                   : HEVC
Width                                    : 3840 pixels

Audio #1
Format                                   : MLP FBA 16-ch
Commercial name                          : Dolby TrueHD with Dolby Atmos
Channel(s)                               : 8 channels
Channel layout                           : L R C LFE Ls Rs Lb Rb

Audio #2
Format                                   : AC-3
Commercial name                          : Dolby Digital
Channel(s)                               : 6 channels
Channel layout                           : L R C LFE Ls Rs
`
	r := ExtractMediaInfo(mi)
	assertEquals(t, "AudioCodec", "TrueHD", r.AudioCodec)
	assertEquals(t, "AudioTechnology", "Atmos", r.AudioTechnology)
	assertEquals(t, "AudioChannels", "7.1", r.AudioChannels)
	assertEqualsInt(t, "AudioTracks", 1, r.AudioTracks) // TrueHD + AC-3 兼容 → 2-1=1
}

func TestExtractMediaInfo_HDRVivid_V265(t *testing.T) {
	mi := `Video
Format                                   : HEVC
HDR format                               : HDR Vivid, Version 1
Width                                    : 3 840 pixels
Writing library                          : Tencent-V265-1.6.9.6

Audio
Format                                   : AAC LC
Channel(s)                               : 2 channels
Channel layout                           : L R
`
	r := ExtractMediaInfo(mi)
	assertEquals(t, "VideoCodec", "HEVC", r.VideoCodec) // V265 → 用 Format
	assertEquals(t, "HDR", "HDR Vivid", r.HDR)
	assertEquals(t, "AudioCodec", "AAC", r.AudioCodec)
	assertEquals(t, "AudioChannels", "2.0", r.AudioChannels)
}

func TestExtractMediaInfo_AV3A(t *testing.T) {
	mi := `Audio #1
Format                                   : av3a
Channel(s)                               : 2 channels
Channel layout                           : L R
`
	r := ExtractMediaInfo(mi)
	assertEquals(t, "AudioCodec", "AV3A", r.AudioCodec)
}

func TestExtractMediaInfo_DTSVariants(t *testing.T) {
	tests := []struct {
		format string
		codec  string
	}{
		{"DTS XLL", "DTS-HD MA"},
		{"DTS LBR", "DTS-HD HR"},
		{"DTS ES XLL", "DTS-ES"},
		{"DTS-UHD", "DTS"},
		{"DTS XLL X", "DTS"},
		{"DTS", "DTS"},
	}
	for _, tt := range tests {
		codec, _ := audioFromMI(tt.format, "")
		if codec != tt.codec {
			t.Errorf("audioFromMI(%q) = %q, want %q", tt.format, codec, tt.codec)
		}
	}
}

func TestExtractMediaInfo_HDRCombos(t *testing.T) {
	tests := []struct {
		hdr  string
		want string
	}{
		{"HDR10", "HDR10"},
		{"HDR10+", "HDR10+"},
		{"SMPTE ST 2094 App 4, Version HDR10+ Profile A", "HDR10+"},
		{"Dolby Vision, Version 1.0, dvhe.05.06", "DoVi"},
		{"Dolby Vision, dvhe.07.06, HDR10 compatible", "DoVi HDR"},
		{"HDR Vivid, Version 1", "HDR Vivid"},
		{"HLG", "HLG"},
		{"", ""},
	}
	for _, tt := range tests {
		got := hdrFromMI(tt.hdr)
		if got != tt.want {
			t.Errorf("hdrFromMI(%q) = %q, want %q", tt.hdr, got, tt.want)
		}
	}
}

func TestExtractMediaInfo_WidthResolution(t *testing.T) {
	tests := []struct {
		width string
		want  string
	}{
		{"7680 pixels", "4320p"},
		{"3 840 pixels", "2160p"},
		{"3 824 pixels", "2160p"},
		{"1 920 pixels", "1080p"},
		{"1 440 pixels", "1440p"},
		{"1 280 pixels", "720p"},
		{"720 pixels", "480p"},
		{"", ""},
	}
	for _, tt := range tests {
		got := resolutionFromWidth(tt.width)
		if got != tt.want {
			t.Errorf("resolutionFromWidth(%q) = %q, want %q", tt.width, got, tt.want)
		}
	}
}

func TestExtractMediaInfo_ChannelsFallback(t *testing.T) {
	tests := []struct {
		layout   string
		channels string
		want     string
	}{
		{"L R", "", "2.0"},
		{"L R C LFE Ls Rs", "", "5.1"},
		{"", "6 channels", "5.1"},
		{"", "2 channels", "2.0"},
		{"", "8 channels", "7.1"},
	}
	for _, tt := range tests {
		got := channelsFromMI(tt.layout, tt.channels)
		if got != tt.want {
			t.Errorf("channelsFromMI(%q, %q) = %q, want %q", tt.layout, tt.channels, got, tt.want)
		}
	}
}

func TestTechProfileFromTitle(t *testing.T) {
	tc := ParseTitle("The Matrix 1999 1080p BluRay x264 DTS-FRDS")
	p := TechProfileFromTitle(tc)
	assertEquals(t, "MainTitle", "The Matrix", p.MainTitle)
	assertEquals(t, "Year", "1999", p.Year)
	assertEquals(t, "Resolution", "1080p", p.Resolution)
	assertEquals(t, "VideoCodec", "x264", p.VideoCodec)
	assertEquals(t, "AudioCodec", "DTS", p.AudioCodec)
	assertEquals(t, "ReleaseGroup", "FRDS", p.ReleaseGroup)
}

func TestMergeTechProfile_MediaInfoOverrides(t *testing.T) {
	tc := ParseTitle("Test 2024 1080p WEB-DL x264 AAC")
	mi := &MediaInfoTech{
		Resolution:    "2160p",
		VideoCodec:    "HEVC",
		AudioCodec:    "DDP",
		AudioChannels: "5.1",
		HDR:           "DoVi",
		BitDepth:      "10bit",
		AudioTracks:   2,
	}
	p := MergeTechProfile(tc, mi)
	assertEquals(t, "Resolution", "2160p", p.Resolution)
	assertEquals(t, "VideoCodec", "HEVC", p.VideoCodec)
	assertEquals(t, "AudioCodec", "DDP", p.AudioCodec)
	assertEquals(t, "AudioChannels", "5.1", p.AudioChannels)
	assertEquals(t, "HDR", "DoVi", p.HDR)
	assertEquals(t, "BitDepth", "10bit", p.BitDepth)
	assertEqualsInt(t, "AudioTracks", 2, p.AudioTracks)
	assertEquals(t, "MainTitle", "Test", p.MainTitle)
	assertEquals(t, "Year", "2024", p.Year)
}

func TestMergeTechProfile_NilMediaInfo(t *testing.T) {
	tc := ParseTitle("Test 2024 1080p x264")
	p := MergeTechProfile(tc, nil)
	assertEquals(t, "Resolution", "1080p", p.Resolution)
	assertEquals(t, "VideoCodec", "x264", p.VideoCodec)
	assertEquals(t, "AudioChannels", "", p.AudioChannels)
	assertEqualsInt(t, "AudioTracks", 0, p.AudioTracks)
}

func TestMergeTechProfile_PartialMediaInfo(t *testing.T) {
	tc := ParseTitle("Test 2024 1080p BluRay x264 DTS-FRDS")
	mi := &MediaInfoTech{
		Resolution: "2160p",
	}
	p := MergeTechProfile(tc, mi)
	assertEquals(t, "Resolution", "2160p", p.Resolution)
	assertEquals(t, "VideoCodec", "x264", p.VideoCodec)
	assertEquals(t, "AudioCodec", "DTS", p.AudioCodec)
}

func TestMergeDOMInto_MediumOverride(t *testing.T) {
	p := ParseTitleTech("Movie 2024 1080p Blu-ray x264 DTS")
	assertEquals(t, "SourceType(标题)", "Blu-ray", p.SourceType)
	// DOM 覆盖媒介（DOM > 标题）
	MergeDOMInto(&p, "WEB-DL", "", "", "")
	assertEquals(t, "SourceType(DOM)", "", p.SourceType)
	assertEquals(t, "Specification(DOM)", "WEB-DL", p.Specification)
	assertEquals(t, "Medium(DOM)", "WEB-DL", p.Medium)
}

func TestMergeDOMInto_TechFallback(t *testing.T) {
	p := TechProfile{} // 空 TechProfile
	// DOM 技术参数只在为空时填充
	MergeDOMInto(&p, "", "1080p", "x264", "AAC")
	assertEquals(t, "Resolution(fallback)", "1080p", p.Resolution)
	assertEquals(t, "VideoCodec(fallback)", "x264", p.VideoCodec)
	assertEquals(t, "AudioCodec(fallback)", "AAC", p.AudioCodec)

	// 已有值时 DOM 不覆盖
	MergeDOMInto(&p, "", "2160p", "HEVC", "DTS")
	assertEquals(t, "Resolution(不覆盖)", "1080p", p.Resolution)
	assertEquals(t, "VideoCodec(不覆盖)", "x264", p.VideoCodec)
}

func TestMergeDOMInto_NilSafe(t *testing.T) {
	var nilP *TechProfile
	MergeDOMInto(nilP, "WEB-DL", "1080p", "x264", "AAC") // 不 panic
}

func assertEquals(t *testing.T, field, want string, got string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", field, got, want)
	}
}

func assertEqualsInt(t *testing.T, field string, want, got int) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %d, want %d", field, got, want)
	}
}
