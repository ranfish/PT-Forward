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
//   [img]海报[/img] + Tab4 简介            ← 落库资产直拼
//   [img]截图 ×N[/img]（≤8 张，无引号）    ← 简介下方
package publish

import (
	"regexp"
	"strings"

	"github.com/ranfish/pt-forward/internal/model"
)

// miLookalikeRe 幸运 MI 误判防御（§59.163 探针实证：description 含
// `Video:`/`Audio:` 等段名+冒号**子串**即触发 DESCRIPTION_CONTAINS_MEDIAINFO——
// 非行首匹配[探针 G]；源站制作注记[Source 对比/Notes]常含此形态）。
var miLookalikeRe = regexp.MustCompile(`(?:General|Video|Audio|Text|Subtitles?|Chapters?|Menu)\s*:`)

// StripMILookalikeLines 按行剥含 MI 段名冒号子串的行（quote 内容防御过滤——
// 探针 E 验证：过滤后站方检测通过；Source # 列表行无冒号形态保留）。
func StripMILookalikeLines(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if miLookalikeRe.MatchString(l) {
			continue
		}
		out = append(out, l)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// assembleDescription 发布描述纯本地组装（零网络依赖）。
// meta 各资产来自种子配置页六 Tab（获取-审核阶段产物）。
func assembleDescription(meta *model.TorrentMetadata) string {
	var b strings.Builder
	if q := strings.TrimSpace(meta.Statement); q != "" {
		if filtered := StripMILookalikeLines(q); filtered != "" {
			b.WriteString("[quote]")
			b.WriteString(filtered)
			b.WriteString("[/quote]\n\n")
		}
	}
	if p := strings.TrimSpace(meta.Poster); p != "" {
		b.WriteString("[img]")
		b.WriteString(p)
		b.WriteString("[/img]\n\n")
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
