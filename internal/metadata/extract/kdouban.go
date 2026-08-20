// Package extract 朋友站 kdouban 框逆向解析（§59.45）。
//
// 朋友站（keepfrds）详情页结构特例：站方插件自动渲染"豆瓣信息"折叠框
// （klappe_ext('douban')/id=kdouban/imdbwp 卡片），发布者约定 descr 只放
// 声明+MI+截图——intro.body 从 descr 取会 93% MI 污染。本文件把 kdouban
// 卡片逆向重建为 PTGen ◎ 格式 BBCode。
//
// 实测 DOM（tid=2782091，非 imdbwp 标准命名）：
//
//	<div id='kdouban'><div class="imdbwp imdbwp--movie ...">
//	  <div class="imdbwp__thumb">   海报 img
//	  <div class="imdbwp__content">
//	    <div class="imdbwp__header"><div class="imdbwp__title"> 片名 ( 年份 )
//	    <div class="imdbwp__meta">   原名 | | 产地
//	    <div class="imdbwp__belt"><div class="imdbwp__star">评分
//	    <div class="imdbwp__rating"> Rating: N / 10 from X users
//	    <div class="imdbwp__teaser"> 剧情简介（英文）
//	    <div class="imdbwp__footer"> Director: ... Actors: ... Aka: ... IMDb链接
package extract

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ExtractKDoubanIntro 从朋友站详情页提取 kdouban 框并重建 ◎ 格式简介。
// 无 kdouban 框返回空串（调用方回退 descr）。
func ExtractKDoubanIntro(doc *goquery.Document) string {
	if doc == nil {
		return ""
	}
	card := doc.Find("#kdouban")
	if card.Length() == 0 {
		return ""
	}

	get := func(sel string) string {
		return strings.TrimSpace(card.Find(sel).First().Text())
	}

	// 片名 ( 年份 )：拆分
	titleRaw := get(".imdbwp__title")
	title, year := splitKDoubanTitle(titleRaw)

	// meta：原名 | | 产地（三段 | 分隔）
	origName, region := "", ""
	if meta := get(".imdbwp__meta"); meta != "" {
		parts := strings.Split(meta, "|")
		if len(parts) >= 1 {
			origName = strings.TrimSpace(parts[0])
		}
		if len(parts) >= 3 {
			region = strings.TrimSpace(parts[len(parts)-1])
		}
	}

	// rating：Rating: 0 / 10 from 0 users
	ratingLine := get(".imdbwp__rating")

	// teaser：剧情简介
	teaser := get(".imdbwp__teaser")

	// footer：Director: X Actors: Y Aka: Z（按标签切）
	director, actors := parseKDoubanFooter(card)

	var b strings.Builder
	b.WriteString("◎译　　名　" + firstNonEmpty(title, origName) + "\n")
	b.WriteString("◎片　　名　" + firstNonEmpty(origName, title) + "\n")
	if year != "" {
		b.WriteString("◎年　　代　" + year + "\n")
	}
	if region != "" {
		b.WriteString("◎产　　地　" + region + "\n")
	}
	if director != "" {
		b.WriteString("◎导　　演　" + director + "\n")
	}
	if ratingLine != "" {
		b.WriteString("◎豆瓣评分　" + strings.TrimSpace(strings.TrimPrefix(ratingLine, "Rating:")) + "\n")
	}
	if teaser != "" {
		b.WriteString("\n" + teaser + "\n")
	}
	if actors != "" {
		b.WriteString("\n◎主　　演　" + actors + "\n")
	}
	out := b.String()
	if strings.Count(out, "◎") < 3 {
		return "" // 部件过少视为无效卡片（防残缺框产出垃圾简介）
	}
	return out
}

// kdoubanTitleRe 匹配 "片名 ( 年份 )" 形态。
var kdoubanTitleRe = regexp.MustCompile(`^(.*?)\s*\(\s*(\d{4})\s*\)\s*$`)

func splitKDoubanTitle(raw string) (title, year string) {
	if m := kdoubanTitleRe.FindStringSubmatch(raw); m != nil {
		return strings.TrimSpace(m[1]), m[2]
	}
	return strings.TrimSpace(raw), ""
}

// parseKDoubanFooter 提取 Director / Actors（footer 内 <strong> 标签分段）。
func parseKDoubanFooter(card *goquery.Selection) (director, actors string) {
	card.Find(".imdbwp__footer strong").Each(func(_ int, s *goquery.Selection) {
		label := strings.TrimSpace(s.Text())
		val := strings.TrimSpace(s.Next().Text())
		switch {
		case strings.HasPrefix(label, "Director"):
			director = val
		case strings.HasPrefix(label, "Actors"):
			actors = val
		}
	})
	return
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// IsMIPollutedIntro 判定简介正文是否为 MediaInfo/BDInfo 拼接形态（§59.45）。
// 朋友站 descr 无简介正文的形态特征：MI/BDInfo 标记词占主导。
func IsMIPollutedIntro(body string) bool {
	if body == "" {
		return false
	}
	markers := []string{"Unique ID", "Complete name", "MediaInfo", "Format : ", "Stream size", "Writing library", "BDINFO:", "DISC INFO"}
	hits := 0
	for _, m := range markers {
		if strings.Contains(body, m) {
			hits++
		}
	}
	// ≥2 个 MI 标记词 → 拼接形态；单 "MediaInfo"（如发布者提一句）不算
	return hits >= 2 || (hits >= 1 && !strings.Contains(body, "◎") && len(body) > 2000)
}

// KDoubanResult 提取结果（body + 海报，海报供 fallback）。
type KDoubanResult struct {
	Body   string
	Poster string
}

// ExtractKDouban 完整提取（含海报）。
func ExtractKDouban(doc *goquery.Document) KDoubanResult {
	body := ExtractKDoubanIntro(doc)
	if body == "" {
		return KDoubanResult{}
	}
	poster, _ := doc.Find("#kdouban .imdbwp__img").First().Attr("src")
	return KDoubanResult{Body: body, Poster: poster}
}

