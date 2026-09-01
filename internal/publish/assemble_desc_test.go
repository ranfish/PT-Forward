package publish

import (
	"strings"
	"testing"
)


// §59.163: MI 误判防御过滤（幸运探针实证——子串非行首触发）。
func TestStripMILookalikeLines(t *testing.T) {
	src := `感谢 FRDS 组
Source #1: GER UHD Koch Media (2019): Video, Audio, Subtitles.
Notes:
Video: AUS has hardcoded subtitles.
Audio: All 7.1 TrueHD Atmos sources are very similar.
Subtitles: en - Compared AUS, GER, Amazon.
Chapters: GBR - only named source available.
Comparisons:
Video: https://slow.pics/c/n3QBQqdA
正常声明行保留。`
	out := StripMILookalikeLines(src)
	for _, gone := range []string{"Video: AUS", "Audio: All 7.1", "Subtitles: en", "Chapters: GBR", "Video: https"} {
		if strings.Contains(out, gone) {
			t.Errorf("MI 形态行应被剥: %q", gone)
		}
	}
	for _, keep := range []string{"感谢 FRDS 组", "Source #1: GER", "正常声明行保留。"} {
		if !strings.Contains(out, keep) {
			t.Errorf("正常行应保留: %q", keep)
		}
	}
	// 全部为 MI 形态行 → 空串（quote 整体省略）
	if StripMILookalikeLines("Video: a\nAudio: b") != "" {
		t.Error("全 MI 形态应返回空")
	}
}
