package seeding

import (
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/rule"
)

func toRuleContext(rec *model.SeedingTorrentRecord, ti *model.TorrentInfo, freeSpace, totalSpace int64, now time.Time, activeUploads, activeDownloads int, globalUpSpeed, globalDownSpeed float64, scoringScore float64, scoringRank, lowScoreCount int) *rule.Context {
	ctx := &rule.Context{
		SiteName:          rec.SiteName,
		Status:            string(rec.Status),
		IsFree:            rec.IsFree,
		HasHR:             rec.HasHR,
		HRSeedTimeH:       rec.HRSeedTimeH,
		Discount:          string(rec.Discount),
		ClientID:          rec.ClientID,
		TorrentID:         rec.TorrentID,
		FreeLevel:         rec.FreeLevel,
		Source:            rec.Source,
		LastActionBy:      rec.LastActionBy,
		Unregistered:      rec.Unregistered,
		UnregisteredMsg:   rec.UnregisteredMsg,
		SubscriptionID:    rec.SubscriptionID,
		FreeEndAt:         rec.FreeEndAt,
		FreeSpace:         freeSpace,
		TotalSpace:        totalSpace,
		Now:               now,
		ActiveUploads:     activeUploads,
		ActiveDownloads:   activeDownloads,
		GlobalUploadSpeed: globalUpSpeed,
		GlobalDownloadSpeed: globalDownSpeed,
		ScoringScore:      scoringScore,
		ScoringRank:       scoringRank,
		LowScoreCount:     lowScoreCount,
	}

	if ti != nil {
		ctx.InfoHash = ti.Hash
		ctx.Name = ti.Name
		ctx.SavePath = ti.SavePath
		ctx.TotalSize = ti.TotalSize
		ctx.TrackerURL = ti.TrackerURL
		ctx.Category = ti.Category
		ctx.Tags = ti.Tags
		ctx.Ratio = ti.Ratio
		ctx.Progress = ti.Progress
		ctx.UploadSpeed = ti.UploadSpeed
		ctx.DownloadSpeed = ti.DownloadSpeed
		ctx.Uploaded = ti.Uploaded
		ctx.Downloaded = ti.Downloaded
		ctx.SeedTime = ti.SeedTime
		ctx.State = ti.State
		ctx.ErrorMsg = ti.Error
		ctx.NumComplete = ti.NumComplete
		ctx.NumIncomplete = ti.NumIncomplete
		ctx.IsFinished = ti.IsFinished
		ctx.IsPaused = ti.IsPaused
		ctx.AddedAt = ti.AddedAt
	}

	return ctx
}
