package transmission

import (
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
)

type trTorrent struct {
	HashString     string   `json:"hashString"`
	Name           string   `json:"name"`
	TotalSize      int64    `json:"totalSize"`
	PercentDone    float64  `json:"percentDone"`
	UploadedEver   int64    `json:"uploadedEver"`
	RateUpload     int64    `json:"rateUpload"`
	RateDownload   int64    `json:"rateDownload"`
	UploadRatio    float64  `json:"uploadRatio"`
	Status         int      `json:"status"`
	Error          int      `json:"error"`
	ErrorString    string   `json:"errorString"`
	DownloadDir    string   `json:"downloadDir"`
	Labels         []string `json:"labels"`
	AddedDate      int64    `json:"addedDate"`
	SecondsSeeding int64    `json:"secondsSeeding"`
	IsFinished     bool     `json:"isFinished"`
	TrackerStats   []struct {
		SeederCount  int `json:"seederCount"`
		LeecherCount int `json:"leecherCount"`
	} `json:"trackerStats"`
	TorrentFile string `json:"torrentFile"`
	ID          int    `json:"id"`
}

var allFields = []string{
	"hashString", "name", "totalSize", "percentDone", "uploadedEver",
	"rateUpload", "rateDownload", "uploadRatio", "status", "error",
	"errorString", "downloadDir", "labels", "addedDate", "secondsSeeding",
	"isFinished", "trackerStats", "torrentFile", "id",
}

func (t trTorrent) toModel() *model.TorrentInfo {
	var errStr string
	if t.Error != 0 {
		errStr = t.ErrorString
	}

	var tags []string
	var category string
	if len(t.Labels) > 0 {
		category = t.Labels[0]
		tags = make([]string, 0, len(t.Labels)-1)
		for i := 1; i < len(t.Labels); i++ {
			tags = append(tags, t.Labels[i])
		}
	}
	if tags == nil {
		tags = []string{}
	}

	var numComplete, numIncomplete int
	if len(t.TrackerStats) > 0 {
		numComplete = t.TrackerStats[0].SeederCount
		numIncomplete = t.TrackerStats[0].LeecherCount
	}

	return &model.TorrentInfo{
		Hash:          t.HashString,
		Name:          t.Name,
		IsFinished:    t.IsFinished || t.PercentDone >= 1.0,
		IsPaused:      t.Status == 0,
		Removed:       false,
		State:         trStatusToString(t.Status),
		Error:         errStr,
		NumComplete:   numComplete,
		NumIncomplete: numIncomplete,
		Ratio:         t.UploadRatio,
		SavePath:      t.DownloadDir,
		Tags:          tags,
		TotalSize:     t.TotalSize,
		Category:      category,
		Progress:      t.PercentDone,
		Uploaded:      t.UploadedEver,
		UploadSpeed:   t.RateUpload,
		DownloadSpeed: t.RateDownload,
		SeedTime:      t.SecondsSeeding,
		AddedAt:       time.Unix(t.AddedDate, 0),
	}
}

func trStatusToString(status int) string {
	switch status {
	case 0:
		return "stopped"
	case 1, 2:
		return "checking"
	case 3, 4:
		return "downloading"
	case 5, 6:
		return "uploading"
	default:
		return "unknown"
	}
}

func isSeedingStatus(status int) bool {
	return status == 5 || status == 6
}

func mergeLabels(currentLabels []string, category string, tags []string) []string {
	result := make([]string, 0, 1+len(tags))
	if category != "" {
		result = append(result, category)
	} else if len(currentLabels) > 0 {
		result = append(result, currentLabels[0])
	}
	result = append(result, tags...)
	if len(result) == 0 {
		return []string{}
	}
	return result
}

func removeLabels(currentLabels []string, tagsToRemove []string, keepCategory bool) []string {
	removeSet := make(map[string]bool, len(tagsToRemove))
	for _, t := range tagsToRemove {
		removeSet[strings.ToLower(t)] = true
	}
	result := make([]string, 0, len(currentLabels))
	for i, l := range currentLabels {
		if i == 0 && keepCategory {
			result = append(result, l)
			continue
		}
		if !removeSet[strings.ToLower(l)] {
			result = append(result, l)
		}
	}
	return result
}
