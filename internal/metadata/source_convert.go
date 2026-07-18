package metadata

import (
	"time"

	"github.com/ranfish/pt-forward/internal/metadata/extract"
	"github.com/ranfish/pt-forward/internal/model"
)

// SeedToDetailSource 将 SeedData 封装为 DetailSourceJSON（详情源持久化）。
func SeedToDetailSource(seed extract.SeedData, fetchedAt time.Time, meta extract.Meta) DetailSourceJSON {
	return DetailSourceJSON{
		Title:         seed.Title,
		Subtitle:      seed.Subtitle,
		Intro:         seed.Intro,
		MediaInfo:     seed.MediaInfo,
		BDInfo:        seed.BDInfo,
		Type:          seed.Type,
		Medium:        seed.Medium,
		VideoCodec:    seed.VideoCodec,
		AudioCodec:    seed.AudioCodec,
		Resolution:    seed.Resolution,
		Source:        seed.Source,
		ReleaseGroup:  seed.ReleaseGroup,
		Tags:          seed.Tags,
		Flags:         seed.Flags,
		IMDbLink:      seed.IMDbLink,
		DoubanLink:    seed.DoubanLink,
		TMDbLink:      seed.TMDbLink,
		FetchedAt:     fetchedAt,
		ExtractorInfo: meta,
	}
}

// DetailSourceToSeed 将 DetailSourceJSON 还原为 SeedData（合并层使用）。
func DetailSourceToSeed(src DetailSourceJSON) extract.SeedData {
	return extract.SeedData{
		Title:        src.Title,
		Subtitle:     src.Subtitle,
		Intro:        src.Intro,
		MediaInfo:    src.MediaInfo,
		BDInfo:       src.BDInfo,
		Type:         src.Type,
		Medium:       src.Medium,
		VideoCodec:   src.VideoCodec,
		AudioCodec:   src.AudioCodec,
		Resolution:   src.Resolution,
		Source:       src.Source,
		ReleaseGroup: src.ReleaseGroup,
		Tags:         src.Tags,
		Flags:        src.Flags,
		IMDbLink:     src.IMDbLink,
		DoubanLink:   src.DoubanLink,
		TMDbLink:     src.TMDbLink,
	}
}

// PTGenToSource 将 PTGenResult 封装为 PTGenSourceJSON（PTGen 源持久化）。
func PTGenToSource(result model.PTGenResult, fetchedAt time.Time) PTGenSourceJSON {
	return PTGenSourceJSON{
		PTGenResult: result,
		FetchedAt:   fetchedAt,
	}
}

// SourceToPTGen 将 PTGenSourceJSON 还原为 PTGenResult（合并层使用）。
func SourceToPTGen(src PTGenSourceJSON) model.PTGenResult {
	return src.PTGenResult
}

// ArtifactToSource 将本地产物封装为 LocalSourceJSON（本地源持久化）。
func ArtifactToSource(mediainfo, bdinfo string, screenshots []string, generatedAt time.Time) LocalSourceJSON {
	return LocalSourceJSON{
		MediaInfo:   mediainfo,
		BDInfo:      bdinfo,
		Screenshots: screenshots,
		GeneratedAt: generatedAt,
	}
}

// SourceToArtifact 将 LocalSourceJSON 还原为本地产物（合并层使用）。
func SourceToArtifact(src LocalSourceJSON) (mediainfo, bdinfo string, screenshots []string) {
	return src.MediaInfo, src.BDInfo, src.Screenshots
}
