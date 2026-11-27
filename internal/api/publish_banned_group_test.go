package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// §59.580: 发布侧黑名单过滤锚（一种多站 handleExecutePublishBatch）——
// 黑名单组种子→目标站列表中的瓷器被剔除（status=banned_group·
// 与 preflightSite 同隔离语义：跳过+提示·不阻塞其它站）。
// 黑名单分支在 goroutine 内早退——executor 空壳构造零触碰。
//
// 阴性对照：删 bannedSites 过滤分支 → 锚 FAIL（banned_group 状态
// 消失·走 executor 空壳报错）。
func TestExecutePublishBatch_BannedGroupSiteFiltered(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	// 黑名单组种子（CHDBits 前缀族）
	db.Create(&model.TorrentSnapshot{
		Hash:      "bannedhash00000000000000000000000000000000",
		ClientUID: 1,
		Name:      "Movie.2026.1080p.BluRay.x264-CHDBits",
		SavePath:  "/x",
	})

	// executor 空壳（黑名单分支早退不触碰）
	h := &PublishTorrentsHandler{
		db:       db,
		logger:   zap.NewNop(),
		executor: publish.NewPublishExecutor(&publish.Pipeline{}),
	}

	body, _ := json.Marshal(map[string]interface{}{
		"info_hash":    "bannedhash00000000000000000000000000000000",
		"target_sites": []string{"瓷器"},
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/publish/execute", bytes.NewReader(body))
	// 直接调内部方法（路由包装形态见 ServeHTTP——单元锚聚焦过滤语义）
	h.handleExecutePublishBatch(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Results []struct {
				Site    string `json:"site"`
				Status  string `json:"status"`
				Message string `json:"message"`
			} `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Data.Results) != 1 {
		t.Fatalf("应 1 个结果·实得 %d: %+v", len(resp.Data.Results), resp.Data.Results)
	}
	r := resp.Data.Results[0]
	if r.Site != "瓷器" {
		t.Errorf("site=%q want 瓷器", r.Site)
	}
	if r.Status != "banned_group" {
		t.Errorf("黑名单组种子的瓷器结果 status=%q want banned_group（过滤分支回归——若 failed 则走了 executor 空壳）", r.Status)
	}
	if r.Message == "" || !contains(r.Message, "CHDBits") {
		t.Errorf("message 应含组名说明: %q", r.Message)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && stringContains(s, sub))
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// §59.580 回归审核补：单发入口黑名单锚（原实施漏点——批量两入口
// 已过滤而单发零过滤）。
func TestExecutePublish_SingleSite_BannedGroupBlocked(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	db.Create(&model.TorrentSnapshot{
		Hash:      "bannedhash00000000000000000000000000000000",
		ClientUID: 1,
		Name:      "Movie.2026.1080p.BluRay.x264-CHDBits",
		SavePath:  "/x",
	})
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	body, _ := json.Marshal(map[string]interface{}{
		"info_hash":   "bannedhash00000000000000000000000000000000",
		"target_site": "瓷器",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/publish/execute", bytes.NewReader(body))
	h.handleExecutePublish(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("黑名单组单发瓷器应 400 拒绝·实得 %d: %s", w.Code, w.Body.String())
	}
	if !stringContains(w.Body.String(), "CHDBits") {
		t.Errorf("拒绝文案应含组名: %s", w.Body.String())
	}
	// 合法组对照（不拦——走 executor 流程·此处 executor nil 503 证明
	// 黑名单门放行）
	db.Create(&model.TorrentSnapshot{
		Hash:      "legalhash0000000000000000000000000000000000",
		ClientUID: 1,
		Name:      "Movie.2026.1080p.BluRay.x264-WiLDCAT",
		SavePath:  "/x",
	})
	body2, _ := json.Marshal(map[string]interface{}{
		"info_hash":   "legalhash0000000000000000000000000000000000",
		"target_site": "瓷器",
	})
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/publish/execute", bytes.NewReader(body2))
	h.handleExecutePublish(w2, req2)
	if w2.Code == http.StatusBadRequest && stringContains(w2.Body.String(), "黑名单") {
		t.Error("合法组不应被黑名单拒绝")
	}
}
