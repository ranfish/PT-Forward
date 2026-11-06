package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.471: HDVideo 地区/风格映射锚——canonical 路由+港台双键+miss 兜底。
func TestRegionStyleMapping(t *testing.T) {
	cfg := &model.PublishFormConfig{
		ValueMappings: map[string][]model.FormValueMapping{
			model.FieldDomainRegion: {
				{Label: "中国大陆", Value: "1", StandardKeys: []string{"region.cn"}},
				{Label: "中国港台", Value: "6", StandardKeys: []string{"region.hk", "region.tw"}},
				{Label: "美国", Value: "2", StandardKeys: []string{"region.us"}},
				{Label: "北欧", Value: "27"},
				{Label: "其他地区", Value: "28"},
			},
			model.FieldDomainStyle: {
				{Label: "喜剧", Value: "2", StandardKeys: []string{"genre.comedy"}},
				{Label: "剧情", Value: "6", StandardKeys: []string{"genre.drama"}},
				{Label: "竞技", Value: "26"},
			},
		},
	}
	e := &PublishExecutor{}
	ptgenUS := &model.TorrentMetadata{
		PTGenSourceJSON: `{"region":["美国"],"genre":["喜剧","爱情"]}`,
	}
	ptgenHK := &model.TorrentMetadata{
		PTGenSourceJSON: `{"region":["中国香港"],"genre":["剧情"]}`,
	}
	ptgenNone := &model.TorrentMetadata{
		PTGenSourceJSON: `{"region":["阿根廷"],"genre":["西部"]}`,
	}

	// 美国+喜剧：canonical 直中
	if m := e.regionMappingOf(cfg, "HDVideo", ptgenUS); m == nil || m.Value != "2" {
		t.Errorf("region 美国 = %v, want value 2", m)
	}
	if m := e.styleMappingOf(cfg, "HDVideo", ptgenUS); m == nil || m.Value != "2" {
		t.Errorf("style 喜剧 = %v, want value 2", m)
	}
	// 港台双键（region.hk → 站方 6）
	if m := e.regionMappingOf(cfg, "HDVideo", ptgenHK); m == nil || m.Value != "6" {
		t.Errorf("region 港台 = %v, want value 6（hk+tw 双键）", m)
	}
	// miss 兜底：region→28 其他地区 / style→6 剧情
	if m := e.regionMappingOf(cfg, "HDVideo", ptgenNone); m == nil || m.Value != "28" {
		t.Errorf("region 阿根廷 miss = %v, want value 28 兜底", m)
	}
	if m := e.styleMappingOf(cfg, "HDVideo", ptgenNone); m == nil || m.Value != "6" {
		t.Errorf("style 西部 miss = %v, want value 6 兜底", m)
	}
	// 未配置域 nil 跳过（其它站零影响）
	emptyCfg := &model.PublishFormConfig{}
	if m := e.regionMappingOf(emptyCfg, "其它站", ptgenUS); m != nil {
		t.Errorf("未配置 region 域应 nil, got %v", m)
	}
	if m := e.styleMappingOf(emptyCfg, "其它站", ptgenUS); m != nil {
		t.Errorf("未配置 style 域应 nil, got %v", m)
	}
	// 无 PTGen 数据：直接兜底
	noPtgen := &model.TorrentMetadata{}
	if m := e.regionMappingOf(cfg, "HDVideo", noPtgen); m == nil || m.Value != "28" {
		t.Errorf("无 PTGen region = %v, want 28 兜底", m)
	}
	if m := e.styleMappingOf(cfg, "HDVideo", noPtgen); m == nil || m.Value != "6" {
		t.Errorf("无 PTGen style = %v, want 6 兜底", m)
	}
}
