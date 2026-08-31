// Package publish 站点发布配置中心——HTML 上传半自动（§59.147 定案，切片 3）。
//
// 流程：用户上传发布页 HTML → 解析草稿（goquery，HTML 即弃不落库不写日志——L2）
// → 与现配置 diff 三分类 → 人工确认 → 落库（diff 确认是唯一写入路径）。
//
// diff 三分类（§59.147 用户定位修正：对不上=信号非异常）：
//   matched  = 基线校准（label/value/语义全同）
//   changed  = 改版信号（字段名变化/label 变化）
//   added    = 新增选项（待标注——standard_keys 空，语义继承失败）
//   removed  = 站方删除选项
//   语义错位 = label 同 value 异 → kind=changed（首版适配审计信号）
//
// 语义继承：草稿只有站方 label/value——standard_keys/auto 按 label 从现配置继承
// （新增选项无继承=待标注清单兜底，L3）。
package publish

import (
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/ranfish/pt-forward/internal/model"
)

// htmlDomainRules 字段名 → 逻辑域识别规则（NexusPHP 家族实测形态——§59.149）。
var htmlDomainRules = []struct {
	prefix string // name 精确或前缀
	domain string
}{
	{"type", model.FieldDomainType},
	{"medium_sel", model.FieldDomainMedium},
	{"codec_sel", model.FieldDomainCodec},
	{"standard_sel", model.FieldDomainStandard},
	{"audiocodec_sel", model.FieldDomainAudiocodec},
	{"team_sel", model.FieldDomainTeam},
	{"tags", model.FieldDomainTags},
}

// ParsePublishFormHTML 解析发布页 HTML → 配置草稿。
// 返回 nil = HTML 无可识别表单结构。
func ParsePublishFormHTML(html string) *model.PublishFormConfig {
	if strings.TrimSpace(html) == "" {
		return nil
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil
	}

	draft := &model.PublishFormConfig{
		Enabled:       true,
		Framework:     "np",
		FormFields:    map[string]string{},
		ValueMappings: map[string][]model.FormValueMapping{},
	}
	seenDomains := map[string]bool{}

	// 下拉族：select → option 列表
	doc.Find("select").Each(func(_ int, sel *goquery.Selection) {
		name, ok := sel.Attr("name")
		if !ok || name == "" {
			return
		}
		domain := domainOfField(name)
		if domain == "" {
			return
		}
		opts := []model.FormValueMapping{}
		sel.Find("option").Each(func(_ int, opt *goquery.Selection) {
			v, _ := opt.Attr("value")
			label := strings.TrimSpace(opt.Text())
			if v == "" || label == "" || v == "0" && (strings.Contains(label, "选择") || label == "") {
				return // 占位项
			}
			opts = append(opts, model.FormValueMapping{Label: label, Value: v})
		})
		if len(opts) == 0 {
			return
		}
		draft.FormFields[domain] = name
		draft.ValueMappings[domain] = opts
		seenDomains[domain] = true
	})

	// tags checkbox 族：input[type=checkbox][name^=tags]
	tags := []model.FormValueMapping{}
	doc.Find("input[type='checkbox']").Each(func(_ int, cb *goquery.Selection) {
		name, _ := cb.Attr("name")
		if !strings.HasPrefix(name, "tags") {
			return
		}
		v, _ := cb.Attr("value")
		if v == "" {
			return
		}
		label := checkboxLabel(cb)
		if label == "" {
			label = v
		}
		tags = append(tags, model.FormValueMapping{Label: label, Value: v})
	})
	if len(tags) > 0 {
		// 字段名取实际 name（tags[4][] 本身即数组形态——原样保留）
		fieldName := ""
		doc.Find("input[type='checkbox']").EachWithBreak(func(_ int, cb *goquery.Selection) bool {
			n, _ := cb.Attr("name")
			if strings.HasPrefix(n, "tags") {
				fieldName = n
				return false
			}
			return true
		})
		draft.FormFields[model.FieldDomainTags] = fieldName
		draft.ValueMappings[model.FieldDomainTags] = tags
		seenDomains[model.FieldDomainTags] = true
	}

	// 文本域存在性（名称记录——供 diff 检测改版）
	for field, domain := range map[string]string{
		"small_descr":     model.FieldDomainSmallDescr,
		"url":             model.FieldDomainIMDBURL,
		"descr":           model.FieldDomainDescription,
		"technical_info":  model.FieldDomainTechInfo,
	} {
		if doc.Find("input[name='"+field+"'], textarea[name='"+field+"']").Length() > 0 {
			draft.FormFields[domain] = field
		}
	}

	if len(draft.FormFields) == 0 {
		return nil
	}
	return draft
}

// domainOfField 字段名 → 逻辑域（精确匹配优先，前缀匹配去 [N] 后缀形态）。
func domainOfField(name string) string {
	for _, r := range htmlDomainRules {
		if name == r.prefix || strings.HasPrefix(name, r.prefix+"[") {
			return r.domain
		}
	}
	return ""
}

// checkboxLabel 提取 checkbox 相邻 label 文本（NexusPHP 形态：label 包裹或紧邻）。
func checkboxLabel(cb *goquery.Selection) string {
	// 形态1: <label><input>文本</label>
	if p := cb.Parent(); p.Is("label") {
		return strings.TrimSpace(p.Text())
	}
	// 形态2: <input><label>文本</label> 紧邻
	if n := cb.Next(); n.Is("label") {
		return strings.TrimSpace(n.Text())
	}
	// 形态3: <span>文本</span><input> 前邻
	if p := cb.Prev(); p.Is("span, label") {
		return strings.TrimSpace(p.Text())
	}
	return ""
}

// FormConfigDiffItem 单条 diff（三分类）。
type FormConfigDiffItem struct {
	Domain       string   `json:"domain"`
	Kind         string   `json:"kind"` // matched/changed/added/removed
	Label        string   `json:"label"`
	CurrentValue string   `json:"current_value,omitempty"`
	DraftValue   string   `json:"draft_value,omitempty"`
	CurrentKeys  []string `json:"current_keys,omitempty"`
	DraftKeys    []string `json:"draft_keys,omitempty"`
	AutoFalse    bool     `json:"auto_false,omitempty"`
	FieldRename  string   `json:"field_rename,omitempty"` // 字段名改版（a → b）
}

// MergeDraftWithCurrent 草稿与现配置合并：语义继承 + diff 生成。
// merged = 草稿形态（站方真名+label+value）+ 继承的 standard_keys/auto；
// diffs = 三分类清单（人工确认的数据源）。
func MergeDraftWithCurrent(current, draft *model.PublishFormConfig) (*model.PublishFormConfig, []FormConfigDiffItem) {
	if current == nil {
		current = &model.PublishFormConfig{FormFields: map[string]string{}, ValueMappings: map[string][]model.FormValueMapping{}}
	}
	merged := &model.PublishFormConfig{
		Enabled:      current.Enabled || draft.Enabled,
		Framework:    draft.Framework,
		PreAuditURL:  current.PreAuditURL, // 预检 URL 非站方 HTML 信息——继承现配置
		FormFields:   map[string]string{},
		ValueMappings: map[string][]model.FormValueMapping{},
		TagConfig:    current.TagConfig,
	}
	diffs := []FormConfigDiffItem{}

	// 字段名级 diff（改版信号）
	for domain, draftField := range draft.FormFields {
		curField := current.FormFields[domain]
		if curField != "" && curField != draftField {
			diffs = append(diffs, FormConfigDiffItem{
				Domain: domain, Kind: "changed", FieldRename: curField + " → " + draftField,
				Label: "(字段名改版)",
			})
		}
		merged.FormFields[domain] = draftField
	}
	// 现配置有而草稿缺的域（站方删表单字段——removed 信号）
	for domain := range current.FormFields {
		if _, ok := draft.FormFields[domain]; !ok {
			diffs = append(diffs, FormConfigDiffItem{Domain: domain, Kind: "removed", Label: "(整域消失)"})
		}
	}

	// 值级 diff：按 label 对齐
	for domain, draftVals := range draft.ValueMappings {
		curVals := indexByLabel(current.ValueMappings[domain])
		mergedVals := make([]model.FormValueMapping, 0, len(draftVals))
		for _, dv := range draftVals {
			if cv, ok := curVals[dv.Label]; ok {
				// 继承语义
				inherited := model.FormValueMapping{Label: dv.Label, Value: dv.Value, StandardKeys: cv.StandardKeys, Auto: cv.Auto}
				mergedVals = append(mergedVals, inherited)
				kind := "matched"
				if cv.Value != dv.Value {
					kind = "changed" // 语义错位信号：label 同 value 异（首版适配审计）
				}
				if kind == "matched" {
					continue // 匹配项不进 diff（基线校准噪音抑制）
				}
				diffs = append(diffs, FormConfigDiffItem{
					Domain: domain, Kind: kind, Label: dv.Label,
					CurrentValue: cv.Value, DraftValue: dv.Value,
					CurrentKeys: cv.StandardKeys,
				})
			} else {
				// 新增选项（待标注——standard_keys 空）
				mergedVals = append(mergedVals, dv)
				diffs = append(diffs, FormConfigDiffItem{
					Domain: domain, Kind: "added", Label: dv.Label, DraftValue: dv.Value,
				})
			}
		}
		merged.ValueMappings[domain] = mergedVals
		// 现配置有而草稿无的 label（站方删除选项）
		draftIdx := indexByLabel(draftVals)
		for _, cv := range current.ValueMappings[domain] {
			if _, ok := draftIdx[cv.Label]; !ok {
				diffs = append(diffs, FormConfigDiffItem{
					Domain: domain, Kind: "removed", Label: cv.Label, CurrentValue: cv.Value,
					CurrentKeys: cv.StandardKeys, AutoFalse: cv.Auto != nil && !*cv.Auto,
				})
			}
		}
	}
	return merged, diffs
}

func indexByLabel(vals []model.FormValueMapping) map[string]model.FormValueMapping {
	m := make(map[string]model.FormValueMapping, len(vals))
	for _, v := range vals {
		m[v.Label] = v
	}
	return m
}
