// Package publish 字段合并策略（§56.23 Q1）。
//
// 合并策略: 描述类覆盖，分类保留（避免误改）。
package publish

// MergeEditFields 合并编辑表单字段。
// existing: 现有表单字段（DB 或 GetEditForm 获取）
// newValues: 新值（来自转载流程）
// 返回合并后的字段（描述类用新值，分类保留现有）。
func MergeEditFields(existing, newValues map[string]string) map[string]string {
	if existing == nil {
		existing = make(map[string]string)
	}
	merged := make(map[string]string, len(existing))
	for k, v := range existing {
		merged[k] = v
	}

	// 描述类字段：新值覆盖
	descFields := []string{"descr", "description", "body", "small_descr", "subtitle"}
	for _, field := range descFields {
		if v, ok := newValues[field]; ok && v != "" {
			merged[field] = v
		}
	}

	// MediaInfo/截图字段：新值覆盖
	techFields := []string{"mediadesc", "media_info", "screenshots"}
	for _, field := range techFields {
		if v, ok := newValues[field]; ok && v != "" {
			merged[field] = v
		}
	}

	// 分类字段：保留现有（避免误改）
	// category/cat/type 等不从 newValues 覆盖

	return merged
}
