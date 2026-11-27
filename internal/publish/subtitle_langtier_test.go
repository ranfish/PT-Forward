package publish

import (
	"strings"
	"testing"
)

// §59.590 八级选轨锚（2026-11-28 用户定案·字典序两级）：
// 体系主键（简/含简→繁/含繁→英/含英→兜底）×形态次键（双字→单语）
// = 简双字(7)>纯简(6)>繁双字(5)>纯繁(4)>英双字(3)>纯英(2)>兜底(0)。
//
// 全部样本源自 243 摸底实证（10674 MI/88659 Text 轨聚合——形态频次见
// docs/31 §59.590）+ 理论形态补全（CHS&CHT/ENG&FRE/KOR&ENG）。
//
// 阴性对照（§59.590）：删 langTier 双字分支 → 双字锚 FAIL；词表回退
// （去 hant/去裸英）→ 对应锚 FAIL；判定序反转（简先）→ [Chinese][CHT] 锚 FAIL。
func TestLangTier_EightLevels(t *testing.T) {
	cases := []struct {
		name  string
		lang  string
		title string
		want  int
	}{
		// tier 7 简双字（摸底 ~3550+400 主力形态）
		{"简双字 CHS&ENG", "chinese", "CHS&ENG", 7},
		{"简双字小写 chs&eng", "chinese", "chs&eng", 7},
		{"简双字后缀变体", "chinese", "CHS&ENG (双语特效)", 7},
		{"简双字中+非英 CHS&FRE", "chinese", "CHS&FRE", 7},
		{"简双字中+非英 CHS&JPN", "chinese", "CHS&JPN", 7},
		{"简双字两字简写 简英", "chi", "简英", 7},             // 裸「英」安全域
		{"简双字简体中英SUP", "chi", "简体中英SUP", 7},        // 摸底形态
		{"简双字位置式 lang 简", "chi", "中上英下SUP", 7},     // 摸底 700 形态
		{"简双字位置式 lang 不分默认简", "", "中上英下SUP", 7},    // lang 空→默认简
		{"简双字中英基底 lang=eng 怪形态", "eng", "中英字幕", 7}, // 发现2：中文基底优先
		{"简双字 CHS&CHT 理论（title 双信号归简）", "chinese", "CHS&CHT", 7},
		// tier 6 纯简
		{"纯简 CHS", "chinese", "CHS", 6},
		{"纯简 Simplified Chinese", "chinese (simplified)", "Simplified Chinese", 6},
		{"纯简 lang chi 无 title", "chi", "", 6},
		{"纯简 中文（简体）", "chinese-hans", "中文（简体）", 6},
		// tier 5 繁双字
		{"繁双字 CHT&ENG", "chinese", "CHT&ENG", 5},
		{"繁双字位置式 lang 繁", "zh-hant", "英上中下SUP", 5},
		{"繁双字位置式 lang Chinese-Hant", "chinese-hant-tw", "中上英下SUP", 5},
		{"繁双字两字简写 繁英", "zh-hant", "繁英", 5},
		// tier 4 纯繁
		{"纯繁 CHT", "chinese", "CHT", 4}, // 摸底 3831——title 繁词优先于 lang 简泛化
		{"纯繁 lang zh-hant", "zh-hant", "", 4},
		{"纯繁 lang hant 补丁（Chinese-Hant-TW 摸底 1071）", "chinese-hant-tw", "", 4},
		{"纯繁中文（繁體）", "chinese", "中文（繁體, 台灣）", 4},
		// tier 3 英双字（八级新增）
		{"英双字 ENG&FRE", "", "ENG&FRE", 3},
		{"英双字 KOR&ENG", "kor", "KOR&ENG", 3},
		// tier 2 纯英
		{"纯英 lang en", "en", "", 2},
		{"纯英 lang eng", "eng", "", 2},
		{"纯英 lang en-US", "en-us", "", 2},
		{"纯英 title English（窄表防自吞噬）", "", "English", 2},
		{"纯英 title 英文（中文组对偶词）", "", "英文", 2},
		{"纯英 lang English 摸底主力", "english", "", 2},
		// tier 0 兜底
		{"兜底纯 kor（Scandal 无中文场景）", "kor", "", 0},
		{"兜底 FRE&GER（无体系双语）", "", "FRE&GER", 0},
		{"兜底 Commentary 双保险", "eng", "Commentary by co-writer & director Josh Safdie", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := langTier(tc.lang, strings.ToLower(tc.title))
			if got != tc.want {
				t.Errorf("langTier(%q,%q)=%d want %d", tc.lang, tc.title, got, tc.want)
			}
		})
	}
}

// §59.590 次序链锚：tier → secondLang（双字内含英>非英）→ codec → 保序。
func TestSelectBestSubtitle_OrderChain(t *testing.T) {
	d := &SubtitleDetector{}

	// ① secondLang 次序：同 tier 简双字——CHS&ENG（subrip）压 CHS&FRE（ass）
	// （第二语言可读性 > 渲染形态差异——次序层级定案）
	c1 := []SubtitleCandidate{
		{StreamIndex: 2, SubIndex: 1, Codec: "subrip", Language: "chinese", Title: "CHS&FRE", IsText: true},
		{StreamIndex: 3, SubIndex: 2, Codec: "ass", Language: "chinese", Title: "CHS&ENG", IsText: true},
	}
	idx, _ := d.SelectBestSubtitle(c1)
	if idx != 3 {
		t.Errorf("次序链：CHS&ENG 应压 CHS&FRE（第二语言含英次序）, got idx=%d", idx)
	}

	// ② tier 压次序：纯简（6）压繁双字（5）——体系优先于形态（用户定案）
	c2 := []SubtitleCandidate{
		{StreamIndex: 2, SubIndex: 1, Codec: "subrip", Language: "zh-hant", Title: "CHT&ENG", IsText: true},
		{StreamIndex: 3, SubIndex: 2, Codec: "subrip", Language: "chi", Title: "", IsText: true},
	}
	idx2, _ := d.SelectBestSubtitle(c2)
	if idx2 != 3 {
		t.Errorf("体系优先：纯简应压繁双字, got idx=%d", idx2)
	}

	// ③ 简双字压纯简（形态次序）
	c3 := []SubtitleCandidate{
		{StreamIndex: 2, SubIndex: 1, Codec: "subrip", Language: "chi", Title: "", IsText: true},
		{StreamIndex: 3, SubIndex: 2, Codec: "subrip", Language: "chi", Title: "CHS&FRE", IsText: true},
	}
	idx3, _ := d.SelectBestSubtitle(c3)
	if idx3 != 3 {
		t.Errorf("形态次序：简双字应压纯简, got idx=%d", idx3)
	}

	// ④ codec 次序不回归：同 tier 纯简——ass 压 subrip
	c4 := []SubtitleCandidate{
		{StreamIndex: 2, SubIndex: 1, Codec: "subrip", Language: "chi", Title: "", IsText: true},
		{StreamIndex: 3, SubIndex: 2, Codec: "ass", Language: "chi", Title: "", IsText: true},
	}
	idx4, _ := d.SelectBestSubtitle(c4)
	if idx4 != 3 {
		t.Errorf("codec 次序：ass 应压 subrip, got idx=%d", idx4)
	}
}
