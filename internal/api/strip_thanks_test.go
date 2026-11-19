package api

import (
	"strings"
	"testing"
)

// §59.260: 声明追加幂等剥离通用化——任意组名（FRDS 硬编码标记盲区修复）
func TestStripAppendedThanks_AnyGroup(t *testing.T) {
	tail := "[quote][b][color=red][size=5]互珍互重，禁转PTT[/size][/color][/b][/quote]"
	cmctBlock := "[quote][b][color=blue][size=5]CMCT官组作品，感谢原制作者发布。[/size][/color][/b][/quote]\n" + tail
	frdBlock := "[quote][b][color=blue][size=5]FRDS官组作品，感谢原制作者发布。[/size][/color][/b][/quote]\n" + tail
	engBlock := "[quote][b][color=blue][size=5]Group UBits release.\nAll thanks to the original uploader![/size][/color][/b][/quote]\n" + tail

	// 基础：源站声明 + 追加块 → 剥离后只剩源站声明
	base := "源站原始声明内容"
	for name, block := range map[string]string{"CMCT": cmctBlock, "FRDS": frdBlock, "英文组": engBlock} {
		got := stripAppendedThanks(base + "\n\n" + block)
		if got != base {
			t.Errorf("[%s] 剥离应回源站原文: got=%q", name, got)
		}
	}

	// 累积两轮的旧数据（重获两次产生的双份）→ 全剥净
	doubled := base + "\n\n" + cmctBlock + "\n\n" + cmctBlock
	got := stripAppendedThanks(doubled)
	if got != base {
		t.Errorf("[累积两轮] 应全剥: got=%q", got)
	}

	// 无追加块原样
	plain := "只有源站声明"
	if got := stripAppendedThanks(plain); got != plain {
		t.Errorf("[无追加] 应原样: got=%q", got)
	}

	// 空 statement
	if got := stripAppendedThanks(""); got != "" {
		t.Errorf("[空] got=%q", got)
	}

	// 源站声明自身含"禁转PTT"字样（非 quote 结构）——不应误剥
	tricky := "源站说明：本资源禁转PTT已获授权" + "\n\n" + cmctBlock
	got = stripAppendedThanks(tricky)
	if !strings.HasPrefix(got, "源站说明") {
		t.Errorf("[含禁转字样的源站文] 不应误剥源站文: got=%q", got)
	}
}

// §59.542 刀2: 站方规范化形态宽容——Pope 案实拉形态（三重改写：
// blue→#0000ff / red→#ff0000 / [quote] 后插换行）·字面锚全失配致累积。
func TestStripAppendedThanks_SiteNormalized(t *testing.T) {
	// Pope 案 243 实拉形态（站方渲染后回流的原始字节）
	siteTail := "[quote]\n[b][color=#ff0000][size=5]互珍互重，禁转PTT[/size][/color][/b][/quote]"
	siteHead := "[quote]\n[b][color=#0000ff][size=5]FRDS torrent官组作品，感谢原制作者发布。[/size][/color][/b][/quote]"
	base := "源站原始声明内容"

	// ① 纯回流形态（295 颗存量形态）→ 全剥净回源站原文
	got := stripAppendedThanks(base + "\n\n" + siteHead + "\n\n" + siteTail)
	if got != base {
		t.Errorf("[回流形态] 应剥净: got=%q", got)
	}

	// ② 原生+回流双份（13 颗累积形态：旧对站方改写剥不掉+新对 append）→ 全剥净
	nativeTail := "[quote][b][color=red][size=5]互珍互重，禁转PTT[/size][/color][/b][/quote]"
	nativeHead := "[quote][b][color=blue][size=5]FRDS torrent官组作品，感谢原制作者发布。[/size][/color][/b][/quote]"
	doubled := base + "\n\n" + siteHead + "\n\n" + siteTail + "\n\n" + nativeHead + "\n\n" + nativeTail
	if got = stripAppendedThanks(doubled); got != base {
		t.Errorf("[双份累积] 应全剥: got=%q", got)
	}

	// ③ 孤立回流头块（尾块缺失/被超预期改写）→ 清扫剥净
	lone := base + "\n\n" + siteHead
	if got = stripAppendedThanks(lone); got != base {
		t.Errorf("[孤立头块] 应清扫: got=%q", got)
	}

	// ④ 源站真声明（红色 quote 但内容不同）不误伤
	realDecl := "[quote]\n[b][color=#ff0000][size=5]本站资源仅供学习交流[/size][/color][/b][/quote]"
	if got = stripAppendedThanks(realDecl + "\n\n" + siteHead + "\n\n" + siteTail); got != realDecl {
		t.Errorf("[源站真声明] 不应误伤: got=%q", got)
	}
}
