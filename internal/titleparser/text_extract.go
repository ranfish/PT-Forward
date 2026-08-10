// Package titleparser text_extract.go
// 移植 auto_feed_js 的 6 个字段提取函数（medium_sel/codec_sel/audiocodec_sel/standard_sel/get_type/source_sel）。
//
// 与 ParseTitle 的区别：
//   - ParseTitle：从 **标题** 严格解析 scene 命名组件（分辨率/季/集/年份等）
//   - ExtractXXX：从 **任意文本**（标题/简介/基本信息聚合文本）正则匹配字段值
//
// 返回值为中文识别结果（与 auto_feed_js 一致），上层用 extract.LookupStandardKey 标准化为 code。
// 例如：ExtractType("大小：18.53 GB  类型: 电视剧") → "电视剧"
//      上层 LookupStandardKey("type", "电视剧") → "category.tv_series"
//
// 数据来源：examples/auto_feed_js/auto_feed_2.1.1.2.js line 2469-2660
// 这些规则经过 auto_feed_js 长期实战验证，覆盖率高。
package titleparser

import (
	"regexp"
	"strings"
)

// ExtractMedium 从文本识别媒介（移植 auto_feed_js String.prototype.medium_sel）。
// name 参数为种子标题（用于排除 webrip 干扰），可为空。
// 返回值：WEB-DL/UHDTV/HDTV/Remux/Blu-ray/UHD/Encode/DVD/TV/VHS/CD/""。
func ExtractMedium(text, name string) string {
	if text == "" {
		return ""
	}
	if regexp.MustCompile(`(?i)Webdl|Web-dl|WEB[. ]`).MatchString(text) &&
		!regexp.MustCompile(`(?i)webrip`).MatchString(name) {
		return "WEB-DL"
	}
	if regexp.MustCompile(`(?i)UHDTV`).MatchString(text) {
		return "UHDTV"
	}
	if regexp.MustCompile(`(?i)HDTV`).MatchString(text) {
		return "HDTV"
	}
	if regexp.MustCompile(`(?i)Remux`).MatchString(text) &&
		!regexp.MustCompile(`(?i)Encode`).MatchString(text) {
		return "Remux"
	}
	if regexp.MustCompile(`(?i)Blu-ray|.MPLS|Bluray原盘`).MatchString(text) &&
		!regexp.MustCompile(`(?i)Encode`).MatchString(text) {
		return "Blu-ray"
	}
	// UHD 排除 Encode/BDRIP/webrip/BluRay/x264/x265/H264/H265
	uhdRe := regexp.MustCompile(`(?i)UHD|UltraHD`)
	excludeUhdRe := regexp.MustCompile(`(?i)Encode|BDRIP|webrip|BluRay|(x|H).?(264|265)`)
	if uhdRe.MatchString(text) && !excludeUhdRe.MatchString(text) {
		return "UHD"
	}
	if regexp.MustCompile(`(?i)Encode|BDRIP|webrip|BluRay`).MatchString(text) ||
		regexp.MustCompile(`(?i)(x|H).?(264|265)`).MatchString(text) {
		return "Encode"
	}
	if regexp.MustCompile(`(?i)DVDRip|DVD`).MatchString(text) {
		return "DVD"
	}
	if strings.Contains(text, "TV") {
		return "TV"
	}
	if strings.Contains(text, "VHS") {
		return "VHS"
	}
	if regexp.MustCompile(`格式: CD|媒介: CD`).MatchString(text) {
		return "CD"
	}
	return ""
}

// ExtractCodec 从文本识别视频编码（移植 auto_feed_js String.prototype.codec_sel）。
// 返回值：H264/H265/H266/X265/X264/VC-1/MPEG-2/MPEG-4/XVID/VP9/DIVX/""。
func ExtractCodec(text string) string {
	if text == "" {
		return ""
	}
	cases := []struct {
		re      *regexp.Regexp
		result  string
	}{
		{regexp.MustCompile(`(?i)H264|H\.264|AVC`), "H264"},
		{regexp.MustCompile(`(?i)HEVC|H265|H\.265`), "H265"},
		{regexp.MustCompile(`(?i)VVC|H266|H\.266`), "H266"},
		{regexp.MustCompile(`(?i)X265`), "X265"},
		{regexp.MustCompile(`(?i)X264`), "X264"},
		{regexp.MustCompile(`(?i)VC-1`), "VC-1"},
		{regexp.MustCompile(`(?i)MPEG-2`), "MPEG-2"},
		{regexp.MustCompile(`(?i)MPEG-4`), "MPEG-4"},
		{regexp.MustCompile(`(?i)XVID`), "XVID"},
		{regexp.MustCompile(`(?i)VP9`), "VP9"},
		{regexp.MustCompile(`(?i)DIVX`), "DIVX"},
	}
	for _, c := range cases {
		if c.re.MatchString(text) {
			return c.result
		}
	}
	return ""
}

// ExtractAudioCodec 从文本识别音频编码（移植 auto_feed_js String.prototype.audiocodec_sel）。
// 返回值：DTS-HDMA:X 7.1/DTS-HDMA/DTS-HDHR/DTS-HD/DTS-X/LPCM/OPUS/AC3/Atmos/AAC/TrueHD/DTS/Flac/APE/MP3/WAV/OGG/""。
func ExtractAudioCodec(text string) string {
	if text == "" {
		return ""
	}
	cases := []struct {
		re     *regexp.Regexp
		result string
	}{
		{regexp.MustCompile(`(?i)DTS-HDMA:X 7\.1|DTS.?X.?7\.1`), "DTS-HDMA:X 7.1"},
		{regexp.MustCompile(`(?i)DTS-HD.?MA`), "DTS-HDMA"},
		{regexp.MustCompile(`(?i)DTS-HD.?HR`), "DTS-HDHR"},
		{regexp.MustCompile(`(?i)DTS-HD`), "DTS-HD"},
		{regexp.MustCompile(`(?i)DTS.?X[^2]`), "DTS-X"},
		{regexp.MustCompile(`(?i)LPCM`), "LPCM"},
		{regexp.MustCompile(`(?i)OPUS`), "OPUS"},
		{regexp.MustCompile(`(?i)[ .]DD|AC3|AC-3|Dolby Digital`), "AC3"},
	}
	for _, c := range cases {
		if c.re.MatchString(text) {
			// Atmos 需要同时匹配 Atmos 和 TrueHD
			if c.result == "AC3" {
				if regexp.MustCompile(`(?i)Atmos`).MatchString(text) &&
					regexp.MustCompile(`(?i)True.?HD`).MatchString(text) {
					return "Atmos"
				}
			}
			return c.result
		}
	}
	// Atmos 单独判断（TrueHD Atmos）
	if regexp.MustCompile(`(?i)Atmos`).MatchString(text) &&
		regexp.MustCompile(`(?i)True.?HD`).MatchString(text) {
		return "Atmos"
	}
	restCases := []struct {
		re     *regexp.Regexp
		result string
	}{
		{regexp.MustCompile(`(?i)AAC`), "AAC"},
		{regexp.MustCompile(`(?i)TrueHD`), "TrueHD"},
		{regexp.MustCompile(`(?i)DTS`), "DTS"},
		{regexp.MustCompile(`(?i)Flac`), "Flac"},
		{regexp.MustCompile(`(?i)APE`), "APE"},
		{regexp.MustCompile(`(?i)MP3`), "MP3"},
		{regexp.MustCompile(`(?i)WAV`), "WAV"},
		{regexp.MustCompile(`(?i)OGG`), "OGG"},
	}
	for _, c := range restCases {
		if c.re.MatchString(text) {
			return c.result
		}
	}
	return ""
}

// ExtractResolution 从文本识别分辨率（移植 auto_feed_js String.prototype.standard_sel）。
// 返回值：8K/1080p/720p/1080i/SD/1440p/4K/""。
func ExtractResolution(text string) string {
	if text == "" {
		return ""
	}
	cases := []struct {
		re     *regexp.Regexp
		result string
	}{
		{regexp.MustCompile(`(?i)4320p|8k`), "8K"},
		{regexp.MustCompile(`(?i)1080p|2K`), "1080p"},
		{regexp.MustCompile(`(?i)720p`), "720p"},
		{regexp.MustCompile(`(?i)1080i`), "1080i"},
		{regexp.MustCompile(`(?i)576[pi]|480[pi]`), "SD"},
		{regexp.MustCompile(`(?i)1440p`), "1440p"},
		{regexp.MustCompile(`(?i)2160p|4k`), "4K"},
	}
	for _, c := range cases {
		if c.re.MatchString(text) {
			return c.result
		}
	}
	return ""
}

// ExtractSource 从文本识别产地（移植 auto_feed_js String.prototype.source_sel）。
// 返回值：大陆/港台/欧美/日韩/香港/台湾/日本/韩国/印度/""。
func ExtractSource(text string) string {
	if text == "" {
		return ""
	}
	cases := []struct {
		re     *regexp.Regexp
		result string
	}{
		{regexp.MustCompile(`(?i)大陆|China|中国|CN|chinese`), "大陆"},
		{regexp.MustCompile(`(?i)HK&TW|港台|thai`), "港台"},
		{regexp.MustCompile(`(?i)EU&US|欧美|US/EU|英美`), "欧美"},
		{regexp.MustCompile(`(?i)JP&KR|日韩|japanese|korean`), "日韩"},
		{regexp.MustCompile(`香港`), "香港"},
		{regexp.MustCompile(`台湾`), "台湾"},
		{regexp.MustCompile(`(?i)日本|JP`), "日本"},
		{regexp.MustCompile(`(?i)韩国|KR`), "韩国"},
		{regexp.MustCompile(`印度`), "印度"},
	}
	for _, c := range cases {
		if c.re.MatchString(text) {
			return c.result
		}
	}
	return ""
}
