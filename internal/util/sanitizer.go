package util

import (
	"regexp"
	"strings"

	"go.uber.org/zap/zapcore"
)

var defaultSensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(passkey[=:]\s*)\S+`),
	regexp.MustCompile(`(?i)(cookie[=:]\s*)\S+`),
	regexp.MustCompile(`(?i)(api[_]?key[=:]\s*)\S+`),
	regexp.MustCompile(`(?i)(bearer_token[=:]\s*)\S+`),
	regexp.MustCompile(`(?i)(password[=:]\s*)\S+`),
	regexp.MustCompile(`(?i)(encryption_key[=:]\s*)\S+`),
	regexp.MustCompile(`(?i)(rss[_]?key[=:]\s*)\S+`),
	regexp.MustCompile(`(?i)(auth[_]?key[=:]\s*)\S+`),
	regexp.MustCompile(`(?i)(secret[=:]\s*)\S+`),
	regexp.MustCompile(`(?i)(token[=:]\s*)\S+`),
}

type SanitizerCore struct {
	zapcore.Core
	patterns []*regexp.Regexp
}

func NewSanitizerCore(core zapcore.Core) *SanitizerCore {
	return &SanitizerCore{
		Core:     core,
		patterns: defaultSensitivePatterns,
	}
}

func (s *SanitizerCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	for i := range fields {
		if fields[i].Type == zapcore.StringType {
			for _, p := range s.patterns {
				if p.MatchString(fields[i].String) {
					fields[i].String = p.ReplaceAllString(fields[i].String, "${1}***")
					break
				}
			}
		}
	}
	return s.Core.Write(entry, fields)
}

func (s *SanitizerCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if s.Enabled(entry.Level) {
		return ce.AddCore(entry, s)
	}
	return ce
}

// siteOpMarkerTailRe §59.99/§59.136: 站点运营标记（副标题/标题尾部）——
// 朋友站 "[中性种子(NL)]"（零魔力, NBSP 前缀）、百分比/返利族 "[50%]" "[30%]" "[75%]" "[2X 50%]"、
// 免费/促销族。非内容数据, 转发引用无意义, 采集层剥除。
// 混排保留组: 尾部 [禁转] 族（合规判据, extractFlags 消费）不剥——"[50%] [禁转]" 只剥 [50%]。
// §59.284: 保留组移除——禁转判定收归 flags 通道（adapter raw 提取），标题净化不再承载
// 可见性；禁转词族入词表直剥（封闭稳定集——四词数十年不变，非促销量词开放变体域）
var siteOpMarkerTailRe = regexp.MustCompile(`(?i)([\s\x{00a0}]*\[(?:中性种子|免费|free|2x|促销|普通|hot|经典|限免|密封|seeding|leeching|禁转|谢绝转载|严禁转载|禁止转载|\d+(?:[Xx×]\s*\d+)?\s*%)[^\]]*\])+\s*$`)

// StripSiteOperationMarkers 剥除标题/副标题尾部的站点运营标记（§59.64 副标题侧同族）。
// §59.136 提升公共方法至 util（metadata 采集层与 db migration 清存量同一实现——
// metadata→reseed→db 依赖环, 不能放 metadata）。迭代处理多标记混排。
// siteStateTailRe §59.248 族二: 圆括号站方状态标注（标题尾部）——织梦系
// "(已审)" 等（fnos 378 行实证）。判据：括号内首字 ∈ {已,待,未}（审核
// 状态语义锚）——内容性括号（"(港)" 地区译名标注）不剥。
var siteStateTailRe = regexp.MustCompile(`(?i)([\s\x{00a0}]*\((?:已|待|未|中性种子|免费|free|2x|促销|普通|hot|经典|限免|密封|seeding|leeching|\d+(?:[Xx×]\s*\d+)?\s*%)[^)]*\))+\s*$`)

// siteTimeTailRe §59.248 族三: 裸文本时间尾段——NexusPHP 免费倒计时标注
// （"[免费] 剩余时间：1天5时" 复合尾段的裸文本半段，fnos 5 行残留实证）。
// 判据：前缀词强锚（剩余时间/优惠剩余/免费剩余）+时间量词串（数字/天时分秒/冒号）。
// 前缀词锚保证不误伤片名（片名不以该三词+冒号+纯时间量构成尾段）。
var siteTimeTailRe = regexp.MustCompile(`(?i)([\s\x{00a0}]*(?:剩余时间|优惠剩余|免费剩余)[：:\s]*[0-9０-９月天日时小分秒钟:：/]+[\s\x{00a0}]*)+$`)

var tailAnnotSegRe = regexp.MustCompile(`(?:[\s\x{00a0}]*(?:[\[（(【][^\]\)）】]*[\]\)）】]|(?:剩余时间|优惠剩余|免费剩余)[：:\s]*[0-9０-９月天日时小分秒钟:：/]+))+$`)


// stripGroupAnnotatedTail §59.248 族四: 组名锚尾段剥除（用户定案——官组名后
// 站方标注内容均应剥除）。两遍法：①剥尾部纯标注段（形态判据——任意括号段/
// 时间尾，不查词表=新造词免疫）得 clean2；②clean2 尾部恰为组名（ExtractGroupName
// 严格三层识别+HasSuffix 验证）→ 标题净化为 clean2（组名即本体终点）。
// clean2 尾非组名（组名后跟内容：官方版本/续作名等）→ 不剥（内容保护）。
func stripGroupAnnotatedTail(title string) string {
	removed := tailAnnotSegRe.FindString(title)
	if removed == "" {
		return title
	}
	// §59.284: 禁转守卫移除——flags 通道单责（§59.136 折中由架构模型取代）
	clean2 := strings.TrimSpace(strings.TrimSuffix(title, removed))
	if clean2 == "" {
		return title
	}
	if g := ExtractGroupName(clean2); g != "" && strings.HasSuffix(clean2, g) {
		return clean2
	}
	return title
}

// groupAnchorRe §59.284 族五: 组名锚 token（PT 圈共识：组名必在种子名末位，
// 无组名标 -NOGROUP 同末位——v1.05 Order 终字段。用户领域定案）
var groupAnchorRe = regexp.MustCompile(`-([A-Za-z0-9@_.]{2,})`)

var bracketContentRe = regexp.MustCompile(`\[[^\]]*\]|\([^)]*\)|【[^】]*】|（[^）]*）`)

// tailStartRe §59.284 族五触发形态：空格分隔 + 括号开头（标注挂载形态）
var tailStartRe = regexp.MustCompile(`^[\s\x{00a0}]+[\[（(【]`)
var latinWordRe = regexp.MustCompile(`[A-Za-z]{3,}`)

// stripTailAfterGroupAnchor §59.284 族五: 锚后全剥兜底——最后一个含字母的
// -组名 token 之后的全部内容，若无非括号标题词（3+ 连续英文字母）则剥除。
// 形态判据反转（"标题词长什么样" vs "标注长什么样"）——站方新造任何标注
// 词免疫（§59.262 月/§59.283 钟两案根治性取代）。保险丝：括号外 3+ 字母
// = 标题内容不剥（100+ 站数据质量长尾防灾难截断）。
func stripTailAfterGroupAnchor(s string) string {
	locs := groupAnchorRe.FindAllStringSubmatchIndex(s, -1)
	for i := len(locs) - 1; i >= 0; i-- {
		tok := s[locs[i][2]:locs[i][3]]
		hasLetter := false
		for _, r := range tok {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				hasLetter = true
				break
			}
		}
		if !hasLetter {
			continue // 纯数字/符号（续集 "-2"）不锚
		}
		tail := s[locs[i][3]:]
		if strings.TrimSpace(tail) == "" {
			return s // 组名即末尾——无事可做
		}
		// 触发形态收紧（§59.284 复盘补）：仅"空格分隔+括号开头"的尾段
		// （UBWEB "[免费]…"/PT地带 "[50%]…" 两实案形态）——dot 连体
		// （"-CMCT.官方版本"）是标题字段延续不碰（族四时代内容保护案回归）
		if !tailStartRe.MatchString(tail) {
			return s
		}
		if latinWordRe.MatchString(bracketContentRe.ReplaceAllString(tail, "")) {
			return s // 标题词保险丝（括号外 3+ 字母）
		}
		return strings.TrimSpace(s[:locs[i][3]])
	}
	return s
}

// StripSiteOperationMarkers 剥除标题/副标题尾部一切【与标题无关的站方标注】
// （§59.248 泛化为标题净化公共方法）——四族：族一方括号运营标记（§59.99/§59.136
// 词表）+族二圆括号站方状态（词表）+族三裸文本时间尾段+族四组名锚尾段剥除
// （形态免疫收官）。迭代处理多标记/跨族混排。存量清除重获自愈（无需 migration）。
func StripSiteOperationMarkers(s string) string {
	s = strings.TrimSpace(s)
	for {
		// 族一（运营标记——§59.284 保留组移除，全剥）
		ns := strings.TrimSpace(siteOpMarkerTailRe.ReplaceAllString(s, ""))
		// 族二（站方状态——无保留组直接剥）
		ns = strings.TrimSpace(siteStateTailRe.ReplaceAllString(ns, ""))
		// 族三（裸文本时间尾段）
		ns = strings.TrimSpace(siteTimeTailRe.ReplaceAllString(ns, ""))
		// 族四（组名锚——纯标注序列整体剥）
		ns = strings.TrimSpace(stripGroupAnnotatedTail(ns))
		// 族五（§59.284 锚后全剥兜底——新造标注词免疫的根治层）
		ns = strings.TrimSpace(stripTailAfterGroupAnchor(ns))
		if ns == s {
			return ns
		}
		s = ns
	}
}
