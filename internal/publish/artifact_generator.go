package publish

import (
	"sync"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/imagehost"
	"github.com/ranfish/pt-forward/internal/setting"
	"github.com/ranfish/pt-forward/internal/screenshot"
	"go.uber.org/zap"
)

type PublishArtifactGenerator struct {
	screenshotEngine  *ScreenshotEngine
	subtitleDetector  *SubtitleDetector
	mediaInfoAnalyzer *MediaInfoAnalyzer
	imageHostMgr      *imagehost.Manager // §56.17 决策 2: 统一图床管理
	logger            *zap.Logger
	// §59.255: 截图引擎参数运行时动态读（改配置即生效免重启——MpvPath/Count/
	// MinInterval/Quality/Enabled 全五项；RuntimeConfig TTL 30s+PUT 立即失效）
	runtimeCfg    *setting.RuntimeConfig
	startupConfig *screenshot.Config // settings 未注入时回落（测试形态）
	engineMu      sync.RWMutex
	engineSig     string // 参数指纹（变更才重建 engine）
}

func NewPublishArtifactGenerator(cfg *screenshot.Config, logger *zap.Logger) *PublishArtifactGenerator {
	g := &PublishArtifactGenerator{logger: logger, startupConfig: cfg}
	g.rebuildEngine(cfg)
	g.subtitleDetector = NewSubtitleDetector(logger)
	g.mediaInfoAnalyzer = NewMediaInfoAnalyzer(logger)
	return g
}

// SetRuntimeConfig §59.255: 注入 RuntimeConfig——截图五参数运行时化
func (g *PublishArtifactGenerator) SetRuntimeConfig(rc *setting.RuntimeConfig) {
	g.runtimeCfg = rc
}

func (g *PublishArtifactGenerator) currentScreenshotConfig() *screenshot.Config {
	if g.runtimeCfg == nil {
		return g.startupConfig
	}
	ctx := context.Background()
	cfg := &screenshot.Config{}
	if !g.runtimeCfg.GetBool(ctx, setting.KeyScreenshotEnabled) {
		return cfg // Enabled=false → MpvPath 空 → 不建引擎（MediaInfo 仍提取）
	}
	cfg.MpvPath = g.runtimeCfg.GetString(ctx, setting.KeyScreenshotMpvPath)
	cfg.Count = g.runtimeCfg.GetInt(ctx, setting.KeyScreenshotCount)
	cfg.MinInterval = g.runtimeCfg.GetInt(ctx, setting.KeyScreenshotMinInterval)
	cfg.JPEGQuality = g.runtimeCfg.GetInt(ctx, setting.KeyScreenshotJPEGQuality)
	cfg.Format = g.runtimeCfg.GetString(ctx, setting.KeyScreenshotFormat)
	if cfg.MpvPath == "" {
		cfg.MpvPath = "mpv"
	}
	if cfg.Count == 0 {
		cfg.Count = 6
	}
	return cfg
}

// screenshotEngineNow 参数指纹变更时重建（幂等）
func (g *PublishArtifactGenerator) screenshotEngineNow() *ScreenshotEngine {
	cfg := g.currentScreenshotConfig()
	sig := fmt.Sprintf("%s|%d|%d|%d|%s", cfg.MpvPath, cfg.Count, cfg.MinInterval, cfg.JPEGQuality, cfg.Format)
	g.engineMu.RLock()
	if g.engineSig == sig {
		e := g.screenshotEngine
		g.engineMu.RUnlock()
		return e
	}
	g.engineMu.RUnlock()
	g.engineMu.Lock()
	defer g.engineMu.Unlock()
	if g.engineSig == sig {
		return g.screenshotEngine
	}
	var engine *ScreenshotEngine
	if cfg.MpvPath != "" {
		engine = NewScreenshotEngine(cfg.MpvPath, cfg.Count, cfg.MinInterval, cfg.JPEGQuality, cfg.Format, g.logger)
	}
	g.screenshotEngine = engine
	g.engineSig = sig
	return engine
}

func (g *PublishArtifactGenerator) rebuildEngine(cfg *screenshot.Config) {
	g.engineMu.Lock()
	defer g.engineMu.Unlock()
	if cfg != nil && cfg.MpvPath != "" {
		g.screenshotEngine = NewScreenshotEngine(cfg.MpvPath, cfg.Count, cfg.MinInterval, cfg.JPEGQuality, cfg.Format, g.logger)
		g.engineSig = fmt.Sprintf("%s|%d|%d|%d|%s", cfg.MpvPath, cfg.Count, cfg.MinInterval, cfg.JPEGQuality, cfg.Format)
	} else {
		g.screenshotEngine = nil
		g.engineSig = ""
	}
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

func (g *PublishArtifactGenerator) GenerateWithStrategy(ctx context.Context, torrentDir string, sourceMediaInfo string, sourceScreenshots []string, strategy string, forcedSid ...int) (*ArtifactResult, error) {
	forcedSubSid := 0
	if len(forcedSid) > 0 {
		forcedSubSid = forcedSid[0]
	}
	result := &ArtifactResult{}

	// 如果 torrentDir 是文件路径（非目录），直接分析该文件（精确匹配模式）
	var videoPath string
	var fallbackVideos []string // §59.540 附五: 截图 fallback 候选（目录模式次大文件）
	if info, statErr := os.Stat(torrentDir); statErr == nil && !info.IsDir() {
		videoPath = torrentDir
	} else {
		candidates, err := g.findLargestVideos(torrentDir, 2) // 主选+次选（短路界）
		if err != nil {
			g.logger.Warn("no video file found, using source artifacts",
				zap.String("dir", torrentDir),
				zap.Error(err))
			result.MediaInfoText = sourceMediaInfo
			result.ScreenshotURLs = sourceScreenshots
			return result, nil
		}
		videoPath = candidates[0]
		if len(candidates) > 1 {
			fallbackVideos = candidates[1:]
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
		result.ScreenshotURLs = g.captureLocalScreenshots(ctx, videoPath, fallbackVideos, forcedSubSid)
		if len(result.ScreenshotURLs) == 0 {
			result.ScreenshotURLs = sourceScreenshots
		}
	default:
		// §59.53 auto 重构（探活归位 §59.49——调用方传入列表已探活，此处不再 validate）：
		//   白名单逐张保留 / 非白名单逐张转存（无 ≥3 阈值）
		//   不足 MinScreenshots → mpv 差额补足（补足者不竞争）
		//   0 张 → mpv 全量（按 Count 配置）
		//   mpv 失败/不可用 → 有几张算几张（<3 由审核门槛挡，§59.53 第7点）
		// §59.53 补丁: 上限 MinScreenshots+5=8 张（疯狂动物城2 129 张案例——
		// 发布者异常全贴图集，全量转存慢且发布简介无用；库值保留全量可手动加回）
		autoSource := sourceScreenshots
		if len(autoSource) > MinScreenshots+5 {
			autoSource = autoSource[:MinScreenshots+5]
			g.logger.Info("screenshot auto capped",
				zap.Int("source_total", len(sourceScreenshots)),
				zap.Int("capped_to", MinScreenshots+5))
		}
		result.ScreenshotURLs = g.processScreenshotsAuto(ctx, autoSource)
		if len(result.ScreenshotURLs) < MinScreenshots {
			need := MinScreenshots - len(result.ScreenshotURLs)
			var localShots []string
			if len(result.ScreenshotURLs) == 0 {
				localShots = g.captureLocalScreenshots(ctx, videoPath, fallbackVideos, forcedSubSid) // 全量（按 Count）
			} else {
				localShots = g.captureLocalScreenshotsCount(ctx, videoPath, fallbackVideos, need) // 差额
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

// uploadWithRetry §59.61 附3 层1+2: 单张上传重试（指数退避）+ 独立每张超时。
// 4005 批次实锤: pixhost 限流时 4min strategyCtx 尾部被排前的张占完, 后续全
// context deadline exceeded（"丢尾部"模式, 370 行 2 张截图根因）——上传预算与
// 捕获 ctx 解耦: 每张独立 60s, 重试 2 次（共 3 次尝试, 退避 2s/8s）。
func uploadWithRetry(parent context.Context,
	upload func(ctx context.Context, data []byte, name string) (string, error),
	data []byte, name string, attempts int) (string, error) {

	var lastErr error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			// 退避: 2s, 8s（parent 取消则提前退出）
			backoff := time.Duration(1<<uint(2*i-1)) * time.Second //nolint:gosec // i 上界 2（重试轮数） // i=1:2s, i=2:8s
			select {
			case <-parent.Done():
				return "", parent.Err()
			case <-time.After(backoff):
			}
		}
		ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 60*time.Second)
		u, err := upload(ctx, data, name)
		cancel()
		if err == nil && u != "" {
			return u, nil
		}
		lastErr = err
	}
	return "", lastErr
}

// captureLocalScreenshots 本地截图+上传。§59.540 附五: fallbackVideos——
// 多文件剧集主选文件解码零帧（源损坏·E24 案）时逐个换次大候选重试；候选
// 耗尽仍失败=短路返回 nil（诚实无图→审核拦截不可发布·删种由用户决定）。
// sid 每候选独立探测（换文件后轨号语义不延续）；forced 人工轨跨候选保持
// （用户显式动作优先·轨缺失时 mpv 自然降级）。
func (g *PublishArtifactGenerator) captureLocalScreenshots(ctx context.Context, videoPath string, fallbackVideos []string, forcedSid ...int) []string {
	engineProbe := g.screenshotEngineNow()
	if engineProbe == nil || !engineProbe.Available() {
		return nil
	}
	forcedSubSid := 0
	if len(forcedSid) > 0 && forcedSid[0] > 0 {
		forcedSubSid = forcedSid[0]
	}
	candidates := append([]string{videoPath}, fallbackVideos...)
	for i, candidate := range candidates {
		localShots, tmpDir := g.captureOneCandidate(ctx, engineProbe, candidate, forcedSubSid)
		if localShots == nil {
			if i+1 < len(candidates) {
				g.logger.Warn("screenshot capture failed on video, falling back to next",
					zap.String("video", filepath.Base(candidate)),
					zap.String("next", filepath.Base(candidates[i+1])))
				continue
			}
			g.logger.Warn("screenshot capture failed on all candidates, honest no-screenshots",
				zap.Int("candidates", len(candidates)),
				zap.String("last_video", filepath.Base(candidate)))
			return nil // 候选耗尽·短路失败（现状语义）
		}
		return g.uploadCapturedShots(ctx, localShots, tmpDir)
	}
	return nil
}

// captureOneCandidate 单候选截图尝试（sid 探测+可观测日志+Capture）。
// 失败返回 (nil, "")（tmpDir 已清理）；成功返回 (本地截图路径, tmpDir)。
func (g *PublishArtifactGenerator) captureOneCandidate(ctx context.Context, engine *ScreenshotEngine, videoPath string, forcedSubSid int) ([]string, string) {
	subtitleSID := 0
	subtitleSource := "none"
	if forcedSubSid > 0 {
		subtitleSID = forcedSubSid // §59.298 C: 人工指定轨（tab3 下拉）——跳过自动选择
		subtitleSource = "forced"
	} else if g.subtitleDetector.Available() {
		if sid, err := g.subtitleDetector.FindSubtitleStreamID(ctx, videoPath); err == nil && sid > 0 {
			subtitleSID = sid
			subtitleSource = "auto"
		}
	}
	// §59.319 附十九: sid 可观测——附十五三轮排查全靠手写探针二分（check-sid
	// 三版），根因是链上零日志（mpv 实际收到什么 sid、走了哪条路径均不可见）
	g.logger.Info("screenshot subtitle selection",
		zap.Int("sid", subtitleSID),
		zap.String("source", subtitleSource),
		zap.String("video", filepath.Base(videoPath)))
	localShots, tmpDir, err := engine.Capture(ctx, videoPath, subtitleSID)
	if err != nil || len(localShots) == 0 {
		if tmpDir != "" {
			_ = os.RemoveAll(tmpDir)
		}
		g.logger.Warn("local screenshot capture failed", zap.Int("shots", len(localShots)), zap.Error(err))
		return nil, ""
	}
	g.logger.Info("local screenshots captured", zap.Int("count", len(localShots)), zap.String("tmpDir", tmpDir))
	return localShots, tmpDir
}

// uploadCapturedShots 截图上传（原 captureLocalScreenshots 上传段·行为零变化）。
func (g *PublishArtifactGenerator) uploadCapturedShots(ctx context.Context, localShots []string, tmpDir string) []string {
	if g.imageHostMgr == nil {
		if tmpDir != "" {
			_ = os.RemoveAll(tmpDir)
		}
		g.logger.Warn("imageHostMgr not configured, skipping upload")
		return nil
	}
	uploaded, failedPaths := g.uploadShotsWithRetry(ctx, localShots)
	if tmpDir != "" {
		if len(uploaded) >= MinScreenshots || len(failedPaths) == 0 {
			_ = os.RemoveAll(tmpDir)
		}
		// §59.61 附3 层3: 不足 Min 且有失败张——tmpDir 保留给二次补传（defer 兜底清理）
		defer func() { _ = os.RemoveAll(tmpDir) }()
	}
	if len(uploaded) == 0 {
		g.logger.Warn("screenshot upload all failed", zap.Int("captured", len(localShots)))
		return uploaded
	}
	if len(failedPaths) > 0 {
		g.logger.Warn("screenshot upload partial failure",
			zap.Int("captured", len(localShots)),
			zap.Int("uploaded", len(uploaded)),
			zap.Int("failed", len(failedPaths)))
	}
	g.logger.Info("uploaded local screenshots", zap.Int("count", len(uploaded)))
	return uploaded
}

// uploadShotsWithRetry §59.61 附3: 批量上传 + 重试 + 层3 不足 Min 二次补传。
// 返回 (成功URL列表, 失败文件路径列表——供补传)。
func (g *PublishArtifactGenerator) uploadShotsWithRetry(ctx context.Context, localShots []string) ([]string, []string) {
	if g.imageHostMgr == nil {
		g.logger.Warn("imageHostMgr not configured, skipping upload")
		return nil, nil
	}
	uploader := func(ctx context.Context, data []byte, name string) (string, error) {
		result, err := g.imageHostMgr.Upload(ctx, data, name)
		if err != nil {
			return "", err
		}
		if result == nil || result.URL == "" {
			return "", errors.New("empty upload result")
		}
		return result.URL, nil
	}
	var uploaded []string
	var failedPaths []string
	for i, shotPath := range localShots {
		data, readErr := os.ReadFile(shotPath) //nolint:gosec // 截图临时目录内部路径
		if readErr != nil {
			g.logger.Warn("screenshot read failed", zap.String("path", shotPath), zap.Error(readErr))
			continue
		}
		u, err := uploadWithRetry(ctx, uploader, data, filepath.Base(shotPath), 3)
		if err != nil {
			g.logger.Warn("screenshot upload skipped after retries",
				zap.Int("idx", i), zap.String("path", shotPath),
				zap.Int("size", len(data)),
				zap.NamedError("upload_err", err))
			failedPaths = append(failedPaths, shotPath)
			continue
		}
		uploaded = append(uploaded, u)
	}
	// 层3: 达标不足 MinScreenshots 且有失败张 → 失败张二次补传（退避后再试一轮）
	if len(uploaded) < MinScreenshots && len(failedPaths) > 0 {
		g.logger.Info("screenshot upload below minimum, retrying failed",
			zap.Int("uploaded", len(uploaded)), zap.Int("failed", len(failedPaths)))
		select {
		case <-ctx.Done():
		case <-time.After(10 * time.Second):
		}
		var still []string
		for _, shotPath := range failedPaths {
			data, readErr := os.ReadFile(shotPath) //nolint:gosec // 截图临时目录内部路径
			if readErr != nil {
				continue
			}
			u, err := uploadWithRetry(ctx, uploader, data, filepath.Base(shotPath), 3)
			if err != nil {
				still = append(still, shotPath)
				continue
			}
			uploaded = append(uploaded, u)
		}
		failedPaths = still
	}
	return uploaded, failedPaths
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

// findLargestVideo 体积最大单候选（单文件形态语义保留）。
func (g *PublishArtifactGenerator) findLargestVideo(dir string) (string, error) {
	paths, err := g.findLargestVideos(dir, 1)
	if err != nil {
		return "", err
	}
	return paths[0], nil
}

// findLargestVideos §59.540 附五: 体积降序前 n 个视频候选——多文件剧集
// 截图 fallback 用（最大文件源损坏（E24 案·h264 码流层坏但容器/BT 校验层
// 完好）→ 次大文件重试）。n 上界即短路界：候选耗尽=诚实无图。
func (g *PublishArtifactGenerator) findLargestVideos(dir string, n int) ([]string, error) {
	if n <= 0 {
		return nil, fmt.Errorf("invalid candidate count %d", n)
	}
	videoExts := map[string]bool{
		".mkv": true, ".mp4": true, ".avi": true, ".ts": true,
		".m2ts": true, ".wmv": true, ".flv": true, ".mov": true,
		".webm": true, ".mpg": true, ".mpeg": true, ".vob": true,
		".iso": true, // §59.319 附二十: ISO 原盘（mpv bd:// 协议直读）
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
		return nil, fmt.Errorf("walk directory: %w", err)
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no video files found in %s", dir)
	}

	sort.Slice(candidates, func(i, j int) bool {
		si, errI := os.Stat(candidates[i])
		sj, errJ := os.Stat(candidates[j])
		if errI != nil || errJ != nil {
			return errI == nil
		}
		return si.Size() > sj.Size()
	})

	if len(candidates) > n {
		candidates = candidates[:n]
	}
	return candidates, nil
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
// §59.540 附五: fallback 候选透传（主文件坏时差额补拍同样换次大文件）。
func (g *PublishArtifactGenerator) captureLocalScreenshotsCount(ctx context.Context, videoPath string, fallbackVideos []string, n int) []string {
	if n <= 0 {
		return nil
	}
	all := g.captureLocalScreenshots(ctx, videoPath, fallbackVideos)
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
