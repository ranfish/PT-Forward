package ptgen

// englishname.go §59.254 项2: 英文规范名兜底链——aka 启发式（端点感知）+
// 罗马数字归一判别 + &imdb 二连（仅兜底场景）。
//
// 架构（docs/31 §59.254 四/六节定案）：
//   常规    ：aka 启发式尽力而为（同次豆瓣响应内——零额外请求）
//   兜底触发：main_title 空/纯 CJK 且 aka 无英文段 → &imdb 二连（+1 请求）
//             → IMDb 源 original_title 权威值（含 type 分类白拿）
//   端点感知：豆影（aka 原名保序——用户规则：首段英文段→无则首段拉丁段）
//             财神（foreign_title ASCII 判定先行→aka 找英文段——aka 与豆影不同源
//             不同序：滤原名+英文常前置；无 IMDb 通道）
//   判别    ：英文段=ASCII 拉丁 ∪ 罗马数字字符（Ⅱ Ⅲ Ⅳ…归一为合法英文段——
//             东北恋歌 Northeastern Bro Ⅲ 案）；重音字符（Â é ü）不归一=
//             原语种标志（L'Âge d'or 案→正确落 The Golden Age）

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"github.com/ranfish/pt-forward/internal/httpclient"
	"github.com/ranfish/pt-forward/internal/model"
)

// romanToASCII 罗马数字字符归一为 ASCII 字母串（Ⅲ→III）
var romanToASCII = map[rune]string{
	'Ⅰ': "I", 'Ⅱ': "II", 'Ⅲ': "III", 'Ⅳ': "IV", 'Ⅴ': "V",
	'Ⅵ': "VI", 'Ⅶ': "VII", 'Ⅷ': "VIII", 'Ⅸ': "IX", 'Ⅹ': "X",
	'Ⅺ': "XI", 'Ⅻ': "XII", 'Ⅼ': "L", 'Ⅽ': "C", 'Ⅾ': "D", 'Ⅿ': "M",
	'ⅰ': "i", 'ⅱ': "ii", 'ⅲ': "iii", 'ⅳ': "iv", 'ⅴ': "v",
	'ⅵ': "vi", 'ⅶ': "vii", 'ⅷ': "viii", 'ⅸ': "ix", 'ⅹ': "x",
}

// IsEnglishSegment 判定段是否为"英文段"：ASCII 拉丁字母/数字/常见标点/NBSP +
// 罗马数字字符（归一合法）。重音字符（Latin-1 扩展 Â é ü 等）与 CJK/西里尔=非英文。
func IsEnglishSegment(seg string) bool {
	seg = strings.ReplaceAll(seg, "\u00a0", " ")
	if seg == "" {
		return false
	}
	for _, r := range seg {
		if r == '’' || r == '‘' {
			continue // 弯引号（There’s Still Tomorrow 案——U+2019≥0x80 但属英文标点）
		}
		if r < 0x80 {
			// ASCII：字母/数字/空格/常见标点（' - . : ! ? & / ()）合法
			switch {
			case unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r):
			case r == '\'' || r == '-' || r == '.' || r == ':' || r == '!' || r == '?' || r == '&' || r == '/' || r == '(' || r == ')' || r == ',' || r == ';':
			default:
				return false
			}
			continue
		}
		if _, ok := romanToASCII[r]; ok {
			continue // 罗马数字归一合法
		}
		return false // 重音/CJK/其它=非英文
	}
	return true
}

// IsLatinSegment 判定段是否为"拉丁段"（兜底：含重音的原语种也算——意大利语原名等）
func IsLatinSegment(seg string) bool {
	seg = strings.ReplaceAll(seg, "\u00a0", " ")
	if seg == "" {
		return false
	}
	hasLetter := false
	for _, r := range seg {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			return false // CJK 排除
		}
		if unicode.IsLetter(r) {
			hasLetter = true
		}
	}
	return hasLetter
}

// NormalizeEnglishName 归一化段：NBSP→空格 + 罗马数字→ASCII + 全角→半角标点。
// §59.258 项4 用户定案：英文名标点及短横杠保留，统一归一为半角
// （：→: －/–/—→- （）→() 全角空格→空格——PTGen 数据全角/en-dash 形态实证）。
func NormalizeEnglishName(seg string) string {
	seg = strings.ReplaceAll(seg, "\u00a0", " ")
	var b strings.Builder
	for _, r := range seg {
		if repl, ok := romanToASCII[r]; ok {
			b.WriteString(repl)
			continue
		}
		if half, ok := fullWidthToHalf[r]; ok {
			b.WriteRune(half)
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// fullWidthToHalf 全角/Unicode 变体 → 半角标点（英文名保留标点定案）
var fullWidthToHalf = map[rune]rune{
	'：': ':',
	'－': '-', '–': '-', '—': '-',
	'（': '(', '）': ')',
	'　': ' ',
	'，': ',',
	'；': ';',
	'！': '!', '？': '?',
	'．': '.',
	'“': '"', '”': '"', '‘': '\'', '\'': '\'',
}

// ExtractEnglishFromSegments 用户规则：首段英文段→无则首段拉丁段。
// segs: 待选段列表（已分段——豆影 aka 数组或 translated_titles 斜杠拆分）。
// 返回归一化后的段（NBSP 清理+罗马数字转 ASCII）；无候选返回空。
func ExtractEnglishFromSegments(segs []string) string {
	firstLatin := ""
	for _, seg := range segs {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		if IsEnglishSegment(seg) {
			return NormalizeEnglishName(seg)
		}
		if firstLatin == "" && IsLatinSegment(seg) {
			firstLatin = seg // 拉丁兜底候选（原名语种）
		}
	}
	if firstLatin != "" {
		return NormalizeEnglishName(firstLatin)
	}
	return ""
}

// SplitTitleSegments 斜杠串拆分（存量 translated_titles 兼容——" / " 分隔）
func SplitTitleSegments(s string) []string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	parts := strings.Split(s, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// EnglishNameCandidates 端点感知的英文名候选段提取。
// 豆影豆瓣源：aka 数组（原名保序）+ translated_titles 存量兼容；
// 财神豆瓣源：foreign_title 若为英文段优先 + aka 数组（不同源不同序——英文常前置）。
// 返回候选段列表（顺序即优先序），交 ExtractEnglishFromSegments 统一规则判定。
func EnglishNameCandidates(r *model.PTGenResult, sourceEndpoint string) []string {
	var segs []string
	if isCsptEndpoint(sourceEndpoint) {
		// 财神：foreign_title 原名位（英文片时=英文）优先
		if r.ForeignTitle != "" && IsEnglishSegment(r.ForeignTitle) {
			segs = append(segs, r.ForeignTitle)
		}
	}
	// aka 数组（双端点共有）
	if len(r.AKA) > 0 {
		segs = append(segs, r.AKA...)
	}
	// 豆影存量 translated_titles（斜杠串）
	if r.TranslatedTitles != "" {
		segs = append(segs, SplitTitleSegments(r.TranslatedTitles)...)
	}
	return segs
}

// ExtractEnglishName 端点感知英文名提取（零额外请求——响应内解析）
func ExtractEnglishName(r *model.PTGenResult, sourceEndpoint string) string {
	return ExtractEnglishFromSegments(EnglishNameCandidates(r, sourceEndpoint))
}

func isCsptEndpoint(endpoint string) bool {
	return strings.Contains(endpoint, "cspt.top")
}

// NeedsIMDbFallback 兜底触发判定：main_title 缺损（空/纯 CJK 无拉丁段）且
// 常规英文名提取无果 → 需 &imdb 二连（豆影独有——仅兜底场景+1 请求）。
func NeedsIMDbFallback(mainTitle string, englishName string) bool {
	if englishName != "" {
		return false // 常规通道已解决
	}
	if mainTitle == "" {
		return true
	}
	hasLatin := false
	for _, r := range mainTitle {
		if r >= 0x80 {
			continue
		}
		if unicode.IsLetter(r) {
			hasLatin = true
			break
		}
	}
	return !hasLatin // 纯 CJK（无拉丁段）
}

// QueryIMDbForEnglish §59.254 项2: &imdb 二连（仅豆影端点）——豆瓣条目强制
// 返回 IMDb 源。返回 original_title（归一化）与 type（movie/tv 分类权威）。
// 财神端点无此通道（返回空）。
func (p *Provider) QueryIMDbForEnglish(ctx context.Context, doubanURL string) (englishName, kindType string) {
	if p == nil || doubanURL == "" || p.client == nil {
		return "", ""
	}
	endpoints := p.endpoints
	if len(endpoints) == 0 {
		endpoints = defaultPTGenEndpoints
	}
	for _, endpointLine := range endpoints {
		endpoint, key := endpointLine, p.apiKey
		if idx := strings.Index(endpointLine, "|"); idx >= 0 {
			endpoint = strings.TrimSpace(endpointLine[:idx])
			key = strings.TrimSpace(endpointLine[idx+1:])
		}
		if endpoint == "" || isCsptEndpoint(endpoint) {
			continue // 财神无 IMDb 通道
		}
		// 豆影：url=<douban>&imdb 强制
		sep := "?"
		if strings.Contains(endpoint, "?") {
			sep = "&"
		}
		reqURL := endpoint + sep + "url=" + url.QueryEscape(doubanURL) + "&imdb"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			continue
		}
		if key != "" {
			req.Header.Set("X-API-KEY", key)
		}
		resp, err := p.client.Do(req)
		if err != nil {
			continue
		}
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		httpclient.DrainBody(resp)
		if resp.StatusCode != http.StatusOK {
			continue
		}
		var raw map[string]any
		if json.Unmarshal(respBody, &raw) != nil {
			continue
		}
		if ot, _ := raw["original_title"].(string); ot != "" {
			ot = NormalizeEnglishName(ot)
			if IsEnglishSegment(ot) {
				t, _ := raw["type"].(string)
				return ot, t
			}
		}
	}
	return "", ""
}
