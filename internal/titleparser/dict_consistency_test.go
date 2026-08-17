package titleparser

import (
	"strings"
	"testing"
)

// §59.35 一致性测试：lookupToken 优先级语义——有 StandardKey 词条的样本
// （canonical + variants）经 lookupToken 首个命中查询，结果必须 == 该词条 canonical。
// 同时验证词条顺序正确与 pattern 精确（DTS:X 不得被普通 DTS 抢、DD+ 不得被 DD 抢）。
// hdr 域例外：extractHDRFormat 是多 token 收集语义（DoVi HDR 样本收集 {Dolby Vision, HDR} 合法组合）。
func TestDictLookupPriority(t *testing.T) {
	for _, domain := range []string{"video_codec", "audio_codec"} {
		tokens := DictTokens(domain)
		for _, probe := range tokens {
			if probe.StandardKey == "" {
				continue // 归并辅助词条无独立语义
			}
			probes := append([]string{probe.Canonical}, probe.Variants...)
			for _, text := range probes {
				text = strings.TrimSpace(text)
				// 跳过点分隔 variants（"x.265"/"x.264" 是 DOM 显示名形态，
				// 标题 pattern `\bX265\b` 不命中——旧 registry 同行为，非回归）
				if text == "" || (strings.Contains(text, ".") && !strings.Contains(probe.Canonical, ".")) {
					continue
				}
				var registry []TokenDef
				if domain == "video_codec" {
					registry = videoCodecRegistry
				} else {
					registry = audioCodecRegistry
				}
				if got := lookupToken(registry, text); got != probe.Canonical {
					t.Errorf("domain %s: lookupToken(%q) = %q, want %q（词条顺序或 pattern 边界问题）",
						domain, text, got, probe.Canonical)
				}
			}
		}
	}
}

// §59.35 一致性测试：引用完整性——
// 1. standard_key 全局唯一（加载器校验 fail-fast，此处验证可加载）
// 2. reverse 无空值（key 存在则 display 非空）
// 3. forward/reverse 闭环：LookupDictKey(variant) → key → ReverseLookup(key) 非空
func TestDictReferenceIntegrity(t *testing.T) {
	st := ensureDict()
	if len(st.reverse) == 0 {
		t.Fatal("reverse map 为空")
	}
	for key, display := range st.reverse {
		if strings.TrimSpace(display) == "" {
			t.Errorf("standard_key %q 的 display 为空", key)
		}
	}
	for _, domain := range DictDomains() {
		for _, tok := range DictTokens(domain) {
			for _, v := range tok.Variants {
				key := LookupDictKey(domain, v)
				if key != tok.StandardKey {
					t.Errorf("LookupDictKey(%q, %q) = %q, want %q", domain, v, key, tok.StandardKey)
					continue
				}
				if d := ReverseLookup(key); d == "" {
					t.Errorf("variant %q → key %q 反查 display 为空", v, key)
				}
			}
		}
	}
}

// §59.35 一致性测试：关键逆向显示名锚定（历史 bug 回归——r4320p 曾反查 2160p）
func TestDictReverseAnchors(t *testing.T) {
	anchors := map[string]string{
		"resolution.r4320p":  "4320p",
		"resolution.r2160p":  "2160p",
		"medium.webdl":       "WEB-DL",
		"medium.uhd_bluray":  "UHD Blu-ray",
		"medium.uhd_remux":   "UHD Blu-ray Remux",
		"medium.encode":      "Encode",
		"video.h264":         "H.264",
		"video.h265":         "HEVC",
		"video.x264":         "x264",
		"video.x265":         "x265",
		"audio.truehd":       "TrueHD",
		"audio.dts_hd_ma":    "DTS-HD MA",
		"hdr.dv":             "DoVi",
		"hdr.dv_hdr":         "DoVi HDR",
		"hdr.dv_hdr10plus":   "DoVi HDR10+",
		"category.movie":     "电影",
		"category.animation": "动漫",
		"source.uk":          "英国",
	}
	for key, want := range anchors {
		if got := ReverseLookup(key); got != want {
			t.Errorf("ReverseLookup(%q) = %q, want %q", key, got, want)
		}
	}
}

// §59.35 一致性测试：forward 锚定（DOM 值归一化历史行为保持）
func TestDictForwardAnchors(t *testing.T) {
	anchors := []struct {
		domain, raw, want string
	}{
		{"type", "电视剧 (TV Series)", "category.tv_series"}, // contains 最长匹配
		{"type", "Movies", "category.movie"},
		{"medium", "UHD BluRay", "medium.uhd_bluray"},
		{"medium", "UHD Blu-ray Remux", "medium.uhd_remux"}, // 最长优先（非 medium.remux）
		{"resolution", "8k", "resolution.r4320p"},
		{"resolution", "4K", "resolution.r2160p"},
		{"video_codec", "HEVC", "video.h265"},
		{"audio_codec", "DD+", "audio.ddp"},
		{"audio_codec", "AC3", "audio.dd"},
		{"source", "欧美", "source.western"},
		{"hdr", "DoVi HDR10+", "hdr.dv_hdr10plus"},
		{"medium", "UNK0", ""},  // 垃圾值 → 空（§59.34 MergeDOMInto 防御依赖）
		{"medium", "UNK11", ""},
	}
	for _, a := range anchors {
		if got := LookupDictKey(a.domain, a.raw); got != a.want {
			t.Errorf("LookupDictKey(%q, %q) = %q, want %q", a.domain, a.raw, got, a.want)
		}
	}
}

// §59.35 边界用例（审计修正后固化）：CI 匹配语义与旧 lookupStandard 等价
// （"uhd bluray" CI 命中 "UHD BluRay"）；contains 方向为 raw 包含 variant
// （"电视" 不含 "电视剧" → 空）；垃圾 key 返回空（§59.34 MergeDOMInto 防御依赖）。
func TestDictLookupEdges(t *testing.T) {
	cases := []struct{ domain, raw, want string }{
		{"type", "movies", "category.movie"},
		{"type", "TV SERIES", "category.tv_series"},
		{"medium", "web-dl", "medium.webdl"},
		{"medium", "uhd bluray", "medium.uhd_bluray"}, // CI 命中 "UHD BluRay"（旧 lookupStandard 同）
		{"type", "电视剧 (TV Series)", "category.tv_series"}, // contains 最长匹配
		{"medium", "UHD Blu-ray Remux", "medium.uhd_remux"},
		{"medium", "Blu-ray Remux", "medium.remux"},
		{"type", "电视", ""},    // 短输入不含长变体（与旧一致）
		{"medium", "UNK0", ""},  // 垃圾值（§59.34 防御依赖）
		{"video_codec", "UNK11", ""},
		{"hdr", "unknown", ""},
		{"type", "  ", ""},
		{"", "x", ""},
		{"notexist", "x", ""},
	}
	for _, c := range cases {
		if got := LookupDictKey(c.domain, c.raw); got != c.want {
			t.Errorf("LookupDictKey(%q, %q) = %q, want %q", c.domain, c.raw, got, c.want)
		}
	}
}

// §59.35 一致性测试：等价组（比较视图）——成员双向等价，非成员不等价
func TestDictEquivGroups(t *testing.T) {
	pairs := [][2]string{
		{"WEB-DL", "WEBRip"},
		{"x265", "HEVC"},
		{"x264", "AVC"},
	}
	for _, p := range pairs {
		if !Equivalent(p[0], p[1]) || !Equivalent(p[1], p[0]) {
			t.Errorf("Equivalent(%q, %q) 应为 true", p[0], p[1])
		}
	}
	nonPairs := [][2]string{
		{"WEB-DL", "HDTV"},
		{"x265", "x264"},
		{"HEVC", "AV1"},
		{"WEB-DL", "x265"}, // 跨组
	}
	for _, p := range nonPairs {
		if Equivalent(p[0], p[1]) {
			t.Errorf("Equivalent(%q, %q) 应为 false", p[0], p[1])
		}
	}
}
