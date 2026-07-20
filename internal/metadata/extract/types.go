// Package extract 类型定义（§56.8 数据模型）。
package extract

import (
	"regexp"
	"strings"
	"time"
)

// IntroData 简介分段（转载语义核心，PTGen 不产 Statement/Screenshots）。
type IntroData struct {
	Statement                string   `json:"statement"`                  // 官组声明（始终保留）
	Poster                   string   `json:"poster"`                     // 海报（URL 或 HTML 片段）
	Body                     string   `json:"body"`                       // 正文 BBCode
	Screenshots              string   `json:"screenshots"`                // 截图 BBCode（[img]url[/img] 拼接）
	RemovedARDTUDeclarations []string `json:"removed_ardtu_declarations"` // 工具签名剥离记录
}

var imgURLRe = regexp.MustCompile(`(?i)\[img\](.*?)\[/img\]`)

// ScreenshotURLs 拆分 Screenshots BBCode 为 URL 数组。
func (i IntroData) ScreenshotURLs() []string {
	if i.Screenshots == "" {
		return nil
	}
	matches := imgURLRe.FindAllStringSubmatch(i.Screenshots, -1)
	urls := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) > 1 && strings.TrimSpace(m[1]) != "" {
			urls = append(urls, strings.TrimSpace(m[1]))
		}
	}
	return urls
}

// SetScreenshotURLs 将 URL 数组打包为 [img]url[/img] BBCode。
func (i *IntroData) SetScreenshotURLs(urls []string) {
	if len(urls) == 0 {
		i.Screenshots = ""
		return
	}
	parts := make([]string, 0, len(urls))
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u != "" {
			parts = append(parts, "[img]"+u+"[/img]")
		}
	}
	i.Screenshots = strings.Join(parts, "\n")
}

// SeedData 详情页提取器输出。
type SeedData struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Intro    IntroData `json:"intro"`
	MediaInfo string `json:"mediainfo"`
	BDInfo    string `json:"bdinfo"`

	// 结构化字段（对齐 TorrentDetail，便于双向转换）
	Type         string   `json:"type"`          // 电影/电视剧/动漫/...
	Medium       string   `json:"medium"`        // Blu-ray/Remux/WEB-DL/...
	VideoCodec   string   `json:"video_codec"`   // H.264/AVC、H.265/HEVC
	AudioCodec   string   `json:"audio_codec"`   // DTS-HD MA、TrueHD
	Resolution   string   `json:"resolution"`    // 1080p、2160p
	Source       string   `json:"source"`        // 产地：中国大陆/日本/...
	ReleaseGroup string   `json:"release_group"` // 制作组：FRDS、PTer
	Tags         []string `json:"tags"`
	Flags        []string `json:"flags"` // 禁转/限转

	IMDbLink   string `json:"imdb_link"`
	DoubanLink string `json:"douban_link"`
	TMDbLink   string `json:"tmdb_link"`

	InfoHash string `json:"info_hash"`
	Size     int64  `json:"size"`

	// yaml 配置回显（保留 PTNexus 风格，便于调试与扩展）
	SourceParams map[string]any `json:"source_params,omitempty"`
}

// IsMeaningful 判断提取结果是否有有效字段，供 Engine 回退逻辑使用。
// 至少 Title/Intro.Body/MediaInfo 之一非空才视为有意义。
func (d SeedData) IsMeaningful() bool {
	return strings.TrimSpace(d.Title) != "" ||
		strings.TrimSpace(d.Intro.Body) != "" ||
		strings.TrimSpace(d.MediaInfo) != "" ||
		strings.TrimSpace(d.BDInfo) != ""
}

// NormalizeWithFallback 统一裁剪值并补齐默认容器。
// 当 Title 为空时用 fallbackTitle 填充。
func (d SeedData) NormalizeWithFallback(fallbackTitle string) SeedData {
	d.Title = strings.TrimSpace(d.Title)
	d.Subtitle = strings.TrimSpace(d.Subtitle)
	d.Intro.Statement = strings.TrimSpace(d.Intro.Statement)
	d.Intro.Poster = strings.TrimSpace(d.Intro.Poster)
	d.Intro.Body = strings.TrimSpace(d.Intro.Body)
	d.Intro.Screenshots = strings.TrimSpace(d.Intro.Screenshots)
	d.MediaInfo = strings.TrimSpace(d.MediaInfo)
	d.BDInfo = strings.TrimSpace(d.BDInfo)
	d.Type = strings.TrimSpace(d.Type)
	d.Medium = strings.TrimSpace(d.Medium)
	d.VideoCodec = strings.TrimSpace(d.VideoCodec)
	d.AudioCodec = strings.TrimSpace(d.AudioCodec)
	d.Resolution = strings.TrimSpace(d.Resolution)
	d.Source = strings.TrimSpace(d.Source)
	d.ReleaseGroup = strings.TrimSpace(d.ReleaseGroup)
	d.IMDbLink = strings.TrimSpace(d.IMDbLink)
	d.DoubanLink = strings.TrimSpace(d.DoubanLink)
	d.TMDbLink = strings.TrimSpace(d.TMDbLink)
	d.InfoHash = strings.TrimSpace(d.InfoHash)

	if d.Title == "" {
		d.Title = strings.TrimSpace(fallbackTitle)
	}
	if d.Tags == nil {
		d.Tags = []string{}
	}
	if d.Flags == nil {
		d.Flags = []string{}
	}
	return d
}

// Meta 提取器路由信息（用于日志追踪）。
type Meta struct {
	ExtractorName  string        `json:"extractor_name"`
	UsedFallback   bool          `json:"used_fallback"`
	FallbackReason string        `json:"fallback_reason,omitempty"`
	Duration       time.Duration `json:"duration"`
}

// Input 提取器执行上下文。
type Input struct {
	SiteCode      string `json:"site_code"`      // "ssd" / "pterclub"（fallback key）
	Domain        string `json:"domain"`         // "pterclub.net"（主键，v0.0.253）
	SiteNickname  string `json:"site_nickname"`  // "不可说" / "猫站"
	BaseURL       string `json:"base_url"`
	Cookie        string `json:"-"`              // 不序列化（敏感）
	TorrentID     string `json:"torrent_id"`
	PageHTML      string `json:"-"`              // 不序列化（体积大）
	FallbackTitle string `json:"fallback_title"`
}

// Extractor 站点特殊提取器接口（§56.13 各站实现）。
// public 提取器（§56.10）也实现此接口。
type Extractor interface {
	Name() string
	Extract(input Input) (SeedData, error)
}
