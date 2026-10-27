package titleparser

import "testing"

// TestSeasonEpisode4Digit §59.357: 4 位集数（千集长剧）提取——柯南
// S01E0263-S01E0303 案（E\d{1,3} 上限 miss 致 SeasonEpisode 空）。
func TestSeasonEpisode4Digit(t *testing.T) {
	cases := []struct{ title, want string }{
		{"Meitantei.Conan.1996.S01E0263-S01E0303.NF.WEB-DL.1080p.H264.DDP.2Audios-CMCTV", "S01E0263-S01E0303"},
		{"Show.S01E05-S01E08.2020", "S01E05-S01E08"},
		{"Show.S01E263-E303", "S01E263-E303"},
		{"Movie.1996.1080p", ""},
	}
	for _, c := range cases {
		got := reSeasonEpisode.FindString(c.title)
		if got != c.want {
			t.Errorf("reSeasonEpisode(%q) = %q, want %q", c.title, got, c.want)
		}
	}
	// 重组端到端（season 槽入 name）
	p := BuildTechProfile("Conan.1996.S01E0263-S01E0303.NF.WEB-DL.1080p.H264", "", "", "", "", "")
	if p.SeasonEpisode != "S01E0263-S01E0303" {
		t.Errorf("SeasonEpisode = %q, want S01E0263-S01E0303", p.SeasonEpisode)
	}
}
