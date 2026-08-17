// Package titleparser 公共字典加载器（§59.35 Token Registry v2）。
//
// v1.05 技术词汇的单一事实来源：dict/*.json 分域数据文件（go:embed）。
// 消费方全部从本模块派生，禁止手写副本：
//   - 标题解析/重组：DictTokens(domain) → lookupToken/removeAllTokenPatterns（token_registry.go）
//   - DOM 值归一化（forward）：LookupDictKey(domain, raw)——变体 → standard key
//   - 显示名反查（reverse）：ReverseLookup(standardKey)——key → display
//   - 等价组（比较视图专用）：Equivalent(a, b)——重组/发布视图禁止使用
//
// 一致性护航（token_registry_test.go + dict_consistency_test.go）：
//   - round-trip：parse(reassemble(profile)) == profile
//   - 互斥性：同域词条 pattern 不得交叉命中
//   - 引用完整性：standard_key 全局唯一；equivalence_group 同组 ≥2 词条
package titleparser

import (
	"embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

//go:embed dict/*.json
var dictFS embed.FS

// TokenDef 单个技术词条定义（v2）。
type TokenDef struct {
	Category       string   `json:"-"`                          // 词条域（加载时填充）：video_codec/audio_codec/hdr/medium/resolution/type/source/platform
	Canonical      string   `json:"canonical"`                  // 内部规范值（解析产物/比较值）
	TitleForm      string   `json:"title_form,omitempty"`       // 重组标题点分隔形式，空 = Canonical
	Pattern        string   `json:"pattern,omitempty"`          // 标题变体匹配正则（空 = 不参与标题解析，仅 DOM 归一化）
	StandardKey    string   `json:"standard_key,omitempty"`     // 桥接 standard key（如 medium.webdl），全局唯一
	EquivGroup     string   `json:"equivalence_group,omitempty"` // 等价组名（如 hevc_impl = {x265, HEVC}），仅比较视图使用
	Display        string   `json:"display,omitempty"`          // 逆向显示名（空 = Canonical）；正逆向不对称时用（AVC→video.h264 但 video.h264→H.264）
	Requires       string   `json:"requires,omitempty"`         // 启用条件："web" = 仅 WEB 上下文（2 字符 platform 缩写专用，§59.35 决策 2）
	Variants       []string `json:"variants,omitempty"`         // DOM 显示名变体（forward map 数据源）
	FullName       string   `json:"full_name,omitempty"`        // 厂商全名（platform 域，展示/审计用）
	CaseSensitive  bool     `json:"case_sensitive,omitempty"`   // pattern 大小写敏感（iP/iT/Baha 等 wiki 明确写法）
	Origin         string   `json:"origin,omitempty"`           // 词条来源："wiki"（v1.05 附件）/"legacy"（本站历史行为保留）——空默认 wiki
	Note           string   `json:"note,omitempty"`             // 词条备注（legacy 词条说明保留原因）
}

// re 返回编译缓存的正则。CaseSensitive 词条不加 (?i)（iP/iT/Baha 等 wiki 明确写法，
// 大小写敏感避免 "IT"/"IP" 等普通词误命中）。
func (t TokenDef) re() *regexp.Regexp {
	if re, ok := tokenReC2[t.Pattern]; ok {
		return re
	}
	pattern := t.Pattern
	if !t.CaseSensitive {
		pattern = `(?i)` + pattern
	}
	re := regexp.MustCompile(pattern)
	tokenReC2[t.Pattern] = re
	return re
}

// matchesWithRequires 按 Requires 约束判定词条是否参与匹配。
// requires="web"：仅 WEB 上下文（标题含 WEB/HDTV token 或上下文未知时按 caller 传入的
// webContext 决定）——§59.35 决策 2：2 字符缩写误命中方向从"剥词污染标题"变为"不提取"。
func (t TokenDef) matchesWithRequires(s string, webContext bool) bool {
	if t.Pattern == "" {
		return false
	}
	if t.Requires == "web" && !webContext {
		return false
	}
	return t.re().MatchString(s)
}

// titleForm 返回重组形式（空则用 Canonical）。
func (t TokenDef) titleForm() string {
	if t.TitleForm != "" {
		return t.TitleForm
	}
	return t.Canonical
}

// display 返回逆向显示名（空则用 Canonical）。
func (t TokenDef) display() string {
	if t.Display != "" {
		return t.Display
	}
	return t.Canonical
}

// dictState 加载后的字典状态（不可变，进程生命周期复用）。
type dictState struct {
	tokens     map[string][]TokenDef       // domain → tokens
	forward    map[string]map[string]string // domain → variant（原样） → standard key
	forwardCI  map[string]map[string]string // domain → variant（lowercase） → standard key
	reverse    map[string]string            // standard key → display
	equivGroup map[string][]string          // group name → canonical 列表
}

var (
	dictOnce  sync.Once
	dict      *dictState
	dictErr   error
	tokenReC2 = map[string]*regexp.Regexp{} // pattern → compiled（t.re() 用）
)

// loadDict 加载 + 校验分域字典。校验失败 fail-fast（错误暴露给 ensureDict 调用方）。
func loadDict() (*dictState, error) {
	entries, err := dictFS.ReadDir("dict")
	if err != nil {
		return nil, fmt.Errorf("read dict dir: %w", err)
	}
	st := &dictState{
		tokens:     map[string][]TokenDef{},
		forward:    map[string]map[string]string{},
		forwardCI:  map[string]map[string]string{},
		reverse:    map[string]string{},
		equivGroup: map[string][]string{},
	}
	seenKey := map[string]string{} // standard_key → domain（全局唯一校验）
	seenCanonical := map[string]string{}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		domain := strings.TrimSuffix(e.Name(), ".json")
		raw, err := dictFS.ReadFile("dict/" + e.Name())
		if err != nil {
			return nil, fmt.Errorf("read dict/%s: %w", e.Name(), err)
		}
		var body struct {
			Tokens []TokenDef `json:"tokens"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			return nil, fmt.Errorf("parse dict/%s: %w", e.Name(), err)
		}
		if len(body.Tokens) == 0 {
			return nil, fmt.Errorf("dict/%s: empty tokens", e.Name())
		}

		for _, t := range body.Tokens {
			t.Category = domain
			if t.Canonical == "" {
				return nil, fmt.Errorf("dict/%s: token missing canonical", e.Name())
			}
			// canonical 同域内 pattern 不得重复（同 canonical 多词条允许——
			// 旧 registry 设计：DTS:X 归并词条与普通 DTS 词条同 canonical 不同优先级）
			ck := domain + "\x00" + t.Canonical + "\x00" + t.Pattern
			if _, dup := seenCanonical[ck]; dup {
				return nil, fmt.Errorf("dict/%s: duplicate canonical+pattern %q", e.Name(), t.Canonical)
			}
			seenCanonical[ck] = domain
			// pattern 编译冒烟（编译期安全由启动校验补位）
			if t.Pattern != "" {
				pattern := t.Pattern
				if !t.CaseSensitive {
					pattern = `(?i)` + pattern
				}
				if _, err := regexp.Compile(pattern); err != nil {
					return nil, fmt.Errorf("dict/%s: token %q pattern: %w", e.Name(), t.Canonical, err)
				}
			}
			// requires 值域（当前仅 "web"）
			if t.Requires != "" && t.Requires != "web" {
				return nil, fmt.Errorf("dict/%s: token %q unknown requires %q", e.Name(), t.Canonical, t.Requires)
			}
			// standard_key 全局唯一
			if t.StandardKey != "" {
				if dom, dup := seenKey[t.StandardKey]; dup {
					return nil, fmt.Errorf("dict/%s: standard_key %q duplicated (also in %s)", e.Name(), t.StandardKey, dom)
				}
				seenKey[t.StandardKey] = domain
				if _, dup := st.reverse[t.StandardKey]; dup {
					return nil, fmt.Errorf("dict/%s: reverse %q duplicated", e.Name(), t.StandardKey)
				}
				st.reverse[t.StandardKey] = t.display()
			}
			// forward map（variants，按域隔离——type 与 source 都有 "其他"）
			for _, v := range t.Variants {
				v = strings.TrimSpace(v)
				if v == "" {
					continue
				}
				if t.StandardKey == "" {
					return nil, fmt.Errorf("dict/%s: variant %q without standard_key", e.Name(), v)
				}
				dm, ok := st.forward[domain]
				if !ok {
					dm = map[string]string{}
					st.forward[domain] = dm
				}
				if _, dup := dm[v]; dup {
					return nil, fmt.Errorf("dict/%s: variant %q duplicated", e.Name(), v)
				}
				dm[v] = t.StandardKey
				ci, ok := st.forwardCI[domain]
				if !ok {
					ci = map[string]string{}
					st.forwardCI[domain] = ci
				}
				lv := strings.ToLower(v)
				if _, dup := ci[lv]; !dup {
					ci[lv] = t.StandardKey
				}
			}
			// 等价组收集
			if t.EquivGroup != "" {
				st.equivGroup[t.EquivGroup] = append(st.equivGroup[t.EquivGroup], t.Canonical)
			}
			st.tokens[domain] = append(st.tokens[domain], t)
		}
	}

	// §59.35 P2: platform 域确定性排序——canonical 长度降序（稳定排序，
	// 同长度保持文件次序）。长缩写优先是提取语义（HMAX 不得被 MAX 抢、
	// TVBAnywhere 不得被 TVB 抢），数据文件追加词条不再需要手工维护顺序。
	for domain := range st.tokens {
		if domain == "platform" {
			toks := st.tokens[domain]
			sort.SliceStable(toks, func(i, j int) bool {
				return len(toks[i].Canonical) > len(toks[j].Canonical)
			})
		}
	}

	// equivalence_group 引用完整性：同组 ≥2 词条
	for g, members := range st.equivGroup {
		if len(members) < 2 {
			return nil, fmt.Errorf("dict: equivalence_group %q has %d members (<2)", g, len(members))
		}
	}

	if len(st.tokens) == 0 {
		return nil, fmt.Errorf("dict: no domain files loaded")
	}
	return st, nil
}

// ensureDict 惰性加载（fail-fast：加载失败 panic，进程不应带坏字典运行）。
func ensureDict() *dictState {
	dictOnce.Do(func() {
		dict, dictErr = loadDict()
		if dictErr != nil {
			panic(fmt.Sprintf("titleparser dict load failed: %v", dictErr))
		}
	})
	return dict
}

// DictTokens 返回指定域的词条列表（顺序即标题提取优先级）。
// 未知域返回 nil。
func DictTokens(domain string) []TokenDef {
	return ensureDict().tokens[domain]
}

// DictDomains 返回已加载的域列表。
func DictDomains() []string {
	st := ensureDict()
	domains := make([]string, 0, len(st.tokens))
	for d := range st.tokens {
		domains = append(domains, d)
	}
	return domains
}

// LookupDictKey DOM 显示值 → standard key（forward，§59.35 派生自词条 Variants）。
//
// 匹配语义（与原 LookupStandardKey 一致）：
//  1. 精确匹配（原样）
//  2. 大小写不敏感精确
//  3. 包含匹配（最长变体优先，避免 "电视" 错配 "电视剧"）
func LookupDictKey(domain, raw string) string {
	st := ensureDict()
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	tokens := st.tokens[domain]
	if len(tokens) == 0 {
		return ""
	}
	// 1. 精确
	for _, t := range tokens {
		for _, v := range t.Variants {
			if v == raw && t.StandardKey != "" {
				return t.StandardKey
			}
		}
	}
	// 2. 大小写不敏感
	if key, ok := st.forwardCI[domain][strings.ToLower(raw)]; ok {
		return key
	}
	// 3. 包含（最长优先）
	best, bestKey := "", ""
	for _, t := range tokens {
		for _, v := range t.Variants {
			if t.StandardKey == "" || len(v) <= len(best) {
				continue
			}
			if strings.Contains(raw, v) {
				best, bestKey = v, t.StandardKey
			}
		}
	}
	return bestKey
}

// Equivalent 判断两个 canonical 值属于同一等价组（比较视图专用，§59.35）。
// 重组/发布视图禁止使用等价替换（x265 标题不能写 HEVC）。
func Equivalent(a, b string) bool {
	if a == b {
		return true
	}
	st := ensureDict()
	for _, members := range st.equivGroup {
		hasA, hasB := false, false
		for _, m := range members {
			if m == a {
				hasA = true
			}
			if m == b {
				hasB = true
			}
		}
		if hasA && hasB {
			return true
		}
	}
	return false
}

// EquivGroupMembers 返回等价组成员（调试/测试用）。
func EquivGroupMembers(group string) []string {
	return ensureDict().equivGroup[group]
}
