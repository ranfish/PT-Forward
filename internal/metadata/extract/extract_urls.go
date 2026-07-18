package extract

import (
	"regexp"

	"github.com/PuerkitoBio/goquery"
)

var (
	imdbURLRe   = regexp.MustCompile(`https?://(?:www\.)?imdb\.com/title/(tt\d+)`)
	doubanURLRe = regexp.MustCompile(`https?://(?:www\.)?(?:movie\.)?douban\.com/(?:subject|movie)/(\d+)`)
	tmdbURLRe   = regexp.MustCompile(`https?://(?:www\.)?themoviedb\.org/(?:movie|tv)/(\d+)`)
)

// extractExternalLinks 从详情页提取 IMDb/豆瓣/TMDb 链接。
// 主路径：goquery 找 <a href> 中的 IMDb/Douban/TMDb。
// Fallback：正则扫 BBCode（用户可能贴纯文本 URL）。
func (p *PublicExtractor) extractExternalLinks(doc *goquery.Document, descrBBCode string) (imdb, douban, tmdb string) {
	// goquery 主路径
	doc.Find(`a[href*="imdb.com"]`).Each(func(_ int, s *goquery.Selection) {
		if imdb != "" {
			return
		}
		if href, ok := s.Attr("href"); ok {
			if m := imdbURLRe.FindStringSubmatch(href); len(m) > 0 {
				imdb = m[0]
			}
		}
	})
	doc.Find(`a[href*="douban.com"]`).Each(func(_ int, s *goquery.Selection) {
		if douban != "" {
			return
		}
		if href, ok := s.Attr("href"); ok {
			if m := doubanURLRe.FindStringSubmatch(href); len(m) > 0 {
				douban = m[0]
			}
		}
	})
	doc.Find(`a[href*="themoviedb.org"]`).Each(func(_ int, s *goquery.Selection) {
		if tmdb != "" {
			return
		}
		if href, ok := s.Attr("href"); ok {
			if m := tmdbURLRe.FindStringSubmatch(href); len(m) > 0 {
				tmdb = m[0]
			}
		}
	})

	// Fallback：BBCode 正则（用户可能贴纯文本 URL，未包 <a>）
	if imdb == "" {
		if m := imdbURLRe.FindString(descrBBCode); m != "" {
			imdb = m
		}
	}
	if douban == "" {
		if m := doubanURLRe.FindString(descrBBCode); m != "" {
			douban = m
		}
	}
	if tmdb == "" {
		if m := tmdbURLRe.FindString(descrBBCode); m != "" {
			tmdb = m
		}
	}
	return imdb, douban, tmdb
}
