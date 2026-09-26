package titleparser

import (
	"regexp"
	"strings"
	"unicode"
)

// §59.290: 主标题 AKA 双名切分——站方发布者命名风格携带 "原名 AKA 英文名"
// （Debaucherous@家园/MNHD-FRDS 欧陆片）或 "英文名 AKA 拼音"（CMCT 系），
// v1.05 主标题须单名（243 "Chi l'ha vista morire? AKA Who Saw Her Die?" 案）。
// 处理层定案：解析层（MainTitle 提取后切分）——元数据保留 raw（采集事实，
// §59.284 raw-token 模型），重组产出规范单名。
//
// 选段策略（常见词计分 + 拼音形态强扣分）：
//   欧陆片 "ItalianName AKA The Hunt"——原名 0 分、英文名 the/hunt 命中 → 取后段
//   华语片 "The.Piano.in.a.Factory.AKA.Gang.de.qin"——拼音全小写强扣 → 取前段
//   双零分（Amaran aka SK21 / Andhra King Taluka aka RaPo 22）→ 平分取前段（正名）
var reAKASplit = regexp.MustCompile(`(?i)(?:\s|\.)\s*(?:AKA)\s*(?:\s|\.)`)

var reNonWord = regexp.MustCompile(`[^\p{L}\p{N}']+`)

// akaCommonWords 常见英文词表（计分用——覆盖功能词+高频实词即可，
// 追求区分度不追求覆盖度：原名/拼音段命中≈0，英文标题常命中 1-4 词）
var akaCommonWords = map[string]bool{
	"the": true, "of": true, "and": true, "in": true, "a": true, "to": true,
	"is": true, "it": true, "you": true, "that": true, "was": true, "for": true,
	"on": true, "are": true, "with": true, "as": true, "his": true, "they": true,
	"at": true, "be": true, "this": true, "have": true, "from": true, "or": true,
	"had": true, "by": true, "but": true, "not": true, "what": true, "all": true,
	"were": true, "when": true, "up": true, "out": true, "who": true, "her": true,
	"how": true, "an": true, "she": true, "their": true, "time": true, "if": true,
	"will": true, "way": true, "about": true, "many": true, "then": true, "them": true,
	"would": true, "like": true, "so": true, "these": true, "him": true, "two": true,
	"has": true, "more": true, "day": true, "could": true, "no": true, "my": true,
	"than": true, "first": true, "down": true, "been": true, "now": true, "find": true,
	"any": true, "new": true, "work": true, "part": true, "take": true, "get": true,
	"place": true, "made": true, "live": true, "where": true, "after": true, "back": true,
	"little": true, "only": true, "man": true, "year": true, "came": true, "show": true,
	"every": true, "good": true, "give": true, "our": true, "under": true, "name": true,
	"very": true, "through": true, "just": true, "form": true, "much": true, "great": true,
	"think": true, "say": true, "help": true, "line": true, "before": true, "turn": true,
	"same": true, "mean": true, "right": true, "boy": true, "old": true, "too": true,
	"does": true, "tell": true, "set": true, "three": true, "want": true, "air": true,
	"well": true, "also": true, "play": true, "small": true, "end": true, "put": true,
	"home": true, "read": true, "hand": true, "large": true, "even": true, "land": true,
	"here": true, "must": true, "high": true, "such": true, "follow": true, "act": true,
	"why": true, "ask": true, "men": true, "change": true, "went": true, "light": true,
	"kind": true, "off": true, "need": true, "house": true, "try": true, "us": true,
	"again": true, "point": true, "mother": true, "world": true, "near": true, "earth": true,
	"father": true, "head": true, "stand": true, "own": true, "page": true, "should": true,
	"country": true, "found": true, "answer": true, "school": true, "grow": true, "still": true,
	"learn": true, "cover": true, "food": true, "sun": true, "four": true, "state": true,
	"keep": true, "eye": true, "never": true, "last": true, "let": true, "city": true,
	"tree": true, "cross": true, "farm": true, "hard": true, "start": true, "might": true,
	"story": true, "saw": true, "far": true, "sea": true, "draw": true, "left": true,
	"late": true, "run": true, "close": true, "night": true, "real": true, "life": true,
	"few": true, "north": true, "open": true, "together": true, "next": true, "begin": true,
	"walk": true, "always": true, "music": true, "those": true, "both": true, "book": true,
	"letter": true, "until": true, "mile": true, "river": true, "car": true, "care": true,
	"second": true, "group": true, "carry": true, "took": true, "rain": true, "eat": true,
	"room": true, "friend": true, "began": true, "idea": true, "fish": true, "once": true,
	"born": true, "face": true, "order": true, "sure": true, "south": true, "rest": true,
	"fight": true, "pass": true, "girl": true, "door": true, "wait": true, "die": true,
	"death": true, "kill": true, "love": true, "war": true, "blood": true, "black": true,
	"white": true, "red": true, "blue": true, "dark": true, "stranger": true, "hunt": true,
	"piano": true, "factory": true, "princess": true, "hope": true, "risk": true, "moon": true,
	"tiger": true, "deer": true, "audition": true, "warrior": true, "fantasy": true, "king": true,
}

// akaSegmentScore 段计分：常见词 +1/词；全小写字母词段（拼音形态）强扣分
func akaSegmentScore(seg string) int {
	score := 0
	letterWords := 0
	lowerWords := 0
	for _, w := range strings.Fields(seg) {
		w = strings.Trim(w, ".,!?;:'\"()-[]")
		if w == "" || !hasLatinLetter(w) {
			continue
		}
		letterWords++
		if akaCommonWords[strings.ToLower(w)] {
			score++
		}
		if w == strings.ToLower(w) {
			lowerWords++
		}
	}
	if letterWords > 0 && lowerWords == letterWords {
		score -= letterWords // 全小写段=拼音/代号形态 → 强扣
	}
	return score
}

func hasLatinLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// SplitAKATitle 主标题 AKA 切分入口。无 AKA 原样返回；有则按计分选段
// （平分取前段=正名）。词边界匹配防误伤（需独立 AKA/aka 词——片名含
// aka 子串如 "Dakar" 不命中）。
func SplitAKATitle(title string) string {
	loc := reAKASplit.FindStringIndex(title)
	if loc == nil {
		return title
	}
	head := strings.TrimSpace(strings.Trim(title[:loc[0]], ". "))
	tail := strings.TrimSpace(strings.Trim(title[loc[1]:], ". "))
	if head == "" || tail == "" {
		return title
	}
	if akaSegmentScore(tail) > akaSegmentScore(head) {
		return tail
	}
	return head
}
