// Token Registry 属性测试（§59.27）。
//
// 核心不变量：
//  1. 变体覆盖：同一物理编码的所有常见写法解析出相同 Canonical
//  2. round-trip：parse(reassemble(profile)) == profile
//  3. 提取/移除对称：extract 命中的 token 一定能被 remove 移除
package titleparser

import (
	"strings"
	"testing"
)

// TestTokenVariantCoverage 变体覆盖：v0.0.602 bug 的回归防护。
// "DTS-HD.MA"（点分隔）与 "DTS-HD MA"（空格分隔）必须解析出相同值。
func TestTokenVariantCoverage(t *testing.T) {
	cases := []struct {
		title string
		want  string
	}{
		// DTS-HD MA 全变体
		{"Movie.DTS-HD.MA.5.1", "DTS-HD MA"},
		{"Movie DTS-HD MA 5.1", "DTS-HD MA"},
		{"Movie.DTS HD MA", "DTS-HD MA"},
		{"Movie.DTSHDMA", ""},
		{"Movie.DTS-HD.MA.7.1-GROUP", "DTS-HD MA"},
		// DTS:X
		{"Movie.DTS.X.5.1", "DTS:X"},
		{"Movie.DTS:X", "DTS:X"},
		// DTS 家族优先级（特例在前）
		{"Movie.DTS-HD.HR", "DTS-HD HR"},
		{"Movie.DTS.5.1", "DTS"},
		// TrueHD
		{"Movie.TrueHD.5.1", "TrueHD"},
		{"Movie.True-HD", "TrueHD"},
		// DDP 变体（含声道后缀）
		{"Movie.DDP5.1", "DDP"},
		{"Movie.E-AC-3", "DDP"},
		{"Movie.DD+.5.1", "DDP"},
		{"Movie.EAC3", "DDP"},
		// DD / AC3
		{"Movie.DD.5.1", "DD"},
		{"Movie.AC3.5.1", "DD"},
		{"Movie.AC-3", "DD"},
		// 视频编码
		{"Movie.x265", "x265"},
		{"Movie.H.265", "HEVC"},
		{"Movie.H265", "HEVC"},
		{"Movie.HEVC", "HEVC"},
		{"Movie.x264", "x264"},
		{"Movie.H.264", "AVC"},
		{"Movie.AVC", "AVC"},
		{"Movie.AV1", "AV1"},
		{"Movie.VC-1", "VC-1"},
		{"Movie.VC1", "VC-1"},
	}
	for _, c := range cases {
		gotAudio := extractAudio(c.title)
		gotVideo := extractVideoCodec(c.title)
		// want 为空表示此项既不是 audio 也不是 video 关注项
		if c.want == "" {
			continue
		}
		if gotAudio != c.want && gotVideo != c.want {
			t.Errorf("title=%q: audio=%q video=%q, want %q", c.title, gotAudio, gotVideo, c.want)
		}
	}
}

// TestTokenExtractRemoveSymmetry 提取/移除对称：
// extract 命中的 token 必须能被 remove*Tokens 从标题移除。
func TestTokenExtractRemoveSymmetry(t *testing.T) {
	audioTitles := []string{
		"Movie.DTS-HD.MA.5.1",
		"Movie DTS-HD MA 5.1",
		"Movie.DTS.X",
		"Movie.TrueHD.Atmos",
		"Movie.DDP5.1",
		"Movie.E-AC-3.5.1",
		"Movie.DD+.2.0",
		"Movie.AC3.5.1",
		"Movie.DD.5.1",
		"Movie.FLAC.1.0",
		"Movie.AAC2.0",
		"Movie.DTS.5.1",
	}
	for _, title := range audioTitles {
		if extractAudio(title) == "" {
			continue
		}
		removed := removeAudioCodecTokens(title)
		if extractAudio(removed) != "" {
			t.Errorf("asymmetry: title=%q removed=%q still extracts %q", title, removed, extractAudio(removed))
		}
	}

	videoTitles := []string{
		"Movie.H.265",
		"Movie.H265",
		"Movie.HEVC",
		"Movie.x265",
		"Movie.x264",
		"Movie.H.264.AVC",
		"Movie.VC-1",
		"Movie.MPEG-2",
		"Movie.AV1",
	}
	for _, title := range videoTitles {
		if extractVideoCodec(title) == "" {
			continue
		}
		removed := removeVideoCodecTokens(title)
		if extractVideoCodec(removed) != "" {
			t.Errorf("asymmetry: title=%q removed=%q still extracts %q", title, removed, extractVideoCodec(removed))
		}
	}
}

// TestTokenRoundTrip round-trip：TechProfile → 重组标题 → 再解析 == 原 profile 关键字段。
// 这是匹配业务（techProfileConflict）正确性的基础保证。
func TestTokenRoundTrip(t *testing.T) {
	profiles := []TechProfile{
		{MainTitle: "Gilda", Year: "1946", Resolution: "2160p", VideoCodec: "x265", AudioCodec: "FLAC", HDR: "DoVi HDR", ReleaseGroup: "FRDS"},
		{MainTitle: "Carrie", Year: "1976", Resolution: "2160p", VideoCodec: "x265", AudioCodec: "DTS-HD MA", HDR: "DoVi HDR", ReleaseGroup: "FRDS"},
		{MainTitle: "Father", Year: "2020", Resolution: "1080p", VideoCodec: "x265", AudioCodec: "DDP", HDR: "HDR10", ReleaseGroup: "GROUP"},
		{MainTitle: "Snatch", Year: "2000", Resolution: "2160p", VideoCodec: "x265", AudioCodec: "TrueHD", HDR: "DoVi HDR", ReleaseGroup: "FRDS"},
		{MainTitle: "Old Movie", Year: "1990", Resolution: "1080p", VideoCodec: "AVC", AudioCodec: "DD", ReleaseGroup: "GRP"},
	}
	tf := DefaultTitleFormat()
	// DefaultTitleFormat.Order 不含 hdr（站点模板可配）。
	// round-trip 测试用 v1.05 完整字段序（含 hdr），验证 registry 双向一致性。
	tf.Order = []string{"title", "year", "season", "resolution", "hdr", "medium", "audio_codec", "video_codec", "group"}
	for _, p := range profiles {
		title := ReassembleFromTechProfile(p, tf)
		if title == "" {
			t.Fatalf("reassemble empty for %+v", p)
		}
		reparsed := ParseTitleTech(title)
		if reparsed.VideoCodec != p.VideoCodec {
			t.Errorf("round-trip VideoCodec: title=%q got=%q want=%q", title, reparsed.VideoCodec, p.VideoCodec)
		}
		if reparsed.AudioCodec != p.AudioCodec {
			t.Errorf("round-trip AudioCodec: title=%q got=%q want=%q", title, reparsed.AudioCodec, p.AudioCodec)
		}
		if reparsed.Resolution != p.Resolution {
			t.Errorf("round-trip Resolution: title=%q got=%q want=%q", title, reparsed.Resolution, p.Resolution)
		}
		if reparsed.ReleaseGroup != p.ReleaseGroup {
			t.Errorf("round-trip ReleaseGroup: title=%q got=%q want=%q", title, reparsed.ReleaseGroup, p.ReleaseGroup)
		}
		if !strings.Contains(reparsed.HDR, strings.Split(p.HDR, " ")[0]) {
			t.Errorf("round-trip HDR: title=%q got=%q want contains %q", title, reparsed.HDR, p.HDR)
		}
	}
}
