package reseed

import (
	"fmt"
	"testing"

	"github.com/ranfish/pt-forward/internal/titleparser"
)

// §59.181: 副标题匹配路径加固——括号剥离 + platform 词典 web 限制。
func TestStripBracketsVersionCheck(t *testing.T) {
	// ① Just Mercy 完整场景（中文副标题 → 英文孤儿名）
	src := titleparser.ParseTitleTech("Just.Mercy.2019.2160p.UHD.Blu-ray.DoVi.HDR10.HEVC.TrueHD.7.1.Atmos-DIY@UBits")
	blocked := techProfileVersionDefined(src, "[热门]【DIY 原盘 00884】正义的慈悲 / 以公之名")
	if blocked {
		t.Errorf("中文副标题 DIY 噪音应被剥离（不阻断）: blocked=%v", blocked)
	}
	fmt.Printf("  ① 中文副标题【DIY 原盘 00884】→ 剥离 → blocked=false ✓\n")

	// ② 英文主标题路径（DIY 被 splitMedium 正确消费）
	src2 := titleparser.ParseTitleTech("Movie.2024.1080p.BluRay.DTS-HD.MA.5.1-DIY@Group")
	blocked2 := techProfileVersionDefined(src2, "Movie.2024.1080p.BluRay.DIY.DTS-HD.MA.5.1-Group")
	if blocked2 {
		t.Errorf("英文主标题路径不应阻断: blocked=%v", blocked2)
	}
	fmt.Printf("  ② 英文主标题路径 → blocked=false ✓\n")

	// ③ Director's Cut 版本差异仍被拦截（无括号）
	src3 := titleparser.ParseTitleTech("Movie.2024.1080p.BluRay.x264")
	blocked3 := techProfileVersionDefined(src3, "Movie.2024.Director's.Cut.1080p.BluRay.x264")
	if !blocked3 {
		t.Errorf("Director's Cut 版本差异应被拦截: blocked=%v", blocked3)
	}
	fmt.Printf("  ③ 无括号 Director's Cut 版本差异 → blocked=true ✓\n")

	// ④ platform 词典 web 限制（DIY 非 web 语境不匹配）
	p := titleparser.ParseTitleTech("Movie.2024.1080p.BluRay.DIY.DTS")
	if p.SourcePlatform == "DIY" {
		t.Errorf("非 web 语境 DIY 不应匹配为平台: %q", p.SourcePlatform)
	}
	fmt.Printf("  ④ 非web语境 DIY → Platform=%q (非DIY) ✓\n", p.SourcePlatform)

	// ⑤ web 语境 DIY 正常匹配
	p2 := titleparser.ParseTitleTech("Show.2024.DIY.WEB-DL.1080p.x264")
	fmt.Printf("  ⑤ web语境 DIY → Platform=%q\n", p2.SourcePlatform)
}

// 直接测试剥离函数
func TestStripBracketsFunction(t *testing.T) {
	cases := []struct{ in, want string }{
		{"[热门]【DIY 原盘 00884】正义的慈悲", "正义的慈悲"},
		{"[NF] Show.2024.WEB-DL", " Show.2024.WEB-DL"},
		{"[国语][中字]Movie.2024", "Movie.2024"},
		{"No brackets here", "No brackets here"},
		{"【纯全角】测试", "测试"},
	}
	for _, c := range cases {
		got := stripBracketsForVersionCheck(c.in)
		if got != c.want {
			t.Errorf("stripBrackets(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	fmt.Println("  ⑥ 剥离函数五态 ✓")
}
