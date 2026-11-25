package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// §59.578 联合回归锚（刀1+刀2+刀3 端到端——三刀 handler 全串联）：
// 生命周期完整旅程——未获取/不支持①（classify）→ 弹窗排除（烙印）→
// unsupported 视图筛选 → DELETE 清除（清烙印）→ 弹窗重现。
// 与 fetch_brand_test 的差异：直调 classify/handler 单点 → 本锚走
// handleListSeeds 全链（Select s.*→classify→item 组装→筛选→分页——
// 联合回归的接缝盲区：单点全绿≠接缝通）。
func TestSeedLifecycleJoint(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:joint_"+t.Name()+".db?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&model.TorrentSnapshot{}, &model.TorrentMetadata{},
		&model.Site{}, &model.ReleaseGroupMapping{}, &model.ExternalReleaseGroup{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	h.SetSourceDetector(publish.NewSourceSiteDetector(db, zap.NewNop()))
	dh := &DownloadHandler{db: db}

	// 收录列表：SPHD 已收录（19 组之一）·无其它组
	db.Create(&model.ExternalReleaseGroup{GroupName: "SPHD", Source: "census"})

	now := time.Now()
	seed := []model.TorrentSnapshot{
		// T1 烙印②：无 metadata+终态失败烙印（Send Help Hybrid 形态）
		{Hash: "joint1", ClientUID: 2, Name: "Movie One 2026 2160p Remux.mkv", SavePath: "/p", Progress: 1.0,
			FetchFailedAt: &now, FetchError: "无可用源站（制作组未映射 + 无覆盖）"},
		// T2 注册表命中：未获取（PTP/SPHD 形态——A 扩展）
		{Hash: "joint2", ClientUID: 2, Name: "Movie Two 2026 1080p BluRay x264-SPHD", SavePath: "/p", Progress: 1.0},
		// T3 不认识①：无组名·无烙印（弹窗可试——B 含①）
		{Hash: "joint3", ClientUID: 2, Name: "Movie Three 2026 Remux Dolby Vision.mkv", SavePath: "/p", Progress: 1.0},
	}
	for i := range seed {
		if err := db.Create(&seed[i]).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	listStatuses := func(status string) map[string]map[string]interface{} {
		t.Helper()
		w := httptest.NewRecorder()
		url := "/api/v1/publish/seeds?client_id=2&save_path=/p&page=1&page_size=50"
		if status != "" {
			url += "&status=" + status
		}
		r := httptest.NewRequest("GET", url, nil)
		h.handleListSeeds(w, r)
		var resp struct {
			Data struct {
				Items []map[string]interface{} `json:"items"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("list parse: %v (body=%s)", err, w.Body.String()[:min(300, len(w.Body.String()))])
		}
		out := map[string]map[string]interface{}{}
		for _, it := range resp.Data.Items {
			if n, ok := it["name"].(string); ok {
				out[n] = it
			}
		}
		return out
	}

	// ① 全量：三态分类（接缝=Select s.*→classify→item）
	all := listStatuses("")
	if st, _ := all["Movie One 2026 2160p Remux.mkv"]["status"].(string); st != "unsupported" {
		t.Errorf("① T1 烙印② 应 unsupported: got %q", st)
	}
	if st, _ := all["Movie Two 2026 1080p BluRay x264-SPHD"]["status"].(string); st != "unfetched" {
		t.Errorf("① T2 注册表命中应 unfetched（A 扩展）: got %q", st)
	}
	if st, _ := all["Movie Three 2026 Remux Dolby Vision.mkv"]["status"].(string); st != "unsupported" {
		t.Errorf("① T3 无组名应 unsupported①: got %q", st)
	}
	// ② 可解释性接缝：T1 的 fetch_error 透出（item key=前端 tooltip 消费 key）
	if fe, _ := all["Movie One 2026 2160p Remux.mkv"]["fetch_error"].(string); fe == "" {
		t.Errorf("② T1 fetch_error 未透出（tooltip 不可解释）")
	}

	// ③ unsupported 筛选视图（批量重置按钮的作用域）
	uns := listStatuses("unsupported")
	if len(uns) != 2 {
		t.Errorf("③ unsupported 视图应 2 行（T1②+T3①）: got %d", len(uns))
	}
	if _, ok := uns["Movie Two 2026 1080p BluRay x264-SPHD"]; ok {
		t.Errorf("③ T2 unfetched 不应入 unsupported 视图")
	}

	// ④ 弹窗（DownloadHandler）：T1 烙印整 name 排除·T2/T3 显示（B 含①）
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest("GET", "/api/v1/downloads/snapshot-unconfigured?client_id=2&save_path=/p", nil)
	dh.handleSnapshotUnconfigured(w2, r2)
	var resp2 struct {
		Data struct {
			Items []struct {
				Name string `json:"name"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("popup parse: %v", err)
	}
	popup := map[string]bool{}
	for _, it := range resp2.Data.Items {
		popup[it.Name] = true
	}
	if popup["Movie One 2026 2160p Remux.mkv"] {
		t.Errorf("④ T1 烙印 name 应被弹窗排除")
	}
	if !popup["Movie Two 2026 1080p BluRay x264-SPHD"] || !popup["Movie Three 2026 Remux Dolby Vision.mkv"] {
		t.Errorf("④ T2/T3 未走链应显示（B 含①）: %v", popup)
	}

	// ⑤ 清除 T1（deleteSeed——无 metadata 行也清烙印）
	w3 := httptest.NewRecorder()
	r3 := httptest.NewRequest("DELETE", "/api/v1/publish/seeds/joint1", nil)
	h.handleDeleteSeed(w3, r3)

	// ⑥ 弹窗重现（清除→回未走链——用户定案通道闭环）
	w4 := httptest.NewRecorder()
	r4 := httptest.NewRequest("GET", "/api/v1/downloads/snapshot-unconfigured?client_id=2&save_path=/p", nil)
	dh.handleSnapshotUnconfigured(w4, r4)
	var resp4 struct {
		Data struct {
			Items []struct {
				Name string `json:"name"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w4.Body.Bytes(), &resp4); err != nil {
		t.Fatalf("popup2 parse: %v", err)
	}
	again := map[string]bool{}
	for _, it := range resp4.Data.Items {
		again[it.Name] = true
	}
	if !again["Movie One 2026 2160p Remux.mkv"] {
		t.Errorf("⑥ 清除后 T1 应回弹窗（烙印清·重走获取链通道）: %v", again)
	}
	// ⑦ 清除后列表态：T1 无烙印→按①身份判（unsupported①·不再带 fetch_error）
	after := listStatuses("")
	if fe, _ := after["Movie One 2026 2160p Remux.mkv"]["fetch_error"].(string); fe != "" {
		t.Errorf("⑦ 清除后 fetch_error 应清空: %q", fe)
	}
}
