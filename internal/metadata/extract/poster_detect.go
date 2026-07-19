// Package extract 海报识别（§56.12 决策 5）。
//
// 4 层优先级：站点特殊识别 > URL 关键词 > 首图启发式 > 无图。
// 站点特殊识别在 2b.7 实施（当前 stub）。
package extract

import "strings"

// posterURLKeywords 海报 URL 关键词（命中则优先当海报）。
var posterURLKeywords = []string{
	"doubanio", "tmdb", "poster", "imdb", "img9",
}

// ClassifyPosterAndScreenshots 海报识别 + 截图分类。
// 4 层优先级：站点特殊 > URL 关键词 > 首图启发式 > 无图。
// 参数 siteCode 用于 2b.7 站点特殊识别（当前 stub）。
func ClassifyPosterAndScreenshots(imageURLs []string, siteCode string) (poster string, screenshots []string) {
	if len(imageURLs) == 0 {
		return "", nil
	}

	// 1. 站点特殊识别（2b.7 TODO，当前 stub）
	// 示例：SSD l_ratio_poster / HDDolby l_ratio_poster

	// 2. URL 关键词识别
	for i, u := range imageURLs {
		lower := strings.ToLower(u)
		for _, kw := range posterURLKeywords {
			if strings.Contains(lower, kw) {
				poster = u
				screenshots = make([]string, 0, len(imageURLs)-1)
				screenshots = append(screenshots, imageURLs[:i]...)
				screenshots = append(screenshots, imageURLs[i+1:]...)
				return poster, screenshots
			}
		}
	}

	// 3. 首图启发式
	return imageURLs[0], imageURLs[1:]
}
