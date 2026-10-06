package titleparser

import (
	"os"
	"testing"
)

func loadBDTestdata(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	return string(b)
}

// §59.319 D5 锚 1：243 PT0 真盘实测——1080p AVC SDR 老盘（无 bit depth/HDR
// 字段，BitDepth/HDR 留空由调用方兜底）。
func TestExtractBDInfo_JR_1080p_SDR(t *testing.T) {
	report := loadBDTestdata(t, "bdinfo_jr_1080p.txt")
	got := ExtractBDInfo(report)

	if got.VideoCodec != "AVC" {
		t.Errorf("VideoCodec = %q, want AVC", got.VideoCodec)
	}
	if got.Resolution != "1080p" {
		t.Errorf("Resolution = %q, want 1080p", got.Resolution)
	}
	if got.FrameRate != "23.976" {
		t.Errorf("FrameRate = %q, want 23.976", got.FrameRate)
	}
	if got.HDR != "" {
		t.Errorf("HDR = %q, want empty (SDR)", got.HDR)
	}
	if got.BitDepth != "" {
		t.Errorf("BitDepth = %q, want empty (老盘无标注)", got.BitDepth)
	}
	if got.AudioCodec != "DTS-HD MA" {
		t.Errorf("AudioCodec = %q, want DTS-HD MA", got.AudioCodec)
	}
	if got.AudioChannels != "2.0" {
		t.Errorf("AudioChannels = %q, want 2.0", got.AudioChannels)
	}
	if got.AudioTracks != 2 {
		t.Errorf("AudioTracks = %d, want 2 (EN+IT DTS-HD MA)", got.AudioTracks)
	}
	if got.Encoded {
		t.Error("Encoded 恒 false（原盘铁证）")
	}
}

// §59.319 D5 锚 2：UHD HEVC DoVi 双层盘——主行 2160p HDR10 + 隐藏行 1080p
// Dolby Vision → "DoVi HDR"；10bit；6 条非隐藏音轨（DTS-HD MA×2 + DD×4）。
func TestExtractBDInfo_Alice_UHD_DoVi(t *testing.T) {
	report := loadBDTestdata(t, "bdinfo_alice_uhd.txt")
	got := ExtractBDInfo(report)

	if got.VideoCodec != "HEVC" {
		t.Errorf("VideoCodec = %q, want HEVC", got.VideoCodec)
	}
	if got.Resolution != "2160p" {
		t.Errorf("Resolution = %q, want 2160p", got.Resolution)
	}
	if got.FrameRate != "23.976" {
		t.Errorf("FrameRate = %q, want 23.976", got.FrameRate)
	}
	if got.BitDepth != "10bit" {
		t.Errorf("BitDepth = %q, want 10bit", got.BitDepth)
	}
	if got.HDR != "DoVi HDR" {
		t.Errorf("HDR = %q, want DoVi HDR (主行 HDR10 + 隐藏行 DV)", got.HDR)
	}
	if got.AudioCodec != "DTS-HD MA" {
		t.Errorf("AudioCodec = %q, want DTS-HD MA", got.AudioCodec)
	}
	if got.AudioChannels != "5.1" {
		t.Errorf("AudioChannels = %q, want 5.1", got.AudioChannels)
	}
	if got.AudioTracks != 6 {
		t.Errorf("AudioTracks = %d, want 6", got.AudioTracks)
	}
}

// §59.319 D5 锚 3：TrueHD Atmos 盘——音轨名内嵌 "/Atmos"（"Dolby TrueHD/Atmos
// Audio"）→ TrueHD + AudioTechnology=Atmos；AC3 Embedded 括注不算独立轨。
func TestExtractBDInfo_Mercy_TrueHD_Atmos(t *testing.T) {
	report := loadBDTestdata(t, "bdinfo_mercy_truehd.txt")
	got := ExtractBDInfo(report)

	if got.VideoCodec != "HEVC" || got.Resolution != "2160p" {
		t.Errorf("video = %q/%q, want HEVC/2160p", got.VideoCodec, got.Resolution)
	}
	if got.AudioCodec != "TrueHD" {
		t.Errorf("AudioCodec = %q, want TrueHD (内嵌 /Atmos 剥离后映射)", got.AudioCodec)
	}
	if got.AudioChannels != "7.1" {
		t.Errorf("AudioChannels = %q, want 7.1", got.AudioChannels)
	}
	if got.AudioTechnology != "Atmos" {
		t.Errorf("AudioTechnology = %q, want Atmos", got.AudioTechnology)
	}
	if got.AudioTracks != 7 {
		t.Errorf("AudioTracks = %d, want 7 (TrueHD/Atmos + DTS-HD MA×2 + DD×4)", got.AudioTracks)
	}
}

// §59.319 P1: ExtractMediaInfo 单点格式分流——BDInfo 报告自动路由到
// ExtractBDInfo（全部 MI 消费方零改动获得 BD 能力）
func TestExtractMediaInfo_BDInfoRouted(t *testing.T) {
	report := loadBDTestdata(t, "bdinfo_alice_uhd.txt")
	got := ExtractMediaInfo(report)
	if got.VideoCodec != "HEVC" || got.Resolution != "2160p" || got.HDR != "DoVi HDR" {
		t.Errorf("分流未生效: %+v", got)
	}
	if got.Encoded {
		t.Error("原盘 Encoded 恒 false")
	}
}

func TestExtractBDInfo_EdgeCases(t *testing.T) {
	if got := ExtractBDInfo(""); got.Resolution != "" || got.VideoCodec != "" {
		t.Error("空输入应返回零值")
	}
	if got := ExtractBDInfo("not a bdinfo report\nGeneral\nComplete name : x.mkv"); got.Resolution != "" {
		t.Error("非 BDInfo 形态（无 QUICK SUMMARY 锚）应返回零值")
	}
	// 裸 QUICK SUMMARY 块（无完整报告头）也应可解析
	bare := "QUICK SUMMARY:\n\nVideo: MPEG-H HEVC Video / 100 kbps / 2160p / 24 fps / 16:9 / Main 10 @ Level 5.1 @ High / 10 bits / HDR10+ / BT.2020\nAudio: English / Dolby TrueHD Audio / 7.1 / 48 kHz / 1000 kbps / Atmos\n"
	got := ExtractBDInfo(bare)
	if got.HDR != "HDR10+" {
		t.Errorf("HDR = %q, want HDR10+", got.HDR)
	}
	if got.AudioCodec != "TrueHD" || got.AudioChannels != "7.1" {
		t.Errorf("audio = %q/%q, want TrueHD/7.1", got.AudioCodec, got.AudioChannels)
	}
	if got.AudioTechnology != "Atmos" {
		t.Errorf("AudioTechnology = %q, want Atmos", got.AudioTechnology)
	}
	// Dolby Vision 无 HDR10 主行（单层 DV 盘形态）
	dv := "QUICK SUMMARY:\nVideo: MPEG-H HEVC Video / 100 kbps / 2160p / 24 fps / 16:9 / Main 10 @ Level 5.1 @ High / 10 bits / Dolby Vision / BT.2020\n"
	if got := ExtractBDInfo(dv); got.HDR != "DoVi" {
		t.Errorf("HDR = %q, want DoVi", got.HDR)
	}
	// §59.319 P3 修复锚：全 playlist 报告多块 QUICK SUMMARY——只解析首块
	// （243 Alice 跨块累计 6→18 事故）
	multi := "QUICK SUMMARY:\n\nVideo: MPEG-H HEVC Video / 100 kbps / 2160p / 24 fps / 16:9 / Main 10 @ Level 5.1 @ High / 10 bits / HDR10 / BT.2020\nAudio: English / Dolby TrueHD Audio / 7.1 / 48 kHz / 1000 kbps / 24-bit\n\n\n********************\nPLAYLIST: 00001.MPLS\n********************\nQUICK SUMMARY:\n\nVideo: MPEG-H HEVC Video / 50 kbps / 1080p / 24 fps / 16:9 / Main 10 @ Level 5.1 @ High / 10 bits / HDR10 / BT.2020\nAudio: English / Dolby TrueHD Audio / 5.1 / 48 kHz / 500 kbps / 24-bit\nAudio: English / Dolby TrueHD Audio / 2.0 / 48 kHz / 300 kbps / 24-bit\n"
	gotMulti := ExtractBDInfo(multi)
	if gotMulti.AudioTracks != 1 {
		t.Errorf("多块报告应只计首块音轨, got %d, want 1", gotMulti.AudioTracks)
	}
	if gotMulti.Resolution != "2160p" {
		t.Errorf("首块分辨率, got %q", gotMulti.Resolution)
	}
}
