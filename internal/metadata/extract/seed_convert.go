package extract

import (
	"github.com/ranfish/pt-forward/internal/model"
)

// SeedToDetail 将 SeedData 转换为 *TorrentDetail（adapter 内部使用）。
// §56.8 决策 1 字段命名映射：
//   - SeedData.Type → Detail.Category（命名差异，语义相同）
//   - SeedData.VideoCodec → Detail.Codec
//   - SeedData.Intro.Body → Detail.Description
//   - SeedData.Intro.Poster → Detail.PosterURL
//   - SeedData.Intro.ScreenshotURLs() → Detail.Screenshots
func SeedToDetail(seed SeedData) *model.TorrentDetail {
	if !seed.IsMeaningful() {
		return nil
	}
	d := &model.TorrentDetail{
		Title:        seed.Title,
		Subtitle:     seed.Subtitle,
		Description:  seed.Intro.Body,
		Category:     seed.Type,
		Source:       seed.Source,
		Resolution:   seed.Resolution,
		Codec:        seed.VideoCodec,
		ReleaseGroup: seed.ReleaseGroup,
		Tags:         seed.Tags,
		Flags:        seed.Flags,
		AudioCodec:   seed.AudioCodec,
		IMDbURL:      seed.IMDbLink,
		DoubanURL:    seed.DoubanLink,
		TMDbURL:      seed.TMDbLink,
		PosterURL:    seed.Intro.Poster,
		Screenshots:  seed.Intro.ScreenshotURLs(),
		MediaInfo:    seed.MediaInfo,
		BDInfo:       seed.BDInfo,
		Size:         seed.Size,
		InfoHash:     seed.InfoHash,
	}
	if d.Tags == nil {
		d.Tags = []string{}
	}
	if d.Flags == nil {
		d.Flags = []string{}
	}
	if d.Screenshots == nil {
		d.Screenshots = []string{}
	}
	return d
}

// DetailToSeed 将 *TorrentDetail 转换为 SeedData（API adapter JSON → SeedData 时使用）。
func DetailToSeed(detail *model.TorrentDetail) SeedData {
	if detail == nil {
		return SeedData{}
	}
	seed := SeedData{
		Title:        detail.Title,
		Subtitle:     detail.Subtitle,
		MediaInfo:    detail.MediaInfo,
		BDInfo:       detail.BDInfo,
		Type:         detail.Category,
		VideoCodec:   detail.Codec,
		AudioCodec:   detail.AudioCodec,
		Resolution:   detail.Resolution,
		Source:       detail.Source,
		ReleaseGroup: detail.ReleaseGroup,
		Tags:         detail.Tags,
		Flags:        detail.Flags,
		IMDbLink:     detail.IMDbURL,
		DoubanLink:   detail.DoubanURL,
		TMDbLink:     detail.TMDbURL,
		InfoHash:     detail.InfoHash,
		Size:         detail.Size,
	}
	seed.Intro.Body = detail.Description
	seed.Intro.Poster = detail.PosterURL
	seed.Intro.SetScreenshotURLs(detail.Screenshots)
	if seed.Tags == nil {
		seed.Tags = []string{}
	}
	if seed.Flags == nil {
		seed.Flags = []string{}
	}
	return seed
}
