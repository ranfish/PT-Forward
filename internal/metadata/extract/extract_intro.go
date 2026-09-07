// Package extract 简介分段（§56.10 核心算法）。
//
// 借鉴 PTNexus review_extract.go:250-391：
//   - 首图位置 + quote 前后二分
//   - Statement 识别（官组声明 + 短文本）
//   - ARDTU 工具签名剥离
//   - Body 清理（移除已归入 Statement/ARDTU 的内容）
package extract

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// quoteBlockRe 提取 BBCode [quote] 块（含位置信息）。
var quoteBlockRe = regexp.MustCompile(`(?is)\[quote(?:=[^\]]*)?\](.*?)\[/quote\]`)

// stripQuoteLayoutImages §59.66→§59.172 附七: quote 引用内容剥离全部 [img] 图。
// §59.66 原方案 A 只剥相对路径布局图（trans.gif 类）、保留绝对 URL——
// §59.172 附七升级（用户定案）：诚实引用不包括图片，发布站点的引用是干净文本
// （引用渲染在海报上方，站规禁止上方有图）；真截图由 screenshots 字段独立管理
// （ExtractImages 在 quote 剥离前已全量提取，图不丢失）。
// Body 移除用 OrigFull（原文）不受剥图影响。
var quoteImgRe = regexp.MustCompile(`(?is)\[img\](.*?)\[/img\]`)

func stripQuoteLayoutImages(bbcode string) string {
	return quoteImgRe.ReplaceAllString(bbcode, "")
}

// quoteBlock BBCode 中的 quote 块（含位置信息）。
type quoteBlock struct {
	Start int    // 在 BBCode 中的起始位置（含 [quote] 标签）
	End   int    // 结束位置（含 [/quote] 标签）
	Full  string // 完整 [quote]...[/quote] 文本（剥图后——声明区消费）
	OrigFull string // 原始未剥图文本（Body 移除消费——ReplaceAll 需与原文精确匹配）
	Inner string // quote 内部文本（剥图后——分类判定消费）
}

// splitIntroSections 简介分段主入口。
// 输入：描述容器 HTML（提取图片用） + BBCode（提取 quote 用）
// + widenKF keepfrds 头区拓宽开关（§59.172）
// 输出：IntroData{Statement, Poster, Body, Screenshots, RemovedARDTUDeclarations}
func (p *PublicExtractor) splitIntroSections(descrHTML, descrBBCode string, widenKF bool) IntroData {
	intro := IntroData{}

	// 1. 抽取所有图片 URL + 归一化 + 去重 + 黑白名单过滤（§56.12）
	validImages := ExtractImages(descrHTML, descrBBCode)

	// 2. 识别海报（4 层优先级：站点特殊 > URL 关键词 > 首图）
	posterURL, screenshots := ClassifyPosterAndScreenshots(validImages, p.siteCode)
	intro.Poster = posterURL
	intro.SetScreenshotURLs(screenshots)

	// 3. 提取 quote 块（§59.66: 引用内容剥站内布局图——trans.gif 类小黑点）
	quotes := extractQuoteBlocks(descrBBCode)
	for i := range quotes {
		quotes[i].Full = stripQuoteLayoutImages(quotes[i].Full)
		quotes[i].Inner = stripQuoteLayoutImages(quotes[i].Inner)
	}
	// §59.67: 仅保留顶层块——嵌套块随外层原样承载（外层 Inner 含内层完整原文），
	// 内层若再分类会重复入 statement（嵌套重复采集 bug 实锤修复）。
	quotes = topLevelQuotes(quotes)

	// 4. 确定拆分点：海报位置 > MediaInfo 位置 > 全部归类
	posterIdx := -1
	if posterURL != "" {
		posterIdx = strings.Index(descrBBCode, posterURL)
	}
	if posterIdx < 0 {
		posterIdx = findMediaInfoPosition(descrBBCode)
	}
	// §59.172: keepfrds 头区拓宽——早期种子的引用/鸣谢块（fieldset 与 dash 归一
	// 后的 [quote]）大量落在【海报→影片详情】之间（tid=9073/4554 实证），原"海报前"
	// 分类域全部漏采。锚拓宽到首个影片详情标记（◎/【 双形态，附三）；无标记
	// （kdouban 框架页）自然回退海报前。
	if widenKF {
		if ki := kfHeadAnchor(descrBBCode); ki > posterIdx {
			posterIdx = ki
		}
	}
	beforePoster, _ := splitQuotesByPosition(quotes, posterIdx)

	// 5. 分类首图前的 quote 块
	// §59.172 附五: keepfrds 头区（含海报前+拓宽段）quote 豁免鸣谢门槛——
	// IsAcknowledgmentQuote 的 <200字/关键词规则为通用站声明短句设计，
	// keepfrds 头区 quote 位置即信号（发布者引用/说明约定区）：
	// tid=9246 制作说明四段 / tid=11254 海报前长鸣谢（"字幕库"≠"字幕组"）双实证。
	// MI/ARDTU 垃圾滤保留。
	statements, ardtuFulls, stmtFulls := classifyBeforePosterQuotes(beforePoster, widenKF)
	intro.Statement = strings.Join(statements, "\n\n")
	intro.RemovedARDTUDeclarations = ardtuFulls

	// 6. 构建 Body：整个 BBCode 去掉 Statement/ARDTU 对应的 quote 整块
	body := descrBBCode
	for _, full := range ardtuFulls {
		body = strings.ReplaceAll(body, full, "")
	}
	for _, full := range stmtFulls {
		body = strings.ReplaceAll(body, full, "")
	}
	body = compactBlankLines(body)
	intro.Body = strings.TrimSpace(body)

	return intro
}

// extractQuoteBlocks 从 BBCode 中提取所有 [quote] 块（含位置信息）。
// §59.26: 栈式解析器，正确处理嵌套 [quote][quote][/quote][/quote]。
// 返回所有匹配对（含内层），按起始位置排序。外层 block 的 inner 包含内层 block 原样。
func extractQuoteBlocks(bbcode string) []quoteBlock {
	openRe := regexp.MustCompile(`(?i)\[quote(?:=[^\]]*)?\]`)
	closeTagLower := "[/quote]"

	type token struct {
		pos    int // tag 起始位置
		end    int // tag 结束位置（exclusive）
		isOpen bool
	}

	var tokens []token
	for _, m := range openRe.FindAllStringIndex(bbcode, -1) {
		tokens = append(tokens, token{pos: m[0], end: m[1], isOpen: true})
	}
	lower := strings.ToLower(bbcode)
	searchFrom := 0
	for {
		idx := strings.Index(lower[searchFrom:], closeTagLower)
		if idx < 0 {
			break
		}
		absIdx := searchFrom + idx
		closeEnd := absIdx + len(closeTagLower)
		tokens = append(tokens, token{pos: absIdx, end: closeEnd, isOpen: false})
		searchFrom = closeEnd
	}

	sort.Slice(tokens, func(i, j int) bool { return tokens[i].pos < tokens[j].pos })

	var stack []token
	var blocks []quoteBlock
	for _, tok := range tokens {
		if tok.isOpen {
			stack = append(stack, tok)
		} else {
			if len(stack) == 0 {
				continue
			}
			openTag := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			blocks = append(blocks, quoteBlock{
				Start: openTag.pos,
				End:   tok.end,
				Full:  bbcode[openTag.pos:tok.end],
				OrigFull: bbcode[openTag.pos:tok.end],
				Inner: bbcode[openTag.end:tok.pos],
			})
		}
	}
	return blocks
}

// topLevelQuotes §59.67: 过滤掉被其他块包含的嵌套块（外层 Inner 已含内层原样，
// 分类只看顶层——嵌套引用"依原样"由外层承载，内层不再独立分类避免重复采集）。
func topLevelQuotes(quotes []quoteBlock) []quoteBlock {
	var out []quoteBlock
	for _, q := range quotes {
		nested := false
		for _, o := range quotes {
			if &o == &q {
				continue
			}
			if o.Start < q.Start && q.End <= o.End {
				nested = true
				break
			}
		}
		if !nested {
			out = append(out, q)
		}
	}
	return out
}

// miFragmentLineRe §59.78: MI 段名/字段行形态。
// mUHD 制作者把 MI 摘要拆成多个小 quote（[quote]General[/quote]、
// [quote]Container: Matroska\nRuntime...[/quote]）——"短文本即声明"启发式
// 对其失效（墓碑镇实锤混入 Statement）。整块每一行都匹配才判定。
var miFragmentLineRe = regexp.MustCompile(`(?i)^(?:\[b\]|\[/b\])*(general|video(\s*\(\d+\))?|audio(\s*\(\d+\))?|subtitles?(\s*\(\d+\))?|text(\s*\(\d+\))?|chapters?|menu|image(?:\s*\(\d+\))?|(container|runtime|size|format|overall bit rate|width|height|bit rate|frame rate|writing library|codec id|duration)\s*:.*|#\d+\s*:.*(?:channels|kb/s|mb/s|gb|mib|gib).*)$`)

// isMISectionQuote 判定 quote 块是否为 MI 碎片（索引段名或元数据字段块）。
// 每行（剥空白）都必须匹配 MI 形态；混有任何非 MI 行即不是碎片。
func isMISectionQuote(inner string) bool {
	trimmedInner := strings.TrimSpace(inner)
	if trimmedInner == "" {
		return false
	}
	lines := strings.Split(trimmedInner, "\n")
	if len(lines) == 0 {
		return false
	}
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		if !miFragmentLineRe.MatchString(ln) {
			return false
		}
	}
	return true
}

// findMediaInfoPosition 在 BBCode 中查找 MediaInfo 段的起始位置。
// §59.26: 无海报时用 MediaInfo 位置作声明/正文的拆分点。
var mediaInfoMarkerRe = regexp.MustCompile(`(?im)(?:^|\n)\s*(?:\[url[^\]]*\]\s*)?MediaInfo\s*[:：]|(?:^|\n)\s*\[quote\]\s*General\s*\n`)

func findMediaInfoPosition(bbcode string) int {
	loc := mediaInfoMarkerRe.FindStringIndex(bbcode)
	if loc != nil {
		return loc[0]
	}
	return -1
}

// splitQuotesByPosition 按 posterIdx 二分 quote 块。
// posterIdx < 0（无首图、无 MediaInfo）→ 全部归 before（跑声明分类，不丢内容）。
func splitQuotesByPosition(quotes []quoteBlock, posterIdx int) (before, after []quoteBlock) {
	if posterIdx < 0 {
		return quotes, nil
	}
	for _, q := range quotes {
		if q.End <= posterIdx {
			before = append(before, q)
		} else {
			after = append(after, q)
		}
	}
	return before, after
}

// classifyBeforePosterQuotes 分类首图前的 quote 块。
// 顺序很重要：By ARDTU 前缀优先（能保留正文），其次工具签名/技术参数（剥离），最后官组声明（保留）。
func classifyBeforePosterQuotes(quotes []quoteBlock, lenientAck bool) (statements, ardtuFulls, stmtFulls []string) {
	for _, q := range quotes {
		text := strings.TrimSpace(q.Inner)
		if text == "" {
			continue
		}
		// 0. §59.78 MI 碎片引用（mUHD 摘要形态）→ 整块剥离（不入 Statement 也不留 Body）
		if isMISectionQuote(q.Inner) {
			ardtuFulls = append(ardtuFulls, q.OrigFull)
			continue
		}
		// 1. By ARDTU@... 前缀 → 剥离前缀归 Statement，整块从 Body 移除
		if IsByARDTUWithGroupInfo(text) {
			stripped := StripByARDTUPrefix(text)
			if stripped != "" {
				statements = append(statements, stripped)
			}
			ardtuFulls = append(ardtuFulls, q.OrigFull)
			continue
		}
		// 2. ARDTU 工具签名 / 技术参数块 → 整块移除
		if IsToolSignatureQuote(text) || IsTechParamsQuote(text) {
			ardtuFulls = append(ardtuFulls, q.OrigFull)
			continue
		}
		// 3. 官组声明 → 归 Statement（完整 BBCode），整块从 Body 移除
		// §59.172 附五: lenientAck（keepfrds 头区）豁免鸣谢门槛——位置即信号
		// §59.172 附七: statement 用剥图 Full（引用=干净文本）；
		// stmtFulls 用 OrigFull（Body ReplaceAll 需原文精确匹配——§59.66 失配教训）
		if lenientAck || IsAcknowledgmentQuote(text) {
			statements = append(statements, q.Full)
			stmtFulls = append(stmtFulls, q.OrigFull)
			continue
		}
		// 4. 其他 → 保留在 Body 中（不丢内容）
	}
	return statements, ardtuFulls, stmtFulls
}

// compactBlankLines 压缩 3+ 换行为 2 个。
func compactBlankLines(s string) string {
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}

// kfHeadDashQuoteRe §59.172: dash 族引用段——----…---- / ——…——（全角破折号变体，
// tid=7025 实证 18 例）。内容限单行内（引用是行内短句，跨行的是正文/分隔线）。
// 内容首字符排除分隔符本身——纯分隔线（全 dash）无内容不匹配（单测实证）。
var kfHeadDashQuoteRe = regexp.MustCompile(`(-{4,}[ \t]*[^\s\n\[\]{}-][^\n\[\]{}]{2,180}?-{4,}|—{2,}[ \t]*[^\s\n\[\]{}—-][^\n\[\]{}]{2,180}?—{2,})`)


// kfHeadAnchor §59.172 附三/附六: 头区锚——首个影片详情标记位置（三形态取先到）：
// ◎（PTGen 格式）/ 【（老式【原 片 名】格式，tid=4186 等 34 种）/
// 导演[：:]（冒号格式豆瓣老模板，tid=5979 实证）。均无返回 -1（kdouban 框架页回退不动）。
func kfHeadAnchor(bbcode string) int {
	anchor := strings.Index(bbcode, "◎")
	for _, marker := range []string{"【", "导演:", "导演："} {
		if k := strings.Index(bbcode, marker); k >= 0 && (anchor < 0 || k < anchor) {
			anchor = k
		}
	}
	return anchor
}

// normalizeKFHeadDashQuotes §59.172: keepfrds 头区 dash 族引用归一——包装 [quote]
// 使分类器可见。域纪律：仅头区（首个影片详情标记前）；无标记不动（回退）。
// 诚实透传语义：内容原样（含 ---- 分隔符本身）不增不删。
// 两种形态（§59.172 附，tid=7025 实证补充）：
//  ① 双侧分隔：----文字---- / ——文字——（kfHeadDashQuoteRe）
//  ② 行首单侧：[b]——文字[/b]——分隔符只在行首（7025 全角破折号变体实为单侧）
// normalizeKFHeadDashQuotes §59.172 附八: keepfrds 头区 dash 族引用归一——
// 合并采集（用户定案：多行引用内容包裹为同一个引用块，剥离源站 ----/—— 引用
// 符号——[quote] 本身承载引用语义，分隔符是源站的无 BBCode 化排版痕迹）。
// 域纪律：仅头区（首个影片详情标记前）；无标记不动（kdouban 框架页回退）。
// 已有 [quote] 区（fieldset 产物）遮蔽保护——fieldset 块独立保留不参与合并。
// normalizeKFHeadDashQuotes §59.172 附八: keepfrds 头区 dash 族引用归一——
// 合并采集（用户定案：多行引用合并为同一个引用块，剥离源站分隔符——
// [quote] 本身承载引用语义，----/—— 是源站无 BBCode 化的排版痕迹）。
// 分隔符变体全覆盖（附八：任意 dash 族字符混搭、任意数量、单侧/双侧）：
// ----…---- / ——…—— / ----…--- / ——… / ──…── 等——逐行判行首分隔符即入选。
// 域纪律：仅头区（首个影片详情标记前）；无标记不动（kdouban 框架页回退）。
// 已有 [quote] 区（fieldset 产物）遮蔽保护——fieldset 块独立保留不参与合并。
func normalizeKFHeadDashQuotes(bbcode string) string {
	headEnd := kfHeadAnchor(bbcode)
	if headEnd < 0 {
		return bbcode
	}
	head, rest := bbcode[:headEnd], bbcode[headEnd:]

	placeholders := make([]string, 0, 4)
	maskQuote := func(s string) string {
		return reQuoteSpan.ReplaceAllStringFunc(s, func(m string) string {
			placeholders = append(placeholders, m)
			return fmt.Sprintf("\x00KFQ%d\x00", len(placeholders)-1)
		})
	}
	unmaskQuote := func(s string) string {
		for i, p := range placeholders {
			s = strings.ReplaceAll(s, fmt.Sprintf("\x00KFQ%d\x00", i), p)
		}
		return s
	}
	head = maskQuote(head)

	// 逐行统一收集：行首有 dash 族分隔符 + 剥壳后非空 → 引用行
	type seg struct{ start, end int; text string }
	var segs []seg
	lineBase := 0
	for _, ln := range strings.Split(head, "\n") {
		lineBase += len(ln) + 1
		t := strings.TrimSpace(ln)
		if t == "" || strings.Contains(t, "[quote]") || strings.Contains(t, "\x00KFQ") {
			continue
		}
		probe := t
		if strings.HasPrefix(probe, "[b]") {
			probe = probe[len("[b]"):]
		}
		probe = strings.TrimSuffix(probe, "[/b]")
		inner := strings.TrimLeft(probe, "-—─ \t")
		if inner == probe {
			continue // 行首无分隔符——非引用行
		}
		inner = strings.TrimRight(inner, "-—─ \t")
		inner = strings.ReplaceAll(inner, "[b]", "")
		inner = strings.ReplaceAll(inner, "[/b]", "")
		inner = strings.TrimSpace(inner)
		if inner == "" {
			continue // 纯分隔线
		}
		lineStart := lineBase - len(ln) - 1 + strings.Index(ln, t)
		segs = append(segs, seg{lineStart, lineStart + len(t), inner})
	}
	if len(segs) == 0 {
		return unmaskQuote(head) + rest
	}

	// 重建：删除全部引用行，首段原位插入合并 [quote]
	insertAt := segs[0].start
	var b strings.Builder
	pos := 0
	for _, sg := range segs {
		if sg.start > pos {
			b.WriteString(head[pos:sg.start])
		}
		pos = sg.end
	}
	b.WriteString(head[pos:])
	texts := make([]string, 0, len(segs))
	for _, sg := range segs {
		texts = append(texts, sg.text)
	}
	merged := "[quote]" + strings.Join(texts, "\n") + "[/quote]"
	out := b.String()
	out = out[:insertAt] + merged + out[insertAt:]
	out = regexp.MustCompile(`(?s)\[b\]\s*\[/b\]`).ReplaceAllString(out, "")
	for strings.Contains(out, "\n\n\n") {
		out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	}
	return unmaskQuote(out) + rest
}

// reQuoteSpan 已有 [quote] 块整体匹配（遮蔽用——栈式配对简化为非贪婪跨块）。
var reQuoteSpan = regexp.MustCompile(`(?s)\[quote[^\]]*\].*?\[/quote\]`)
