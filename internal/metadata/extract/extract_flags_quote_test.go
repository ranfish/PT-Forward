package extract

import (
	"strings"
	"testing"
)

// §59.39: 上游声明（quote 引用块）不误标禁转——Fantasm 案例锚定。
// 发布者引用的上游声明（约束上游站点）非本种子禁转标记；发布者自身声明在 quote 外。
func TestExtractFlagsQuoteExclusion(t *testing.T) {
	p := PublicExtractor{}

	fantasmDescr := `[quote]
制作说明
美版原盘@AdBlue,感谢分享者,字幕由本人原创自译
仅在PD22测试,未经允许禁止转载和二次提取素材使用,谢谢[/quote]

[quote][b][color=blue][size=5]FRDS官组作品，转载请注明出处[/size][/color][/b][/quote]
感谢观看`

	cases := []struct {
		name, title, subtitle, descr string
		want                         bool // 是否应含禁转类 flag
	}{
		// Fantasm 案例：禁转词全在 quote 内（上游声明）→ 不标
		{"上游声明_不标", "Fantasm 1976 BluRay 1080p x265 FLAC MNHD-FRDS", "【幻想】情色系列",
			fantasmDescr, false},
		// 发布者自身声明（quote 外）含禁转词 → 标
		{"自身声明_标", "Movie 2024 BluRay x264-GROUP", "",
			"本资源禁止转载，谢谢合作", true},
		// quote 内含禁转词 + quote 外也有 → 标（quote 外为准）
		{"引用_加_自身_标", "Movie 2024", "",
			"[quote]上游禁止转载[/quote] 本资源也严禁转载", true},
		// quote 外无禁转词 → 不标
		{"仅引用_不标", "Movie 2024", "",
			"[quote]上游谢绝转载[/quote] 感谢观看", false},
		// 站点标记层不排除：[禁转] 形态即使在 quote 内也标（权威源语义）
		{"站点标记_在quote内_仍标", "Movie 2024", "",
			"[quote]资源说明[/quote][禁转]", true},
		// 站点标记在 quote 外 → 标
		{"站点标记_quote外_标", "Movie 2024 【禁转】", "副标题", "简介", true},
		// 嵌套 quote：内层禁转词丢弃，外层外的保留
		{"嵌套quote_内层丢弃", "Movie 2024", "",
			"[quote]外层 [quote]内层禁止转载[/quote] 继续[/quote] 正文正常", false},
		// 不成对标签：保守保留（宁可多扫）
		{"不成对quote_保守", "Movie 2024", "",
			"[quote]上游声明禁止转载", true},
		// 无 quote 的正常禁转
		{"无quote_正常标", "Movie 2024", "禁转资源", "简介", true},
	}

	forbiddenSet := map[string]bool{
		"禁转": true, "禁止转载": true, "谢绝转载": true, "严禁转载": true,
		"谢绝搬运": true, "独占": true, "限时禁转": true, "限转": true,
	}
	for _, c := range cases {
		flags := p.extractFlags(c.title, c.subtitle, "", c.descr)
		hit := false
		for _, f := range flags {
			if forbiddenSet[f] {
				hit = true
				break
			}
		}
		if hit != c.want {
			t.Errorf("%s: got hit=%v flags=%v, want %v", c.name, hit, flags, c.want)
		}
	}
}

// §59.39: stripQuoteBlocks 单元锚定
func TestStripQuoteBlocks(t *testing.T) {
	cases := []struct{ in, want string }{
		{"无标签原文", "无标签原文"},
		{"[quote]块内[/quote]块外", " 块外"},
		{"[quote=作者]带属性块[/quote]外", " 外"},
		{"[quote]a[quote]b[/quote]c[/quote]d", " d"},
		{"前[quote]未闭合", "前未闭合"}, // 不成对：剥标签留文本（保守）
	}
	for _, c := range cases {
		if got := stripQuoteBlocks(c.in); strings.TrimSpace(got) != strings.TrimSpace(c.want) {
			t.Errorf("stripQuoteBlocks(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// §59.40: 站方标签禁转检测——tags 并入两层
func TestExtractFlagsTags(t *testing.T) {
	p := PublicExtractor{}
	forbiddenSet := map[string]bool{
		"禁转": true, "禁止转载": true, "谢绝转载": true, "严禁转载": true,
		"谢绝搬运": true, "独占": true, "限时禁转": true, "限转": true,
	}
	cases := []struct {
		name, title, subtitle, tags, descr string
		want                               bool
	}{
		// 标签禁转（§33 模式 1 主流形态）
		{"标签_禁转", "Movie 2024 BluRay x264-GROUP", "副标题", "禁转 首发 国语", "简介正文", true},
		{"标签_禁止转载", "Movie 2024", "", "禁止转载", "", true},
		{"标签_独占", "Movie 2024", "", "独占", "", true},
		// 标签形态的站点标记（cspt [禁转] checkbox 值形态）
		{"标签_标记形态", "Movie 2024", "", "[禁转]", "", true},
		// 正常标签不受影响
		{"标签_正常", "Movie 2024", "", "dolby_vision hdr10 chinese_subtitle", "简介", false},
		// 空 tags 回归
		{"无标签_副标题禁转回归", "Movie 2024", "禁转资源", "", "简介", true},
		// 标签禁转 + 简介里引用上游（两者独立，标签命中即可）
		{"标签命中_引用不干扰", "Movie 2024", "", "禁转", "[quote]上游禁止转载[/quote]正文", true},
		// 描述+标签组合：quote 剥离只作用于 descr，tags 原文参与
		{"仅引用_标签也无_不标", "Movie 2024", "", "国语", "[quote]上游禁止转载[/quote]", false},
	}
	for _, c := range cases {
		flags := p.extractFlags(c.title, c.subtitle, c.tags, c.descr)
		hit := false
		for _, f := range flags {
			if forbiddenSet[f] {
				hit = true
				break
			}
		}
		if hit != c.want {
			t.Errorf("%s: hit=%v flags=%v, want %v", c.name, hit, flags, c.want)
		}
	}
}
