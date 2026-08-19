package extract

import (
	"strings"
)

// 默认禁转关键词（2b.4 硬编码 + TODO 接入 §56.1 standard_keys.is_protected）。
// 后续 6a/§56.21 实施后改为 DB 驱动。
var defaultForbiddenTransferKeywords = []string{
	"禁转", "禁止转载", "谢绝转载", "严禁转载", "谢绝搬运",
	"独占", "限时禁转", "限转", "分集",
}

// 站点级禁转标记形态（§32 keepfrds.md 六章：无标签体系站点的权威标记，
// 带方括号包裹，出现在列表页种子名称后/详情页内）。
var siteForbiddenMarkers = []string{
	"[禁转]", "【禁转】",
	"[限时禁转]", "【限时禁转】",
	"【暂时禁转，待确认】",
	"[暂时禁转]",
	"[限转资源]", "【限转资源】",
}


// directedNoTransferSuffixes 定向禁转后缀："禁转"+目标站名（禁转PTT/禁转BT）。
// 是发种者/致谢模板对特定站点的定向声明，非全站禁转。
var directedNoTransferSites = []string{"PTT", "BT", "PTT "}

// stripQuoteBlocks 剥离 BBCode [quote]...[/quote] 块（§59.39）。
// 逐段扫描，块内文本丢弃；嵌套以 [quote 计数、[/quote] 减计数处理；
// 不成对标签保守保留其后文本（宁可多扫不可漏扫）。
func stripQuoteBlocks(s string) string {
	var b strings.Builder
	depth := 0
	for len(s) > 0 {
		qi := strings.Index(s, "[quote")
		ei := strings.Index(s, "[/quote]")
		if qi >= 0 && (ei < 0 || qi < ei) {
			if depth == 0 {
				b.WriteString(s[:qi])
			}
			tagEnd := strings.Index(s[qi:], "]")
			if tagEnd < 0 {
				return b.String()
			}
			s = s[qi+tagEnd+1:]
			depth++
		} else if ei >= 0 {
			if depth > 0 {
				depth--
			}
			s = s[ei+len("[/quote]"):]
		} else {
			// §59.39 保守语义：未闭合 quote 的剩余文本按正文保留（宁可多扫不可漏扫）
			b.WriteString(s)
			return b.String()
		}
	}
	return b.String()
}

// extractFlags 从文本中检测禁转/限转标记。
// 扫描范围限定为：标题 + 副标题 + 简介 BBCode。
//
// §32 keepfrds.md 对齐的两层检测：
//  1. 站点标记形态（[禁转]/[限时禁转] 等）— 精确匹配，权威源
//  2. 关键词匹配 — "禁转"/"限转" 短词逐命中点检查上下文：
//     排除跨词伪命中（"严禁转发"中"禁转"二字相邻）和定向禁转（"禁转PTT"）
//
// §59.39: 关键词层扫描文本剥离 [quote] 引用块——发布者引用的上游声明
// （"美版原盘@AdBlue…仅在PD22测试,未经允许禁止转载"类溯源文本）约束的是
// 上游站点，非本种子禁转标记；站点标记层不排除（站方权威标记不会藏在上游引用里）。
func (p *PublicExtractor) extractFlags(title, subtitle, descrBBCode string) []string {
	combined := title + " " + subtitle + " " + descrBBCode
	var flags []string
	seen := make(map[string]struct{})

	addFlag := func(kw string) {
		if _, ok := seen[kw]; !ok {
			seen[kw] = struct{}{}
			flags = append(flags, kw)
		}
	}

	// 1. 站点标记形态：精确匹配（权威源，直接命中）——不剥离 quote（§59.39）
	for _, marker := range siteForbiddenMarkers {
		if strings.Contains(combined, marker) {
			switch {
			case strings.Contains(marker, "限时禁转"):
				addFlag("限时禁转")
			case strings.Contains(marker, "限转资源"):
				addFlag("限转")
			default:
				addFlag("禁转")
			}
		}
	}

	// 2. 关键词匹配——剥离 quote 引用块后扫描（§59.39：只判发布者自身文本）
	ownText := stripQuoteBlocks(combined)
	for _, kw := range defaultForbiddenTransferKeywords {
		if !strings.Contains(ownText, kw) {
			continue
		}
		// 长词（禁止转载/谢绝转载/严禁转载/谢绝搬运/独占/限时禁转/分集）
		// 语义自足，直接命中。
		// 注意："限时禁转" 含 "禁转" 子串，先于 "禁转" 判定（defaultForbiddenTransferKeywords 顺序）。
		if kw == "禁转" || kw == "限转" {
			if !hasGenuineHit(ownText, kw) {
				continue
			}
		}
		addFlag(kw)
	}

	if flags == nil {
		flags = []string{}
	}
	return flags
}

// hasGenuineHit 检查 target（"禁转"/"限转"）是否存在至少一个"真命中"：
//   - 跨词伪命中（前缀是 严禁/禁止/谢绝 → "严禁转发"形态）→ 排除
//   - 定向禁转（后缀紧跟站点名 PTT/BT，如 "禁转PTT"）→ 排除
//   - 其余（独立"禁转"词，如 "[禁转]"已由层1处理、"本资源禁转"）→ 真命中
func hasGenuineHit(s, target string) bool {
	runes := []rune(s)
	targetRunes := []rune(target)
	n := len(targetRunes)
	for i := 0; i+n <= len(runes); i++ {
		if string(runes[i:i+n]) != target {
			continue
		}
		if isFalsePositiveHit(runes, i, n) {
			continue
		}
		return true
	}
	return false
}

// crossWordPrevRunes "禁转"/"限转"跨词伪命中的前一 rune：
// "严禁转(发)" / "禁止转(发)" / "谢绝转(发)" 中命中点前一个 rune。
var crossWordPrevRunes = []rune{'严', '止', '绝', '勿', '得'}

// isFalsePositiveHit 判断 runes[i:i+n] 处的命中是否为伪命中。
func isFalsePositiveHit(runes []rune, i, n int) bool {
	// 前一 rune 检查：严/止/绝/勿/得 + target → "严禁转发"跨词形态
	if i > 0 {
		prev := runes[i-1]
		for _, r := range crossWordPrevRunes {
			if prev == r {
				return true
			}
		}
	}
	// 后缀检查：target + 站点名（允许中间空格/冒号）→ 定向禁转
	rest := strings.TrimLeft(string(runes[i+n:]), " 　:：")
	for _, site := range directedNoTransferSites {
		siteTrim := strings.TrimSpace(site)
		if siteTrim != "" && strings.HasPrefix(rest, siteTrim) {
			return true
		}
	}
	// 英文/数字紧跟（禁转2天/禁转BT等其它目标）也视为定向
	if len(runes) > i+n {
		next := runes[i+n]
		if isDirectedRune(next) {
			return true
		}
	}
	return false
}

// isDirectedRune 判断 target 后紧跟的 rune 是否构成定向修饰
// （英文字母/数字，如 禁转PTT、禁转BT、限转3天）。
func isDirectedRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// ExtractFlagsFromText 公共出口（§59.28 I1）：legacy 正则兜底路径复用两层检测。
// 输入文本（title/subtitle/tags 拼接），输出 flags（与 Engine 主路径口径一致）。
func ExtractFlagsFromText(text string) []string {
	p := PublicExtractor{}
	return p.extractFlags(text, "", "")
}
