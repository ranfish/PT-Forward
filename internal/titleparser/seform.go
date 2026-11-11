package titleparser

import "regexp"

// §59.514 刀1: S/E 形态语义内核——A 链（internal/classify 孤儿目录分类）
// 与 B 链（发布链 SeasonEpisode）共用的单点判定。两套平行体系归一：
// 原 util.ClassifyTorrent 五正则（reSxxExx/reEPRange/reChineseAll/
// reChineseRange/reChineseSingle）与 titleparser 季集锚各自演化的分叉
// 终结（裸 E01 盲区两套共享实证）。
//
// 语义边界（讨论定案）：纯形态判定（信号无关）——完结信号归一
// （tags/副标/文件树 → S01）是发布链 NormalizeSeasonPack 的消费语义，
// 本内核只回答「token 长什么样、形态是什么」。v1.05 权威（qingwapt
// W 章）：单季合集=S01（即使只有 1 季也必须写）/多季=S01-S05/
// 分集=S01E03 或 S01E03-E05。
type SEForm struct {
	SeasonEpisode string // 原始 token（S01 / S01E03 / S01E01-E12 / E01 / E01-E06 / S01-S02）
	Season        string // 季号（跨季=S01-S02；无季信号空）
	FirstEp       string // 首集号（"03"；可空）
	LastEp        string // 区间尾集号（"12"；可空）
	Form          string // FormSeasonPack / FormPartialPack / FormSingleEpisode / FormCrossSeason / FormUnknown
}

// Form 枚举（A/B 链共用词表——A 链 TorrentType.Form 值域对齐）。
const (
	FormSeasonPack    = "season_pack"     // 单季合集（S01 / E01 起区间）
	FormPartialPack   = "partial_pack"    // 部分合集（E05 起区间）
	FormSingleEpisode = "single_episode"  // 单集（S01E03 / 裸 E03）
	FormCrossSeason   = "cross_season"    // 跨季合集（S01-S02）
	FormUnknown       = "unknown"         // 无 S/E 信号
)

var (
	reSESeasonOnly = regexp.MustCompile(`(?i)^(S\d{1,2})$`)
	reSECross      = regexp.MustCompile(`(?i)^(S\d{1,2})[-+](S\d{1,2})$`)
	reSESingle     = regexp.MustCompile(`(?i)^(?:S\d{1,2})?E(\d{1,4})$`)
	reSERange      = regexp.MustCompile(`(?i)^(?:S\d{1,2})?E(\d{1,4})-S?\d*E?(\d{1,4})$`)
	// reSETitleEPRange §59.187 ② A 链语义保留（归一迁移）：仅季号+标题
	// 残留 EP/纯数字区间（"S01.EP01-E06" 分段形态——季锚要求 E 紧邻 S，
	// 非紧邻区间留在 remaining；Obi-Wan tid=67993 案）。
	reSETitleEPRange = regexp.MustCompile(`(?i)\bE?P?(\d{1,2})\s*[-~]\s*E?P?(\d{1,3})\b`)
	// reSEAdjacentEp §59.514 A 链语义保留：空格分隔季集（"S01 E03"——
	// 季锚只吃 S01，E03 留 remaining 首位；原 A 链 reSxxExx 允许 \s*E）。
	reSEAdjacentEp = regexp.MustCompile(`(?i)^[\s.\-_]*E(\d{1,3})(?:[\s.\-_]|$)`)
	// reSECNCountEnd §59.514 A 链语义保留：「N集全」形态（reChineseAllEnd
	// ——B 链锚只认 全N集/第N-M集，此形态 se 空需回查标题）。
	reSECNCountEnd = regexp.MustCompile(`(\d{1,4})集全`)
)

// ParseSEForm 从标题提取季集形态（纯函数·无状态）。内部走
// extractBoundaryAnchor 单点提取（含裸 Exx/中文集数归一通道），
// 判定 v1.05 形态枚举。无 S/E 信号返回 FormUnknown（电影/动漫电影
// 天然无信号——消费链按自身语义处置）。
func ParseSEForm(title string) SEForm {
	se, _, remaining, _ := extractBoundaryAnchor(title)
	if se == "" {
		// §59.514 A 链语义保留：「N集全」形态（B 链锚不认——se 空回查）
		if reSECNCountEnd.MatchString(title) {
			return SEForm{Form: FormSeasonPack}
		}
		return SEForm{Form: FormUnknown}
	}
	f := ClassifySEForm(se)
	// §59.187 ② A 链语义保留（仅季号形态）：残留 EP 区间/空格分隔集
	if f.Form == FormSeasonPack && f.Season == se && reSESeasonOnly.MatchString(se) {
		if m := reSETitleEPRange.FindStringSubmatch(remaining); m != nil {
			f.FirstEp, f.LastEp = m[1], m[2]
			if m[1] == "1" || padTwo(m[1]) == "01" {
				f.Form = FormSeasonPack // E01 起=完整季
			} else {
				f.Form = FormPartialPack
			}
		} else if m := reSEAdjacentEp.FindStringSubmatch(remaining); m != nil {
			f.FirstEp = m[1]
			f.Form = FormSingleEpisode
		}
	}
	return f
}

// ClassifySEForm 季集 token → v1.05 形态（纯形态判定·无标题语境）。
// 仲裁层消费（tp.SeasonEpisode 已是提取终态 token——techFollow 语境
// 判定不适用单 token 输入）。
func ClassifySEForm(se string) SEForm {
	if se == "" {
		return SEForm{Form: FormUnknown}
	}
	f := SEForm{SeasonEpisode: se}
	switch {
	case reSECross.MatchString(se):
		m := reSECross.FindStringSubmatch(se)
		f.Season = m[1] + "-" + m[2]
		f.Form = FormCrossSeason
	case reSESeasonOnly.MatchString(se):
		f.Season = se
		// v1.05: 单季合集必须写 S01（标题声明语义；A 链原 conservative
		// unknown 判定退役——文件树修正层 refineFormFromFileTree 兜底半季）
		f.Form = FormSeasonPack
	case reSERange.MatchString(se):
		m := reSERange.FindStringSubmatch(se)
		f.FirstEp, f.LastEp = m[1], m[2]
		if m[1] == "1" || padTwo(m[1]) == "01" {
			f.Form = FormSeasonPack // E01 起=完整季形态（信号归一前置判定）
		} else {
			f.Form = FormPartialPack
		}
	case reSESingle.MatchString(se):
		m := reSESingle.FindStringSubmatch(se)
		f.FirstEp = m[1]
		f.Form = FormSingleEpisode
	default:
		f.Form = FormUnknown
	}
	return f
}

// padTwo "1"→"01"（集号形态对齐比较）。
func padTwo(s string) string {
	if len(s) == 1 {
		return "0" + s
	}
	return s
}
