package util

import "strings"

// ExtractGroupName 从标题末尾提取制作组名。
//
// 支持三种分隔符（按优先级）：
//   - "-"  标准格式：Title.2024-CMRG → CMRG
//   - "@"  老种子格式：-uploader@GROUP → GROUP
//   - "￡"  SSD 特有格式：￡CMCT发布者 → CMCT（取连续英文）
//
// 自动忽略 NOGROUP/N/A/NONE/UNKNOWN，自动去除文件扩展名。
func ExtractGroupName(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}

	clean := title
	for _, ext := range []string{".mkv", ".mp4", ".avi", ".ts", ".wmv", ".flv"} {
		if strings.HasSuffix(strings.ToLower(clean), ext) {
			clean = clean[:len(clean)-len(ext)]
			break
		}
	}

	lastDash := strings.LastIndex(clean, "-")
	if lastDash > 0 && lastDash < len(clean)-1 {
		group := strings.TrimSpace(clean[lastDash+1:])
		if atIdx := strings.LastIndex(group, "@"); atIdx >= 0 && atIdx < len(group)-1 {
			group = group[atIdx+1:]
		}
		group = strings.TrimSpace(group)
		upper := strings.ToUpper(group)
		ignore := map[string]bool{"NOGROUP": true, "N/A": true, "NONE": true, "UNKNOWN": true}
		if !ignore[upper] && len(group) >= 2 && len(group) <= 30 {
			return group
		}
	}

	if pIdx := strings.LastIndex(clean, "￡"); pIdx >= 0 && pIdx < len(clean)-len("￡") {
		rest := clean[pIdx+len("￡"):]
		var b strings.Builder
		for _, r := range rest {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				b.WriteRune(r)
			} else {
				break
			}
		}
		if b.Len() >= 2 {
			return b.String()
		}
	}

	return ""
}
