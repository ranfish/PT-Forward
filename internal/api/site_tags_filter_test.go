package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/publish"

	"go.uber.org/zap"
)

// §59.547 三态筛选三面并集 + 四色 tag 数据锚：
//
//	已发布 = 记录面 ∪ 覆盖面 ∪ tracker 面（与执行层 CoveredSitesOfCluster
//	         同口径——列表说可发布的种子发布时永不撞跳过）
//	可发布 = 三面全 miss ∧ reviewed
//	site_tags（选站上下文）：选中站置顶+ours/seeded/covered 三属性
//
// 场景：簇 A 覆盖面命中 / 簇 B 记录面命中 / 簇 C 三面 miss。
func TestListSeedsThreeFaceFilterAndSiteTags(t *testing.T) {
	db := clusterTestDB(t)
	h := &PublishTorrentsHandler{db: db, logger: zap.NewNop(),
		resourceResolver: publish.NewResourceResolver(db)}
	Z := func(p string) string { return p + strings.Repeat("0", 40-len(p)) }
	meta := func(hash, title string) *model.TorrentMetadata {
		return &model.TorrentMetadata{InfoHash: hash, SiteName: "X", Title: title,
			Poster: "p", Description: "d", Screenshots: `["u"]`, Tags: `["x"]`,
			MediaInfo: "mi", Reviewed: true}
	}

	// 簇 A：覆盖面命中（目标站 has·无记录无副本）——用户四色定案的「蓝/黄」场景
	db.Create(&model.TorrentSnapshot{Hash: Z("acover"), ClientUID: 1, SavePath: "/v", Name: "A.Cover.2026", Progress: 1})
	db.Create(meta(Z("acover"), "TA"))
	db.Create(&model.SiteCoverageCache{InfoHash: Z("acover"), SiteName: "目标站",
		Status: model.CoverageConfirmedHas, Source: model.CoverageSourceIYUU,
		Confidence: 1, QueriedAt: time.Now(), ExpiresAt: time.Now().Add(24 * time.Hour)})

	// 簇 B：记录面命中（我们发过目标站·pushed）——「绿」场景
	db.Create(&model.TorrentSnapshot{Hash: Z("brec"), ClientUID: 1, SavePath: "/v", Name: "B.Rec.2026", Progress: 1})
	db.Create(meta(Z("brec"), "TB"))
	db.Create(&model.PublishResultRecord{SourceInfoHash: Z("brec"), TargetSite: "目标站",
		Status: "pushed", TorrentID: "123", Trigger: "manual"})

	// 簇 C：三面 miss（reviewed → 可发布）
	db.Create(&model.TorrentSnapshot{Hash: Z("cmiss"), ClientUID: 1, SavePath: "/v", Name: "C.Miss.2026", Progress: 1})
	db.Create(meta(Z("cmiss"), "TC"))

	// 簇 D：existing 终态（文案信任·没发上去）——§59.547 回归审核补：
	// 四色绿必须是发布事实口径（Skip 五终态）·existing 不得标绿
	// （站上其实是别人的·标绿会误导归属）
	db.Create(&model.TorrentSnapshot{Hash: Z("dexist"), ClientUID: 1, SavePath: "/v", Name: "D.Exist.2026", Progress: 1})
	db.Create(meta(Z("dexist"), "TD"))
	db.Create(&model.PublishResultRecord{SourceInfoHash: Z("dexist"), TargetSite: "目标站",
		Status: "existing", Trigger: "manual"})

	fetch := func(state string) map[string]map[string]interface{} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/publish/seeds", nil)
		q := url.Values{}
		q.Set("publish_state", state)
		q.Set("target_site", "目标站")
		q.Set("page", "1")
		q.Set("page_size", "50")
		req.URL.RawQuery = q.Encode()
		rec := httptest.NewRecorder()
		h.handleListSeeds(rec, req)
		var resp struct {
			Data struct {
				Items []map[string]interface{} `json:"items"`
			} `json:"data"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("state=%s 解码失败: %v body=%s", state, err, rec.Body.String())
		}
		out := map[string]map[string]interface{}{}
		for _, it := range resp.Data.Items {
			out[it["name"].(string)] = it
		}
		return out
	}

	// ① publishable：仅簇 C（三面全 miss）——簇 D（existing）命中记录面 List
	// 也不在可发布（List 六终态筛选用）·簇 C 是唯一三面 miss
	pub := fetch("publishable")
	if len(pub) != 1 || pub["C.Miss.2026"] == nil {
		t.Fatalf("① 可发布应仅含簇 C（三面 miss）·got %d 项: %v", len(pub), keysOf(pub))
	}

	// ② published：簇 A（覆盖面）+ 簇 B（记录面）+ 簇 D（existing 记录）——
	// 覆盖面命中也归已发布；existing 属 List 六终态（记录在案口径）
	ped := fetch("published")
	if ped["A.Cover.2026"] == nil || ped["B.Rec.2026"] == nil || ped["D.Exist.2026"] == nil {
		t.Fatalf("② 已发布应含簇 A（覆盖面）/B（记录面）/D（existing）·got %v", keysOf(ped))
	}
	if ped["C.Miss.2026"] != nil {
		t.Fatalf("② 簇 C 不应在已发布")
	}

	// ③ 四色数据：簇 A 选中站 tag（置顶第一位·covered=true·非 ours）
	tagsA := siteTagsOf(ped["A.Cover.2026"])
	if len(tagsA) == 0 || tagsA[0]["name"] != "目标站" {
		t.Fatalf("③ 簇 A 选中站应置顶 site_tags 首位·got %v", tagsA)
	}
	if tagsA[0]["covered"] != true || tagsA[0]["ours"] != false || tagsA[0]["seeded"] != false {
		t.Fatalf("③ 簇 A 选中站属性应 covered=true ours=false seeded=false·got %v", tagsA[0])
	}
	// 簇 B 选中站 tag（ours=true=绿）
	tagsB := siteTagsOf(ped["B.Rec.2026"])
	if len(tagsB) == 0 || tagsB[0]["ours"] != true {
		t.Fatalf("③ 簇 B 选中站应 ours=true（绿）·got %v", tagsB)
	}
	// 簇 D 选中站 tag（existing·记录面命中→蓝置顶）：绿=发布事实口径
	// （Skip 五终态）·existing 不标绿（ours=false·别人发的蓝）
	tagsD := siteTagsOf(ped["D.Exist.2026"])
	if len(tagsD) == 0 || tagsD[0]["name"] != "目标站" {
		t.Fatalf("③ 簇 D 选中站应置顶（existing 命中记录面）·got %v", tagsD)
	}
	if tagsD[0]["ours"] != false {
		t.Fatalf("③ 簇 D existing 终态不应标绿（ours 应 false——站上是别人的）·got %v", tagsD[0])
	}
	// 簇 C（可发布）：三面 miss → 选中站无 tag（site_tags 为空或不含目标站）
	tagsC := siteTagsOf(pub["C.Miss.2026"])
	for _, tg := range tagsC {
		if tg["name"] == "目标站" {
			t.Fatalf("③ 簇 C 三面 miss·选中站不应有 tag·got %v", tagsC)
		}
	}
}

func keysOf(m map[string]map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func siteTagsOf(item map[string]interface{}) []map[string]interface{} {
	raw, ok := item["site_tags"]
	if !ok {
		return nil
	}
	b, _ := json.Marshal(raw)
	var out []map[string]interface{}
	_ = json.Unmarshal(b, &out)
	return out
}
