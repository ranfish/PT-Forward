package publish

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newDirectedTestPipeline(t *testing.T) (*Pipeline, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TorrentMetadata{}); err != nil {
		t.Fatal(err)
	}
	return &Pipeline{db: db, logger: zap.NewNop()}, db
}

func seedMetaFlags(t *testing.T, db *gorm.DB, infoHash, site string, flags []string, until *time.Time) {
	t.Helper()
	data, _ := json.Marshal(flags)
	meta := &model.TorrentMetadata{
		InfoHash: infoHash, SiteName: site, Flags: string(data),
		NoTransferUntil: until,
	}
	if err := db.Create(meta).Error; err != nil {
		t.Fatal(err)
	}
}

// §59.369 附二: 深度站"限转"两态消费——
// ① 目标 ∈ 定向名单（时间/肉丝/家园）→ 永久拦
// ② 目标 ∉ 名单 → NoTransferUntil（发布时间+72h）窗内拦 / 窗外放行
// ③ until 缺失（提取失败/老数据）→ 不拦窗（名单已拦三站；缺失时无限保守=老种永久误拦）
func TestDirectedNoTransferConsumption(t *testing.T) {
	p, db := newDirectedTestPipeline(t)
	ctx := context.Background()

	// 深度站限转种（窗内：until=未来）
	future := time.Now().Add(48 * time.Hour)
	past := time.Now().Add(-2 * time.Hour)
	seedMetaFlags(t, db, "hash1", "深度", []string{"限转"}, &future)
	seedMetaFlags(t, db, "hash2", "深度", []string{"限转"}, &past)   // 窗外
	seedMetaFlags(t, db, "hash3", "深度", []string{"限转"}, nil)   // until 缺失
	seedMetaFlags(t, db, "hash4", "深度", []string{"官方"}, nil)   // 非限转

	// ① 名单内三站——永久拦（无论窗）
	for _, target := range []string{"时间", "肉丝", "家园"} {
		for _, h := range []string{"hash1", "hash2", "hash3"} {
			blocked, reason := p.checkFlagsFromMetadata(ctx, h, "深度", target)
			if !blocked {
				t.Fatalf("%s → 名单站 %s 应拦", h, target)
			}
			if reason == "" {
				t.Fatal("拦截必须带 reason")
			}
		}
	}

	// ② 名单外——窗内拦 / 窗外放行 / 缺失放行
	if blocked, _ := p.checkFlagsFromMetadata(ctx, "hash1", "深度", "织梦"); !blocked {
		t.Fatal("窗内（until 未来）名单外站应拦")
	}
	if blocked, _ := p.checkFlagsFromMetadata(ctx, "hash2", "深度", "织梦"); blocked {
		t.Fatal("窗外（until 已过）名单外站应放行——限转解除后可转其它站")
	}
	if blocked, _ := p.checkFlagsFromMetadata(ctx, "hash3", "深度", "织梦"); blocked {
		t.Fatal("until 缺失名单外站应放行（名单已拦三站，缺失不无限保守）")
	}

	// ③ 非限转 flag 不受新逻辑影响
	if blocked, _ := p.checkFlagsFromMetadata(ctx, "hash4", "深度", "时间"); blocked {
		t.Fatal("非限转 flag 不应被名单拦")
	}

	// ④ 其它站"限转"（keepfrds 等——暂无名单映射）不受名单+72h 窗影响
	//（keepfrds 的窗走"限时禁转"独立通道——窗判定限定名单映射站）
	seedMetaFlags(t, db, "hash5", "朋友", []string{"限转"}, &future)
	if blocked, _ := p.checkFlagsFromMetadata(ctx, "hash5", "朋友", "时间"); blocked {
		t.Fatal("无名单映射的站'限转'不应按深度站名单/窗拦（keepfrds 维持现状）")
	}

	// ⑤ 既有禁转族回归（forbiddenFlags 通道不变）
	seedMetaFlags(t, db, "hash6", "深度", []string{"禁转"}, nil)
	if blocked, _ := p.checkFlagsFromMetadata(ctx, "hash6", "深度", "织梦"); !blocked {
		t.Fatal("既有'禁转'永久拦回归")
	}
}

// §59.409 附: 优堡官种定向禁转——统一基建链（ExtractGroupName →
// release_group_mappings 查 site_name+is_official）。
// rules.php 实时权威："任何官方资源(UBits/UBWEB/UBTV)，禁止转载到以下
// 黑名单PT站点：PTTime，CMCT（SpringSunDay）"——CMCT=不可说由互斥对
// 覆盖，名单仅时间。
func TestUbitsOfficialDirectedNoTransfer(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TorrentMetadata{}, &model.ReleaseGroupMapping{}); err != nil {
		t.Fatal(err)
	}
	p := &Pipeline{db: db, logger: zap.NewNop()}
	ctx := context.Background()

	// 优堡官组资产（对齐 seed 行——含 @复合 DIY 形态）
	for _, g := range []string{"UBits", "UBWEB", "UBTV", "SHB931@UBWEB"} {
		if err := db.Create(&model.ReleaseGroupMapping{
			GroupName: g, Domain: "ubits.club", SiteName: "优堡", IsOfficial: true,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	official := "Movie 2024 1080p Blu-ray DD5.1 x264-UBits"
	officialAt := "Show S01 2160p WEB-DL DV HDR10 DDP5.1 Atoms DIY@UBWEB"
	unofficial := "Film 2023 1080p WEB-DL DDP5.1 H.264-FRDS"

	// ① 官种 → 时间：拦（含文案）
	if blocked, reason := p.checkOfficialDirectedNoTransfer(ctx, "优堡", official, "时间"); !blocked {
		t.Fatal("优堡官种 → 时间 应拦")
	} else if !strings.Contains(reason, "官种") {
		t.Fatalf("拦截文案缺官种标识: %s", reason)
	}

	// ② 官种 → 名单外：放行
	if blocked, _ := p.checkOfficialDirectedNoTransfer(ctx, "优堡", official, "织梦"); blocked {
		t.Fatal("优堡官种 → 非名单站应放行")
	}

	// ③ 非官种 → 时间：放行（FRDS 非优堡官组）
	if blocked, _ := p.checkOfficialDirectedNoTransfer(ctx, "优堡", unofficial, "时间"); blocked {
		t.Fatal("优堡非官种（FRDS）→ 时间 应放行")
	}

	// ④ 源站限定：朋友站的 UBits 词形不拦（映射行 site_name=优堡）
	if blocked, _ := p.checkOfficialDirectedNoTransfer(ctx, "朋友", official, "时间"); blocked {
		t.Fatal("非优堡源站即使组名命中也不拦")
	}

	// ⑤ @复合 DIY 形态：官方组版归一 UBWEB → 拦
	if blocked, _ := p.checkOfficialDirectedNoTransfer(ctx, "优堡", officialAt, "时间"); !blocked {
		t.Fatal("DIY@UBWEB 复合形态应归一到 UBWEB 官种拦")
	}

	// ⑥ 无组名标题：放行
	if blocked, _ := p.checkOfficialDirectedNoTransfer(ctx, "优堡", "Movie 2024 1080p WEB-DL", "时间"); blocked {
		t.Fatal("无组名标题应放行")
	}

	// ⑦ 限转块排除优堡回归：优堡 flags 含"限转"+窗内未来——
	// 非官种不进名单/窗拦（优堡非"限转"语义站）
	future := time.Now().Add(48 * time.Hour)
	seedMetaFlags(t, db, "ubhash1", "优堡", []string{"限转"}, &future)
	if blocked, _ := p.checkFlagsFromMetadata(ctx, "ubhash1", "优堡", "时间"); blocked {
		t.Fatal("优堡'限转'不应走深度站名单/72h 窗（官种通道独立判）")
	}
}

// §59.409 附二: 发布行为前置检查（preflightNoTransfer——Execute 活链
// 单点）。用户语义定案：获取链打标与目标无关；发布链拦"转载/发布行为
// 本身"与目标相关。七态：
// ① 绝对禁转 flags → 拦全部目标站
// ② 限时禁转窗内 → 拦；窗外 → 放行
// ③ 优堡官种 → 定向名单站（时间）拦；名单外放行
// ④ 互斥转载对（优堡→不可说）拦
// ⑤ 用户排除规则拦
// ⑥ 非官种/无标记 → 放行
// ⑦ 空 meta/空目标站 → 放行（防御态）
func TestPreflightNoTransfer(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TorrentMetadata{}, &model.ReleaseGroupMapping{}, &model.PublishExclusion{}); err != nil {
		t.Fatal(err)
	}
	pipe := &Pipeline{db: db, logger: zap.NewNop()}
	e := &PublishExecutor{pipe: pipe, db: db, logger: zap.NewNop()}
	ctx := context.Background()

	for _, g := range []string{"UBits", "UBWEB", "UBTV"} {
		if err := db.Create(&model.ReleaseGroupMapping{
			GroupName: g, Domain: "ubits.club", SiteName: "优堡", IsOfficial: true,
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&model.PublishExclusion{TargetSite: "织梦", SourceSite: "朋友"}).Error; err != nil {
		t.Fatal(err)
	}

	future := time.Now().Add(48 * time.Hour)
	past := time.Now().Add(-2 * time.Hour)
	seedMetaFlags(t, db, "pf1", "朋友", []string{"禁转"}, nil)          // 绝对禁转
	seedMetaFlags(t, db, "pf2", "朋友", []string{"限时禁转"}, &future)   // 窗内
	seedMetaFlags(t, db, "pf3", "朋友", []string{"限时禁转"}, &past)     // 窗外
	seedMetaFlags(t, db, "pf4", "优堡", nil, nil)                       // 官种载体（flags 空）
	seedMetaFlags(t, db, "pf5", "优堡", nil, nil)                       // 非官种载体
	seedMetaFlags(t, db, "pf6", "朋友", nil, nil)                       // 排除规则载体

	var m1, m2, m3, m4, m5, m6 model.TorrentMetadata
	for h, m := range map[string]*model.TorrentMetadata{
		"pf1": &m1, "pf2": &m2, "pf3": &m3, "pf4": &m4, "pf5": &m5, "pf6": &m6,
	} {
		if err := db.Where("info_hash = ?", h).First(m).Error; err != nil {
			t.Fatal(err)
		}
	}
	m4.Title = "Movie 2024 1080p Blu-ray DD5.1 x264-UBits" // 官种
	m5.Title = "Film 2023 1080p WEB-DL DDP5.1 H.264-FRDS"  // 非官种
	if err := db.Save(&m4).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Save(&m5).Error; err != nil {
		t.Fatal(err)
	}

	// ① 绝对禁转 → 拦全部目标站
	if blocked, reason := e.preflightNoTransfer(ctx, &m1, "织梦"); !blocked {
		t.Fatal("绝对禁转应拦全部目标站")
	} else if !strings.Contains(reason, "禁转") {
		t.Fatalf("文案缺禁转标识: %s", reason)
	}

	// ② 限时禁转窗内拦 / 窗外放行（用"克隆"——避开 ⑤ 排除规则的朋友→织梦组合）
	if blocked, _ := e.preflightNoTransfer(ctx, &m2, "克隆"); !blocked {
		t.Fatal("限时禁转窗内应拦")
	}
	if blocked, _ := e.preflightNoTransfer(ctx, &m3, "克隆"); blocked {
		t.Fatal("限时禁转窗外应放行")
	}

	// ③ 优堡官种 → 时间拦；织梦放行
	if blocked, reason := e.preflightNoTransfer(ctx, &m4, "时间"); !blocked {
		t.Fatal("优堡官种 → 时间 应拦")
	} else if !strings.Contains(reason, "官种") {
		t.Fatalf("文案缺官种标识: %s", reason)
	}
	if blocked, _ := e.preflightNoTransfer(ctx, &m4, "克隆"); blocked {
		t.Fatal("优堡官种 → 名单外站应放行")
	}

	// ④ 互斥转载对：优堡 → 不可说拦
	if blocked, reason := e.preflightNoTransfer(ctx, &m5, "不可说"); !blocked {
		t.Fatal("优堡 → 不可说 互斥应拦")
	} else if !strings.Contains(reason, "互斥") {
		t.Fatalf("文案缺互斥标识: %s", reason)
	}

	// ⑤ 排除规则：朋友 → 织梦拦
	if blocked, reason := e.preflightNoTransfer(ctx, &m6, "织梦"); !blocked {
		t.Fatal("排除规则应拦")
	} else if !strings.Contains(reason, "排除") {
		t.Fatalf("文案缺排除标识: %s", reason)
	}

	// ⑥ 非官种无标记 → 放行（对照）
	if blocked, _ := e.preflightNoTransfer(ctx, &m5, "克隆"); blocked {
		t.Fatal("非官种无标记应放行")
	}

	// ⑦ 防御态：空 meta/空目标站
	if blocked, _ := e.preflightNoTransfer(ctx, nil, "织梦"); blocked {
		t.Fatal("空 meta 应放行（防御）")
	}
	if blocked, _ := e.preflightNoTransfer(ctx, &m5, ""); blocked {
		t.Fatal("空目标站应放行（防御）")
	}
}
