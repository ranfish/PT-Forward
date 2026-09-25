package api

import "testing"

// §59.281: OTA 任务门控——BusyTaskDesc 空闲态
func TestBusyTaskDescIdle(t *testing.T) {
	h := &PublishTorrentsHandler{}
	h.siteBatch.tasks = map[string]*siteBatchTask{}
	h.siteBatch.active = map[string]string{}
	if d := h.BusyTaskDesc(); d != "" {
		t.Errorf("空闲态应空, got %q", d)
	}
}

// §59.285: 主链 PTGen 查询键三级化选择逻辑（DoubanURL→IMDbURL→Title）
func TestMainlinePTGenQueryKey(t *testing.T) {
	cases := []struct {
		douban, imdb, title, want, wantKind string
	}{
		{"https://douban.com/s/1", "", "T", "https://douban.com/s/1", "douban"},   // 豆瓣优先
		{"", "https://imdb.com/tt1", "T", "https://imdb.com/tt1", "imdb"},          // Diesel 案
		{"", "", "Diesel 2025 1080P", "Diesel 2025 1080P", "title"},                // name 兜底
		{"", "", "", "", ""},                                                        // 全空 skip
	}
	for _, c := range cases {
		q, kind := mainlineQueryKey(c.douban, c.imdb, c.title)
		if q != c.want || kind != c.wantKind {
			t.Errorf("[%v|%v|%v] got (%v,%v) want (%v,%v)", c.douban, c.imdb, c.title, q, kind, c.want, c.wantKind)
		}
	}
}
