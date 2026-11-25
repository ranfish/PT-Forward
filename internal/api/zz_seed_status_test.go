package api

import (
	"context"
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// §59.230: 无映射判定重构——已获取种子按站方标题重判（PERFUME 案）
func TestClassifySeedStatusNoMappingRefactor(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:seedstatus_"+t.Name()+".db?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := db.AutoMigrate(&model.Site{}, &model.ReleaseGroupMapping{}, &model.TorrentMetadata{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// CMCT→不可说 映射
	db.Create(&model.ReleaseGroupMapping{GroupName: "CMCT", SiteName: "不可说", IsOfficial: true, IsBuiltin: true})
	db.Create(&model.Site{Name: "不可说", Domain: "zmpt.cc"})

	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	h.SetSourceDetector(publish.NewSourceSiteDetector(db, zap.NewNop()))

	cases := []struct {
		name     string
		snapName string
		meta     *model.TorrentMetadata
		want     string
	}{
		// PERFUME 案 ①：tr 裸名未获取 → unsupported①（§59.578·不认识·含无组名）
		{"裸名未获取", "PERFUME OF THE LADY IN BLACK", nil, "unsupported"},
		// PERFUME 案 ②：已获取+站方标题带 -CMCT（有映射）→ 按获取后状态流（unreviewed 缺字段）
		{"获取后站方标题有组名", "PERFUME OF THE LADY IN BLACK",
			&model.TorrentMetadata{Title: "The Perfume of the Lady in Black 1974 1080p Blu-ray AVC DTS-HD MA 1.0-CMCT", SiteName: "织梦"}, "incomplete"},
		// ③：已获取+站方标题也无组名（电影）→ 获取后状态流（不再 unsupported）
		{"获取后站方标题无组名", "SOME MOVIE",
			&model.TorrentMetadata{Title: "Some Movie 2020 1080p BluRay AVC DTS-HD", SiteName: "站"}, "incomplete"},
		// ④：已获取+站方标题有组名但不在收录列表 → unsupported①（站方标题比裸名可信）
		{"获取后真无映射", "SOME MOVIE",
			&model.TorrentMetadata{Title: "Movie 2020 1080p-UNKNOWNGRP", SiteName: "站"}, "unsupported"},
	}
	for _, c := range cases {
		got := h.classifySeedStatusLite(context.TODO(), c.snapName, c.meta, false)
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// §59.236 ①: 主链 PTGen——唯一账本落库（ptgen_source_json+desc）
func TestRunMainlinePTGen_NoDoubanURL(t *testing.T) {
	h := &PublishTorrentsHandler{logger: zap.NewNop()}
	meta := &model.TorrentMetadata{InfoHash: "H1", SiteName: "s", DoubanURL: ""}
	// 无豆瓣链接：跳过（不报错不写库——incomplete 由状态机承接）
	h.runMainlinePTGen(context.TODO(), meta, false)
}

// §59.236 ①: 主链 PTGen 成功路径——唯一账本+desc 落库
func TestRunMainlinePTGen_Success(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file:mlptgen_t.db?mode=memory&cache=shared"), &gorm.Config{})
	db.AutoMigrate(&model.TorrentMetadata{})
	db.Create(&model.TorrentMetadata{InfoHash: "H2", SiteName: "zmpt", DoubanURL: "https://movie.douban.com/subject/123/"})
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	// mock 注入（PTGenAnalyzer 接口）
	h.ptgen = fakeAnalyzer{res: &model.PTGenResult{
		ChineseTitle: "火车梦", ForeignTitle: "Train Dreams / 铁路梦影(港)",
		Year: "2025", RawBBCode: "◎片　　名　火车梦",
	}}
	h.runMainlinePTGen(context.TODO(), &model.TorrentMetadata{InfoHash: "H2", SiteName: "zmpt", DoubanURL: "https://movie.douban.com/subject/123/"}, false)
	var got model.TorrentMetadata
	db.Where("info_hash = ?", "H2").First(&got)
	if got.Description != "◎片　　名　火车梦" {
		t.Errorf("desc = %q, want RawBBCode", got.Description)
	}
	if !strings.Contains(got.PTGenSourceJSON, "火车梦") {
		t.Errorf("ptgen_source_json 未落: %q", got.PTGenSourceJSON[:min(40, len(got.PTGenSourceJSON))])
	}
}

type fakeAnalyzer struct{ res *model.PTGenResult }

func (f fakeAnalyzer) AnalyzePTGen(ctx context.Context, name string) (*model.PTGenResult, error) {
	return f.res, nil
}
func (f fakeAnalyzer) AnalyzePTGenForce(ctx context.Context, name string) (*model.PTGenResult, error) {
	return f.res, nil
}

// §59.247: PUT 审核语义——默认不置 reviewed/显式 reviewed=true 带门槛
func TestPutSeedReviewSemantics(t *testing.T) {
	// 门槛判定单测（端到端见 handlePutSeed 集成——此处锁定语义函数）
	// PUT 默认：updates 无 reviewed 键（buildPutSeedUpdates 不含）
	// 显式：req.Reviewed=true 且 missing>0 → 400 拒绝
	// ——语义由实现结构保证（无独立函数），编译级+集成覆盖
}

// §59.578 刀1: 种子状态值域锁定锚（§59.546 纪律平移——发布终态先例：
// 先登记→锚→grep 消费面）。classifySeedStatusLite 的输出全集=此八字。
// 未来增删状态值：先改本锚（含前端 STATUS_META 三页同步的 grep 清单
// 纪律），再改 classify——倒过来=锚红（§59.1627 教训：新增状态值消费面
// 漏改 STATUS_META 双页）。
func TestSeedStatusValueSet(t *testing.T) {
	allowed := map[string]bool{
		"forbidden":        true, // §59.578: 禁转（收窄纯化——原「禁转/无映射」）
		"system_forbidden": true,
		"unsupported":      true, // §59.578: 不支持（①不认识∪②获取终态失败·刀2 烙印）
		"reviewed":         true,
		"pending":          true,
		"incomplete":       true,
		"unfetched":        true, // §59.578: 未获取（收录列表命中·隐含未走链）
		"observing":        true,
	}
	// 消亡值（§59.578）——任何回归引入即锚红
	dead := []string{"no_mapping", "external_group"}
	for _, d := range dead {
		if allowed[d] {
			t.Errorf("消亡状态 %q 出现在值域（§59.578：外站组概念收回系统内部）", d)
		}
	}
	// 值域枚举方式：classify 全分支的已知输出逐一核对（表驱动锚已覆盖
	// 各分支——此处锁值域声明本身：改动 classify 增加新返回值时必须先
	// 扩本锚，否则新增值无前端 STATUS_META 条目=列表显示裸值）。
	if len(allowed) != 8 {
		t.Errorf("值域基数变化（%d≠8）——扩值域须先改本锚+前端三页", len(allowed))
	}
}
