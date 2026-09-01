package publish

import (
	"strings"
	"testing"
)


// §59.163: MI 误判防御——段名冒号全角化（探针 H1 实证；用户定案内容零丢失）。
func TestFullwidthMIColons(t *testing.T) {
	src := `感谢 FRDS 组
Source #1: GER UHD Koch Media (2019): Video, Audio, Subtitles.
Video: AUS has hardcoded subtitles.
Audio: All 7.1 TrueHD Atmos sources are very similar.
Subtitles: en - Compared AUS, GER, Amazon.
Chapters: GBR - only named source available.`
	out := FullwidthMIColons(src)
	// 段名冒号全角化
	for _, fw := range []string{"Video： AUS", "Audio： All 7.1", "Subtitles： en", "Chapters： GBR"} {
		if !strings.Contains(out, fw) {
			t.Errorf("段名冒号应全角化: %q", fw)
		}
	}
	// 半角形态清除（站方检测通过条件）
	if strings.Contains(out, "Video:") || strings.Contains(out, "Audio:") ||
		strings.Contains(out, "Subtitles:") || strings.Contains(out, "Chapters:") {
		t.Error("MI 段名半角冒号应全部替换")
	}
	// 内容零丢失（行数/其余文本不变）
	if strings.Count(out, "\n") != strings.Count(src, "\n") {
		t.Error("内容行数不应变化（零丢失）")
	}
	if !strings.Contains(out, "Source #1: GER") || !strings.Contains(out, "感谢 FRDS 组") {
		t.Error("无形态文本应原样保留")
	}
	// MI 老段名 General 也覆盖
	if got := FullwidthMIColons("General : x"); got != "General： x" {
		t.Errorf("General 空格变体应替换, got %q", got)
	}
}
