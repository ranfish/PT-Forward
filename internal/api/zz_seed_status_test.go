package api

import (
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
		name    string
		snapName string
		meta    *model.TorrentMetadata
		want    string
	}{
		// PERFUME 案 ①：tr 裸名未获取 → no_mapping（获取通道提醒）
		{"裸名未获取", "PERFUME OF THE LADY IN BLACK", nil, "no_mapping"},
		// PERFUME 案 ②：已获取+站方标题带 -CMCT（有映射）→ 按获取后状态流（unreviewed 缺字段）
		{"获取后站方标题有组名", "PERFUME OF THE LADY IN BLACK",
			&model.TorrentMetadata{Title: "The Perfume of the Lady in Black 1974 1080p Blu-ray AVC DTS-HD MA 1.0-CMCT", SiteName: "织梦"}, "incomplete"},
		// ③：已获取+站方标题也无组名（电影）→ 获取后状态流（不再 no_mapping）
		{"获取后站方标题无组名", "SOME MOVIE",
			&model.TorrentMetadata{Title: "Some Movie 2020 1080p BluRay AVC DTS-HD", SiteName: "站"}, "incomplete"},
		// ④：已获取+站方标题有组名但真无映射 → no_mapping（站方标题比裸名可信）
		{"获取后真无映射", "SOME MOVIE",
			&model.TorrentMetadata{Title: "Movie 2020 1080p-UNKNOWNGRP", SiteName: "站"}, "no_mapping"},
	}
	for _, c := range cases {
		got := h.classifySeedStatusLite(nil, c.snapName, c.meta)
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// §59.235 P2: PTGen 资产后置提取（◎行→四列——§59.168 断链重建）
func TestExtractPTGenAssets(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file:ptgen_assets_test.db?mode=memory&cache=shared"), &gorm.Config{})
	db.AutoMigrate(&model.TorrentMetadata{})
	meta := &model.TorrentMetadata{InfoHash: "H1", Description: "[img]x[/img]\n\n◎片　　名　刘老庄八十二壮士\n◎译　　名　82 Warriors / Il disprezzo\n◎类　　别　历史/战争\n◎年　　代　2013"}
	db.Create(meta)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	h.extractPTGenAssets(nil, meta)
	var got model.TorrentMetadata
	db.Where("info_hash = ?", "H1").First(&got)
	if got.ChineseTitle != "刘老庄八十二壮士" {
		t.Errorf("chinese_title = %q", got.ChineseTitle)
	}
	if got.EnglishTitle != "82" || len(got.EnglishTitle) < 2 {
		if got.EnglishTitle != "82 Warriors" {
			t.Errorf("english_title = %q, want 82 Warriors", got.EnglishTitle)
		}
	}
	if got.Genre != `["历史","战争"]` {
		t.Errorf("genre = %q", got.Genre)
	}
}
