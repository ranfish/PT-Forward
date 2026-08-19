package titleparser

import "testing"

// §59.41: Auro3D 对象信息补齐——v1.05 :193 值域另一半（Atmos 之外的 Auro3D）。
// 三处：标题提取 / 标签推断（dict tag.json auro_3d） / MI Commercial name 检测。
func TestAuro3DTitleExtraction(t *testing.T) {
	cases := []struct{ title, want string }{
		{"Movie 2024 BluRay 2160p TrueHD Auro3D 7.1-GROUP", "Auro3D"},
		{"Movie 2024 BluRay TrueHD Auro-3D 7.1-GROUP", "Auro3D"},
		{"Movie 2024 BluRay TrueHD Auro 3D 7.1-GROUP", "Auro3D"},
		{"Movie 2024 BluRay TrueHD Atmos 7.1-GROUP", "Atmos"},
		{"Movie 2024 BluRay DTS-HD MA 5.1-GROUP", ""},
		{"Movie 2024 BluRay DTS:X 7.1-GROUP", ""}, // DTS:X 编码即对象，不标（v1.05 :175）
	}
	for _, c := range cases {
		if got := extractAudioTechnologyFromTitle(c.title); got != c.want {
			t.Errorf("标题 %q → %q, want %q", c.title, got, c.want)
		}
	}
}

func TestAuro3DTagInfer(t *testing.T) {
	hits := TagInferMatches(TagInferInput{Title: "Movie Auro-3D 2024"})
	found := false
	for _, h := range hits {
		if h == "auro_3d" {
			found = true
		}
	}
	if !found {
		t.Errorf("Auro-3D 标题应推断 auro_3d 标签, got %v", hits)
	}
	// Atmos 标签回归
	hits2 := TagInferMatches(TagInferInput{Title: "Movie Atmos 2024"})
	hasAtmos := false
	for _, h := range hits2 {
		if h == "dolby_atmos" {
			hasAtmos = true
		}
	}
	if !hasAtmos {
		t.Error("Atmos 标题应推断 dolby_atmos（回归）")
	}
}

func TestAuro3DMI(t *testing.T) {
	cases := []struct{ format, comm, wantCodec, wantTech string }{
		{"MLP FBA 16-ch", "Dolby TrueHD with Dolby Atmos", "TrueHD", "Atmos"},
		{"MLP FBA 16-ch", "Auro-3D 13.1", "TrueHD", "Auro3D"},
		{"DTS XLL", "DTS-HD Master Audio", "DTS-HD MA", ""},
		{"E-AC-3 JOC", "Dolby Digital Plus with Dolby Atmos", "DDP", "Atmos"},
	}
	for _, c := range cases {
		codec, tech := audioFromMI(c.format, c.comm)
		if codec != c.wantCodec || tech != c.wantTech {
			t.Errorf("audioFromMI(%q, %q) = (%q, %q), want (%q, %q)", c.format, c.comm, codec, tech, c.wantCodec, c.wantTech)
		}
	}
}
