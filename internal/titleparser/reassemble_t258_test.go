package titleparser

import (
	"strings"
	"testing"
)

func Test258_HSBS_Kept(t *testing.T) {
	p := ParseTitleTech("Pirates.Of.The.Caribbean.On.Stranger.Tides.2011.3D.BluRay.1080p.HSBS.x264.DTS-HD.MA7.1.2Audios-CMCT")
	rt := ReassembleFromTechProfile(p, V105TitleFormat())
	if !strings.Contains(rt, "HSBS") {
		t.Errorf("HSBS 应保留: %q", rt)
	}
	if strings.Contains(rt, "DTS-HD.MA ") {
		t.Errorf("空格模式应 DTS-HD MA（无点连）: %q", rt)
	}
	t.Logf("OUT: %s", rt)
}

func Test258_SpaceModeAudioCanonical(t *testing.T) {
	p := ParseTitleTech("Movie.2020.BluRay.1080p.x264.DTS-HD.MA.5.1-GROUP")
	rt := ReassembleFromTechProfile(p, V105TitleFormat())
	if !strings.Contains(rt, "DTS-HD MA ") {
		t.Errorf("audio 应为 canonical DTS-HD MA: %q", rt)
	}
	t.Logf("OUT: %s", rt)
}

// §59.273: platform 槽与媒介槽同值去重（CCTV6 湾区晚会 "HDTV HDTV" 案）
func TestReassemble_PlatformMediumDedup(t *testing.T) {
	p := TechProfile{
		MainTitle: "CCTV6 The Greater Bay Area Film Concert 2026", Year: "2026",
		Resolution: "1080p", SourcePlatform: "HDTV", SourceType: "HDTV",
		VideoCodec: "H264", AudioCodec: "AAC",
	}
	tf := TitleFormat{Separator: " ", Order: []string{"title", "year", "resolution", "platform", "medium", "video_codec", "audio_full", "group"}}
	got := ReassembleFromTechProfile(p, tf)
	if strings.Contains(got, "HDTV HDTV") {
		t.Errorf("HDTV 重复未去: %q", got)
	}
	if !strings.Contains(got, "1080p HDTV H264") {
		t.Errorf("媒介链应保留单次 HDTV: %q", got)
	}
	// WEB-DL platform（NF/AMZN）不误伤
	p2 := TechProfile{MainTitle: "Show", Year: "2026", Resolution: "1080p",
		SourcePlatform: "NF", SourceType: "WEB", Specification: "WEB-DL", VideoCodec: "H264"}
	got2 := ReassembleFromTechProfile(p2, tf)
	if !strings.Contains(got2, "NF") {
		t.Errorf("NF platform 被误删: %q", got2)
	}
}
