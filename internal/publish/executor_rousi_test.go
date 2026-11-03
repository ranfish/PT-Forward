package publish

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.458-②③: 肉丝 attributes 注入锚——genre 剥 miss/region norm+slug/
// poster 直塞（站方词表内直投+外词剥除+空兜其它）。
func TestRousiPublishInject(t *testing.T) {
	meta := &model.TorrentMetadata{
		Poster: "https://doubaninfo.com/dbposter/x.jpg",
		PTGenSourceJSON: `{"genre": ["动画", "歌舞", "同性"], "region": ["中国大陆 / 中国香港"]}`,
	}
	form := map[string]string{}
	rousiPublishInject(form, meta)

	// genre：同性（站方未收）剥除——剩 动画/歌舞
	if form["rs_genre"] != `["动画","歌舞"]` {
		t.Errorf("genre: %q", form["rs_genre"])
	}
	// region：合拍首值 中国大陆→大陆→mainland-china
	if form["rs_region"] != "mainland-china" {
		t.Errorf("region: %q", form["rs_region"])
	}
	if form["rs_poster"] != meta.Poster {
		t.Errorf("poster: %q", form["rs_poster"])
	}

	// miss 产区兜 other + 空 genre 兜 其它
	meta2 := &model.TorrentMetadata{PTGenSourceJSON: `{"genre": ["同性"], "region": ["伊朗"]}`}
	form2 := map[string]string{}
	rousiPublishInject(form2, meta2)
	if form2["rs_genre"] != `["其它"]` {
		t.Errorf("空兜 genre: %q", form2["rs_genre"])
	}
	if form2["rs_region"] != "other" {
		t.Errorf("miss region: %q", form2["rs_region"])
	}

	// 美国/日本直配 slug
	meta3 := &model.TorrentMetadata{PTGenSourceJSON: `{"region": ["美国 / 英国"]}`}
	form3 := map[string]string{}
	rousiPublishInject(form3, meta3)
	if form3["rs_region"] != "usa" {
		t.Errorf("usa: %q", form3["rs_region"])
	}
}

// §59.459: 野马注入锚——regionList 多产区全投≤3 去重/译名/poster。
func TestYemaPublishInject(t *testing.T) {
	meta := &model.TorrentMetadata{
		Poster: "https://img.example/p.jpg",
		PTGenSourceJSON: `{"region": ["中国大陆 / 美国", "日本"], "translated_titles": "测试译名"}`,
	}
	form := map[string]string{}
	yemaPublishInject(form, meta)
	// 大陆→1 美国→4 日本→6（三产区全投）
	if form["yema_regions"] != `["1","4","6"]` {
		t.Errorf("regions: %q", form["yema_regions"])
	}
	if form["yema_translated"] != "测试译名" {
		t.Errorf("translated: %q", form["yema_translated"])
	}
	if form["yema_poster"] != meta.Poster {
		t.Errorf("poster: %q", form["yema_poster"])
	}
	// miss 产区不投
	meta2 := &model.TorrentMetadata{PTGenSourceJSON: `{"region": ["伊朗"]}`}
	form2 := map[string]string{}
	yemaPublishInject(form2, meta2)
	if form2["yema_regions"] != "" {
		t.Errorf("miss regions 应空: %q", form2["yema_regions"])
	}
}
