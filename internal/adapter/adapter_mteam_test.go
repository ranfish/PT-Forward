package adapter

import "testing"


// §59.297: imdb/douban 字段双形态——纯 ID 拼接 vs 完整 URL 直用（防双拼）
func TestMTeamURLFieldDualForm(t *testing.T) {
	// 纯 ID 形态（标准）
	u := buildIMDbURL("tt37535298")
	if u != "https://www.imdb.com/title/tt37535298/" {
		t.Errorf("纯ID: %q", u)
	}
	// 完整 URL 形态（Under Current 案 douban 双拼实证）
	d := buildDoubanURL("https://movie.douban.com/subject/30174089/")
	if d != "https://movie.douban.com/subject/30174089/" {
		t.Errorf("URL直用: %q", d)
	}
}
