package orphan

import "testing"

func TestShouldSkip(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"movie.mkv", false},
		{".hidden", true},
		{".", false},
		{"..", false},
		{".DS_Store", true},
		{"Thumbs.db", true},
		{"desktop.ini", true},
		{"__pycache__", true},
		{"download.!qB", true},
		{"file.parts", true},
		{"temp.tmp", true},
		{"DATA.PARTS", true},
		{"FILE.TMP", true},
		{"normal.folder", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldSkip(tt.name); got != tt.want {
				t.Errorf("shouldSkip(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestStripChineseBracketPrefix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"[GROUP] Movie.Name", "Movie.Name"},
		{"【小组】电影名", "电影名"},
		{"Movie.Name", "Movie.Name"},
		{"", ""},
		{"[unclosed", "[unclosed"},
		{"[GROUP].Movie.Name", "Movie.Name"},
		{"【组】.Test", "Test"},
		{"[G] ..Name", "Name"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stripChineseBracketPrefix(tt.input)
			if got != tt.want {
				t.Errorf("stripChineseBracketPrefix(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsClaimedFuzzy(t *testing.T) {
	tests := []struct {
		name    string
		dirName string
		claimed map[string]bool
		want    bool
	}{
		{
			name:    "exact match",
			dirName: "movie.mkv",
			claimed: map[string]bool{"movie.mkv": true},
			want:    true,
		},
		{
			name:    "bracket prefix stripped matches",
			dirName: "[GROUP] movie",
			claimed: map[string]bool{"movie": true},
			want:    true,
		},
		{
			name:    "both have brackets same stripped",
			dirName: "[A] file",
			claimed: map[string]bool{"[B] file": true},
			want:    true,
		},
		{
			name:    "no match",
			dirName: "other",
			claimed: map[string]bool{"movie": true},
			want:    false,
		},
		{
			name:    "empty claimed",
			dirName: "movie",
			claimed: map[string]bool{},
			want:    false,
		},
		{
			name:    "chinese bracket prefix",
			dirName: "【组】电影",
			claimed: map[string]bool{"电影": true},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isClaimedFuzzy(tt.dirName, tt.claimed); got != tt.want {
				t.Errorf("isClaimedFuzzy(%q) = %v, want %v", tt.dirName, got, tt.want)
			}
		})
	}
}
