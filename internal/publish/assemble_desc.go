// Package publish 发布描述组装——纯本地（§59.159 资产消费白名单）。
//
// 发布链网络动作白名单（三允许，其余禁止——AGENTS.md 铁律）：
//   ① 目标站交互（pre-audit/dedup/上传/下载新种）——发布动作组成
//   ② 下载器 RPC（导出/加种）——本地资源引用
//   ③ 无了。PTGen/图床转存/豆瓣/一切外部 API 禁止——必须消费 torrent_metadata
//      落库资产（历史违规两例：queryPTGen/rehostPoster——renderDescription
//      采集时代组件的旧习惯，本函数即其发布专用替代）。
//
// 组装形态（§59.159 用户定案）：
//   [quote]Tab2 声明完整 BBCode[/quote]  ← 置顶
//   Tab4 简介（自带海报——§59.166 不再插 Tab2 海报防双份）
//   [img]截图 ×N[/img]（≤8 张，无引号）    ← 简介下方
package publish

import (
	"regexp"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
)

// miColonRe 幸运 MI 误判防御（§59.163 探针四发实证：description 含
// `Video:`/`Audio:` 段名+冒号**子串**即触发 DESCRIPTION_CONTAINS_MEDIAINFO——
// 非行首[探针 G]、空格变体仍命中[H3]；**全角冒号无命中[H1]**）。
// 用户定案：尽量保留源站引用——不剥行，仅段名冒号全角化（内容零丢失，
// 中文排版无害）。
var miColonRe = regexp.MustCompile(`((?:General|Video|Audio|Text|Subtitles?|Chapters?|Menu))\s*:`)

// FullwidthMIColons quote 内容 MI 段名冒号全角化（`Video:`→`Video：`——
// 探针 H1 实证站方检测通过；Source # 列表无此形态不受影响）。
func FullwidthMIColons(s string) string {
	return miColonRe.ReplaceAllString(s, "$1：")
}

// assembleDescription 发布描述纯本地组装（零网络依赖）。
// §59.166 海报双份回归修复：Tab4 简介（rendered PTGen 产物）头部自带海报
// （§59.88 同型：doubaninfo format 头部图）——不再插入 Tab2 海报（用户定案）；
// 新建本函数时未继承 §59.88 防御致双份回归。
// meta 各资产来自种子配置页六 Tab（获取-审核阶段产物）。
func assembleDescription(meta *model.TorrentMetadata) string {
	var b strings.Builder
	if q := strings.TrimSpace(meta.Statement); q != "" {
		b.WriteString("[quote]")
		b.WriteString(FullwidthMIColons(q))
		b.WriteString("[/quote]\n\n")
	}
	if d := strings.TrimSpace(meta.Description); d != "" {
		b.WriteString(d)
	}
	shots := parseScreenshotsCol(meta.Screenshots)
	if len(shots) > 8 { // §59.84 上限
		shots = shots[:8]
	}
	if len(shots) > 0 {
		lines := make([]string, 0, len(shots))
		for _, u := range shots {
			lines = append(lines, "[img]"+u+"[/img]")
		}
		b.WriteString("\n\n")
		b.WriteString(strings.Join(lines, "\n"))
	}
	return b.String()
}
