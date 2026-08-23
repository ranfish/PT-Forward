package publish

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/comment"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// §59.61 层选①c: 簇 comment tid 嵌入组映射层——J1(自指)/J1b(溯源) 在 coverage_tid 之前
func setupDetector(t *testing.T) (*SourceSiteDetector, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	// 朋友站（cookie 就绪） + FRDS→朋友 组映射
	db.Create(&model.Site{Name: "朋友", Domain: "pt.keepfrds.com", BaseURL: "https://pt.keepfrds.com", Framework: "nexusphp", Cookie: "c=1", Enabled: true, IsSource: true})
	db.Exec("INSERT INTO release_group_mappings (group_name, site_name, is_official, domain) VALUES ('FRDS', '朋友', 1, 'keepfrds.com')")
	return NewSourceSiteDetector(db, zap.NewNop()), db
}

func TestSelectFetchSite_CommentTid_J1(t *testing.T) {
	d, _ := setupDetector(t)
	// J1: 簇内朋友站副本 comment 自指 tid
	res := d.SelectFetchSite(
		context.Background(),
		"时空奇旅.Arco.2025.UHD.BluRay.2160p.x265.10bit.DV.HDR.DTS-HD.MA.7.1.mUHD-FRDS",
		nil, // 无 coverage——纯 comment 直达
		[]comment.DirectTarget{{SiteName: "朋友", TorrentID: "2782048"}},
	)
	if res.SourceSite != "朋友" {
		t.Fatalf("应选朋友站, got %q", res.SourceSite)
	}
	if res.TorrentID != "2782048" {
		t.Fatalf("J1 comment tid 应直达: got %q", res.TorrentID)
	}
}

func TestSelectFetchSite_CommentTid_CoverageBackup(t *testing.T) {
	d, _ := setupDetector(t)
	// comment 指向别的站（J2 场景，官组链不消费）→ coverage_tid 仍是官组链兜底
	res := d.SelectFetchSite(
		context.Background(),
		"xxx.2025.1080p.WEB-DL-FRDS",
		[]model.SiteCoverageCache{{SiteName: "朋友", TorrentID: "123", Status: model.CoverageConfirmedHas}},
		[]comment.DirectTarget{{SiteName: "财神", TorrentID: "999"}},
	)
	if res.TorrentID != "123" {
		t.Fatalf("官组链应保持 coverage_tid 兜底, got %q", res.TorrentID)
	}
}

func TestSelectFetchSite_J2_GrayLogOnly(t *testing.T) {
	d, db := setupDetector(t)
	// 非官组: J2 目标站须 enabled+cookie 才可直达（财神没配置 cookie → 不命中）
	db.Create(&model.Site{Name: "财神", Domain: "cspt.top", BaseURL: "https://cspt.top", Framework: "nexusphp", Cookie: "c=1", Enabled: true})
	res := d.SelectFetchSite(
		context.Background(),
		"某无组后缀种子名.2024",
		nil,
		[]comment.DirectTarget{{SiteName: "财神", TorrentID: "170568"}},
	)
	if res.SourceSite != "财神" || res.TorrentID != "170568" {
		t.Fatalf("J2 应直达财神: got %+v", res)
	}
}
