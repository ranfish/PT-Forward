package titleparser

import (
	"strings"
)

// InferCategory 从多源信号推断类型分类
// 优先级: 源站分类 > 标题季集 > PTGen genre > 默认电影
func InferCategory(c TitleComponents, sourceCategory string, ptgenGenre string, ptgenEpisodes string) string {
	// ① 源站分类优先（最可信）
	if cat := normalizeSourceCategory(sourceCategory); cat != "" {
		return cat
	}

	// ② 标题季集 → 剧集
	if c.SeasonEpisode != "" {
		return "category.tv_series"
	}

	// ③ PTGen genre 辅助
	genreLower := strings.ToLower(ptgenGenre)
	if genreLower != "" {
		if containsAny(genreLower, "动画", "anime", "animation") {
			return "category.animation"
		}
		if containsAny(genreLower, "纪录", "documentary") {
			return "category.documentary"
		}
		if containsAny(genreLower, "综艺", "variety", "show") {
			return "category.tv_shows"
		}
		if containsAny(genreLower, "音乐", "music", "演唱会", "concert") {
			return "category.music"
		}
		if containsAny(genreLower, "体育", "sport") {
			return "category.sports"
		}
	}

	// ④ PTGen episodes > 1 → 剧集
	if ptgenEpisodes != "" && ptgenEpisodes != "0" && ptgenEpisodes != "1" {
		return "category.tv_series"
	}

	// ⑤ 默认电影
	return "category.movie"
}

// normalizeSourceCategory 将源站分类归一化为标准键
func normalizeSourceCategory(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	lower := strings.ToLower(raw)

	switch {
	case containsAny(lower, "电影", "movie"):
		return "category.movie"
	case containsAny(lower, "电视剧", "剧集", "tv series", "tv-series", "series"):
		return "category.tv_series"
	case containsAny(lower, "综艺", "variety", "tv show", "show"):
		return "category.tv_shows"
	case containsAny(lower, "动画", "动漫", "anime", "animation", "cartoon"):
		return "category.animation"
	case containsAny(lower, "纪录", "documentary", "document"):
		return "category.documentary"
	case containsAny(lower, "音乐", "music", "concert", "演唱会"):
		return "category.music"
	case containsAny(lower, "体育", "sport"):
		return "category.sports"
	default:
		return ""
	}
}

// FallbackChain 标准分类降级链
// 目标站不支持某分类时，按链降级
var FallbackChain = map[string][]string{
	"category.animation":   {"category.movie", "category.other"},
	"category.documentary": {"category.movie", "category.other"},
	"category.tv_shows":    {"category.tv_series", "category.other"},
	"category.music":       {"category.other"},
	"category.sports":      {"category.other"},
	"category.other":       {"category.movie"},
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
