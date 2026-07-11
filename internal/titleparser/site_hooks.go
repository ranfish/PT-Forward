package titleparser

import "strings"

// SiteTitleHook 站点特殊标题格式化钩子
// 当模板无法表达复杂规则时（如字段顺序随媒介变化），用 hook 替代
type SiteTitleHook func(c TitleComponents, tf TitleFormat) string

var siteHooks map[string]SiteTitleHook

func init() {
	siteHooks = map[string]SiteTitleHook{
		"pterclub": pterclubHook,
	}
}

// GetSiteHook 按 hook 名获取钩子函数
func GetSiteHook(name string) SiteTitleHook {
	return siteHooks[name]
}

// pterclubHook 猫站特殊规则：
// 原盘/REMUX → 编码在音频前
// Encode/WEB-DL → 编码在音频后
func pterclubHook(c TitleComponents, tf TitleFormat) string {
	isDisc := strings.Contains(strings.ToLower(c.Medium), "blu") ||
		strings.Contains(strings.ToLower(c.Medium), "remux")

	var order []string
	if isDisc {
		order = []string{"title", "year", "season", "resolution", "medium", "video_codec", "audio_codec", "group"}
	} else {
		order = []string{"title", "year", "season", "resolution", "medium", "audio_codec", "video_codec", "group"}
	}

	tf.Order = order
	tf.Separator = " "
	tf.GroupConnector = "-"
	tf.StripChinese = true
	tf.Forbidden = []string{"BDMV", "BDISO", "BDBOX", "DVDISO", "BDRip"}
	tf.Hook = "" // 清除 hook 防止递归

	result := reassembleWithTemplate(c, tf)
	result = strings.ReplaceAll(result, "BDRip", "BluRay")
	return result
}
