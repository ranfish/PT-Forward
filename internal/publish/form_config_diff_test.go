package publish

import (
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.156 切片 3: HTML 解析+合并继承+diff 三分类——幸运实测形态（§59.149）锚定。
const luckHTML = `
<html><body><form action="takeupload.php" method="post">
<input type="text" name="name" />
<input type="text" name="small_descr" />
<input type="text" name="url" />
<input type="text" name="pt_gen" />
<input type="text" name="dburl" />
<input type="checkbox" name="uplver" value="yes" />
<textarea name="descr"></textarea>
<textarea name="technical_info"></textarea>
<select name="type">
  <option value="0">请选择</option>
  <option value="0">請選擇一項</option>
  <option value="401">电影</option>
  <option value="402">电视剧</option>
  <option value="413">短剧</option>
</select>
<select name="medium_sel[4]">
  <option value="7">Encode</option>
  <option value="11">WEB-DL</option>
  <option value="14">Vinyl</option>
</select>
<select name="codec_sel[4]">
  <option value="1">H.264/AVC</option>
  <option value="6">H.265/HEVC</option>
</select>
<input type="checkbox" name="tags[4][]" value="20" /><label>Dolby Vision</label>
<input type="checkbox" name="tags[4][]" value="19" /><label>HDR10</label>
<input type="checkbox" name="tags[4][]" value="22" /><label>英语</label>
<input type="checkbox" name="tags[4][]" value="2" /><label>首发</label>
</form></body></html>`

func currentLuckCfg() *model.PublishFormConfig {
	no := false
	return &model.PublishFormConfig{
		Enabled:     true,
		PreAuditURL: "/api/auto-audit/pre-audit",
		FormFields: map[string]string{
			model.FieldDomainType:   "type",
			model.FieldDomainMedium: "medium_sel[4]",
			model.FieldDomainTags:   "tags[4][]",
		},
		ValueMappings: map[string][]model.FormValueMapping{
			model.FieldDomainType: {
				{Label: "电影", Value: "401", StandardKeys: []string{"category.movie"}},
				{Label: "电视剧", Value: "402", StandardKeys: []string{"category.tv_series"}},
			},
			model.FieldDomainMedium: {
				{Label: "Encode", Value: "7", StandardKeys: []string{"medium.encode"}},
				{Label: "WEB-DL", Value: "11", StandardKeys: []string{"medium.webdl"}},
				{Label: "Blu-ray", Value: "1", StandardKeys: []string{"medium.bluray"}},
			},
			model.FieldDomainTags: {
				{Label: "Dolby Vision", Value: "20", StandardKeys: []string{"tag.dolby_vision"}},
				{Label: "首发", Value: "2", Auto: &no},
			},
		},
	}
}

func TestParsePublishFormHTML(t *testing.T) {
	draft := ParsePublishFormHTML(luckHTML, nil)
	if draft == nil {
		t.Fatal("应解析出草稿")
	}
	// 字段名（含 [N] 后缀与 tags[] 形态）
	if draft.FormFields[model.FieldDomainType] != "type" {
		t.Errorf("type 字段名: %v", draft.FormFields[model.FieldDomainType])
	}
	if draft.FormFields[model.FieldDomainMedium] != "medium_sel[4]" {
		t.Errorf("medium 字段名应含后缀: %v", draft.FormFields[model.FieldDomainMedium])
	}
	if draft.FormFields[model.FieldDomainTags] != "tags[4][]" {
		t.Errorf("tags 字段名: %v", draft.FormFields[model.FieldDomainTags])
	}
	// 占位项过滤（value=0 请选择）
	for _, m := range draft.ValueMappings[model.FieldDomainType] {
		if m.Value == "0" {
			t.Error("占位项应被过滤")
		}
	}
	if len(draft.ValueMappings[model.FieldDomainType]) != 3 {
		t.Errorf("type 域应 3 项（含短剧）, got %d", len(draft.ValueMappings[model.FieldDomainType]))
	}
	// tags checkbox 4 项
	if len(draft.ValueMappings[model.FieldDomainTags]) != 4 {
		t.Errorf("tags 应 4 项, got %d", len(draft.ValueMappings[model.FieldDomainTags]))
	}
	// §59.159: pt_gen/dburl/uplver 存在性（migration 30 域与解析器对齐）
	for _, domain := range []string{model.FieldDomainPTGen, model.FieldDomainDoubanURL, model.FieldDomainUplver} {
		if draft.FormFields[domain] == "" {
			t.Errorf("%s 域应被解析收录（防 HTML diff 误判整域消失）", domain)
		}
	}
	// HTML 即弃（返回值无原文）
	if strings.Contains(draft.Serialize(), "takeupload") {
		t.Error("草稿不应含 HTML 原文")
	}
	// 空/垃圾 HTML
	if ParsePublishFormHTML("", nil) != nil {
		t.Error("空 HTML 应返回 nil")
	}
	if ParsePublishFormHTML("<div>no form</div>", nil) != nil {
		t.Error("无表单 HTML 应返回 nil")
	}
}

func TestMergeDraftWithCurrent(t *testing.T) {
	current := currentLuckCfg()
	draft := ParsePublishFormHTML(luckHTML, nil)
	merged, diffs := MergeDraftWithCurrent(current, draft)

	// ① 继承：电影→401 应继承 category.movie
	mv := merged.ValueMappings[model.FieldDomainType]
	found := false
	for _, m := range mv {
		if m.Label == "电影" {
			found = true
			if m.Value != "401" || len(m.StandardKeys) != 1 || m.StandardKeys[0] != "category.movie" {
				t.Errorf("电影应继承语义: %+v", m)
			}
		}
	}
	if !found {
		t.Fatal("merged 应含电影")
	}
	// ② auto:false 继承（首发）
	for _, m := range merged.ValueMappings[model.FieldDomainTags] {
		if m.Label == "首发" && (m.Auto == nil || *m.Auto) {
			t.Error("首发应继承 auto:false")
		}
	}
	// ③ 新增（added）：短剧/Vinyl/HDR10/英语——待标注
	added := 0
	for _, d := range diffs {
		if d.Kind == "added" {
			added++
		}
	}
	if added < 4 {
		t.Errorf("新增项应 ≥4（短剧/Vinyl/HDR10/英语）, got %d: %+v", added, diffs)
	}
	// ④ 删除（removed）：Blu-ray
	removedBlu := false
	for _, d := range diffs {
		if d.Kind == "removed" && d.Label == "Blu-ray" {
			removedBlu = true
		}
	}
	if !removedBlu {
		t.Error("Blu-ray 应标记 removed")
	}
	// ⑤ pre_audit_url 继承（非站方信息）
	if merged.PreAuditURL != "/api/auto-audit/pre-audit" {
		t.Errorf("pre_audit_url 应继承: %v", merged.PreAuditURL)
	}
	// ⑥ matched 不进 diff（基线校准噪音抑制）
	for _, d := range diffs {
		if d.Kind == "matched" {
			t.Error("matched 项不应出现在 diff 清单")
		}
	}
}

// §59.164: parse 词条补 StandardKeys（label 宽松反查——修道院实战暴露）
func TestParseStandardKeysEnrich(t *testing.T) {
	html := `<form>
	<select name="type"><option value="0">请选择</option><option value="401">电影（Movies）</option></select>
	<select name="standard_sel[4]"><option value="8">4K/2160i/2160P</option><option value="6">1080i/1080P</option></select>
	<select name="team_sel[4]"><option value="25">FRDS</option></select>
	<select name="medium_sel[4]"><option value="7">Encode</option></select>
	</form>`
	draft := ParsePublishFormHTML(html, nil)
	lookup := func(domain, wantCode string) {
		t.Helper()
		for _, m := range draft.ValueMappings[domain] {
			for _, k := range m.StandardKeys {
				if k == wantCode {
					return
				}
			}
		}
		t.Errorf("%s 词条未补到 StandardKeys %q（含 %v）", domain, wantCode, draft.ValueMappings[domain])
	}
	lookup("type", "category.movie")
	lookup("standard", "resolution.r2160p") // 大小写不敏感（4K/2160i/2160P）
	lookup("team", "team.frds")
	lookup("medium", "medium.encode")
}

// §59.388: 模式互斥控件过滤锚——聆音双模式站（mode=4 影视+mode=8 阅听）
// 阅听槽 audiocodec_sel[8]（电子书格式 PDF/MOBI/EPUB/AZW3）被域启发
// 认成 audiocodec 域 → 5 条假 added（用户 243 Diff 实证）。三断言：
// ① [8] 槽不进 draft ② [4] 槽正常收录 ③ 裸名站（无后缀配置）不过滤。
func TestParsePublishFormHTMLModeExclusiveSelects(t *testing.T) {
	html := `<form action="takeupload.php">
	<select name="type" id="browsecat" data-mode='4'><option value="401">电影</option></select>
	<select name="type" id="specialcat" data-mode='8'><option value="499">有声书</option></select>
	<select name="medium_sel[4]" data-mode="medium_4"><option value="7">Encode</option></select>
	<select name="audiocodec_sel[8]" data-mode="audiocodec_8"><option value="2">EPUB</option><option value="18">PDF</option></select>
	</form>`

	// ① 聆音形态：配置 [4] 后缀 → 主模式 4 → [8] 槽跳过
	fields := map[string]string{"type": "type", "medium": "medium_sel[4]", "team": "team_sel[4]"}
	draft := ParsePublishFormHTML(html, fields)
	if draft == nil {
		t.Fatal("解析 nil")
	}
	if _, has := draft.ValueMappings[model.FieldDomainAudiocodec]; has {
		t.Fatalf("① [8] 阅听槽不应进 draft（audiocodec 域被收录=%v）", draft.ValueMappings[model.FieldDomainAudiocodec])
	}
	if got := draft.ValueMappings[model.FieldDomainMedium]; len(got) != 1 || got[0].Value != "7" {
		t.Fatalf("② [4] 主模式槽应正常收录，got %v", got)
	}

	// ③ 裸名站（五二形态——配置无后缀）→ 不过滤（audiocodec_sel[8]
	// 若被域启发收录是既有行为，零变化保证）
	bare := map[string]string{"type": "type", "medium": "medium_sel"}
	draft2 := ParsePublishFormHTML(html, bare)
	if draft2 == nil {
		t.Fatal("③ 裸名站解析 nil")
	}
	if _, has := draft2.ValueMappings[model.FieldDomainMedium]; !has {
		t.Fatal("③ 裸名站 medium_sel[4] 应收录（无主模式不过滤）")
	}
}

// §59.394: 模式互斥过滤扩展到 input 锚——蟹黄堡双区站 tags[4][] 与
// tags[6][] 并存，前缀匹配吃进 [6] 特别区标签（23 条假 added——
// formdiff 门禁发版前抓到）。
func TestParsePublishFormHTMLModeExclusiveCheckbox(t *testing.T) {
	html := `<form action="takeupload.php">
	<select name="medium_sel[4]"><option value="7">Encode</option></select>
	<input type="checkbox" name="tags[4][]" value="1">禁转
	<input type="checkbox" name="tags[6][]" value="53">历史军事
	<input type="checkbox" name="tags[6][]" value="54">轻小说
	</form>`
	fields := map[string]string{"medium": "medium_sel[4]", "tags": "tags[4][]"}
	draft := ParsePublishFormHTML(html, fields)
	if draft == nil {
		t.Fatal("解析 nil")
	}
	tags := draft.ValueMappings[model.FieldDomainTags]
	if len(tags) != 1 || tags[0].Value != "1" {
		t.Fatalf("[6] 特别区标签应被过滤，got %v", tags)
	}
	if draft.FormFields[model.FieldDomainTags] != "tags[4][]" {
		t.Fatalf("字段名应为 tags[4][]（主模式），got %q", draft.FormFields[model.FieldDomainTags])
	}
}

// §59.398: tags 域 current 提示——学校站标签字段名 span[]（非标准
// tags 前缀，全库唯一）。三保证锚：①current 提示命中 span[] ②无
// current 走标准启发照旧 miss（真改名 diff 可报）③标准 tags 前缀站
// 行为零变化（current 名=标准名同路径）。
func TestParsePublishFormHTMLSpanTagsCurrentHint(t *testing.T) {
	html := `<form action="takeupload.php">
	<select name="medium_sel"><option value="7">Encode</option></select>
	<input type="checkbox" name="span[]" value="1">禁转
	<input type="checkbox" name="span[]" value="2">首发
	<input type="checkbox" name="span[]" value="6">中字
	</form>`
	// ① current 提示：tags=span[] → 识别 3 项+字段名 span[]
	draft := ParsePublishFormHTML(html, map[string]string{"medium": "medium_sel", "tags": "span[]"})
	if draft == nil {
		t.Fatal("解析 nil")
	}
	tags := draft.ValueMappings[model.FieldDomainTags]
	if len(tags) != 3 || tags[0].Value != "1" || tags[2].Value != "6" {
		t.Fatalf("current 提示应识别 span[] 3 项, got %v", tags)
	}
	if tags[0].Label != "禁转" || tags[2].Label != "中字" {
		t.Fatalf("裸文本后邻形态 label 应提取（§59.398 形态4）, got %v", tags)
	}
	if draft.FormFields[model.FieldDomainTags] != "span[]" {
		t.Fatalf("字段名应为 span[], got %q", draft.FormFields[model.FieldDomainTags])
	}
	if draft.TagConfig == nil || draft.TagConfig.Mode != model.TagModeCheckboxSpan {
		t.Fatalf("span[] 数组形态应派生 checkbox_span 模式, got %+v", draft.TagConfig)
	}
	// ② 无 current 提示（nil）：标准启发照旧 miss——真改名 diff 可报
	draft2 := ParsePublishFormHTML(html, nil)
	if _, ok := draft2.ValueMappings[model.FieldDomainTags]; ok {
		t.Fatal("无 current 提示时 span[] 不应被标准启发识别")
	}
	// ③ 标准 tags 前缀站 + current 名=标准名：同路径零变化
	htmlStd := `<form action="takeupload.php">
	<input type="checkbox" name="tags[4][]" value="1">禁转
	</form>`
	draft3 := ParsePublishFormHTML(htmlStd, map[string]string{"tags": "tags[4][]"})
	if len(draft3.ValueMappings[model.FieldDomainTags]) != 1 {
		t.Fatal("标准 tags 前缀站行为应零变化")
	}
}
