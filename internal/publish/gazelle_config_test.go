package publish

import "testing"

func TestGazelleReleaseTypes_Count(t *testing.T) {
	if len(GazelleReleaseTypes) != 15 {
		t.Errorf("expected 15 release types, got %d", len(GazelleReleaseTypes))
	}
}

func TestGazelleReleaseTypes_Album(t *testing.T) {
	if GazelleReleaseTypes[1] != "Album" {
		t.Errorf("ID 1 should be Album, got %q", GazelleReleaseTypes[1])
	}
	if GazelleReleaseTypes[9] != "Single" {
		t.Errorf("ID 9 should be Single, got %q", GazelleReleaseTypes[9])
	}
}

func TestGazelleReleaseTypeByName(t *testing.T) {
	if GazelleReleaseTypeByName["Album"] != 1 {
		t.Error("Album should be ID 1")
	}
	if GazelleReleaseTypeByName["EP"] != 5 {
		t.Error("EP should be ID 5")
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

func TestGazelleEncodings_Count(t *testing.T) {
	if len(GazelleEncodings) != 16 {
		t.Errorf("expected 16 encodings, got %d", len(GazelleEncodings))
	}
}

func TestGazelleArtistTypes(t *testing.T) {
	if GazelleArtistTypes[1] != "Main" {
		t.Error("ID 1 should be Main")
	}
	if GazelleArtistTypeByName["Composer"] != 4 {
		t.Error("Composer should be ID 4")
	}
}

func TestGazelleUploadFields(t *testing.T) {
	if GazelleUploadFields["release_type"] != "releasetype" {
		t.Error("release_type mapping mismatch")
	}
	if GazelleUploadFields["encoding"] != "bitrate" {
		t.Error("encoding should map to bitrate")
	}
	if GazelleUploadFields["record_label"] != "remaster_record_label" {
		t.Error("record_label mapping mismatch")
	}
}
