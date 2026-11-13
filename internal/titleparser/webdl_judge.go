package titleparser

import (
	"regexp"
	"strconv"
	"strings"
)

// §59.517: WEB-DL/WEBRip MI 判定引擎——设计定案的 Go 实施。
//
// 五公理：P1 不读文件名·不读组名·只读 MI 四段特征；P2 视频轨证据优先
// （字幕/音频/容器层信号只在视频层无定论时补刀）；P3 MI 在场→二值输出
// （无第三态）；P4 本质判据=视频流是否经二压（WEB-DL 生态本身是民间
// 解密+重封装·容器被民间经手不构成 WEBRip 证据）；P5 信号准入=全库
// 零反例+用户终审案例零错误（2379 颗全库验证·证伪信号见 §59.517 墓园）。
//
// 分层架构（顺序敏感）：
//   铁证层（单信号终审）→ 交叉层（组合连线·视频轨信号为主）→
//   中性公共尾部（AI字幕→古董混流器→辅助层→默认官方）
//
// 良性混合防御（AI 字幕补刀位）：官方视频+民间 AI 字幕混封 → 视频层
// 正面信号先触发 → official（Guardians 型活体）；AI 字幕只在视频层
// 零正面信号时定罪。

// WEB 族引擎裁决值（TechProfile.MIWebVerdict / MediaInfoTech.WEBVerdict）。
const (
	// WEBVerdictOfficial 官方（WEB-DL——视频流未二压）
	WEBVerdictOfficial = "official"
	// WEBVerdictCivilian 民间（WEBRip——视频流经二压）
	WEBVerdictCivilian = "civilian"
)

// ── 提取层正则（§59.517 三/四/五节规则配套·全部全库验证）──

var (
	// 铁证② DV Profile 5（dvhe.05/Profile 5——IPTPQc2 私有色彩空间·x265 无法生成）
	webDVP5RE = regexp.MustCompile(`(?i)dvhe\.05|Profile\s*5`)
	// 铁证③ 平台编码器（域名/专属编码器名/纯 IP 形态版本号）
	webPlatformDomainRE = regexp.MustCompile(`(?i)\.(com|net|org|cn|tv|cc)\b|tencent|qiyi|iqiyi|bilibili|lagranji|mgtv|kkstream`)
	webPlatformHeadRE   = regexp.MustCompile(`(?i)^(o264|ptw[\s-]|vision-264|volohevc|volo\d|arcvideo|avc coding|hevc coding)`)
	webVerNumIPRE       = regexp.MustCompile(`^\d+\.\d+\.\d+\.\d+$`)
	// 铁证①-GPU 显卡/硬件编码器（Video WL——民用硬件编码·平台用自家编码器）
	webHWEncRE = regexp.MustCompile(`(?i)nvenc|qsv\b|videotoolbox|amf\b`)
	// 铁证①-FFmpeg直调：Video 段裸 Lavc（区别于 General 段 Lavf 官方合并
	// 指纹——Sin Trade 官方变体实证；GPU 检查在前·hevc_nvenc 形态已消费）
	webBareLavcRE = regexp.MustCompile(`^lavc\d+\.\d+`)
	// 铁证①-民用编码器：民用剪辑软件内核/上古民用编码器（不可能出现在
	// 平台流——§59.517 追记·全库零命中纯纵深）
	webCivEncoderRE = regexp.MustCompile(`(?i)mainconcept|divx|xvid`)
	// 交叉: CDN 资产 ID（General Comment "vid:v[hex]"——西瓜视频等平台）
	webCDNVIDRE = regexp.MustCompile(`(?i)^vid:v[0-9a-f]+`)
	// 交叉: Source 洗版声明（Video Source 字段——发布组自留来源标注）
	webSourceDeclRE = regexp.MustCompile(`(?i)WEB|Blu|DVD|UHD|2160|1080|720`)
	// 交叉: 古董混流器/时代错位（mkvmerge 主版本——2015 组件 × 新编码时间）
	webMuxerVerRE = regexp.MustCompile(`(?i)mkvmerge v(\d+)\.`)
	// 尾部: AI 字幕（Text 段轨道 Title 的 AI 生成标记——平台永不分发）
	// 三形态全覆盖：AI（DS）/AI（DB）/(AI)
	webAISubRE = regexp.MustCompile(`AI[（(]|AI字幕|[_\s\[（(]AI\b`)
	// 铁证⑥ EIA 隐藏字幕（在场=官方·单向）
	webEIARE = regexp.MustCompile(`(?i)EIA-?608|EIA-?708`)
	// 铁证⑩ Menu 章节看点（DRM manifest 元数据——时间形态）
	webMenuTimeRE = regexp.MustCompile(`\d{2}:\d{2}:\d{2}`)
)

// webdlEvidence §59.517 提取层产物（内部——按引擎消费维度组织）。
type webdlEvidence struct {
	// General
	genReenc  bool // 铁证① 民用转码器黑名单（复用 generalReencoderDetected 词表）
	cdnVid    bool // 交叉: CDN 资产 ID
	oldMuxer  bool // 尾部: 古董混流器（mkvmerge v≤20）
	// Video
	fmtUpper    string // HEVC/AVC/AV1/VP9...
	vidWL       string // Video Writing library（小写）
	vidES       string // Video Encoding settings（归一空白）
	hwEncoder   bool   // 铁证①-GPU 显卡/硬件编码器
	x264, x265  bool
	x265Linux   bool
	x265Win     bool
	bd10, bd8   bool
	colorFull   bool // cp/tc/mc 三件套齐全
	crFull      bool // Color range: Full（DoVi 母带指纹）
	dvP5        bool
	dv81HDR10   bool
	p81Full     bool
	hdrVivid    bool
	vQuicktime  bool
	eia         bool
	sc, st      string // scenecut / stitchable
	ki, kim     int    // keyint / keyint-min
	integerSec  bool   // 整数秒 keyint（排除 x264 默认 GOP 250/25）
	gopLock     bool   // GOP 锁定（ki==kim）
	crfDecimal  bool   // crf 非零小数（".0" 不算——NF 官方 16.0）
	tightVBV    bool
	crudeParams bool
	bitrateRound bool
	cropped     bool // 仅作 x264 组合腿（单独信号已被证伪——§59.517 墓园）
	hasFS       bool // Format settings 行在场（AVC 平台封装）
	sourceDecl  bool
	fps         float64
	// Text
	aiSub bool
	webvtt bool
	sdh    bool
	textMany bool
	// Audio
	multiTrackUniform bool
	// Menu
	menuChapters bool
}

// ExtractWEBdlVerdict §59.517 判定引擎：MI 五层 → (verdict, rule)。
//
// verdict 空 = MI 无 Video 段（B1 契约：不判定·维持 name——二值原则的
// 适用边界）。rule 为命中规则名（仲裁 note/DryRun 透出）。
func ExtractWEBdlVerdict(s MISections) (string, string) {
	if len(s.Videos) == 0 || s.Videos[0] == nil {
		return "", ""
	}
	ev := extractWEBdlEvidence(s, s.Videos[0])
	return judgeWEBDL(ev)
}

// extractWEBdlEvidence 从 MI 五层提取引擎消费特征。
func extractWEBdlEvidence(s MISections, v map[string]string) webdlEvidence {
	var ev webdlEvidence
	// ── General ──
	ev.genReenc = generalReencoderDetected(s.General)
	if c := s.General["comment"]; webCDNVIDRE.MatchString(strings.TrimSpace(c)) {
		ev.cdnVid = true
	}
	if m := webMuxerVerRE.FindStringSubmatch(s.General["writing application"]); m != nil {
		if ver, err := strconv.Atoi(m[1]); err == nil && ver <= 20 {
			ev.oldMuxer = true
		}
	}

	// ── Video ──
	ev.fmtUpper = strings.ToUpper(strings.TrimSpace(v["format"]))
	ev.vidWL = strings.ToLower(strings.TrimSpace(v["writing library"]))
	ev.vidES = strings.Join(strings.Fields(v["encoding settings"]), " ")
	ev.hwEncoder = webHWEncRE.MatchString(ev.vidWL)
	ev.x264 = strings.Contains(ev.vidWL, "x264")
	ev.x265 = strings.Contains(ev.vidWL, "x265")
	ev.x265Linux = ev.x265 && strings.Contains(ev.vidWL, "linux")
	ev.x265Win = ev.x265 && (strings.Contains(ev.vidWL, "windows") || strings.Contains(ev.vidWL, "[win"))

	bd := strings.TrimSpace(v["bit depth"])
	ev.bd10 = strings.Contains(bd, "10")
	ev.bd8 = strings.Contains(bd, "8")

	ev.colorFull = strings.TrimSpace(v["color primaries"]) != "" &&
		strings.TrimSpace(v["transfer characteristics"]) != "" &&
		strings.TrimSpace(v["matrix coefficients"]) != ""
	ev.crFull = strings.EqualFold(strings.TrimSpace(v["color range"]), "Full")

	hdr := v["hdr format"]
	ev.dvP5 = webDVP5RE.MatchString(hdr)
	ev.dv81HDR10 = strings.Contains(hdr, "Profile 8.1") && strings.Contains(strings.ToUpper(hdr), "HDR10")
	ev.p81Full = strings.Contains(hdr, "Profile 8.1") && ev.crFull
	ev.hdrVivid = strings.Contains(strings.ToUpper(hdr), "HDR VIVID")
	ev.vQuicktime = strings.Contains(strings.ToUpper(v["codec id"]), "QUICKTIME")

	// EIA：Video 段任意字段（键或值）含 EIA-608/708
	for k, val := range v {
		if webEIARE.MatchString(k) || webEIARE.MatchString(val) {
			ev.eia = true
			break
		}
	}

	ev.sc = webESParam(ev.vidES, "scenecut")
	ev.st = webESParam(ev.vidES, "stitchable")
	ev.ki = webESInt(ev.vidES, "keyint")
	ev.kim = webESInt(ev.vidES, "keyint-min")
	if ev.kim == 0 {
		ev.kim = webESInt(ev.vidES, "min-keyint")
	}
	ev.fps = webMIFps(v["frame rate"])
	// 整数秒 keyint：fps×keyint=整数秒·排除 x264 开源默认 GOP 对
	// 250/25（25fps 下恰好 10 秒的假对齐——Kanjoos 教训·§59.517 十一④）
	if ev.ki > 0 && ev.fps > 0 {
		sec := float64(ev.ki) / ev.fps
		r := mathRound(sec)
		ev.integerSec = absF(sec-r) < 0.01 && r >= 1 && !(ev.ki == 250 && ev.kim == 25)
	}
	ev.gopLock = ev.ki > 0 && ev.ki == ev.kim

	crf := webESParam(ev.vidES, "crf")
	if f, err := strconv.ParseFloat(crf, 64); err == nil && strings.Contains(crf, ".") {
		ev.crfDecimal = f != float64(int(f)) // 非零小数（16.0=False·20.2=True）
	}
	vbvMax := webESInt(ev.vidES, "vbv-maxrate")
	vbvBuf := webESInt(ev.vidES, "vbv-bufsize")
	ev.tightVBV = vbvBuf > 0 && vbvMax > 0 && vbvBuf <= vbvMax && vbvMax < 70000
	me := webESParam(ev.vidES, "me")
	subme := webESInt(ev.vidES, "subme")
	trellis := webESParam(ev.vidES, "trellis")
	ev.crudeParams = (me == "hex" || me == "dia") && subme > 0 && subme <= 3 && (trellis == "0" || trellis == "")
	ev.bitrateRound = func() bool {
		br := webESInt(ev.vidES, "bitrate")
		return br > 0 && br%500 == 0
	}()
	// cropped：仅作 x264 组合腿——高度对比标准满幅高度（容差 4px）
	w, h := parseMIInt(v["width"]), parseMIInt(v["height"])
	if stdH, ok := map[int]int{1920: 1080, 3840: 2160, 1280: 720}[w]; ok {
		ev.cropped = h > 0 && h < stdH-4
	}
	for k := range v {
		if strings.HasPrefix(k, "format settings") {
			ev.hasFS = true
			break
		}
	}
	ev.sourceDecl = webSourceDeclRE.MatchString(v["source"])

	// ── Text ──
	for _, t := range s.Texts {
		if t == nil {
			continue
		}
		title := t["title"]
		if webAISubRE.MatchString(title) {
			ev.aiSub = true
		}
		if strings.Contains(strings.ToUpper(t["format"]+t["codec id"]), "WEBVTT") {
			ev.webvtt = true
		}
		if strings.Contains(strings.ToUpper(title+t["description"]), "SDH") {
			ev.sdh = true
		}
	}
	ev.textMany = len(s.Texts) >= 10

	// ── Audio：多轨同码率（≥4 轨 ≤2 种码率——印度平台特征·辅助）──
	if len(s.Audios) >= 4 {
		distinct := map[string]bool{}
		for _, a := range s.Audios {
			if a != nil {
				distinct[strings.ReplaceAll(a["bit rate"], " ", "")] = true
			}
		}
		ev.multiTrackUniform = len(distinct) <= 2
	}

	// ── Menu：章节看点（时间形态）──
	for _, m := range s.Menus {
		if m == nil {
			continue
		}
		for _, val := range m {
			if webMenuTimeRE.MatchString(val) {
				ev.menuChapters = true
				break
			}
		}
	}
	return ev
}

// judgeWEBDL §59.517 分层判定（顺序敏感——Python v7 参照实现的 Go 移植）。
func judgeWEBDL(ev webdlEvidence) (string, string) {
	// ═══ 铁证层（单信号终审）═══
	if ev.genReenc {
		return WEBVerdictCivilian, "铁证①软件名"
	}
	if ev.hwEncoder {
		return WEBVerdictCivilian, "铁证①GPU硬编码"
	}
	// §59.517 追记（用户讨论吸收）：Video 段裸 Lavc = ffmpeg -c:v 直调
	// 痕迹（原厂封装工业编码头绝不留此名）。
	if webBareLavcRE.MatchString(ev.vidWL) {
		return WEBVerdictCivilian, "铁证①FFmpeg直调"
	}
	// §59.517 追记（用户讨论吸收）：民用编码器黑名单——MainConcept
	// （Premiere 等民用剪辑软件导出内核）/DivX/XviD（上古民用编码器）。
	if webCivEncoderRE.MatchString(ev.vidWL) {
		return WEBVerdictCivilian, "铁证①民用编码器"
	}
	if ev.dvP5 {
		return WEBVerdictOfficial, "铁证②DV P5"
	}
	if webPlatformDomainRE.MatchString(ev.vidWL) || webPlatformHeadRE.MatchString(ev.vidWL) ||
		webVerNumIPRE.MatchString(strings.TrimSpace(ev.vidWL)) {
		return WEBVerdictOfficial, "铁证③平台编码器"
	}
	if ev.dv81HDR10 || ev.p81Full {
		return WEBVerdictOfficial, "铁证④复合HDR"
	}
	if ev.vQuicktime {
		return WEBVerdictOfficial, "铁证⑦V_QUICKTIME"
	}
	if ev.hdrVivid {
		return WEBVerdictOfficial, "铁证⑧HDR Vivid"
	}
	if ev.eia {
		return WEBVerdictOfficial, "铁证⑥EIA"
	}
	if ev.menuChapters {
		return WEBVerdictOfficial, "铁证⑩Menu章节"
	}

	// ═══ 交叉层 ═══
	// AV1/VP9 默认官方（新编码·民用二压生态不存在——§59.517 B4）
	if (ev.fmtUpper == "AV1" || ev.fmtUpper == "VP9") && !ev.x265 {
		return WEBVerdictOfficial, "交叉:AV1默认官方"
	}
	// Source 洗版声明（发布组自留来源标注）
	if ev.sourceDecl {
		return WEBVerdictCivilian, "交叉:Source洗版声明"
	}
	// CDN 资产 ID
	if ev.cdnVid {
		return WEBVerdictOfficial, "交叉:CDN资产ID"
	}

	// ── x264 组合（负面在前·正面在后——§59.517 顺序）──
	if ev.x264 {
		sc40 := ev.sc == "40" || ev.sc == "60"
		if sc40 && ev.crfDecimal {
			return WEBVerdictCivilian, "交叉:x264+sc40+crf小数"
		}
		if sc40 && ev.cropped && ev.ki == 250 {
			return WEBVerdictCivilian, "交叉:x264+sc40+裁切+开源keyint"
		}
		if ev.crfDecimal {
			return WEBVerdictCivilian, "交叉:x264+crf小数"
		}
		if ev.st == "1" {
			return WEBVerdictOfficial, "交叉:x264+stitchable"
		}
		if ev.sc == "0" {
			return WEBVerdictOfficial, "交叉:x264+sc0"
		}
		if ev.integerSec {
			return WEBVerdictOfficial, "交叉:x264+整数秒keyint"
		}
		if ev.tightVBV && ev.crudeParams {
			return WEBVerdictOfficial, "交叉:x264+紧VBV+粗糙参数"
		}
		if ev.hasFS && ev.bitrateRound {
			return WEBVerdictOfficial, "交叉:x264+FS+整千码率"
		}
		if ev.hasFS {
			return WEBVerdictOfficial, "交叉:x264+FS"
		}
		// 无组合信号 → 视频层中性·落公共尾部
	}

	// ── x265 组合 ──
	if ev.x265 {
		if ev.sc == "40" && ev.crfDecimal {
			return WEBVerdictCivilian, "交叉:x265+sc40+crf小数"
		}
		if ev.x265Win {
			return WEBVerdictCivilian, "交叉:x265-Windows"
		}
		if ev.x265Linux {
			if ev.sc == "0" {
				return WEBVerdictOfficial, "交叉:x265Linux+sc0"
			}
			if ev.gopLock || ev.integerSec {
				return WEBVerdictOfficial, "交叉:x265Linux+GOP锁定"
			}
			// sc40 无 crf → 视频层中性·落公共尾部
		} else {
			// 无平台标记的 x265 → 民间（平台用 x265 必带 Linux 构建串）
			return WEBVerdictCivilian, "交叉:x265"
		}
	}

	// ── HEVC 无 WL：色彩三态 ──
	if ev.fmtUpper == "HEVC" && ev.bd10 && ev.vidWL == "" {
		if ev.colorFull {
			return WEBVerdictOfficial, "交叉:HEVC10bit+色彩全"
		}
		// Color range: Full = DoVi 母带指纹（无 DV 标记形态——官方服务器
		// 处理高规格杜比视界母带才产出 Full 范围流·x265 民间压制强制
		// Limited·全库 94 颗 Full 全 WEB-DL 标题零 Rip）
		if ev.crFull {
			return WEBVerdictOfficial, "交叉:HEVC10bit+FullRange"
		}
		// 色彩缺 → 视频层中性·落公共尾部（官方流也会缺色彩参数）
	}
	if ev.fmtUpper == "HEVC" && ev.bd8 && ev.vidWL == "" {
		return WEBVerdictOfficial, "交叉:HEVC8bit无WL"
	}

	// ── AVC 无 WL + FS ──
	if ev.fmtUpper == "AVC" && ev.vidWL == "" && ev.hasFS {
		return WEBVerdictOfficial, "交叉:AVC无WL+FS"
	}

	// ── 通用负面 ──
	if ev.crfDecimal {
		return WEBVerdictCivilian, "交叉:CRF小数"
	}
	if ev.fps > 0 && ev.fps < 23 {
		return WEBVerdictCivilian, "交叉:帧率异常"
	}

	// ═══ 中性公共尾部（视频层无任何正面/负面终局信号时到达）═══
	// AI 字幕（Text 段行为指纹·补刀位——良性混合防御：视频层有任何
	// 正面官方信号时永远到不了这里）
	if ev.aiSub {
		return WEBVerdictCivilian, "交叉:AI字幕"
	}
	// 古董混流器/时代错位（2015 组件 × 新编码时间·全库 21/22 Rip）
	if ev.oldMuxer {
		return WEBVerdictCivilian, "交叉:古董混流器"
	}
	// 辅助层（50-70%·只补强）
	if ev.webvtt {
		return WEBVerdictOfficial, "辅助:WEBVTT"
	}
	if ev.textMany && ev.sdh {
		return WEBVerdictOfficial, "辅助:字幕SDH"
	}
	if ev.multiTrackUniform {
		return WEBVerdictOfficial, "辅助:多轨同码率"
	}
	if ev.fps >= 59.0 && ev.fps <= 60.1 && !ev.genReenc {
		return WEBVerdictOfficial, "辅助:60fps"
	}
	// 终局默认（二值原则·先验压倒性·误判方向安全）
	return WEBVerdictOfficial, "交叉:无信号默认官方"
}

// webESParam 提取 Encoding settings 的 key=value 参数
// （形态 "scenecut=40 / crf=20.2 / keyint=75"——精确 token 前缀匹配，
// keyint 与 keyint-min 不互扰）。
func webESParam(es, name string) string {
	for _, tok := range strings.Split(es, "/") {
		tok = strings.TrimSpace(tok)
		if strings.HasPrefix(tok, name+"=") {
			return strings.TrimSpace(tok[len(name)+1:])
		}
	}
	return ""
}

// webESInt 提取整型参数（非数字/缺席=0）。
func webESInt(es, name string) int {
	n, _ := strconv.Atoi(webESParam(es, name))
	return n
}

// webMIFps 解析帧率原值（"25.000 FPS"→25.0）。
func webMIFps(v string) float64 {
	f := strings.Fields(strings.TrimSpace(v))
	if len(f) == 0 {
		return 0
	}
	x, err := strconv.ParseFloat(f[0], 64)
	if err != nil {
		return 0
	}
	return x
}

// absF 绝对值（避免引入 math 包仅为一处 round/abs——引擎内联小函数）。
func absF(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// mathRound 就近取整（半值远离零——正数场景等价四舍五入）。
func mathRound(x float64) float64 {
	if x-float64(int(x)) >= 0.5 {
		return float64(int(x) + 1)
	}
	return float64(int(x))
}

// isWebSpecToken WEB 族规格形态（titleparser 侧单点——publish.isWebSpec
// 同语义·跨包不 import publish）。
func isWebSpecToken(s string) bool {
	return strings.EqualFold(s, "WEB-DL") || strings.EqualFold(s, "WEBRip")
}
