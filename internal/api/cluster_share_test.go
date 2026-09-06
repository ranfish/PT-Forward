package api

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/metadata"
	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func clusterTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}

// §59.61 第 4 步: 簇传播——获取成功后站点数据+MI 复制到无数据副本行
func TestPropagateClusterMetadata(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	// 簇: self + 2 siblings（快照）
	for _, hh := range []string{"selfhash0000000000000000000000000000000000000", "sib10000000000000000000000000000000000000", "sib20000000000000000000000000000000000000"} {
		db.Create(&model.TorrentSnapshot{Hash: hh, ClientID: "PT0", Name: "Arco.2025", SavePath: "/x"})
	}
	// self 完整 metadata
	db.Create(&model.TorrentMetadata{InfoHash: "selfhash0000000000000000000000000000000000000", SiteName: "朋友", TorrentID: "123", Title: "Arco", Description: "d", Poster: "p", MediaInfo: "mi", MediaInfoSource: "local"})
	// sibling1 已有自己的行（不覆盖）；sibling2 无行（应复制）
	db.Create(&model.TorrentMetadata{InfoHash: "sib10000000000000000000000000000000000000", SiteName: "财神", Title: "existing"})

	h.propagateClusterMetadata(context.Background(), "PT0", "/x", "Arco.2025",
		"selfhash0000000000000000000000000000000000000", "朋友")

	var cpy model.TorrentMetadata
	if err := db.Where("info_hash = ?", "sib20000000000000000000000000000000000000").First(&cpy).Error; err != nil {
		t.Fatalf("sibling2 应有复制行: %v", err)
	}
	if cpy.SiteName != "朋友" || cpy.TorrentID != "123" || cpy.MediaInfo != "mi" || cpy.MediaInfoSource != "local" {
		t.Errorf("复制内容不完整: %+v", cpy)
	}
	if cpy.FetchSource != "cluster" {
		t.Errorf("fetch_source 应为 cluster: %q", cpy.FetchSource)
	}
	var sib1 model.TorrentMetadata
	db.Where("info_hash = ?", "sib10000000000000000000000000000000000000").First(&sib1)
	if sib1.SiteName != "财神" {
		t.Errorf("已有行不应被覆盖: %+v", sib1)
	}
}

// §59.61 第 4 步: 批内跳过——簇内已有完整数据（含传播行）的副本跳过获取
func TestHasCompleteMetadata(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	empty := "e000000000000000000000000000000000000000"
	full := "f000000000000000000000000000000000000000"
	if h.hasCompleteMetadata(context.Background(), empty) {
		t.Error("无行应返回 false")
	}
	db.Create(&model.TorrentMetadata{InfoHash: full, SiteName: "朋友", Title: "t", Description: "d"})
	if !h.hasCompleteMetadata(context.Background(), full) {
		t.Error("完整行应返回 true")
	}
}

// §59.61 第 4 步: 截图二次传播——策略完成后补簇内空截图行
func TestPropagateClusterScreenshots(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	db.Create(&model.TorrentSnapshot{Hash: "shotself0000000000000000000000000000000000", ClientID: "PT0", Name: "N", SavePath: "/s"})
	db.Create(&model.TorrentSnapshot{Hash: "shotsib00000000000000000000000000000000000", ClientID: "PT0", Name: "N", SavePath: "/s"})
	db.Create(&model.TorrentMetadata{InfoHash: "shotself0000000000000000000000000000000000", SiteName: "朋友", Title: "t", Screenshots: `["https://a/1.jpg"]`})
	db.Create(&model.TorrentMetadata{InfoHash: "shotsib00000000000000000000000000000000000", SiteName: "朋友", Title: "t", Screenshots: ""})
	// 已有截图的第三方行（不同簇）不应被写
	db.Create(&model.TorrentMetadata{InfoHash: "other0000000000000000000000000000000000000", SiteName: "朋友", Title: "t", Screenshots: ""})

	h.propagateClusterScreenshots(context.Background(), "PT0", "/s", "N",
		"shotself0000000000000000000000000000000000", `["https://a/1.jpg"]`)

	var sib model.TorrentMetadata
	db.Where("info_hash = ?", "shotsib00000000000000000000000000000000000").First(&sib)
	if sib.Screenshots != `["https://a/1.jpg"]` {
		t.Errorf("簇内空行应补截图: %q", sib.Screenshots)
	}
	var other model.TorrentMetadata
	db.Where("info_hash = ?", "other0000000000000000000000000000000000000").First(&other)
	if other.Screenshots != "" {
		t.Error("非簇行不应被写")
	}
}

// §59.61 附: 传播竞态修复——异步修复（PTGen 海报/简介、截图策略）完成后必须回传簇行。
// 场景实锤（243 墓碑镇 57 副本）: t1 传播不完整态 → t2 PTGen 只修首副本 → 簇行永久裂图。
func TestPropagateClusterScreenshots_OverwritePartialClusterRows(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	// 簇行带 2 张部分截图（fetch_source=cluster）——必须被覆盖（非仅空行）
	db.Create(&model.TorrentSnapshot{Hash: "pself000000000000000000000000000000000000", ClientID: "PT0", Name: "P", SavePath: "/p"})
	db.Create(&model.TorrentSnapshot{Hash: "psib0000000000000000000000000000000000000", ClientID: "PT0", Name: "P", SavePath: "/p"})
	db.Create(&model.TorrentMetadata{InfoHash: "pself000000000000000000000000000000000000", SiteName: "朋友", Title: "t", Screenshots: `["https://a/5.jpg"]`})
	db.Create(&model.TorrentMetadata{InfoHash: "psib0000000000000000000000000000000000000", SiteName: "朋友", Title: "t", Screenshots: `["https://a/1.jpg","https://a/2.jpg"]`, FetchSource: "cluster"})

	h.propagateClusterScreenshots(context.Background(), "PT0", "/p", "P",
		"pself000000000000000000000000000000000000", `["https://a/5.jpg"]`)

	var sib model.TorrentMetadata
	db.Where("info_hash = ?", "psib0000000000000000000000000000000000000").First(&sib)
	if sib.Screenshots != `["https://a/5.jpg"]` {
		t.Errorf("cluster 部分截图行应被覆盖: %q", sib.Screenshots)
	}
}

// PTGen 修复回传: propagateClusterPosters 把首副本终态 poster/description 复制到簇行
func TestPropagateClusterPosters(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	db.Create(&model.TorrentSnapshot{Hash: "qself000000000000000000000000000000000000", ClientID: "PT0", Name: "Q", SavePath: "/q"})
	db.Create(&model.TorrentSnapshot{Hash: "qsib0000000000000000000000000000000000000", ClientID: "PT0", Name: "Q", SavePath: "/q"})
	// 首副本已被 PTGen 修复
	db.Create(&model.TorrentMetadata{InfoHash: "qself000000000000000000000000000000000000", SiteName: "朋友", Title: "t",
		Poster: "https://doubaninfo.com/dbposter/x.jpg", Description: "PTGen简介", FetchSource: "rss_detail"})
	// 簇行残留裂图+垃圾简介
	db.Create(&model.TorrentMetadata{InfoHash: "qsib0000000000000000000000000000000000000", SiteName: "朋友", Title: "t",
		Poster: "https://img.keepfrds.com/dead", Description: "[url=javascript:void(0)] MediaInfo: x.", FetchSource: "cluster"})
	// 同簇但有独立数据的行（rss_detail）——不可被覆盖
	db.Create(&model.TorrentSnapshot{Hash: "qown0000000000000000000000000000000000000", ClientID: "PT0", Name: "Q", SavePath: "/q"})
	db.Create(&model.TorrentMetadata{InfoHash: "qown0000000000000000000000000000000000000", SiteName: "朋友", Title: "t",
		Poster: "own", Description: "own-desc", FetchSource: "rss_detail"})

	h.propagateClusterPosters(context.Background(), "PT0", "/q", "Q", "qself000000000000000000000000000000000000")

	var sib model.TorrentMetadata
	db.Where("info_hash = ?", "qsib0000000000000000000000000000000000000").First(&sib)
	if sib.Poster != "https://doubaninfo.com/dbposter/x.jpg" || sib.Description != "PTGen简介" {
		t.Errorf("簇行应被 PTGen 终态覆盖: poster=%q desc=%q", sib.Poster, sib.Description)
	}
	var own model.TorrentMetadata
	db.Where("info_hash = ?", "qown0000000000000000000000000000000000000").First(&own)
	if own.Poster != "own" || own.Description != "own-desc" {
		t.Errorf("独立数据行（非 cluster）不应被覆盖: %+v", own)
	}
}

// §59.61 附2: propagateClusterPosters 不依赖内存 map——从 snapshots 反查簇上下文。
// 4005 批次实锤: posterClusterCtx 容量清空把尾部 3 部上下文丢掉 → PTGen 修复不回传。
func TestPropagateClusterPosters_NoMemoryMap(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	db.Create(&model.TorrentSnapshot{Hash: "rself000000000000000000000000000000000000", ClientID: "PT0", Name: "R", SavePath: "/r"})
	db.Create(&model.TorrentSnapshot{Hash: "rsib0000000000000000000000000000000000000", ClientID: "PT0", Name: "R", SavePath: "/r"})
	db.Create(&model.TorrentMetadata{InfoHash: "rself000000000000000000000000000000000000", SiteName: "朋友", Title: "t",
		Poster: "https://doubaninfo.com/dbposter/y.jpg", Description: "d2", FetchSource: "rss_detail"})
	db.Create(&model.TorrentMetadata{InfoHash: "rsib0000000000000000000000000000000000000", SiteName: "朋友", Title: "t",
		Poster: "https://img.keepfrds.com/dead2", FetchSource: "cluster"})

	// 不注册 posterClusterCtx（模拟 map 被清空/进程重启后）——直接调用也必须回传成功
	h.propagateClusterPosters(context.Background(), "PT0", "/r", "R", "rself000000000000000000000000000000000000")
	var sib model.TorrentMetadata
	db.Where("info_hash = ?", "rsib0000000000000000000000000000000000000").First(&sib)
	if sib.Poster != "https://doubaninfo.com/dbposter/y.jpg" {
		t.Errorf("反查回传失败: %q", sib.Poster)
	}
}

// §59.61 附2: clusterCtxFor 反查——map 空/丢失时从 snapshots 恢复上下文
func TestClusterCtxFor_FallbackToSnapshots(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	db.Create(&model.TorrentSnapshot{Hash: "ctxhash00000000000000000000000000000000", ClientID: "PT7", Name: "Ctx", SavePath: "/ctx"})
	// map 为空（未注册）
	c, ok := h.clusterCtxFor(context.Background(), "ctxhash00000000000000000000000000000000")
	if !ok || c.clientID != "PT7" || c.name != "Ctx" || c.savePath != "/ctx" {
		t.Errorf("反查失败: %+v ok=%v", c, ok)
	}
	// 未知 hash
	if _, ok := h.clusterCtxFor(context.Background(), "nonexist000000000000000000000000000000"); ok {
		t.Error("未知 hash 应返回 false")
	}
}

// §59.61 附5: 尾部终局传播——fetchSingleTorrent 的 INSERT 循环与异步 applyPosterFallback
// 的回传 UPDATE 并发竞态（疯狂动物城2 BluRay 27/54 行残留站点态实锤）。修复:
// finalizeClusterPropagation 等 fallback 终局后 INSERT + 终态回传。
func TestFinalizeClusterPropagation_WaitsForFallback(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	db.Create(&model.TorrentSnapshot{Hash: "fself00000000000000000000000000000000000", ClientID: "PT0", Name: "F", SavePath: "/f"})
	db.Create(&model.TorrentSnapshot{Hash: "fsib000000000000000000000000000000000000", ClientID: "PT0", Name: "F", SavePath: "/f"})
	// 首副本初始为站点态（fallback 尚未完成）
	db.Create(&model.TorrentMetadata{InfoHash: "fself00000000000000000000000000000000000", SiteName: "朋友", Title: "t",
		Poster: "https://img.keepfrds.com/site", Description: "site-desc", FetchSource: "rss_detail"})

	// 模拟异步 fallback: 50ms 后把首副本修复为 PTGen 终态
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		db.Model(&model.TorrentMetadata{}).
			Where("info_hash = ?", "fself00000000000000000000000000000000000").
			Update("poster", "https://doubaninfo.com/dbposter/f.jpg")
	}()

	h.finalizeClusterPropagation(context.Background(), &wg, "PT0", "/f", "F",
		"fself00000000000000000000000000000000000", "朋友")

	var sib model.TorrentMetadata
	db.Where("info_hash = ?", "fsib000000000000000000000000000000000000").First(&sib)
	if sib.Poster != "https://doubaninfo.com/dbposter/f.jpg" {
		t.Errorf("尾部传播必须等 fallback 终局后携带终态: poster=%q", sib.Poster)
	}
	if sib.FetchSource != "cluster" {
		t.Errorf("传播行应为 cluster: %q", sib.FetchSource)
	}
}

// §59.70: PTGen 简介落库后 t2 重推标签——豆瓣评分行在 t0 不存在，t2 补推 high_rating。
func TestRefreshInferredTags_AfterPTGen(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	hash := "hself00000000000000000000000000000000000"
	db.Create(&model.TorrentMetadata{InfoHash: hash, SiteName: "朋友", Title: "电影.2024.1080p",
		Description: "◎豆瓣评分　8.4/10 (12345 人评价)\n◎简　介　剧情", Tags: `["chinese_subtitle"]`, FetchSource: "rss_detail"})

	h.refreshInferredTags(context.Background(), hash, "朋友")

	var m model.TorrentMetadata
	db.Where("info_hash = ?", hash).First(&m)
	var tags []string
	_ = json.Unmarshal([]byte(m.Tags), &tags)
	has := func(k string) bool { for _, x := range tags { if x == k { return true } }; return false }
	if !has("high_rating") {
		t.Errorf("t2 重推应补 high_rating: %v", tags)
	}
	if !has("chinese_subtitle") {
		t.Errorf("既有标签不可丢: %v", tags)
	}
}

// §59.75: t2 PTGen 源持久化——ptgen_source_json 落库（Region/Genre 结构化资产，
// Tab1 产地/类型展示 + 未来发布映射消费）。接入遗漏修复：机制（列/Marshal）早已有，
// 获取链 querier 只取 RawBBCode/PosterURL 把完整 result 丢了。
func TestApplyPTGenSourcePersist(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	hash := "pgen00000000000000000000000000000000000"
	db.Create(&model.TorrentMetadata{InfoHash: hash, SiteName: "朋友", Title: "t",
		Poster: "https://img.keepfrds.com/x", FetchSource: "rss_detail"})

	h.persistPTGenSource(context.Background(), hash, "朋友", &model.PTGenResult{
		Region:       []string{"美国", "英国"},
		Genre:        []string{"剧情", "动作", "科幻"},
		ChineseTitle: "测试片",
	})

	var m model.TorrentMetadata
	db.Where("info_hash = ?", hash).First(&m)
	if m.PTGenSourceJSON == "" {
		t.Fatal("ptgen_source_json 应落库")
	}
	src, err := metadata.UnmarshalPTGenSource(m.PTGenSourceJSON)
	if err != nil || src == nil {
		t.Fatalf("Unmarshal 失败: %v", err)
	}
	if len(src.Region) != 2 || src.Region[0] != "美国" {
		t.Errorf("Region 应持久化: %v", src.Region)
	}
	if len(src.Genre) != 3 {
		t.Errorf("Genre 应持久化: %v", src.Genre)
	}
}

// §59.80: 源站无声明时追加致谢不得产生前导空行（米仔睡着了实锤
// '\n\n[quote]FRDS官组作品...'——thanks 固定 \n\n 前缀 + 空 base）。
func TestAppendThanksNoLeadingBlank(t *testing.T) {
	thanks := "[quote][b][color=blue][size=5]FRDS官组作品，感谢原制作者发布。[/size][/color][/b][/quote]\n[quote][b][color=red][size=5]请遵守PT互相遵重共识，禁转PTT[/size][/color][/b][/quote]"
	join := func(base string) string {
		if base == "" {
			return thanks
		}
		return base + "\n\n" + thanks
	}
	if got := join(""); strings.HasPrefix(got, "\n") {
		t.Errorf("空 base 不应前导空行: %q", got[:30])
	}
	base := "[quote]源站声明[/quote]"
	got := join(base)
	if !strings.HasPrefix(got, base+"\n\n") {
		t.Errorf("非空 base 保持双换行分隔: %q", got[:40])
	}
}

// §59.89: PUT 空值不覆盖——只改 tags 不带 description 时简介不得被清空
//（v0.0.738 验证脚本 PUT {} 污染预览实锤的根因；前端部分保存场景同风险）。
func TestPutSeedEmptyNotOverwrite(t *testing.T) {
	// 直接测 updates 构造语义: buildPutSeedUpdates 纯函数化
	u := buildPutSeedUpdates(putSeedRequest{Tags: []string{"high_bitrate"}}, "poster0", "desc0", "shots0")
	if _, ok := u["description"]; ok {
		t.Errorf("空 description 不应覆盖: %v", u)
	}
	if _, ok := u["poster"]; ok {
		t.Errorf("空 poster 不应覆盖")
	}
	if _, ok := u["screenshots"]; ok {
		t.Errorf("空 screenshots 不应覆盖")
	}
	if u["tags"] != `["high_bitrate"]` {
		t.Errorf("tags 应写入: %v", u["tags"])
	}
	// 带值时正常写
	u2 := buildPutSeedUpdates(putSeedRequest{Poster: "p", Description: "d", Screenshots: []string{"s1"}}, "", "", "")
	if u2["poster"] != "p" || u2["description"] != "d" {
		t.Errorf("非空应写入: %v", u2)
	}
}

// §59.93: 审核状态簇同步——PUT reviewed 写行 + 簇内全部行（同簇数据相同，
// 审核一份数据=审核全簇；仙履奇缘 54 副本实锤：编辑行 reviewed=1 其余 53 行=0，
// 列表逐快照行展示时其他快照挂的兄弟行仍 pending）。
func TestPropagateClusterReviewed(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	// 簇: self + 2 兄弟
	db.Create(&model.TorrentSnapshot{Hash: "rself00000000000000000000000000000000000", ClientID: "PT0", Name: "R", SavePath: "/r"})
	db.Create(&model.TorrentSnapshot{Hash: "rsib10000000000000000000000000000000000", ClientID: "PT0", Name: "R", SavePath: "/r"})
	db.Create(&model.TorrentSnapshot{Hash: "rsib20000000000000000000000000000000000", ClientID: "PT0", Name: "R", SavePath: "/r"})
	db.Create(&model.TorrentMetadata{InfoHash: "rself00000000000000000000000000000000000", SiteName: "朋友", Title: "t", FetchSource: "rss_detail"})
	db.Create(&model.TorrentMetadata{InfoHash: "rsib10000000000000000000000000000000000", SiteName: "朋友", Title: "t", FetchSource: "cluster"})
	db.Create(&model.TorrentMetadata{InfoHash: "rsib20000000000000000000000000000000000", SiteName: "朋友", Title: "t", FetchSource: "cluster"})

	h.propagateClusterReviewed(context.Background(), "PT0", "/r", "R", "rself00000000000000000000000000000000000", true)

	var n int64
	db.Model(&model.TorrentMetadata{}).
		Where("info_hash IN ? AND reviewed = ?", []string{"rsib10000000000000000000000000000000000", "rsib20000000000000000000000000000000000"}, true).
		Count(&n)
	if n != 2 {
		t.Errorf("簇内兄弟行应同步 reviewed: %d", n)
	}
	// 取消同步
	h.propagateClusterReviewed(context.Background(), "PT0", "/r", "R", "rself00000000000000000000000000000000000", false)
	db.Model(&model.TorrentMetadata{}).
		Where("info_hash IN ? AND reviewed = ?", []string{"rsib10000000000000000000000000000000000", "rsib20000000000000000000000000000000000"}, false).
		Count(&n)
	if n != 2 {
		t.Errorf("取消也应同步: %d", n)
	}
}

// §59.94: 簇共享公共方法——clusterKeyOf(info_hash→簇键) + syncClusterReviewedByIDs
//（batchReview/saveSeedData 两写点统一接入）。
func TestClusterKeyOfAndSyncByIDs(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	db.Create(&model.TorrentSnapshot{Hash: "kself0000000000000000000000000000000000", ClientID: "PT1", Name: "K", SavePath: "/k"})
	db.Create(&model.TorrentSnapshot{Hash: "ksib00000000000000000000000000000000000", ClientID: "PT1", Name: "K", SavePath: "/k"})
	db.Create(&model.TorrentMetadata{InfoHash: "kself0000000000000000000000000000000000", SiteName: "朋友", Title: "t", ID: 9001})
	db.Create(&model.TorrentMetadata{InfoHash: "ksib00000000000000000000000000000000000", SiteName: "朋友", Title: "t", ID: 9002})

	// 簇键解析（hash → client/path/name）
	ck, ok := clusterKeyOf(db, "kself0000000000000000000000000000000000")
	if !ok || ck.name != "K" || ck.savePath != "/k" || ck.clientID != "PT1" {
		t.Fatalf("clusterKeyOf: %+v ok=%v", ck, ok)
	}
	// 按 metadata ID 批量同步 reviewed
	h.syncClusterReviewedByIDs(context.Background(), []uint{9001}, true)
	var n int64
	db.Model(&model.TorrentMetadata{}).Where("info_hash = ? AND reviewed = ?", "ksib00000000000000000000000000000000000", true).Count(&n)
	if n != 1 {
		t.Errorf("按 ID 同步应覆盖兄弟行: %d", n)
	}
}

// §59.169: 手动截图路径簇传播——propagateClusterScreenshotsDB 包级函数
// （海王2 45 副本死循环实锤：手动捕获只写单行，种子中心逐行展示下缺截图循环）。
func TestPropagateClusterScreenshotsDB_ManualCapturePath(t *testing.T) {
	db := clusterTestDB(t)
	logger := zap.NewNop()
	// 簇：1 源行 + 2 空副本 + 1 已有完整截图的独立获取行（rss_detail——不应被覆盖）
	db.Create(&model.TorrentSnapshot{Hash: "mself000000000000000000000000000000000000", ClientID: "PT0", Name: "M", SavePath: "/m"})
	db.Create(&model.TorrentSnapshot{Hash: "msib1000000000000000000000000000000000000", ClientID: "PT0", Name: "M", SavePath: "/m"})
	db.Create(&model.TorrentSnapshot{Hash: "msib2000000000000000000000000000000000000", ClientID: "PT0", Name: "M", SavePath: "/m"})
	db.Create(&model.TorrentMetadata{InfoHash: "mself000000000000000000000000000000000000", SiteName: "憨憨", Title: "t", Screenshots: `["https://a/9.jpg"]`})
	db.Create(&model.TorrentMetadata{InfoHash: "msib1000000000000000000000000000000000000", SiteName: "憨憨", Title: "t", Screenshots: ""})
	db.Create(&model.TorrentMetadata{InfoHash: "msib2000000000000000000000000000000000000", SiteName: "憨憨", Title: "t", Screenshots: "", FetchSource: "cluster"})
	db.Create(&model.TorrentMetadata{InfoHash: "mrss0000000000000000000000000000000000000", SiteName: "憨憨", Title: "t", Screenshots: `["https://a/old.jpg"]`, FetchSource: "rss_detail"})

	// 手动捕获路径：同簇键调用包级函数
	propagateClusterScreenshotsDB(db, logger, context.Background(), "PT0", "/m", "M",
		"mself000000000000000000000000000000000000", `["https://a/9.jpg"]`)

	var sib1, sib2, rssRow model.TorrentMetadata
	db.Where("info_hash = ?", "msib1000000000000000000000000000000000000").First(&sib1)
	db.Where("info_hash = ?", "msib2000000000000000000000000000000000000").First(&sib2)
	db.Where("info_hash = ?", "mrss0000000000000000000000000000000000000").First(&rssRow)
	if sib1.Screenshots != `["https://a/9.jpg"]` {
		t.Errorf("空副本应被补齐: %q", sib1.Screenshots)
	}
	if sib2.Screenshots != `["https://a/9.jpg"]` {
		t.Errorf("cluster 副本应被补齐: %q", sib2.Screenshots)
	}
	if rssRow.Screenshots != `["https://a/old.jpg"]` {
		t.Errorf("独立获取行(rss_detail)不应被覆盖: %q", rssRow.Screenshots)
	}

	// nil 守卫：db/logger 空、空 JSON、空簇键不 panic
	propagateClusterScreenshotsDB(nil, logger, context.Background(), "PT0", "/m", "M", "mself000000000000000000000000000000000000", `[]`)
	propagateClusterScreenshotsDB(db, nil, context.Background(), "PT0", "/m", "M", "mself000000000000000000000000000000000000", "")
	propagateClusterScreenshotsDB(db, logger, context.Background(), "", "/m", "M", "mself000000000000000000000000000000000000", `["https://a/9.jpg"]`)
}

// §59.170: 空 screenshots（[] 与 nil 同义）不覆盖不降级——直开预览自动保存
// 空表单的三连击防线（行覆写/reviewed 合成降级/簇连坐）前两层在此验证。
func TestPutSeedEmptyScreenshotsNotOverwrite(t *testing.T) {
	// ① 写入层：空数组不覆盖（nil 同义）
	u := buildPutSeedUpdates(putSeedRequest{Screenshots: []string{}}, "poster0", "desc0", `["https://a/1.jpg"]`)
	if _, ok := u["screenshots"]; ok {
		t.Errorf("空 screenshots 数组不应覆盖: %v", u)
	}
	// 非空正常写
	u2 := buildPutSeedUpdates(putSeedRequest{Screenshots: []string{"s1"}}, "", "", "")
	if u2["screenshots"] != `["s1"]` {
		t.Errorf("非空 screenshots 应写入: %v", u2["screenshots"])
	}

	// ② 审核合成层：空数组沿用库中已有截图 → 9 字段校验不误判缺截图
	// 直接复刻 handlePutSeed 的合成语义（纯逻辑段，不 mock HTTP）：
	meta := model.TorrentMetadata{Screenshots: `["https://a/1.jpg"]`}
	req := putSeedRequest{Screenshots: []string{}}
	existingShots := meta.Screenshots
	if len(req.Screenshots) > 0 {
		meta.Screenshots = model.FormatScreenshotColumn(req.Screenshots)
	} else {
		meta.Screenshots = existingShots
	}
	if len(model.ParseScreenshotColumn(meta.Screenshots)) != 1 {
		t.Errorf("空表单 PUT 应沿用库中截图（校验诚实），实际: %q", meta.Screenshots)
	}
}

// §59.171 A: 同名多行确定性排序——权威行（非 cluster）优先于更新的 cluster 副本
//（PT31 抽签实锤的反向验证：无序时 info_hash 索引序可让 cluster 空行当选）。
func TestSortMetasAuthoritative(t *testing.T) {
	metas := []model.TorrentMetadata{
		{ID: 3, FetchSource: "cluster", MediaInfo: "", UpdatedAt: time.Now()},
		{ID: 1, FetchSource: "rss_detail", MediaInfo: "MI-FULL", UpdatedAt: time.Now().Add(-time.Hour)},
		{ID: 2, FetchSource: "cluster", MediaInfo: "", UpdatedAt: time.Now()},
	}
	model.SortMetasAuthoritative(metas)
	if metas[0].ID != 1 || metas[0].FetchSource != "rss_detail" {
		t.Errorf("权威行应排首: %+v", metas[0])
	}
	// 两个 cluster 行：updated_at 相同 → id 升序
	if metas[1].ID > metas[2].ID {
		t.Errorf("同级行应 id 升序: %d, %d", metas[1].ID, metas[2].ID)
	}
	// selectSourceMeta 端到端：无制作组映射时 best=排序后首行（权威行）
	h := &PublishTorrentsHandler{db: nil, logger: zap.NewNop()}
	picked := h.selectSourceMeta(metas)
	if picked == nil || picked.ID != 1 {
		t.Errorf("selectSourceMeta 应选权威行: %+v", picked)
	}
}

// §59.171 B: MI 簇传播——空副本/cluster 副本补齐，rss_detail 独立行不动。
func TestPropagateClusterMediainfoDB(t *testing.T) {
	db := clusterTestDB(t)
	logger := zap.NewNop()
	db.Create(&model.TorrentSnapshot{Hash: "mself11000000000000000000000000000000000", ClientID: "PT0", Name: "W", SavePath: "/w"})
	db.Create(&model.TorrentSnapshot{Hash: "msib110000000000000000000000000000000000", ClientID: "PT0", Name: "W", SavePath: "/w"})
	db.Create(&model.TorrentSnapshot{Hash: "msib120000000000000000000000000000000000", ClientID: "PT0", Name: "W", SavePath: "/w"})
	db.Create(&model.TorrentMetadata{InfoHash: "mself11000000000000000000000000000000000", SiteName: "朋友", Title: "t", MediaInfo: "MI-LOCAL"})
	db.Create(&model.TorrentMetadata{InfoHash: "msib110000000000000000000000000000000000", SiteName: "朋友", Title: "t", MediaInfo: ""})
	db.Create(&model.TorrentMetadata{InfoHash: "msib120000000000000000000000000000000000", SiteName: "朋友", Title: "t", MediaInfo: "MI-OLD", FetchSource: "cluster"})
	// 独立获取行（不同簇键外的 rss 行——用其它名字簇的行模拟不动语义）
	db.Create(&model.TorrentMetadata{InfoHash: "mrss110000000000000000000000000000000000", SiteName: "朋友", Title: "t", MediaInfo: "MI-SRC", FetchSource: "rss_detail"})
	db.Create(&model.TorrentSnapshot{Hash: "mrss110000000000000000000000000000000000", ClientID: "PT0", Name: "OTHER", SavePath: "/w"})

	propagateClusterMediainfoDB(db, logger, context.Background(), "PT0", "/w", "W",
		"mself11000000000000000000000000000000000", "MI-LOCAL")

	var sib1, sib2, rssRow model.TorrentMetadata
	db.Where("info_hash = ?", "msib110000000000000000000000000000000000").First(&sib1)
	db.Where("info_hash = ?", "msib120000000000000000000000000000000000").First(&sib2)
	db.Where("info_hash = ?", "mrss110000000000000000000000000000000000").First(&rssRow)
	if sib1.MediaInfo != "MI-LOCAL" {
		t.Errorf("空副本应补 MI: %q", sib1.MediaInfo)
	}
	if sib2.MediaInfo != "MI-LOCAL" {
		t.Errorf("cluster 副本应被覆盖: %q", sib2.MediaInfo)
	}
	// §59.171 附: 标签随货走——media_info_source 一并写
	if sib1.MediaInfoSource != "local" || sib2.MediaInfoSource != "local" {
		t.Errorf("传播应带 MI 出处标签: %q/%q", sib1.MediaInfoSource, sib2.MediaInfoSource)
	}
	if rssRow.MediaInfo != "MI-SRC" {
		t.Errorf("非簇行不应被写: %q", rssRow.MediaInfo)
	}
	// nil/空守卫
	propagateClusterMediainfoDB(nil, logger, context.Background(), "PT0", "/w", "W", "mself11000000000000000000000000000000000", "MI")
	propagateClusterMediainfoDB(db, nil, context.Background(), "PT0", "/w", "W", "mself11000000000000000000000000000000000", "MI")
	propagateClusterMediainfoDB(db, logger, context.Background(), "PT0", "/w", "W", "mself11000000000000000000000000000000000", "")
}

// §59.171 D: MI 九字段校验口径——仅源站 MI（SourceMediaInfo）不再判缺。
func TestCheckRequiredFieldsMediainfoFallback(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	// 完整字段 + 仅 SourceMediaInfo（本地列空）
	meta := &model.TorrentMetadata{
		Title: "t", Poster: "p", Screenshots: `["1","2","3"]`, Description: "d",
		MediaInfo: "", SourceMediaInfo: "MI-FROM-SOURCE-SITE",
		Resolution: "1080p", VideoCodec: "h265",
	}
	missing := h.checkRequiredFields(meta)
	for _, m := range missing {
		if m == "mediainfo" {
			t.Errorf("仅源站 MI 不应判缺（本地优先+源站兜底设计口径）: %v", missing)
		}
	}
	// 双列皆空才判缺
	meta.SourceMediaInfo = ""
	missing2 := h.checkRequiredFields(meta)
	found := false
	for _, m := range missing2 {
		if m == "mediainfo" {
			found = true
		}
	}
	if !found {
		t.Errorf("双列皆空应判缺: %v", missing2)
	}
}
