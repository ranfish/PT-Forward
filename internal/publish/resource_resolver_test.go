package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupResolverDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file:res_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db.Migrator().DropTable(&model.TorrentSnapshot{}, &model.TorrentMetadata{})
	if err := db.AutoMigrate(&model.TorrentSnapshot{}, &model.TorrentMetadata{}); err != nil {
		t.Fatal(err)
	}
	return db
}

// §59.44: 核心场景——挂载 hash ≠ 保留行 hash（黑暗侵袭 404 修复锚定）
func TestResolveResourceCrossHash(t *testing.T) {
	db := setupResolverDB(t)
	rr := NewResourceResolver(db)
	name := "黑暗侵袭.The.Descent.2005.Unrated.UHD.BluRay.2160p.x265.mUHD-FRDS"
	path := "/home/pt/pt1/FRDS"
	// 15 变体：挂载行 722b + 保留行 ea86 + 其他
	db.Create(&model.TorrentSnapshot{Hash: "h_keep", ClientID: "PT0", SavePath: path, Name: name, IsHidden: false})
	db.Create(&model.TorrentSnapshot{Hash: "h_meta", ClientID: "PT0", SavePath: path, Name: name, IsHidden: false})
	db.Create(&model.TorrentSnapshot{Hash: "h_other", ClientID: "PT0", SavePath: path, Name: name, IsHidden: false})
	db.Create(&model.TorrentMetadata{InfoHash: "h_meta", SiteName: "朋友", Title: "The Descent 2005", TorrentID: "123"})

	// 用保留行 hash（无 metadata）解析 → 应找到 h_meta 的数据
	rv := rr.ResolveResource(nil, "h_keep")
	if rv == nil || rv.Meta == nil {
		t.Fatalf("保留行解析应命中挂载行数据: %+v", rv)
	}
	if rv.Meta.InfoHash != "h_meta" {
		t.Errorf("应聚合到挂载行 h_meta, got %s", rv.Meta.InfoHash)
	}
	if len(rv.Hashes) != 3 {
		t.Errorf("应圈出 3 个活跃 hash, got %d", len(rv.Hashes))
	}
	if rv.FromFallback {
		t.Error("主路径不应走兜底")
	}
}

// §59.44: BC 场景——同 name 跨目录两副本独立（三元组键）
func TestResolveResourceCrossDir(t *testing.T) {
	db := setupResolverDB(t)
	rr := NewResourceResolver(db)
	name := "少年吔，安啦！.1992.TWN.1080p"
	// PT5 副本挂 meta；PT7 副本无
	db.Create(&model.TorrentSnapshot{Hash: "pt5_h", ClientID: "fnOS", SavePath: "/PT5/SSD", Name: name, IsHidden: false})
	db.Create(&model.TorrentSnapshot{Hash: "pt7_h", ClientID: "fnOS", SavePath: "/PT7/SSD", Name: name, IsHidden: false})
	db.Create(&model.TorrentMetadata{InfoHash: "pt5_h", SiteName: "朋友", Title: "x"})

	rv5 := rr.ResolveResource(nil, "pt5_h")
	rv7 := rr.ResolveResource(nil, "pt7_h")
	if rv5 == nil || rv5.Meta == nil {
		t.Fatal("PT5 应有数据")
	}
	if rv7 == nil || rv7.Meta != nil {
		t.Fatalf("PT7 应独立无数据（不串 PT5）: %+v", rv7)
	}
}

// §59.44: 分集场景——同目录不同 name 天然多资源
func TestResolveResourceEpisodes(t *testing.T) {
	db := setupResolverDB(t)
	rr := NewResourceResolver(db)
	db.Create(&model.TorrentSnapshot{Hash: "ep2", ClientID: "PT7", SavePath: "/PT/temp", Name: "Iron.Wok.Jan.2026.S01E02.1080p-AnoZu", IsHidden: false})
	db.Create(&model.TorrentSnapshot{Hash: "ep3", ClientID: "PT7", SavePath: "/PT/temp", Name: "Iron.Wok.Jan.2026.S01E03.1080p-AnoZu", IsHidden: false})
	db.Create(&model.TorrentMetadata{InfoHash: "ep2", SiteName: "CR", Title: "E02"})

	rv3 := rr.ResolveResource(nil, "ep3")
	if rv3 == nil || rv3.Meta != nil {
		t.Fatalf("E03 不应拿到 E02 的数据: %+v", rv3)
	}
}

// §59.44: hidden 兜底——编辑期间删种，资源消失但数据还在
func TestResolveResourceHiddenFallback(t *testing.T) {
	db := setupResolverDB(t)
	rr := NewResourceResolver(db)
	name := "Movie.2024-GROUP"
	db.Create(&model.TorrentSnapshot{Hash: "h1", ClientID: "PT0", SavePath: "/p", Name: name, IsHidden: false})
	db.Create(&model.TorrentMetadata{InfoHash: "h1", SiteName: "朋友", Title: "M"})
	// 模拟删种：hidden
	db.Model(&model.TorrentSnapshot{}).Where("hash = ?", "h1").Update("is_hidden", true)

	rv := rr.ResolveResource(nil, "h1")
	if rv == nil || rv.Meta == nil {
		t.Fatalf("hidden 后应兜底命中数据: %+v", rv)
	}
	if !rv.FromFallback {
		t.Error("应标记 FromFallback")
	}
}

// §59.44: 真未获取（无快照无数据）→ nil
func TestResolveResourceNil(t *testing.T) {
	db := setupResolverDB(t)
	rr := NewResourceResolver(db)
	if rv := rr.ResolveResource(nil, "nonexistent"); rv != nil {
		t.Errorf("应返回 nil, got %+v", rv)
	}
	_ = zap.NewNop()
}
