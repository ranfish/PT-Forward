package titleparser

import (
	"strings"
	"testing"
)

// §59.307 地区码重组保留——TechProfileFromTitle 曾漏拷 RegionCode 致
// ParseTitleTech/BuildTechProfile 链重组丢失（CHN/GBR 原盘 Remux 应出现在
// resolution 后槽位，v1.05 槽序+媒介门双重满足）
func Test307_RegionCodeReassembled(t *testing.T) {
	cases := []struct{ title, region string }{
		{"[天若有情].A.Moment.of.Romance.1990.CHN.UHD.BluRay.Remux.2160p.HEVC.DoVi.HDR10.DTS-HD.MA.5.1.2Audios-CMCT", "CHN"},
		{"[中南海保镖].The.Bodyguard.from.Beijing.1994.GBR.UHD.BluRay.Remux.2160p.HEVC.DoVi.HDR10.FLAC.2Audios-CMCT", "GBR"},
	}
	for _, c := range cases {
		tp := BuildTechProfile(c.title, "", "", "", "", "")
		if tp.RegionCode != c.region {
			t.Errorf("Region=%q want %q", tp.RegionCode, c.region)
		}
		rt := ReassembleFromTechProfile(tp, V105TitleFormat())
		if !strings.Contains(rt, " "+c.region+" ") {
			t.Errorf("重组缺地区码 %q: %q", c.region, rt)
		}
		// 槽位断言：region 在 resolution 后、媒介前
		ri := strings.Index(rt, c.region)
		ri2 := strings.Index(rt, "2160p")
		if ri < ri2 {
			t.Errorf("region 应在 resolution 后: %q", rt)
		}
	}
}
