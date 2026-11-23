package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.558 PR-2 前置锚（用户边界定案）：handleListSeeds 筛选参数组合行为锁定。
// 拆分（编排/helper 抽取）前先跑绿锁定现状·拆后再跑应仍绿（零行为变更锚）。
// fixture：两簇（一 reviewed 完备簇 + 一未审核簇）+ 发布记录（簇 A 已发布某站）。

func seedListFixture(t *testing.T, env *testEnv) {
	now := time.Now()
	mkSnap := func(hash, name string, clientUID uint, path string) *model.TorrentSnapshot {
		return &model.TorrentSnapshot{
			Hash: hash, Name: name, ClientUID: clientUID, SavePath: path,
			Progress: 1.0, TrackerDomain: "tracker.example.org",
			CreatedAt: now, UpdatedAt: now,
		}
	}
	// 簇 A：reviewed 完备（title 含可搜词）
	env.db.Create(mkSnap("aaa111", "Movie Alpha 2020", 1, "/data"))
	env.db.Create(&model.TorrentMetadata{
		InfoHash: "aaa111", SiteName: "站点A", Title: "Movie Alpha 2020",
		Reviewed: true, Description: "d", MediaInfo: "mi",
		FetchSource: "batch_fetch", CreatedAt: now, UpdatedAt: now,
	})
	// 簇 B：未审核
	env.db.Create(mkSnap("bbb222", "Show Beta 2021", 1, "/data"))
	env.db.Create(&model.TorrentMetadata{
		InfoHash: "bbb222", SiteName: "站点B", Title: "Show Beta 2021",
		Reviewed: false, CreatedAt: now, UpdatedAt: now,
	})
	// 簇 A 发布记录（已发布至「目标站X」）
	env.db.Create(&model.PublishResultRecord{
		SourceSite: "站点A", TargetSite: "目标站X", SourceInfoHash: "aaa111",
		Status: "pushed", CreatedAt: now, UpdatedAt: now,
	})
}

func seedListGet(t *testing.T, env *testEnv, query string) (total float64, names []string) {
	t.Helper()
	w := env.doRequest(http.MethodGet, "/api/v1/publish/seeds?"+query, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET seeds?%s → %d·body=%s", query, w.Code, w.Body.String()[:min(300, w.Body.Len())])
	}
	var resp struct {
		Data struct {
			Total float64                  `json:"total"`
			Items []map[string]interface{} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	for _, it := range resp.Data.Items {
		names = append(names, fmt.Sprint(it["name"]))
	}
	return resp.Data.Total, names
}

func TestHandleListSeeds_Filters(t *testing.T) {
	env := setupTestEnv(t)
	seedListFixture(t, env)

	// ① 无筛选：两簇全出
	total, names := seedListGet(t, env, "")
	if total != 2 || len(names) != 2 {
		t.Fatalf("① 全量: total=%v names=%v（应 2 簇）", total, names)
	}

	// ② ready=true：只 reviewed 簇 A
	_, names = seedListGet(t, env, "ready=true")
	if len(names) != 1 || names[0] != "Movie Alpha 2020" {
		t.Fatalf("② ready=true: %v（应仅簇 A）", names)
	}
	// ③ ready=false：只未审核簇 B
	_, names = seedListGet(t, env, "ready=false")
	if len(names) != 1 || names[0] != "Show Beta 2021" {
		t.Fatalf("③ ready=false: %v（应仅簇 B）", names)
	}

	// ④ publish_state=published&target_site=目标站X：簇 A（发布记录命中）
	_, names = seedListGet(t, env, "publish_state=published&target_site=目标站X")
	if len(names) != 1 || names[0] != "Movie Alpha 2020" {
		t.Fatalf("④ published: %v（应仅簇 A——记录面命中）", names)
	}
	// ⑤ publish_state=publishable&target_site=目标站X：簇 B（A 已发布被排除·B reviewed=false 也排——空集）
	_, names = seedListGet(t, env, "publish_state=publishable&target_site=目标站X")
	if len(names) != 0 {
		t.Fatalf("⑤ publishable: %v（应空——A 已发布·B 未 reviewed）", names)
	}
	// ⑥ publish_state=published&target_site=不存在站：空（无命中）
	_, names = seedListGet(t, env, "publish_state=published&target_site=无此站")
	if len(names) != 0 {
		t.Fatalf("⑥ published 无命中: %v（应空）", names)
	}

	// ⑦ search 命中 name
	_, names = seedListGet(t, env, "search=Alpha")
	if len(names) != 1 || names[0] != "Movie Alpha 2020" {
		t.Fatalf("⑦ search name: %v", names)
	}
	// ⑧ search 命中 title（Go 层三域双保险）
	_, names = seedListGet(t, env, "search=Beta")
	if len(names) != 1 || names[0] != "Show Beta 2021" {
		t.Fatalf("⑧ search title: %v", names)
	}

	// ⑨ 分页：page_size=1 首页单簇
	total, names = seedListGet(t, env, "page_size=1")
	if total != 2 || len(names) != 1 {
		t.Fatalf("⑨ 分页: total=%v len=%v（应 total2 页1）", total, len(names))
	}
}

func TestHandleListSeeds_Observing(t *testing.T) {
	env := setupTestEnv(t)
	seedListFixture(t, env)
	// observing 分支短路（独立视图·空 fixture 下 200+空集——行为锚：不进主查询）
	total, _ := seedListGet(t, env, "status=observing")
	if total != 0 {
		t.Fatalf("observing 空数据应 0: %v", total)
	}
}
