package api

import (
	"context"
	"strings"
	"testing"
	"time"

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

// §59.476 Fix 3: selectSourceMeta music 在场行优先——Tiesto 串种案
// （车站影视行完爆音乐行）终态回归锚。
func TestSelectSourceMetaMusicFirst(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}

	metas := []model.TorrentMetadata{
		{InfoHash: "h1", SiteName: "车站", Title: "Dream to You S01 1080p friDay WEB-DL H.264-CHDWEB",
			Resolution: "1080p", VideoCodec: "H.264", FetchSource: "rss_detail", UpdatedAt: parseTimeLater()},
		{InfoHash: "h1", SiteName: "皇后", Title: "Tiesto Collection 2012 FLAC",
			FetchSource: "rss_detail", DetailSourceJSON: `{"music":{"artist":"Tiesto"}}`},
	}
	got := h.selectSourceMeta(metas)
	if got.SiteName != "皇后" {
		t.Errorf("music 行应优先, got %s", got.SiteName)
	}

	// 影视簇（无 music 行）→ 原排序语义不变（权威/新者优先）
	videoMetas := []model.TorrentMetadata{
		{InfoHash: "h2", SiteName: "天空", Title: "Movie 1080p", UpdatedAt: parseTimeLater()},
		{InfoHash: "h2", SiteName: "末日", Title: "Movie 1080p", UpdatedAt: parseTimeEarly()},
	}
	got = h.selectSourceMeta(videoMetas)
	if got.SiteName != "天空" {
		t.Errorf("影视簇排序语义应保持（新者优先）, got %s", got.SiteName)
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

func parseTimeLater() time.Time { return time.Now() }
func parseTimeEarly() time.Time { return time.Now().Add(-24 * time.Hour) }

// §59.481: music-first 在权威序内选——簇传播副本（音乐数据已传播）与
// 权威源行并存时选权威源行（Ultimate Box Set 案：展示家园副本→皇后源行）。
func TestSelectSourceMetaMusicAuthoritativeOrder(t *testing.T) {
	h := &PublishTorrentsHandler{}
	metas := []model.TorrentMetadata{
		// 簇传播副本（音乐数据已从源行传播而来·site 标签是旧站）
		{InfoHash: "h1", SiteName: "家园", Title: "群星 - Decca Box Set",
			FetchSource: "cluster", DetailSourceJSON: `{"music":{"artist":"群星"}}`},
		// 权威源行（皇后真取）
		{InfoHash: "h1", SiteName: "皇后", Title: "Various Artists - Decca Box Set",
			FetchSource: "rss_detail", DetailSourceJSON: `{"music":{"artist":"Various Artists"}}`},
	}
	got := h.selectSourceMeta(metas)
	if got.SiteName != "皇后" {
		t.Errorf("音乐行集合内应选权威源行 皇后, got %s（簇传播副本）", got.SiteName)
	}
}

// §59.482 刀7/§59.483 双通道: 桥候选查询+同词守卫锚。
func TestSiblingTitleCandidates(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop()}
	Z := func(p string) string { return p + strings.Repeat("0", 40-len(p)) }

	t.Run("通道①coverage", func(t *testing.T) {
		// 簇：皇后副本 + 龙PT 副本（coverage tid 挂在龙PT 副本 hash 上）
		db.Create(&model.TorrentSnapshot{Hash: Z("qn"), ClientUID: 2, SavePath: "/PT2", Name: "KC 50th", TrackerDomain: "tracker.open.cd"})
		db.Create(&model.TorrentSnapshot{Hash: Z("long"), ClientUID: 2, SavePath: "/PT2", Name: "KC 50th", TrackerDomain: "pt.52pt.com"})
		db.Create(&model.SiteCoverageCache{InfoHash: Z("long"), SiteName: "龙PT", TorrentID: "416",
			Status: model.CoverageConfirmedHas, Source: "pieces_hash", Confidence: 1.0})
		cands := h.siblingTitleCandidates(context.Background(), 2, "/PT2", "KC 50th", Z("qn"), nil)
		var found bool
		for _, c := range cands {
			if c.SiteName == "龙PT" && c.TorrentID == "416" {
				found = true
			}
		}
		if !found {
			t.Errorf("通道①应找到龙PT tid=416: %+v", cands)
		}
		// excludeSites 生效
		cands = h.siblingTitleCandidates(context.Background(), 2, "/PT2", "KC 50th", Z("qn"), []string{"龙PT"})
		for _, c := range cands {
			if c.SiteName == "龙PT" {
				t.Errorf("排除后不应有龙PT: %+v", cands)
			}
		}
	})

	t.Run("通道②comment_新副本零coverage即生效", func(t *testing.T) {
		// School Rumble 案：龙PT 新副本·无 coverage tid·comment 在快照
		db.Create(&model.TorrentSnapshot{Hash: Z("srqn"), ClientUID: 2, SavePath: "/PT3", Name: "School Rumble", TrackerDomain: "tracker.open.cd"})
		db.Create(&model.TorrentSnapshot{Hash: Z("srlong"), ClientUID: 2, SavePath: "/PT3", Name: "School Rumble", TrackerDomain: "longpt.org",
			Comment: "https://longpt.org/details.php?id=50493"})
		// 站点表有龙PT（comment 域名解析需要）
		db.Create(&model.Site{Domain: "longpt.org", Name: "龙PT", Enabled: true, Cookie: "c"})
		cands := h.siblingTitleCandidates(context.Background(), 2, "/PT3", "School Rumble", Z("srqn"), nil)
		var found bool
		for _, c := range cands {
			if c.SiteName == "龙PT" && c.TorrentID == "50493" {
				found = true
			}
		}
		if !found {
			t.Errorf("通道②应从 comment 解析龙PT tid=50493（零 coverage）: %+v", cands)
		}
	})

	t.Run("跨通道去重", func(t *testing.T) {
		// 同一 (龙PT, 416) 同时在 coverage 和 comment → 只出现一次
		cands := h.siblingTitleCandidates(context.Background(), 2, "/PT2", "KC 50th", Z("qn"), nil)
		count := 0
		for _, c := range cands {
			if c.SiteName == "龙PT" && c.TorrentID == "416" {
				count++
			}
		}
		// comment 通道 KC 50th 簇的龙PT comment 不在（无 comment 字段）→ 只有 coverage 一条
		if count > 1 {
			t.Errorf("龙PT tid=416 出现 %d 次（应 ≤1 去重）", count)
		}
	})
}

// §59.482 刀7: 同词守卫——兄弟标题与原名同关键词跳过。
func TestSiblingKeywordUsable(t *testing.T) {
	// KC 案：龙PT 标题产出不同关键词 ✓（& 分隔+FLAC 剥+2019 保留）
	if kw, ok := siblingKeywordUsable("KC 50th", "Various Artists - Heaven & Earth 2019-FLAC 整軌"); !ok {
		t.Error("KC 案兄弟标题应可用")
	} else if kw != "Various Artists Heaven Earth 2019" {
		t.Errorf("KC 案关键词: %q", kw)
	}
	// 同词（兄弟标题与原名产出相同关键词）→ 跳过（年份保留态下
	// "…2020" 与 "…2020 FLAC" 产出不同词——用真同词形态测）
	if _, ok := siblingKeywordUsable("Carol Albert - Stronger Now 2020", "Carol Albert - Stronger Now 2020 FLAC"); ok {
		t.Error("同词应跳过（白烧请求防御）")
	}
	// 空标题/同名
	if _, ok := siblingKeywordUsable("X", ""); ok {
		t.Error("空标题应跳过")
	}
	if _, ok := siblingKeywordUsable("Some Name", "Some Name"); ok {
		t.Error("同名应跳过")
	}
}
