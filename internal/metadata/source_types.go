// Package metadata 三源 JSON 类型（§56.8）。
// 三源 = 详情页采集 / PTGen API / 本地产物。
package metadata

import (
	"time"

	"github.com/ranfish/pt-forward/internal/metadata/extract"
	"github.com/ranfish/pt-forward/internal/model"
)

// DetailSourceJSON 详情页采集原始结果。
type DetailSourceJSON struct {
	Title         string            `json:"title"`
	Subtitle      string            `json:"subtitle"`
	Intro         extract.IntroData `json:"intro"`
	MediaInfo     string            `json:"mediainfo"`
	BDInfo        string            `json:"bdinfo"`
	Type          string            `json:"type"`
	Medium        string            `json:"medium"`
	VideoCodec    string            `json:"video_codec"`
	AudioCodec    string            `json:"audio_codec"`
	Resolution    string            `json:"resolution"`
	Source        string            `json:"source"`
	ReleaseGroup  string            `json:"release_group"`
	Tags          []string          `json:"tags"`
	Flags         []string          `json:"flags"`
	IMDbLink      string            `json:"imdb_link"`
	DoubanLink    string            `json:"douban_link"`
	TMDbLink      string            `json:"tmdb_link"`
	FetchedAt     time.Time         `json:"fetched_at"`
	ExtractorInfo extract.Meta      `json:"extractor_info"`
}

// PTGenSourceJSON PTGen API 原始结果（内嵌 PTGenResult，字段不重复定义）。
type PTGenSourceJSON struct {
	model.PTGenResult
	FetchedAt time.Time `json:"fetched_at"`
}

// LocalSourceJSON 本地产物原始结果。
type LocalSourceJSON struct {
	MediaInfo   string   `json:"mediainfo"`
	BDInfo      string   `json:"bdinfo"`
	Screenshots []string `json:"screenshots"`
	GeneratedAt time.Time `json:"generated_at"`
}
