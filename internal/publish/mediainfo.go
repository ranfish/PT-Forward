package publish

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

type MediaInfoResult struct {
	General   *MediaInfoTrack   `json:"general,omitempty"`
	Video     *MediaInfoTrack   `json:"video,omitempty"`
	Audio     []*MediaInfoTrack `json:"audio,omitempty"`
	Text      []*MediaInfoTrack `json:"text,omitempty"`
	RawOutput string            `json:"raw_output,omitempty"`
}

type MediaInfoTrack struct {
	Format             string `json:"format,omitempty"`
	CodecID            string `json:"codec_id,omitempty"`
	Duration           string `json:"duration,omitempty"`
	BitRate            string `json:"bit_rate,omitempty"`
	Width              string `json:"width,omitempty"`
	Height             string `json:"height,omitempty"`
	FrameRate          string `json:"frame_rate,omitempty"`
	DisplayAspectRatio string `json:"display_aspect_ratio,omitempty"`
	Language           string `json:"language,omitempty"`
	Channels           string `json:"channels,omitempty"`
	SamplingRate       string `json:"sampling_rate,omitempty"`
	Title              string `json:"title,omitempty"`
	FileSize           string `json:"file_size,omitempty"`
	OverallBitRate     string `json:"overall_bit_rate,omitempty"`
}

type MediaInfoAnalyzer struct {
	mediainfoPath string
	logger        *zap.Logger
}

func NewMediaInfoAnalyzer(logger *zap.Logger) *MediaInfoAnalyzer {
	return &MediaInfoAnalyzer{
		mediainfoPath: "mediainfo",
		logger:        logger,
	}
}

func (a *MediaInfoAnalyzer) Available() bool {
	_, err := exec.LookPath(a.mediainfoPath)
	return err == nil
}

func (a *MediaInfoAnalyzer) Analyze(ctx context.Context, filePath string) (*MediaInfoResult, error) {
	if !a.Available() {
		return nil, fmt.Errorf("mediainfo not found")
	}

	cmd := exec.CommandContext(ctx, a.mediainfoPath, "--Output=JSON", filePath) //nolint:gosec // intentional subprocess
	cmd.Env = mediainfoEnv()
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("mediainfo execution: %w", err)
	}

	result := &MediaInfoResult{RawOutput: string(output)}

	var miJSON struct {
		MediaInfo struct {
			Track []map[string]interface{} `json:"track"`
		} `json:"media"`
	}

	if err := json.Unmarshal(output, &miJSON); err != nil {
		rawText, textErr := a.getRawText(ctx, filePath)
		if textErr != nil {
			return nil, fmt.Errorf("parse mediainfo JSON: %w", textErr)
		}
		result.RawOutput = rawText
		return result, nil
	}

	// media 为 null（mediainfo 无法读取文件）→ 尝试 raw text 输出
	if len(miJSON.MediaInfo.Track) == 0 {
		a.logger.Warn("mediainfo JSON returned empty tracks, falling back to raw text",
			zap.String("path", filePath))
		rawText, textErr := a.getRawText(ctx, filePath)
		if textErr != nil {
			return result, nil // 返回空 result（保持向后兼容）
		}
		result.RawOutput = rawText
		return result, nil
	}

	for _, track := range miJSON.MediaInfo.Track {
		trackType, _ := track["@type"].(string)
		switch strings.ToLower(trackType) {
		case "general":
			result.General = parseTrack(track)
		case "video":
			result.Video = parseTrack(track)
		case "audio":
			if result.Audio == nil {
				result.Audio = []*MediaInfoTrack{}
			}
			result.Audio = append(result.Audio, parseTrack(track))
		case "text":
			if result.Text == nil {
				result.Text = []*MediaInfoTrack{}
			}
			result.Text = append(result.Text, parseTrack(track))
		}
	}

	return result, nil
}

// mediainfoEnv 返回调用 mediainfo 用的环境变量。
// mediainfo 25.04 在 POSIX/C locale 下对非 ASCII 路径（中文/方括号）静默失败（media:null）。
// 设置 LC_ALL=C.UTF-8 让 mediainfo 正确处理 UTF-8 文件名。
func mediainfoEnv() []string {
	envs := []string{"LC_ALL=C.UTF-8", "LANG=C.UTF-8"}
	for _, kv := range execEnv() {
		if strings.HasPrefix(kv, "LC_ALL=") || strings.HasPrefix(kv, "LANG=") {
			continue
		}
		envs = append(envs, kv)
	}
	return envs
}

func (a *MediaInfoAnalyzer) GetRawText(ctx context.Context, filePath string) (string, error) {
	return a.getRawText(ctx, filePath)
}

func (a *MediaInfoAnalyzer) getRawText(ctx context.Context, filePath string) (string, error) {
	cmd := exec.CommandContext(ctx, a.mediainfoPath, filePath) //nolint:gosec // intentional subprocess
	cmd.Env = mediainfoEnv()
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("mediainfo raw: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// execEnv 返回当前进程环境变量（不含 LC_ALL/LANG，由 mediainfoEnv 重设）。
func execEnv() []string {
	return append([]string(nil), os.Environ()...)
}

func parseTrack(track map[string]interface{}) *MediaInfoTrack {
	t := &MediaInfoTrack{}
	if v, ok := track["Format"].(string); ok {
		t.Format = v
	}
	if v, ok := track["CodecID"].(string); ok {
		t.CodecID = v
	}
	if v, ok := track["Duration"].(string); ok {
		t.Duration = v
	}
	if v, ok := track["BitRate"].(string); ok {
		t.BitRate = v
	}
	if v, ok := track["Width"].(string); ok {
		t.Width = v
	}
	if v, ok := track["Height"].(string); ok {
		t.Height = v
	}
	if v, ok := track["FrameRate"].(string); ok {
		t.FrameRate = v
	}
	if v, ok := track["DisplayAspectRatio"].(string); ok {
		t.DisplayAspectRatio = v
	}
	if v, ok := track["Language"].(string); ok {
		t.Language = v
	}
	if v, ok := track["Channels"].(string); ok {
		t.Channels = v
	}
	if v, ok := track["SamplingRate"].(string); ok {
		t.SamplingRate = v
	}
	if v, ok := track["Title"].(string); ok {
		t.Title = v
	}
	if v, ok := track["FileSize"].(string); ok {
		t.FileSize = v
	}
	if v, ok := track["OverallBitRate"].(string); ok {
		t.OverallBitRate = v
	}
	return t
}
