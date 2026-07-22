package site

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestTrackerMatcher_TrackerDomains 验证显式 TrackerDomains 配置优先精确匹配。
// v0.0.266: 解决 agsvpt.trackers.work 这类 tracker 域名与站点域名不一致的问题。
func TestTrackerMatcher_TrackerDomains(t *testing.T) {
	db := setupMatcherTestDB(t)
	matcher := NewTrackerMatcher(db)

	cases := []struct {
		name    string
		tracker string
		want    string
	}{
		// agsvpt: tracker 域名 agsvpt.trackers.work ≠ 站点域名 www.agsvpt.com
		// 靠 TrackerDomains 显式配置才能匹配
		{"agsvpt trackers.work", "https://agsvpt.trackers.work/announce.php?passkey=xxx", "末日"},
		{"agsvpt tracker.cn", "https://tracker.agsvpt.cn/announce", "末日"},
		{"agsvpt ipv6", "https://ipv6.agsvpt.cn/announce", "末日"},
		// 核心域名 fallback 仍有效（憨憨没配 TrackerDomains，靠 tracker.hhanclub.net → hhanclub.net）
		{"hhanclub fallback", "https://tracker.hhanclub.net/announce", "憨憨"},
		// hdsky 配了 TrackerDomains，精确匹配
		{"hdsky exact", "https://tracker.hdsky.me/announce", "天空"},
		{"hdsky alt", "https://tracker.hdsky.my/announce", "天空"},
		// 未知 tracker
		{"unknown", "https://unknown.example.com/announce", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := matcher.Match(c.tracker)
			if got != c.want {
				t.Errorf("Match(%q) = %q, want %q", c.tracker, got, c.want)
			}
		})
	}
}

// TestTrackerMatcher_MatchAll 验证多 tracker 匹配（去重保序）。
func TestTrackerMatcher_MatchAll(t *testing.T) {
	db := setupMatcherTestDB(t)
	matcher := NewTrackerMatcher(db)

	urls := []string{
		"https://agsvpt.trackers.work/announce",
		"https://tracker.hhanclub.net/announce",
		"https://tracker.hdsky.me/announce",
		"https://agsvpt.trackers.work/announce", // 重复，应去重
		"https://unknown.example.com/announce",  // 未知，应忽略
	}
	got := matcher.MatchAll(urls)
	want := []string{"末日", "憨憨", "天空"}
	if len(got) != len(want) {
		t.Fatalf("MatchAll got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("MatchAll[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func setupMatcherTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Site{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sites := []model.Site{
		{Name: "末日", Domain: "www.agsvpt.com", BaseURL: "https://www.agsvpt.com", Enabled: true,
			TrackerDomains: `["agsvpt.trackers.work","tracker.agsvpt.cn","ipv6.agsvpt.cn","tracker.agsvpt.work"]`},
		{Name: "憨憨", Domain: "hhanclub.net", BaseURL: "https://hhanclub.net", Enabled: true},
		{Name: "天空", Domain: "hdsky.me", BaseURL: "https://hdsky.me", Enabled: true,
			TrackerDomains: `["tracker.hdsky.me","tracker.hdsky.my"]`},
	}
	for i := range sites {
		if err := db.Create(&sites[i]).Error; err != nil {
			t.Fatalf("create site: %v", err)
		}
	}
	return db
}
