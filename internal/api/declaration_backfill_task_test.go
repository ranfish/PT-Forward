package api

import (
	"context"
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"github.com/ranfish/pt-forward/internal/setting"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// §59.516 v0.0.1461 锚：简介引用清洗任务——残留行净化（引用块整删·正文
// 保留）·干净行零触碰·幂等（二轮 changed=0）。
func TestDeclBackfillTask(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.TorrentMetadata{}, &setting.Setting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	f := publish.NewDeclarationFilter(setting.NewRepository(db), nil)
	h := &PublishTorrentsHandler{db: db, declFilter: f}
	rows := []model.TorrentMetadata{
		{InfoHash: "aa", Statement: "[quote][color=#ff0000]您的保种是(1PTBA)站长久发展的重要保证！自觉保种[/color][/quote]正文声明保留"},
		{InfoHash: "bb", Description: "[quote]郑重声明：本站提供的所有作品均是用户自行搜集并且上传[/quote]简介正文保留"},
		{InfoHash: "cc", Statement: "干净的声明"},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	ctx := context.Background()
	patterns := f.GetPatterns(ctx)
	if len(patterns) == 0 {
		t.Fatal("前置：生效词表为空")
	}
	h.declBackfill.total = len(rows)
	h.runDeclBackfillTask(rows, patterns)

	// ① 残留净化：quote 块剥除·正文保留
	var aa model.TorrentMetadata
	db.Where("info_hash = ?", "aa").First(&aa)
	if aa.Statement != "正文声明保留" {
		t.Errorf("aa statement=%q（应剥 quote 块留正文）", aa.Statement)
	}
	var bb model.TorrentMetadata
	db.Where("info_hash = ?", "bb").First(&bb)
	if bb.Description != "简介正文保留" {
		t.Errorf("bb description=%q（应剥 quote 块留正文）", bb.Description)
	}
	// ② 干净行零触碰
	var cc model.TorrentMetadata
	db.Where("info_hash = ?", "cc").First(&cc)
	if cc.Statement != "干净的声明" {
		t.Errorf("cc statement=%q（干净行不应被改）", cc.Statement)
	}
	// ③ 计数：changed=2·done=3·failed=0
	if h.declBackfill.changed != 2 || h.declBackfill.done != 3 || h.declBackfill.failed != 0 {
		t.Errorf("计数 changed=%d done=%d failed=%d（应 2/3/0）", h.declBackfill.changed, h.declBackfill.done, h.declBackfill.failed)
	}
	// ④ 幂等：二轮（重新查询——生产语义=任务启动时读库·已净化行零变化）
	var fresh []model.TorrentMetadata
	db.Where("info_hash IN ?", []string{"aa", "bb", "cc"}).Find(&fresh)
	h.declBackfill.changed = 0
	h.declBackfill.done = 0
	h.declBackfill.active = true
	h.declBackfill.finished = false
	h.declBackfill.total = len(fresh)
	h.runDeclBackfillTask(fresh, patterns)
	if h.declBackfill.changed != 0 || h.declBackfill.done != 3 {
		t.Errorf("幂等失败：changed=%d done=%d（应 0/3）", h.declBackfill.changed, h.declBackfill.done)
	}
}

// §59.542 追记五锚：预筛 SQL 端到端——v0.0.1523 生产事故（反引号字符串里
// 字面 \" 致 LIKE 永不命中·total=0）。必须用真实 JSON 形态的行直接调生产
// 预筛方法验证（提取 declBackfillSelectRows 的原因——测试里复制查询=与
// 实现共享误解·抓不到转义错）。
func TestDeclBackfillSelectRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.TorrentMetadata{}, &setting.Setting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	f := publish.NewDeclarationFilter(setting.NewRepository(db), nil)
	h := &PublishTorrentsHandler{db: db, declFilter: f}
	seed := []model.TorrentMetadata{
		// ① 空 statement + 官组（正常组名）→ 必须选中（16625 行目标形态）
		{InfoHash: "sel1", DetailSourceJSON: `{"site_name":"朋友","release_group":"FRDS"}`},
		// ② 空 statement + NOGROUP → 不选（无致谢语义）
		{InfoHash: "skip1", DetailSourceJSON: `{"site_name":"朋友","release_group":"NOGROUP"}`},
		// ③ 空 statement + 空组名 → 不选
		{InfoHash: "skip2", DetailSourceJSON: `{"site_name":"朋友","release_group":""}`},
		// ④ 空 statement + other → 不选
		{InfoHash: "skip3", DetailSourceJSON: `{"site_name":"朋友","release_group":"other"}`},
		// ⑤ 空 statement + 无 release_group 键 → 不选
		{InfoHash: "skip4", DetailSourceJSON: `{"site_name":"朋友"}`},
		// ⑥ 含水印词 statement → 选中（LIKE 通道）
		{InfoHash: "sel2", Statement: "含互珍互重，禁转PTT的声明"},
		// ⑦ 干净且非空 statement + 无官组 → 不选
		{InfoHash: "skip5", Statement: "干净声明", DetailSourceJSON: `{"site_name":"朋友"}`},
	}
	for i := range seed {
		if err := db.Create(&seed[i]).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	ctx := context.Background()
	patterns := f.GetPatterns(ctx)
	rows, err := h.declBackfillSelectRows(ctx, patterns)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	got := map[string]bool{}
	for _, r := range rows {
		got[r.InfoHash] = true
	}
	for _, want := range []string{"sel1", "sel2"} {
		if !got[want] {
			t.Errorf("预筛漏选 %s（目标形态未被选中=re-append 无法触达）", want)
		}
	}
	for _, skip := range []string{"skip1", "skip2", "skip3", "skip4", "skip5"} {
		if got[skip] {
			t.Errorf("预筛误选 %s（应跳过）", skip)
		}
	}
}

// §59.542 追记五锚：空 statement + 官组 → 行级直接 re-append——原实现被
// `row.Statement != ""` 守卫整体跳过（即使预选中也不处理）。
// 追记六锚升级：组名从 title 提取（发布链 :3053 同源）——detail_source_json
// 的 release_group 是槽位键（team.other）·误用致「team.other官组作品」错形态。
func TestDeclBackfillEmptyStatementReappend(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.TorrentMetadata{}, &setting.Setting{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	f := publish.NewDeclarationFilter(setting.NewRepository(db), nil)
	h := &PublishTorrentsHandler{db: db, declFilter: f}
	rows := []model.TorrentMetadata{
		// ① 空 statement + title 含真实组名（Pope 案形态：detail_source
		// 是槽位键 team.other·title 里才是 FRDS 真身）→ 致谢用 FRDS
		{InfoHash: "ee", SiteName: "柠檬不甜", Title: "The Pope's Exorcist 2023 2160p UHD BluRay DoVi HDR x265 TrueHD 7.1 Atmos-mUHD-FRDS", DetailSourceJSON: `{"site_name":"柠檬不甜","release_group":"team.other"}`},
		// ② 空 statement + title 无组名 + 槽位键 → 不 append（组名提取空）
		{InfoHash: "ff", SiteName: "朋友", Title: "Some Movie 2023 1080p BluRay", DetailSourceJSON: `{"site_name":"朋友","release_group":"NOGROUP"}`},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	ctx := context.Background()
	patterns := f.GetPatterns(ctx)
	h.declBackfill.total = len(rows)
	h.runDeclBackfillTask(rows, patterns)

	var ee model.TorrentMetadata
	db.Where("info_hash = ?", "ee").First(&ee)
	if ee.Statement == "" {
		t.Fatal("ee statement 仍为空（空 statement+官组未 re-append）")
	}
	if !strings.Contains(ee.Statement, "FRDS") || !strings.Contains(ee.Statement, "互珍互重，禁转PTT") {
		t.Errorf("ee statement=%q（应含真实组名 FRDS+禁转PTT 两句致谢）", ee.Statement)
	}
	if strings.Contains(ee.Statement, "team.other") {
		t.Errorf("ee statement=%q（致谢组名误用槽位键 team.other——应从 title 提取真实组名）", ee.Statement)
	}
	if !strings.Contains(ee.Statement, "color=blue") {
		t.Errorf("ee statement=%q（应为 blue 原生形态）", ee.Statement)
	}
	var ff model.TorrentMetadata
	db.Where("info_hash = ?", "ff").First(&ff)
	if ff.Statement != "" {
		t.Errorf("ff statement=%q（无组名应保持空）", ff.Statement)
	}
	// 计数：changed=1（ee）·done=2·failed=0
	if h.declBackfill.changed != 1 || h.declBackfill.done != 2 || h.declBackfill.failed != 0 {
		t.Errorf("计数 changed=%d done=%d failed=%d（应 1/2/0）", h.declBackfill.changed, h.declBackfill.done, h.declBackfill.failed)
	}
	// 幂等：二轮零变化（Filter 剥掉再加回 final==原值不 UPDATE）
	var fresh []model.TorrentMetadata
	db.Where("info_hash IN ?", []string{"ee", "ff"}).Find(&fresh)
	h.declBackfill.changed = 0
	h.declBackfill.done = 0
	h.declBackfill.active = true
	h.declBackfill.finished = false
	h.declBackfill.total = len(fresh)
	h.runDeclBackfillTask(fresh, patterns)
	if h.declBackfill.changed != 0 {
		t.Errorf("幂等失败：changed=%d（应 0）", h.declBackfill.changed)
	}
}
