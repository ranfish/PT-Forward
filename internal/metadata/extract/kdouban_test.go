package extract

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// §59.45: kdouban 框逆向解析——双种子实测 DOM 固定用例（tid=2782091 结构）。
const kdoubanSampleHTML = `<html><body>
<div id='kdouban'>
<div class="imdbwp imdbwp--movie blue">
<div class="imdbwp__thumb"><a class="imdbwp__link" href="https://movie.douban.com/subject/3567504/"><img class="imdbwp__img" src="https://pt.keepfrds.com/poster_douban/w185/2782091.webp"></a></div>
<div class="imdbwp__content">
<div class="imdbwp__header"><div class="imdbwp__title">幻想 ( 1976 )</div></div>
<div class="imdbwp__meta"> Fantasm | | 澳大利亚 </div>
<div class="imdbwp__belt"><div class="imdbwp__star">0</div></div>
<div class="imdbwp__rating"> Rating: 0 / 10 from 0 users </div>
<div class="imdbwp__teaser"> Professor Jurgen Notafreud explores the 10 most common female sexual fantasies. </div>
<div class="imdbwp__footer">
<div><strong> Director: </strong> <span>理查德·富兰克林 Richard Franklin</span></div>
<div><strong> Actors: </strong> <span>坎蒂·桑普尔斯 Candy Samples William Margold William Margold</span></div>
</div>
</div>
</div>
</div>
</body></html>`

func TestExtractKDouban(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(kdoubanSampleHTML))
	if err != nil {
		t.Fatal(err)
	}
	kd := ExtractKDouban(doc)
	if kd.Body == "" {
		t.Fatal("应提取出简介")
	}
	checks := []struct{ want string }{
		{"◎译　　名　幻想"},
		{"◎片　　名　Fantasm"},
		{"◎年　　代　1976"},
		{"◎产　　地　澳大利亚"},
		{"理查德·富兰克林 Richard Franklin"},
		{"Professor Jurgen Notafreud"},
	}
	for _, c := range checks {
		if !strings.Contains(kd.Body, c.want) {
			t.Errorf("简介缺 %q:\n%s", c.want, kd.Body[:200])
		}
	}
	if kd.Poster == "" || !strings.Contains(kd.Poster, "2782091.webp") {
		t.Errorf("海报提取失败: %q", kd.Poster)
	}
}

func TestExtractKDoubanNoCard(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader("<html><body>无框页面</body></html>"))
	if got := ExtractKDoubanIntro(doc); got != "" {
		t.Errorf("无 kdouban 框应返回空, got %q", got[:50])
	}
}

// §59.45: MI 污染判定
func TestIsMIPollutedIntro(t *testing.T) {
	cases := []struct {
		name, body string
		want       bool
	}{
		{"MI拼接_污染", "MediaInfo: F:\\PT\\x.mkv\n[quote]General\nUnique ID : 123\nFormat : Matroska", true},
		{"BDInfo_污染", "BDINFO:\nDISC INFO:\nStream size", true},
		{"◎正文_保留", "◎译　　名　英雄本色2\n◎片　　名　A Better Tomorrow II\n简介正文……", false},
		{"纯文本_保留", "这是一段正常的人工简介，讲述影片内容。", false},
		{"单词MediaInfo不污染", "影片信息参见 MediaInfo 栏。简介正文。", false},
		{"空", "", false},
	}
	for _, c := range cases {
		if got := IsMIPollutedIntro(c.body); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// §59.46: 空 body 也回退 kdouban（纯声明形态）——IsMIPollutedIntro("") 为 false，
// 由调用层 body=="" || IsMIPolluted 组合判定，此处锚定 IsMIPolluted 对空串语义
func TestIsMIPollutedEmptyNotPolluted(t *testing.T) {
	if IsMIPollutedIntro("") {
		t.Error("空 body 不应判污染（调用层用 body==\"\" 独立判定回退）")
	}
}
