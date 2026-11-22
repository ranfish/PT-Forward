// Package description 感谢引言生成（§56.20 决策 2）。
//
// 借鉴 easy-upload，全局配置 + 每站模板覆盖。
// 默认中文站: "转自[b]{source_site}[/b]，感谢原发布者！"
// 默认英文站: "Torrent from [b]{source_site}[/b]. All thanks to the original uploader!"
package description

import (
	"strings"

	"github.com/ranfish/pt-forward/internal/util"
)

// ThanksQuoteConfig 感谢引言配置。
type ThanksQuoteConfig struct {
	Enabled         bool   // 全局开关
	ChineseTemplate string // 中文站模板（空=用默认）
	EnglishTemplate string // 英文站模板（空=用默认）
	SiteTemplate    string // per-site 覆盖（空=用全局）
}

// 默认模板常量。
const (
	// §59.20: 加 {group_name} 占位符
	// §59.68: 中文模板去"转自{source_site}"段（用户定案——致谢不披露转载来源）
	DefaultChineseThanksQuote = "{group_name}官组作品，感谢原制作者发布。"
	DefaultEnglishThanksQuote = "Group {group_name} release.\nAll thanks to the original uploader!"

	// ThanksQuotePlaceholder 源站名占位符。
	ThanksQuotePlaceholder = "{source_site}"

	// GroupNamePlaceholder §59.20: 制作组名占位符。
	GroupNamePlaceholder = "{group_name}"

	// ThanksCoreText §59.542 刀1: 禁转引言文本单源——追加锚/剥离锚/
	// 声明过滤词表全部派生于此（改措辞只动这一处·生成与剥离永不漂移）。
	// 系统独有措辞（用户确认未见于其它发布者）。
	// §59.555 用户措辞定案: 末尾句号——两句是独立句子应带句号（与蓝头
	// 句对称）。剥离锚须走 Legacy 形态（无句号是句号版子串·双形态兼容）。
	ThanksCoreText = "互珍互重，禁转PTT。"
)

// ThanksCoreTextLegacy §59.555: 无句号历史形态锚——v0.0.1555 前生成与
// 存量 backfill 声明均无句号；剥离消费面（词表/正则）用本锚=同时命中
// 存量与新形态（无句号子串在句号文本中亦 Contains）。
var ThanksCoreTextLegacy = strings.TrimSuffix(ThanksCoreText, "。")

// GenerateThanksQuote 生成感谢引言。
//
//	sourceSite: 源站名
//	groupName: 制作组名（§59.20 新增）
//	isEnglishSite: 是否英文站（true=用英文模板）
//	config: 配置（nil=默认配置，Enabled=true）
//
// 返回空字符串表示禁用或模板为空。
func GenerateThanksQuote(sourceSite, groupName string, isEnglishSite bool, config *ThanksQuoteConfig) string {
	if config != nil && !config.Enabled {
		return ""
	}

	template := selectTemplate(isEnglishSite, config)
	if template == "" {
		return ""
	}

	result := strings.ReplaceAll(template, ThanksQuotePlaceholder, sourceSite)
	result = strings.ReplaceAll(result, GroupNamePlaceholder, groupName)
	return result
}

// selectTemplate 按优先级选择模板（SiteTemplate > 全局对应语言模板 > 默认模板）。
func selectTemplate(isEnglishSite bool, config *ThanksQuoteConfig) string {
	if config != nil && config.SiteTemplate != "" {
		return config.SiteTemplate
	}
	if isEnglishSite {
		if config != nil && config.EnglishTemplate != "" {
			return config.EnglishTemplate
		}
		return DefaultEnglishThanksQuote
	}
	if config != nil && config.ChineseTemplate != "" {
		return config.ChineseTemplate
	}
	return DefaultChineseThanksQuote
}

// BuildOfficialThanks 生成完整两句官方致谢块（§59.542 追记十二：
// 消费时合成单点——水印从「入库资产」变「消费时合成」）。
// 形态与历史获取链 append（v0.0.597-1530）完全一致。
func BuildOfficialThanks(siteName, group string) string {
	thanksLine := GenerateThanksQuote(siteName, group, false, nil)
	head := "[quote][b][color=blue][size=5]" + thanksLine + "[/size][/color][/b][/quote]"
	tail := "[quote][b][color=red][size=5]" + ThanksCoreText + "[/size][/color][/b][/quote]"
	return head + "\n" + tail
}

// HasOfficialThanks statement 是否已含我方致谢（消费合成判别——历史
// backfill 行与回流形态均含「官组作品」锚词·两态兼容判别）。
func HasOfficialThanks(statement string) bool {
	return strings.Contains(statement, "官组作品")
}

// ComposeStatement §59.542 追记十二（用户定案·回归 v0.0.597 原始设计）：
// 消费时合成——tab2 展示与发布组装共用单点。DB statement 保持净化态
// （站方原始声明·数据与水印分离·获取链不再入库 append）；消费层动态
// 合成：官组行（title 组名提取·发布链 :2487 同源 util.ExtractGroupName）
// 且无致谢 → base + 致谢；已有致谢（历史 backfill 行）原样返回（两态
// 兼容·无需存量再清洗）；非官组/无组名 → 原样。
func ComposeStatement(statement, siteName, title string) string {
	group := util.ExtractGroupName(title)
	// Other=站方兜底槽语义（无真实组名）——同 NOGROUP 不合成致谢。
	if group == "" || group == "NOGROUP" || group == "Other" {
		return statement
	}
	if HasOfficialThanks(statement) {
		return statement
	}
	thanks := BuildOfficialThanks(siteName, group)
	if strings.TrimSpace(statement) == "" {
		return thanks
	}
	return statement + "\n\n" + thanks
}
