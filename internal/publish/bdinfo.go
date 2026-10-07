package publish

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	bdinfo "github.com/ranfish/pt-forward/internal/bdinfo"
	"go.uber.org/zap"
)
type BDInfoScanner struct {
	logger *zap.Logger
}

func NewBDInfoScanner(logger *zap.Logger) *BDInfoScanner {
	return &BDInfoScanner{logger: logger}
}

// DetectOriginalDisc 从种子文件列表判定是否原盘（§59.319 D2 识别门）。
// 输入=RPC 文件相对路径列表（qbittorrent contents / transmission files[].name），
// 远程本地通用于入队判定（无需磁盘访问）。
// 判据：任一路径位于 BDMV/ 目录下（"BDMV/xxx" 或 "dir/BDMV/xxx"，大小写
// 不敏感），或含 .iso 镜像文件。单 .m2ts 不算（无 CLPI/MPLS 非完整盘，
// BDInfo 无法扫描）。
func DetectOriginalDisc(fileNames []string) bool {
	for _, name := range fileNames {
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "bdmv/") || strings.Contains(lower, "/bdmv/") {
			return true
		}
		if strings.HasSuffix(lower, ".iso") {
			return true
		}
	}
	return false
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

	// 检测 .iso 文件与子目录中的 BDMV（§59.319 P0：原子目录检测在
	// .iso 分支后被 entry.IsDir() continue 不可达——重排修复）
	entries, err := os.ReadDir(savePath)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			subBDMV := filepath.Join(savePath, entry.Name(), "BDMV")
			if info, err := os.Stat(subBDMV); err == nil && info.IsDir() {
				return filepath.Join(savePath, entry.Name())
			}
			continue
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".iso") {
			return filepath.Join(savePath, entry.Name())
		}
	}

	return ""
}

// DetectDiscPath 定位种子的盘根路径（§59.319 P1 识别门磁盘侧）。
// 收紧语义（P3 事故修复——243 HDT 158 误入队案）：WEB-DL mkv 单文件种子的
// savePath=HDT 根目录，原实现回退到 DetectBDPath(savePath) 子目录扫描，
// 命中邻居盘（Alice 的 BDMV）→ BDInfo 错挂 mkv 簇。现规则：
//   - findTorrentEntry 必须精确定位到种子内容（失败不回退——savePath 级
//     扫描天然会命中邻居盘）
//   - 定位到文件：仅 .iso 算原盘（mkv/mp4/m2ts 单文件=非盘）
//   - 定位到目录：仅"BDMV 直下"算原盘（不向上找父、不扫子目录——多盘
//     合集种子挂账不支持）
func (s *BDInfoScanner) DetectDiscPath(savePath, name string) string {
	entryPath, isDir := findTorrentEntry(savePath, name)
	if entryPath == "" {
		return ""
	}
	if !isDir {
		if strings.EqualFold(filepath.Ext(entryPath), ".iso") {
			return entryPath
		}
		return ""
	}
	if info, err := os.Stat(filepath.Join(entryPath, "BDMV")); err == nil && info.IsDir() {
		return entryPath
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
	// §59.319 P3 修复：只扫主 playlist——对齐站方惯例（发站贴的 BDInfo 均为
	// 单主 playlist 形态，CLI --main 同款）。原全 playlist 报告多块 QUICK
	// SUMMARY 致解析器跨块累计（243 Alice 6 轨变 18 实锤），且全 playlist
	// 扫描耗时更长
	settings.MainPlaylistOnly = true

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

	// §59.319 P1: 30min 上限（原 5min——90G 盘实测 ~11min 会拦腰截断；
	// ctx 取消新版库真实生效——P0 真盘对拍同源）
	scanCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
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

