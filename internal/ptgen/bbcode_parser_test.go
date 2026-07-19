package ptgen

import (
	"strings"
	"testing"
)

const sampleBBCode = `[img]https://img9.doubanio.com/photo/poster.jpg[/img]

◎译　名 测试电影译名 / 备用译名
◎片　名 Test Movie
◎年　代 2024
◎产　地 中国大陆
◎类　型 动作 / 喜剧
◎语　言 普通话 / 英语
◎上映日期 2024-01-01
◎IMDb评分 8.5/10 from 100,000 users
◎豆瓣评分 8.2 12345/54321
◎片　长 120分钟
◎导　演 张三
◎编　剧 李四
◎主　演 王五 / 赵六
◎简　介 这是一部测试电影的简介。
◎获奖情况 最佳影片

[url=https://movie.douban.com/subject/12345/]豆瓣[/url]
[url=https://www.imdb.com/title/tt1234567/]IMDb[/url]`

func TestParseBBCodeFormat_Complete(t *testing.T) {
	result := ParseBBCodeFormat(sampleBBCode)

	if result.PosterURL != "https://img9.doubanio.com/photo/poster.jpg" {
		t.Errorf("PosterURL mismatch: %q", result.PosterURL)
	}
	if result.ChineseTitle != "测试电影译名" {
		t.Errorf("ChineseTitle mismatch: %q", result.ChineseTitle)
	}
	if result.ForeignTitle != "Test Movie" {
		t.Errorf("ForeignTitle mismatch: %q", result.ForeignTitle)
	}
	if result.Year != "2024" {
		t.Errorf("Year mismatch: %q", result.Year)
	}
	if len(result.Region) != 1 || result.Region[0] != "中国大陆" {
		t.Errorf("Region mismatch: %v", result.Region)
	}
	if len(result.Genre) != 2 {
		t.Errorf("Genre mismatch: %v", result.Genre)
	}
	if result.Playdate != "2024-01-01" {
		t.Errorf("Playdate mismatch: %q", result.Playdate)
	}
	if result.IMDBRating != "8.5" {
		t.Errorf("IMDBRating mismatch: %q", result.IMDBRating)
	}
	if result.IMDBVotes != "100000" {
		t.Errorf("IMDBVotes mismatch: %q", result.IMDBVotes)
	}
	if result.DoubanRating != "8.2" {
		t.Errorf("DoubanRating mismatch: %q", result.DoubanRating)
	}
	if result.DoubanVotes != "12345" {
		t.Errorf("DoubanVotes mismatch: %q", result.DoubanVotes)
	}
	if result.Duration != "120分钟" {
		t.Errorf("Duration mismatch: %q", result.Duration)
	}
	if len(result.Director) != 1 || result.Director[0] != "张三" {
		t.Errorf("Director mismatch: %v", result.Director)
	}
	if len(result.Cast) != 2 {
		t.Errorf("Cast mismatch: %v", result.Cast)
	}
	if !strings.Contains(result.Introduction, "测试电影") {
		t.Errorf("Introduction mismatch: %q", result.Introduction)
	}
	if len(result.Awards) != 1 || result.Awards[0] != "最佳影片" {
		t.Errorf("Awards mismatch: %v", result.Awards)
	}
	if result.DoubanURL == "" || !strings.Contains(result.DoubanURL, "12345") {
		t.Errorf("DoubanURL mismatch: %q", result.DoubanURL)
	}
	if !strings.Contains(result.IMDBURL, "tt1234567") {
		t.Errorf("IMDBURL mismatch: %q", result.IMDBURL)
	}
	if result.IMDBID != "tt1234567" {
		t.Errorf("IMDBID mismatch: %q", result.IMDBID)
	}
}

func TestParseBBCodeFormat_Empty(t *testing.T) {
	result := ParseBBCodeFormat("")
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.PosterURL != "" {
		t.Errorf("empty BBCode should have empty PosterURL, got %q", result.PosterURL)
	}
	if result.RawBBCode != "" {
		t.Errorf("RawBBCode should be empty")
	}
}

func TestParseBBCodeFormat_NoFields(t *testing.T) {
	bbcode := "这是一段没有 ◎字段的普通文本"
	result := ParseBBCodeFormat(bbcode)
	if result.ChineseTitle != "" {
		t.Errorf("ChineseTitle should be empty")
	}
	if result.RawBBCode != bbcode {
		t.Errorf("RawBBCode should be preserved")
	}
}

func TestParseBBCodeFormat_FullWidthSpace(t *testing.T) {
	// ◎译　名（全角空格）
	bbcode := "◎译　名 全角空格测试"
	result := ParseBBCodeFormat(bbcode)
	if result.ChineseTitle != "全角空格测试" {
		t.Errorf("full-width space field not parsed, got %q", result.ChineseTitle)
	}
}

func TestParseBBCodeFormat_HalfWidthSpace(t *testing.T) {
	// ◎译名（无全角空格）
	bbcode := "◎译名 半角空格测试"
	result := ParseBBCodeFormat(bbcode)
	if result.ChineseTitle != "半角空格测试" {
		t.Errorf("half-width space field not parsed, got %q", result.ChineseTitle)
	}
}

func TestParseBBCodeFormat_ColonSeparator(t *testing.T) {
	// ◎译名: 值（冒号分隔）
	bbcode := "◎译名：冒号分隔测试"
	result := ParseBBCodeFormat(bbcode)
	if result.ChineseTitle != "冒号分隔测试" {
		t.Errorf("colon separator not parsed, got %q", result.ChineseTitle)
	}
}

func TestParseBBCodeFormat_RatingFormats(t *testing.T) {
	cases := []struct {
		in        string
		wantR, wantV string
	}{
		{"8.5/10 from 100,000 users", "8.5", "100000"},
		{"8.2 12345/54321", "8.2", "12345"},
		{"7.0", "7.0", ""},
	}
	for _, c := range cases {
		r, v := parseRating(c.in)
		if r != c.wantR || v != c.wantV {
			t.Errorf("parseRating(%q) = (%q, %q), want (%q, %q)", c.in, r, v, c.wantR, c.wantV)
		}
	}
}

func TestParseBBCodeFormat_PartialFields(t *testing.T) {
	bbcode := `[img]https://example.com/poster.jpg[/img]
◎片　名 Only Title
◎年　代 2024`
	result := ParseBBCodeFormat(bbcode)
	if result.PosterURL != "https://example.com/poster.jpg" {
		t.Errorf("PosterURL mismatch")
	}
	if result.ForeignTitle != "Only Title" {
		t.Errorf("ForeignTitle mismatch: %q", result.ForeignTitle)
	}
	if result.Year != "2024" {
		t.Errorf("Year mismatch: %q", result.Year)
	}
	if result.ChineseTitle != "" {
		t.Errorf("ChineseTitle should be empty")
	}
}

func TestParseBBCodeFormat_RegionAndGenre(t *testing.T) {
	bbcode := `◎产　地 中国大陆 / 中国香港
◎类　型 动作 / 喜剧 / 剧情`
	result := ParseBBCodeFormat(bbcode)
	if len(result.Region) != 2 {
		t.Errorf("Region should have 2 items, got %v", result.Region)
	}
	if len(result.Genre) != 3 {
		t.Errorf("Genre should have 3 items, got %v", result.Genre)
	}
}

func TestParseBBCodeFormat_AKA(t *testing.T) {
	bbcode := "◎译　名 主译名 / 别名一 / 别名二"
	result := ParseBBCodeFormat(bbcode)
	if result.ChineseTitle != "主译名" {
		t.Errorf("ChineseTitle mismatch: %q", result.ChineseTitle)
	}
	if len(result.AKA) != 3 {
		t.Errorf("AKA should have 3 items, got %v", result.AKA)
	}
}

func TestSplitBySlash(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"a / b / c", 3},
		{"a，b，c", 3},
		{"a / b，c", 3},
		{"single", 1},
		{"", 0},
		{"  ", 0},
	}
	for _, c := range cases {
		got := splitBySlash(c.in)
		if len(got) != c.want {
			t.Errorf("splitBySlash(%q) = %v, want %d items", c.in, got, c.want)
		}
	}
}

func TestOrderedFieldNames_DescByLength(t *testing.T) {
	names := orderedFieldNames()
	if len(names) < 2 {
		t.Fatal("expected at least 2 field names")
	}
	// "获奖情况" 应该在 "获奖" 前面（长度降序）
	idxLong, idxShort := -1, -1
	for i, n := range names {
		if n == "获奖情况" {
			idxLong = i
		}
		if n == "获奖" {
			idxShort = i
		}
	}
	if idxLong >= 0 && idxShort >= 0 && idxLong > idxShort {
		t.Errorf("'获奖情况' should come before '获奖' to avoid prefix mismatch")
	}
}
