package publish

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	bdinfo "github.com/ranfish/pt-forward/internal/bdinfo"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
)

type BDInfoScanner struct {
	logger *zap.Logger
}

func NewBDInfoScanner(logger *zap.Logger) *BDInfoScanner {
	return &BDInfoScanner{logger: logger}
}

// DetectBDPath 检测 save_path 下是否有 Blu-ray 内容（BDMV 目录或 .iso 文件）
// 返回 BD 内容的根路径，如果没有则返回空字符串
func DetectBDPath(savePath string) string {
	if savePath == "" {
		return ""
	}

	// 检测 BDMV 目录
	bdmvPath := filepath.Join(savePath, "BDMV")
	if info, err := os.Stat(bdmvPath); err == nil && info.IsDir() {
		return savePath
	}

	// savePath 本身可能是 BDMV 的父目录的子目录
	// 向上查找一层
	parent := filepath.Dir(savePath)
	parentBDMV := filepath.Join(parent, "BDMV")
	if info, err := os.Stat(parentBDMV); err == nil && info.IsDir() {
		return parent
	}

	// 检测 .iso 文件
	entries, err := os.ReadDir(savePath)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		lower := strings.ToLower(entry.Name())
		if strings.HasSuffix(lower, ".iso") || strings.HasSuffix(lower, ".m2ts") {
			fullPath := filepath.Join(savePath, entry.Name())
			if strings.HasSuffix(lower, ".iso") {
				return fullPath
			}
		}
		// 检测子目录中的 BDMV
		if entry.IsDir() {
			subBDMV := filepath.Join(savePath, entry.Name(), "BDMV")
			if info, err := os.Stat(subBDMV); err == nil && info.IsDir() {
				return filepath.Join(savePath, entry.Name())
			}
		}
	}

	return ""
}

// Scan 扫描 Blu-ray 内容并返回 BDInfo 文本报告
// progressCB 可选：用于实时报告进度
func (s *BDInfoScanner) Scan(ctx context.Context, path string, progressCB func(percent int, text string)) (string, error) {
	if path == "" {
		return "", nil
	}

	s.logger.Info("BDInfo scan starting", zap.String("path", path))

	settings := bdinfo.DefaultSettings("")
	settings.GenerateTextSummary = true

	options := bdinfo.Options{
		Path:     path,
		Settings: settings,
		OnProgress: func(event bdinfo.ProgressEvent) {
			s.logger.Debug("BDInfo progress",
				zap.String("stage", string(event.Stage)),
				zap.Int("completed", event.Completed),
				zap.Int("total", event.Total))
			if progressCB != nil {
				percent := 0
				if event.Total > 0 {
					percent = event.Completed * 100 / event.Total
				}
				stageText := string(event.Stage)
				switch event.Stage {
				case bdinfo.StageStarting:
					stageText = "正在启动"
				case bdinfo.StageDiscovered:
					stageText = "已发现播放列表"
				case bdinfo.StageScanning:
					stageText = "正在扫描"
				case bdinfo.StageClipInfo:
					stageText = "正在分析剪辑信息"
				case bdinfo.StagePlaylist:
					stageText = "正在分析播放列表"
				case bdinfo.StageStream:
					stageText = "正在分析流信息"
				}
				progressCB(percent, fmt.Sprintf("BDInfo: %s (%d/%d)", stageText, event.Completed, event.Total))
			}
		},
	}

	scanCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	result, err := bdinfo.Run(scanCtx, options)
	if err != nil {
		s.logger.Error("BDInfo scan failed", zap.String("path", path), zap.Error(err))
		return "", err
	}

	s.logger.Info("BDInfo scan completed",
		zap.String("path", path),
		zap.Int("playlists", len(result.Playlists)),
		zap.Int("report_len", len(result.Report)))

	return result.Report, nil
}

// ScanIfBD 检测并扫描（如果不是 BD 内容则返回空字符串，不报错）
func (s *BDInfoScanner) ScanIfBD(ctx context.Context, savePath string, progressCB func(percent int, text string)) (string, error) {
	bdPath := DetectBDPath(savePath)
	if bdPath == "" {
		return "", nil
	}
	if progressCB != nil {
		progressCB(1, "检测到蓝光原盘，正在启动 BDInfo 扫描...")
	}
	return s.Scan(ctx, bdPath, progressCB)
}

// FillBDInfo 将 BDInfo 报告填充到 DescriptionData
func FillBDInfo(descData *model.DescriptionData, bdinfoText string) {
	if bdinfoText != "" {
		descData.BDInfoText = bdinfoText
	}
}
