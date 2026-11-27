package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
)

// 16 刀2（v0.0.1664）: 趋势图窗口/标签一致性锚（V-02 防回归）——
// 端到端直调生产 handler（/stats/by-site/{site}/trend）断言真实
// points：本地凌晨 07:30 记录应归当日（date 标签=当日+count 含该
// 记录）。原实现 Truncate(24h)+dayStart.Format 双错：07:30 落
// [前日08:00, 当日08:00) 窗口被计入昨日点+标签显示前一日。
//
// 阴性对照（08 篇方法论——锚必须可触发）：
//   - dayStart 改回 Truncate(24h) → 今日 count=0+昨日 count=2 FAIL；
//   - 标签改回 dayStart.Format → date 值显示前一日 FAIL。
func TestSeedingSiteTrend_LocalDayBoundary(t *testing.T) {
	env := setupTestEnv(t)

	// 造记录：本地今日 07:30（V-02 原案时刻——原实现归昨日窗口）
	today := time.Now()
	morning := time.Date(today.Year(), today.Month(), today.Day(), 7, 30, 0, 0, today.Location())
	env.db.Create(&model.SeedingTorrentRecord{
		ClientUID: 1, InfoHash: "trendhash1", SiteName: "trendsite",
		TorrentID: "t1", Status: "seeding", Discount: "none", CreatedAt: morning,
	})
	// 对照：昨日 12:00（任何口径都归昨日）
	yesterday := morning.AddDate(0, 0, -1).Add(5 * time.Hour)
	env.db.Create(&model.SeedingTorrentRecord{
		ClientUID: 1, InfoHash: "trendhash2", SiteName: "trendsite",
		TorrentID: "t2", Status: "seeding", Discount: "none", CreatedAt: yesterday,
	})

	// range=24h → days=1 → 两日点（昨日+今日）
	w := env.doRequest("GET", "/api/v1/seeding/stats/by-site/trendsite/trend?range=24h", nil)
	if w.Code != 200 {
		t.Fatalf("trend 端点: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Trends []struct {
				Date  string `json:"date"`
				Count int64  `json:"count"`
			} `json:"trends"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	points := resp.Data.Trends
	if len(points) != 2 {
		t.Fatalf("range=24h 应两日点·实得 %d: %+v", len(points), points)
	}

	wantToday := today.Format("2006-01-02")
	wantYesterday := today.AddDate(0, 0, -1).Format("2006-01-02")
	// points[0]=昨日（i=days=1 先出）·points[1]=今日
	if got := points[0].Date; got != wantYesterday {
		t.Errorf("昨日点标签=%q want %q（标签显示前一天=dayStart.Format 回归）", got, wantYesterday)
	}
	if got := points[0].Count; got != 1 {
		t.Errorf("昨日点 count=%d want 1（对照记录 12:00）——若=2 则 07:30 凌晨记录被计入昨日（窗口退回 08:00 偏移=V-02 回归）", got)
	}
	if got := points[1].Date; got != wantToday {
		t.Errorf("今日点标签=%q want %q", got, wantToday)
	}
	if got := points[1].Count; got != 1 {
		t.Errorf("今日点 count=%d want 1（07:30 凌晨记录应归今日）——若=0 则窗口退回 Truncate(24h)〔V-01/V-02 回归〕", got)
	}
}
