package extract

import "github.com/PuerkitoBio/goquery"

// §56.10 splitIntroSections：简介分段（核心算法）。
// 2b.4 阶段实现简化版（首图当海报 + 剩余 BBCode 当 Body）。
// 后续完善：quote 前后二分 + ARDTU 剥离 + Statement 识别。
func (p *PublicExtractor) splitIntroSections(descrHTML, descrBBCode string) IntroData {
	intro := IntroData{
		Body: descrBBCode,
	}
	// 提取所有图片
	urls := extractImageURLsFromHTML(descrHTML)
	if len(urls) > 0 {
		intro.Poster = urls[0]
		if len(urls) > 1 {
			intro.SetScreenshotURLs(urls[1:])
		}
	}
	return intro
}

// extractImageURLsFromHTML 从 HTML 中提取所有 <img src> 和 data-src。
func extractImageURLsFromHTML(htmlStr string) []string {
	doc, err := ParseHTML(htmlStr)
	if err != nil {
		return nil
	}
	var urls []string
	seen := make(map[string]struct{})
	doc.Find("img").Each(func(_ int, s *goquery.Selection) {
		src, ok := s.Attr("src")
		if !ok || src == "" {
			src, ok = s.Attr("data-src")
			if !ok || src == "" {
				return
			}
		}
		if _, exists := seen[src]; exists {
			return
		}
		seen[src] = struct{}{}
		urls = append(urls, src)
	})
	return urls
}
