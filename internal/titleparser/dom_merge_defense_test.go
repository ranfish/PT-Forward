package titleparser

import "testing"

// §59.34 审计: MergeDOMInto 防御——DOM medium 解析不出有效片源/规格时
// 不覆盖标题派生值（UNK* key / "Encode" 等非媒介语义值）。
func TestMergeDOMIntoGarbageMediumDefense(t *testing.T) {
	cases := []struct {
		name        string
		title       string
		domMedium   string
		wantSrc     string
		wantSpec    string
	}{
		// 垃圾 DOM medium：不覆盖 title 值
		{"unk_key", "Movie 2160p UHD BluRay x265 GRP", "UNK0", "UHD BluRay", ""},
		{"encode_word", "Movie 1080p BluRay x264 GRP", "Encode", "BluRay", ""},
		// 有效 DOM medium：覆盖 title 值（DOM > title，完整覆盖含清除）
		{"webdl_key_normalized", "Movie 1080p BluRay x264 GRP", "WEB-DL", "", "WEB-DL"},
		{"remux", "Movie 1080p BluRay x264 GRP", "Remux", "", "Remux"},
		// 空 DOM medium：跳过
		{"empty", "Movie 2160p UHD BluRay x265 GRP", "", "UHD BluRay", ""},
	}
	for _, c := range cases {
		p := ParseTitleTech(c.title)
		MergeDOMInto(&p, c.domMedium, "", "", "")
		if p.SourceType != c.wantSrc || p.Specification != c.wantSpec {
			t.Errorf("%s: (%q dom=%q) got (%q,%q), want (%q,%q)",
				c.name, c.title, c.domMedium, p.SourceType, p.Specification, c.wantSrc, c.wantSpec)
		}
	}
}

// §59.34 审计: standard key 归一化链路——ReverseLookup(raw key) → 规范显示名
// → splitMedium 可解析。未映射 key 返回空。
func TestReverseLookupMediumChain(t *testing.T) {
	cases := []struct{ key, want string }{
		{"medium.webdl", "WEB-DL"},
		{"medium.uhd_bluray", "UHD Blu-ray"},
		{"medium.remux", "Remux"},
		{"resolution.r2160p", "2160p"},
		{"video.h265", "HEVC"},
		{"UNK0", ""},
		{"UNK11", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := ReverseLookup(c.key); got != c.want {
			t.Errorf("ReverseLookup(%q) = %q, want %q", c.key, got, c.want)
		}
	}
}
