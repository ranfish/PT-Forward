package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.269: 导入器标签形态派生——tags[4][] 数组 → checkbox_span（修道院案根因）
func TestParsePublishFormHTML_TagMode(t *testing.T) {
	html := `<html><body><form>
	<select name="type"><option value="401">电影</option></select>
	<select name="codec_sel[4]"><option value="1">H.264</option></select>
	<input type="checkbox" name="tags[4][]" value="5">国语
	<input type="checkbox" name="tags[4][]" value="6">中字
	<textarea name="descr"></textarea>
	</form></body></html>`
	cfg := ParsePublishFormHTML(html)
	if cfg == nil {
		t.Fatal("draft nil")
	}
	if cfg.TagConfig == nil {
		t.Fatal("TagConfig 未派生（修道院案根因）")
	}
	if cfg.TagConfig.Mode != model.TagModeCheckboxSpan {
		t.Errorf("mode=%q want checkbox_span", cfg.TagConfig.Mode)
	}
	if cfg.FormFields[model.FieldDomainTags] != "tags[4][]" {
		t.Errorf("tags field: %q", cfg.FormFields[model.FieldDomainTags])
	}
}
