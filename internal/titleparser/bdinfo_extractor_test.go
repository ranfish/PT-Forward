package titleparser

import (
	"os"
	"strings"
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

// §59.319 附四：格式化三规则+幂等锚
func TestFormatBDInfoReport(t *testing.T) {
	report := loadBDTestdata(t, "bdinfo_mercy_truehd.txt")
	got := FormatBDInfoReport(report)

	// a. 头部清除（DISC INFO: 起步）——头部独有内容断言
	// （Disc Label/Disc Size 在 DISC INFO 正文块内也有，属保留内容）
	if !strings.HasPrefix(got, "DISC INFO:") {
		t.Errorf("应以 DISC INFO: 开头, got %q", firstLine(got))
	}
	for _, gone := range []string{"BDINFO HOME", "Cinema Squid", "BEGIN FORUMS PASTE", "UniqProject"} {
		if strings.Contains(got, gone) {
			t.Errorf("头部残留: %s", gone)
		}
	}
	// b. [code] 标记清除
	if strings.Contains(got, "[code]") || strings.Contains(got, "[/code]") {
		t.Error("[code] 标记应剥除")
	}
	// c. END FORUMS 行清除
	if strings.Contains(got, "END FORUMS PASTE") {
		t.Error("END FORUMS 行应删除")
	}
	// 九块正文保留（用户权威清单 §59.319 附四）
	for _, block := range []string{"DISC INFO:", "PLAYLIST REPORT:", "VIDEO:", "AUDIO:", "SUBTITLES:", "FILES:", "CHAPTERS:", "STREAM DIAGNOSTICS:", "QUICK SUMMARY:"} {
		if !strings.Contains(got, block) {
			t.Errorf("正文块缺失: %s", block)
		}
	}
	// 幂等
	if again := FormatBDInfoReport(got); again != got {
		t.Error("重复格式化应幂等")
	}
	// 非报告形态原样
	if got := FormatBDInfoReport("General\nComplete name : x.mkv"); got != "General\nComplete name : x.mkv" {
		t.Error("非报告形态应原样返回")
	}
}

func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l != "" {
			return l
		}
	}
	return ""
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

// §59.319 附十六: FromBDInfo 原盘铁证——压制写法启发式不覆盖
func TestMediumCanonicalOf_FromBDInfo(t *testing.T) {
	// Under Current 形态：标题无连字符 BluRay（MTeam 惯用）+ BDInfo 报告
	report := "QUICK SUMMARY:\n\nVideo: MPEG-4 AVC Video / 28143 kbps / 1080p / 23.976 fps / 16:9 / High Profile 4.1\n"
	p := BuildTechProfile("Under Current 2025 BluRay 1080p AVC LPCM5.1-MTeam", report, "", "", "", "")
	if !p.FromBDInfo {
		t.Fatal("BDInfo 输入应置 FromBDInfo")
	}
	if got := MediumCanonicalOf(p); got != "Blu-ray 原盘" {
		t.Errorf("Medium = %q, want Blu-ray 原盘（BDInfo 铁证不受压制写法覆盖）", got)
	}
	if IsEncode(p) {
		t.Error("原盘 IsEncode 应 false")
	}
	// UHD 联动：2160p + BluRay 标题写法 + BDInfo → UHD Blu-ray 原盘
	uhdReport := "QUICK SUMMARY:\n\nVideo: MPEG-H HEVC Video / 81085 kbps / 2160p / 23.976 fps / 16:9 / Main 10 @ Level 5.1 @ High / 10 bits / HDR10 / BT.2020\n"
	pu := BuildTechProfile("Alice 1951 BluRay 2160p HEVC DTS-HD MA5.1-MTeam", uhdReport, "", "", "", "")
	if got := MediumCanonicalOf(pu); got != "UHD Blu-ray 原盘" {
		t.Errorf("UHD Medium = %q, want UHD Blu-ray 原盘", got)
	}
	// 对照回归：同标题写法但输入是普通 MI（无编码痕迹）→ 维持 Encode 判定（启发式不回归）
	mi := "General\nComplete name : x.mkv\nVideo\nFormat : AVC\nWriting library : x265"
	pm := BuildTechProfile("Some 2025 BluRay 1080p x265-MTeam", mi, "", "", "", "")
	if pm.FromBDInfo {
		t.Error("MI 输入不应置 FromBDInfo")
	}
	if !IsEncode(pm) {
		t.Error("压制写法+x265 编码族应维持 Encode（回归锚）")
	}
	// 对照：连字符原盘写法无 BDInfo → 原盘族（既有行为不回归）
	pd := BuildTechProfile("Some.Disc.2025.Blu-ray.AVC-GRP", "", "", "", "", "")
	if got := MediumCanonicalOf(pd); got != "Blu-ray 原盘" {
		t.Errorf("连字符原盘 Medium = %q, want Blu-ray 原盘（回归）", got)
	}
}

// §59.319 附二十三: MINBD 排除——x264 标记不被 FromBDInfo 铁证覆盖
func TestMediumCanonicalOf_MINBD(t *testing.T) {
	// Inception MINBD 实测形态：标题 x264 + BDInfo 报告（BDMV 结构在）
	report := "QUICK SUMMARY:\n\nVideo: MPEG-4 AVC Video / 9418 kbps / 1080p / 23.976 fps / 16:9 / High Profile 4.1\n"
	p := BuildTechProfile("Inception.2010.BluRay.x264.DTS.MINBD1080P-CMCT", report, "", "", "", "")
	if p.FromBDInfo {
		t.Fatal("MINBD 不应置 FromBDInfo（x264 重编码排除）")
	}
	if !p.MIEncoded {
		t.Fatal("MINBD 应置 MIEncoded（x264 重编码证据）")
	}
	if got := MediumCanonicalOf(p); got != "Encode" {
		t.Errorf("MINBD Medium = %q, want Encode（x264 重编码不被铁证覆盖）", got)
	}
	if !IsEncode(p) {
		t.Error("MINBD IsEncode 应 true")
	}
	// 对照回归：真原盘（无 x264 标记）不被影响
	pure := BuildTechProfile("Under Current 2025 BluRay 1080p AVC LPCM5.1-MTeam", report, "", "", "", "")
	if got := MediumCanonicalOf(pure); got != "Blu-ray 原盘" {
		t.Errorf("真原盘 Medium = %q, want Blu-ray 原盘（回归锚）", got)
	}
	// 对照回归：HEVC UHD 真原盘（无 x265）
	uhd := BuildTechProfile("Alice 1951 UHD BluRay 2160p HEVC DTS-HD MA5.1-MTeam",
		"QUICK SUMMARY:\n\nVideo: MPEG-H HEVC Video / 81085 kbps / 2160p / 23.976 fps / 16:9 / Main 10 / 10 bits / HDR10 / BT.2020\n", "", "", "", "")
	if got := MediumCanonicalOf(uhd); got != "UHD Blu-ray 原盘" {
		t.Errorf("真 UHD 原盘 Medium = %q, want UHD Blu-ray 原盘（回归）", got)
	}
	// x265 MINBD 也排除
	x265 := BuildTechProfile("Some.2025.BluRay.x265.MINBD1080P-CMCT", report, "", "", "", "")
	if got := MediumCanonicalOf(x265); got != "Encode" {
		t.Errorf("x265 MINBD Medium = %q, want Encode", got)
	}
}
