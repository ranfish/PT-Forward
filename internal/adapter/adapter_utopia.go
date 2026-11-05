package adapter

import (
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
)

// 乌托邦（musopia.vip）音乐特化（§59.469 附五）。
//
// 策略=enrich 模式（非皇后完整独立提取）：乌托邦基础版面=标准 NP
// （h1#top/kdescr/rowhead 字段行——Engine 提取已工作），仅音乐字段
// 后处理挂载（fillMediaFromRawBlock 同款先例）。
//
// 标题双形态（split " - " 容错）：
//   - 官组种三段：艺人 - 专辑 年份 格式 模式 - ZHENGLIFAN@Musopia
//   - 转载种四段：艺人 - 专辑 年份 - 格式 模式 - 来源站🌱保种区尾巴
//
// 字段行 DOM 细节：rowhead nowrap 双引号形态+来源站点行单引号形态；
// 基本信息值=嵌套 b 键值串。

var (
	// 字段行：双/单引号 class 兼容（来源站点行单引号形态实证 tid=10072）
	reUtopiaField = regexp.MustCompile(`(?s)<td class=["']rowhead[^"']*["'][^>]*>([^<：]+?)：?</td>\s*<td class=["']rowfollow["'][^>]*>(.*?)</td>`)
	// 标签行值=span 串（每 span 一标签·多维：首发/官方/地区/流派中英混合）
	reUtopiaTagSpan = regexp.MustCompile(`<span[^>]*>([^<]+)</span>`)
	// 基本信息嵌套 b 键值串（`<b>类型:</b>&nbsp;HQ Audio`——内层 b 匹配；
	// 值前 &nbsp;/空白 跳过后捕获）
	reUtopiaBasicKV = regexp.MustCompile(`<b>\s*(大小|类型|媒介|音频编码|制作组)\s*[:：]\s*</b>(?:&nbsp;|\s)*([^&<]+)`)
	// 转载自：URL（href 内 &amp; 需 unescape）
	reUtopiaOriginURL = regexp.MustCompile(`转载自：?\s*</span>\s*<a href=['"]([^'"]+)['"]`)
	// 抓轨 Log 区（codemain pre——多碟 FindAll join）
	reUtopiaRipLog = regexp.MustCompile(`(?s)抓轨 Log[^<]*</td>\s*<td[^>]*>.*?<pre><code>(.*?)</code></pre>`)
	// h1 尾部状态标记（[免费]/🌱保种区）
	reUtopiaTailFree = regexp.MustCompile(`\s*\[免费\]\s*$`)
	reUtopiaTailSeed = regexp.MustCompile(`🌱.*$`)
	// 标题年份（中段末位剥取）
	reUtopiaYear = regexp.MustCompile(`\b(19|20)\d{2}\b`)
	// 格式词表（标题中段剥取——从右往左）
	utopiaFormatWords = []string{"FLAC", "WavPack", "WAV", "APE", "DFF", "DSF", "MP3", "TTA", "AAC", "OGG", "ALAC"}
)

// isUtopiaDomain 乌托邦域名判定（挂载分流口——与 isOpenCDDomain 同款）。
func isUtopiaDomain(domain string) bool {
	return strings.Contains(strings.ToLower(domain), "musopia")
}

// enrichUtopiaMusic 乌托邦音乐字段后处理（enrich 模式·非乌托邦零开销）。
// 基础字段（Title/Subtitle/Description/Category…）由 Engine/legacy 提取，
// 本函数只组装 MusicMeta 挂 detail.Music（§59.466 独立通道）。
func (a *NexusPHPAdapter) enrichUtopiaMusic(config *model.SiteConfig, rawHTML string, detail *model.TorrentDetail) {
	if !isUtopiaDomain(config.Domain) || detail == nil {
		return
	}
	mm := buildUtopiaMusicMeta(rawHTML, detail.Title)
	if mm != nil {
		detail.Music = mm
	}
}

// buildUtopiaMusicMeta 组装乌托邦 MusicMeta（17 字段中 14 填——
// TrackList 无结构化区不提（desc 原样保真）/OtherVersions 站方无此字段）。
func buildUtopiaMusicMeta(rawHTML, h1Title string) *model.MusicMeta {
	mm := &model.MusicMeta{}
	fields := extractUtopiaFields(rawHTML)

	// ① 标题四段解析（Artist/Album/Year/Format/AudioMode）
	parseUtopiaMusicTitle(cleanUtopiaText(h1Title), mm)

	// ② 字段行：Log score（三源最强——直读）
	if v, ok := fields["Log score"]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			mm.LogScore = n
		}
	}
	// Log score 行或抓轨 Log 区在场 → HasLog
	if _, ok := fields["Log score"]; ok {
		mm.HasLog = true
	}
	// ③ 标签行（span 全量多维）+ Genre（含括号英文注释维——流行(Pop)）
	// （fields 值保留原始 HTML——span 结构供本提取）
	if tagsRaw, ok := fields["标签"]; ok {
		for _, m := range reUtopiaTagSpan.FindAllStringSubmatch(tagsRaw, -1) {
			if t := strings.TrimSpace(m[1]); t != "" {
				mm.Tags = append(mm.Tags, t)
				if mm.Genre == "" && strings.Contains(t, "(") {
					mm.Genre = t
				}
			}
		}
	}
	// ④ 基本信息嵌套 b 键值串（音频编码/媒介/类型）
	for _, m := range reUtopiaBasicKV.FindAllStringSubmatch(rawHTML, -1) {
		v := strings.TrimSpace(html.UnescapeString(m[2]))
		if v == "" {
			continue
		}
		switch m[1] {
		case "音频编码":
			if mm.Format == "" {
				mm.Format = v
			}
		case "媒介":
			mm.Media = v
		case "类型":
			mm.ReleaseType = v
		}
	}
	// ⑤ 副标题规格斜杠链 → HasCue（转载种形态"FLAC / Lossless / CD / Log (100%) / Cue"）
	if sub, ok := fields["副标题"]; ok {
		if strings.Contains(sub, "Cue") || strings.Contains(sub, "CUE") {
			mm.HasCue = true
		}
	}	// ⑥ Origin：转载自 URL（最强链——直达源种页面）+&amp; unescape
	if m := reUtopiaOriginURL.FindStringSubmatch(rawHTML); len(m) > 1 {
		mm.Origin = html.UnescapeString(m[1])
	}
	// ⑦ LogText：抓轨 Log 区（多碟 join）
	var logs []string
	for _, m := range reUtopiaRipLog.FindAllStringSubmatch(rawHTML, -1) {
		if t := strings.TrimSpace(cleanUtopiaText(m[1])); t != "" {
			logs = append(logs, t)
		}
	}
	if len(logs) > 0 {
		mm.LogText = strings.Join(logs, "\n\n")
		mm.HasLog = true
	}
	// ⑧ ShareRule：简介声明文本 grep（乌托邦无字段行——与皇后异；
	// 官组种声明"欢迎转载各大PT"）
	if descrIdx := strings.Index(rawHTML, `id="kdescr"`); descrIdx >= 0 {
		descrText := cleanUtopiaText(rawHTML[descrIdx:])
		for _, kw := range []string{"欢迎转载", "禁止转载", "不得转载", "请勿转载", "谢绝转载"} {
			if strings.Contains(descrText, kw) {
				mm.ShareRule = kw
				break
			}
		}
	}
	// 挂载门（§59.466 同款）：任一核心字段在场才挂
	if mm.Artist != "" || mm.Album != "" || mm.Format != "" {
		return mm
	}
	return nil
}

// parseUtopiaMusicTitle 标题双形态解析（split " - " 容错）：
// 三段（官组）：`陈慧娴 - 归来吧 1992 FLAC 分轨 - ZHENGLIFAN@Musopia`
// 四段（转载）：`7immy OST - INCON7ENIBLE 2007 - FLAC 分軌 - Redacted🌱…`
// 首段=Artist；len≥3 末段=后缀（不进 MusicMeta——发布翻译层消费）；
// 中段 join 后从右剥：AudioMode（皇后同款词序归一简体）→Format→Year→Album。
func parseUtopiaMusicTitle(title string, mm *model.MusicMeta) {
	// h1 尾部状态标记剥（[免费]/🌱保种区）
	title = reUtopiaTailSeed.ReplaceAllString(title, "")
	title = reUtopiaTailFree.ReplaceAllString(title, "")
	title = strings.TrimSpace(title)

	parts := strings.Split(title, " - ")
	if len(parts) < 2 {
		return
	}
	mm.Artist = strings.TrimSpace(parts[0])
	var mid []string
	if len(parts) >= 3 {
		mid = parts[1 : len(parts)-1] // 首段=艺人·末段=后缀（@Musopia/来源站）
	} else {
		mid = parts[1:2]
	}
	s := strings.TrimSpace(strings.Join(mid, " "))

	// AudioMode（皇后词序：分轨→分軌→整轨→整軌·归一简体；任意位置删词
	// ——乌托邦标题自由度高，模式词后可能有尾缀词（Enrique 种"FLAC 分轨
	// tulipa"实证——TrimSuffix 剥不掉致 Album 残留））
	for _, seg := range []string{"分轨", "分軌", "整轨", "整軌"} {
		if idx := strings.Index(s, seg); idx >= 0 {
			mm.AudioMode = string([]rune(seg)[:1]) + "轨" // 首字+轨（分軌→分轨）
			s = s[:idx] + " " + s[idx+len(seg):]
			break
		}
	}
	// Format（词表从长到短防 WAV/WavPack 前缀吞；任意位置删词）
	for _, w := range utopiaFormatWords {
		if idx := strings.Index(s, w); idx >= 0 {
			mm.Format = w
			s = s[:idx] + " " + s[idx+len(w):]
			break
		}
	}
	// Year（末位 4 位年份剥取）
	if locs := reUtopiaYear.FindAllStringIndex(s, -1); len(locs) > 0 {
		last := locs[len(locs)-1]
		mm.Year = s[last[0]:last[1]]
		s = s[:last[0]] + s[last[1]:]
	}
	// Album=剩余（剥词残留多空格归一——保真原则：站方尾缀词如 tulipa 保留；
	// 尾部 " -" 分隔符残留剥除——种名内部含 " - " 时 join 残留·许景淳案）
	mm.Album = strings.TrimSpace(strings.TrimSuffix(
		strings.Join(strings.Fields(s), " "), "-"))
}

// extractUtopiaFields rowhead 字段行提取键值对（双/单引号 class 兼容）。
// 值保留原始 HTML 片段（仅 nbsp 替换）——标签行 span 结构供提取，
// 纯文本字段消费方自行 clean。
func extractUtopiaFields(rawHTML string) map[string]string {
	fields := make(map[string]string)
	for _, m := range reUtopiaField.FindAllStringSubmatch(rawHTML, -1) {
		key := strings.TrimSpace(m[1])
		val := strings.TrimSpace(strings.ReplaceAll(m[2], "&nbsp;", " "))
		if key != "" && val != "" {
			fields[key] = val
		}
	}
	return fields
}

// cleanUtopiaText HTML 片段转纯文本（标签剥除+实体还原+空白归一）。
func cleanUtopiaText(s string) string {
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = regexp.MustCompile(`(?s)<[^>]+>`).ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(s, " "))
}
