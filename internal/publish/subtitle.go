package publish

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
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

func (d *SubtitleDetector) SelectBestChinese(candidates []SubtitleCandidate) (int, string) {
	var bestASS, bestSRT, bestPGS *SubtitleCandidate

	for i := range candidates {
		c := &candidates[i]
		switch {
		case c.Codec == "ass" && c.Score > 0:
			if bestASS == nil || c.Score > bestASS.Score {
				bestASS = c
			}
		case c.IsText && c.Score > 0:
			// §59.291: 文本轨统一 SRT 槽（subrip/webvtt/mov_text/未知 codec
			// 兜底候选——纯文本渲染同质，ass 优先级不变）
			if bestSRT == nil || c.Score > bestSRT.Score {
				bestSRT = c
			}
		case !c.IsText && c.Score > 0:
			if bestPGS == nil || c.Score > bestPGS.Score {
				bestPGS = c
			}
		}
	}

	switch {
	case bestASS != nil:
		return bestASS.StreamIndex, bestASS.Codec
	case bestSRT != nil:
		return bestSRT.StreamIndex, bestSRT.Codec
	case bestPGS != nil:
		return bestPGS.StreamIndex, bestPGS.Codec
	}
	// §59.298: 无标记 PGS 双兜底——蓝光原盘结构性缺失（m2ts 容器不存 language/
	// title/disposition，Under Current 2026 双 PGS 全 0 分实证）
	//   A. 副标题中字特征词（简/繁/中字/双语字幕）+ 原盘轨序约定（首 PGS 常中字）→ 首个 PGS
	//   B. 纯兜底：唯一/首个 PGS 轨（宁英文不裸图）——比无字幕好（人工纠偏见 C 层 tab3）
	var pgs []*SubtitleCandidate
	for i := range candidates {
		if !candidates[i].IsText {
			pgs = append(pgs, &candidates[i])
		}
	}
	if len(pgs) > 0 {
		return pgs[0].StreamIndex, pgs[0].Codec
	}
	return 0, ""
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
