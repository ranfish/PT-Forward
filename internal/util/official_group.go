package util

import "strings"

// OfficialGroupKey §59.211: @子组署名的官方组键。
// "cXcY@FRDS" → "FRDS"（@ 后段=官方组，前段=子组/发布者）；
// 无 @ 形态原样返回。§59.179 组名提取保留完整署名（身份/展示），
// 映射/家族/选站查表用本键（克兰弗德/恐怖大师/根 三种无源站映射案：
// 映射键 FRDS 与完整署名 cXcY@FRDS 失配）。
func OfficialGroupKey(group string) string {
	if at := strings.LastIndex(group, "@"); at >= 0 && at < len(group)-1 {
		return group[at+1:]
	}
	return group
}

// FuzzyGroupPrefixKey §59.220: 组名前缀回退——missed 组名以某已知键为前缀
// （CMCTf ⊃ CMCT）时返回最长键（发布者署名粘连字母/子组后缀形态）。
// 多候选取最长（最具体）；无前缀关系返回空。区别于编辑距离：CMCTA/CMCTV
// 与 CMCTf 同距 1 但非前缀，天然消歧。
func FuzzyGroupPrefixKey(missed string, keys []string) string {
	best := ""
	for _, k := range keys {
		if k == "" || len(k) >= len(missed) {
			continue
		}
		if strings.HasPrefix(missed, k) && len(k) > len(best) {
			best = k
		}
	}
	return best
}
