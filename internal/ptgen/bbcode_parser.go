// Package ptgen ◎字段 BBCode 解析器（§56.16 决策 1 + §10.11）。
//
// PTGen（豆影/财神）返回的 BBCode 格式（◎风格）：
//   [img]海报URL[/img]
//
//   ◎译　名 xxx / yyy
//   ◎片　名 Original Title
//   ◎年　代 2024
//   ◎产　地 中国大陆
//   ◎类　型 剧情 / 动作
//   ◎上映日期 2024-01-01
//   ◎IMDb评分 8.5/10 from 100,000 users
//   ◎豆瓣评分 8.2 12345/54321
//   ◎片　长 120分钟
//   ◎导　演 张三
//   ◎主　演 李四 / 王五
//   ◎简　介 剧情简介...
//   ◎获奖情况 ...
//
// 解析失败时不阻塞主流程（决策 15）：返回部分填充结果，RawBBCode 完整。
package ptgen

import (
	"regexp"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
)

var (
	bbcodeImgRe  = regexp.MustCompile(`(?i)\[img\](.*?)\[/img\]`)
	doubanURLRe  = regexp.MustCompile(`https?://(?:www\.)?(?:movie\.)?douban\.com/(?:subject|movie)/\d+`)
	imdbURLRe    = regexp.MustCompile(`https?://(?:www\.)?imdb\.com/title/(tt\d+)`)
	tmdbURLRe    = regexp.MustCompile(`https?://(?:www\.)?themoviedb\.org/(?:movie|tv)/\d+`)
	ratingSlashRe = regexp.MustCompile(`([\d.]+)\s*/\s*10\s*(?:from\s*)?([\d,]+)\s*(?:users)?`)
	ratingSpaceRe = regexp.MustCompile(`([\d.]+)\s+(\d+)/(\d+)`)
)

// fieldMapper 字段映射函数类型。
type fieldMapper func(result *model.PTGenResult, value string)

// fieldMappers ◎字段名（去全角空格后）→ 映射函数。
var fieldMappers = map[string]fieldMapper{
	"译名":    mapChineseTitle,
	"又名":    mapAKA,
	"片名":    mapForeignTitle,
	"年代":    mapYear,
	"产地":    mapRegion,
	"地区":    mapRegion,
	"类型":    mapGenre,
	"类别":    mapGenre,
	"语言":    mapLanguage,
	"上映日期": mapPlaydate,
	"首映":    mapPlaydate,
	"IMDb评分": mapIMDbRating,
	"IMDB评分": mapIMDbRating,
	"豆瓣评分": mapDoubanRating,
	"片长":    mapDuration,
	"导演":    mapDirector,
	"编剧":    mapWriter,
	"主演":    mapCast,
	"简介":    mapIntroduction,
	"获奖":    mapAwards,
	"获奖情况": mapAwards,
}

// ParseBBCodeFormat 解析 PTGen 返回的 ◎字段 BBCode 为结构化 PTGenResult。
// 解析失败时不阻塞，返回部分填充的结果（RawBBCode 完整）。
func ParseBBCodeFormat(bbcode string) *model.PTGenResult {
	result := &model.PTGenResult{
		RawBBCode:   bbcode,
		Region:      []string{},
		Genre:       []string{},
		Language:    []string{},
		Director:    []string{},
		Cast:        []model.PersonInfo{},
		Writer:      []string{},
		Awards:      []string{},
		AKA:         []string{},
	}

	if strings.TrimSpace(bbcode) == "" {
		return result
	}

	// 1. 提取海报（第一个 [img]...[/img]）
	if m := bbcodeImgRe.FindStringSubmatch(bbcode); len(m) > 1 {
		result.PosterURL = strings.TrimSpace(m[1])
	}

	// 2. 按 ◎字段逐行解析
	for _, line := range strings.Split(bbcode, "\n") {
		parseBBCodeFieldLine(result, line)
	}

	// 3. 正则提取外部链接
	if m := doubanURLRe.FindString(bbcode); m != "" {
		result.DoubanURL = m
	}
	if m := imdbURLRe.FindString(bbcode); m != "" {
		result.IMDBURL = m
		if sub := imdbURLRe.FindStringSubmatch(bbcode); len(sub) > 1 {
			result.IMDBID = sub[1]
		}
	}
	if m := tmdbURLRe.FindString(bbcode); m != "" {
		result.TMDbURL = m
	}

	return result
}

// parseBBCodeFieldLine 解析单个 ◎字段行。
// 容错（决策 14）：去掉全角空格后匹配字段名，支持空格/冒号分隔。
func parseBBCodeFieldLine(result *model.PTGenResult, line string) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "◎") {
		return
	}
	line = strings.TrimPrefix(line, "◎")
	line = strings.TrimSpace(line)

	// 去掉全角空格后匹配（"译　名" → "译名"）
	normalized := strings.ReplaceAll(line, "　", "")

	// 按字段名长度降序匹配（避免"获奖"先于"获奖情况"匹配）
	for _, fieldName := range orderedFieldNames() {
		if !strings.HasPrefix(normalized, fieldName) {
			continue
		}
		rest := normalized[len(fieldName):]
		// 跳过分隔符（空格/冒号/中文冒号）
		value := strings.TrimLeft(rest, " \t:：")
		if value == "" {
			return
		}
		fieldMappers[fieldName](result, value)
		return
	}
}

// orderedFieldNames 按长度降序的字段名列表（避免短前缀先匹配）。
var orderedFields = func() []string {
	names := make([]string, 0, len(fieldMappers))
	for name := range fieldMappers {
		names = append(names, name)
	}
	// 简单冒泡排序（字段名数量少）
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if len(names[j]) > len(names[i]) {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	return names
}()

func orderedFieldNames() []string { return orderedFields }

// ===== 字段映射函数 =====

func mapChineseTitle(r *model.PTGenResult, v string) {
	parts := splitBySlash(v)
	if len(parts) > 0 {
		r.ChineseTitle = parts[0]
	}
	r.AKA = append(r.AKA, parts...)
}

func mapAKA(r *model.PTGenResult, v string) {
	r.AKA = append(r.AKA, splitBySlash(v)...)
}

func mapForeignTitle(r *model.PTGenResult, v string) {
	r.ForeignTitle = v
}

func mapYear(r *model.PTGenResult, v string) {
	r.Year = v
}

func mapRegion(r *model.PTGenResult, v string) {
	r.Region = splitBySlash(v)
}

func mapGenre(r *model.PTGenResult, v string) {
	r.Genre = splitBySlash(v)
}

func mapLanguage(r *model.PTGenResult, v string) {
	r.Language = splitBySlash(v)
}

func mapPlaydate(r *model.PTGenResult, v string) {
	r.Playdate = v
}

func mapIMDbRating(r *model.PTGenResult, v string) {
	rating, votes := parseRating(v)
	r.IMDBRating = rating
	r.IMDBVotes = votes
}

func mapDoubanRating(r *model.PTGenResult, v string) {
	rating, votes := parseRating(v)
	r.DoubanRating = rating
	r.DoubanVotes = votes
}

func mapDuration(r *model.PTGenResult, v string) {
	r.Duration = v
}

func mapDirector(r *model.PTGenResult, v string) {
	r.Director = splitBySlash(v)
}

func mapWriter(r *model.PTGenResult, v string) {
	r.Writer = splitBySlash(v)
}

func mapCast(r *model.PTGenResult, v string) {
	for _, name := range splitBySlash(v) {
		r.Cast = append(r.Cast, model.PersonInfo{Name: name})
	}
}

func mapIntroduction(r *model.PTGenResult, v string) {
	r.Introduction = v
}

func mapAwards(r *model.PTGenResult, v string) {
	r.Awards = splitBySlash(v)
}

// ===== 工具函数 =====

// splitBySlash 按 / 和 ， 分割字段值。
func splitBySlash(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var result []string
	for _, part := range strings.Split(s, "/") {
		for _, sub := range strings.Split(part, "，") {
			sub = strings.TrimSpace(sub)
			if sub != "" {
				result = append(result, sub)
			}
		}
	}
	return result
}

// parseRating 解析评分。支持两种格式：
//   "8.5/10 from 100,000 users" → rating="8.5", votes="100,000"
//   "8.2 12345/54321"            → rating="8.2", votes="12345"（豆影特殊格式）
func parseRating(s string) (rating, votes string) {
	s = strings.TrimSpace(s)
	if m := ratingSlashRe.FindStringSubmatch(s); len(m) > 2 {
		return m[1], strings.ReplaceAll(m[2], ",", "")
	}
	if m := ratingSpaceRe.FindStringSubmatch(s); len(m) > 2 {
		return m[1], m[2]
	}
	return s, ""
}
