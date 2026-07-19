package model

import "testing"

func TestParseTagConfig_Empty(t *testing.T) {
	cfg := ParseTagConfig("")
	if cfg.Mode != TagModeTaglist {
		t.Errorf("empty should default to taglist, got %q", cfg.Mode)
	}
}

func TestParseTagConfig_Valid(t *testing.T) {
	cfg := ParseTagConfig(`{"mode":"checkbox_id","tags":{"hdr10":"tag_hdr10"}}`)
	if cfg.Mode != TagModeCheckboxID {
		t.Errorf("mode mismatch: %q", cfg.Mode)
	}
	if cfg.Tags["hdr10"] != "tag_hdr10" {
		t.Errorf("tag mapping mismatch: %q", cfg.Tags["hdr10"])
	}
}

func TestParseTagConfig_InvalidJSON(t *testing.T) {
	cfg := ParseTagConfig("{invalid}")
	if cfg.Mode != TagModeTaglist {
		t.Errorf("invalid JSON should fallback to taglist, got %q", cfg.Mode)
	}
}

func TestParseTagConfig_MissingMode(t *testing.T) {
	cfg := ParseTagConfig(`{"tags":{}}`)
	if cfg.Mode != TagModeTaglist {
		t.Errorf("missing mode should default to taglist, got %q", cfg.Mode)
	}
}

func TestSiteTagConfig_Serialize(t *testing.T) {
	cfg := &SiteTagConfig{
		Mode: TagModeCheckboxID,
		Tags: map[string]string{"hdr10": "tag_hdr10"},
	}
	s := cfg.Serialize()
	if s == "" {
		t.Fatal("serialize returned empty")
	}
	parsed := ParseTagConfig(s)
	if parsed.Mode != TagModeCheckboxID || parsed.Tags["hdr10"] != "tag_hdr10" {
		t.Errorf("round-trip mismatch: %+v", parsed)
	}
}
