package util

import "strings"

// ContainsAny §59.551: 报告 s 是否包含 subs 中任意一个子串。
// 变参子串匹配形态——标准库无直接等价（区别于 slices.Contains 的相等语义）。
// 收敛自 titleparser/inference.go 与 publish/subtitle.go 的两份同实现私有副本。
func ContainsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
