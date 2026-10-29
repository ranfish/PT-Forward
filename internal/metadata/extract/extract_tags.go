package extract

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// extractTags 从详情页提取标签（NexusPHP <a class="torrent_tag">）。
// 排除侧栏标签云（sb-tag/chs_tag，PTer 等站会列出所有可选 tag 链接，
// 如 "禁转/官方/国语/DIY原盘/...", 这些不是种子的实际 tag）。
func (p *PublicExtractor) extractTags(doc *goquery.Document) []string {
	tags := []string{}
	seen := make(map[string]struct{})

	// 主路径：<a class="torrent_tag*">（NexusPHP 种子实际 tag 的标准 class）
	doc.Find(`a[class*="torrent_tag"]`).Each(func(_ int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text == "" {
			return
		}
		if _, ok := seen[text]; ok {
			return
		}
		seen[text] = struct{}{}
		tags = append(tags, text)
	})

	// §59.369 附二: NP 新版前端形态——结构化字段区"标签"的
	// <span class="torrent-content-tag">（深度站 dstudio.me 实证：
	// 官方/限转/原生等；详情页无侧栏标签云、筛选器在列表页不进本
	// 提取——无标签云误伤面）。
	doc.Find(`span[class*="torrent-content-tag"]`).Each(func(_ int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text == "" {
			return
		}
		if _, ok := seen[text]; ok {
			return
		}
		seen[text] = struct{}{}
		tags = append(tags, text)
	})

	return tags
}
