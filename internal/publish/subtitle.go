package publish

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

type SubtitleCandidate struct {
	StreamIndex int
	Codec       string
	Language    string
	Title       string
	IsText      bool
	Score       int
}

type SubtitleDetector struct {
	ffprobePath string
	logger      *zap.Logger
}

func NewSubtitleDetector(logger *zap.Logger) *SubtitleDetector {
	return &SubtitleDetector{
		ffprobePath: "ffprobe",
		logger:      logger,
	}
}

func (d *SubtitleDetector) Available() bool {
	_, err := exec.LookPath(d.ffprobePath)
	return err == nil
}

func (d *SubtitleDetector) Detect(ctx context.Context, videoPath string) ([]SubtitleCandidate, error) {
	if !d.Available() {
		return nil, fmt.Errorf("ffprobe not found")
	}

	cmd := exec.CommandContext(ctx, d.ffprobePath, //nolint:gosec // intentional subprocess
		"-v", "error",
		"-select_streams", "s",
		"-show_entries", "stream=index,codec_name:stream_tags=language,title:stream_disposition=comment,hearing_impaired,visual_impaired",
		"-of", "json",
		videoPath,
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe subtitle detection: %w", err)
	}

	var result struct {
		Streams []struct {
			Index     int    `json:"index"`
			CodecName string `json:"codec_name"`
			Tags      struct {
				Language string `json:"language"`
				Title    string `json:"title"`
			} `json:"tags"`
			Disposition map[string]interface{} `json:"disposition"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("parse ffprobe output: %w", err)
	}

	var candidates []SubtitleCandidate
	// §59.291: webvtt（NF/AMZN WEB-DL MKV 内嵌标配——UNABOMBER 中文简繁
	// 双轨全 WEBVTT 实证）+ mov_text（MP4 内嵌）补入——原白名单致 Detect
	// 全滤 → sid=0 → --sid=no 裸图
	textCodecs := map[string]bool{"ass": true, "subrip": true, "srt": true, "ssa": true, "webvtt": true, "mov_text": true}
	graphicCodecs := map[string]bool{
		"hdmv_pgs_subtitle": true, "pgssub": true,
		"dvd_subtitle": true, "dvdsub": true,
		"dvb_subtitle": true, "dvbsub": true,
	}

	for _, stream := range result.Streams {
		codec := strings.ToLower(stream.CodecName)
		lang := strings.ToLower(stream.Tags.Language)
		title := strings.ToLower(stream.Tags.Title)
		disposition := stream.Disposition

		if getDispositionFlag(disposition, "comment") ||
			getDispositionFlag(disposition, "hearing_impaired") ||
			getDispositionFlag(disposition, "visual_impaired") {
			continue
		}

		_, isText := textCodecs[codec]
		_, isGraphic := graphicCodecs[codec]
		if !isText && !isGraphic {
			// §59.291 附: codec unknown 兜底——MKV 内嵌 WEBVTT 的 CodecID
			// （W_WEBVTT）ffprobe 报 codec_name 缺省（UNABOMBER 实证：三轨
			// chi/中文简繁全 unknown 被白名单滤掉 → sid=0 裸图）。判据反转：
			// 有语言/标题标记的字幕流（-select_streams s 已保证是字幕流）
			// 按文本候选兜底——mpv 渲染兼容 webvtt
			if codec == "" || codec == "unknown" {
				if lang != "" || title != "" {
					isText = true
				} else {
					continue
				}
			} else {
				continue
			}
		}

		score := 0
		if lang == "chi" || lang == "zho" || lang == "zh" {
			score += 10
		}

		switch {
		case containsAny(title, "简", "chs", "sc", "simplified"):
			score += 5
		case containsAny(title, "繁", "cht", "tc", "traditional"):
			score += 3
		case containsAny(title, "中", "chinese"):
			score += 2
		}

		candidates = append(candidates, SubtitleCandidate{
			StreamIndex: stream.Index,
			Codec:       codec,
			Language:    lang,
			Title:       stream.Tags.Title,
			IsText:      isText,
			Score:       score,
		})
	}

	return candidates, nil
}

// PickSubtitleSidFromBDInfo §59.319 附十四：从 BDInfo 报告的 Subtitle 行
// 序+语言直接推 mpv sid（原盘场景——m2ts PGS 语言在 CLPI/MPLS 不在容器
// 流，ffprobe/mpv 均读不到，ffprobe 自动选轨失效源）。
// 行序≈m2ts 流序（PMT/PID 同源——Under Current 实证：报告 English 行 1/
// Chinese 行 2 与 mpv --sid=1/2 一一对应）。
// 三级优先与 SelectBestChinese 同语义：简中>繁中>无中文（含无显式语言）
// 按行序第一轨。非报告形态/无 Subtitle 行返回 0（调用方走原自动链）。
var bdSubtitleLineRe = regexp.MustCompile(`^(\* )?Subtitle: ([^/]+?)(?:\s*\/|$)`)

func PickSubtitleSidFromBDInfo(report string) int {
	subs := []int{}
	for _, line := range strings.Split(report, "\n") {
		m := bdSubtitleLineRe.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m == nil {
			continue
		}
		// BDInfo Subtitle 行无 title 槽——语言即唯一信号
		subs = append(subs, chineseTier(strings.ToLower(strings.TrimSpace(m[2])), ""))
	}
	if len(subs) == 0 {
		return 0
	}
	best, bestTier := 0, -1
	for i, tier := range subs {
		if tier > bestTier {
			best, bestTier = i+1, tier // 1-based = mpv sid
		}
	}
	if bestTier == 0 {
		return 1 // 无中文：按行序第一轨
	}
	return best
}

// chineseTier §59.319 附十：字幕轨语言层级（用户定案三级优先）——
// 简中(3) > 繁中(2) > 无中文(0)。
// 繁中标记先判（防宽松词 "zh" 误吞 zh-hant）；不分简繁的中文
// （chi/zho/zh/chinese/标题"中"）默认归简体级（中文即中，尽量烧上）。
func chineseTier(lang, title string) int {
	if containsAny(lang, "zh-hant", "zh-tw", "zh-hk", "cht") {
		return 2
	}
	if containsAny(title, "繁", "cht", "traditional") {
		return 2
	}
	if containsAny(lang, "zh-hans", "zh-cn", "chs", "chi", "zho", "zh", "chinese") {
		return 3
	}
	if containsAny(title, "简", "chs", "simplified") || containsAny(title, "中", "chinese") {
		return 3
	}
	return 0
}

func (d *SubtitleDetector) SelectBestChinese(candidates []SubtitleCandidate) (int, string) {
	// §59.319 附十：主序=语言 tier（简中>繁中），次序=codec（ass>文本>图形），
	// 同分保序（ffprobe 流序）。tier 全 0（无中文或无显式语言）→ 按顺序第一
	// 字幕轨（宁英文不裸图——§59.298 PGS 兜底语义扩展到文本轨）。
	best := -1
	bestTier, bestRank := -1, -1
	codecRank := func(c *SubtitleCandidate) int {
		if c.Codec == "ass" {
			return 2
		}
		if c.IsText {
			return 1
		}
		return 0
	}
	for i := range candidates {
		tier := chineseTier(strings.ToLower(candidates[i].Language), strings.ToLower(candidates[i].Title))
		rank := codecRank(&candidates[i])
		if tier > bestTier || (tier == bestTier && rank > bestRank) {
			best, bestTier, bestRank = i, tier, rank
		}
	}
	if best < 0 {
		return 0, ""
	}
	if bestTier == 0 {
		// 无中文：按顺序第一字幕轨（非 codec 优先）
		return candidates[0].StreamIndex, candidates[0].Codec
	}
	return candidates[best].StreamIndex, candidates[best].Codec
}

func (d *SubtitleDetector) FindSubtitleStreamID(ctx context.Context, videoPath string) (int, error) {
	candidates, err := d.Detect(ctx, videoPath)
	if err != nil {
		return 0, err
	}

	streamID, _ := d.SelectBestChinese(candidates)
	if streamID == 0 {
		return 0, nil
	}

	sid := 1
	for _, c := range candidates {
		if c.StreamIndex == streamID {
			return sid, nil
		}
		sid++
	}
	return 1, nil
}

func getDispositionFlag(d map[string]interface{}, key string) bool {
	if v, ok := d[key]; ok {
		switch val := v.(type) {
		case float64:
			return val != 0
		case string:
			b, _ := strconv.ParseBool(val)
			return b
		case bool:
			return val
		}
	}
	return false
}

func containsAny(s string, keywords ...string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}
