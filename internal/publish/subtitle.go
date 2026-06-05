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
	textCodecs := map[string]bool{"ass": true, "subrip": true, "srt": true, "ssa": true}
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
			continue
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
		case c.Codec == "subrip" && c.Score > 0:
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
	default:
		return 0, ""
	}
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
