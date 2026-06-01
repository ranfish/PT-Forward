package qbittorrent

import (
	"strings"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
)

type qbTorrent struct {
	Hash          string  `json:"hash"`
	Name          string  `json:"name"`
	TotalSize     int64   `json:"total_size"`
	Progress      float64 `json:"progress"`
	Uploaded      int64   `json:"uploaded"`
	Downloaded    int64   `json:"downloaded"`
	UploadSpeed   int64   `json:"upspeed"`
	DownloadSpeed int64   `json:"dlspeed"`
	Ratio         float64 `json:"ratio"`
	State         string  `json:"state"`
	SavePath      string  `json:"save_path"`
	Category      string  `json:"category"`
	Tags          string  `json:"tags"`
	NumComplete   int     `json:"num_complete"`
	NumIncomplete int     `json:"num_incomplete"`
	SeedingTime   int64   `json:"seeding_time"`
	AddedOn       int64   `json:"added_on"`
}

func (t qbTorrent) toModel() *model.TorrentInfo {
	var tags []string
	if t.Tags != "" {
		for _, tag := range strings.Split(t.Tags, ", ") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}
	if tags == nil {
		tags = []string{}
	}

	var errStr string
	if t.State == "error" || t.State == "missingFiles" {
		errStr = t.State
	}

	return &model.TorrentInfo{
		Hash:          t.Hash,
		Name:          t.Name,
		IsFinished:    t.Progress >= 1.0,
		IsPaused:      t.State == "pausedDL" || t.State == "pausedUP" || t.State == "stoppedDL" || t.State == "stoppedUP",
		Removed:       false,
		State:         t.State,
		Error:         errStr,
		NumComplete:   t.NumComplete,
		NumIncomplete: t.NumIncomplete,
		Ratio:         t.Ratio,
		SavePath:      t.SavePath,
		Tags:          tags,
		TotalSize:     t.TotalSize,
		Category:      t.Category,
		Progress:      t.Progress,
		Uploaded:      t.Uploaded,
		Downloaded:    t.Downloaded,
		UploadSpeed:   t.UploadSpeed,
		DownloadSpeed: t.DownloadSpeed,
		SeedTime:      t.SeedingTime,
		AddedAt:       time.Unix(t.AddedOn, 0),
	}
}
