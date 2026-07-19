package metadata

import (
	"github.com/ranfish/pt-forward/internal/metadata/extract"
	"github.com/ranfish/pt-forward/internal/model"
)

// MergedMetadata 三源合并后的统一表示，传给渲染层和表单填充层。
type MergedMetadata struct {
	// SourceOf 字段来源标注（UI 显示"标题来自详情页"等提示）
	SourceOf map[string]string `json:"source_of"`

	// 字段（最终值）
	Title        string            `json:"title"`
	Subtitle     string            `json:"subtitle"`
	Intro        extract.IntroData `json:"intro"`
	MediaInfo    string            `json:"mediainfo"`
	BDInfo       string            `json:"bdinfo"`
	Type         string            `json:"type"`
	Medium       string            `json:"medium"`
	VideoCodec   string            `json:"video_codec"`
	AudioCodec   string            `json:"audio_codec"`
	Resolution   string            `json:"resolution"`
	Source       string            `json:"source"`
	ReleaseGroup string            `json:"release_group"`
	Tags         []string          `json:"tags"`
	Flags        []string          `json:"flags"`
	IMDbURL      string            `json:"imdb_url"`
	DoubanURL    string            `json:"douban_url"`
	TMDbURL      string            `json:"tmdb_url"`
	InfoHash     string            `json:"info_hash"`
	Size         int64             `json:"size"`

	// PTGen 独有字段（始终保留，detail_first 模式不影响）
	PTGen model.PTGenResult `json:"ptgen"`

	// §56.18: 海报来源标注（由 PosterResolver 设置，Merge 函数不设置）
	// 值: "ptgen" / "detail" / ""（空表示无可用海报，用户需手动填写 URL）
	PosterSource string `json:"poster_source,omitempty"`
}

// MergeMode 合并模式（对应 UI toggle）。
type MergeMode string

const (
	MergeModePTGenFirst  MergeMode = "ptgen_first"  // 默认
	MergeModeDetailFirst MergeMode = "detail_first"
)

// Source 来源标签。
const (
	SourceDetail = "detail"
	SourcePTGen  = "ptgen"
	SourceLocal  = "local"
)

// Merge 三源合并的纯函数。
// 任一源指针为 nil 表示该源未取到（跳过，不影响其他源）。
//
// 合并规则（§56.8 字段级合并表）：
//   - 始终保留：Statement / Tags / Flags / Type/VideoCodec/AudioCodec/Resolution/Medium/Source/ReleaseGroup
//   - PTGen 独有字段（Cast/Writer/Awards/AKA/Genre/Episodes 等）始终保留
//   - MediaInfo/BDInfo：Local > Detail（local 优先，因为本地产物最新鲜）
//   - Screenshots：Local > Detail
//   - toggle 影响：Title/Subtitle/Intro.Body/Poster/IMDb/Douban/TMDb URL
func Merge(detail *DetailSourceJSON, ptgen *PTGenSourceJSON, local *LocalSourceJSON, mode MergeMode) MergedMetadata {
	m := MergedMetadata{
		SourceOf: make(map[string]string),
		Tags:     []string{},
		Flags:    []string{},
	}

	// 始终保留字段：Statement（来自 Detail）
	if detail != nil {
		m.Intro.Statement = detail.Intro.Statement
		if detail.Intro.Statement != "" {
			m.SourceOf["statement"] = SourceDetail
		}
	}

	// MediaInfo / BDInfo：Local 优先 > Detail
	m.MediaInfo = pickWithSource(&m.SourceOf, "mediainfo",
		local != nil, func() string { return local.MediaInfo }, SourceLocal,
		detail != nil, func() string { return detail.MediaInfo }, SourceDetail,
	)
	m.BDInfo = pickWithSource(&m.SourceOf, "bdinfo",
		local != nil, func() string { return local.BDInfo }, SourceLocal,
		detail != nil, func() string { return detail.BDInfo }, SourceDetail,
	)

	// Screenshots：Local 优先 > Detail
	var screenshots []string
	if local != nil && len(local.Screenshots) > 0 {
		screenshots = local.Screenshots
		m.SourceOf["screenshots"] = SourceLocal
	} else if detail != nil {
		screenshots = detail.Intro.ScreenshotURLs()
		if len(screenshots) > 0 {
			m.SourceOf["screenshots"] = SourceDetail
		}
	}
	m.Intro.SetScreenshotURLs(screenshots)

	// 结构化字段（始终保留 Detail 的值）
	if detail != nil {
		m.Type = detail.Type
		m.Medium = detail.Medium
		m.VideoCodec = detail.VideoCodec
		m.AudioCodec = detail.AudioCodec
		m.Resolution = detail.Resolution
		m.Source = detail.Source
		m.ReleaseGroup = detail.ReleaseGroup
		m.Tags = appendNonEmpty(m.Tags, detail.Tags)
		m.Flags = appendNonEmpty(m.Flags, detail.Flags)
	}

	// toggle 影响字段
	if mode == MergeModeDetailFirst {
		mergeDetailFirst(&m, detail, ptgen)
	} else {
		mergePTGenFirst(&m, detail, ptgen)
	}

	// PTGen 独有字段始终保留
	if ptgen != nil {
		m.PTGen = ptgen.PTGenResult
	}

	return m
}

// mergePTGenFirst PTGen 优先合并模式（默认）。
func mergePTGenFirst(m *MergedMetadata, detail *DetailSourceJSON, ptgen *PTGenSourceJSON) {
	// 标题：PTGen.ChineseTitle > Detail.Title
	if ptgen != nil && ptgen.ChineseTitle != "" {
		m.Title = ptgen.ChineseTitle
		m.SourceOf["title"] = SourcePTGen
	} else if detail != nil {
		m.Title = detail.Title
		if m.Title != "" {
			m.SourceOf["title"] = SourceDetail
		}
	}

	// 副标题：PTGen.ForeignTitle > Detail.Subtitle
	if ptgen != nil && ptgen.ForeignTitle != "" {
		m.Subtitle = ptgen.ForeignTitle
		m.SourceOf["subtitle"] = SourcePTGen
	} else if detail != nil {
		m.Subtitle = detail.Subtitle
		if m.Subtitle != "" {
			m.SourceOf["subtitle"] = SourceDetail
		}
	}

	// 简介 Body：PTGen.Introduction > Detail.Intro.Body
	if ptgen != nil && ptgen.Introduction != "" {
		m.Intro.Body = ptgen.Introduction
		m.SourceOf["body"] = SourcePTGen
	} else if detail != nil {
		m.Intro.Body = detail.Intro.Body
		if m.Intro.Body != "" {
			m.SourceOf["body"] = SourceDetail
		}
	}

	// 海报：PTGen.PosterURL > Detail.Intro.Poster
	if ptgen != nil && ptgen.PosterURL != "" {
		m.Intro.Poster = ptgen.PosterURL
		m.SourceOf["poster"] = SourcePTGen
	} else if detail != nil {
		m.Intro.Poster = detail.Intro.Poster
		if m.Intro.Poster != "" {
			m.SourceOf["poster"] = SourceDetail
		}
	}

	// IMDb URL：PTGen > Detail
	if ptgen != nil && ptgen.IMDBURL != "" {
		m.IMDbURL = ptgen.IMDBURL
		m.SourceOf["imdb_url"] = SourcePTGen
	} else if detail != nil {
		m.IMDbURL = detail.IMDbLink
		if m.IMDbURL != "" {
			m.SourceOf["imdb_url"] = SourceDetail
		}
	}

	// 豆瓣 URL：PTGen > Detail
	if ptgen != nil && ptgen.DoubanURL != "" {
		m.DoubanURL = ptgen.DoubanURL
		m.SourceOf["douban_url"] = SourcePTGen
	} else if detail != nil {
		m.DoubanURL = detail.DoubanLink
		if m.DoubanURL != "" {
			m.SourceOf["douban_url"] = SourceDetail
		}
	}

	// TMDb URL：PTGen > Detail
	if ptgen != nil && ptgen.TMDbURL != "" {
		m.TMDbURL = ptgen.TMDbURL
		m.SourceOf["tmdb_url"] = SourcePTGen
	} else if detail != nil {
		m.TMDbURL = detail.TMDbLink
		if m.TMDbURL != "" {
			m.SourceOf["tmdb_url"] = SourceDetail
		}
	}
}

// mergeDetailFirst Detail 优先合并模式。
func mergeDetailFirst(m *MergedMetadata, detail *DetailSourceJSON, ptgen *PTGenSourceJSON) {
	// 标题：Detail.Title > PTGen.ChineseTitle
	if detail != nil && detail.Title != "" {
		m.Title = detail.Title
		m.SourceOf["title"] = SourceDetail
	} else if ptgen != nil && ptgen.ChineseTitle != "" {
		m.Title = ptgen.ChineseTitle
		m.SourceOf["title"] = SourcePTGen
	}

	// 副标题：Detail.Subtitle > PTGen.ForeignTitle
	if detail != nil && detail.Subtitle != "" {
		m.Subtitle = detail.Subtitle
		m.SourceOf["subtitle"] = SourceDetail
	} else if ptgen != nil && ptgen.ForeignTitle != "" {
		m.Subtitle = ptgen.ForeignTitle
		m.SourceOf["subtitle"] = SourcePTGen
	}

	// 简介 Body：Detail.Intro.Body > PTGen.Introduction
	if detail != nil && detail.Intro.Body != "" {
		m.Intro.Body = detail.Intro.Body
		m.SourceOf["body"] = SourceDetail
	} else if ptgen != nil && ptgen.Introduction != "" {
		m.Intro.Body = ptgen.Introduction
		m.SourceOf["body"] = SourcePTGen
	}

	// 海报：Detail.Intro.Poster > PTGen.PosterURL
	if detail != nil && detail.Intro.Poster != "" {
		m.Intro.Poster = detail.Intro.Poster
		m.SourceOf["poster"] = SourceDetail
	} else if ptgen != nil && ptgen.PosterURL != "" {
		m.Intro.Poster = ptgen.PosterURL
		m.SourceOf["poster"] = SourcePTGen
	}

	// IMDb URL：Detail > PTGen
	if detail != nil && detail.IMDbLink != "" {
		m.IMDbURL = detail.IMDbLink
		m.SourceOf["imdb_url"] = SourceDetail
	} else if ptgen != nil && ptgen.IMDBURL != "" {
		m.IMDbURL = ptgen.IMDBURL
		m.SourceOf["imdb_url"] = SourcePTGen
	}

	// 豆瓣 URL：Detail > PTGen
	if detail != nil && detail.DoubanLink != "" {
		m.DoubanURL = detail.DoubanLink
		m.SourceOf["douban_url"] = SourceDetail
	} else if ptgen != nil && ptgen.DoubanURL != "" {
		m.DoubanURL = ptgen.DoubanURL
		m.SourceOf["douban_url"] = SourcePTGen
	}

	// TMDb URL：Detail > PTGen
	if detail != nil && detail.TMDbLink != "" {
		m.TMDbURL = detail.TMDbLink
		m.SourceOf["tmdb_url"] = SourceDetail
	} else if ptgen != nil && ptgen.TMDbURL != "" {
		m.TMDbURL = ptgen.TMDbURL
		m.SourceOf["tmdb_url"] = SourcePTGen
	}
}

// pickWithSource 双源择优（first 优先），同时记录来源。
func pickWithSource(sourceOf *map[string]string, field string,
	firstOK bool, firstFn func() string, firstSrc string,
	secondOK bool, secondFn func() string, secondSrc string,
) string {
	if firstOK {
		v := firstFn()
		if v != "" {
			(*sourceOf)[field] = firstSrc
			return v
		}
	}
	if secondOK {
		v := secondFn()
		if v != "" {
			(*sourceOf)[field] = secondSrc
			return v
		}
	}
	return ""
}

// appendNonEmpty 追加非空字符串到 dst（去重）。
func appendNonEmpty(dst, src []string) []string {
	if dst == nil {
		dst = []string{}
	}
	seen := make(map[string]struct{}, len(dst))
	for _, s := range dst {
		seen[s] = struct{}{}
	}
	for _, s := range src {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		dst = append(dst, s)
	}
	return dst
}
