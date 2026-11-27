package reseed

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.580: 辅种侧黑名单门锚（matchAtSite 站点循环单点）——源种子
// 组在目标站黑名单→该站不出候选（保守防御·用户定案①）。
// 断言口径=黑名单门专属 Debug 日志（路径可观测——返回值 nil 与
// 后续链 nil 同值无法区分·日志是门走到的唯一证据）。
//
// 阴性对照：删门/放行形态 → 专属日志消失 → 锚 FAIL。
func TestMatchAtSite_BannedGroupFiltered(t *testing.T) {
	newEngine := func() (*Engine, *observer.ObservedLogs) {
		core, logs := observer.New(zap.DebugLevel)
		return &Engine{logger: zap.New(core)}, logs
	}
	mc := &matchConfig{ps: &preloadedSites{}}

	// 前缀族命中（CHD*——彩虹岛官组 6 组全前缀实证）
	e, logs := newEngine()
	src := sourceTorrent{
		InfoHash: "abc123", SiteName: "源头站",
		Name: "Movie.2026.1080p.BluRay.x264-CHDBits",
	}
	siteInfo := &model.SiteInfo{Name: "瓷器"}
	if c := e.matchAtSite(mc, src, siteInfo); c != nil {
		t.Errorf("CHD* 组种子在瓷器辅种应被过滤（返回 nil）·实得 %+v——黑名单门回归", c)
	}
	found := false
	for _, l := range logs.All() {
		if l.Message == "reseed site banned group filtered" {
			found = true
		}
	}
	if !found {
		t.Error("黑名单门专属日志未出现——门被删/放行形态回归（阴性对照应 FAIL 的路径）")
	}

	// 精确命中（大小写不敏感）
	e2, logs2 := newEngine()
	src2 := sourceTorrent{InfoHash: "h2", SiteName: "源头站", Name: "Show.S01.720p.WEB-DL.x264-seehd"}
	if c := e2.matchAtSite(mc, src2, siteInfo); c != nil {
		t.Errorf("seehd（大小写不敏感）应被过滤·实得 %+v", c)
	}
	found2 := false
	for _, l := range logs2.All() {
		if l.Message == "reseed site banned group filtered" {
			found2 = true
		}
	}
	if !found2 {
		t.Error("精确命中路径的门日志未出现")
	}

	// 非黑名单站：门不走（无日志）+后续链不 panic
	e3, logs3 := newEngine()
	siteInfo2 := &model.SiteInfo{Name: "别的站"}
	_ = e3.matchAtSite(mc, src, siteInfo2)
	for _, l := range logs3.All() {
		if l.Message == "reseed site banned group filtered" {
			t.Error("别的站（无黑名单）不应触发门日志")
		}
	}
}
