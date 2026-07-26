package compliance

import (
	"testing"
)

func TestDetectAdult(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		subtitle string
		wantHit  bool
	}{
		// --- True positives: XXX keyword ---
		{"XXX standalone", "StudioX XXX FakeActress 2160p MP4-WRB", "", true},
		{"XXX in subtitle", "StudioX", "25 11 21 FakeActress XXX 2160p MP4-WRB", true},

		// --- False positives: XXX keyword ---
		{"xXx 2002 movie", "xXx 2002 1080p BluRay x264-GROUP", "", false},
		{"xXx 2005 movie", "xXx State of the Union 2005 1080p", "", false},
		{"xXx Return of Xander Cage", "xXx Return of Xander Cage 2017 2160p", "", false},
		{"XXXTentacion music (no word boundary)", "XXXTentacion - Skins 2018 FLAC", "", false},
		{"xxx inside word (no boundary)", "fooxxxbar sample", "", false},

		// --- True positives: JAV code (no video/music markers in remainder) ---
		{"JAV code ABP-100", "ABP-100 某某标题", "", true},
		{"JAV code SSIS-500", "SSIS-500.Some.Title", "", true},
		{"JAV code lowercase", "ipx-123.something", "", true},
		{"JAV code 4-letter STAR", "STAR-001 某标题", "", true},
		{"JAV code alone", "ABP-100", "", true},

		// --- False positives: JAV-like code + video markers in remainder ---
		{"JAV-like code + S01E02", "AAEJ-123 Some Show S01E02 1080p", "", false},
		{"JAV-like code + 1080p", "IPX-123 Big Movie 1080p BluRay", "", false},
		{"JAV-like code + x264", "IPX-123 Big Movie 1080p x264", "", false},

		// --- False positives: RIAJ music code + music markers ---
		{"RIAJ music code ABCD-1234", "ABCD-1234 Some Album FLAC", "", false},
		{"FLAC spec in title", "Some.Album.FLAC-2496", "", false},

		// --- True positives: adult date pattern ---
		{"adult date 6-digit 010124_001", "010124_001 Some title", "", true},
		{"adult date 8-digit 20240101_001", "20240101_001 Some title", "", true},
		{"adult date in subtitle", "StudioX", "20240101_001 Actress", true},

		// --- True positives: bracket date ---
		{"bracket date [2023.08.01]", "[2023.08.01] Some Title", "", true},
		{"bracket date [2024.01.01]", "[2024.01.01] Another Title", "", true},

		// --- Normal PT titles (no false positive) ---
		{"normal movie", "壮志凌云.Top.Gun.Maverick.2022.2160p.UHD.BluRay.x265-CHD", "", false},
		{"normal TV", "权力的游戏.Game.of.Thrones.S08.COMPLETE.1080p.BluRay.x264-ROVERS", "", false},
		{"normal anime", "鬼灭之刃.Kimetsu.no.Yaiba.2024.1080p.WEB-DL.x264-ANON", "", false},
		{"normal music", "周杰伦.-.最伟大的作品.2022.FLAC.24bit.96kHz-MTEAM", "", false},
		{"normal with AC3", "Movie.Name.2024.1080p.BluRay.x264.AC3-FGT", "", false},
		{"normal with DTS", "Movie.Name.2024.2160p.UHD.BluRay.DTS-HD.MA.5.1-PTER", "", false},

		// --- Edge cases ---
		{"empty strings", "", "", false},
		{"just title no markers", "Some Random Title", "", false},
		{"real release group -CHD", "Movie.2024.1080p-CHD", "", false},
		{"real release group -FRDS", "Movie.2024.2160p.UHD.BluRay.x265.HDR.ATMOS.TrueHD.7.1-FRDS", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hit, reason := DetectAdult(tt.title, tt.subtitle)
			if hit != tt.wantHit {
				t.Errorf("DetectAdult(%q, %q) = (%v, %q), want hit=%v",
					tt.title, tt.subtitle, hit, reason, tt.wantHit)
			}
		})
	}
}

func TestRIAJDetection(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"ABCD-1234 Some Album", "cd"},       // 3rd char 'C' = cd
		{"MHCL-5001 Album", "cd"},            // 3rd char 'C' = cd
		{"ABGD-001 Music", "sacd"},           // 3rd char 'G' = sacd
		{"ABXD-001 Blu-ray", "bluray"},       // 3rd char 'X' = bluray
		{"No Code Here", ""},
		{"BBC-123 Too short", ""},            // 3 letters, reRIAJ needs 4
	}
	for _, tt := range tests {
		got := detectRIAJMediaType(tt.title)
		if got != tt.want {
			t.Errorf("detectRIAJMediaType(%q) = %q, want %q", tt.title, got, tt.want)
		}
	}
}

func TestBenignXXX(t *testing.T) {
	tests := []struct {
		title    string
		subtitle string
		want     bool
	}{
		{"xxx 2002", "", true},
		{"xxx return of xander cage", "", true},
		{"xxx adult content", "", false},
		{"", "xxx 2002", true},
		{"normal movie", "", false},
	}
	for _, tt := range tests {
		got := isBenignXXX(tt.title, tt.subtitle)
		if got != tt.want {
			t.Errorf("isBenignXXX(%q, %q) = %v, want %v", tt.title, tt.subtitle, got, tt.want)
		}
	}
}
