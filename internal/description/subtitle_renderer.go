// Package description 副标题模板渲染（§56.20 Q3）。
//
// 5 变量简化版副标题模板引擎：
//   {source_subtitle}      源站副标题（清理后）
//   {ptgen.chinese_title}  PTGen 中文名
//   {ptgen.foreign_title}  PTGen 外文名
//   {ptgen.year}           年份
//   {title.main}           主标题
package description

import "strings"

// SubtitleData 副标题渲染数据。
type SubtitleData struct {
	SourceSubtitle    string // 源站副标题（清理后）
	PTGenChineseTitle string // PTGen 中文名
	PTGenForeignTitle string // PTGen 外文名
	PTGenYear         string // 年份
	TitleMain         string // 主标题
}

// 副标题模板变量。
const (
	VarSourceSubtitle    = "{source_subtitle}"
	VarPTGenChineseTitle = "{ptgen.chinese_title}"
	VarPTGenForeignTitle = "{ptgen.foreign_title}"
	VarPTGenYear         = "{ptgen.year}"
	VarTitleMain         = "{title.main}"
)

// RenderSubtitle 按模板渲染副标题。
//   template 为空时返回 SourceSubtitle（默认行为）。
//   变量为空时替换为空字符串（可能导致分隔符残留，调用方需清理）。
func RenderSubtitle(template string, data SubtitleData) string {
	if template == "" {
		return data.SourceSubtitle
	}
	r := strings.NewReplacer(
		VarSourceSubtitle, data.SourceSubtitle,
		VarPTGenChineseTitle, data.PTGenChineseTitle,
		VarPTGenForeignTitle, data.PTGenForeignTitle,
		VarPTGenYear, data.PTGenYear,
		VarTitleMain, data.TitleMain,
	)
	return strings.TrimSpace(r.Replace(template))
}

// DefaultSubtitleTemplate 默认副标题模板。
// 格式: "ForeignTitle Year" 或 "ChineseTitle / ForeignTitle Year"
const DefaultSubtitleTemplate = "{ptgen.chinese_title} {ptgen.foreign_title} {ptgen.year}"
