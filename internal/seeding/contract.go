package seeding

import (
	"fmt"

	"github.com/ranfish/pt-forward/internal/rule"
)

// 15 刀3（v0.0.1661）: 声明式条件配置期校验——与 ValidateExpr 对称
// （expr 路径已有编译期拦截先例·声明式 conditions 原先零校验直落 DB·
// 拼错字段/操作符运行期恒假静默失效〔F-01 取证：29 实态 6 条规则全
// 已知——校验接线零兼容风险〕）。
//
// ruleConditionFields 15 刀3（v0.0.1661）: 条件字段白名单——
// **fieldValue 实现真实键集**（49 键·全函数正则提取·含蛇形旧键
// 兼容层〔site_name/is_free/has_hr 等〕+驼峰新键并存——非 docs31
// 42 键规划表：实施中实证规划表 12 键未实现〔声明式亦无〕·实现集
// 另有 unregistered/subscription_id 等 42 表外键且 29 生产规则在用
// ——校验集必须对齐实现而非文档·文档差距记 §59.579 附六）。
// scoringCycleID 的 fieldValue 恒 not-known（case 存在但 return
// "", false）——排除。
var ruleConditionFields = map[string]struct{}{
	// Record 层
	"free_level": {}, "status": {}, "discount": {},
	"source": {}, "last_action_by": {}, "unregistered": {},
	"unregistered_msg": {}, "subscription_id": {}, "client_id": {},
	"torrent_id": {}, "tracker_url": {}, "site_name": {},
	// Torrent 层
	"name": {}, "size": {}, "totalSize": {}, "progress": {}, "state": {},
	"uploaded": {}, "downloaded": {}, "uploadSpeed": {}, "downloadSpeed": {},
	"ratio": {}, "seedingTime": {}, "seed_time": {}, "category": {}, "tags": {},
	"savePath": {}, "seeder": {}, "num_complete": {}, "leecher": {}, "num_incomplete": {}, "error": {},
	// 状态/标记层（蛇形兼容键）
	"is_free": {}, "is_finished": {}, "is_paused": {}, "has_hr": {},
	"hr_seed_time_h": {},
	// 上下文层
	"freeRemainSec": {}, "downloadUploadRatio": {}, "addedTime": {},
	"hrRemainSec": {}, "freeSpace": {}, "totalSpace": {},
	"diskUsedPercent": {}, "hour": {}, "activeUploads": {},
	"activeDownloads": {}, "globalUploadSpeed": {}, "globalDownloadSpeed": {},
	// 评分层
	"scoringScore": {}, "scoringRank": {}, "lowScoreCount": {},
}

// ruleConditionOperators 操作符白名单（两实现 evalCondition 的
// switch case 并集·:241-259/:514-528 基线）。
var ruleConditionOperators = map[string]struct{}{
	"equals": {}, "==": {}, "not_equals": {}, "!=": {},
	"contains": {}, "not_contains": {},
	"includeIn": {}, "notIncludeIn": {},
	"regExp": {}, "notRegExp": {},
	"bigger": {}, ">": {}, "smaller": {}, "<": {},
	"bigger_eq": {}, ">=": {}, "smaller_eq": {}, "<=": {},
}

// ValidateConditions 15 刀3: 声明式条件校验——字段与操作符均须在
// 白名单（拼错当场显性化·运行期兜底统一拒绝见 evalCondition）。
func ValidateConditions(conditionsJSON string) error {
	conds := ParseConditions(conditionsJSON)
	if conditionsJSON != "" && len(conds) == 0 {
		return fmt.Errorf("conditions JSON 解析失败")
	}
	for i, cond := range conds {
		if _, ok := ruleConditionFields[cond.Field]; !ok {
			return fmt.Errorf("条件 %d 字段 %q 不在支持清单（拼写错误？）", i+1, cond.Field)
		}
		if _, ok := ruleConditionOperators[cond.Operator]; !ok {
			return fmt.Errorf("条件 %d 操作符 %q 不支持（拼写错误？）", i+1, cond.Operator)
		}
		// §59.596: unit 白名单（前端回显键·拼错配置期拦截——15 篇纪律）。
		if cond.Unit != "" && cond.Unit != "minutes" && cond.Unit != "hours" && cond.Unit != "days" {
			return fmt.Errorf("条件 %d 单位 %q 不支持（minutes/hours/days）", i+1, cond.Unit)
		}
	}
	return nil
}

// RuleConditionFields 15 刀3: 白名单导出（42/42 覆盖锚消费·
// 断言双 expr 环境字段存在性）。
func RuleConditionFields() []string {
	out := make([]string, 0, len(ruleConditionFields))
	for k := range ruleConditionFields {
		out = append(out, k)
	}
	return out
}

// 编译期断言：确保 rule 包语义单点在 seeding 侧可达（双路径一致性）。
var _ = rule.DownloadUploadRatioOf
