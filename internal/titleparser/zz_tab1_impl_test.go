package titleparser

import (
	"testing"
)

// §59.226 附八: DIY 组名族——媒介正则收窄+伪复合剥离
func TestDIYGroupExtraction(t *testing.T) {
	cases := []struct{ title, want string }{
		{"Batman.Mystery.of.the.Batwoman.2003.USA.Blu-ray.1080p.AVC.DTS-HD.MA.5.1-DIY@CMCT", "CMCT"},
		{"宾虚.Ben-Hur.1959.2160p.UHD.Blu-ray.DoVi.HDR10.HEVC.Atmos.TrueHD.7.1-DIY@UBits", "UBits"},
		{"Wanted.2008.Blu-Ray.1080p.AVC.DTS-HD.5.1-DIY-cqkyjj2", "cqkyjj2"},
		{"Movie.2023.1080p.BluRay.x264-SHB931@UBWEB", "SHB931@UBWEB"}, // 真复合整段
	}
	for _, c := range cases {
		if got := ParseTitle(c.title).ReleaseGroup; got != c.want {
			t.Errorf("group(%q) = %q, want %q", c.title, got, c.want)
		}
	}
}

// §59.226 附六: ST 反哺——平台词当源形态
func TestPlatformBackfeedST(t *testing.T) {
	// AMZN 无 requires 标记（NF requires=web 需 WEB 词激活——反哺受益者
	// 是无 requires 平台词）
	p := ParseTitleTech("Movie.2023.1080p.AMZN.DDP5.1.H.264-GROUP")
	if p.SourceType != "WEB" {
		t.Errorf("ST = %q, want WEB（platform=AMZN 反哺）", p.SourceType)
	}
	if p.SourcePlatform != "AMZN" {
		t.Errorf("platform = %q, want AMZN", p.SourcePlatform)
	}
}

// §59.226 附二十一: 帧率 MI 提取+非默认判定链
func TestFrameRateFromMI(t *testing.T) {
	cases := []struct{ in, want string }{
		{"23.976 FPS", "23.976"},
		{"60.000 FPS", "60.000"},
		{"24.000 FPS", "24.000"},
		{"", ""},
		{"29.97", "29.97"},
	}
	for _, c := range cases {
		if got := frameRateFromMI(c.in); got != c.want {
			t.Errorf("frameRateFromMI(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// §59.226 附四: MediumCanonicalOf 纯函数
func TestMediumCanonicalOf(t *testing.T) {
	cases := []struct {
		p    TechProfile
		want string
	}{
		{TechProfile{Specification: "Remux", Resolution: "1080p"}, "Remux"},
		{TechProfile{Specification: "Remux", Resolution: "2160p"}, "UHD Remux"},
		{TechProfile{Specification: "WEB-DL"}, "WEB-DL"},
		{TechProfile{Specification: "WEBRip"}, "Encode"}, // §59.166 From S04
		{TechProfile{Specification: "BDRip"}, "Encode"},
		{TechProfile{SourceType: "Blu-ray"}, "Blu-ray 原盘"},
		{TechProfile{SourceType: "BluRay"}, "Encode"},
		{TechProfile{SourceType: "DVD"}, "DVD"},
	}
	for _, c := range cases {
		if got := MediumCanonicalOf(c.p); got != c.want {
			t.Errorf("MediumCanonicalOf(%+v) = %q, want %q", c.p, got, c.want)
		}
	}
}

// §59.226 附三: 中文集数解析
func TestCNEpisode(t *testing.T) {
	tc := ParseTitle("大江大河之岁月如歌.第01-24集.2024.1080p.WEB-DL.H264-GROUP")
	if tc.SeasonEpisode != "E01-E24" {
		t.Errorf("SeasonEpisode = %q, want E01-E24", tc.SeasonEpisode)
	}
	tc2 := ParseTitle("BBC王朝全集.全五集.2018.720p-GROUP")
	if tc2.SeasonEpisode == "" {
		t.Errorf("全五集 应解析出 E01-E05")
	}
}
