// Package publish tag 互斥/覆盖规则引擎（§56.22 Q7）。
//
// HDR/DV 共存规则（§33 调研）:
//   - 杜比视界 + HDR10 → 可以共存
//   - HDR10 / HDR10+ / HDR Vivid / HLG / SDR 相互之间不能共存
//   - HDR10+ 存在时自动移除 HDR10（超集关系）
package publish

import "github.com/ranfish/pt-forward/internal/titleparser"

// tagCoverRules 覆盖规则（key 是超集，value 是被覆盖的子集）。
// 当 key 存在时，自动移除 value。
var tagCoverRules = map[string]string{
	"hdr10_plus": "hdr10", // HDR10+ 是 HDR10 超集
}

// tagMutexGroups 互斥规则组（组内仅保留首个命中的）。
var tagMutexGroups = [][]string{
	{"hdr10", "hdr10_plus", "hlg", "sdr"}, // HDR 标准互斥（DV 可共存）
	// §59.71 B1 剧集状态互斥（藏宝阁 7.6 判定优先级：检测到完结不算分集；
	// 检测到分集或合集不算完结——同族标识互斥，序=抑制方向）
	{"complete", "episode_split", "collection"},
	// §59.72 B2: 连载↔合集互斥（ubits 文档: 连载/合集二选一）
	{"ongoing", "collection"},
}

// MergeTags §59.74: 直采+推断标签合并单点。
// ①直采惰性归一（NormalizeTagDisplay——自愈 v0.0.718 前落库的显示名旧数据，miss 保留原文）
// ②直采在前（互斥组保序=直采赢——"直采优先，推断补差"在冲突时强制成立）
// ③并集去重 → ApplyTagRules 互斥/覆盖规则作用于**合并结果**（跨源冲突不再并存：
// 直采 hdr10+推断 hdr10_plus → hdr10_plus；直采 完结+推断 分集 → 完结）
func MergeTags(existing, inferred []string) []string {
	norm := make([]string, 0, len(existing)+len(inferred))
	for _, t := range existing {
		if n := titleparser.NormalizeTagDisplay(t); n != "" {
			norm = append(norm, n)
		}
	}
	norm = append(norm, inferred...)
	return ApplyTagRules(norm)
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

	// 2. 互斥规则（仅保留首个命中的——§59.74 改输入序：合并路径直采在前=直采赢；
	// 推断-only 路径输入序=字典命中序，与原组序行为一致）
	for _, group := range tagMutexGroups {
		memberSet := make(map[string]bool, len(group))
		for _, member := range group {
			memberSet[member] = true
		}
		firstHit := ""
		for _, t := range deduped {
			if memberSet[t] {
				firstHit = t
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
