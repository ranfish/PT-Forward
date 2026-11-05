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
	// §59.466: <a> 内文本反锚提取（tooltip data-options 属性含 jQuery('<div></div>')
	// 的 > 会把 <[^>]+> 提前断开致 JS 噪声残留——值内含 <a> 时取 >text</a> 纯文本段
	reOpenCDAInner = regexp.MustCompile(`>([^<>]+)</a>`)
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

	// §59.466: 音乐结构化元数据（MusicMeta——发布消费独立通道）
	mm := &model.MusicMeta{
		Album:     fields["专辑名称"],
		Artist:    fields["艺术家名"],
		Genre:     fields["类别"],
		Media:     fields["媒介"],
		ShareRule: fields["转载规则"],
		Origin:    fields["来自"],
	}
	// 标签列表（"欧美 古典(Classical)" 双维 Split）
	if tags, ok := fields["标签列表"]; ok {
		// 值内多 <a> join 为 " / "（cleanOpenCDFieldValue）——Split 兼容空白与 /
		for _, t := range strings.FieldsFunc(tags, func(r rune) bool { return r == ' ' || r == '/' || r == '\t' }) {
			if t = strings.TrimSpace(t); t != "" && t != "/" {
				mm.Tags = append(mm.Tags, t)
			}
		}
	}
	// 格式行拆解（"FLAC 分轨"→Format+AudioMode；纯格式值单填）
	if f, ok := fields["格式"]; ok {
		for _, seg := range []string{"分轨", "分軌", "整轨", "整軌"} {
			if strings.Contains(f, seg) {
				mm.AudioMode = strings.TrimSuffix(strings.TrimSuffix(seg, "轨"), "軌") + "轨"
				f = strings.TrimSpace(strings.TrimSuffix(f, seg))
				break
			}
		}
		mm.Format = f
	}
	// 发行年份（"2026年"→"2026"）
	if y, ok := fields["发行年份"]; ok {
		mm.Year = strings.TrimSuffix(strings.TrimSpace(y), "年")
	}
	// 其它版本（"0 个其它版本"→保留原文语义）
	if ov, ok := fields["其它版本"]; ok {
		mm.OtherVersions = ov
	}
	// 曲目/log 独立通道（原样搬运拼接保留）
	if m := reOpenCDTrack.FindStringSubmatch(rawHTML); len(m) > 1 {
		if tl := cleanOpenCDText(m[1]); tl != "" {
			mm.TrackList = tl
		}
	}
	if m := reOpenCDNFO.FindStringSubmatch(rawHTML); len(m) > 1 {
		if nfo := cleanOpenCDText(m[1]); nfo != "" {
			mm.LogText = nfo
		}
	}
	if mm.Artist != "" || mm.Album != "" || mm.Format != "" {
		detail.Music = mm
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
		val := cleanOpenCDFieldValue(m[2])
		if key != "" && val != "" {
			fields[key] = val
		}
	}
	return fields
}

// cleanOpenCDFieldValue 字段值清理（§59.466）——值含 <a> 时优先取内文本
// （tooltip JS 噪声防御），否则照旧。
func cleanOpenCDFieldValue(s string) string {
	if strings.Contains(s, "<a") {
		var parts []string
		for _, m := range reOpenCDAInner.FindAllStringSubmatch(s, -1) {
			if t := strings.TrimSpace(html.UnescapeString(m[1])); t != "" {
				parts = append(parts, t)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, " / ")
		}
	}
	return cleanOpenCDText(s)
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
