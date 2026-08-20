// Package model screenshots 列解析（§59.47）。
//
// torrent_metadata.screenshots 列存在两种历史格式：
//   - JSON 数组字符串（现行写入格式，fetcher.buildMetadata json.Marshal）
//   - 换行分隔（老 rss_detail 约定，理论存在的历史行）
//
// 读取方（detail API / refresh / 转存链）必须经 ParseScreenshotColumn 统一解析，
// 禁止直接 strings.Split——写读格式分裂曾致所有截图显示为 1 张损坏图（§59.47）。
package model

import (
	"encoding/json"
	"strings"
)

// ParseScreenshotColumn 解析 screenshots 列 → URL 列表。
// JSON 数组优先；失败回退换行拆分（兼容历史格式）；空串返回空切片（非 nil 语义等同）。
func ParseScreenshotColumn(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "[]" {
		return nil
	}
	if strings.HasPrefix(s, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(s), &arr); err == nil && len(arr) > 0 {
			out := make([]string, 0, len(arr))
			for _, u := range arr {
				if u = strings.TrimSpace(u); u != "" {
					out = append(out, u)
				}
			}
			return out
		}
	}
	// 换行分隔回退（老格式兼容）
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}
