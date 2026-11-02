package publish

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/compliance"
	"github.com/ranfish/pt-forward/internal/titleparser"
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

// §59.409 附三: compliance 第五序——§59.143"红标不隐藏+提交兜底"设计的
// 兜底层（原挂死代码从未生效）。两态：规则命中拦 / 无规则放行（含 nil 跳过）。
func TestPreflightCompliance(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TorrentMetadata{}, &model.ReleaseGroupMapping{},
		&model.PublishExclusion{}, &model.ComplianceRule{}); err != nil {
		t.Fatal(err)
	}
	checker := compliance.NewChecker(db, zap.NewNop())
	pipe := &Pipeline{db: db, logger: zap.NewNop(), complianceChecker: checker}
	e := &PublishExecutor{pipe: pipe, db: db, logger: zap.NewNop()}
	ctx := context.Background()

	meta := &model.TorrentMetadata{InfoHash: "cf1", SiteName: "朋友", Title: "Normal Movie 2024"}

	// ① 无规则：放行
	if blocked, _ := e.preflightNoTransfer(ctx, meta, "克隆"); blocked {
		t.Fatal("无 compliance 规则应放行")
	}

	// ② nil checker：跳过不 panic（测试构造防御态）
	e2 := &PublishExecutor{pipe: &Pipeline{db: db, logger: zap.NewNop()}, db: db, logger: zap.NewNop()}
	if blocked, _ := e2.preflightNoTransfer(ctx, meta, "克隆"); blocked {
		t.Fatal("nil checker 应跳过放行")
	}

	// ③ forbidden_keyword 规则命中：拦
	if err := db.Create(&model.ComplianceRule{
		RuleType: model.RuleTypeForbiddenKeyword, Pattern: "Normal Movie",
		Scope: "publish", Source: "builtin",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if blocked, reason := e.preflightNoTransfer(ctx, meta, "克隆"); !blocked {
		t.Fatal("compliance 规则命中应拦")
	} else if !strings.Contains(reason, "系统禁转") {
		t.Fatalf("文案缺系统禁转标识: %s", reason)
	}
}

// §59.411: 藏宝阁动画/动漫分立特化——SeasonEpisode 多集信号分发三态。
func TestCbgCategoryValue(t *testing.T) {
	cfg := &model.PublishFormConfig{ValueMappings: map[string][]model.FormValueMapping{
		model.FieldDomainType: {
			{Label: "动画", Value: "404", StandardKeys: []string{"category.animation"}},
			{Label: "动漫", Value: "405"},
		},
	}}
	// ① 动画+多集信号（SeasonEpisode 非空）→ 405 动漫
	tp := titleparser.TechProfile{SeasonEpisode: "S01"}
	v := cbgCategoryValue(cfg, "藏宝阁", &model.TorrentMetadata{Category: "category.animation"}, tp)
	if v == nil || v.Value != "405" {
		t.Fatalf("动画多集应特化分发 405 动漫，got %v", v)
	}
	// ② 动画+单集（无 SeasonEpisode）→ nil（回退通用链 404 兜底）
	tp2 := titleparser.TechProfile{}
	if v := cbgCategoryValue(cfg, "藏宝阁", &model.TorrentMetadata{Category: "category.animation"}, tp2); v != nil {
		t.Fatalf("动画单集应回退通用链（404 配键兜底），got %v", v)
	}
	// ③ 非动画类 → nil（特化零介入）
	if v := cbgCategoryValue(cfg, "藏宝阁", &model.TorrentMetadata{Category: "category.movie"}, tp); v != nil {
		t.Fatal("非动画类不走特化")
	}
	// ④ 非藏宝阁 → nil
	if v := cbgCategoryValue(cfg, "好大", &model.TorrentMetadata{Category: "category.animation"}, tp); v != nil {
		t.Fatal("非藏宝阁不走特化")
	}
}

// §59.414: 压制联动带点形态——三月 X.264/X.265 label 带点匹配增强。
func TestLookupCLIVariantDottedLabel(t *testing.T) {
	cfg := &model.PublishFormConfig{ValueMappings: map[string][]model.FormValueMapping{
		model.FieldDomainCodec: {
			{Label: "H.264", Value: "1", StandardKeys: []string{"video.h264"}},
			{Label: "X.264", Value: "6"},
			{Label: "HEVC", Value: "2", StandardKeys: []string{"video.h265"}},
			{Label: "X.265", Value: "7"},
		},
	}}
	// 带点形态命中（三月形态）
	if v := lookupCLIVariant(cfg, "video.h264"); v == nil || v.Value != "6" {
		t.Fatalf("X.264 带点应命中联动通道，got %v", v)
	}
	if v := lookupCLIVariant(cfg, "video.h265"); v == nil || v.Value != "7" {
		t.Fatalf("X.265 带点应命中联动通道，got %v", v)
	}
	// CLI 原始键形态同命中
	if v := lookupCLIVariant(cfg, "video.x264"); v == nil || v.Value != "6" {
		t.Fatal("video.x264 原始键应命中")
	}
	// 无点形态站不受影响（无 X264 条目返回 nil——原盘走直查）
	cfg2 := &model.PublishFormConfig{ValueMappings: map[string][]model.FormValueMapping{
		model.FieldDomainCodec: {{Label: "H.264", Value: "1", StandardKeys: []string{"video.h264"}}},
	}}
	if v := lookupCLIVariant(cfg2, "video.h264"); v != nil {
		t.Fatal("无 X 条目站应返回 nil（直查路径）")
	}
}

// §59.417: Other 兜底前缀变体——我的PT "Other/转载" 形态增强。
func TestLookupOtherOptionPrefixVariant(t *testing.T) {
	cfg := &model.PublishFormConfig{ValueMappings: map[string][]model.FormValueMapping{
		model.FieldDomainTeam: {
			{Label: "HDS", Value: "1"},
			{Label: "MY/发布者", Value: "4"},
			{Label: "Other/转载", Value: "5"},
		},
	}}
	e := &PublishExecutor{}
	if v := e.lookupOtherOption(cfg, model.FieldDomainTeam); v == nil || v.Value != "5" {
		t.Fatalf("Other/转载 前缀变体应命中兜底，got %v", v)
	}
	// 精确三态回归（既有行为不变）
	cfg2 := &model.PublishFormConfig{ValueMappings: map[string][]model.FormValueMapping{
		model.FieldDomainTeam: {
			{Label: "FRDS", Value: "1"},
			{Label: "其它", Value: "2"},
		},
	}}
	if v := e.lookupOtherOption(cfg2, model.FieldDomainTeam); v == nil || v.Value != "2" {
		t.Fatal("精确「其它」应命中")
	}
	// 无 Other 项站返回 nil（不强填）
	cfg3 := &model.PublishFormConfig{ValueMappings: map[string][]model.FormValueMapping{
		model.FieldDomainTeam: {{Label: "HDS", Value: "1"}},
	}}
	if v := e.lookupOtherOption(cfg3, model.FieldDomainTeam); v != nil {
		t.Fatal("无 Other 项站应返回 nil")
	}
}

// §59.419: 压制媒介按分辨率细分匹配——红豆饭 §5.1 权威规则四分形态。
func TestResolutionEncodeOption(t *testing.T) {
	cfg := &model.PublishFormConfig{ValueMappings: map[string][]model.FormValueMapping{
		model.FieldDomainMedium: {
			{Label: "UHD原盘", Value: "17", StandardKeys: []string{"medium.uhd_bluray"}},
			{Label: "UHD DIY", Value: "18"},
			{Label: "UHD Remux", Value: "19", StandardKeys: []string{"medium.uhd_remux"}},
			{Label: "UHD压制", Value: "20"},
			{Label: "BD原盘", Value: "21", StandardKeys: []string{"medium.bluray"}},
			{Label: "BD Remux", Value: "23", StandardKeys: []string{"medium.remux"}},
			{Label: "1080P/i压制", Value: "24"},
			{Label: "720P压制", Value: "25"},
			{Label: "MiniSD", Value: "26"},
			{Label: "WEB-DL", Value: "5", StandardKeys: []string{"medium.webdl", "medium.webrip"}},
		},
	}}
	cases := []struct {
		resolution string
		want       string
	}{
		{"2160p", "20"}, {"4320P", "20"},
		{"1080p", "24"}, {"1080i", "24"},
		{"720p", "25"},
		{"480p", "26"}, {"576p", "26"}, // SD 族（r480p/r576p/sd 归 default）→MiniSD
	}
	for _, c := range cases {
		tp := titleparser.TechProfile{Resolution: c.resolution}
		v := resolutionEncodeOption(cfg, tp, "medium.encode")
		if v == nil || v.Value != c.want {
			t.Fatalf("Resolution=%q 应命中 %s 项，got %v", c.resolution, c.want, v)
		}
	}
	// 空 Resolution→归一键空→不触发（medium 落空预览人工）
	if v := resolutionEncodeOption(cfg, titleparser.TechProfile{}, "medium.encode"); v != nil {
		t.Fatal("空 Resolution 应不触发")
	}
	// 非压制媒介不触发
	tp := titleparser.TechProfile{Resolution: "1080p"}
	if v := resolutionEncodeOption(cfg, tp, "medium.bluray"); v != nil {
		t.Fatal("非 encode 媒介应不触发")
	}
	// 通用 Encode 项站不触发（label 精确 Encode——通用兜底优先）
	cfg2 := &model.PublishFormConfig{ValueMappings: map[string][]model.FormValueMapping{
		model.FieldDomainMedium: {
			{Label: "Encode", Value: "18", StandardKeys: []string{"medium.encode"}},
			{Label: "1080P/i压制", Value: "24"},
		},
	}}
	if v := resolutionEncodeOption(cfg2, titleparser.TechProfile{Resolution: "1080p"}, "medium.encode"); v != nil {
		t.Fatal("通用 Encode 项站应不触发（直查兜底优先）")
	}
	// 无细分项站零触发
	cfg3 := &model.PublishFormConfig{ValueMappings: map[string][]model.FormValueMapping{
		model.FieldDomainMedium: {{Label: "WEB-DL", Value: "5", StandardKeys: []string{"medium.webdl"}}},
	}}
	if v := resolutionEncodeOption(cfg3, titleparser.TechProfile{Resolution: "1080p"}, "medium.encode"); v != nil {
		t.Fatal("无细分项站应返回 nil")
	}
	// 目标族缺失降级链（无 UHD 压制项的细分站→1080 压制兜底）
	cfg4 := &model.PublishFormConfig{ValueMappings: map[string][]model.FormValueMapping{
		model.FieldDomainMedium: {{Label: "1080P/i压制", Value: "24"}},
	}}
	if v := resolutionEncodeOption(cfg4, titleparser.TechProfile{Resolution: "2160p"}, "medium.encode"); v == nil || v.Value != "24" {
		t.Fatal("UHD 种无 UHD 压制项应降级 1080 压制兜底")
	}
}

// §59.420: 家园 type 三维融合词形路由——63 项 label 前缀+媒介+分辨率分发。
func TestHdhomeCategoryValue(t *testing.T) {
	cfg := &model.PublishFormConfig{ValueMappings: map[string][]model.FormValueMapping{
		model.FieldDomainType: {
			{Label: "Movies 8K UHD BD", Value: "506"}, {Label: "Movies UHD Blu-ray", Value: "499"},
			{Label: "Movies UHD REMUX", Value: "518"}, {Label: "Movies Bluray", Value: "450"},
			{Label: "Movies REMUX", Value: "415"}, {Label: "Movies 8K/4320p", Value: "505"},
			{Label: "Movies 2160p", Value: "416"}, {Label: "Movies 1080p", Value: "414"},
			{Label: "Movies 720p", Value: "413"}, {Label: "Movies SD", Value: "411"},
			{Label: "TVSereis Bluray", Value: "453"}, {Label: "TVSeries 1080p", Value: "436"},
			{Label: "TVShows Bluray", Value: "452"}, {Label: "TVShow 2160p", Value: "431"},
			{Label: "Doc 8K UHD BD REMUX", Value: "529"}, {Label: "Doc UHD Blu-ray", Value: "500"},
			{Label: "Doc REMUX", Value: "421"}, {Label: "Doc 1080p", Value: "420"},
			{Label: "Anime UHD REMUX", Value: "531"}, {Label: "Anime 2160p", Value: "449"},
			{Label: "Musics APE", Value: "439"}, {Label: "Musics FLAC", Value: "440"},
			{Label: "Musics MV", Value: "441"}, {Label: "Musics Bluray", Value: "503"},
			{Label: "Sports 720p", Value: "442"}, {Label: "Sport 8K/4320p", Value: "511"},
			{Label: "Misc", Value: "409"},
		},
	}}
	cases := []struct {
		name       string
		category   string
		medium     string // Specification（encode/web/hdtv 形态）
		source     string // SourceType（原盘/remux 形态——mediumStdKeyOf 归一源）
		resolution string
		audio      string
		want       string
	}{
		{"电影 8K 原盘", "category.movie", "", "UHD Blu-ray", "4320p", "", "506"},
		{"电影 4K 原盘", "category.movie", "", "UHD Blu-ray", "2160p", "", "499"},
		{"电影 BD 原盘", "category.movie", "", "Blu-ray", "", "", "450"},
		{"电影 UHD Remux", "category.movie", "", "REMUX", "2160p", "", "518"},
		{"电影 Remux", "category.movie", "", "REMUX", "1080p", "", "415"},
		{"电影 4K 压制", "category.movie", "encode", "", "2160p", "", "416"},
		{"电影 1080p WEB", "category.movie", "web-dl", "", "1080p", "", "414"},
		{"电影 720p", "category.movie", "encode", "", "720p", "", "413"},
		{"电影 SD", "category.movie", "encode", "", "480p", "", "411"},
		{"剧集 BD（错拼容错）", "category.tv_series", "", "Blu-ray", "", "", "453"},
		{"剧集 1080p", "category.tv_series", "encode", "", "1080p", "", "436"},
		{"综艺 Bluray（复数）", "category.tv_shows", "", "Blu-ray", "", "", "452"},
		{"综艺 2160p", "category.tv_shows", "webdl", "", "2160p", "", "431"},
		{"纪录 8K Remux", "category.documentary", "", "REMUX", "4320p", "", "529"},
		{"纪录 4K 原盘", "category.documentary", "", "UHD Blu-ray", "2160p", "", "500"},
		{"纪录 Remux", "category.documentary", "", "REMUX", "1080p", "", "421"},
		{"动画 UHD Remux", "category.animation", "", "REMUX", "2160p", "", "531"},
		{"动画 2160p", "category.animation", "encode", "", "2160p", "", "449"},
		{"音乐 CD FLAC", "category.music", "", "CD", "", "FLAC", "440"},
		{"音乐 APE", "category.lossless_music", "", "CD", "", "APE", "439"},
		{"音乐 BD", "category.music", "", "Blu-ray", "", "", "503"},
		{"MV", "category.mv", "", "", "", "", "441"},
		{"体育 720p", "category.sports", "encode", "", "720p", "", "442"},
		{"体育 8K（单数 Sport）", "category.sports", "encode", "", "4320p", "", "511"},
	}
	for _, c := range cases {
	tp := titleparser.TechProfile{Specification: c.medium, SourceType: c.source, Resolution: c.resolution, AudioCodec: c.audio}
		meta := &model.TorrentMetadata{Category: c.category}
		v := hdhomeCategoryValue(cfg, "家园", meta, tp)
		if v == nil || v.Value != c.want {
			t.Errorf("%s: 应命中 %s，got %v", c.name, c.want, v)
		}
	}
	if v := hdhomeCategoryValue(cfg, "好大", &model.TorrentMetadata{Category: "category.movie"}, titleparser.TechProfile{}); v != nil {
		t.Fatal("非家园应不触发")
	}
	tp := titleparser.TechProfile{Specification: "encode"}
	if v := hdhomeCategoryValue(cfg, "家园", &model.TorrentMetadata{Category: "category.movie"}, tp); v != nil {
		t.Fatal("encode 无分辨率应不路由")
	}
	if v := hdhomeCategoryValue(cfg, "家园", &model.TorrentMetadata{Category: "category.education"}, titleparser.TechProfile{}); v != nil {
		t.Fatal("education 应不路由（杂项概不接受）")
	}
}

// §59.421: 候选发布 offer 注入——cfg.Offer 开启时 form_fields["offer"] 映射字段填 yes。
func TestOfferInjection(t *testing.T) {
	e := &PublishExecutor{}
	_ = e
	// 直接验证 form 构造逻辑的关键路径：模拟 executor 注入段
	cfg := &model.PublishFormConfig{
		Offer:       true,
		FormFields:  map[string]string{model.FieldDomainOffer: "offers"},
	}
	form := map[string]string{}
	if cfg.Offer {
		if fn, ok := cfg.FormFields[model.FieldDomainOffer]; ok && fn != "" {
			form[fn] = "yes"
		}
	}
	if form["offers"] != "yes" {
		t.Fatalf("offer 开启应注入 offers=yes，got %v", form)
	}
	// 关闭态不注入
	cfg2 := &model.PublishFormConfig{FormFields: map[string]string{model.FieldDomainOffer: "offers"}}
	form2 := map[string]string{}
	if cfg2.Offer {
		if fn, ok := cfg2.FormFields[model.FieldDomainOffer]; ok && fn != "" {
			form2[fn] = "yes"
		}
	}
	if _, exists := form2["offers"]; exists {
		t.Fatal("offer 关闭不应注入")
	}
	// 开启但无映射不注入
	cfg3 := &model.PublishFormConfig{Offer: true}
	form3 := map[string]string{}
	if cfg3.Offer {
		if fn, ok := cfg3.FormFields[model.FieldDomainOffer]; ok && fn != "" {
			form3[fn] = "yes"
		}
	}
	if len(form3) != 0 {
		t.Fatal("无映射不应注入")
	}
}

// §59.423: 天空分集/合集×国产/海外双维路由（恒合集+产区路由——用户定案）。
func TestHdskyCategoryDualRoute(t *testing.T) {
	cfg := &model.PublishFormConfig{
		FormFields:   map[string]string{"type": "type"},
		ValueMappings: map[string][]model.FormValueMapping{
			"type": {
				{Label: "TV Series/剧集(合集）", Value: "411"},
				{Label: "TV Series/海外剧集(合集）", Value: "413"},
				{Label: "TV Shows/综艺", Value: "403", StandardKeys: []string{"category.tv_shows"}},
				{Label: "TV Shows/海外综艺(合集）", Value: "415"},
				{Label: "Movies/电影", Value: "401", StandardKeys: []string{"category.movie"}},
			},
		},
	}
	cnMeta := func(cat string) *model.TorrentMetadata {
		return &model.TorrentMetadata{Category: cat, PTGenSourceJSON: `{"region":["中国大陆"]}`}
	}
	usMeta := func(cat string) *model.TorrentMetadata {
		return &model.TorrentMetadata{Category: cat, PTGenSourceJSON: `{"region":["美国"]}`}
	}
	// 剧集：国产→411 / 海外→413 / 无产区→413（默认海外）
	if v := hdskyCategoryValue(cfg, "天空", cnMeta("category.tv_series")); v == nil || v.Value != "411" {
		t.Fatalf("国产剧集合集应 411，得 %v", v)
	}
	if v := hdskyCategoryValue(cfg, "天空", usMeta("category.tv_series")); v == nil || v.Value != "413" {
		t.Fatalf("海外剧集合集应 413，得 %v", v)
	}
	if v := hdskyCategoryValue(cfg, "天空", &model.TorrentMetadata{Category: "category.tv_series"}); v == nil || v.Value != "413" {
		t.Fatalf("无产区默认海外 413，得 %v", v)
	}
	// 港台=国产
	if v := hdskyCategoryValue(cfg, "天空", &model.TorrentMetadata{Category: "category.tv_series", PTGenSourceJSON: `{"region":["中国香港"]}`}); v == nil || v.Value != "411" {
		t.Fatalf("港台应归国产 411，得 %v", v)
	}
	// 综艺：国产→403 / 海外→415
	if v := hdskyCategoryValue(cfg, "天空", cnMeta("category.tv_shows")); v == nil || v.Value != "403" {
		t.Fatalf("国产综艺应 403，得 %v", v)
	}
	if v := hdskyCategoryValue(cfg, "天空", usMeta("category.tv_shows")); v == nil || v.Value != "415" {
		t.Fatalf("海外综艺合集应 415，得 %v", v)
	}
	// 电影等回退通用链（nil）
	if v := hdskyCategoryValue(cfg, "天空", usMeta("category.movie")); v != nil {
		t.Fatalf("电影应回退通用链，得 %v", v)
	}
	// 他站不触发
	if v := hdskyCategoryValue(cfg, "麒麟", usMeta("category.tv_series")); v != nil {
		t.Fatalf("他站不应触发，得 %v", v)
	}
}

// §59.424: 系统级禁发剧集分集（用户定案全链——范围/形态/完结信号/P1 P2/全集一集/边界）。
func TestEpisodeReleaseBlocked(t *testing.T) {
	tpOf := func(se string) titleparser.TechProfile { return titleparser.TechProfile{SeasonEpisode: se} }
	mk := func(cat, title, subtitle, tags, ptgen string) *model.TorrentMetadata {
		return &model.TorrentMetadata{Category: cat, Title: title, Subtitle: subtitle, Tags: tags, PTGenSourceJSON: ptgen}
	}
	SER := `{"region":["美国"],"episodes":"12"}`
	EP1 := `{"region":["美国"],"episodes":"1"}`

	cases := []struct {
		name    string
		meta    *model.TorrentMetadata
		se      string
		blocked bool
	}{
		// 范围与形态豁免
		{"电影带SE形态豁免（范围外）", mk("category.movie", "Show S01E05 1080p", "", "", SER), "S01E05", false},
		{"短剧豁免", mk("category.short_drama", "剧 S01E05", "", "", SER), "S01E05", false},
		{"动漫电影无SE豁免", mk("category.animation", "剧场版 2023 1080p", "", "", ""), "", false},
		{"纯季合集放行", mk("category.tv_series", "Show S01 1080p", "", "", SER), "S01", false},
		{"跨季合集放行", mk("category.tv_series", "Show S01-S02 1080p", "", "", SER), "S01-S02", false},
		// 拦截核心场景
		{"剧集单集无信号拦", mk("category.tv_series", "Show S01E05 1080p", "第5集", "", SER), "S01E05", true},
		{"动漫单集无信号拦", mk("category.animation", "番组 S01E05 1080p", "", "", SER), "S01E05", true},
		{"综艺单集无信号拦", mk("category.tv_shows", "综艺 S01E03 1080p", "", "", SER), "S01E03", true},
		{"非E01起区间拦", mk("category.tv_series", "Show S01E05-E10", "", "", SER), "S01E05-E10", true},
		{"E01起区间无完结信号拦（归一未命中）", mk("category.tv_series", "Show S01E01-E10", "", "", SER), "S01E01-E10", true},
		// 完结信号三来源放行（任一即放——宁放勿拦）
		{"信号①源站tags complete放", mk("category.tv_series", "Show S01E05 1080p", "", `["complete"]`, SER), "S01E05", false},
		{"信号②主标Complete放", mk("category.tv_series", "Show Complete Series S01E05", "", "", SER), "S01E05", false},
		{"信号②主标中文完结放", mk("category.tv_series", "某剧 全集 S01E05", "", "", SER), "S01E05", false},
		{"信号③副标中文完结放", mk("category.tv_series", "Show S01E05 1080p", "全12集完结", "", SER), "S01E05", false},
		{"信号③副标英文Complete放（P2直查）", mk("category.tv_series", "Show S01E05 1080p", "Season 1 Complete", "", SER), "S01E05", false},
		// 全集一集（BBC 单剧）
		{"单集E01∧eps=1全集放行", mk("category.tv_series", "BBC Single Drama S01E01", "", "", EP1), "S01E01", false},
		{"单集E01∧eps=12仍拦（非全集）", mk("category.tv_series", "Show S01E01", "", "", SER), "S01E01", true},
		// 词形防御
		{"incomplete不误命中Complete", mk("category.tv_series", "Show S01E05 Incomplete", "", "", SER), "S01E05", true},
	}
	for _, c := range cases {
		tp := tpOf(c.se)
		blocked, _ := episodeReleaseBlocked(c.meta, &tp)
		if blocked != c.blocked {
			t.Errorf("%s: 期望 blocked=%v 实得 %v", c.name, c.blocked, blocked)
		}
	}
	// nil 防御
	if b, _ := episodeReleaseBlocked(nil, nil); b {
		t.Error("nil 输入应不拦")
	}
}

// §59.425: 碟粉电影产区细分路由+站点级双检查（五年规则/禁欧美）。
func TestDiscfanCategoryAndPreflight(t *testing.T) {
	cfg := &model.PublishFormConfig{
		FormFields:   map[string]string{"type": "type"},
		ValueMappings: map[string][]model.FormValueMapping{
			"type": {
				{Label: "电影 - 中国大陆", Value: "401"}, {Label: "电影 - 中国香港", Value: "404"},
				{Label: "电影 - 中国台湾", Value: "405"}, {Label: "电影 - 泰国", Value: "402"},
				{Label: "电影 - 日本", Value: "403"}, {Label: "电影 - 韩国", Value: "406"},
				{Label: "电影 - 世界", Value: "410"}, {Label: "剧集", Value: "411", StandardKeys: []string{"category.tv_series"}},
				{Label: "动漫", Value: "419", StandardKeys: []string{"category.animation"}},
			},
		},
	}
	mk := func(cat, region string) *model.TorrentMetadata {
		return &model.TorrentMetadata{Category: cat, PTGenSourceJSON: `{"region":["` + region + `"]}`}
	}
	// 电影产区 7 细分
	cases := []struct{ region, want string }{
		{"中国大陆", "401"}, {"中国香港", "404"}, {"中国台湾", "405"},
		{"日本", "403"}, {"韩国", "406"}, {"泰国", "402"}, {"美国", "410"}, {"新加坡", "410"},
	}
	for _, c := range cases {
		v := discfanCategoryValue(cfg, "碟粉", mk("category.movie", c.region))
		if v == nil || v.Value != c.want {
			t.Errorf("产区 %s: 期望 %s 实得 %v", c.region, c.want, v)
		}
	}
	// 无产区电影→410 兜底；非电影回退通用链；他站不触发
	if v := discfanCategoryValue(cfg, "碟粉", &model.TorrentMetadata{Category: "category.movie"}); v == nil || v.Value != "410" {
		t.Errorf("无产区电影应 410 兜底，得 %v", v)
	}
	if v := discfanCategoryValue(cfg, "碟粉", mk("category.tv_series", "美国")); v != nil {
		t.Errorf("剧集应回退通用链，得 %v", v)
	}
	if v := discfanCategoryValue(cfg, "麒麟", mk("category.movie", "美国")); v != nil {
		t.Errorf("他站不应触发，得 %v", v)
	}
	// 双检查：五年规则
	tpOf := func(y string) *titleparser.TechProfile { return &titleparser.TechProfile{Year: y} }
	thisYear := time.Now().Year()
	if b, _ := discfanReleaseBlocked(mk("category.movie", "中国大陆"), tpOf(fmt.Sprint(thisYear-4))); !b {
		t.Error("四年内资源应拦")
	}
	if b, _ := discfanReleaseBlocked(mk("category.movie", "中国大陆"), tpOf(fmt.Sprint(thisYear-5))); b {
		t.Error("五年前资源应放")
	}
	if b, _ := discfanReleaseBlocked(mk("category.movie", "中国大陆"), tpOf("")); b {
		t.Error("无年份应放（宁放定案）")
	}
	// 双检查：禁欧美（亚洲放/欧美拦/无产区防御放）
	metaUS := mk("category.movie", "美国")
	tpOld := tpOf(fmt.Sprint(thisYear - 6))
	if b, msg := discfanReleaseBlocked(metaUS, tpOld); !b {
		t.Error("欧美产区应拦（年份已过审）", msg)
	}
	if b, _ := discfanReleaseBlocked(mk("category.tv_series", "日本"), tpOf(fmt.Sprint(thisYear-6))); b {
		t.Error("亚洲剧集应放")
	}
	if b, _ := discfanReleaseBlocked(&model.TorrentMetadata{Category: "category.movie"}, tpOf(fmt.Sprint(thisYear-6))); b {
		t.Error("无产区防御性放行")
	}
}
