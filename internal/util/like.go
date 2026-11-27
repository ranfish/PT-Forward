package util

import "strings"

// LikeEscapeSuffix 13 刀1（v0.0.1656）: LIKE ? 必配的 ESCAPE 子句。
// 铁律：转义值（LikeEscape 产出含 \%/\_ 形态）与 ESCAPE 子句必须成对——
// 缺子句时反斜杠是字面量，\% 匹配字面「反斜杠+百分号」而非 %，字面
// 通配符查询反漏匹配（13 篇 V-02：ptgen 现状查 100%done 返回空）。
//
// 用法：q.Where("name LIKE ?"+util.LikeEscapeSuffix, "%"+util.LikeEscape(s)+"%")
// ——多列 OR 链每处 LIKE ? 均须后缀本常量。
const LikeEscapeSuffix = " ESCAPE '\\'"

// LikeEscape 13 刀1（v0.0.1656）: LIKE 模式转义公共单点（v0.0.1656 收敛
// declaration_backfill_task.go 私有 likeEscape——全库唯一真源）。
//
// 顺序铁律：\ → \\ 必须最先（否则后继引入的 \% 被二次转义为 \\%）。
// 用户输入进入 LIKE 模式前一律经本函数——禁止各端点手搓拼接
// （13 篇 V-01：10 处搜索端点裸拼 "%"+用户输入+"%"，用户输入 % 时
// 模式 %%% 命中全表·搜索框失去过滤作用·实测 5 行伪表全命中）。
// 参数绑定防注入但挡不住通配符语义劫持——转义与 ESCAPE 两者缺一即错。
func LikeEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// LikeContains 13 刀1: 「包含匹配」模式工厂——%转义值%。
// 与 LikeEscapeSuffix 配套使用；消费端禁止手拼 "%" 前后缀。
func LikeContains(s string) string {
	return "%" + LikeEscape(s) + "%"
}
