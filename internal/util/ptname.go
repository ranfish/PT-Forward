package util

import (
	"regexp"
	"sync/atomic"
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
// groupLexicon §59.233: 组名映射资产扫描注册表（ExtractGroupName v2 第一层）。
// 词条源=release_group_mappings（sites.json groups seed + UI 词条）——
// 启动加载注入 + RefreshCache 联动重建（handleCreateGroupMapping 挂钩）。
// atomic.Pointer 编译缓存（§59.186 懒编译并发首写 fatal 教训——重建全量替换）。
var groupLexicon atomic.Pointer[regexp.Regexp]

// SetGroupLexicon §59.233: 注入组名词表（重建 alternation 编译正则——
// 词长降序防短词先中；(?i) 扫描返回标题原词）。
func SetGroupLexicon(words []string) {
	filtered := make([]string, 0, len(words))
	for _, w := range words {
		w = strings.TrimSpace(w)
		if len(w) >= 2 {
			filtered = append(filtered, regexp.QuoteMeta(w))
		}
	}
	if len(filtered) == 0 {
		groupLexicon.Store(nil)
		return
	}
	// 词长降序（长词优先防子串抢占）
	for i := 0; i < len(filtered); i++ {
		for j := i + 1; j < len(filtered); j++ {
			if len(filtered[j]) > len(filtered[i]) {
				filtered[i], filtered[j] = filtered[j], filtered[i]
			}
		}
	}
	re, err := regexp.Compile(`(?i)(?:^|[\s._\-￡])(` + strings.Join(filtered, "|") + `)\b`)
	if err != nil {
		return
	}
	groupLexicon.Store(re)
}

// reTailSuffix §59.233: 尾部约束——命中词右侧仅剩分隔/空白/括号尾缀
// （"(已审)"/"[Free]" 类；V1.05 组名仅出现在尾部——非尾部排除，§59.233 ②定案）。
var reTailSuffix = regexp.MustCompile(`^[\s._\-]*(\([^)]*\)|\[[^\]]*\])?[\s._\-]*$`)

// matchGroupLexicon §59.233: 资产扫描——尾部约束内命中返回标题原词。
func matchGroupLexicon(clean string) string {
	re := groupLexicon.Load()
	if re == nil {
		return ""
	}
	for _, loc := range re.FindAllStringSubmatchIndex(clean, -1) {
		if loc[2] < 0 || loc[3] < 0 {
			continue
		}
		tail := clean[loc[3]:]
		if reTailSuffix.MatchString(tail) {
			return clean[loc[2]:loc[3]] // 标题原词（§59.233 ③定案）
		}
	}
	return ""
}

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
	// （release_group_mappings 必失配）。归一 NBSP + 循环剥离尾部 [..] 标记后提取。
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

	// §59.233 第一层：映射资产扫描（尾部约束内命中返回标题原词）。
	// @ 形态标题跳过（既有 @ 后段=官方组规则优先——§59.211；@ 是最强
	// 无歧义锚不依赖词表，DIY@ 通用前缀由后段规则天然剥离）。
	if !strings.Contains(clean, "@") {
		if g := matchGroupLexicon(clean); g != "" {
			return g
		}
	}

	// 尝试 "-" 分隔符
	lastDash := strings.LastIndex(clean, "-")
	// §59.193: 词内连字符（媒介/音频复合词 Blu-ray/WEB-DL/DTS-HD）非组分隔——
	// "…Live Aid 4K Blu-ray" 曾误提取组名 "ray"（Queen 案）。取连字符前词段
	// （至空格/点），为已知媒介片段时该连字符不构成分隔。
	// §59.196 修订: 需双侧证据——前段为媒介/音频片段 AND 后段为已知复合续段
	// （ray/dl/hd/ma/x 等）。仅前段命中曾误杀 "DTS-CMCT" 形态（DTS 片段为
	// DTS-HD 而设，后随组名时实为音频+组分隔——永安镇故事集案：组名提取
	// 失败→恢复链源站优先阶段整段跳过→未能从不可说恢复）。
	if lastDash > 0 {
		fragStart := strings.LastIndexAny(clean[:lastDash], " ._")
		frag := strings.ToLower(clean[fragStart+1 : lastDash])
		afterTok := clean[lastDash+1:]
		if sp := strings.IndexAny(afterTok, " ._"); sp >= 0 {
			afterTok = afterTok[:sp]
		}
		afterLower := strings.ToLower(afterTok)
		switch frag {
		case "blu", "web", "dts", "dts-hd", "true", "hd":
			switch afterLower {
			case "ray", "dl", "rip", "hd", "ma", "x", "hra", "dvd":
				lastDash = -1
			}
		}
	}
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
		// §59.211 终案（用户方案 B）: @ 后段=官方组——与 dash 规则语义统一
		// （"-JK@UBits"→"UBits" §59.184 附二先例）。"￡cXcY@FRDS"→"FRDS"
		// （子组前缀无判别价值：同官方组不同子组=同族；完整署名由 titleparser
		// 保留供 Tab1 展示/重组）。此前遇 @ 截断取 "cXcY" 致映射永久 miss。
		if at := strings.LastIndex(rest, "@"); at >= 0 && at < len(rest)-1 {
			rest = rest[at+1:]
		}
		var b strings.Builder
		for _, r := range rest {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
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
