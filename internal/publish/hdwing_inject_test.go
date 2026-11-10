package publish

// §59.511 瓷器发布适配锚——type 混维路由+官组回流规则⑥+固定注入。
import (
	"context"
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func hdwingTestCfg() *model.PublishFormConfig {
	cfg := &model.PublishFormConfig{FormFields: map[string]string{
		"type": "type", "share_rule": "share_rule", "board": "board", "is3d": "is3d",
	}}
	cfg.ValueMappings = map[string][]model.FormValueMapping{
		model.FieldDomainType: {
			{Label: "原盘(Full BD)", Value: "20"},
			{Label: "电影Movie(1080p)", Value: "17"},
			{Label: "电影Movie(1080i)", Value: "16"},
			{Label: "电影Movie(720p)", Value: "9"},
			{Label: "欧美剧", Value: "13"},
			{Label: "欧美剧集包", Value: "21"},
			{Label: "中港台剧集", Value: "25"},
			{Label: "中港台剧集包", Value: "22"},
			{Label: "日剧", Value: "24"},
			{Label: "日韩剧集包", Value: "23"},
			{Label: "韩剧", Value: "26"},
			{Label: "动画片", Value: "14", StandardKeys: []string{"category.animation"}},
			{Label: "4K UltraHD", Value: "410"},
		},
		model.FieldDomainMedium: {
			{Label: "Blu-ray", Value: "11", StandardKeys: []string{"medium.bluray"}},
			{Label: "Encode", Value: "5", StandardKeys: []string{"medium.encode"}},
			{Label: "WEB-DL", Value: "21", StandardKeys: []string{"medium.webdl"}},
		},
	}
	return cfg
}

func TestHdwingCategoryValue(t *testing.T) {
	e := &PublishExecutor{}
	cfg := hdwingTestCfg()

	cases := []struct {
		name string
		meta *model.TorrentMetadata
		tp   titleparser.TechProfile
		want string
	}{
		// ① 4K 大桶：任意媒介/类型 → 410（分辨率优先）
		{"4K原盘电影", &model.TorrentMetadata{Category: "category.movie"}, titleparser.TechProfile{Resolution: "2160p"}, "410"},
		{"4K压制电影", &model.TorrentMetadata{Category: "category.movie"}, titleparser.TechProfile{Resolution: "4320p"}, "410"},
		// ② 电影层：媒介分桶（medium 由 tp.SourceType 判——此处直验 tp.Resolution 路径）
		{"1080i电影", &model.TorrentMetadata{Category: "category.movie"}, titleparser.TechProfile{Resolution: "1080i"}, "16"},
		{"720p电影", &model.TorrentMetadata{Category: "category.movie"}, titleparser.TechProfile{Resolution: "720p"}, "9"},
		// ②b 电影原盘分桶（medium 判定链→bluray 族→20）
		{"1080p原盘电影", &model.TorrentMetadata{Category: "category.movie", Title: "Oldboy 2003 1080p JPN Blu-ray AVC DTS-HD MA-FRDS"}, titleparser.TechProfile{Resolution: "1080p", SourceType: "Blu-ray"}, "20"},
		// ③ 剧集产区轴（PTGen Region）
		{"美剧", &model.TorrentMetadata{Category: "category.tv_series", Title: "Lost S02 1080p", PTGenSourceJSON: `{"region":["美国"]}`}, titleparser.TechProfile{Resolution: "1080p"}, "13"},
		{"日剧", &model.TorrentMetadata{Category: "category.tv_series", Title: "X S01", PTGenSourceJSON: `{"region":["日本"]}`}, titleparser.TechProfile{}, "24"},
		{"韩剧", &model.TorrentMetadata{Category: "category.tv_series", Title: "X S01", PTGenSourceJSON: `{"region":["韩国"]}`}, titleparser.TechProfile{}, "26"},
		{"国产剧", &model.TorrentMetadata{Category: "category.tv_series", Title: "X S01", PTGenSourceJSON: `{"region":["中国大陆"]}`}, titleparser.TechProfile{}, "25"},
		{"多季包", &model.TorrentMetadata{Category: "category.tv_series", Title: "Westworld S01-S04 COMPLETE", PTGenSourceJSON: `{"region":["美国"]}`}, titleparser.TechProfile{}, "21"},
		{"产区缺失兜底欧美", &model.TorrentMetadata{Category: "category.tv_series", Title: "X S01"}, titleparser.TechProfile{}, "13"},
	}
	for _, c := range cases {
		got := e.hdwingCategoryValue(cfg, "瓷器", c.meta, c.tp)
		if got == nil || got.Value != c.want {
			t.Errorf("%s: want %s got %+v", c.name, c.want, got)
		}
	}
	// ④ 直映射类（animation/documentary/sports/tv_shows/concert/mv/music）
	// hdwingCategoryValue 返回 nil 回退通用链（typeMappingOf 末尾
	// lookupByStdKey 消费 value_mappings 的 standard_keys→动画片 14）
	anim := &model.TorrentMetadata{Category: "category.animation", Title: "Kaiji S01"}
	if got := e.hdwingCategoryValue(cfg, "瓷器", anim, titleparser.TechProfile{}); got != nil {
		t.Errorf("直映射类应回退通用链（nil），got %+v", got)
	}
	if got := e.typeMappingOf(cfg, "瓷器", anim, titleparser.TechProfile{}); got == nil || got.Value != "14" {
		t.Errorf("通用链 animation→14 失败: %+v", got)
	}
	// 非瓷器站 nil（路由不越权）
	if e.hdwingCategoryValue(cfg, "天空", cases[0].meta, cases[0].tp) != nil {
		t.Error("非瓷器站应返回 nil")
	}
}

func TestHdwingPublishInject(t *testing.T) {
	cfg := hdwingTestCfg()
	form := map[string]string{}
	hdwingPublishInject(form, cfg, nil, titleparser.TechProfile{})
	if form["share_rule"] != "3" {
		t.Errorf("share_rule 恒 3（欢迎转载·权利链定案），got %q", form["share_rule"])
	}
	if form["board"] != "1" {
		t.Errorf("board 恒 1（普通区），got %q", form["board"])
	}
	if _, ok := form["is3d"]; ok {
		t.Error("非 3D 不勾 is3d")
	}
	form2 := map[string]string{}
	hdwingPublishInject(form2, cfg, nil, titleparser.TechProfile{Stereo3D: "HSBS"})
	if form2["is3d"] != "yes" {
		t.Error("Stereo3D 在场应勾 is3d=yes")
	}
}

// §59.511 规则⑥：官组种禁回流源站（全站通用——组→mappings(is_official=1)
// 归属站==目标站→拦截）。
func TestOfficialGroupBackflow(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TorrentMetadata{}, &model.ReleaseGroupMapping{}); err != nil {
		t.Fatal(err)
	}
	// 官组资产：HDCTV→瓷器 ｜ FRDS→朋友（真实归属）
	for _, m := range []model.ReleaseGroupMapping{
		{GroupName: "HDCTV", Domain: "hdwing.org", SiteName: "瓷器", IsOfficial: true},
		{GroupName: "FRDS", Domain: "pt.keepfrds.com", SiteName: "朋友", IsOfficial: true},
	} {
		if err := db.Create(&m).Error; err != nil {
			t.Fatal(err)
		}
	}
	e := &PublishExecutor{db: db}
	ctx := context.Background()

	// ① HDCTV 组种 → 瓷器：拦（回流）
	hd := &model.TorrentMetadata{Title: "Film 2024 1080p BluRay DTS-HD MA 5.1-HDCTV"}
	if blocked, reason := e.checkOfficialGroupBackflow(ctx, hd, "瓷器"); !blocked {
		t.Fatal("HDCTV 官组种 → 瓷器 应拦（回流）")
	} else if !strings.Contains(reason, "回流") {
		t.Fatalf("拦截文案缺回流标识: %s", reason)
	}
	// ② HDCTV 组种 → 其它站：放（非归属站）
	if blocked, _ := e.checkOfficialGroupBackflow(ctx, hd, "天空"); blocked {
		t.Fatal("HDCTV 种 → 天空 不拦（非归属站）")
	}
	// ③ FRDS 组种 → 朋友：拦（全站通用验证）
	fr := &model.TorrentMetadata{Title: "Lost S02 1080p BluRay x264-FRDS"}
	if blocked, _ := e.checkOfficialGroupBackflow(ctx, fr, "朋友"); !blocked {
		t.Fatal("FRDS 种 → 朋友 应拦（全站通用）")
	}
	// ④ 无组种：放
	if blocked, _ := e.checkOfficialGroupBackflow(ctx, &model.TorrentMetadata{Title: "No Group Movie 2024"}, "瓷器"); blocked {
		t.Fatal("无组种不拦")
	}
}
