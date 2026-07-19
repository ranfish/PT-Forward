// Package publish tag 互斥/覆盖规则引擎（§56.22 Q7）。
//
// HDR/DV 共存规则（§33 调研）:
//   - 杜比视界 + HDR10 → 可以共存
//   - HDR10 / HDR10+ / HDR Vivid / HLG / SDR 相互之间不能共存
//   - HDR10+ 存在时自动移除 HDR10（超集关系）
package publish

// tagCoverRules 覆盖规则（key 是超集，value 是被覆盖的子集）。
// 当 key 存在时，自动移除 value。
var tagCoverRules = map[string]string{
	"hdr10_plus": "hdr10", // HDR10+ 是 HDR10 超集
}

// tagMutexGroups 互斥规则组（组内仅保留首个命中的）。
var tagMutexGroups = [][]string{
	{"hdr10", "hdr10_plus", "hlg", "sdr"}, // HDR 标准互斥（DV 可共存）
}

// ApplyTagRules 应用互斥/覆盖规则。
//   1. 覆盖规则：hdr10_plus 存在时移除 hdr10
//   2. 互斥规则：互斥组内仅保留首个命中的
//   3. 去重去空
func ApplyTagRules(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}

	// 去重
	seen := make(map[string]struct{})
	deduped := make([]string, 0, len(tags))
	for _, t := range tags {
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		deduped = append(deduped, t)
	}

	// 1. 覆盖规则
	for cover, covered := range tagCoverRules {
		if containsTag(deduped, cover) {
			deduped = removeTag(deduped, covered)
		}
	}

	// 2. 互斥规则（仅保留首个命中的）
	for _, group := range tagMutexGroups {
		firstHit := ""
		for _, member := range group {
			if containsTag(deduped, member) {
				firstHit = member
				break
			}
		}
		if firstHit == "" {
			continue
		}
		// 移除组内其他成员
		for _, member := range group {
			if member != firstHit {
				deduped = removeTag(deduped, member)
			}
		}
	}

	return deduped
}

func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

func removeTag(tags []string, tag string) []string {
	result := make([]string, 0, len(tags))
	for _, t := range tags {
		if t != tag {
			result = append(result, t)
		}
	}
	return result
}
