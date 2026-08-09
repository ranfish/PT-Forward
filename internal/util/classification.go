package util

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// TorrentType 表示种子的分类结果。
type TorrentType struct {
	Category string `json:"category"` // movie / tv_series / music / other / unknown
	Form     string `json:"form"`     // season_pack / partial_pack / single_episode / unknown（仅 episode 类内容有意义）
	Source   string `json:"source"`   // title / file_tree / mixed
}

// VideoFileInfo 保存视频文件名和大小。
type VideoFileInfo struct {
	Name string
	Size int64
}

// DirClassification 是孤儿目录的分类结果。
type DirClassification struct {
	Type       TorrentType
	VideoFiles []VideoFileInfo // 按大小降序排列
	TotalSize  int64
}

var (
	// S01 / S01E03 / S01E01-E12 / S01 E03 / S01 E01-E03
	// 捕获组：season, firstEpisode(可空), lastEpisode(可空)
	reSxxExx = regexp.MustCompile(`(?i)(?:^|[\s._\-])S(\d{1,2})(?:\s*E(\d{1,3})(?:\s*[-~]\s*E?(\d{1,3}))?)?`)

	// 文件名中的单集模式 S01E03（用于文件树分析）
	reFileEpisode = regexp.MustCompile(`(?i)S\d{1,2}E(\d{1,3})`)

	// 中文标记
	reChineseAll    = regexp.MustCompile(`全(\d+)集`)
	reChineseAllEnd = regexp.MustCompile(`(\d+)集全`)
	reChineseSingle = regexp.MustCompile(`第(\d+)集`)
	reChineseRange  = regexp.MustCompile(`第(\d+)[-~](\d+)集`)
)

// ClassifyTorrent 从标题和文件树判定种子类型。
// title: 种子标题或目录名（必填）
// fileTree: 文件路径→大小映射（可选，提高精度）
func ClassifyTorrent(title string, fileTree map[string]int64) TorrentType {
	// 1. 音乐检测（文件树优先）
	if len(fileTree) > 0 {
		if DetectContentType(fileTree) == "music" {
			return TorrentType{Category: "music", Source: "file_tree"}
		}
	}

	result := TorrentType{Category: "unknown", Source: "title"}

	// 2. 标题分类
	classifyFromTitle(title, &result)

	// 3. 文件树修正 Form（文件树是事实，优先于标题）
	if len(fileTree) > 0 {
		refineFormFromFileTree(fileTree, &result)
	}

	return result
}

func classifyFromTitle(title string, result *TorrentType) {
	if title == "" {
		return
	}

	// 中文标记（最显式，优先检查）
	if reChineseAll.MatchString(title) || reChineseAllEnd.MatchString(title) {
		result.Category = "tv_series"
		result.Form = "season_pack"
		return
	}
	if m := reChineseRange.FindStringSubmatch(title); m != nil {
		result.Category = "tv_series"
		start, _ := strconv.Atoi(m[1])
		if start == 1 {
			result.Form = "season_pack"
		} else {
			result.Form = "partial_pack"
		}
		return
	}
	if reChineseSingle.MatchString(title) {
		result.Category = "tv_series"
		result.Form = "single_episode"
		return
	}

	// 英文模式
	upper := strings.ToUpper(title)
	hasComplete := strings.Contains(upper, "COMPLETE")

	match := reSxxExx.FindStringSubmatch(title)
	if match != nil {
		result.Category = "tv_series"
		if match[2] != "" {
			// 有集号
			if match[3] != "" {
				// 区间 S01E01-E12
				startEp, _ := strconv.Atoi(match[2])
				if startEp == 1 {
					result.Form = "season_pack"
				} else {
					result.Form = "partial_pack"
				}
			} else {
				// 单集 S01E03
				result.Form = "single_episode"
			}
		} else {
			// 仅季号 S01
			if hasComplete {
				result.Form = "season_pack"
			} else {
				result.Form = "unknown"
			}
		}
		return
	}

	// 无季集模式 → 电影
	result.Category = "movie"
}

func refineFormFromFileTree(fileTree map[string]int64, result *TorrentType) {
	episodeCount := 0
	minEpisode := 9999

	for path := range fileTree {
		filename := filepath.Base(path)
		if m := reFileEpisode.FindStringSubmatch(filename); m != nil {
			episodeCount++
			epNum, _ := strconv.Atoi(m[1])
			if epNum < minEpisode {
				minEpisode = epNum
			}
		}
	}

	if episodeCount == 0 {
		return
	}

	// 文件树含 SxxExx 文件 → 修正 Form
	if episodeCount == 1 {
		result.Form = "single_episode"
	} else if minEpisode == 1 {
		result.Form = "season_pack"
	} else {
		result.Form = "partial_pack"
	}

	// 文件树说有集数但标题说电影 → 修正为剧集
	if result.Category == "movie" {
		result.Category = "tv_series"
	}
	result.Source = "mixed"
}

// ClassifyFromDir 扫描孤儿目录并分类。
// 返回分类结果 + 视频文件列表（按大小降序）+ 总大小。
func ClassifyFromDir(dirPath, dirName string) (*DirClassification, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	fileTree := make(map[string]int64)
	var videoFiles []VideoFileInfo
	var totalSize int64

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		name := entry.Name()
		size := info.Size()
		fileTree[name] = size
		totalSize += size

		ext := strings.ToLower(filepath.Ext(name))
		for _, e := range videoExtensions {
			if ext == e {
				videoFiles = append(videoFiles, VideoFileInfo{Name: name, Size: size})
				break
			}
		}
	}

	sort.Slice(videoFiles, func(i, j int) bool {
		return videoFiles[i].Size > videoFiles[j].Size
	})

	return &DirClassification{
		Type:       ClassifyTorrent(dirName, fileTree),
		VideoFiles: videoFiles,
		TotalSize:  totalSize,
	}, nil
}
