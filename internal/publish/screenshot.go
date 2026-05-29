package publish

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

type ScreenshotEngine struct {
	mpvPath     string
	ffmpegPath  string
	ffprobePath string
	count       int
	minInterval float64
	quality     int
	logger      *zap.Logger
}

func NewScreenshotEngine(mpvPath string, count int, minInterval int, quality int, logger *zap.Logger) *ScreenshotEngine {
	if mpvPath == "" {
		mpvPath = "mpv"
	}
	if count <= 0 {
		count = 5
	}
	if minInterval <= 0 {
		minInterval = 30
	}
	if quality <= 0 {
		quality = 85
	}
	return &ScreenshotEngine{
		mpvPath:     mpvPath,
		ffmpegPath:  "ffmpeg",
		ffprobePath: "ffprobe",
		count:       count,
		minInterval: float64(minInterval),
		quality:     quality,
		logger:      logger,
	}
}

func (e *ScreenshotEngine) Available() bool {
	_, mpvErr := exec.LookPath(e.mpvPath)
	_, ffErr := exec.LookPath(e.ffmpegPath)
	if mpvErr != nil && ffErr != nil {
		return false
	}
	_, probeErr := exec.LookPath(e.ffprobePath)
	return probeErr == nil
}

type videoInfo struct {
	duration float64
	isHDR    bool
}

func (e *ScreenshotEngine) Capture(ctx context.Context, videoPath string, subtitleStreamID int) ([]string, error) {
	if !e.Available() {
		return nil, fmt.Errorf("screenshot tools not available (need ffprobe + mpv or ffmpeg)")
	}

	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("video file not found: %s", videoPath)
	}

	info, err := e.probeVideo(ctx, videoPath)
	if err != nil {
		return nil, fmt.Errorf("probe video: %w", err)
	}

	points := e.generateTimePoints(info.duration)
	tmpDir, err := os.MkdirTemp("", "pt-screenshot-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	useFFmpeg := info.isHDR && subtitleStreamID <= 0 && e.ffmpegAvailable()

	var paths []string
	for i, ts := range points {
		outPath := filepath.Join(tmpDir, fmt.Sprintf("shot_%03d.jpg", i))
		var capErr error
		if useFFmpeg {
			capErr = e.captureFrameFFmpeg(ctx, videoPath, ts, outPath)
		} else {
			capErr = e.captureFrameMPV(ctx, videoPath, ts, subtitleStreamID, outPath)
		}
		if capErr != nil {
			engine := "mpv"
			if useFFmpeg {
				engine = "ffmpeg"
			}
			e.logger.Warn("screenshot capture failed",
				zap.String("engine", engine),
				zap.Float64("timestamp", ts),
				zap.Error(capErr))
			continue
		}
		paths = append(paths, outPath)
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("all screenshot captures failed")
	}
	return paths, nil
}

func (e *ScreenshotEngine) ffmpegAvailable() bool {
	_, err := exec.LookPath(e.ffmpegPath)
	return err == nil
}

func (e *ScreenshotEngine) probeVideo(ctx context.Context, videoPath string) (*videoInfo, error) {
	probePath := e.ffprobePath
	if _, lookupErr := exec.LookPath(probePath); lookupErr != nil {
		return nil, fmt.Errorf("ffprobe not available: %w", lookupErr)
	}

	cmd := exec.CommandContext(ctx, probePath, //nolint:gosec // intentional subprocess
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=color_transfer,color_primaries:format=duration",
		"-of", "json",
		videoPath,
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe: %w", err)
	}

	var result struct {
		Streams []struct {
			ColorTransfer  string `json:"color_transfer"`
			ColorPrimaries string `json:"color_primaries"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("parse ffprobe output: %w", err)
	}

	d := 0.0
	if result.Format.Duration != "" {
		d, _ = strconv.ParseFloat(strings.TrimSpace(result.Format.Duration), 64)
	}
	if d <= 0 {
		cmd2 := exec.CommandContext(ctx, probePath, //nolint:gosec // intentional subprocess
			"-v", "error",
			"-show_entries", "format=duration",
			"-of", "default=noprint_wrappers=1:nokey=1",
			videoPath,
		)
		out2, err2 := cmd2.Output()
		if err2 == nil {
			d, _ = strconv.ParseFloat(strings.TrimSpace(string(out2)), 64)
		}
	}
	if d <= 0 {
		return nil, fmt.Errorf("invalid duration")
	}

	isHDR := false
	if len(result.Streams) > 0 {
		trc := strings.ToLower(result.Streams[0].ColorTransfer)
		isHDR = trc == "smpte2084" || trc == "arib-std-b67" || trc == "pq" || trc == "hlg"
	}

	return &videoInfo{duration: d, isHDR: isHDR}, nil
}

func (e *ScreenshotEngine) generateTimePoints(duration float64) []float64 {
	goldenStart := duration * 0.30
	goldenEnd := duration * 0.80
	span := goldenEnd - goldenStart

	interval := span / float64(e.count)
	if interval < e.minInterval {
		interval = e.minInterval
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec // non-crypto random is fine for screenshot timestamps
	points := make([]float64, 0, e.count)
	current := goldenStart

	for i := 0; i < e.count; i++ {
		offset := 0.0
		if interval > 0 {
			offset = rng.Float64() * interval * 0.5
		}
		pt := current + offset
		if pt > goldenEnd {
			pt = goldenEnd
		}
		if pt < 0 {
			pt = 0
		}
		points = append(points, pt)
		current += interval
	}

	return points
}

func (e *ScreenshotEngine) captureFrameFFmpeg(ctx context.Context, videoPath string, timestamp float64, outPath string) error {
	args := []string{
		"-ss", strconv.FormatFloat(timestamp, 'f', 3, 64),
		"-i", videoPath,
		"-frames:v", "1",
		"-vf", "zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv,format=yuv420p",
		"-q:v", strconv.Itoa(2 + (100-e.quality)/10),
		"-y", outPath,
	}

	cmd := exec.CommandContext(ctx, e.ffmpegPath, args...) //nolint:gosec // intentional subprocess
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg exited: %w, output: %s", err, string(output))
	}

	if _, statErr := os.Stat(outPath); statErr != nil {
		return fmt.Errorf("ffmpeg output file not found: %s", outPath)
	}
	return nil
}

func (e *ScreenshotEngine) captureFrameMPV(ctx context.Context, videoPath string, timestamp float64, subtitleStreamID int, outPath string) error {
	outDir := filepath.Dir(outPath)
	args := []string{
		"--vo=image",
		"--ao=null",
		"--no-audio",
		"--start=" + strconv.FormatFloat(timestamp, 'f', 3, 64),
		"--frames=1",
		"--no-terminal",
		"--no-config",
		"--vo-image-format=jpg",
		"--vo-image-jpeg-quality=" + strconv.Itoa(e.quality),
		"--vo-image-outdir=" + outDir,
	}

	if subtitleStreamID > 0 {
		args = append(args,
			"--sid="+strconv.Itoa(subtitleStreamID),
			"--sub-visibility=yes",
			"--blend-subtitles=yes",
		)
	} else {
		args = append(args, "--sid=no")
	}

	args = append(args, videoPath)

	cmd := exec.CommandContext(ctx, e.mpvPath, args...) //nolint:gosec // intentional subprocess
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mpv exited: %w, output: %s", err, string(output))
	}

	mpvOutput := filepath.Join(outDir, "00000001.jpg")
	if _, statErr := os.Stat(mpvOutput); statErr == nil {
		if mpvOutput != outPath {
			if renameErr := os.Rename(mpvOutput, outPath); renameErr != nil {
				return fmt.Errorf("rename output: %w", renameErr)
			}
		}
		return nil
	}

	return fmt.Errorf("mpv output file not found: %s", mpvOutput)
}
