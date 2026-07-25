package adapter

import (
	"html"
	"regexp"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
)

// OpenCD（皇后）详情页专用提取（§56.34 音乐转发"原样搬运"）。
//
// OpenCD 版面和标准 NexusPHP 差异：
//   - 标题：<div class="title">（非 <h1 id="top">）
//   - 副标题：<div class="smalltitle">
//   - 简介：<div id="divdescr">（非 #kdescr）
//   - 曲目：<div id="divtracklist">（OpenCD 专有）
//   - EAC/LOG：<div id="divnfo"><pre>（OpenCD 专有）
//   - 封面：<div class="cover"><img onclick="Previewurl('...')">
//   - 字段行：<td class="rowtitle">（非 rowhead）

var (
	reOpenCDTitle = regexp.MustCompile(`(?s)<div class="title">(.*?)</div>`)
	reOpenCDSmall = regexp.MustCompile(`(?s)<div class="smalltitle">(.*?)</div>`)
	reOpenCDDescr = regexp.MustCompile(`(?s)<div id="divdescr">(.*?)</div>`)
	reOpenCDTrack = regexp.MustCompile(`(?s)<div id="divtracklist"[^>]*>(.*?)</div>`)
	reOpenCDNFO   = regexp.MustCompile(`(?s)<div id="divnfo"[^>]*>(.*?)</div>`)
	// 封面：优先 onclick="Previewurl('path')"，fallback 到 src
	reOpenCDCover = regexp.MustCompile(`(?s)<div class="cover"[^>]*>.*?onclick="Previewurl\('([^']+)'\)"`)
	reOpenCDCoverSrc = regexp.MustCompile(`(?s)<div class="cover"[^>]*>.*?<img[^>]*src="([^"]+)"`)
	// 字段行：<td class="rowtitle">字段名：</td><td ...>值</td>
	reOpenCDField = regexp.MustCompile(`(?s)<td class="rowtitle"[^>]*>([^<：]+)：?</td>\s*<td[^>]*>(.*?)</td>`)
)

// extractOpenCDDetail 从 OpenCD 详情页 HTML 提取字段（"原样搬运"模式）。
func extractOpenCDDetail(rawHTML string) *model.TorrentDetail {
	detail := &model.TorrentDetail{RawHTML: rawHTML}

	// 标题
	if m := reOpenCDTitle.FindStringSubmatch(rawHTML); len(m) > 1 {
		title := cleanOpenCDText(m[1])
		if title != "" {
			detail.Title = title
		}
	}

	// 副标题
	if m := reOpenCDSmall.FindStringSubmatch(rawHTML); len(m) > 1 {
		detail.Subtitle = cleanOpenCDText(m[1])
	}

	// 封面
	coverURL := ""
	if m := reOpenCDCover.FindStringSubmatch(rawHTML); len(m) > 1 {
		coverURL = m[1]
	} else if m := reOpenCDCoverSrc.FindStringSubmatch(rawHTML); len(m) > 1 {
		coverURL = m[1]
	}

	// 简介（拼接：封面 + 专辑介绍 + 曲目列表 + EAC/LOG）
	var descParts []string

	if coverURL != "" {
		descParts = append(descParts, "[img]"+coverURL+"[/img]")
	}

	if m := reOpenCDDescr.FindStringSubmatch(rawHTML); len(m) > 1 {
		intro := cleanOpenCDText(m[1])
		if intro != "" {
			descParts = append(descParts, intro)
		}
	}

	if m := reOpenCDTrack.FindStringSubmatch(rawHTML); len(m) > 1 {
		tracklist := cleanOpenCDText(m[1])
		if tracklist != "" {
			descParts = append(descParts, tracklist)
		}
	}

	if m := reOpenCDNFO.FindStringSubmatch(rawHTML); len(m) > 1 {
		nfo := cleanOpenCDText(m[1])
		if nfo != "" {
			descParts = append(descParts, "[hide=Log]"+nfo+"[/hide]")
		}
	}

	if len(descParts) > 0 {
		detail.Description = strings.Join(descParts, "\n\n")
	}

	// 字段行提取（专辑名称/艺术家/媒介/类型/大小/发行年份等）
	fields := extractOpenCDFields(rawHTML)
	if album, ok := fields["专辑名称"]; ok && detail.Subtitle == "" {
		detail.Subtitle = album
	}
	if mediaType, ok := fields["类型"]; ok {
		detail.Category = mediaType
	}
	if medium, ok := fields["媒介"]; ok {
		detail.Source = medium
	}

	// 截图/海报：用封面
	if coverURL != "" {
		detail.Screenshots = []string{coverURL}
	}

	return detail
}

// extractOpenCDFields 从 rowtitle 字段行提取键值对。
func extractOpenCDFields(html string) map[string]string {
	fields := make(map[string]string)
	matches := reOpenCDField.FindAllStringSubmatch(html, -1)
	for _, m := range matches {
		key := strings.TrimSpace(m[1])
		val := cleanOpenCDText(m[2])
		if key != "" && val != "" {
			fields[key] = val
		}
	}
	return fields
}

// cleanOpenCDText 清理 OpenCD HTML 片段为纯文本。
func cleanOpenCDText(s string) string {
	// 移除 <img> 标签（如优惠图标）
	s = regexp.MustCompile(`<img[^>]*>`).ReplaceAllString(s, "")
	// <br /> / <br> → 换行
	s = regexp.MustCompile(`(?i)<br\s*/?>`).ReplaceAllString(s, "\n")
	// 移除其他 HTML 标签
	s = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, "")
	// HTML 实体反转义
	s = html.UnescapeString(s)
	// 清理多余空白
	s = strings.TrimSpace(s)
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")
	return s
}

// isOpenCD 判断是否为 OpenCD 站点。
func isOpenCDDomain(domain string) bool {
	return strings.Contains(domain, "open.cd")
}
