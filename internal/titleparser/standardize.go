// Package titleparser 标准键显示名反查（§59.35 v2：派生自 dict 词条）。
//
// 旧机制（standard_mappings.json + canonicalDisplay 手写 map）已删除，
// reverse map 由 dict/*.json 的 StandardKey + Display 派生——
// r4320p 类抄写错误结构性消失（dict_consistency_test.go 护航）。
package titleparser

// ReverseLookup 标准键 → 规范显示名（§59.35 派生自 dict reverse map）。
// 未收录 key 返回空。
func ReverseLookup(standardKey string) string {
	return ensureDict().reverse[standardKey]
}

// ReverseLookupWithFallback 标准键 → 规范显示名，带降级链
func ReverseLookupWithFallback(standardKey string, fallbackChains map[string][]string) string {
	display := ReverseLookup(standardKey)
	if display != "" {
		return display
	}
	// 降级链：category.animation → [category.movie, category.other]
	if chain, ok := fallbackChains[standardKey]; ok {
		for _, fb := range chain {
			if d := ReverseLookup(fb); d != "" {
				return d
			}
		}
	}
	return ""
}
