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
<textarea name="descr"></textarea>
<textarea name="technical_info"></textarea>
<select name="type">
  <option value="0">请选择</option>
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
	draft := ParsePublishFormHTML(luckHTML)
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
	// HTML 即弃（返回值无原文）
	if strings.Contains(draft.Serialize(), "takeupload") {
		t.Error("草稿不应含 HTML 原文")
	}
	// 空/垃圾 HTML
	if ParsePublishFormHTML("") != nil {
		t.Error("空 HTML 应返回 nil")
	}
	if ParsePublishFormHTML("<div>no form</div>") != nil {
		t.Error("无表单 HTML 应返回 nil")
	}
}

func TestMergeDraftWithCurrent(t *testing.T) {
	current := currentLuckCfg()
	draft := ParsePublishFormHTML(luckHTML)
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
