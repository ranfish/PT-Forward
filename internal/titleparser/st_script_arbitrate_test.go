package titleparser

import "testing"

// TestArbitrateSTScript §59.349: ST 词形与媒介铁证冲突仲裁四组合
// （+根门）——v1.05 原盘/压制以连字符区分，判定覆盖提取
// （与 §59.166 B2 MI 纠错同模式）。
func TestArbitrateSTScript(t *testing.T) {
	// 实录锚：Zhizn I Sudba 案标题（x265 压制 + 站方连字符写法）
	const zhiznTitle = "Zhizn I Sudba S01 2012 1080p Blu-ray x265 10bit AC3￡cXcY@FRDS"
	// 构造锚：驼峰写法压制标题（组合 3——一致不动）
	const camelEncTitle = "Some.Movie.2012.1080p.BluRay.x265-GROUP"
	// 构造锚：驼峰写法 + BDInfo 报告（组合 2——原盘铁证升格）
	const camelDiscTitle = "Some.Movie.2012.1080p.BluRay.AVC-GROUP"
	const bdinfoMI = "DISC INFO:\n\nDisc Title: X\nQUICK SUMMARY:\nVideo: AVC\n"

	cases := []struct {
		name  string
		title string
		mi    string
		want  string
	}{
		// 组合 1：压制铁证（x265）∧ 连字符族 → 降格驼峰
		{"encode x hyphen -> camel (Zhizn case)", zhiznTitle, "", "BluRay"},
		{"encode x hyphen UHD -> camel UHD", "Movie 2160p UHD Blu-ray x265-GROUP", "", "UHD BluRay"},
		// 组合 2：原盘铁证（QUICK SUMMARY）∧ 驼峰族 → 升格连字符（对称防御）
		{"disc x camel -> hyphen", camelDiscTitle, bdinfoMI, "Blu-ray"},
		// 组合 3：压制 ∧ 驼峰 → 一致不动
		{"encode x camel unchanged", camelEncTitle, "", "BluRay"},
		// 组合 4：原盘 ∧ 连字符 → 一致不动（无压制标记+QUICK SUMMARY）
		{"disc x hyphen unchanged", "Movie 2012 1080p Blu-ray AVC-GROUP", bdinfoMI, "Blu-ray"},
		// 根门：无铁证 → 不干预（连字符保持——证据缺席忠实提取）
		{"no evidence x hyphen unchanged", "Movie 2012 1080p Blu-ray AVC-GROUP", "", "Blu-ray"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := BuildTechProfile(c.title, c.mi, "", "", "", "")
			if p.SourceType != c.want {
				t.Errorf("ST = %q, want %q", p.SourceType, c.want)
			}
		})
	}
}
