package publish

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/imagehost"
	"github.com/ranfish/pt-forward/internal/screenshot"
	"go.uber.org/zap"
)

type PublishArtifactGenerator struct {
	screenshotEngine  *ScreenshotEngine
	subtitleDetector  *SubtitleDetector
	mediaInfoAnalyzer *MediaInfoAnalyzer
	imageUploader     *ImageHostUploader
	imageHostMgr      *imagehost.Manager // §56.17 决策 2: 统一图床管理
	logger            *zap.Logger
}

func NewPublishArtifactGenerator(cfg *screenshot.Config, logger *zap.Logger) *PublishArtifactGenerator {
	g := &PublishArtifactGenerator{logger: logger}
	if cfg != nil {
		g.screenshotEngine = NewScreenshotEngine(cfg.MpvPath, cfg.Count, cfg.MinInterval, cfg.JPEGQuality, logger)
	}
	g.subtitleDetector = NewSubtitleDetector(logger)
	g.mediaInfoAnalyzer = NewMediaInfoAnalyzer(logger)
	g.imageUploader = NewImageHostUploader(logger)
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

	videoPath, err := g.findLargestVideo(torrentDir)
	if err != nil {
		g.logger.Warn("no video file found, using source artifacts",
			zap.String("dir", torrentDir),
			zap.Error(err))
		result.MediaInfoText = sourceMediaInfo
		result.ScreenshotURLs = sourceScreenshots
		return result, nil
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

	validSourceShots := g.validateScreenshots(ctx, sourceScreenshots)

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
		if len(validSourceShots) >= 3 {
			result.ScreenshotURLs = g.rehostScreenshots(ctx, validSourceShots)
		}
		if len(result.ScreenshotURLs) < 3 {
			localShots := g.captureLocalScreenshots(ctx, videoPath)
			if len(localShots) > len(result.ScreenshotURLs) {
				result.ScreenshotURLs = localShots
			}
		}
		if len(result.ScreenshotURLs) == 0 {
			result.ScreenshotURLs = sourceScreenshots
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

func (g *PublishArtifactGenerator) validateScreenshots(ctx context.Context, urls []string) []string {
	if len(urls) == 0 {
		return nil
	}
	client := &http.Client{Timeout: 10 * time.Second}
	var valid []string
	for _, u := range urls {
		req, err := http.NewRequestWithContext(ctx, "HEAD", u, nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			g.logger.Debug("screenshot validation failed", zap.String("url", u), zap.Error(err))
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			valid = append(valid, u)
		}
	}
	return valid
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
		g.logger.Warn("local screenshot capture failed", zap.Error(err))
		return nil
	}
	// §56.17 决策 1: 优先用 imagehost.Manager.Upload（统一接口）
	var uploaded []string
	if g.imageHostMgr != nil {
		for _, shotPath := range localShots {
			data, readErr := os.ReadFile(shotPath)
			if readErr != nil {
				continue
			}
			result, uploadErr := g.imageHostMgr.Upload(ctx, data, filepath.Base(shotPath))
			if uploadErr != nil || result == nil || result.URL == "" {
				continue
			}
			uploaded = append(uploaded, result.URL)
		}
	} else {
		// fallback: 旧版 ImageHostUploader（向后兼容）
		uploaded, err = g.imageUploader.UploadMultiple(ctx, localShots)
	}
	if tmpDir != "" {
		_ = os.RemoveAll(tmpDir)
	}
	if err != nil || len(uploaded) == 0 {
		g.logger.Warn("screenshot upload failed", zap.Error(err))
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
