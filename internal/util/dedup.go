package util

// DedupByKey 按 key 函数去重，保留第一次出现的元素（调用方应先按需要的顺序排序）。
func DedupByKey[T any](items []T, getKey func(T) string) []T {
	if len(items) == 0 {
		return items
	}
	seen := make(map[string]bool, len(items))
	result := make([]T, 0, len(items))
	for _, item := range items {
		key := getKey(item)
		if key == "" {
			result = append(result, item)
			continue
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, item)
	}
	return result
}

// DedupStrings §59.551: 保序去重字符串切片，空字符串也按值去重（只保留首次）。
// 行为契约（勿改委托 DedupByKey）：DedupByKey 对空 key 直接保留不去重——
// 本函数与历史 dedupTags 语义完全对齐（空串参与去重）。若要统一空 key
// 策略属契约级讨论，须先评估调用面再动。
// 收敛自 publish/tag_inferer.go 的 dedupTags 私有副本。
func DedupStrings(items []string) []string {
	if len(items) == 0 {
		return items
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, s := range items {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
