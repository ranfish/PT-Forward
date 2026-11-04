package util

import (
	"strings"
	"unicode"
)

// PersonName §59.462: 人名中英拆分（公共层——GazellePW 建组 artists 四件套
// 的 Name/SubName 分离；未来其它要 Name/SubName 的站复用）。
//
// PTGen director/cast 形态：中文名+英文名空格混排（"克里斯·帕拉特 Chris Pratt"、
// "提莫·贝克曼贝托夫 Timur Bekmambetov"）；纯英文（"Quentin Tarantino"）或
// 纯中文（"张艺谋"）形态也容错。
type PersonName struct {
	Name    string // 英文名（拉丁部分）
	SubName string // 中文名（CJK 部分）
}

// SplitPersonName 中英人名拆分——按 CJK 与 Latin 词边界切分。
// 找到首个连续 Latin 词段起点作为英文名起点（后面全部归英文名——姓氏
// 空格+中点句点等标点跟随）；CJK 段（含中点·）为中文名。
// 纯单一语种时 Name=原文/SubName 空（英文形态）或 Name 空/SubName=原文
// （中文形态——调用方按站方语义处理缺名兜底）。
func SplitPersonName(s string) PersonName {
	s = strings.TrimSpace(s)
	if s == "" {
		return PersonName{}
	}
	runes := []rune(s)
	// 找首个 Latin 字母起点（前导允许 CJK/中点）
	latinStart := -1
	for i, r := range runes {
		if isLatinRune(r) {
			latinStart = i
			break
		}
	}
	if latinStart < 0 {
		// 无 Latin（纯中文）——全 SubName
		return PersonName{SubName: s}
	}
	if latinStart == 0 {
		// Latin 开头——全 Name（纯英文形态）
		return PersonName{Name: s}
	}
	// 混排：[0,latinStart)=中文 [latinStart,)=英文
	sub := strings.TrimSpace(string(runes[:latinStart]))
	name := strings.TrimSpace(string(runes[latinStart:]))
	return PersonName{Name: name, SubName: sub}
}

func isLatinRune(r rune) bool {
	return unicode.Is(unicode.Latin, r)
}
