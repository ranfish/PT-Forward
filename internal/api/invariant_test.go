package api

// §59.251 机制 C：三条架构不变式显式锁定（docs/31 §59.251 架构不变式）——
// ①改名→数据连续 ②删除→数据全消（级联） ③重加→全新（无旧数据回流）。
// 架构不变式：clients.id 唯一恒定标识；引用=数字 ID；name 仅显示别名。

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// seedClientData 在 client_uid=uid 下铺满全部核心数据表（级联面全矩阵）
func seedClientData(t *testing.T, env *testEnv, uid uint) {
	t.Helper()
	db := env.db
	rows := []interface{}{
		&model.TorrentSnapshot{Hash: fmt.Sprintf("snap%032d", uid), ClientUID: uid, Name: "Res.2024", SavePath: "/PT0", Size: 1024},
		&model.ClusterScreenshotCache{ClientUID: uid, SavePath: "/PT0", Name: "Res.2024", Screenshots: `["https://x/1.jpg"]`},
		&model.SeedingTorrentRecord{ClientUID: uid, InfoHash: fmt.Sprintf("rec%032d", uid), SiteName: "s", TorrentID: "1", Status: model.SeedingStatusSeeding},
		&model.SeedingClientState{ClientUID: uid, Initialized: true},
		&model.SeedingClientConfig{ClientUID: uid, Enabled: true},
		&model.FreeWaitEntry{ClientUID: uid, SiteName: "s", TorrentID: "1", InfoHash: fmt.Sprintf("fw%032d", uid)},
		&model.DownloadTask{ClientUID: uid, InfoHash: fmt.Sprintf("dt%032d", uid), TorrentName: "t", SavePath: "/PT0", Status: model.DownloadStatusPending},
		&model.ClientPublishTarget{ClientUID: uid, SiteName: "friend"},
		&model.TorrentTraffic{ClientUID: uid, InfoHash: fmt.Sprintf("tt%032d", uid), SiteName: "s"},
		&model.DownloaderSpeedSnapshot{ClientUID: uid, UploadSpeed: 100},
		&model.TrafficStatsHourly{ClientUID: uid},
		&model.PublishCandidate{ClientUID: uid, SourceSite: "s", SourceTorrentID: "1", PublishStatus: model.CandidatePending},
		&model.PublishGroupMember{PublishGroupID: 1, ClientUID: uid, InfoHash: fmt.Sprintf("gm%032d", uid), SiteName: "s", Status: model.MemberStatusSeedingConfirmed},
		&model.ReseedMatch{ClientUID: uid, SourceSite: "s", SourceTorrentID: "1", SourceInfoHash: fmt.Sprintf("rm%032d", uid)},
		&model.ScoringLog{ClientUID: uid, InfoHash: fmt.Sprintf("sl%032d", uid), CycleID: "c1", ScoreType: "cleanup"},
		&model.OrphanScanConfig{ClientUID: uid, ScanPath: "/PT0/SSD"},
	}
	for _, r := range rows {
		if err := db.Create(r).Error; err != nil {
			t.Fatalf("seed data: %v", err)
		}
	}
}

// cascadeTables 级联面全矩阵（表 → 该 uid 残留计数）
func cascadeTables(env *testEnv, uid uint) map[string]int64 {
	count := func(m interface{}, col string) int64 {
		var n int64
		env.db.Model(m).Where(col+" = ?", uid).Count(&n)
		return n
	}
	return map[string]int64{
		"torrent_snapshots":       count(&model.TorrentSnapshot{}, "client_uid"),
		"cluster_screenshot":      count(&model.ClusterScreenshotCache{}, "client_uid"),
		"seeding_records":         count(&model.SeedingTorrentRecord{}, "client_uid"),
		"seeding_states":          count(&model.SeedingClientState{}, "client_uid"),
		"seeding_configs":         count(&model.SeedingClientConfig{}, "client_uid"),
		"free_wait":               count(&model.FreeWaitEntry{}, "client_uid"),
		"download_tasks":          count(&model.DownloadTask{}, "client_uid"),
		"publish_targets":         count(&model.ClientPublishTarget{}, "client_uid"),
		"torrent_traffic":         count(&model.TorrentTraffic{}, "client_uid"),
		"speed_snapshots":         count(&model.DownloaderSpeedSnapshot{}, "client_uid"),
		"traffic_hourly":          count(&model.TrafficStatsHourly{}, "client_uid"),
		"publish_candidates":      count(&model.PublishCandidate{}, "client_uid"),
		"publish_group_members":   count(&model.PublishGroupMember{}, "client_uid"),
		"reseed_matches":          count(&model.ReseedMatch{}, "client_uid"),
		"scoring_logs":            count(&model.ScoringLog{}, "client_uid"),
		"orphan_scan_configs":     count(&model.OrphanScanConfig{}, "client_uid"),
	}
}

// 不变式①：改名 → 引用数据零变动（client_uid 连续、簇键身份不变）
func TestInvariant_RenameContinuity(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/downloaders", map[string]interface{}{
		"name": "qb-main", "type": "qbittorrent", "url": "http://127.0.0.1:19090",
		"username": "a", "password": "b", "role": "download", "enabled": true,
	})
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	seedClientData(t, env, 1)

	// 改名
	w = env.doRequest("PUT", "/api/v1/downloaders/1", map[string]interface{}{
		"name": "qb-renamed", "type": "qbittorrent", "url": "http://127.0.0.1:19090",
		"username": "a", "password": "b", "role": "download", "enabled": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("rename: %d %s", w.Code, w.Body.String())
	}

	for table, n := range cascadeTables(env, 1) {
		if n == 0 {
			t.Errorf("不变式①破坏：%s 在改名后数据丢失（期望连续）", table)
		}
	}
	// 簇键身份解析仍通（hash → client_uid/save_path/name 三元组不变）
	ck, ok := clusterKeyOf(env.db, fmt.Sprintf("snap%032d", 1))
	if !ok || ck.clientUID != 1 || ck.name != "Res.2024" || ck.savePath != "/PT0" {
		t.Errorf("簇键解析在改名后失效：%+v ok=%v（快照身份与下载器显示名无关）", ck, ok)
	}
}

// 不变式②：删除 → 级联面全矩阵清零（数据跟 ID 走，删除=数据全消）
func TestInvariant_DeleteCascades(t *testing.T) {
	env := setupTestEnv(t)

	w := env.doRequest("POST", "/api/v1/downloaders", map[string]interface{}{
		"name": "qb-del", "type": "qbittorrent", "url": "http://127.0.0.1:19091",
		"username": "a", "password": "b", "role": "download", "enabled": true,
	})
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("create: %d", w.Code)
	}
	seedClientData(t, env, 1)
	for table, n := range cascadeTables(env, 1) {
		if n == 0 {
			t.Fatalf("预置失败：%s 未铺数据", table)
		}
	}

	w = env.doRequest("DELETE", "/api/v1/downloaders/1", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}

	for table, n := range cascadeTables(env, 1) {
		if n != 0 {
			t.Errorf("不变式②破坏：%s 删除后残留 %d 行（删除=数据全消）", table, n)
		}
	}
	// clients 行本体 + 路径映射同灭
	var clients int64
	env.db.Unscoped().Model(&model.ClientConfig{}).Where("id = ?", 1).Count(&clients)
	if clients != 0 {
		t.Error("clients 行未删除")
	}
	var subs int64
	env.db.Model(&model.RSSSubscription{}).Count(&subs)
}

// 不变式③：重加同 URL → 全新下载器（无旧数据回流；旧 uid 数据已级联消）
func TestInvariant_ReAddIsFresh(t *testing.T) {
	env := setupTestEnv(t)

	create := func(name, url string) uint {
		w := env.doRequest("POST", "/api/v1/downloaders", map[string]interface{}{
			"name": name, "type": "qbittorrent", "url": url,
			"username": "a", "password": "b", "role": "download", "enabled": true,
		})
		if w.Code != http.StatusOK && w.Code != http.StatusCreated {
			t.Fatalf("create: %d %s", w.Code, w.Body.String())
		}
		resp := parseResponse(t, w)
		data, _ := resp.Data.(map[string]interface{})
		return uint(data["id"].(float64))
	}

	id1 := create("qb-first", "http://127.0.0.1:19092")
	seedClientData(t, env, id1)
	env.doRequest("DELETE", fmt.Sprintf("/api/v1/downloaders/%d", id1), nil)

	id2 := create("qb-second", "http://127.0.0.1:19092")
	if id2 == id1 && cascadeTables(env, id1)["torrent_snapshots"] != 0 {
		t.Errorf("不变式③破坏：uid 复用且旧数据回流（id=%d）", id2)
	}
	// 无论 uid 是否复用：新下载器名下必须零旧数据
	for table, n := range cascadeTables(env, id2) {
		if n != 0 {
			t.Errorf("不变式③破坏：%s 在重加下载器（uid=%d）名下有旧数据 %d 行", table, id2, n)
		}
	}
	// 另一个不受牵连的下载器数据保持（级联精确性）
	id3 := create("qb-other", "http://127.0.0.1:19093")
	if id3 == id1 || id3 == id2 {
		t.Fatalf("测试预置失败：id 冲突 %d %d %d", id1, id2, id3)
	}
	seedClientData(t, env, id3)
	env.doRequest("DELETE", fmt.Sprintf("/api/v1/downloaders/%d", id2), nil)
	for table, n := range cascadeTables(env, id3) {
		if n == 0 {
			t.Errorf("级联误伤：删除 id2 波及无辜下载器数据（%s 清零）", table)
		}
	}
}
