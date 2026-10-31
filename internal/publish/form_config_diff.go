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
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/ranfish/pt-forward/internal/metadata/extract"
	"github.com/ranfish/pt-forward/internal/model"
)

// pairDivergentKeys 松查键分叉族校验：含 extended 侧键而缺 dict 侧键时
// 前插补齐（单向——发布消费恒 dict 侧，多键任一命中无害）。
func pairDivergentKeys(keys []string) []string {
	for _, fam := range model.CanonicalDivergentFamilies {
		dictKey, extKey := fam[0], fam[1]
		hasDict, hasExt := false, false
		for _, k := range keys {
			if k == dictKey {
				hasDict = true
			}
			if k == extKey {
				hasExt = true
			}
		}
		if hasExt && !hasDict {
			return append([]string{dictKey}, keys...)
		}
	}
	return keys
}

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
// unwrapViewSourceSave 兼容浏览器"查看源代码"标签页整体另存的包装页（Chrome/Edge
// view-source Ctrl+S 形态：真源码被 HTML 转义包裹在 td.line-content 单元格 +
// 行号 gutter 的查看器框架内——修道院 422 案实测形态）。识别签名→提取单元格
// 文本→反转义还原真源码；非该形态原样返回。
func unwrapViewSourceSave(src string) string {
	if !strings.Contains(src, "line-content") ||
		!strings.Contains(src, "&lt;") {
		return src
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(src))
	if err != nil {
		return src
	}
	var lines []string
	doc.Find("td.line-content").Each(func(_ int, td *goquery.Selection) {
		lines = append(lines, td.Text())
	})
	if len(lines) == 0 {
		return src
	}
	joined := strings.Join(lines, "\n")
	// 双重保险：还原后必须出现真实标签才算成功（防误判把正常页面拆坏）
	if unescaped := html.UnescapeString(joined); strings.Contains(unescaped, "<select") ||
		strings.Contains(unescaped, "<input") || strings.Contains(unescaped, "<form") {
		return unescaped
	}
	return src
}

// ParsePublishFormHTML 解析发布页 HTML → 配置草稿。
// currentFields（可选，nil=纯标准启发）：现行配置的 form_fields——
// §59.373 尊重配置槽位映射：已适配站（壹吧四槽媒介体系等 NP 变体）
// 的槽位特化以适配定案为权威，diff 只检值域漂移不产生槽位特化噪声
// （壹吧 medium→source_sel[4] 曾产生 13 条连锁假 diff）。
// 兼容性三保证：①current 名=标准启发名的站（幸运/修道院/龙之家）
// 匹配路径同结果零变化；②nil 走现状；③current 名在页面消失（站方
// 真改名）→ current 匹配不到 → fallback 标准启发 → diff 报字段名改版
// （真实改版检测路径保留）。
func ParsePublishFormHTML(html string, currentFields map[string]string) *model.PublishFormConfig {
	if strings.TrimSpace(html) == "" {
		return nil
	}
	html = unwrapViewSourceSave(html)

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

	// §59.388: 模式互斥控件过滤——主模式锚=配置字段名后缀众数
	// （聆音 form_fields 全 [4] 后缀→主模式 4；裸名站（五二）无后缀
	// →不过滤零变化）。控件 name 后缀 [N]≠主模式即跳过（聆音阅听
	// mode=8 槽 audiocodec_sel[8] 是电子书格式 select——域启发命中
	// audiocodec 域产生 5 条假 added：PDF/MOBI/EPUB/AZW3/Other，
	// 用户 243 Diff 实证+29 同栈复现）。壹吧 specialcat 是同 name
	// 双 type（无后缀）不受此过滤影响（special id 判别继续兜底）。
	mainMode := 0
	modeCounts := map[int]int{}
	for _, name := range currentFields {
		if m := fieldNameModeSuffix(name); m > 0 {
			modeCounts[m]++
		}
	}
	for m, c := range modeCounts {
		if c > modeCounts[mainMode] {
			mainMode = m
		}
	}

	// §59.373: current 反查表（字段名 → 域）——select/文本域识别的
	// 第一优先级。仅含"页面仍存在该字段名"的提示（消失的留标准启发
	// 抓新名保改版检测）。
	currentNameToDomain := map[string]string{}
	for domain, name := range currentFields {
		if name != "" {
			currentNameToDomain[name] = domain
		}
	}

	// 下拉族：select → option 列表
	doc.Find("select").Each(func(_ int, sel *goquery.Selection) {
		name, ok := sel.Attr("name")
		if !ok || name == "" {
			return
		}
		// §59.388: 非主模式控件跳过（聆音 audiocodec_sel[8] 阅听槽）
		if mainMode > 0 {
			if m := fieldNameModeSuffix(name); m > 0 && m != mainMode {
				return
			}
		}
		domain := domainOfField(name)
		// §59.373: current 配置字段名优先——已适配站槽位特化权威
		// （壹吧 medium 域配置 source_sel[4]：该 select 先匹配收录，
		// 标准 medium_sel[4] 后到被 seenDomains 守卫跳过——§59.372）。
		if cd, isCurrent := currentNameToDomain[name]; isCurrent {
			domain = cd
		}
		if domain == "" {
			return
		}
		// §59.372: 双区互斥站（壹吧 type×2——id=browsecat 种子区在前/
		// specialcat 成人区禁发布在后）——同 name 后匹配直接赋值会覆盖
		// 种子区（specialcat 16 项成人分类冒充表单现状→Diff 大量假
		// added/removed——用户上传 HTML Diff 实证）。双防：special 跳过
		// + 已收录域跳过（首见=种子区，HTML 顺序发布页先普通区）。
		if id, hasID := sel.Attr("id"); hasID && strings.Contains(id, "special") {
			return
		}
		if seenDomains[domain] {
			return
		}
		opts := []model.FormValueMapping{}
		seenOptLabels := map[string]bool{} // §59.377: 同 label 双值去重（五二 H.265 1/14、APE 2/5 站方原始重复——label 对齐二次匹配假 changed）
		sel.Find("option").Each(func(_ int, opt *goquery.Selection) {
			v, _ := opt.Attr("value")
			label := strings.TrimSpace(opt.Text())
			if v == "" || label == "" || v == "0" {
				return // 占位项（NP 家族 value=0 惯例——语言无关；§59.157 繁体「請選擇一項」漏过滤教训：
				// 占位判定不得依赖文案子串，简/繁/英占位词各异）
			}
			if seenOptLabels[label] {
				return // 站方原始重复值（同 label 多 value 语义相同）——首见保留
			}
			seenOptLabels[label] = true
			m := model.FormValueMapping{Label: label, Value: v}
			// §59.164: 词条补 StandardKeys（label→standard key 宽松反查）——
			// parse 产物缺此键则 executor lookupByStdKey 反查全空 → 五域全丢
			// → 站方"类型必填"拒绝（修道院首例实战暴露——幸运靠 migration 27
			// 手工映射掩盖了此缺口）
			if cat, ok := domainToCategory[domain]; ok {
				if sk := extract.LookupStandardKeyLoose(cat, label); sk != "" {
					m.StandardKeys = []string{sk}
				}
			}
			opts = append(opts, m)
		})
		if len(opts) == 0 {
			return
		}
		draft.FormFields[domain] = name
		draft.ValueMappings[domain] = opts
		seenDomains[domain] = true
	})

	// tags checkbox 族：input[type=checkbox][name^=tags]
	// §59.394: 模式互斥过滤扩展到 input——蟹黄堡 tags[4][] 与
	// tags[6][] 并存（双区互斥站），前缀匹配吃进 [6] 特别区标签
	// 23 条假 added（formdiff 门禁发版前抓到）。
	// §59.398: current 提示扩展到 tags 域——学校站标签字段名
	// span[]（非标准 tags 前缀——全库唯一），标准启发恒 miss 致
	// formdiff 假"整域消失"；current 字段名精确匹配兜底（§59.373
	// 三保证同构：tags 前缀命中的站匹配路径同结果零变化/current
	// 缺失走标准启发/current 名页面消失 fallback 标准启发照旧
	// miss → diff 可报改版）。
	currentTagName := ""
	if currentFields != nil {
		currentTagName = currentFields[model.FieldDomainTags]
	}
	isTagName := func(name string) bool {
		if strings.HasPrefix(name, "tags") {
			return true
		}
		return currentTagName != "" && name == currentTagName
	}
	tags := []model.FormValueMapping{}
	doc.Find("input[type='checkbox']").Each(func(_ int, cb *goquery.Selection) {
		name, _ := cb.Attr("name")
		if !isTagName(name) {
			return
		}
		if mainMode > 0 {
			if m := fieldNameModeSuffix(name); m > 0 && m != mainMode {
				return
			}
		}
		v, _ := cb.Attr("value")
		if v == "" {
			return
		}
		label := checkboxLabel(cb)
		if label == "" {
			label = v
		}
		tm := model.FormValueMapping{Label: label, Value: v}
		if sk := extract.LookupStandardKeyLoose("tag", label); sk != "" {
			tm.StandardKeys = []string{sk}
		}
		tags = append(tags, tm)
	})
	if len(tags) > 0 {
		// 字段名取实际 name（tags[4][] 本身即数组形态——原样保留）
		fieldName := ""
		doc.Find("input[type='checkbox']").EachWithBreak(func(_ int, cb *goquery.Selection) bool {
			n, _ := cb.Attr("name")
			if isTagName(n) {
				if m := fieldNameModeSuffix(n); m == 0 || m == mainMode || mainMode == 0 {
					fieldName = n
					return false
				}
			}
			return true
		})
		draft.FormFields[model.FieldDomainTags] = fieldName
		draft.ValueMappings[model.FieldDomainTags] = tags
		seenDomains[model.FieldDomainTags] = true
		// §59.269: 标签控件形态 → 提交模式（上传页表单结构事实派生——导入即
		// 完整配置。tags[4][]/span[] 数组形态=checkbox_span；修道院案根因：
		// 导入不产 TagConfig → executor 默认 taglist → 字段名硬编码 tagList
		// → 标签全丢）
		mode := model.TagModeTaglist
		if strings.Contains(fieldName, "[") {
			mode = model.TagModeCheckboxSpan
		}
		draft.TagConfig = &model.SiteTagConfig{Mode: mode, Tags: map[string]string{}}
	}

	// 文本域存在性（名称记录——供 diff 检测改版）
	// §59.159: pt_gen/dburl/uplver 必须收录——否则 HTML diff 误判"整域消失"，
	// 确认落库即丢配置（migration 30 注册的域与解析器对齐）
	textFieldDomains := map[string]string{
		"small_descr":    model.FieldDomainSmallDescr,
		"cnname":         model.FieldDomainCNName, // §59.164: 修道院独立中文名字段
		"url":            model.FieldDomainIMDBURL,
		"descr":          model.FieldDomainDescription,
		"technical_info": model.FieldDomainTechInfo,
		"pt_gen":         model.FieldDomainPTGen,
		"dburl":          model.FieldDomainDoubanURL,
	}
	// §59.373: current 配置的文本域字段名优先（壹吧简介 name=b——
	// 标准表无 b 恒 miss 致假"字段名改版"）；current 名页面存在才用。
	textDomainSet := map[string]bool{}
	for _, d := range textFieldDomains {
		textDomainSet[d] = true
	}
	for domain, name := range currentFields {
		if name == "" || !textDomainSet[domain] {
			continue
		}
		if doc.Find("input[name='"+name+"'], textarea[name='"+name+"']").Length() > 0 {
			draft.FormFields[domain] = name
			seenDomains[domain] = true
		}
	}
	for field, domain := range textFieldDomains {
		if seenDomains[domain] {
			continue // current 已收录（特化名）——标准名不再覆盖
		}
		if doc.Find("input[name='"+field+"'], textarea[name='"+field+"']").Length() > 0 {
			draft.FormFields[domain] = field
		}
	}
	// uplver（checkbox——存在性记录；语义由 form_config.Anonymous 承载）
	if doc.Find("input[type='checkbox'][name='uplver']").Length() > 0 {
		draft.FormFields[model.FieldDomainUplver] = "uplver"
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

	// 值级 diff：两级对齐（§59.378）
	// 第一级 label 精确对齐（原语义）；第二级 value 对齐（label miss 时——
	// 站方文案微调场景：keys/Auto 跟 value 走，kind="renamed" 轻信号）。
	// 三重保护：精配多键不塌缩 / label-only 特化继承（防松查乱挂键改变
	// 行为）/ 文案变化与真增删区分（removed 两级收紧——不再误报对）。
	for domain, draftVals := range draft.ValueMappings {
		curVals := indexByLabel(current.ValueMappings[domain])
		curByValue := indexByValue(current.ValueMappings[domain])
		mergedVals := make([]model.FormValueMapping, 0, len(draftVals))
		draftValueSeen := map[string]bool{}
		for _, dv := range draftVals {
			if cv, ok := curVals[dv.Label]; ok {
				// 第一级：label 精确命中
				inherited := model.FormValueMapping{Label: dv.Label, Value: dv.Value, StandardKeys: cv.StandardKeys, Auto: cv.Auto}
				mergedVals = append(mergedVals, inherited)
				draftValueSeen[dv.Value] = true
				if cv.Value == dv.Value {
					continue // matched——匹配项不进 diff（噪音抑制）
				}
				diffs = append(diffs, FormConfigDiffItem{ // changed：label 同 value 异（语义错位信号）
					Domain: domain, Kind: "changed", Label: dv.Label,
					CurrentValue: cv.Value, DraftValue: dv.Value,
					CurrentKeys: cv.StandardKeys,
				})
			} else if cv, ok := curByValue[dv.Value]; ok {
				// 第二级：value 命中（站方 label 文案微调）——keys/Auto 继承
				inherited := model.FormValueMapping{Label: dv.Label, Value: dv.Value, StandardKeys: cv.StandardKeys, Auto: cv.Auto}
				mergedVals = append(mergedVals, inherited)
				draftValueSeen[dv.Value] = true
				diffs = append(diffs, FormConfigDiffItem{
					Domain: domain, Kind: "renamed", Label: cv.Label + " → " + dv.Label,
					CurrentValue: cv.Value, DraftValue: dv.Value,
					CurrentKeys: cv.StandardKeys,
				})
			} else {
				// 真 新增选项：draft 松查 keys 过分叉族校验（extended 侧
				// 单键自动补 dict 侧双键——§59.376/§59.377 家族）
				added := dv
				added.StandardKeys = pairDivergentKeys(dv.StandardKeys)
				mergedVals = append(mergedVals, added)
				draftValueSeen[dv.Value] = true
				diffs = append(diffs, FormConfigDiffItem{
					Domain: domain, Kind: "added", Label: dv.Label, DraftValue: dv.Value,
				})
			}
		}
		merged.ValueMappings[domain] = mergedVals
		// 现配置有而草稿 label+value 均无（站方真删除——两级收紧：同 value
		// 不同 label 不再误报 removed+added 对——五二 8 条转录不全实证形态）
		draftIdx := indexByLabel(draftVals)
		for _, cv := range current.ValueMappings[domain] {
			_, labelHit := draftIdx[cv.Label]
			if !labelHit && !draftValueSeen[cv.Value] {
				diffs = append(diffs, FormConfigDiffItem{
					Domain: domain, Kind: "removed", Label: cv.Label, CurrentValue: cv.Value,
					CurrentKeys: cv.StandardKeys, AutoFalse: cv.Auto != nil && !*cv.Auto,
				})
			}
		}
	}
	return merged, diffs
}

// indexByValue §59.378: 按 value 建索引（两级对齐第二级——同 value 不同
// label 时 keys 继承；首见优先防站方重复值干扰）。
func indexByValue(vals []model.FormValueMapping) map[string]model.FormValueMapping {
	m := make(map[string]model.FormValueMapping, len(vals))
	for _, v := range vals {
		if _, dup := m[v.Value]; !dup {
			m[v.Value] = v
		}
	}
	return m
}

func indexByLabel(vals []model.FormValueMapping) map[string]model.FormValueMapping {
	m := make(map[string]model.FormValueMapping, len(vals))
	for _, v := range vals {
		m[v.Label] = v
	}
	return m
}

// domainToCategory §59.164: FormFields 域 → standard_keys 词表 category
// （parse 词条补 StandardKeys 用）。
var domainToCategory = map[string]string{
	model.FieldDomainType:       "type",
	model.FieldDomainMedium:     "medium",
	model.FieldDomainCodec:      "video_codec",
	model.FieldDomainAudiocodec: "audio_codec",
	model.FieldDomainStandard:   "resolution",
	model.FieldDomainTeam:       "team",
	model.FieldDomainTags:       "tag",
}

// fieldNameModeSuffix §59.388: 提取字段名的模式后缀（"audiocodec_sel[8]"→8；
// 裸名/无数字后缀返回 0）。NP 模式互斥站的控件名带 [N] 后缀指示归属
// 模式（data-mode='4' 影视 / 8 阅听等）。
func fieldNameModeSuffix(name string) int {
	m := reFieldNameMode.FindStringSubmatch(name)
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return n
}

// §59.394: 两种形态——select 尾部 "medium_sel[4]" 与 checkbox 中间
// "tags[6][]"（尾部还有 []）——取第一个数字括号。
var reFieldNameMode = regexp.MustCompile(`\[(\d+)\]`)
