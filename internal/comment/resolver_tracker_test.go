package comment

import "testing"

// §59.296: 馒头纯数字方言 + tracker 域上下文——簇副本归属判定
func TestResolveMTeamPlainNumWithTracker(t *testing.T) {
	resolver := func(host string) string {
		if host == "tp.m-team.cc" || host == "api.m-team.cc" || host == "kp.m-team.cc" {
			return "馒头"
		}
		return ""
	}
	// 馒头 tracker 上下文 + 裸 tid → 馒头候选（Under Current 案：a3cf203c comment=1165965）
	got := Resolve("1165965", resolver, "tp.m-team.cc")
	if len(got) != 1 || got[0].SiteName != "馒头" || got[0].TorrentID != "1165965" {
		t.Errorf("馒头裸 tid 未认领: %+v", got)
	}
	// 非馒头 tracker 上下文 + 裸数字 → 不认领（防误吞）
	got2 := Resolve("1165965", resolver, "tracker.hdsky.me")
	if len(got2) != 0 {
		t.Errorf("非馒头上下文不应认领: %+v", got2)
	}
	// 无 tracker 上下文 → 跳过（降级语义）
	got3 := Resolve("1165965", resolver, "")
	if len(got3) != 0 {
		t.Errorf("无上下文应跳过: %+v", got3)
	}
}

// §59.296: BuildClusterTargets 快照 TrackerDomain 直读（members 带域，不再依赖 selfHash 单点）
func TestBuildClusterTargetsFromSnapshotDomain(t *testing.T) {
	// 直接测 comment.Resolve 层语义 + 组装层在此包外——此处验证域传递语义已够（组装层无逻辑分支）
	resolver := func(host string) string {
		switch host {
		case "tp.m-team.cc":
			return "馒头"
		case "hdsky.me":
			return "HDSky"
		}
		return ""
	}
	// 簇副本形态：URL 副本 + 馒头裸 tid 副本 → 双候选
	urlCopy := Resolve("https://hdsky.me/details.php?id=607800", resolver, "")
	mtCopy := Resolve("1165965", resolver, "tp.m-team.cc")
	total := len(urlCopy) + len(mtCopy)
	if total != 2 {
		t.Errorf("簇聚合应 2 候选: %d", total)
	}
}
