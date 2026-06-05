package publish

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ranfish/pt-forward/internal/screenshot"
	"go.uber.org/zap"
)

type PublishArtifactGenerator struct {
	screenshotEngine  *ScreenshotEngine
	subtitleDetector  *SubtitleDetector
	mediaInfoAnalyzer *MediaInfoAnalyzer
	imageUploader     *ImageHostUploader
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

type ArtifactResult struct {
	ScreenshotURLs []string
	MediaInfoText  string
	SubtitleSID    int
}

func (g *PublishArtifactGenerator) Generate(ctx context.Context, torrentDir string, sourceMediaInfo string, sourceScreenshots []string) (*ArtifactResult, error) {
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

	if g.mediaInfoAnalyzer.Available() {
		mi, err := g.mediaInfoAnalyzer.Analyze(ctx, videoPath)
		if err != nil {
			g.logger.Warn("local mediainfo failed", zap.Error(err))
		} else if mi != nil {
			result.MediaInfoText = mi.RawOutput
		}
	}
	if result.MediaInfoText == "" {
		result.MediaInfoText = sourceMediaInfo
	}

	if g.screenshotEngine != nil && g.screenshotEngine.Available() {
		subtitleSID := 0
		if g.subtitleDetector.Available() {
			if sid, err := g.subtitleDetector.FindSubtitleStreamID(ctx, videoPath); err == nil && sid > 0 {
				subtitleSID = sid
				g.logger.Info("detected chinese subtitle stream", zap.Int("sid", subtitleSID))
			}
		}

		localScreenshots, err := g.screenshotEngine.Capture(ctx, videoPath, subtitleSID)
		if err != nil {
			g.logger.Warn("local screenshot capture failed, using source screenshots",
				zap.Error(err))
			result.ScreenshotURLs = sourceScreenshots
		} else if len(localScreenshots) > 0 {
			if uploadedURLs, err := g.imageUploader.UploadMultiple(ctx, localScreenshots); err == nil && len(uploadedURLs) > 0 {
				result.ScreenshotURLs = uploadedURLs
				g.logger.Info("uploaded local screenshots",
					zap.Int("count", len(uploadedURLs)))
			} else {
				g.logger.Warn("screenshot upload failed, using source screenshots",
					zap.Error(err))
				result.ScreenshotURLs = sourceScreenshots
			}
		}
	} else {
		result.ScreenshotURLs = sourceScreenshots
	}

	return result, nil
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
