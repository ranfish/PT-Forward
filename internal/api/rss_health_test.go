package api

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/setting"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newHealthTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.RSSFetchLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func addFetchLog(t *testing.T, db *gorm.DB, subID string, status string, detectCount, detectFailures int, errMsg string) {
	t.Helper()
	l := model.RSSFetchLog{
		SubscriptionID: subID,
		Status:         status,
		DetectCount:    detectCount,
		DetectFailures: detectFailures,
		ErrorMsg:       errMsg,
	}
	if err := db.Create(&l).Error; err != nil {
		t.Fatalf("create fetch log: %v", err)
	}
}

// §59.368: 订阅连续失败轮数反向统计——三类轮语义：
// ① status=error（拉取失败）计失败
// ② ok 且 detect 全败（隐性停摆）计失败
// ③ 健康（ok 正常 / detect_count=0 历史轮 / 部分失败有成功）为断点
func TestComputeSubHealth(t *testing.T) {
	db := newHealthTestDB(t)
	h := &RSSHandler{db: db}

	// 订阅 A：error → 检测全败 → error（连续 3 轮失败，最新 error 带原因）
	addFetchLog(t, db, "1", "ok", 10, 3, "")    // 最早：部分失败=健康断点之外
	addFetchLog(t, db, "1", "error", 0, 0, "请求失败: 超时")
	addFetchLog(t, db, "1", "ok", 50, 50, "")   // 检测全败（隐性停摆）
	addFetchLog(t, db, "1", "error", 0, 0, "读取响应失败: xxx")
	f, capped, reason, first := h.computeSubHealth(context.Background(), 1)
	if f != 3 || capped {
		t.Fatalf("订阅 A 连续失败应 3 轮未截断，got %d capped=%v", f, capped)
	}
	if reason == "" || first == nil {
		t.Fatal("最新失败轮应有原因与首败时间")
	}

	// 订阅 B：最新轮健康 → 计数断（历史失败不累计）
	addFetchLog(t, db, "2", "error", 0, 0, "超时")
	addFetchLog(t, db, "2", "ok", 5, 0, "")
	f2, _, _, _ := h.computeSubHealth(context.Background(), 2)
	if f2 != 0 {
		t.Fatalf("最新轮健康应计数归零，got %d", f2)
	}

	// 订阅 C：detect_count=0 的 ok 历史轮（migration58 前存量）=健康
	addFetchLog(t, db, "3", "ok", 0, 0, "")
	f3, _, _, _ := h.computeSubHealth(context.Background(), 3)
	if f3 != 0 {
		t.Fatalf("存量历史轮（无 detect 数据）应判健康，got %d", f3)
	}

	// 订阅 D：50 条全失败 → capped
	for i := 0; i < 50; i++ {
		addFetchLog(t, db, "4", "ok", 10, 10, "")
	}
	f4, capped4, _, _ := h.computeSubHealth(context.Background(), 4)
	if f4 != 50 || !capped4 {
		t.Fatalf("50 条全失败应 capped=50+，got %d capped=%v", f4, capped4)
	}

	// 订阅 E：无任何轮（新建订阅）
	f5, capped5, reason5, first5 := h.computeSubHealth(context.Background(), 99)
	if f5 != 0 || capped5 || reason5 != "" || first5 != nil {
		t.Fatalf("无轮订阅应零态，got %d/%v/%q", f5, capped5, reason5)
	}
}

// 阈值读取默认语义：无键=5；非法值=5；合法值=用户值。
func TestFailAlertThresholdDefault(t *testing.T) {
	db := newHealthTestDB(t)
	if err := db.AutoMigrate(&setting.Setting{}); err != nil {
		t.Fatal(err)
	}
	h := &RSSHandler{db: db}
	ctx := context.Background()

	if got := h.failAlertThreshold(ctx); got != 5 {
		t.Fatalf("无键默认应 5，got %d", got)
	}
	db.Exec("INSERT INTO system_settings (`key`, value) VALUES ('rss_fail_alert_threshold', 'abc')")
	if got := h.failAlertThreshold(ctx); got != 5 {
		t.Fatalf("非法值应回落 5，got %d", got)
	}
	db.Exec("UPDATE system_settings SET value='8' WHERE `key`='rss_fail_alert_threshold'")
	if got := h.failAlertThreshold(ctx); got != 8 {
		t.Fatalf("合法值应 8，got %d", got)
	}
}
