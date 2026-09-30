package publish

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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
	if logger == nil {
		logger = zap.NewNop() // 测试构造容错（§59.300 附十五 hr-seek 兜底路径打日志）
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
		ffprobePath: "ffprobe",
		count:       count,
		minInterval: float64(minInterval),
		quality:     quality,
		logger:      logger,
	}
}

func (e *ScreenshotEngine) Available() bool {
	_, mpvErr := exec.LookPath(e.mpvPath)
	_, probeErr := exec.LookPath(e.ffprobePath)
	return mpvErr == nil && probeErr == nil
}

type videoInfo struct {
	duration float64
	isHDR    bool
	// dualHEVC: m2ts 内含两个 HEVC 视频流（DoVi Profile 7 BL+EL 结构）。
	// RPU 挂在 EL 流上，需 vf dovi_reshape 做 CPU reshaping/重标记；
	// 且 seek 落点首帧常为参考缺失的 concealment 帧，需 IDR 精确起点+前向取帧。
	dualHEVC bool
	fps      float64
	// doviProfile: 容器 DOVI configuration record 的 dv_profile（0=无）。
	// P5=IPT 原生需 dovi_reshape 反变换；P7/P8=BL 兼容 HDR10 直通即可（§59.300 附十四）
	doviProfile int
	// isMpegTS: m2ts/ts 无索引容器——mpv 落点不确定+open-GOP concealment，
	// 必须 IDR 精确慢链；mkv/mp4 精确 seek，frames=1 快链（发布链 4min 预算兼容，
	// §59.300 附十八回归审计）
	isMpegTS bool
}

func (e *ScreenshotEngine) Capture(ctx context.Context, videoPath string, subtitleStreamID int) ([]string, string, error) {
	if !e.Available() {
		return nil, "", fmt.Errorf("screenshot tools not available (need ffprobe + mpv)")
	}

	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return nil, "", fmt.Errorf("video file not found: %s", videoPath)
	}

	info, err := e.probeVideo(ctx, videoPath)
	if err != nil {
		return nil, "", fmt.Errorf("probe video: %w", err)
	}

	points := e.generateTimePoints(info.duration)
	tmpDir, err := os.MkdirTemp("", "pt-screenshot-*")
	if err != nil {
		return nil, "", fmt.Errorf("create temp dir: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	var paths []string
	for i, ts := range points {
		outPath := filepath.Join(tmpDir, fmt.Sprintf("shot_%03d.jpg", i))
		capErr := e.captureFrameMPVEx(ctx, videoPath, ts, subtitleStreamID, info.isHDR, info.isMpegTS, info.fps, info.doviProfile, outPath)
		if capErr != nil {
			e.logger.Warn("screenshot capture failed",
				zap.Float64("timestamp", ts),
				zap.Bool("hdr", info.isHDR),
				zap.Error(capErr))
			continue
		}
		paths = append(paths, outPath)
	}

	if len(paths) == 0 {
		return nil, "", fmt.Errorf("all screenshot captures failed")
	}
	return paths, tmpDir, nil
}

func (e *ScreenshotEngine) probeVideo(ctx context.Context, videoPath string) (*videoInfo, error) {
	probePath := e.ffprobePath
	if _, lookupErr := exec.LookPath(probePath); lookupErr != nil {
		return nil, fmt.Errorf("ffprobe not available: %w", lookupErr)
	}

	cmd := exec.CommandContext(ctx, probePath, //nolint:gosec // intentional subprocess
		"-v", "error",
		"-select_streams", "v",
		// §59.300 附十六：side_data_list（DOVI configuration record/dv_profile）在
		// entries 过滤下输出空对象（该 ffprobe 版本行为）——stream 段全量拿
		"-show_entries", "stream:format=duration",
		"-of", "json",
		videoPath,
	)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe: %w", err)
	}

	var result struct {
		Streams []struct {
			CodecName      string `json:"codec_name"`
			AvgFrameRate   string `json:"avg_frame_rate"`
			ColorTransfer  string `json:"color_transfer"`
			ColorPrimaries string `json:"color_primaries"`
			SideDataList []struct {
				SideDataType string `json:"side_data_type"`
				DVProfile    int    `json:"dv_profile"`
			} `json:"side_data_list,omitempty"`
		} `json:"streams"`
		Format struct {
			Duration   string `json:"duration"`
			FormatName string `json:"format_name"`
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

	hevcCount := 0
	for _, st := range result.Streams {
		if st.CodecName == "hevc" {
			hevcCount++
		}
	}
	fps := 0.0
	if len(result.Streams) > 0 {
		fps = parseFraction(result.Streams[0].AvgFrameRate)
	}
	if fps <= 0 {
		fps = 23.976
	}

	doviProfile := 0
	if len(result.Streams) > 0 {
		for _, sd := range result.Streams[0].SideDataList {
			if strings.Contains(strings.ToLower(sd.SideDataType), "dovi") {
				doviProfile = sd.DVProfile
				break
			}
		}
	}

	// §59.300 附十八：容器感知——mpv 对带索引容器（mkv/mp4/webm）默认 hr-seek
	return &videoInfo{
		duration:    d,
		isHDR:       isHDR,
		dualHEVC:    hevcCount >= 2,
		fps:         fps,
		doviProfile: doviProfile,
		isMpegTS:    strings.Contains(strings.ToLower(result.Format.FormatName), "mpegts"),
	}, nil
}

func parseFraction(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "0/0" {
		return 0
	}
	parts := strings.SplitN(s, "/", 2)
	num, err1 := strconv.ParseFloat(parts[0], 64)
	if err1 != nil {
		return 0
	}
	if len(parts) == 2 {
		den, err2 := strconv.ParseFloat(parts[1], 64)
		if err2 != nil || den == 0 {
			return 0
		}
		return num / den
	}
	return num
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

func (e *ScreenshotEngine) captureFrameMPV(ctx context.Context, videoPath string, timestamp float64, subtitleStreamID int, isHDR bool, outPath string) error {
	return e.captureFrameMPVEx(ctx, videoPath, timestamp, subtitleStreamID, isHDR, false, 23.976, 0, outPath)
}

// captureFrameMPVEx: dualHEVC 时走 DoVi P7 专用路径（vf dovi_reshape + IDR 精确起点前向取帧）。
func (e *ScreenshotEngine) captureFrameMPVEx(ctx context.Context, videoPath string, timestamp float64, subtitleStreamID int, isHDR, isMpegTS bool, fps float64, doviProfile int, outPath string) error {
	outDir := filepath.Dir(outPath)

	start := timestamp
	frames := 1
	hrSeek := false

	// §59.300 附十五/附十八：IDR 精确慢链仅 mpegts（无索引容器 open-GOP concealment，
	// Cold Storage/原始灰花投诉全为 m2ts）；mkv/mp4 精确 seek 保留 frames=1 快链
	//（发布链截图策略 4min 预算兼容——全量化曾致其必然超时）。m2ts 找不到关键帧 →
	// hr-seek 兜底（mpv 内部关键帧前向解码，取 120 帧末帧，cs120 实证）
	if isMpegTS {
		idr, err := e.findIDRBefore(ctx, videoPath, timestamp)
		if err == nil && idr > 0 && timestamp > idr {
			offset := (timestamp - idr) * fps
			n := int(math.Ceil(offset)) + 2 + int(math.Ceil(fps*1.2))
			if n < 3 {
				n = 3
			}
			if n > 360 {
				n = 360
			}
			start = idr
			frames = n
		} else {
			start = timestamp
			frames = 120
			hrSeek = true
			e.logger.Warn("screenshot: no IDR found before target, hr-seek fallback frames=120",
				zap.Float64("timestamp", timestamp))
		}
	}

	args := []string{
		"--vo=image",
		"--ao=null",
		"--no-audio",
	}
	if hrSeek {
		args = append(args, "--hr-seek=yes")
	}
	args = append(args,
		"--start="+strconv.FormatFloat(start, 'f', 6, 64),
		"--frames="+strconv.Itoa(frames),
		"--no-terminal",
		"--no-config",
		"--vo-image-format=jpg",
		"--vo-image-jpeg-quality="+strconv.Itoa(e.quality),
		"--vo-image-outdir="+outDir,
	)

	vfParts := []string{}
	// §59.300 附十四/附十五：
	// - P5（IPT 原生）：必须 dovi_reshape 反变换（帧自带 RPU 元数据路径）+ lavfi
	//   tonemap=PQ→SDR（iTunes WEB 紫→暖色自然实测）
	// - P7/P8 兼容盘：BL 即 HDR10，直通+lavfi（16 盘全量验证；reshape 会放大
	//   色度噪声成花屏）
	if doviProfile == 5 {
		vfParts = append(vfParts, "dovi_reshape")
		vfParts = append(vfParts, "lavfi=[tonemap=mobius]")
	} else if isHDR {
		vfParts = append(vfParts, "lavfi=[tonemap=mobius]")
	}
	if len(vfParts) > 0 {
		args = append(args, "--vf="+strings.Join(vfParts, ","))
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

	// 取编号最大的输出帧（frames>1 时最后一帧为目标帧）
	entries, listErr := os.ReadDir(outDir)
	if listErr != nil {
		return fmt.Errorf("read output dir: %w", listErr)
	}
	best := ""
	bestNum := -1
	for _, ent := range entries {
		name := ent.Name()
		if !strings.HasSuffix(name, ".jpg") {
			continue
		}
		base := strings.TrimSuffix(name, ".jpg")
		num, convErr := strconv.Atoi(base)
		if convErr != nil {
			continue
		}
		if num > bestNum {
			bestNum = num
			best = filepath.Join(outDir, name)
		}
	}
	if best == "" {
		return fmt.Errorf("mpv output file not found in %s", outDir)
	}
	// 清理本次 mpv 运行产生的多余数字编号帧（0000000N.jpg）。
	// 回归审核教训（§59.300 附二）：不得删除目录内其它文件——多时间点共用同一
	// tmpDir，前序迭代已重命名的 shot_00X.jpg 曾被误删，导致上传仅 1 张、
	// 低于 MinScreenshots 判"上传全失败"（普通盘/DoVi 盘同根）。
	for _, ent := range entries {
		name := ent.Name()
		if filepath.Join(outDir, name) == best || !strings.HasSuffix(name, ".jpg") {
			continue
		}
		base := strings.TrimSuffix(name, ".jpg")
		if _, convErr := strconv.Atoi(base); convErr != nil {
			continue // 非数字编号命名（如 shot_000.jpg）不动
		}
		_ = os.Remove(filepath.Join(outDir, name))
	}
	if best != outPath {
		if renameErr := os.Rename(best, outPath); renameErr != nil {
			return fmt.Errorf("rename output: %w", renameErr)
		}
	}
	return nil
}

// findIDRBefore: 目标时间点之前最近的关键帧 pts_time（秒）。
func (e *ScreenshotEngine) findIDRBefore(ctx context.Context, videoPath string, t float64) (float64, error) {
	windowStart := t - 5
	if windowStart < 0 {
		windowStart = 0
	}
	cmd := exec.CommandContext(ctx, e.ffprobePath, //nolint:gosec // intentional subprocess
		"-v", "error",
		"-read_intervals", fmt.Sprintf("%f%%%f", windowStart, t+0.5),
		"-skip_frame", "nokey",
		"-select_streams", "v:0",
		"-show_entries", "frame=pts_time",
		"-of", "csv=p=0",
		videoPath,
	)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe idr: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		line = strings.TrimSuffix(line, ",")
		if line == "" {
			continue
		}
		pt, parseErr := strconv.ParseFloat(line, 64)
		if parseErr == nil && pt > 0 && pt <= t {
			return pt, nil
		}
	}
	return 0, fmt.Errorf("no keyframe found before %f", t)
}
