package description

import (
	"strings"
	"testing"
)

// §59.542 追记十二锚：消费时合成五态——水印从「入库资产」变「消费时合成」。
func TestComposeStatement(t *testing.T) {
	title := "Hatfields & McCoys S01 2012 1080p Blu-ray x265 10bit DTSHD￡cXcY@FRDS"
	siteDecl := "[quote][b][color=#0000ff]SUP中英编译声明[/color][/b][/quote]"

	t.Run("官组+无致谢=站方声明+致谢", func(t *testing.T) {
		got := ComposeStatement(siteDecl, "朋友", title)
		if !strings.Contains(got, "SUP中英编译声明") {
			t.Error("站方原始声明须保留")
		}
		if !strings.Contains(got, "FRDS官组作品，感谢原制作者发布。") {
			t.Errorf("致谢缺失: %q", got)
		}
		if !strings.Contains(got, "互珍互重，禁转PTT") {
			t.Error("第二句致谢缺失")
		}
	})

	t.Run("官组+空=纯致谢", func(t *testing.T) {
		got := ComposeStatement("", "朋友", title)
		if !strings.Contains(got, "FRDS官组作品") || strings.Contains(got, "\n\n\n") {
			t.Errorf("空 statement 应得纯致谢无多余空行: %q", got)
		}
	})

	t.Run("官组+已有致谢=原样（backfill 行兼容）", func(t *testing.T) {
		existing := "[quote][b][color=blue][size=5]FRDS官组作品，感谢原制作者发布。[/size][/color][/b][/quote]\n[quote][b][color=red][size=5]互珍互重，禁转PTT[/size][/color][/b][/quote]"
		if got := ComposeStatement(existing, "朋友", title); got != existing {
			t.Errorf("已有致谢应原样返回（不二次追加）: %q", got)
		}
	})

	t.Run("非官组=原样", func(t *testing.T) {
		if got := ComposeStatement(siteDecl, "朋友", "Some Movie 2020 1080p BluRay x264-Other"); got != siteDecl {
			t.Errorf("非官组不应追加: %q", got)
		}
	})

	t.Run("无组名+空=原样空", func(t *testing.T) {
		if got := ComposeStatement("", "朋友", "Plain Title 2020"); got != "" {
			t.Errorf("无组名空 statement 应保持空: %q", got)
		}
	})

	t.Run("形态与历史 append 一致（Hatfields 现场复刻）", func(t *testing.T) {
		// 243 现场：站方声明（#0000ff 形态）+ title 带 £cXcY@FRDS
		// 合成后 = 声明 \n\n 致谢两句（blue+red）
		got := ComposeStatement(siteDecl, "朋友", title)
		idxDecl := strings.Index(got, "SUP中英")
		idxThanks := strings.Index(got, "[color=blue]")
		if idxDecl == -1 || idxThanks == -1 || idxDecl > idxThanks {
			t.Errorf("声明应在致谢之前: %q", got)
		}
	})
}
