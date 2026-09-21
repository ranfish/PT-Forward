package titleparser

// merge_ptgen.go §59.254 项1: PT-Gen 第四源合并层。
//
// 定案（docs/31 §59.254 三/六节）：
//   year 校正：单 4 位年且 ≠ PTGen 年 → 用 PTGen 值 + year_corrected 日志；
//              区间（2015-2022）/日期（20240326）形态跳过（语义不同维度）
//   main_title 兜底：空/纯 CJK（无拉丁段）→ EnglishName 填入
//                    （兜底值由调用方按端点感知链准备：aka 启发式 → &imdb 二连）
//   title_mismatch 可观测：归一化比较（小写/去标点/去冠词/词集）不匹配只记不换
//   作用域：TechProfile 合并层（meta.Title 源数据不动——审计可回溯）

import (
	"strings"
	"unicode"
)

// PTGenMergeInput 第四源输入（调用方从 PTGenResult 派生）
type PTGenMergeInput struct {
	EnglishName string // 端点感知链产出的英文规范名（aka 启发式/&imdb 二连）
	Year        string // PTGen ◎年代
}

// MergePTGenInto §59.254 项1: 第四源合并（在 DOM 合并后调用——内容元数据最后仲裁）
// logger 可为 nil（跳过日志——测试形态）。
func MergePTGenInto(p *TechProfile, in PTGenMergeInput, loggerWarn func(msg string, fields ...any)) {
	if p == nil {
		return
	}
	mergePTGenYear(p, in, loggerWarn)
	mergePTGenMainTitle(p, in, loggerWarn)
}

// mergePTGenYear year 校正（争议点 2 终案+三补丁）
func mergePTGenYear(p *TechProfile, in PTGenMergeInput, warn func(string, ...any)) {
	py := strings.TrimSpace(in.Year)
	if py == "" || !isFourDigitYear(py) {
		return // PTGen 无年/非 4 位（如 "2010-2012"）不参与
	}
	ty := strings.TrimSpace(p.Year)
	if ty == "" || !isFourDigitYear(ty) {
		if ty == "" && isFourDigitYear(py) {
			p.Year = py // 标题空年→补（良性填充）
		}
		return // 区间/日期/空形态跳过（语义不同维度）
	}
	if ty != py {
		old := p.Year
		p.Year = py
		if warn != nil {
			warn("year_corrected (PTGen override)", "title_year", old, "ptgen_year", py)
		}
	}
}

// isFourDigitYear 严格单 4 位年（参与校正的唯一形态）
func isFourDigitYear(s string) bool {
	if len(s) != 4 {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// mergePTGenMainTitle 兜底+可观测（争议点 1 终案）
func mergePTGenMainTitle(p *TechProfile, in PTGenMergeInput, warn func(string, ...any)) {
	en := strings.TrimSpace(in.EnglishName)
	mt := strings.TrimSpace(p.MainTitle)
	if mt == "" {
		// 空标题 → 兜底
		if en != "" {
			p.MainTitle = en
			if warn != nil {
				warn("main_title_backfilled (PTGen english)", "english_name", en)
			}
		}
		return
	}
	if !hasLatinSegment(mt) {
		// 纯 CJK（无拉丁段）→ 兜底
		if en != "" {
			p.MainTitle = en
			if warn != nil {
				warn("main_title_backfilled (pure CJK)", "original", mt, "english_name", en)
			}
		}
		return
	}
	// 有拉丁段：可观测 title_mismatch（只记不换——观察期攒证据）
	if en != "" && !titlesNormalizedEqual(mt, en) {
		if warn != nil {
			warn("title_mismatch (observe only, no replace)", "title", mt, "ptgen_english", en)
		}
	}
}

// hasLatinSegment 含 ASCII 拉丁字母段（意语原名等组风格保留判定）
func hasLatinSegment(s string) bool {
	for _, r := range s {
		if r < 0x80 && unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// titlesNormalizedEqual 归一化标题比较：小写+去标点+去冠词（the/a/an）+词集
func titlesNormalizedEqual(a, b string) bool {
	wa := normalizeTitleWords(a)
	wb := normalizeTitleWords(b)
	if len(wa) != len(wb) {
		return false
	}
	set := make(map[string]int, len(wa))
	for _, w := range wa {
		set[w]++
	}
	for _, w := range wb {
		set[w]--
		if set[w] < 0 {
			return false
		}
	}
	return true
}

// normalizeTitleWords 词归一：小写/去非字母数字/去冠词/罗马数字字符转写
func normalizeTitleWords(s string) []string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9':
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteRune(' ')
		}
	}
	parts := strings.Fields(b.String())
	out := make([]string, 0, len(parts))
	for _, w := range parts {
		switch w {
		case "the", "a", "an":
			continue // 冠词不参与
		}
		out = append(out, w)
	}
	return out
}
