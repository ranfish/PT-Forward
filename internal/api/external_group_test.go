package api

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// §59.577 刀1: 外站组三级链分类锚——注册表命中→external_group 新状态。
// 覆盖：Ⅲ未获取快照名/Ⅱ标题组名/Ⅰ级救回/Ⅰ级官组走状态流/13city 反例。
func TestClassifySeedStatusExternalGroup(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:extgrp_"+t.Name()+".db?mode=memory&cache=shared&_journal_mode=WAL"), &gorm.Config{})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	if err := db.AutoMigrate(&model.Site{}, &model.ReleaseGroupMapping{}, &model.ExternalReleaseGroup{}, &model.TorrentMetadata{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// 官组映射（HHWEB→憨憨）+ 外站注册表（MAMA/PTP）
	db.Create(&model.ReleaseGroupMapping{GroupName: "HHWEB", SiteName: "憨憨", IsOfficial: true, IsBuiltin: true})
	db.Create(&model.Site{Name: "憨憨", Domain: "hhanclub.net"})
	db.Create(&model.ExternalReleaseGroup{GroupName: "MAMA", Source: "census"})
	db.Create(&model.ExternalReleaseGroup{GroupName: "PTP", Source: "census"})

	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	h.SetSourceDetector(publish.NewSourceSiteDetector(db, zap.NewNop()))

	cases := []struct {
		name     string
		snapName string
		meta     *model.TorrentMetadata
		want     string
	}{
		// Ⅲ 未获取+快照名外站组（注册表命中）→ unfetched（§59.578 A 扩展：
		// 收录列表命中=可获取候选·簇/覆盖通道——SPHD 先例）
		{"未获取快照外站组", "Some Movie 2020 1080p BluRay AVC-MAMA", nil, "unfetched"},
		// Ⅲ 未获取+未知组 → unsupported①（不认识——窄门 A：未收录）
		{"未获取未知组", "Some Movie 2020 1080p BluRay AVC-UNKNOWNGRP", nil, "unsupported"},
		// Ⅱ 已获取+标题外站组 → 状态流（§59.577 刀2 形态修正：已获取外站组
		// 走 ④⑤⑥——§59.230 原则·身份剥离为并行字段）
		{"获取后标题外站组", "SOME MOVIE",
			&model.TorrentMetadata{Title: "Some Movie 2020 1080p BluRay AVC DTS-MAMA", SiteName: "家园"}, "incomplete"},
		// Ⅰ 标题无组名+release_group 列外站组（team.mama）→ 状态流（同上）
		{"Ⅰ级救回外站组", "SOME MOVIE",
			&model.TorrentMetadata{Title: "Some Movie 2020 1080p BluRay AVC DTS", SiteName: "家园", ReleaseGroup: "team.mama"}, "incomplete"},
		// Ⅰ 标题无组名+release_group 列官组（team.hhweb）→ 状态流（LookupGroup 命中走官组路径）
		{"Ⅰ级官组状态流", "SOME MOVIE",
			&model.TorrentMetadata{Title: "Some Movie 2020 1080p BluRay AVC DTS", SiteName: "家园", ReleaseGroup: "team.hhweb"}, "incomplete"},
		// Ⅰ team.other → 落Ⅱ级（标题无组名→状态流）
		{"Ⅰ级other落Ⅱ级", "SOME MOVIE",
			&model.TorrentMetadata{Title: "Some Movie 2020 1080p BluRay AVC DTS", SiteName: "家园", ReleaseGroup: "team.other"}, "incomplete"},
		// 13city 反例：Ⅰ级 team.hhweb + 标题带外站组 MAMA——Ⅱ级标题组名优先消费
		// （标题是发布链消费面）→ MAMA 注册表命中 → 状态流（身份并行字段）
		{"Ⅰ级官组Ⅱ级外站组", "SOME MOVIE",
			&model.TorrentMetadata{Title: "Some Movie 2020 1080p BluRay AVC DTS-MAMA", SiteName: "家园", ReleaseGroup: "team.hhweb"}, "incomplete"},
	}
	for _, c := range cases {
		got := h.classifySeedStatusLite(context.TODO(), c.snapName, c.meta, false, false)
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// §59.577 刀1: groupFromReleaseGroupSlot 三态锚（team.xxx 剥前缀·
// team.other/NOGROUP/空→空）。
func TestGroupFromReleaseGroupSlot(t *testing.T) {
	cases := map[string]string{
		"team.hhweb":   "hhweb",
		"team.mama":    "mama",
		"team.other":   "",
		"NOGROUP":      "",
		"":             "",
		"team.frds":    "frds",
		"  team.ptp  ": "ptp",
	}
	for in, want := range cases {
		if got := groupFromReleaseGroupSlot(in); got != want {
			t.Errorf("groupFromReleaseGroupSlot(%q) = %q, want %q", in, got, want)
		}
	}
}

// §59.577 刀1: LookupExternalGroup 锚——命中/miss/@子组回退。
func TestLookupExternalGroup(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file:extlookup_"+t.Name()+".db?mode=memory&cache=shared&_journal_mode=WAL"), &gorm.Config{})
	db.AutoMigrate(&model.ExternalReleaseGroup{})
	db.Create(&model.ExternalReleaseGroup{GroupName: "MAMA"})
	db.Create(&model.ExternalReleaseGroup{GroupName: "PTP"})

	d := publish.NewSourceSiteDetector(db, zap.NewNop())
	ctx := context.TODO()
	if !d.LookupExternalGroup(ctx, "MAMA") {
		t.Error("MAMA 应命中")
	}
	if !d.LookupExternalGroup(ctx, "mama") {
		t.Error("大小写不敏感应命中")
	}
	if d.LookupExternalGroup(ctx, "UNKNOWNGRP") {
		t.Error("未注册组应 miss")
	}
	if d.LookupExternalGroup(ctx, "") {
		t.Error("空组名应 miss")
	}
	// @子组回退：Yumi@PTP → PTP（镜像 LookupGroup OfficialGroupKey 先例）
	if !d.LookupExternalGroup(ctx, "Yumi@PTP") {
		t.Error("@子组应回退命中 PTP")
	}
}

// §59.577 刀1: .iso 顺修锚——TNND@CMCT.iso 案。
// 剥 .iso 扩展后提取器 @ 归一→官方键 CMCT（LookupGroup 命中不可说）——
// 该噪音种从 no_mapping 变已知官组（顺修的完整收益链）。
func TestExtractGroupNameIso(t *testing.T) {
	got := publish.ExtractGroupName("Some Movie 2020 1080p BluRay AVC-TNND@CMCT.iso")
	if got != "CMCT" {
		t.Errorf("got %q, want CMCT（@ 归一官方键）", got)
	}
	// 对照：不带 .iso 的同形态（既有行为回归）
	got2 := publish.ExtractGroupName("Some Movie 2020 1080p BluRay AVC-TNND@CMCT")
	if got2 != "CMCT" {
		t.Errorf("got %q, want CMCT", got2)
	}
}
