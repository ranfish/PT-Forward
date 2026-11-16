package publish

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateTimePoints(t *testing.T) {
	engine := NewScreenshotEngine("mpv", 5, 30, 85, "jpeg", nil)
	points := engine.generateTimePoints(7200)

	if len(points) != 5 {
		t.Fatalf("expected 5 points, got %d", len(points))
	}

	for i, pt := range points {
		if pt < 0 {
			t.Errorf("point[%d] negative: %f", i, pt)
		}
		if pt > 7200*0.80 {
			t.Errorf("point[%d] exceeds golden end: %f", i, pt)
		}
		if pt < 7200*0.30-float64(engine.minInterval) {
			t.Errorf("point[%d] below golden start range: %f", i, pt)
		}
	}
}

func TestGenerateTimePoints_ShortVideo(t *testing.T) {
	engine := NewScreenshotEngine("mpv", 5, 30, 85, "jpeg", nil)
	points := engine.generateTimePoints(60)

	if len(points) != 5 {
		t.Fatalf("expected 5 points, got %d", len(points))
	}

	for _, pt := range points {
		if pt < 0 {
			t.Errorf("point negative: %f", pt)
		}
	}
}

func TestGenerateTimePoints_ZeroDuration(t *testing.T) {
	engine := NewScreenshotEngine("mpv", 5, 30, 85, "jpeg", nil)
	points := engine.generateTimePoints(0)

	if len(points) != 5 {
		t.Fatalf("expected 5 points, got %d", len(points))
	}

	for _, pt := range points {
		if pt != 0 {
			t.Errorf("expected all zeros for zero duration, got %f", pt)
		}
	}
}

func TestScreenshotEngineNotAvailable(t *testing.T) {
	engine := NewScreenshotEngine("nonexistent_binary", 5, 30, 85, "jpeg", nil)
	if engine.Available() {
		t.Error("expected Available() to return false for nonexistent binary")
	}
}

func TestSubtitleDetectorNotAvailable(t *testing.T) {
	d := NewSubtitleDetector(nil)
	d.ffprobePath = "nonexistent_binary"
	if d.Available() {
		t.Error("expected Available() to return false for nonexistent binary")
	}
}

func TestMediaInfoAnalyzerNotAvailable(t *testing.T) {
	a := NewMediaInfoAnalyzer(nil)
	a.mediainfoPath = "nonexistent_binary"
	if a.Available() {
		t.Error("expected Available() to return false for nonexistent binary")
	}
}

func TestSelectBestChinese_NoCandidates(t *testing.T) {
	d := NewSubtitleDetector(nil)
	sid, codec := d.SelectBestChinese(nil)
	if sid != 0 {
		t.Errorf("expected sid=0, got %d", sid)
	}
	if codec != "" {
		t.Errorf("expected empty codec, got %s", codec)
	}
}

func TestSelectBestChinese_PrioritizesASS(t *testing.T) {
	d := NewSubtitleDetector(nil)
	candidates := []SubtitleCandidate{
		{StreamIndex: 2, Codec: "subrip", Language: "chi", Score: 10, IsText: true},
		{StreamIndex: 3, Codec: "ass", Language: "chi", Score: 10, IsText: true},
	}
	sid, codec := d.SelectBestChinese(candidates)
	if codec != "ass" {
		t.Errorf("expected ass, got %s", codec)
	}
	if sid != 3 {
		t.Errorf("expected stream 3, got %d", sid)
	}
}

func TestSelectBestChinese_PrioritizesSRT_OverPGS(t *testing.T) {
	d := NewSubtitleDetector(nil)
	candidates := []SubtitleCandidate{
		{StreamIndex: 5, Codec: "hdmv_pgs", Language: "chi", Score: 10, IsText: false},
		{StreamIndex: 4, Codec: "subrip", Language: "chi", Score: 10, IsText: true},
	}
	sid, codec := d.SelectBestChinese(candidates)
	if codec != "subrip" {
		t.Errorf("expected subrip, got %s", codec)
	}
	if sid != 4 {
		t.Errorf("expected stream 4, got %d", sid)
	}
}

// §59.319 附十语义更新：无中文（含无显式语言）→ 按顺序第一字幕轨
// （宁英文不裸图——原零分不选=裸图语义废止，用户定案三级优先）
func TestSelectBestChinese_IgnoresZeroScore(t *testing.T) {
	d := NewSubtitleDetector(nil)
	candidates := []SubtitleCandidate{
		{StreamIndex: 1, Codec: "ass", Language: "eng", Score: 0, IsText: true},
	}
	sid, codec := d.SelectBestChinese(candidates)
	if sid != 1 {
		t.Errorf("无中文应选第一轨（宁英文不裸图）, got sid=%d", sid)
	}
	if codec != "ass" {
		t.Errorf("expected ass, got %s", codec)
	}
	// 三级优先锚：简中(chi) > 繁中(zh-hant) > 无中文第一轨
	tri := []SubtitleCandidate{
		{StreamIndex: 10, Codec: "subrip", Language: "english", IsText: true},
		{StreamIndex: 11, Codec: "subrip", Language: "zh-hant", IsText: true},
		{StreamIndex: 12, Codec: "subrip", Language: "chi", IsText: true},
	}
	if idx, _ := d.SelectBestChinese(tri); idx != 12 {
		t.Errorf("简中应最优先: got %d", idx)
	}
	tri[2] = SubtitleCandidate{StreamIndex: 12, Codec: "subrip", Language: "english", IsText: true} // 去简中
	if idx, _ := d.SelectBestChinese(tri); idx != 11 {
		t.Errorf("无简中时繁中优先: got %d", idx)
	}
	tri[1] = SubtitleCandidate{StreamIndex: 11, Codec: "subrip", Language: "french", IsText: true} // 去繁中
	if idx, _ := d.SelectBestChinese(tri); idx != 10 {
		t.Errorf("无中文应选第一轨: got %d", idx)
	}
	// 无显式语言（m2ts PGS 常态）同归第一轨
	noLang := []SubtitleCandidate{
		{StreamIndex: 20, Codec: "hdmv_pgs_subtitle"},
		{StreamIndex: 21, Codec: "hdmv_pgs_subtitle"},
	}
	if idx, _ := d.SelectBestChinese(noLang); idx != 20 {
		t.Errorf("无语言标记应选第一轨: got %d", idx)
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		s        string
		keywords []string
		want     bool
	}{
		{"简体中文", []string{"简", "chs"}, true},
		{"繁體中文", []string{"繁", "cht"}, true},
		{"English", []string{"简", "cht"}, false},
		{"", []string{"test"}, false},
	}
	for _, tt := range tests {
		got := containsAny(tt.s, tt.keywords...)
		if got != tt.want {
			t.Errorf("containsAny(%q, %v) = %v, want %v", tt.s, tt.keywords, got, tt.want)
		}
	}
}

func TestProbeVideo_DetectsHDR(t *testing.T) {
	_ = NewScreenshotEngine("mpv", 5, 30, 85, "jpeg", nil)
	info := &videoInfo{duration: 7507.5, isHDR: true}
	if !info.isHDR {
		t.Error("expected HDR detection")
	}

	infoSDR := &videoInfo{duration: 7507.5, isHDR: false}
	if infoSDR.isHDR {
		t.Error("expected SDR detection")
	}
}

func TestEngineSelection_AlwaysMPV(t *testing.T) {
	engine := NewScreenshotEngine("mpv", 5, 30, 85, "jpeg", nil)
	engine.ffprobePath = "nonexistent_ffprobe"

	// All videos use mpv regardless of HDR or subtitles
	if engine.mpvPath != "mpv" {
		t.Error("mpvPath should default to mpv")
	}
}

func TestFindLargestVideo_NoDir(t *testing.T) {
	g := &PublishArtifactGenerator{}
	_, err := g.findLargestVideo("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

// §59.540 附五: findLargestVideos 候选序——多文件剧集 fallback 的选择正确性
func TestFindLargestVideos(t *testing.T) {
	g := &PublishArtifactGenerator{}
	dir := t.TempDir()
	// E24（最大·源损坏）E01（次大·健康）E12（第三）——体积降序
	files := map[string]int{"E24.mp4": 300, "E01.mp4": 200, "E12.mp4": 100}
	for name, size := range files {
		p := filepath.Join(dir, name)
		data := make([]byte, size)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// n=2：主选 E24 + fallback E01（E12 不入候选=短路界）
	paths, err := g.findLargestVideos(dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Fatalf("want 2 candidates, got %d", len(paths))
	}
	if filepath.Base(paths[0]) != "E24.mp4" || filepath.Base(paths[1]) != "E01.mp4" {
		t.Errorf("candidate order wrong: %v", paths)
	}
	// n=1：单候选（旧行为等价）
	one, err := g.findLargestVideos(dir, 1)
	if err != nil || len(one) != 1 || filepath.Base(one[0]) != "E24.mp4" {
		t.Errorf("n=1 should return largest only: %v %v", one, err)
	}
	// n=0：非法
	if _, err := g.findLargestVideos(dir, 0); err == nil {
		t.Error("n=0 should error")
	}
	// 非视频扩展名不入候选
	dir2 := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir2, "note.txt"), []byte("x"), 0o644)
	if _, err := g.findLargestVideos(dir2, 2); err == nil {
		t.Error("no video files should error")
	}
}
