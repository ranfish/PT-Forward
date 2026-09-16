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
