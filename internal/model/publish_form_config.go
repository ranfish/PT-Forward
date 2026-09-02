package model

import "encoding/json"

// §59.147 站点发布配置中心 — PublishFormConfig: 每站一份发布表单完整配置文档。
//
// 存于 sites.publish_form_config（单列 JSON），三层消费流：
//   种子流: sites_source_keys JSON → 新站接入初始值（出厂模板）
//   更新流: 配置 UI（HTML 上传 diff 确认）改 DB 即时生效——站方改版零发版响应
//   消费流: 发布链 DB 优先（切片 2 执行器切读；切片 1 先落库）
//
// C1 双开关: Enabled=配置启停（处方）；发布时选站勾选=动作开关（开药）。
type PublishFormConfig struct {
	// Enabled 配置启用位（数据+启停一体——§59.147 C1）
	Enabled bool `json:"enabled"`

	// Framework 配置生成形态（np/unit3d/...——L5 框架边界，切片 2 限 NP 家族）
	Framework string `json:"framework,omitempty"`

	// PreAuditURL 站方官方预审接口（§59.150：提交前预检——passed 才提交）
	PreAuditURL string `json:"pre_audit_url,omitempty"`

	// Anonymous 匿名发布站点默认（§59.159 用户定案：站点配置-发布配置勾选项——
	// 勾选=该站发布默认匿名 uplver=yes；发布请求可显式覆盖）
	Anonymous bool `json:"anonymous,omitempty"`

	// FormFields 逻辑域 → 表单字段名（站方 HTML 真名，如幸运 medium_sel[4] data-mode 后缀）
	FormFields map[string]string `json:"form_fields,omitempty"`

	// ValueMappings 域 → 站方值列表（label→standard_key 化——§59.147 C2 归拢）
	ValueMappings map[string][]FormValueMapping `json:"value_mappings,omitempty"`

	// TagConfig 存量兼容（mode+span 字段名已入 FormFields.tags；切片 2 收敛后可移除）
	TagConfig *SiteTagConfig `json:"tag_config_legacy,omitempty"`
}

// 逻辑域常量（FormFields key）：发布链内部统一域，站点差异收敛在字段名。
const (
	FieldDomainType        = "type"
	FieldDomainMedium      = "medium"
	FieldDomainCodec       = "codec"
	FieldDomainStandard    = "standard"
	FieldDomainAudiocodec  = "audiocodec"
	FieldDomainTeam        = "team"
	FieldDomainTags        = "tags"
	FieldDomainSmallDescr  = "small_descr"
	FieldDomainIMDBURL     = "imdb_url"
	FieldDomainPTGen       = "pt_gen"   // §59.159: PT-Gen 链接（幸运必填——用户实战指认）
	FieldDomainDoubanURL   = "dburl"    // §59.159: 豆瓣链接（PTNexus 同款）
	FieldDomainUplver      = "uplver"   // §59.159: 匿名发布（checkbox value=yes——语义由 Anonymous 承载，域仅记录存在性）
	FieldDomainCNName      = "cnname"  // §59.164: 中文名独立字段（修道院——幸运并入 name）
	FieldDomainDescription = "description"
	FieldDomainTechInfo    = "technical_info"
)

// FormValueMapping 单个下拉/checkbox 选项的三方绑定。
type FormValueMapping struct {
	// Label 站方显示名（HTML option 文本）
	Label string `json:"label"`
	// Value 站方表单值（HTML option value / checkbox 数字 ID）
	Value string `json:"value"`
	// StandardKeys 反查 standard_key（可多个——vc 双形态 canonical x265/HEVC 双键等）
	StandardKeys []string `json:"standard_keys,omitempty"`
	// Auto 条件标签标记（nil=可自动映射；false=组合条件禁自动勾——§59.150 幸运"英语 22"）
	Auto *bool `json:"auto,omitempty"`
}

// ParseFormConfig 从 JSON 字符串解析 PublishFormConfig；空串返回 nil（未配置）。
func ParseFormConfig(s string) *PublishFormConfig {
	if s == "" {
		return nil
	}
	var cfg PublishFormConfig
	if err := json.Unmarshal([]byte(s), &cfg); err != nil {
		return nil
	}
	return &cfg
}

// Serialize 序列化为 JSON 字符串（存 sites.publish_form_config）。
func (c *PublishFormConfig) Serialize() string {
	data, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return string(data)
}

// MappingByLabel 按 label 查域内映射（发布链正向：standard_key→value 走 ReverseLookup）。
func (c *PublishFormConfig) MappingByLabel(domain, label string) *FormValueMapping {
	for i, m := range c.ValueMappings[domain] {
		if m.Label == label {
			return &c.ValueMappings[domain][i]
		}
	}
	return nil
}
