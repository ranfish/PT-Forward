package publish

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ranfish/pt-forward/internal/imagehost"
	"github.com/ranfish/pt-forward/internal/screenshot"
	"go.uber.org/zap"
)

type PublishArtifactGenerator struct {
	screenshotEngine  *ScreenshotEngine
	subtitleDetector  *SubtitleDetector
	mediaInfoAnalyzer *MediaInfoAnalyzer
	imageHostMgr      *imagehost.Manager // §56.17 决策 2: 统一图床管理
	logger            *zap.Logger
}

func NewPublishArtifactGenerator(cfg *screenshot.Config, logger *zap.Logger) *PublishArtifactGenerator {
	g := &PublishArtifactGenerator{logger: logger}
	// v0.0.255: MpvPath 为空时不创建 screenshotEngine（screenshot_enabled=false 场景）
	// 但 MediaInfoAnalyzer/SubtitleDetector 始终创建（不依赖 screenshot 开关）
	if cfg != nil && cfg.MpvPath != "" {
		g.screenshotEngine = NewScreenshotEngine(cfg.MpvPath, cfg.Count, cfg.MinInterval, cfg.JPEGQuality, logger)
	}
	g.subtitleDetector = NewSubtitleDetector(logger)
	g.mediaInfoAnalyzer = NewMediaInfoAnalyzer(logger)
	return g
}

// SetImageHostManager §56.17 决策 2: 注入统一图床管理器。
// 注入后 rehostScreenshots 会调用 Manager.Rehost 转存截图；
// 未注入时保留原始 URL（向后兼容）。
func (g *PublishArtifactGenerator) SetImageHostManager(mgr *imagehost.Manager) {
	g.imageHostMgr = mgr
}

type ArtifactResult struct {
	ScreenshotURLs []string
	MediaInfoText  string
	SubtitleSID    int
}

func (g *PublishArtifactGenerator) Generate(ctx context.Context, torrentDir string, sourceMediaInfo string, sourceScreenshots []string) (*ArtifactResult, error) {
	return g.GenerateWithStrategy(ctx, torrentDir, sourceMediaInfo, sourceScreenshots, "auto")
}

func (g *PublishArtifactGenerator) GenerateWithStrategy(ctx context.Context, torrentDir string, sourceMediaInfo string, sourceScreenshots []string, strategy string) (*ArtifactResult, error) {
	result := &ArtifactResult{}

	// 如果 torrentDir 是文件路径（非目录），直接分析该文件（精确匹配模式）
	var videoPath string
	if info, statErr := os.Stat(torrentDir); statErr == nil && !info.IsDir() {
		videoPath = torrentDir
	} else {
		var err error
		videoPath, err = g.findLargestVideo(torrentDir)
		if err != nil {
			g.logger.Warn("no video file found, using source artifacts",
				zap.String("dir", torrentDir),
				zap.Error(err))
			result.MediaInfoText = sourceMediaInfo
			result.ScreenshotURLs = sourceScreenshots
			return result, nil
		}
	}

	// MediaInfo analysis 与截图并行（V: 产物并行生成）
	type miResult struct {
		text string
		err  error
	}
	miCh := make(chan miResult, 1)
	if g.mediaInfoAnalyzer.Available() {
		go func() {
			mi, err := g.mediaInfoAnalyzer.Analyze(ctx, videoPath)
			text := ""
			if err == nil && mi != nil {
				text = mi.RawOutput
			}
			miCh <- miResult{text: text, err: err}
		}()
	} else {
		miCh <- miResult{}
	}

	switch strategy {
	case "source_direct":
		result.ScreenshotURLs = sourceScreenshots
	case "source_rehost":
		result.ScreenshotURLs = g.rehostScreenshots(ctx, sourceScreenshots)
	case "local_upload":
		result.ScreenshotURLs = g.captureLocalScreenshots(ctx, videoPath)
		if len(result.ScreenshotURLs) == 0 {
			result.ScreenshotURLs = sourceScreenshots
		}
	default:
		// §59.53 auto 重构（探活归位 §59.49——调用方传入列表已探活，此处不再 validate）：
		//   白名单逐张保留 / 非白名单逐张转存（无 ≥3 阈值）
		//   不足 MinScreenshots → mpv 差额补足（补足者不竞争）
		//   0 张 → mpv 全量（按 Count 配置）
		//   mpv 失败/不可用 → 有几张算几张（<3 由审核门槛挡，§59.53 第7点）
		result.ScreenshotURLs = g.processScreenshotsAuto(ctx, sourceScreenshots)
		if len(result.ScreenshotURLs) < MinScreenshots {
			need := MinScreenshots - len(result.ScreenshotURLs)
			var localShots []string
			if len(result.ScreenshotURLs) == 0 {
				localShots = g.captureLocalScreenshots(ctx, videoPath) // 全量（按 Count）
			} else {
				localShots = g.captureLocalScreenshotsCount(ctx, videoPath, need) // 差额
			}
			result.ScreenshotURLs = append(result.ScreenshotURLs, localShots...) // 源站在前补足在后
		}
	}

	if len(result.ScreenshotURLs) == 0 {
		result.ScreenshotURLs = sourceScreenshots
	}

	// 等待 MediaInfo 完成
	miRes := <-miCh
	if miRes.err != nil {
		g.logger.Warn("local mediainfo failed", zap.Error(miRes.err))
	}
	result.MediaInfoText = miRes.text
	if result.MediaInfoText == "" {
		result.MediaInfoText = sourceMediaInfo
	}

	return result, nil
}

func (g *PublishArtifactGenerator) captureLocalScreenshots(ctx context.Context, videoPath string) []string {
	if g.screenshotEngine == nil || !g.screenshotEngine.Available() {
		return nil
	}
	subtitleSID := 0
	if g.subtitleDetector.Available() {
		if sid, err := g.subtitleDetector.FindSubtitleStreamID(ctx, videoPath); err == nil && sid > 0 {
			subtitleSID = sid
		}
	}
	localShots, tmpDir, err := g.screenshotEngine.Capture(ctx, videoPath, subtitleSID)
	if err != nil || len(localShots) == 0 {
		if tmpDir != "" {
			_ = os.RemoveAll(tmpDir)
		}
		g.logger.Warn("local screenshot capture failed", zap.Int("shots", len(localShots)), zap.Error(err))
		return nil
	}
	g.logger.Info("local screenshots captured", zap.Int("count", len(localShots)), zap.String("tmpDir", tmpDir))
	if g.imageHostMgr == nil {
		if tmpDir != "" {
			_ = os.RemoveAll(tmpDir)
		}
		g.logger.Warn("imageHostMgr not configured, skipping upload")
		return nil
	}
	var uploaded []string
	for i, shotPath := range localShots {
		data, readErr := os.ReadFile(shotPath)
		if readErr != nil {
			g.logger.Warn("screenshot read failed", zap.String("path", shotPath), zap.Error(readErr))
			continue
		}
		result, uploadErr := g.imageHostMgr.Upload(ctx, data, filepath.Base(shotPath))
		if uploadErr != nil || result == nil || result.URL == "" {
			g.logger.Warn("screenshot upload skipped",
				zap.Int("idx", i), zap.String("path", shotPath),
				zap.Int("size", len(data)),
				zap.NamedError("upload_err", uploadErr),
				zap.Any("result", result))
			continue
		}
		uploaded = append(uploaded, result.URL)
	}
	if tmpDir != "" {
		_ = os.RemoveAll(tmpDir)
	}
	if len(uploaded) == 0 {
		g.logger.Warn("screenshot upload all failed", zap.Int("captured", len(localShots)))
		return nil
	}
	g.logger.Info("uploaded local screenshots", zap.Int("count", len(uploaded)))
	return uploaded
}

// rehostScreenshots §56.17 决策 2: 截图转存（补全空实现）。
// 调用 imageHostMgr.Rehost 逐张转存，失败时 fallback 原始 URL。
// 未配置 imageHostMgr 时保留原始 URL（向后兼容）。
func (g *PublishArtifactGenerator) rehostScreenshots(ctx context.Context, urls []string) []string {
	if len(urls) == 0 {
		return urls
	}
	if g.imageHostMgr == nil {
		return urls
	}

	rehosted := make([]string, 0, len(urls))
	for _, url := range urls {
		result, err := g.imageHostMgr.Rehost(ctx, url)
		if err != nil || result == nil || result.URL == "" {
			g.logger.Warn("screenshot rehost failed, using source url",
				zap.String("url", url),
				zap.Error(err))
			rehosted = append(rehosted, url)
			continue
		}
		rehosted = append(rehosted, result.URL)
	}
	return rehosted
}

func (g *PublishArtifactGenerator) findLargestVideo(dir string) (string, error) {
	videoExts := map[string]bool{
		".mkv": true, ".mp4": true, ".avi": true, ".ts": true,
		".m2ts": true, ".wmv": true, ".flv": true, ".mov": true,
		".webm": true, ".mpg": true, ".mpeg": true, ".vob": true,
	}

	var candidates []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if videoExts[ext] {
			candidates = append(candidates, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk directory: %w", err)
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no video files found in %s", dir)
	}

	sort.Slice(candidates, func(i, j int) bool {
		si, errI := os.Stat(candidates[i])
		sj, errJ := os.Stat(candidates[j])
		if errI != nil || errJ != nil {
			return errI == nil
		}
		return si.Size() > sj.Size()
	})

	return candidates[0], nil
}

// processScreenshotsAuto §59.53: 源站截图逐张处置——白名单（pixhost 家族/
// doubaninfo）保留原样，非白名单转存到系统图床。转存失败保留源 URL（活链有价值，
// §59.53 第3点：转存失败≠死链）。
func (g *PublishArtifactGenerator) processScreenshotsAuto(ctx context.Context, urls []string) []string {
	if len(urls) == 0 {
		return nil
	}
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		if IsTrustedImageSource(u) {
			out = append(out, u) // 白名单保留
			continue
		}
		if g.imageHostMgr == nil {
			out = append(out, u) // 无图床配置保留原样
			continue
		}
		if r, err := g.imageHostMgr.Rehost(ctx, u); err == nil && r != nil && r.URL != "" {
			out = append(out, r.URL)
		} else {
			g.logger.Warn("screenshot rehost failed, keeping source",
				zap.String("url", u[:min(60, len(u))]), zap.Error(err))
			out = append(out, u)
		}
	}
	return out
}

// captureLocalScreenshotsCount §59.53: mpv 截指定张数（差额补足）。
// 内部复用全量截图后取前 n 张（时间点均布，前 n 张即为按序补足）。
func (g *PublishArtifactGenerator) captureLocalScreenshotsCount(ctx context.Context, videoPath string, n int) []string {
	if n <= 0 {
		return nil
	}
	all := g.captureLocalScreenshots(ctx, videoPath)
	if len(all) <= n {
		return all
	}
	return all[:n]
}

// ProcessScreenshotsRemote §59.53 第6点: 远程下载器策略——只逐张处置（白名单保留/
// 非白名单转存），不截图。0 张返回 nil（留空，审核挡）。
func (g *PublishArtifactGenerator) ProcessScreenshotsRemote(urls []string) []string {
	if len(urls) == 0 {
		return nil
	}
	// 无 ctx 需求时用 context.Background（Rehost 内部有自身超时）
	return g.processScreenshotsAuto(context.Background(), urls)
}
