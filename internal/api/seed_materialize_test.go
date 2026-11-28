package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.598（classify 六态物化）: 三层锚——直读物化列/钩子更新/自愈+backfill。
//
// 阴性对照（三阴性）：
// A=删 buildSeedListItems 的物化列读取（退回恒现场 classify）→ 锚①场景
//   FAIL（预物化错误值行被现场计算纠正——物化失效不可检测）
// B=删审核钩子（reclassifyByMetaIDs 不再调用）→ 锚② FAIL（物化列不更新）
// C=删 selfHealPending → 锚③ FAIL（空值行走兜底不落库）

// 锚①: 物化列直读——预物化行（status 列有值·与现场计算不一致时）读列
// 不现场纠正（物化权威性·判定源唯一性——classifySeedStatusLite 结果的
// 缓存即真相·错误值由钩子/B2 自愈负责纠正·查询路径零计算）。
func TestSeedListMaterializedColumnAuthoritative(t *testing.T) {
	env := setupTestEnv(t)
	now := time.Now()
	// 预物化错误值：meta reviewed=true 现场应算 reviewed·但物化列写 forbidden
	env.db.Create(&model.TorrentSnapshot{
		Hash: "mat001", Name: "Mat Test 2020", ClientUID: 1, SavePath: "/data",
		Progress: 1.0, Status: "forbidden", // 预物化错误值
		CreatedAt: now, UpdatedAt: now,
	})
	env.db.Create(&model.TorrentMetadata{
		InfoHash: "mat001", SiteName: "站点A", Title: "Mat Test 2020",
		Reviewed: true, Description: "d", MediaInfo: "mi",
		FetchSource: "batch_fetch", CreatedAt: now, UpdatedAt: now,
	})
	total, names := seedListGet(t, env, "status=forbidden")
	if total != 1 || len(names) != 1 || names[0] != "Mat Test 2020" {
		t.Fatalf("锚①: 物化列权威性失败（forbidden 筛选应命中预物化行）: total=%v names=%v", total, names)
	}
	_, names = seedListGet(t, env, "status=reviewed")
	if len(names) != 0 {
		t.Fatalf("锚①: 物化列 forbidden 行不应出现在 reviewed 桶（若现场计算覆盖=物化失效）: %v", names)
	}
}

// 锚②: 审核钩子——PUT /seed-data/{id}（reviewed=true）后物化列更新。
func TestSeedListReviewUpdatesMaterialized(t *testing.T) {
	env := setupTestEnv(t)
	now := time.Now()
	env.db.Create(&model.TorrentSnapshot{
		Hash: "rev001", Name: "Rev Test 2020", ClientUID: 1, SavePath: "/data",
		Progress: 1.0, Status: "pending", // 预物化 pending
		CreatedAt: now, UpdatedAt: now,
	})
	var meta model.TorrentMetadata
	env.db.Create(&model.TorrentMetadata{
		InfoHash: "rev001", SiteName: "站点A", Title: "Rev Test 2020",
		Reviewed: false, Description: "d", MediaInfo: "mi",
		FetchSource: "batch_fetch", CreatedAt: now, UpdatedAt: now,
	})
	env.db.Where("info_hash = ?", "rev001").First(&meta)
	// 审核（handleSaveSeedData——含钩②）
	w := env.doRequest(http.MethodPut, "/api/v1/publish/seed-data/"+fmt.Sprint(meta.ID),
		map[string]interface{}{"title": "Rev Test 2020"})
	if w.Code != http.StatusOK {
		t.Fatalf("审核 PUT → %d: %s", w.Code, w.Body.String()[:min(200, w.Body.Len())])
	}
	var snap model.TorrentSnapshot
	env.db.Where("hash = ?", "rev001").First(&snap)
	if snap.Status != "reviewed" {
		t.Fatalf("锚②: 审核后物化列应 reviewed·实=%q（钩②断裂）", snap.Status)
	}
}

// 锚③: 自愈——空 status 行查询时补算落库（B2）。
func TestSeedListSelfHealEmptyStatus(t *testing.T) {
	env := setupTestEnv(t)
	now := time.Now()
	env.db.Create(&model.TorrentSnapshot{
		Hash: "heal001", Name: "Heal Test 2020", ClientUID: 1, SavePath: "/data",
		Progress: 1.0, Status: "", // 空值（syncer 新建行形态）
		CreatedAt: now, UpdatedAt: now,
	})
	env.db.Create(&model.TorrentMetadata{
		InfoHash: "heal001", SiteName: "站点A", Title: "Heal Test 2020",
		Reviewed: true, Description: "d", MediaInfo: "mi",
		FetchSource: "batch_fetch", CreatedAt: now, UpdatedAt: now,
	})
	// 一次查询触发自愈
	total, names := seedListGet(t, env, "status=reviewed")
	if total != 1 || len(names) != 1 || names[0] != "Heal Test 2020" {
		t.Fatalf("锚③: 自愈后应入 reviewed 桶: total=%v names=%v", total, names)
	}
	// 落库验证（B2：自 Heal 后 DB 有值·下次查询零自愈成本）
	var snap model.TorrentSnapshot
	env.db.Where("hash = ?", "heal001").First(&snap)
	if snap.Status != "reviewed" {
		t.Fatalf("锚③: 自愈未落库（B2 断裂·status=%q）", snap.Status)
	}
}
