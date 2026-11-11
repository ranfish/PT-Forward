// Package classify §59.514 刀1: 孤儿目录分类器独立包（原 util.ClassifyTorrent
// 迁移）——S/E 形态判定改消费 titleparser.ParseSEForm 单点内核（两套平行
// 体系归一）；依赖方向 classify→titleparser/util 单向（titleparser 已依赖
// util 的组名提取，util 不能反向依赖 titleparser——故本包独立）。
package classify

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/titleparser"
	"github.com/ranfish/pt-forward/internal/util"
)

// videoExtensions 视频后缀（util 同款副本——util.content_type 未导出；
// 归一时保留副本而非导出 util 变量，避免 util 公共面扩张）。
var videoExtensions = []string{".mkv", ".mp4", ".avi", ".ts", ".m2ts", ".wmv", ".flv", ".mov"}

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
	// §59.187 ②: EP 范围签名——EP01-E06 / E01-E12 / EP01-06
	reEPRange = regexp.MustCompile(`(?i)\bE?P?(\d{1,2})\s*-\s*E?P?(\d{1,3})\b`)

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
		if util.DetectContentType(fileTree) == "music" {
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
	// §59.514 刀1: S/E 形态判定迁移 titleparser.ParseSEForm 单点
	// （原 reSxxExx/reEPRange/reChineseAll/reChineseRange/reChineseSingle
	// 五正则退役——裸 Exx 盲区与发布链同步根治；v1.05 语义：S01=单季
	// 合集声明，原"S01 无 COMPLETE=unknown"保守判定退役，文件树修正层
	// refineFormFromFileTree 兜底半季场景）
	sef := titleparser.ParseSEForm(title)
	if sef.Form == titleparser.FormUnknown {
		result.Category = "movie" // 无 S/E 信号 → 电影（动漫电影等天然无信号）
		return
	}
	result.Category = "tv_series"
	result.Form = sef.Form
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
