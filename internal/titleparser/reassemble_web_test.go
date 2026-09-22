package titleparser

import (
	"strings"
	"testing"
)

// §59.257: WEB 类片源类型省略——"WEB WEB-DL" 重复修复（243 大王饶命案）
func TestReassemble_WEBSourceTypeOmitted(t *testing.T) {
	cases := []struct{ title, wantST string }{
		{"Spare Me Great Lord S01 2021 1080p WEB-DL H.265 AAC-CSWEB", "1080p WEB-DL H.265"},
		{"Movie.2024.1080p.NF.WEB-DL.DDP.5.1-GROUP", "NF WEB-DL"},
		{"Show.2020.720p.WEBRip.AAC-GROUP", "WEBRip"},
	}
	for _, c := range cases {
		p := ParseTitleTech(c.title)
		rt := ReassembleFromTechProfile(p, V105TitleFormat())
		if !hasSub(rt, c.wantST) {
			t.Errorf("in=%q got=%q want-substr=%q", c.title, rt, c.wantST)
		}
		if hasSub(rt, "WEB WEB-DL") || hasSub(rt, "WEB WEBRip") {
			t.Errorf("in=%q got=%q WEB 重复", c.title, rt)
		}
	}
}

func hasSub(s, sub string) bool { return strings.Contains(s, sub) }
