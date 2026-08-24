// Package publish MediaTag 推断器（§56.22 Q1 + §59.26 增强 + §59.35 P4 查表化）。
//
// 推断规则数据源：dict/tag.json（Layer 1 tag 域，canonical/infer_pattern/infer_scope）。
// 本文件只保留输入编排 + ApplyTagRules 后处理（覆盖/互斥规则）。
package publish

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/titleparser"
)

// MediaTagInferer 从多源文本推断 MediaTags。
type MediaTagInferer struct{}

// NewMediaTagInferer 创建推断器。
func NewMediaTagInferer() *MediaTagInferer {
	return &MediaTagInferer{}
}

// TagInput 推断输入（§59.26：对齐 auto_feed label_str = 副标题+标题+简介+MI+NFO）。
type TagInput struct {
	MediaInfo   string // MediaInfo 全文
	Title       string // 主标题
	Subtitle    string // 副标题
	Description string // 简介（BBCode）
	NFO         string // NFO/BDInfo
	Size        int64  // §59.72 B2: 种子体积（字节）——big_pack >1TB 判据（AGSV 审核硬规则）
	Statement   string // §59.73: 引用/声明区（源站 quote 块）——特效字幕等发布者声明
}

// Infer 从多源文本推断 MediaTags。
// 返回推断出的标准键列表（如 hdr10/dolby_vision/chinese_subtitle）。
func (i *MediaTagInferer) Infer(mediaInfoText, title string) []string {
	return i.InferFull(TagInput{MediaInfo: mediaInfoText, Title: title})
}

// InferFull 从完整输入推断 MediaTags（§59.35 P4：规则查 dict/tag.json）。
// 数据源对齐 auto_feed：副标题+主标题+简介+MI+NFO。
func (i *MediaTagInferer) InferFull(in TagInput) []string {
	tags := titleparser.TagInferMatches(titleparser.TagInferInput{
		Subtitle:    in.Subtitle,
		Title:       in.Title,
		Description: in.Description,
		MediaInfo:   in.MediaInfo,
		NFO:         in.NFO,
		Statement:   in.Statement,
	})
	// §59.69: 高码/高帧数值判据（regex 表达不了阈值比较，代码层判定）。
	// 高码: MI Overall bit rate(General 段,用户定案) ≥15Mb/s@宽度≥4K / ≥9Mb/s@宽度<4K
	// 高帧: MI Frame rate ≥60（59.940 NTSC 不算）
	// 仅视频类种子（无 Video 段解析不到宽度/帧率，自然不命中）
	hasHB, hasHF := inferNumericSpecTags(in.MediaInfo)
	// §59.70: 高分——豆瓣评分 ≥8.0（Description 源——PTGen 简介行，
	// "◎豆瓣评分　8.2/10"；无评分/暂无评分不命中）
	hasHR := parseDoubanRatingScore(in.Description) >= 8.0
	has := func(k string) bool {
		for _, t := range tags {
			if t == k {
				return true
			}
		}
		return false
	}
	if hasHB && !has("high_bitrate") {
		tags = append(tags, "high_bitrate")
	}
	if hasHF && !has("high_frame_rate") {
		tags = append(tags, "high_frame_rate")
	}
	if hasHR && !has("high_rating") {
		tags = append(tags, "high_rating")
	}
	// §59.72 B2: 组合判据——连载 = 分集 && 非完结 && 非合集
	//（ubits: 连载=分集资源; 词条 regex 表达不了依赖，代码层）
	if has("episode_split") && !has("complete") && !has("collection") && !has("ongoing") {
		tags = append(tags, "ongoing")
	}
	// §59.72 B2: 大包 = size > 1TB（AGSV 审核脚本硬规则，字面 >1T）
	if in.Size > 1024*1024*1024*1024 && !has("big_pack") {
		tags = append(tags, "big_pack")
	}
	return ApplyTagRules(dedupTags(tags))
}

// inferNumericSpecTags §59.69: 从 MI 文本解析码率/宽度/帧率做阈值判定。
// MI 数字含千分位空格（"3 840"），单位 Mb/s 或 kb/s。
func inferNumericSpecTags(mi string) (highBitrate, highFrameRate bool) {
	rate := parseMIOverallBitrateMbps(mi)
	width := parseMIWidthPixels(mi)
	fps := parseMIFrameRate(mi)

	if rate > 0 && width > 0 {
		if width >= 3840 { // ≥4K（2160p/4320p，DCI 4096 同覆盖）
			highBitrate = rate >= 15
		} else { // <4K（1080p 及以下）
			highBitrate = rate >= 9
		}
	}
	if fps >= 60 { // 59.940 NTSC 不算（用户定案：字面 ≥60）
		highFrameRate = true
	}
	return highBitrate, highFrameRate
}

var (
	miOverallBitrateRe = regexp.MustCompile(`(?i)Overall bit rate\s*:\s*([\d\s.,]+)\s*(Mb/s|kb/s|Gb/s)`)
	miWidthRe          = regexp.MustCompile(`(?im)^\s*Width\s*:\s*([\d\s]+)\s*pixels`)
	miFpsRe            = regexp.MustCompile(`(?im)^\s*Frame rate\s*:\s*([\d.]+)\s*FPS`)
)

// parseMIOverallBitrateMbps 解析 General 段 Overall bit rate，归一 Mb/s。
func parseMIOverallBitrateMbps(mi string) float64 {
	m := miOverallBitrateRe.FindStringSubmatch(mi)
	if m == nil {
		return 0
	}
	num, _ := strconv.ParseFloat(strings.ReplaceAll(strings.ReplaceAll(m[1], " ", ""), ",", ""), 64)
	if num == 0 {
		return 0
	}
	switch strings.ToLower(m[2]) {
	case "gb/s":
		return num * 1000
	case "kb/s":
		return num / 1000
	default:
		return num
	}
}

// parseMIWidthPixels 解析 Video 段 Width（像素，含千分位空格）。
func parseMIWidthPixels(mi string) int {
	m := miWidthRe.FindStringSubmatch(mi)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.ReplaceAll(m[1], " ", ""))
	return n
}

// doubanRatingLineRe §59.70: PTGen 豌豆评分行——全角/半角空格、"N.N"或"N.N/10" 形态。
// IMDb 行因前缀不同（◎IMDb评分）天然不匹配。
var doubanRatingLineRe = regexp.MustCompile(`◎豆瓣评分[\s\x{3000}]+([0-9]+(?:\.[0-9]+)?)(?:\s*/\s*10)?`)

// parseDoubanRatingScore 从简介文本解析豆瓣评分（无行/暂无评分返回 0）。
func parseDoubanRatingScore(desc string) float64 {
	m := doubanRatingLineRe.FindStringSubmatch(desc)
	if m == nil {
		return 0
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	return v
}

// parseMIFrameRate 解析 Video 段 Frame rate（取最大值——多段 MI 场景保守）。
func parseMIFrameRate(mi string) float64 {
	var max float64
	for _, m := range miFpsRe.FindAllStringSubmatch(mi, -1) {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil && v > max {
			max = v
		}
	}
	return max
}

// dedupTags 保序去重。
func dedupTags(tags []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range tags {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}
