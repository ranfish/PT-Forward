package publish

import "testing"

// §59.399: IMDb/豆瓣编号提取三态锚（学校站 imdbid/doubanid 编号形态）。
func TestExtractIMDbDoubanID(t *testing.T) {
	// IMDb：标准 URL / 带查询参数 / 纯编号直出 / 空与非形态 miss
	cases := []struct{ in, want string }{
		{"https://www.imdb.com/title/tt4154756/", "tt4154756"},
		{"https://www.imdb.com/title/tt4154756/?ref_=fn_al_tt_1", "tt4154756"},
		{"tt4154756", "tt4154756"},
		{"https://www.imdb.com/find?q=x", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := extractIMDbID(c.in); got != c.want {
			t.Errorf("extractIMDbID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	// 豆瓣：标准 subject/ 路径 / 纯数字直出 / 非条目页 miss / 空 miss
	dcases := []struct{ in, want string }{
		{"https://movie.douban.com/subject/24773958/", "24773958"},
		{"https://douban.com/subject/24773958/?from=x", "24773958"},
		{"24773958", "24773958"},
		{"https://movie.douban.com/explore", ""},
		{"", ""},
	}
	for _, c := range dcases {
		if got := extractDoubanID(c.in); got != c.want {
			t.Errorf("extractDoubanID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
