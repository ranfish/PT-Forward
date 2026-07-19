package publish

import "testing"

// ===== 海豚站真实数据测试（dicmusic.com 采集 2026-07-19）=====

func TestDicmusicReleaseTypes_Count(t *testing.T) {
	if len(DicmusicReleaseTypes) != 15 {
		t.Errorf("expected 15 dicmusic release types, got %d", len(DicmusicReleaseTypes))
	}
}

func TestDicmusicReleaseTypes_Chinese(t *testing.T) {
	if DicmusicReleaseTypes[1] != "专辑" {
		t.Errorf("ID 1 should be 专辑, got %q", DicmusicReleaseTypes[1])
	}
	if DicmusicReleaseTypes[19] != "DJ混音" {
		t.Errorf("ID 19 should be DJ混音, got %q", DicmusicReleaseTypes[19])
	}
}

func TestDicmusicReleaseTypeByName(t *testing.T) {
	if DicmusicReleaseTypeByName["专辑"] != 1 {
		t.Error("专辑 should be ID 1")
	}
	if DicmusicReleaseTypeByName["Album"] != 1 {
		t.Error("Album alias should also be ID 1")
	}
}

func TestDicmusicFormats_HasDSD(t *testing.T) {
	found := false
	for _, f := range DicmusicFormats {
		if f == "DSD" {
			found = true
		}
	}
	if !found {
		t.Error("dicmusic should have DSD format (源码无)")
	}
}

func TestDicmusicMedia_HasBluRay(t *testing.T) {
	found := false
	for _, m := range DicmusicMedia {
		if m == "Blu-ray" {
			found = true
		}
	}
	if !found {
		t.Error("dicmusic uses Blu-ray (源码用 BD)")
	}
}

func TestDicmusicArtistTypes_Chinese(t *testing.T) {
	if DicmusicArtistTypes[1] != "主要" {
		t.Error("ID 1 should be 主要")
	}
	if DicmusicArtistTypes[6] != "DJ／编曲" {
		t.Error("ID 6 should be DJ／编曲（合并了源码 DJ+Arranger）")
	}
}

func TestDicmusicUploadFields_HasExtraFields(t *testing.T) {
	extras := []string{"subtitle", "genre_tags", "sample_rate", "jinzhuan", "diy", "buy"}
	for _, field := range extras {
		if _, ok := DicmusicUploadFields[field]; !ok {
			t.Errorf("dicmusic should have extra field %q", field)
		}
	}
}

// ===== Gazelle 框架默认值测试（源码，保留供海豹站）=====

func TestGazelleReleaseTypes_Count(t *testing.T) {
	if len(GazelleReleaseTypes) != 15 {
		t.Errorf("expected 15 gazelle release types, got %d", len(GazelleReleaseTypes))
	}
}

func TestGazelleReleaseTypes_Album(t *testing.T) {
	if GazelleReleaseTypes[1] != "Album" {
		t.Errorf("ID 1 should be Album, got %q", GazelleReleaseTypes[1])
	}
}

func TestGazelleFormats_Count(t *testing.T) {
	if len(GazelleFormats) != 6 {
		t.Errorf("expected 6 formats, got %d", len(GazelleFormats))
	}
}

func TestGazelleMedia_Count(t *testing.T) {
	if len(GazelleMedia) != 9 {
		t.Errorf("expected 9 media types, got %d", len(GazelleMedia))
	}
}

func TestGazelleArtistTypes(t *testing.T) {
	if GazelleArtistTypes[1] != "Main" {
		t.Error("ID 1 should be Main")
	}
}

func TestGazelleUploadFields(t *testing.T) {
	if GazelleUploadFields["release_type"] != "releasetype" {
		t.Error("release_type mapping mismatch")
	}
}
