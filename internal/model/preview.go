package model

// §56.24 决策 2/Q3 — PreviewResponse: 字段预览响应。
// 用于 9d reverse mapping UI，显示目标站将接收的最终字段。
type PreviewResponse struct {
	TargetSite    string            `json:"target_site"`     // 目标站名
	Mode          string            `json:"mode"`            // ptgen_first / detail_first
	Fields        []PreviewField    `json:"fields"`          // 字段列表
	Completeness  PreviewCompleteness `json:"completeness"`  // 完整度检查
}

// PreviewField 单个字段预览。
type PreviewField struct {
	Category     string `json:"category"`       // 元数据/简介/媒体/链接/标签/来源标注
	StandardKey  string `json:"standard_key"`   // 标准化键（如 title/subtitle/resolution）
	Label        string `json:"label"`          // 显示名（如 "标题"/"分辨率"）
	TargetValue  string `json:"target_value"`   // 目标站字段值（reverse mapping 后）
	SourceValue  string `json:"source_value"`   // 原始值（mapping 前）
	Source       string `json:"source"`         // 来源（ptgen/detail/local/user）
	IsUserEdited bool   `json:"is_user_edited"` // 是否用户编辑
	Required     bool   `json:"required"`       // 是否必填
	Missing      bool   `json:"missing"`        // 是否缺失（必填但空）
}

// PreviewCompleteness 完整度检查。
type PreviewCompleteness struct {
	Total    int `json:"total"`     // 总字段数
	Filled   int `json:"filled"`    // 已填充数
	Missing  int `json:"missing"`   // 缺失必填数
	Percent  int `json:"percent"`   // 完整度百分比
}
