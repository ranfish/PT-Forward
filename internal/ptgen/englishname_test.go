package ptgen

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.254 项2: 英文段判别/罗马数字归一/用户规则（首段英文段→拉丁兜底）/端点感知
func TestIsEnglishSegment(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"Northeastern Bro Ⅲ", true},      // 罗马数字归一（东北恋歌案）
		{"The Golden Age", true},           // 纯英文
		{"L'Âge d'or", false},              // 重音=原语种（法语法）
		{"C'è ancora domani", false},       // 重音=意语
		{"东北恋歌：冬天里的一把火", false},        // CJK
		{"Hawaii Five-0 Season 1", true},   // 数字合法
		{"There’s Still Tomorrow", true},   // 弯引号（财神案）
		{"The\u00a0Butcher's\u00a0Blade", true}, // NBSP（财神 aka 污染）
		{"Александр Невский", false},       // 西里尔
		{"", false},
	}
	for _, c := range cases {
		if got := IsEnglishSegment(c.in); got != c.want {
			t.Errorf("IsEnglishSegment(%q)=%v want %v", c.in, got, c.want)
		}
	}
}

func TestExtractEnglishFromSegments(t *testing.T) {
	// 豆影 aka 原名保序形态
	got := ExtractEnglishFromSegments([]string{"Александр Невский", "俄军抗德记", "Alexander Nevsky"})
	if got != "Alexander Nevsky" {
		t.Errorf("首段英文段: %q", got)
	}
	// 无英文段→首段拉丁（意语兜底）
	got = ExtractEnglishFromSegments([]string{"C'è ancora domani", "明天还有梦(港)"})
	if got != "C'è ancora domani" {
		t.Errorf("拉丁兜底: %q", got)
	}
	// 罗马数字归一输出
	got = ExtractEnglishFromSegments([]string{"东北恋歌", "Northeastern Bro Ⅲ", "Never Alone Again"})
	if got != "Northeastern Bro III" {
		t.Errorf("罗马数字归一: %q", got)
	}
	// 全 CJK 无果
	got = ExtractEnglishFromSegments([]string{"老郑飞到天上去了", "无事生非"})
	if got != "" {
		t.Errorf("全 CJK 应空: %q", got)
	}
}

func TestExtractEnglishName_EndpointAware(t *testing.T) {
	// 豆影形态（L'Âge 案：重音排除→The Golden Age）
	r := &model.PTGenResult{
		AKA: []string{"L'Âge d'or", "The Golden Age", "L'Age d'Or", "Age of Gold"},
	}
	if got := ExtractEnglishName(r, "https://doubaninfo.com/api/v1_douban.php"); got != "The Golden Age" {
		t.Errorf("豆影 aka: %q", got)
	}
	// 财神形态（foreign_title 英文优先——蓝霹雳案：aka 纯中文）
	r2 := &model.PTGenResult{
		ForeignTitle: "Blue Thunder",
		AKA:         []string{"蓝色的雷", "蓝色霹雳号"},
	}
	if got := ExtractEnglishName(r2, "https://cspt.top"); got != "Blue Thunder" {
		t.Errorf("财神 ft 优先: %q", got)
	}
	// 财神 foreign_title 中文（华语片）→aka 无英文→空（待 IMDb 通道）
	r3 := &model.PTGenResult{
		ForeignTitle: "九龍城寨·圍城",
		AKA:         []string{"Marvel队长2(港)", "惊奇联盟"},
	}
	if got := ExtractEnglishName(r3, "https://cspt.top"); got != "" {
		t.Errorf("财神华语片应空: %q", got)
	}
}

func TestNeedsIMDbFallback(t *testing.T) {
	if NeedsIMDbFallback("Movie.2024", "Some Name") {
		t.Error("常规（有英文名）不触发")
	}
	if !NeedsIMDbFallback("", "") {
		t.Error("空标题+无英文名应触发")
	}
	if !NeedsIMDbFallback("东北恋歌", "") {
		t.Error("纯 CJK 无英文名应触发")
	}
	if NeedsIMDbFallback("东北恋歌", "Northeastern Bro III") {
		t.Error("纯 CJK 但英文名已有（aka 命中）不触发")
	}
	if NeedsIMDbFallback("L'ultima volta", "") {
		t.Error("有拉丁段（意语原名=组风格保留）不触发")
	}
}

func TestSplitTitleSegments(t *testing.T) {
	segs := SplitTitleSegments("Hawaii Five-0 Season 1 / 天堂执法者 第一季 / 檀岛骑警 第一季")
	if len(segs) != 3 || segs[0] != "Hawaii Five-0 Season 1" {
		t.Errorf("斜杠拆分: %v", segs)
	}
}
