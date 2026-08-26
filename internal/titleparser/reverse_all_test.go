package titleparser

import "testing"

func TestReverseLookupAllTags(t *testing.T) {
	for _, k := range []string{"tag.japanese_audio", "tag.chinese_audio", "tag.english_audio"} {
		if v := ReverseLookup(k); v == "" || v == k {
			t.Errorf("ReverseLookup(%q) = %q", k, v)
		} else {
			t.Log(k, "->", v)
		}
	}
}
