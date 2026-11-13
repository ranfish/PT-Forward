package titleparser

import (
	"strings"
	"testing"
)

// §59.517 测试锚：WEB-DL/WEBRip MI 判定引擎。
//
// 锚组构成：
//   - 合成良性混合防御矩阵 8 态（§59.517 七——全库无活体·合成验证）
//   - 真实案例锚（用户人工终审 40+ 案例的代表形态——mi3/mi5/mi6/mi8）

// webMIOpts 合成 MI 构造器特征项。
type webMIOpts struct {
	color       bool // 色彩三件套齐全
	fullRange   bool // Color range: Full
	dvP5        bool
	hdrVivid    bool
	x264WL      bool
	x265LinuxWL bool
	sc0         bool
	stitchable  bool
	keyint      string
	aiSub       bool
	oldMux      bool
	bitDepth    string
	width       string
	height      string
	fps         string
}

// webMI 合成 MI 文本构造器（可调特征——模拟真实 MI 五层结构）。
func webMI(o webMIOpts) string {
	colorLines := ""
	if o.color {
		colorLines = "Color primaries                          : BT.2020\n" +
			"Transfer characteristics                 : PQ\n" +
			"Matrix coefficients                      : BT.2020 non-constant\n"
	}
	rangeLine := ""
	if o.fullRange {
		rangeLine = "Color range                              : Full\n"
	}
	hdrLine := ""
	if o.dvP5 {
		hdrLine = "HDR format                               : Dolby Vision, Version 1.0, Profile 5, dvhe.05.06, BL+RPU\n"
	} else if o.hdrVivid {
		hdrLine = "HDR format                               : HDR Vivid, Version 1\n"
	}
	wl, es := "", ""
	if o.x264WL {
		wl = "Writing library                          : x264 core 148 r2744 b97ae06\n"
	}
	if o.x265LinuxWL {
		wl = "Writing library                          : x265 [info]: 3.4+30:[Linux][GCC][x86_64] 64bit\n"
	}
	var parts []string
	if o.sc0 {
		parts = append(parts, "scenecut=0")
	}
	if o.stitchable {
		parts = append(parts, "stitchable=1")
	}
	if o.keyint != "" {
		parts = append(parts, "keyint="+o.keyint, "keyint-min="+o.keyint)
	}
	if len(parts) > 0 {
		es = "Encoding settings                        : " + strings.Join(parts, " / ") + "\n"
	}
	mux := "mkvmerge v94.0 ('Initiate') 64-bit"
	if o.oldMux {
		mux = "mkvmerge v8.3.0 ('Over the Horizon') 64bit"
	}
	subTitle := "Chinese"
	if o.aiSub {
		subTitle = "ChsEng_AI（DS）字幕"
	}
	depth := o.bitDepth
	if depth == "" {
		depth = "10 bits"
	}
	w, h := o.width, o.height
	if w == "" {
		w, h = "3 840 pixels", "1 608 pixels"
	}
	fps := o.fps
	if fps == "" {
		fps = "25.000 FPS"
	}
	return "General\n" +
		"Complete name                            : Test.2026.2160p.WEB-DL.H265.DDP5.1-GRP.mkv\n" +
		"Writing application                      : " + mux + "\n" +
		"Writing library                          : libebml v1.4.5 + libmatroska v1.7.1\n" +
		"\nVideo\n" +
		"ID                                       : 1\n" +
		"Format                                   : HEVC\n" +
		hdrLine +
		"Codec ID                                 : V_MPEGH/ISO/HEVC\n" +
		"Width                                    : " + w + "\n" +
		"Height                                   : " + h + "\n" +
		"Frame rate                               : " + fps + "\n" +
		"Bit depth                                : " + depth + "\n" +
		wl + es + colorLines + rangeLine +
		"\nAudio\n" +
		"ID                                       : 2\n" +
		"Format                                   : E-AC-3\n" +
		"Bit rate                                 : 448 kb/s\n" +
		"\nText\n" +
		"ID                                       : 3\n" +
		"Format                                   : UTF-8\n" +
		"Title                                    : " + subTitle + "\n"
}

// TestWEBdlBenignHybridMatrix §59.517 七：良性混合防御合成矩阵 8 态。
// 良性混合（官方视频+AI字幕混封）必须官方——视频层正面信号先触发·
// AI 字幕补刀位永远轮不到；剥净形态由 AI 字幕/古董混流器定罪。
func TestWEBdlBenignHybridMatrix(t *testing.T) {
	cases := []struct {
		name    string
		mi      string
		verdict string
		rule    string
	}{
		{"色彩全+AI字幕→官方", webMI(webMIOpts{color: true, aiSub: true}), WEBVerdictOfficial, "交叉:HEVC10bit+色彩全"},
		{"DV P5+AI字幕→官方", webMI(webMIOpts{dvP5: true, aiSub: true}), WEBVerdictOfficial, "铁证②DV P5"},
		{"HDR Vivid+AI字幕→官方", webMI(webMIOpts{hdrVivid: true, aiSub: true}), WEBVerdictOfficial, "铁证⑧HDR Vivid"},
		{"平台x264+AI字幕→官方", "General\nWriting application: mkvmerge v94.0 64-bit\nWriting library: libebml v1.4.5\n\n" +
			"Video\nFormat: AVC\nWriting library: x264 core 148\nEncoding settings: scenecut=0 / stitchable=1 / keyint=60 / keyint-min=60\n" +
			"Format settings: CABAC\nFrame rate: 30.000 FPS\nBit depth: 8 bits\nWidth: 1 920 pixels\nHeight: 1 080 pixels\n\n" +
			"Text\nFormat: UTF-8\nTitle: ChsEng_AI（DS）字幕\n", WEBVerdictOfficial, "交叉:x264+stitchable"},
		{"平台x265Linux+AI字幕→官方", webMI(webMIOpts{x265LinuxWL: true, sc0: true, keyint: "75", aiSub: true}), WEBVerdictOfficial, "交叉:x265Linux+sc0"},
		{"剥净+AI字幕→民间", webMI(webMIOpts{aiSub: true}), WEBVerdictCivilian, "交叉:AI字幕"},
		{"剥净+古董混流器→民间", webMI(webMIOpts{oldMux: true}), WEBVerdictCivilian, "交叉:古董混流器"},
		{"零信号→官方默认", webMI(webMIOpts{}), WEBVerdictOfficial, "交叉:无信号默认官方"},
	}
	for _, c := range cases {
		v, r := ExtractWEBdlVerdict(ParseMISections(c.mi))
		if v != c.verdict || r != c.rule {
			t.Errorf("[%s] 判定=%q 规则=%q（期望 %q/%q）", c.name, v, r, c.verdict, c.rule)
		}
	}
}

// TestWEBdlRealCaseAnchors 真实案例锚（用户人工终审代表形态）。
func TestWEBdlRealCaseAnchors(t *testing.T) {
	cases := []struct {
		name    string
		mi      string
		verdict string
		rule    string
	}{
		// Kanjoos（mi3 终审=WEBRip）：ZEE5 标题标 WEB-DL 实为 x264 民压——
		// sc=40+裁切(1080p容器高度816)+开源默认 keyint 250/25 组合
		{"Kanjoos组合", "General\nWriting application: mkvmerge v94.0 64-bit\nWriting library: libebml v1.4.5\n\n" +
			"Video\nFormat: AVC\nWriting library: x264 core 148\nEncoding settings: scenecut=40 / keyint=250 / keyint-min=25\n" +
			"Frame rate: 25.000 FPS\nBit depth: 8 bits\nWidth: 1 920 pixels\nHeight: 816 pixels\n",
			WEBVerdictCivilian, "交叉:x264+sc40+裁切+开源keyint"},
		// Mana Shankara（用户终审=WEBRip）：ZEE5 x265 Linux 构建 sc40+crf 非零小数
		{"ManaShankara", "General\nWriting application: mkvmerge v94.0 64-bit\nWriting library: libebml v1.4.5\n\n" +
			"Video\nFormat: HEVC\nWriting library: x265 [info]: 3.4+30:[Linux][GCC][x86_64] 64bit\n" +
			"Encoding settings: keyint=75 / min-keyint=75 / scenecut=40 / crf=20.2\n" +
			"Bit depth: 10 bits\nFrame rate: 25.000 FPS\n", WEBVerdictCivilian, "交叉:x265+sc40+crf小数"},
		// nvenc（mi7 终审=WEBRip）：General 干净 + Video WL GPU 硬编码
		{"nvencGPU", "General\nWriting application: mkvmerge v8.3.0 64bit\nWriting library: libebml v1.3.1 + libmatroska v1.4.2\n\n" +
			"Video\nFormat: HEVC\nWriting library: Lavc61.31.101 hevc_nvenc\nBit depth: 10 bits\nFrame rate: 25.000 FPS\n" +
			"Color range: Limited\nColor primaries: BT.709\nTransfer characteristics: BT.709\nMatrix coefficients: BT.709\n",
			WEBVerdictCivilian, "铁证①GPU硬编码"},
		// Sin Trade HQ DoVi（mi5 终审=官方）：色彩三件套缺 + Full range
		// （DoVi 母带指纹——标题写 DoVi 但 MI 无 DV 标记形态）
		{"FullRange", webMI(webMIOpts{fullRange: true}), WEBVerdictOfficial, "交叉:HEVC10bit+FullRange"},
		// Empire Falls（mi6/mi8 终审=WEBRip）：剥净+AI 字幕（mkvmerge v8.3.0）
		{"EmpireFalls", webMI(webMIOpts{aiSub: true, oldMux: true, bitDepth: "10 bits",
			width: "1 920 pixels", height: "1 080 pixels"}), WEBVerdictCivilian, "交叉:AI字幕"},
		// HEVC 8bit 无 WL（Sin Trade DDP5.1 Atmos 变体·终审=官方）
		{"HEVC8bit无WL", webMI(webMIOpts{bitDepth: "8 bits"}), WEBVerdictOfficial, "交叉:HEVC8bit无WL"},
		// AVC 无 WL + Format settings（Sin Trade 1080p 变体·终审=官方）
		{"AVC无WL+FS", "General\nWriting application: mkvmerge v78.0 64-bit\nWriting library: libebml v1.4.4 + libmatroska v1.7.1 / Lavf60.10.100\n\n" +
			"Video\nFormat: AVC\nFormat settings: CABAC / 3 Ref Frames\nFormat settings, CABAC: Yes\nBit depth: 8 bits\nFrame rate: 25.000 FPS\n" +
			"Color primaries: BT.709\nTransfer characteristics: BT.709\nMatrix coefficients: BT.709\n",
			WEBVerdictOfficial, "交叉:AVC无WL+FS"},
	}
	for _, c := range cases {
		v, r := ExtractWEBdlVerdict(ParseMISections(c.mi))
		if v != c.verdict || r != c.rule {
			t.Errorf("[%s] 判定=%q 规则=%q（期望 %q/%q）", c.name, v, r, c.verdict, c.rule)
		}
	}
}

// TestWEBdlNoVideoB1 B1 契约：MI 无 Video 段 → 不判定（verdict 空）。
func TestWEBdlNoVideoB1(t *testing.T) {
	v, r := ExtractWEBdlVerdict(ParseMISections("General\nWriting library: libebml / ShanaEncoder\n"))
	if v != "" || r != "" {
		t.Errorf("无 Video 段应不判定·得到 %q/%q", v, r)
	}
	mi := ExtractMediaInfo("General\nWriting library: libebml\nFormat: Matroska\n")
	if mi.WEBVerdict != "" {
		t.Errorf("ExtractMediaInfo 无 Video 段 WEBVerdict 应空·得到 %q", mi.WEBVerdict)
	}
}

// TestWEBdlCRFDecimalForms crf 假小数防御：".0" 不算（NF 官方 16.0）·
// 非零小数才算（20.2 民压）。
func TestWEBdlCRFDecimalForms(t *testing.T) {
	// x265 Linux + sc0 + crf=16.0（整数·NF 管线）→ 官方（sc0 正面组合）
	nf := "General\nWriting application: mkvmerge v94.0 64-bit\n\nVideo\nFormat: HEVC\n" +
		"Writing library: x265 [info]: Linux\nEncoding settings: scenecut=0 / crf=16.0 / keyint=75 / min-keyint=75\n" +
		"Bit depth: 10 bits\nFrame rate: 25.000 FPS\n"
	if v, r := ExtractWEBdlVerdict(ParseMISections(nf)); v != WEBVerdictOfficial || r != "交叉:x265Linux+sc0" {
		t.Errorf("crf=16.0 应走 sc0 官方·得到 %q/%q", v, r)
	}
	// x265 Linux + sc40 + crf=20.2（非零小数）→ 民间
	civ := "General\nWriting application: mkvmerge v94.0 64-bit\n\nVideo\nFormat: HEVC\n" +
		"Writing library: x265 [info]: Linux\nEncoding settings: scenecut=40 / crf=20.2\n" +
		"Bit depth: 10 bits\nFrame rate: 25.000 FPS\n"
	if v, r := ExtractWEBdlVerdict(ParseMISections(civ)); v != WEBVerdictCivilian || r != "交叉:x265+sc40+crf小数" {
		t.Errorf("sc40+crf=20.2 应民间·得到 %q/%q", v, r)
	}
}

// TestWEBdlIntegerSecGOPDefault 整数秒 keyint 排除 x264 默认 GOP 对
// 250/25（25fps 下恰好 10 秒的假对齐——Kanjoos 教训·§59.517 十一④）。
func TestWEBdlIntegerSecGOPDefault(t *testing.T) {
	// keyint=250/min=25 @25fps：数字上整 10 秒但为开源默认 → 不触发整数秒
	mi := "General\nWriting application: mkvmerge v94.0 64-bit\n\nVideo\nFormat: AVC\n" +
		"Writing library: x264 core 148\nEncoding settings: scenecut=0 / keyint=250 / keyint-min=25\n" +
		"Frame rate: 25.000 FPS\nBit depth: 8 bits\nWidth: 1 920 pixels\nHeight: 1 080 pixels\n"
	if v, r := ExtractWEBdlVerdict(ParseMISections(mi)); v != WEBVerdictOfficial || r != "交叉:x264+sc0" {
		t.Errorf("默认 GOP 250/25 不应触发整数秒（sc0 才是终审规则）·得到 %q/%q", v, r)
	}
	// keyint=60/min=60 @30fps = 真 2 秒对齐（LiTV Clash 形态）→ 整数秒官方
	mi2 := strings.Replace(mi, "keyint=250 / keyint-min=25", "keyint=60 / keyint-min=60", 1)
	mi2 = strings.Replace(mi2, "25.000 FPS", "30.000 FPS", 1)
	mi2 = strings.Replace(mi2, "scenecut=0 / ", "", 1)
	if v, r := ExtractWEBdlVerdict(ParseMISections(mi2)); v != WEBVerdictOfficial || r != "交叉:x264+整数秒keyint" {
		t.Errorf("keyint=60@30fps 应整数秒官方·得到 %q/%q", v, r)
	}
}

// TestWEBdlIsEncodeExemption §59.517: IsEncode WEB 族豁免——平台 x264 流
// MIEncoded=true（WL 在场）但引擎判 official → 非 Encode；civilian/非 WEB
// 族语义维持。
func TestWEBdlIsEncodeExemption(t *testing.T) {
	mi := webMI(webMIOpts{x264WL: true, sc0: true, stitchable: true, keyint: "60",
		fps: "30.000 FPS", bitDepth: "8 bits", width: "1 920 pixels", height: "1 080 pixels"})
	mi = strings.Replace(mi, "Format                                   : HEVC", "Format                                   : AVC", 1)
	tp := BuildTechProfile("Clash 2024 1080p AMZN WEB-DL H264 DDP5.1-GRP", mi, "", "", "", "")
	if tp.Specification != "WEB-DL" {
		t.Fatalf("前置: Specification=%q（应 WEB-DL）", tp.Specification)
	}
	if !tp.MIEncoded {
		t.Fatalf("前置: MIEncoded 应 true（x264 WL 在场——原始信号）")
	}
	if tp.MIWebVerdict != WEBVerdictOfficial || tp.MIWebRule != "交叉:x264+stitchable" {
		t.Fatalf("引擎裁决=%q/%q（期望 official/交叉:x264+stitchable）", tp.MIWebVerdict, tp.MIWebRule)
	}
	if IsEncode(tp) {
		t.Errorf("平台 x264 WEB-DL（引擎 official）IsEncode 应 false")
	}
	// 对照：民压裁决（civilian）不豁免
	tp2 := tp
	tp2.MIWebVerdict = WEBVerdictCivilian
	tp2.Specification = "WEBRip"
	if !IsEncode(tp2) {
		t.Errorf("civilian 裁决 IsEncode 语义应维持")
	}
	// 对照：非 WEB 族（BluRay 语境）不豁免——Encoded 语义不变
	tp3 := tp
	tp3.Specification = "BluRay"
	if !IsEncode(tp3) {
		t.Errorf("非 WEB 族 MIEncoded 语义应不变（IsEncode=true）")
	}
}

// TestWEBdlCivilianEncoderBlacklist §59.517 追记：Video WL 民用编码器
// 黑名单纵深——裸 Lavc（FFmpeg 直调）/MainConcept/DivX/XviD；顺序锚
// （hevc_nvenc 形态由 GPU 规则先消费）。
func TestWEBdlCivilianEncoderBlacklist(t *testing.T) {
	base := "General\nWriting application: mkvmerge v94.0 64-bit\nWriting library: libebml v1.4.5\n\n" +
		"Video\nFormat: AVC\nWriting library: %s\nEncoding settings: crf=18\nBit depth: 8 bits\nFrame rate: 25.000 FPS\n"
	cases := []struct {
		name, wl, rule string
	}{
		{"裸Lavc直调", "Lavc61.9.100", "铁证①FFmpeg直调"},
		{"nvenc先GPU消费", "Lavc61.31.101 hevc_nvenc", "铁证①GPU硬编码"},
		{"MainConcept", "MainConcept HEVC Encoder 16.20", "铁证①民用编码器"},
		{"XviD", "XviD", "铁证①民用编码器"},
	}
	for _, c := range cases {
		v, r := ExtractWEBdlVerdict(ParseMISections(fmt1(base, c.wl)))
		if v != WEBVerdictCivilian || r != c.rule {
			t.Errorf("[%s] 判定=%q 规则=%q（期望 civilian/%q）", c.name, v, r, c.rule)
		}
	}
}

func fmt1(format string, arg string) string {
	return strings.Replace(format, "%s", arg, 1)
}
