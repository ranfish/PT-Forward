package titleparser

import (
	"strings"
	"testing"
)

// §59.514 附六：音频族 MI 唯一权威（用户定案——From 案信压制参数哲学三连）。
// Lost Prince 案：MI DTS XBR（DTS-HD HR）+ 壹吧回流坏标题 "DTS 5.1"——
// 词表缺口（XBR→裸 DTS 兜底）曾让降级值与坏标题互相强化流出被拒。
func TestAudioMIAuthorityLostPrince(t *testing.T) {
	// 案发形态：壹吧回流标题（DTS 5.1 错）+ MI（DTS XBR 铁证）
	mi := "General\nComplete name : x.mkv\nFormat : Matroska\nVideo\nFormat : HEVC\nWriting library : x265\nAudio #1\nFormat : DTS XBR\nCommercial name : DTS-HD High Resolution Audio\nAudio #2\nFormat : AC-3\nCommercial name : Dolby Digital\n"
	tp := BuildTechProfile("The Lost Prince S01 2003 1080p BluRay HEVC DTS 5.1 2Audios-cXcY@FRDS", mi, "", "", "", "")
	if tp.AudioCodec != "DTS-HD HR" {
		t.Errorf("AudioCodec=%q want DTS-HD HR（MI XBR 铁证覆盖坏标题 DTS）", tp.AudioCodec)
	}
	if tp.AudioTracks != 2 {
		t.Errorf("AudioTracks=%d want 2（MI 双轨）", tp.AudioTracks)
	}
	rt := ReassembleFromTechProfile(tp, V105TitleFormat())
	if !strings.Contains(rt, "DTS-HD HR") || strings.Contains(rt, "XBR") {
		t.Errorf("重组=%q（DTS-HD HR v1.05 写法·XBR 不泄露）", rt)
	}
}

// 词表补丁：DTS XBR → DTS-HD HR（§59.514 附六 唯一实证缺口——4000 抽样
// 审计 14 种 Format 全覆盖的例外；XLL/MLP FBA 已在先例回归锚）。
func TestDTSXBRPattern(t *testing.T) {
	if c := lookupToken(audioCodecRegistry, "DTS XBR"); c != "DTS-HD HR" {
		t.Errorf("DTS XBR → %q want DTS-HD HR", c)
	}
	// 先例回归：XLL→MA / MLP FBA→TrueHD（§59.27 P2 已兑现·不得回归）
	if c := lookupToken(audioCodecRegistry, "DTS XLL"); c != "DTS-HD MA" {
		t.Errorf("DTS XLL → %q want DTS-HD MA", c)
	}
	if c := lookupToken(audioCodecRegistry, "MLP FBA"); c != "TrueHD" {
		t.Errorf("MLP FBA → %q want TrueHD", c)
	}
}

// HasAudio 权威矩阵：
// ①MI Audio 段在场+提取全 → 覆盖（既有行为回归）
// ②MI Audio 段在场+提取 miss（未知 Format）→ 清零标题值（新语义·宁缺毋错）
// ③MI 缺失 → 标题回退（既有行为回归）
func TestHasAudioAuthorityMatrix(t *testing.T) {
	// ① 覆盖
	tp1 := BuildTechProfile("Show S01 DTS 1080p-GRP", "General\nAudio #1\nFormat : AC-3\n", "", "", "", "")
	if tp1.AudioCodec != "DD" {
		t.Errorf("① AudioCodec=%q want DD（MI 覆盖标题 DTS）", tp1.AudioCodec)
	}
	// ② miss 清零（未知 Format：HasAudio=true 但 lookup miss）
	tp2 := BuildTechProfile("Show S01 DTS 1080p-GRP", "General\nAudio #1\nFormat : SOMEPASTFORMAT\n", "", "", "", "")
	if tp2.AudioCodec != "" {
		t.Errorf("② AudioCodec=%q want 空（MI 在场提取 miss 清零标题 DTS·宁缺毋错）", tp2.AudioCodec)
	}
	// ③ MI 缺失回退
	tp3 := BuildTechProfile("Show S01 DTS 1080p-GRP", "", "", "", "", "")
	if tp3.AudioCodec != "DTS" {
		t.Errorf("③ AudioCodec=%q want DTS（MI 缺失标题回退）", tp3.AudioCodec)
	}
}
