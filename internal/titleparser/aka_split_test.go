package titleparser

import "testing"

// §59.290: AKA 双名切分——摸底六环境实测形态
func TestSplitAKATitle(t *testing.T) {
	cases := []struct{ in, want string }{
		// 欧陆片：原名前 英文名后 → 取后段（常见词命中）
		{"Chi l'ha vista morire? AKA Who Saw Her Die?", "Who Saw Her Die?"},
		{"Jagten AKA The Hunt", "The Hunt"},
		{"Mononoke-hime AKA Princess Mononoke", "Princess Mononoke"},
		{"Ifigeneia AKA Iphigenia", "Ifigeneia"}, // 双零分 → 前段正名
		{"Muži v naději AKA Men in Hope", "Men in Hope"},
		{"L'Étranger AKA The Stranger", "The Stranger"},
		{"La teta y la luna AKA The Tit and the Moon", "The Tit and the Moon"},
		// 华语片：英文名前 拼音后（全小写强扣）→ 取前段
		{"The.Piano.in.a.Factory.AKA.Gang.de.qin", "The.Piano.in.a.Factory"},
		{"Doctor.Mack.AKA.Lau.man.yi.sang", "Doctor.Mack"},
		{"The.Fantasy.of.Deer.Warrior.AKA.Da.xia.mei.hua.lu", "The.Fantasy.of.Deer.Warrior"},
		{"Tunnel.Warfare.AKA.Di.dao.zhan", "Tunnel.Warfare"},
		{"The.Tiger.AKA.Der.Tiger", "The.Tiger"}, // 双语名双零分 → 前段
		// 双零分代号（印度片昵称）→ 前段正名
		{"Amaran aka SK21", "Amaran"},
		{"Andhra King Taluka aka RaPo 22 (2025)", "Andhra King Taluka"},
		// 无 AKA 原样
		{"Inception", "Inception"},
		{"Akira", "Akira"}, // 含 aka 子串非独立词——不误伤
	}
	for _, tc := range cases {
		if got := SplitAKATitle(tc.in); got != tc.want {
			t.Errorf("in=%q\n got=%q\nwant=%q", tc.in, got, tc.want)
		}
	}
}

// 端到端：完整标题解析后主标题切分（年份/组名不受影响）
func TestParseTitle_AKAEndToEnd(t *testing.T) {
	c := ParseTitle("Chi l'ha vista morire? AKA Who Saw Her Die? 1972 1080p Blu-ray AVC LPCM 1.0-Debaucherous")
	if c.MainTitle != "Who Saw Her Die?" {
		t.Errorf("MainTitle=%q", c.MainTitle)
	}
	if c.Year != "1972" {
		t.Errorf("Year=%q", c.Year)
	}
	if c.ReleaseGroup != "Debaucherous" {
		t.Errorf("Group=%q", c.ReleaseGroup)
	}
	c2 := ParseTitle("Andhra King Taluka aka RaPo 22 (2025) 1080P NF WEB-DL H264 DDP5.1 5Audio-SHB931@UBWEB")
	if c2.MainTitle != "Andhra King Taluka" {
		t.Errorf("2 MainTitle=%q", c2.MainTitle)
	}
	if c2.Year != "2025" {
		t.Errorf("2 Year=%q", c2.Year) // 年份在括注内——年份提取须仍命中
	}
}
