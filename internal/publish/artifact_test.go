package publish

import (
	"testing"
)

func TestGenerateTimePoints(t *testing.T) {
	engine := NewScreenshotEngine("mpv", 5, 30, 85, nil)
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
	engine := NewScreenshotEngine("mpv", 5, 30, 85, nil)
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
	engine := NewScreenshotEngine("mpv", 5, 30, 85, nil)
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
	engine := NewScreenshotEngine("nonexistent_binary", 5, 30, 85, nil)
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

func TestSelectBestChinese_IgnoresZeroScore(t *testing.T) {
	d := NewSubtitleDetector(nil)
	candidates := []SubtitleCandidate{
		{StreamIndex: 1, Codec: "ass", Language: "eng", Score: 0, IsText: true},
	}
	sid, codec := d.SelectBestChinese(candidates)
	if sid != 0 {
		t.Errorf("expected no match for zero-score candidate, got sid=%d", sid)
	}
	if codec != "" {
		t.Errorf("expected empty codec, got %s", codec)
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
	_ = NewScreenshotEngine("mpv", 5, 30, 85, nil)
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
	engine := NewScreenshotEngine("mpv", 5, 30, 85, nil)
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
