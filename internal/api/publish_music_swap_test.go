package api

import (
	"context"
	"strings"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// §59.476 位置3: resolveMusicReplicaHash——簇内音乐源站副本换 hash
// （辅种代表→皇后副本·素材量序）；影视簇/无音乐副本原样。
func TestResolveMusicReplicaHash(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	ctx := context.Background()

	// 站点：皇后（音乐源站·tracker 域）+ 天空（影视）
	db.Create(&model.Site{Domain: "open.cd", Name: "皇后", Enabled: true, Cookie: "c", TrackerDomains: `["tracker.open.cd"]`})
	db.Create(&model.Site{Domain: "hdsky.me", Name: "天空", Enabled: true, Cookie: "c", TrackerDomains: `["tracker.hdsky.me"]`})

	Z := func(prefix string) string { return prefix + strings.Repeat("0", 40-len(prefix)) }
	name := "Gwyneth Herbert - Bittersweet And Blue (2005) [FLAC]"
	// Gwyneth 簇（12 环境实证形态）：辅种代表 + 皇后副本 + 影视辅种
	db.Create(&model.TorrentSnapshot{Hash: Z("pter"), ClientUID: 2, SavePath: "/PT2/OpenCD", Name: name, TrackerDomain: "tracker.pterclub.net", Size: 100})
	queen := Z("a901")
	db.Create(&model.TorrentSnapshot{Hash: queen, ClientUID: 2, SavePath: "/PT2/OpenCD", Name: name, TrackerDomain: "tracker.open.cd", Size: 101})

	// 换 hash：辅种代表 → 皇后副本（size 对齐）
	got, size := h.resolveMusicReplicaHash(ctx, 2, Z("pter"), 100)
	if got != queen {
		t.Errorf("应换成皇后副本 %s, got %s", queen, got)
	}
	if size != 101 {
		t.Errorf("size 应对齐皇后副本快照: %d", size)
	}
	// 皇后副本自身 → 原样（幂等）
	got, size = h.resolveMusicReplicaHash(ctx, 2, queen, 99)
	if got != queen || size != 99 {
		t.Errorf("皇后副本应原样: %s/%d", got, size)
	}

	// 影视簇（无音乐源站副本）→ 原样
	vname := "Movie 2025 1080p WEB-DL"
	db.Create(&model.TorrentSnapshot{Hash: Z("vid"), ClientUID: 2, SavePath: "/PT3", Name: vname, TrackerDomain: "tracker.hdsky.me", Size: 5})
	got, size = h.resolveMusicReplicaHash(ctx, 2, Z("vid"), 5)
	if got != Z("vid") || size != 5 {
		t.Errorf("影视簇应原样: %s/%d", got, size)
	}

	// 快照缺失/跨下载器 → 原样
	got, _ = h.resolveMusicReplicaHash(ctx, 9, Z("pter"), 1)
	if got != Z("pter") {
		t.Errorf("无簇上下文应原样: %s", got)
	}
}

// §59.476 兜底站序：fetch_priority∩音乐源站优先 → 素材量序补齐；
// 仅 enabled+凭证站；影视站结构性出局。
func TestMusicFallbackSites(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	ctx := context.Background()

	// 无音乐源站配置 → nil
	if ms := h.musicFallbackSites(ctx); len(ms) != 0 {
		t.Errorf("无配置应 nil: %v", ms)
	}

	db.Create(&model.Site{Domain: "open.cd", Name: "皇后", Enabled: true, Cookie: "c", TrackerDomains: `["tracker.open.cd"]`})
	db.Create(&model.Site{Domain: "dicmusic.com", Name: "海豚", Enabled: true, Cookie: "c", TrackerDomains: `["tracker.52dic.vip"]`})
	utp := &model.Site{Domain: "www.musopia.vip", Name: "乌托邦", Enabled: true, Cookie: "c", TrackerDomains: `["www.musopia.vip"]`} // 创建后关启用（gorm default:true 零值陷阱）
	db.Create(utp)
	db.Model(utp).Update("enabled", false)
	db.Create(&model.Site{Domain: "hdsky.me", Name: "天空", Enabled: true, Cookie: "c", TrackerDomains: `["tracker.hdsky.me"]`})

	ms := h.musicFallbackSites(ctx)
	if len(ms) != 2 || ms[0] != "皇后" || ms[1] != "海豚" {
		t.Errorf("应 [皇后 海豚]（乌托邦未启用出局·天空影视出局）: %v", ms)
	}

	// fetch_priority 含海豚 → 海豚先
	db.Exec(`INSERT INTO system_settings (key, value) VALUES ('fetch_priority', '["海豚","不存在站"]')`)
	ms = h.musicFallbackSites(ctx)
	if len(ms) != 2 || ms[0] != "海豚" || ms[1] != "皇后" {
		t.Errorf("priority 应抬海豚首位: %v", ms)
	}
}

// §59.476 S2+ 词表推导：release_group_mappings 音乐源站行 → 词表
// （集成锚——migration 151 种子行 + 推导链）。
func TestMusicOfficialGroupWords(t *testing.T) {
	db := clusterTestDB(t)
	ctx := context.Background()

	for _, m := range []model.ReleaseGroupMapping{
		{GroupName: "OpenCD", Domain: "open.cd", SiteName: "皇后", IsOfficial: true},
		{GroupName: "LLM", Domain: "open.cd", SiteName: "皇后", IsOfficial: true},
		{GroupName: "TSxD", Domain: "open.cd", SiteName: "皇后", IsOfficial: true},
		{GroupName: "KHQ", Domain: "open.cd", SiteName: "皇后", IsOfficial: true},
		{GroupName: "Musopia", Domain: "www.musopia.vip", SiteName: "乌托邦", IsOfficial: true},
		{GroupName: "CHDWEB", Domain: "chdbits.co", SiteName: "天空", IsOfficial: true}, // 影视组——不入词表
	} {
		db.Create(&m)
	}
	words := publishMusicWords(t, db)
	if len(words) != 5 {
		t.Fatalf("词表应 5 词（影视组出局）: %v", words)
	}
	for _, want := range []string{"OpenCD", "LLM", "TSxD", "KHQ", "Musopia"} {
		if !containsStr(words, want) {
			t.Errorf("词表缺 %s: %v", want, words)
		}
	}
	// 端到端：handler 判定（S2+ 经 DB 词表）
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	if !h.isMusicSeedForRouting(ctx, "Various Artist-NAXOS 25周年纪念 9Box 90CDs-wav-OpenCD", nil) {
		t.Error("S2+ 尾段 OpenCD 应判音乐种")
	}
	if !h.isMusicSeedForRouting(ctx, "精选 LLM 之作 2024 FLAC", nil) {
		t.Error("S2+ 非组位无技术词应判音乐种")
	}
	if h.isMusicSeedForRouting(ctx, "The Rise of LLM 2024 1080p WEB-DL", nil) {
		t.Error("否决闸应拦影视形态")
	}
}

// 词表推导桥：publish.MusicOfficialGroupWords 直调（集成锚）
func publishMusicWords(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	return publish.MusicOfficialGroupWords(context.Background(), db)
}

