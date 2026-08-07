package util

import "strings"

var audioExtensions = []string{".flac", ".wav", ".ape", ".tta", ".wv", ".mp3", ".m4a", ".ogg", ".opus", ".aac", ".dsf", ".dff", ".wma", ".aiff", ".m4b"}

var videoExtensions = []string{".mkv", ".mp4", ".avi", ".ts", ".m2ts", ".wmv", ".flv", ".mov"}

// DetectContentType 根据文件树判断种子内容类型。
// 有音频文件且无视频文件 → "music"，否则 → "video"。
func DetectContentType(fileTree map[string]int64) string {
	if len(fileTree) == 0 {
		return "video"
	}
	hasAudio := false
	hasVideo := false
	for path := range fileTree {
		lower := strings.ToLower(path)
		for _, ext := range audioExtensions {
			if strings.HasSuffix(lower, ext) {
				hasAudio = true
			}
		}
		for _, ext := range videoExtensions {
			if strings.HasSuffix(lower, ext) {
				hasVideo = true
			}
		}
	}
	if hasAudio && !hasVideo {
		return "music"
	}
	return "video"
}

// ContentTypeCompatible 判断种子内容类型与站点类型是否兼容。
// 站点 content_type 值域：music=纯音乐站, video=纯视频站, ""=综合站。
// 规则：两者都有值且不同 → 不兼容；任一为空（综合站/未知）→ 兼容。
func ContentTypeCompatible(torrentType, siteType string) bool {
	if torrentType == "" || siteType == "" {
		return true
	}
	return torrentType == siteType
}

// SizeDiffPercent 计算两个体积的差异百分比。
// 返回 |sourceSize-targetSize| / sourceSize * 100，sourceSize<=0 时返回 -1。
func SizeDiffPercent(sourceSize, targetSize int64) float64 {
	if sourceSize <= 0 {
		return -1
	}
	diff := targetSize - sourceSize
	if diff < 0 {
		diff = -diff
	}
	return float64(diff) / float64(sourceSize) * 100
}

// SizeWithinTolerance 判断目标体积是否在源体积的容差范围内。
// tolerancePercent 为百分比阈值（如 1.0 表示 1%）。
// sourceSize<=0 或 targetSize<=0 时返回 true（跳过校验）。
func SizeWithinTolerance(sourceSize, targetSize int64, tolerancePercent float64) bool {
	if sourceSize <= 0 || targetSize <= 0 {
		return true
	}
	return SizeDiffPercent(sourceSize, targetSize) <= tolerancePercent
}
