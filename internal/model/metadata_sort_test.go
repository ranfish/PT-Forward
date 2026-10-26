package model

import "testing"
import "time"


// TestSortMetasAuthoritativeCompleteness §59.351: 完备度键（族③案——
// 簇传播统一 bump updated_at 平手时，残行（壹吧/修道院站产物少）不该
// 因 id 小恒赢全行（幸运/织梦行））。
func TestSortMetasAuthoritativeCompleteness(t *testing.T) {
	base := time.Date(2026, 10, 25, 14, 49, 17, 529081649, time.Local)
	metas := []TorrentMetadata{
		{ID: 1, FetchSource: "rss_detail", SiteName: "壹吧", Category: "", Poster: "", Screenshots: "", Description: "", MediaInfo: "mi", UpdatedAt: base}, // 残行（先插 id 小）
		{ID: 2, FetchSource: "rss_detail", SiteName: "幸运", Category: "category.movie", Poster: "p", Screenshots: `["a"]`, Description: "d", MediaInfo: "mi", UpdatedAt: base}, // 全行
	}
	SortMetasAuthoritative(metas)
	if metas[0].SiteName != "幸运" {
		t.Errorf("完备度平手决胜应选全行（幸运），got %s", metas[0].SiteName)
	}
	// 全残/全全时行为不变（id ASC 决胜）
	metas2 := []TorrentMetadata{
		{ID: 5, FetchSource: "rss_detail", SiteName: "B", Category: "x", UpdatedAt: base},
		{ID: 3, FetchSource: "rss_detail", SiteName: "A", Category: "x", UpdatedAt: base},
	}
	SortMetasAuthoritative(metas2)
	if metas2[0].SiteName != "A" {
		t.Errorf("完备度同分时 id ASC 决胜（A 先），got %s", metas2[0].SiteName)
	}
	// 权威级仍最优先（cluster 全行 vs 直获残行——直获赢）
	metas3 := []TorrentMetadata{
		{ID: 1, FetchSource: "cluster", SiteName: "cluster全", Category: "x", Poster: "p", Screenshots: "s", Description: "d", MediaInfo: "mi", UpdatedAt: base.Add(time.Hour)},
		{ID: 9, FetchSource: "rss_detail", SiteName: "直获残", Category: "", UpdatedAt: base},
	}
	SortMetasAuthoritative(metas3)
	if metas3[0].SiteName != "直获残" {
		t.Errorf("直获权威优先级应高于完备度，got %s", metas3[0].SiteName)
	}
}
