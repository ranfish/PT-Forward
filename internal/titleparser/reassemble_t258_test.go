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
