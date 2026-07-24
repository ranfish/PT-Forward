package titleparser

import (
	"testing"
)

func TestReverseLookup(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"resolution.r1080p", "1080p"},
		{"video.x264", "x264"},
		{"audio.dts_hd_ma", "DTS-HD MA"},
		{"hdr.dv", "DoVi"},
		{"medium.bluray", "Blu-ray"},
		{"nonexistent", ""},
	}
	for _, tt := range tests {
		got := ReverseLookup(tt.key)
		if got != tt.want {
			t.Errorf("ReverseLookup(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}
