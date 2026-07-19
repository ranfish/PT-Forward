// Package description PTGen 结构化渲染（§56.16 决策 6/7）。
//
// 把 PTGenResult 渲染为 BBCode，支持多套模板：
//   - douban（默认）：◎风格，中文标签（兼容现有 PTGenBody 格式）
//   - imdb：英文风格，英文标签
package description

import (
	"fmt"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
)

// PTGenTemplate PTGen 渲染模板。
type PTGenTemplate string

const (
	PTGenTemplateDouban PTGenTemplate = "douban" // ◎风格（默认）
	PTGenTemplateIMDb   PTGenTemplate = "imdb"   // 英文风格
	PTGenTemplateCustom PTGenTemplate = "custom" // 自定义（未来扩展）
)

// FormatPTGen 把结构化 PTGenResult 渲染为 BBCode。
// result 为 nil 或全空时返回空字符串。
func FormatPTGen(result *model.PTGenResult, template PTGenTemplate) string {
	if result == nil || isEmptyPTGen(result) {
		return ""
	}
	switch template {
	case PTGenTemplateIMDb:
		return formatIMDbStyle(result)
	default:
		return formatDoubanStyle(result)
	}
}

// formatDoubanStyle ◎风格渲染（默认，兼容现有 PTGenBody 格式）。
func formatDoubanStyle(r *model.PTGenResult) string {
	var b strings.Builder
	if r.PosterURL != "" {
		b.WriteString("[img]")
		b.WriteString(r.PosterURL)
		b.WriteString("[/img]\n\n")
	}
	writeField(&b, "◎译　名", joinTitles(r.ChineseTitle, r.AKA))
	writeField(&b, "◎片　名", r.ForeignTitle)
	writeField(&b, "◎年　代", r.Year)
	writeField(&b, "◎产　地", strings.Join(r.Region, " / "))
	writeField(&b, "◎类　型", strings.Join(r.Genre, " / "))
	writeField(&b, "◎语　言", strings.Join(r.Language, " / "))
	writeField(&b, "◎上映日期", r.Playdate)
	writeField(&b, "◎IMDb评分", formatRating(r.IMDBRating, r.IMDBVotes))
	writeField(&b, "◎豆瓣评分", formatRating(r.DoubanRating, r.DoubanVotes))
	writeField(&b, "◎片　长", r.Duration)
	writeField(&b, "◎导　演", strings.Join(r.Director, " / "))
	writeField(&b, "◎编　剧", strings.Join(r.Writer, " / "))
	writeField(&b, "◎主　演", formatPeople(r.Cast, 10))
	writeField(&b, "◎简　介", r.Introduction)
	if len(r.Awards) > 0 {
		writeField(&b, "◎获奖情况", strings.Join(r.Awards, "\n"))
	}
	writeURL(&b, r.DoubanURL, "豆瓣链接")
	writeURL(&b, r.IMDBURL, "IMDb链接")
	return strings.TrimRight(b.String(), "\n")
}

// formatIMDbStyle 英文风格渲染。
func formatIMDbStyle(r *model.PTGenResult) string {
	var b strings.Builder
	if r.PosterURL != "" {
		b.WriteString("[img]")
		b.WriteString(r.PosterURL)
		b.WriteString("[/img]\n\n")
	}
	writeField(&b, "Title:", r.ForeignTitle)
	writeField(&b, "Chinese Title:", r.ChineseTitle)
	writeField(&b, "Year:", r.Year)
	writeField(&b, "Country:", strings.Join(r.Region, " / "))
	writeField(&b, "Genre:", strings.Join(r.Genre, " / "))
	writeField(&b, "Language:", strings.Join(r.Language, " / "))
	writeField(&b, "Released:", r.Playdate)
	writeField(&b, "Runtime:", r.Duration)
	writeField(&b, "IMDb Rating:", formatRating(r.IMDBRating, r.IMDBVotes))
	writeField(&b, "Douban Rating:", formatRating(r.DoubanRating, r.DoubanVotes))
	writeField(&b, "Director:", strings.Join(r.Director, " / "))
	writeField(&b, "Writer:", strings.Join(r.Writer, " / "))
	writeField(&b, "Cast:", formatPeople(r.Cast, 10))
	writeField(&b, "Plot:", r.Introduction)
	if len(r.Awards) > 0 {
		writeField(&b, "Awards:", strings.Join(r.Awards, "\n"))
	}
	writeURL(&b, r.DoubanURL, "Douban")
	writeURL(&b, r.IMDBURL, "IMDb")
	return strings.TrimRight(b.String(), "\n")
}

// writeField 写入一个字段（跳过空值）。
func writeField(b *strings.Builder, label, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	b.WriteString(label)
	b.WriteString(" ")
	b.WriteString(value)
	b.WriteString("\n")
}

// writeURL 写入 URL 标签（跳过空值）。
func writeURL(b *strings.Builder, url, text string) {
	url = strings.TrimSpace(url)
	if url == "" {
		return
	}
	b.WriteString("[url=")
	b.WriteString(url)
	b.WriteString("]")
	b.WriteString(text)
	b.WriteString("[/url]\n")
}

// formatPeople 格式化演员列表（限制数量）。
func formatPeople(people []model.PersonInfo, limit int) string {
	if len(people) == 0 {
		return ""
	}
	if limit > 0 && len(people) > limit {
		people = people[:limit]
	}
	names := make([]string, 0, len(people))
	for _, p := range people {
		if p.Name != "" {
			names = append(names, p.Name)
		}
	}
	return strings.Join(names, " / ")
}

// formatRating 格式化评分（rating + votes）。
func formatRating(rating, votes string) string {
	rating = strings.TrimSpace(rating)
	votes = strings.TrimSpace(votes)
	if rating == "" {
		return ""
	}
	if votes == "" {
		return rating
	}
	return fmt.Sprintf("%s/10 from %s users", rating, votes)
}

// joinTitles 合并主标题 + 别名。
func joinTitles(main string, aka []string) string {
	parts := make([]string, 0, 1+len(aka))
	if main != "" {
		parts = append(parts, main)
	}
	parts = append(parts, aka...)
	return strings.Join(parts, " / ")
}

// isEmptyPTGen 判断 PTGenResult 是否全空。
func isEmptyPTGen(r *model.PTGenResult) bool {
	return r.ChineseTitle == "" && r.ForeignTitle == "" && r.Year == "" &&
		len(r.Region) == 0 && len(r.Genre) == 0 && r.Introduction == "" &&
		r.PosterURL == "" && r.IMDBURL == "" && r.DoubanURL == ""
}

// FormatField 公开 helper（决策 7）：格式化单个字段，空值返回空字符串。
func FormatField(label string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return label + " " + value + "\n"
}

// FormatPeople 公开 helper（决策 7）：格式化演员列表。
func FormatPeople(people []model.PersonInfo, limit int) string {
	return formatPeople(people, limit)
}

// FormatRating 公开 helper（决策 7）：格式化评分。
func FormatRating(rating, votes string) string {
	return formatRating(rating, votes)
}
