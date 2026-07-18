// Package extract PublicExtractor（§56.10 总编排）。
package extract

import (
	"fmt"

	"github.com/PuerkitoBio/goquery"
)

// PublicExtractor 公共提取器，所有 NexusPHP/Generic/TNode/Yemapt adapter 共用。
// 实现 Extractor 接口，可作为 Engine.public 注入。
type PublicExtractor struct {
	siteCode        string
	siteNickname    string
	bbcodeConverter *HTMLToBBCodeConverter
}

// NewPublicExtractor 创建公共提取器实例。
func NewPublicExtractor(siteCode, siteNickname string) *PublicExtractor {
	return &PublicExtractor{
		siteCode:        siteCode,
		siteNickname:    siteNickname,
		bbcodeConverter: NewHTMLToBBCodeConverter(siteCode, siteNickname),
	}
}

// Name 实现 Extractor 接口。
func (p *PublicExtractor) Name() string { return "public" }

// Extract 执行详情页全字段提取。
//
// 阶段（§56.10 PublicExtractor 总编排）：
//  1. 标题/副标题
//  2. 描述容器 HTML + BBCode 转换
//  3. 简介分段（Statement/Poster/Body/Screenshots）
//  4. MediaInfo/BDInfo（2b.5）
//  5. 基本信息表格行（type/medium/codec/...）
//  6. 标签 + InfoHash + Size + URL + Flags
func (p *PublicExtractor) Extract(input Input) (SeedData, error) {
	if input.PageHTML == "" {
		return SeedData{}, fmt.Errorf("extract: empty page html")
	}
	doc, err := ParseHTML(input.PageHTML)
	if err != nil {
		return SeedData{}, fmt.Errorf("parse html: %w", err)
	}

	seed := SeedData{}

	// 阶段 1: 标题/副标题
	seed.Title = p.extractTitle(doc, input.FallbackTitle)
	seed.Subtitle = p.extractSubtitle(doc, input.PageHTML)

	// 阶段 2: 描述容器 HTML + BBCode 转换
	descrHTML := p.extractDescriptionHTML(doc)
	descrBBCode, _ := p.bbcodeConverter.Convert(descrHTML)

	// 阶段 3: 简介分段
	seed.Intro = p.splitIntroSections(descrHTML, descrBBCode)

	// 阶段 4: MediaInfo/BDInfo（2b.5 stub）
	seed.MediaInfo, seed.BDInfo = p.extractMediaInfo(descrHTML, descrBBCode)

	// 阶段 5: 基本信息表格行
	p.fillBasicInfoFields(doc, &seed)

	// 阶段 6: 标签 + InfoHash + Size + URL + Flags
	seed.Tags = p.extractTags(doc)
	seed.InfoHash = p.extractInfoHash(input.PageHTML, doc)
	seed.Size = p.extractSize(doc, input.PageHTML)
	seed.IMDbLink, seed.DoubanLink, seed.TMDbLink = p.extractExternalLinks(doc, descrBBCode)
	seed.Flags = p.extractFlags(seed.Title, seed.Subtitle, descrBBCode, input.PageHTML)

	return seed.NormalizeWithFallback(input.FallbackTitle), nil
}

// extractDescriptionHTML 提取描述容器 HTML（NexusPHP #kdescr 优先）。
func (p *PublicExtractor) extractDescriptionHTML(doc *goquery.Document) string {
	// NexusPHP 标准 selector
	selectors := []string{
		"#kdescr",
		"#kdescription",
		`div[id*="kdescr"]`,
		`div[class*="torrent_description"]`,
		`div[class*="torrent-description"]`,
		"#message",
		".torrentinfo",
	}
	for _, sel := range selectors {
		s := doc.Find(sel).First()
		if s.Length() > 0 {
			htmlStr, err := s.Html()
			if err == nil && htmlStr != "" {
				return htmlStr
			}
		}
	}
	return ""
}
