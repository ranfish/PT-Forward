package titleparser

import (
	"strings"
)

// InferCategory 从多源信号推断类型分类
// 优先级: 源站分类 > 标题季集 > PTGen genre > 默认电影
func InferCategory(c TitleComponents, sourceCategory string, ptgenGenre string, ptgenEpisodes string) string {
	// ① 源站分类优先（最可信）——但泛化 movie 让位 PTGen 精确类型
	// §59.319 附二十六: 光之子案——源站(CSWEB)将纪录片归 movie，PTGen
	// genre=纪录片更具体；泛化分类不应覆盖精确分类
	if cat := NormalizeSourceCategory(sourceCategory); cat != "" {
		if cat == "category.movie" {
			// 泛化 movie：PTGen 有更具体类型时让位
			if specific := ptgenGenreToSpecificCategory(ptgenGenre); specific != "" && specific != "category.movie" {
				return specific
			}
		}
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

// NormalizeSourceCategory 将源站分类归一化为标准键（§59.317 全键族重写）。
// 覆盖 dict category.* 全部 canonical 键 + 站方中文词；特定键先于泛化键
// （lossless_music 先于 music——防前缀误吞）。旧映射表遗留值兼容：
// "category.cartoon" 仍归 animation（存量行 P1 刷正前的显示一致性）。
// 导出：api 层 normalizeCategorySimple 委托此单点（消灭平行实现 §59.26 教训）。
func NormalizeSourceCategory(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	lower := strings.ToLower(raw)

	switch {
	// §59.317 新键族（特定→泛化排序）
	case containsAny(lower, "lossless", "无损"):
		return "category.lossless_music"
	case containsAny(lower, "audiobook", "有声书", "有聲書"):
		return "category.audiobook"
	case containsAny(lower, "ebook", "电子书", "電子書"):
		return "category.ebook"
	case containsAny(lower, "concert", "演唱"):
		return "category.concert"
	case containsAny(lower, "education", "教育", "study"):
		return "category.education"
	case containsAny(lower, "game", "游戏", "遊戲"):
		return "category.game"
	case containsAny(lower, "software", "软件", "軟體"):
		return "category.software"
	case containsAny(lower, "short_drama", "短剧", "短劇", "playlet"):
		return "category.short_drama"
	case containsAny(lower, "comic", "漫画", "漫畫"):
		return "category.comic"
	case containsAny(lower, "adult", "成人"):
		return "category.adult"
	// 媒体主类（原有键族保持）
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
	case containsAny(lower, "音乐", "music", "演唱会"):
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

// ptgenGenreToSpecificCategory §59.319 附二十六: PTGen genre → 具体
// category（仅当比泛化 movie 更具体时有值）。documentary/music/sports/
// tv_shows/animation 均比 movie 更具体。
func ptgenGenreToSpecificCategory(ptgenGenre string) string {
	genreLower := strings.ToLower(ptgenGenre)
	if genreLower == "" {
		return ""
	}
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
	return ""
}
