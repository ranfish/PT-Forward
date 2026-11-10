package publish

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

// ═══ §59.166 LocalAudit 内部重组审核规范（advisory——仅提示不阻塞）═══
//
// §59.504 规则可达性审查（12→6 条·实战数据驱动删减）：
// 删 6 条——REASSEMBLED（1652 次高频·重组=设计行为非错误·无行动价值·
//   透明性已由 DryRun form 预览+站方页面承担）；
// NON_ASCII（死规则——重组出口 StripChinese 恒剥中文/全角·两面皆不可达）；
// SUBTITLE_EMPTY（3 次全 pushed 零站方拒·零行动价值）；
// MEDIAINFO_EMPTY（口径错位——单列检查 vs 审核门三列·原盘 BDInfo 行
//   误报；连 DryRun 面一起删——与审核门同判据的提示是重复信息源）；
// IMDB_EMPTY（205 次零 failed·98.5% 成功率高于基线 57%·imdb 表单域
//   1/72 站——"7 站必填"在 62 站适配面未兑现；失败兜底=站方拒信回流）；
// TITLE_WORD_FORM（71 次全 "HDR10→HDR" 伪规则误报——HDR10 是 tp.HDR
//   合法值+站点自治开关 HDR10ToHDR 已存在；大写P/AC3/SDR 三死检查——
//   槽级归一早已结构性保证；4K 真缺口移 normalizeResolution 兜底修）。
// 修 1 条——IMAGES 阈值 2→3（对齐审核门 MinScreenshots）+解析函数
//   单源化（model.ParseScreenshotColumn·防双源漂移）。
// 留 5 条——映射类三条（9 次 medium miss 全 pushed 实证 advisory 定位
//   正确·不升级 fail-fast）；粤语无据（行为防御·真行动）；禁发组
//   （低频高止损·FGT 种发禁发站=删种+警告）。
//
// 用户方法论：结构性防止错误可达——提示是兜底，上游能修的修上游
// （4K 词形→重组器兜底归一；噪声→删规则）。

// LocalAuditFinding 单条提示（全 WARN）。
type LocalAuditFinding struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Source  string `json:"source"` // 共性规则="common"；站特例=站名
}

// localAuditInput 引擎输入（executor ⑤ tags 段后的终态产物）。
// §59.504: 删 PublishTitle/AppliedTags（消费者 #4/#6/#12 随规则删减归零）。
type localAuditInput struct {
	TargetSite   string
	TP           titleparser.TechProfile
	Form         map[string]string
	Cfg          *model.PublishFormConfig
	TagOverrides []string
	Meta         *model.TorrentMetadata
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

// localAuditRules 规则集（§59.504 终局 6 条·文案=用户定版纯事实+带值）。
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
				return "媒介域未选择"
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
				if in.TP.Resolution != "" {
					return "分辨率域未选择（" + in.TP.Resolution + "）"
				}
				return "分辨率域未选择"
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
				if in.TP.AudioCodec != "" {
					return "音频域未选择（" + in.TP.AudioCodec + "）"
				}
				return "音频域未选择"
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
						return "人工勾选粤语但 MI/副标题无粤语依据"
					}
				}
			}
			return ""
		},
	},
	{
		Code:   "LOCAL_IMAGES_INSUFFICIENT",
		Source: "common",
		Check: func(in localAuditInput) string {
			// §59.504: 阈值对齐审核门 MinScreenshots=3（原 2 过时）；解析
			// 单源化 model.ParseScreenshotColumn（原 executor 私有
			// parseScreenshotsCol 双源漂移——空串过滤口径与审核门一致）。
			// 实战命中面=force 审核旁路（§59.484）+DryRun 未审核种子引导。
			shots := model.ParseScreenshotColumn(in.Meta.Screenshots)
			if len(shots) < 3 {
				return "截图不足（" + itoa(len(shots)) + " 张）"
			}
			return ""
		},
	},
	{
		Code:   "LOCAL_BANNED_GROUP",
		Source: "common",
		Check: func(in localAuditInput) string {
			// §59.504: 消费源改标题原文（PublishTitle 字段已删——Meta.Title
			// 含组名段同等可达：源组名来自标题解析）。
			if g := containsBannedGroup(in.Meta.Title); g != "" {
				return "标题含多站禁发组（" + g + "）"
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

func itoa(n int) string {
	return strconv.Itoa(n)
}

// FormatLocalAudit 提示格式化（成功态 message 后缀）。
// §59.504: 去内部 CODE 前缀——英文代号对用户无意义（溯源走 logs 列
// JSON 的 code 字段，单点保留）。
func FormatLocalAudit(findings []LocalAuditFinding) string {
	if len(findings) == 0 {
		return ""
	}
	parts := make([]string, 0, len(findings))
	for _, f := range findings {
		parts = append(parts, f.Message)
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
