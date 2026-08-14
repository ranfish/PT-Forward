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
