package metadata

import "testing"

// §59.34 审计二轮: Merge Type/Source/ReleaseGroup 归一化——
// detail 提取器存 standard key，preview 显示需规范显示名；
// team.* 无映射表保留 raw（空值比 raw 更糟）。
func TestMergeTypeSourceReleaseGroupDisplay(t *testing.T) {
	d := makeDetail("t", "b")
	d.Type = "category.movie"
	d.Source = "source.uk"
	d.ReleaseGroup = "team.hhweb"
	m := Merge(d, nil, nil, MergeModePTGenFirst)
	if m.Type != "电影" {
		t.Errorf("Type = %q, want 电影", m.Type)
	}
	if m.Source != "英国" {
		t.Errorf("Source = %q, want 英国", m.Source)
	}
	if m.ReleaseGroup != "team.hhweb" {
		t.Errorf("ReleaseGroup = %q, want raw team.hhweb（无映射表保留 raw）", m.ReleaseGroup)
	}
}
