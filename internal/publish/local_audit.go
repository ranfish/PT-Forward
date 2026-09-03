package publish

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

// ═══ §59.166 LocalAudit 内部重组审核规范（advisory——仅提示不阻塞）═══
//
// 用户定案：站点适配阶段规则未经验证，发布后才知道规则是否正确——全 WARN 不拦。
// 规则源：幸运探针实证 + v1.05 规范 + 9 站种审脚本共性抽取（docs/38——
// Source 标注来源，同规则多站命中=共性，单站=站特例按 SiteOnly 过滤）。
//
// 校验分工（回归审核定案）：判据引擎产 tags（自动有据）、executor 硬拦行为
// 铁律（首发禁勾等）——LocalAudit 只做校验类（映射缺口/纠错透明/override 有据）。

// LocalAuditFinding 单条提示（全 WARN——message 含修法建议）。
type LocalAuditFinding struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Source  string `json:"source"` // 共性规则="common"；站特例=站名
}

// localAuditInput 引擎输入（executor ⑤ tags 段后的终态产物）。
type localAuditInput struct {
	TargetSite   string
	TP           titleparser.TechProfile
	Form         map[string]string
	Cfg          *model.PublishFormConfig
	AppliedTags  []string
	TagOverrides []string
	Meta         *model.TorrentMetadata
	PublishTitle string // §59.166 B2 重组终态标题
}

// RunLocalAudit 执行全部适用规则（共性全站 + 站特例按 TargetSite 过滤）。
func RunLocalAudit(in localAuditInput) []LocalAuditFinding {
	findings := make([]LocalAuditFinding, 0, 4)
	for _, r := range localAuditRules {
		if r.SiteOnly != "" && r.SiteOnly != in.TargetSite {
			continue
		}
		if msg := r.Check(in); msg != "" {
			findings = append(findings, LocalAuditFinding{Code: r.Code, Message: msg, Source: r.Source})
		}
	}
	return findings
}

type localAuditRule struct {
	Code     string
	Source   string // "common" | 站名
	SiteOnly string // 空=共性全站；站名=特例
	Check    func(in localAuditInput) string
}

// §59.166 R11 禁发组共性黑名单（AGSV/CS/春日/劳改所/青蛙五站名单交集——
// docs/38 共性矩阵；FRDS/BeiTai/GodDramas/VCB-Studio 为劳改所/春日站特例，
// 转存主业站合法不进共性——特例站接入时按 SiteOnly 追加）。
var bannedGroupsCommon = []string{
	"FGT", "hao4k", "mp4ba", "rarbg", "gpthd", "seeweb", "dreamhd", "blacktv",
	"xiaomi", "huawei", "momohd", "momoweb", "ddhdtv", "tagweb", "sonyhd", "minihd",
	"bitstv", "nukehd", "zerotv", "hottv", "enttv", "gamehd", "smy", "seehd",
	"verypsp", "dwr", "xlmv", "xjctv", "ctrlhd",
}

// localAuditRules 规则集（初始=幸运实证五规则 + 9 站共性七规则——docs/38 矩阵）。
var localAuditRules = []localAuditRule{
	{
		Code:   "LOCAL_MEDIUM_UNMAPPED",
		Source: "common",
		Check: func(in localAuditInput) string {
			f := in.Cfg.FormFields[model.FieldDomainMedium]
			if f == "" {
				return "" // 站点无该域非违例
			}
			if _, ok := in.Form[f]; !ok {
				return "媒介域未选择（站点词表可能缺词条）——建议核对站点发布配置的媒介映射"
			}
			return ""
		},
	},
	{
		Code:   "LOCAL_STANDARD_UNMAPPED",
		Source: "common",
		Check: func(in localAuditInput) string {
			f := in.Cfg.FormFields[model.FieldDomainStandard]
			if f == "" {
				return ""
			}
			if _, ok := in.Form[f]; !ok {
				return "分辨率域未选择（站点词表可能缺词条）——建议核对分辨率映射（标题 " + in.TP.Resolution + "）"
			}
			return ""
		},
	},
	{
		Code:   "LOCAL_AUDIO_UNMAPPED",
		Source: "common",
		Check: func(in localAuditInput) string {
			f := in.Cfg.FormFields[model.FieldDomainAudiocodec]
			if f == "" {
				return "" // 站点无音频域（修道院形态）非违例
			}
			if _, ok := in.Form[f]; !ok {
				titleU := strings.ToUpper(in.Meta.Title)
				hasAudioTrace := in.TP.AudioCodec != ""
				for _, w := range []string{"AUDIO", "DTS", "DDP", "DD+", "AC-3", "AC3", "FLAC", "AAC", "TRUEHD", "TRUE-HD", "PCM", "LPCM", "ATMOS", "5.1", "7.1", "2.0"} {
					if strings.Contains(titleU, w) {
						hasAudioTrace = true
						break
					}
				}
				if !hasAudioTrace {
					return "" // 标题/MI 均无音频痕迹不提示
				}
				return fmt.Sprintf("音频域未选择——标题/MI 有音频信息（%s）但未映射到站点词表，建议核对音频词条", in.TP.AudioCodec)
			}
			return ""
		},
	},
	{
		Code:   "LOCAL_TITLE_REASSEMBLED",
		Source: "common",
		Check: func(in localAuditInput) string {
			if in.PublishTitle != "" && in.Meta.Title != "" && in.PublishTitle != in.Meta.Title {
				return "发布标题已按 MI 纠错重组（库存标题与 MI 不一致时以 MI 权威）——如不符预期请核对种子配置 MediaInfo"
			}
			return ""
		},
	},
	{
		Code:   "LOCAL_TAG_OVERRIDE_NO_EVIDENCE",
		Source: "common",
		Check: func(in localAuditInput) string {
			for _, o := range in.TagOverrides {
				if strings.Contains(o, "cantonese") {
					hasEvidence := strings.Contains(in.Meta.Subtitle, "粤") ||
						strings.Contains(in.Meta.MediaInfo, "Cantonese")
					if !hasEvidence {
						return "人工勾选粤语但 MI/副标题无粤语依据——建议核对音轨语言（谨防站方审核判 WRONGLY）"
					}
				}
			}
			return ""
		},
	},
	{
		Code:   "LOCAL_TITLE_NON_ASCII",
		Source: "common",
		Check: func(in localAuditInput) string {
			// 9/9 站共性（docs/38 C1）：主标题禁中文/全角——tp 重组应保证纯 ASCII
			if hasFullWidthOrChinese(in.PublishTitle) {
				return "发布标题含中文/全角字符（9 站共性禁令）——请核对标题重组（中文名应走 cnname/副标题）"
			}
			return ""
		},
	},
	{
		Code:   "LOCAL_SUBTITLE_EMPTY",
		Source: "common",
		Check: func(in localAuditInput) string {
			if strings.TrimSpace(in.Meta.Subtitle) == "" {
				return "副标题为空（9 站共性必填）——建议补齐副标题再发布"
			}
			return ""
		},
	},
	{
		Code:   "LOCAL_MEDIAINFO_EMPTY",
		Source: "common",
		Check: func(in localAuditInput) string {
			if strings.TrimSpace(in.Meta.MediaInfo) == "" {
				return "MediaInfo 为空（9 站共性必填，形态各异）——建议先在种子配置页完成 MI 获取"
			}
			return ""
		},
	},
	{
		Code:   "LOCAL_IMAGES_INSUFFICIENT",
		Source: "common",
		Check: func(in localAuditInput) string {
			// 宽松阈值 2（AGSV≥2/春日≥3/青蛙藏宝阁≥1——docs/38 C5）
			shots := parseScreenshotsCol(in.Meta.Screenshots)
			if len(shots) < 2 {
				return fmt.Sprintf("截图不足（%d 张 < 2，6 站共性 ≥1-3）——建议补齐截图", len(shots))
			}
			return ""
		},
	},
	{
		Code:   "LOCAL_IMDB_EMPTY",
		Source: "common",
		Check: func(in localAuditInput) string {
			if strings.TrimSpace(in.Meta.IMDbURL) == "" {
				return "IMDb 链接为空（7 站共性必填）——建议补齐 IMDb 链接"
			}
			return ""
		},
	},
	{
		Code:   "LOCAL_BANNED_GROUP",
		Source: "common",
		Check: func(in localAuditInput) string {
			if g := containsBannedGroup(in.PublishTitle); g != "" {
				return "标题含多站禁发组 " + g + "（AGSV/CS/春日/劳改所/青蛙名单交集）——目标站可能禁发该组，发布前请确认"
			}
			return ""
		},
	},
	{
		Code:   "LOCAL_TITLE_WORD_FORM",
		Source: "common",
		Check: func(in localAuditInput) string {
			// 词形规范防御（青蛙/织梦词形+多站归一——docs/38 §三）
			t := " " + in.PublishTitle + " "
			var bad []string
			if strings.Contains(t, " 4K ") {
				bad = append(bad, "4K→2160p")
			}
			for _, p := range []string{" 1080P ", " 720P ", " 2160P ", " 480P ", " 4320P "} {
				if strings.Contains(t, p) {
					bad = append(bad, strings.TrimSpace(p)+"→小写p")
					break
				}
			}
			if strings.Contains(t, " AC3 ") {
				bad = append(bad, "AC3→DD")
			}
			if strings.Contains(t, " HDR10 ") {
				bad = append(bad, "HDR10→HDR")
			}
			for _, w := range []string{" HQ ", " EDR ", " SDR "} {
				if strings.Contains(t, w) {
					bad = append(bad, strings.TrimSpace(w)+"应删除")
				}
			}
			if len(bad) > 0 {
				return "标题词形违规（" + strings.Join(bad, "；") + "）——请核对标题重组"
			}
			return ""
		},
	},
}

// containsBannedGroup 词边界匹配（青蛙/藏宝阁同款——防 "alt" 伪命中 "Altitude"）。
func containsBannedGroup(title string) string {
	u := strings.ToUpper(title)
	for _, g := range bannedGroupsCommon {
		gu := strings.ToUpper(g)
		idx := strings.Index(u, gu)
		for idx >= 0 {
			before := byte(' ')
			if idx > 0 {
				before = u[idx-1]
			}
			after := byte(' ')
			if idx+len(gu) < len(u) {
				after = u[idx+len(gu)]
			}
			if !isAlnum(before) && !isAlnum(after) {
				return g
			}
			next := strings.Index(u[idx+1:], gu)
			if next < 0 {
				break
			}
			idx = next + idx + 1
		}
	}
	return ""
}

func isAlnum(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// hasFullWidthOrChinese 全角字符或中文检测（9 站共性 C1——同款 [\u4e00-\u9fa5\uFF01-\uFF60]）。
func hasFullWidthOrChinese(s string) bool {
	for _, r := range s {
		if (r >= 0x4e00 && r <= 0x9fa5) || (r >= 0xFF01 && r <= 0xFF60) {
			return true
		}
	}
	return false
}

// FormatLocalAudit 提示格式化（成功态 message 后缀）。
func FormatLocalAudit(findings []LocalAuditFinding) string {
	if len(findings) == 0 {
		return ""
	}
	parts := make([]string, 0, len(findings))
	for _, f := range findings {
		parts = append(parts, f.Code+": "+f.Message)
	}
	return "内部规范提示[" + strings.Join(parts, "；") + "]"
}

// SerializeLocalAudit 落库形态（logs 列 JSON——实战回流分析对照站方结果）。
func SerializeLocalAudit(findings []LocalAuditFinding) string {
	if len(findings) == 0 {
		return ""
	}
	b, err := json.Marshal(findings)
	if err != nil {
		return ""
	}
	return string(b)
}
