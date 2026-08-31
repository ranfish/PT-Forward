// Package publish Tag 表单应用器（§56.22 决策 5）。
//
// 把 standard_keys 标准标签应用到 multipart 表单。
// 4 种 Mode:
//   taglist: 标准 NP tagList[] 数组
//   checkbox_id: PTer 类 checkbox（tag_gf/tag_jz/...）
//   checkbox_span: BTSchool span[] 数组
//   independent_field: HDRoute 独立 input name
package publish

import "github.com/ranfish/pt-forward/internal/model"

// TagApplyFunc 表单写入回调。
// field: 表单字段名，value: 字段值。
type TagApplyFunc func(field, value string)

// TagApplier tag 表单应用器。
type TagApplier struct {
	config *model.SiteTagConfig
}

// NewTagApplier 创建应用器。config 为 nil 时用默认 taglist 模式。
func NewTagApplier(config *model.SiteTagConfig) *TagApplier {
	if config == nil {
		config = &model.SiteTagConfig{Mode: model.TagModeTaglist}
	}
	return &TagApplier{config: config}
}

// Apply 把标准标签应用到表单。
// 调用方提供 write 回调（如 fw.writeField）。
func (a *TagApplier) Apply(tags []string, write TagApplyFunc) {
	if write == nil || len(tags) == 0 {
		return
	}

	for _, tag := range tags {
		if tag == "" {
			continue
		}
		switch a.config.Mode {
		case model.TagModeTaglist:
			// taglist 模式：直接用标准键
			write("tagList", tag)
		case model.TagModeCheckboxID:
			// PTer 类：selector 是 checkbox name
			if selector, ok := a.config.Tags[tag]; ok {
				write(selector, "on")
			}
		case model.TagModeCheckboxSpan:
			// BTSchool/幸运：selector 是 span[] 的 value；字段名参数化（§59.156
			// SpanField——幸运 tags[4][]，空默认 span[] 旧配置零影响）
			if selector, ok := a.config.Tags[tag]; ok {
				field := a.config.SpanField
				if field == "" {
					field = "span[]"
				}
				write(field, selector)
			}
		case model.TagModeIndependentField:
			// HDRoute：selector 是独立 input name
			if selector, ok := a.config.Tags[tag]; ok {
				write(selector, "1")
			}
		}
	}
}

// SupportedTags 返回目标站支持的 tag 列表（Tags map 的 key）。
// taglist 模式返回空（支持所有标准键）。
func (a *TagApplier) SupportedTags() []string {
	if a.config.Mode == model.TagModeTaglist {
		return nil // 支持所有
	}
	tags := make([]string, 0, len(a.config.Tags))
	for k := range a.config.Tags {
		tags = append(tags, k)
	}
	return tags
}

// IsSupported 判断目标站是否支持某 tag。
// taglist 模式总是返回 true。
func (a *TagApplier) IsSupported(tag string) bool {
	if a.config.Mode == model.TagModeTaglist {
		return true
	}
	_, ok := a.config.Tags[tag]
	return ok
}
