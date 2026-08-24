package util

import (
	"strings"
)

// ExtractGroupName 从标题末尾提取制作组名。
//
// 支持三种分隔符（按优先级）：
//   - "-"  标准格式：Title.2024-CMRG → CMRG
//   - "@"  老种子格式：-uploader@GROUP → GROUP
//   - "￡"  SSD 特有格式：￡CMCT发布者 → CMCT（取连续英文）
//
// 所有分支统一剥离尾部中文（发布者名），自动忽略 NOGROUP/N/A/NONE/UNKNOWN，
// 自动去除文件扩展名。
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

	// §59.64: 克隆站等站点把促销状态渲染进种子标题尾部——
	// "…10bit-Yumi@FRDS\u00a0\u00a0\u00a0 [2X 50%]"（NBSP 分隔，243 The.Boys 实锤）。
	// NBSP(U+00A0) ≥128 会被 isValidGroupName 判非 ASCII 连坐整个候选（→ 空组名，
	// 编辑器报"缺失字段：release_group"）；普通空格变体则产出脏组名"BHD [Free]"
	//（release_group_mappings 必失配）。归一 NBSP + 循环剥离尾部 [..] 标记后提取。
	clean = strings.ReplaceAll(clean, "\u00a0", " ")
	for {
		trimmed := strings.TrimRight(clean, " ")
		if !strings.HasSuffix(trimmed, "]") {
			break
		}
		open := strings.LastIndex(trimmed, "[")
		if open <= 0 {
			break
		}
		clean = strings.TrimRight(trimmed[:open], " ")
	}

	// 尝试 "-" 分隔符
	lastDash := strings.LastIndex(clean, "-")
	if lastDash > 0 && lastDash < len(clean)-1 {
		group := strings.TrimSpace(clean[lastDash+1:])
		if atIdx := strings.LastIndex(group, "@"); atIdx >= 0 && atIdx < len(group)-1 {
			group = group[atIdx+1:]
		}
		group = stripTrailingNonASCII(strings.TrimSpace(group))
		upper := strings.ToUpper(group)
		ignore := map[string]bool{"NOGROUP": true, "N/A": true, "NONE": true, "UNKNOWN": true}
		if !ignore[upper] && len(group) >= 2 && len(group) <= 30 && isValidGroupName(group) {
			return group
		}
	}

	// 尝试 "￡" 分隔符（SSD 特有格式）
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

// stripTrailingNonASCII 剥离尾部的非 ASCII 字符（中文发布者名）。
// 例: "CMCT九洲客" → "CMCT", "CMCTV蒙太奇" → "CMCTV"
func stripTrailingNonASCII(s string) string {
	out := []rune(s)
	for len(out) > 0 {
		r := out[len(out)-1]
		if r >= 128 {
			out = out[:len(out)-1]
		} else {
			break
		}
	}
	return string(out)
}

// isValidGroupName 判定 "-" 分支提取的候选是否为合法制作组名。
// 含非 ASCII 字符（CJK/特殊符号）或点时返回 false——
// "2016.国语中字￡CMCT" 不是组名，应 fall through 到 ￡ 分支。
func isValidGroupName(s string) bool {
	for _, r := range s {
		if r >= 128 || r == '.' {
			return false
		}
	}
	return true
}
