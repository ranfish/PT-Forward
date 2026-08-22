// Package model screenshots 列解析与写入（§59.47 / §59.59 附二）。
//
// torrent_metadata.screenshots 列存在两种历史格式：
//   - JSON 数组字符串（现行权威格式，fetcher.buildMetadata json.Marshal）
//   - 换行分隔（老 rss_detail 约定，理论存在的历史行）
//
// 读取方（detail API / refresh / 转存链）必须经 ParseScreenshotColumn 统一解析，
// 禁止直接 strings.Split——写读格式分裂曾致所有截图显示为 1 张损坏图（§59.47）。
// 写入方必须经 FormatScreenshotColumn / NormalizeScreenshotColumn——禁止 Join("\n")
// 或裸透传（§59.59 附二第五处残留：handlePutSeed/persistAnalysis 换行写回，
// e96ec7d0e1 实锤；读侧兼容掩盖了分裂，列格式卫生靠单点机制不靠记忆）。
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

// FormatScreenshotColumn URL 列表 → 列值（JSON 数组字符串，权威格式）。
// 空列表写 "[]"（与空串读侧语义等同，但保持列格式统一）。
func FormatScreenshotColumn(urls []string) string {
	if len(urls) == 0 {
		return "[]" // json.Marshal(nil) 产出 "null"——§59.57 同款坑，显式拦截
	}
	data, err := json.Marshal(urls)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// NormalizeScreenshotColumn 透传写点归一：任意历史格式（换行/裸 URL/JSON）→ 权威 JSON。
// JSON 输入幂等。用于前端原样回传或手动输入的写路径。
func NormalizeScreenshotColumn(raw string) string {
	return FormatScreenshotColumn(ParseScreenshotColumn(raw))
}
