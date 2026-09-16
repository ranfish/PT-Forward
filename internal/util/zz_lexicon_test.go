package util

import "testing"

// §59.233: ExtractGroupName v2 资产扫描层
func TestExtractGroupNameLexicon(t *testing.T) {
	SetGroupLexicon([]string{"CMCT", "CMCTV", "UBits", "FRDS", "UBWEB", "WiKi"})
	defer SetGroupLexicon(nil)

	cases := []struct {
		name, want string
	}{
		// 第一层资产扫描：dash 被剥除链吃掉的裸词（Ilo.Ilo 案）
		{"Ilo.Ilo.2013.TWN.BluRay.1080p.x264.DDP.5.1-CMCT", "CMCT"},
		// 尾部约束：括号尾缀允许
		{"Ilo.Ilo.2013.TWN.BluRay.1080p.x264.DDP.5.1-CMCT  (已审)", "CMCT"},
		// @ 形态跳过扫描——既有 @ 后段规则（识别层语义）
		{"Movie.2023.1080p-SHB931@UBits", "UBits"},
		// 尾部约束：命中词右侧还有内容 → 扫描排除 → 锚兜底
		{"Movie.CMCT.2023.1080p-RealGroup", "RealGroup"},
		// 未收录组：锚兜底
		{"Movie.2023.1080p.x264-NEWGROUPX", "NEWGROUPX"},
		// 扩展名预处理
		{"Show.2023.1080p-CMCTV.mp4", "CMCTV"},
	}
	for _, c := range cases {
		if got := ExtractGroupName(c.name); got != c.want {
			t.Errorf("ExtractGroupName(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

// §59.233: 尾部约束防误命中
func TestLexiconTailConstraint(t *testing.T) {
	SetGroupLexicon([]string{"WiKi"})
	defer SetGroupLexicon(nil)
	// WiKi 在非尾部（标题词）→ 扫描排除 → 锚提取真组名
	if got := ExtractGroupName("The.WiKi.Movie.2020.1080p-GROUP"); got != "GROUP" {
		t.Errorf("非尾部命中应排除: got %q", got)
	}
	// WiKi 在尾部 → 扫描命中
	if got := ExtractGroupName("Movie.2020.1080p-WiKi"); got != "WiKi" {
		t.Errorf("尾部命中应提取: got %q", got)
	}
}
