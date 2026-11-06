package api

import (
	"context"
	"testing"

	"github.com/ranfish/pt-forward/internal/comment"
	"github.com/ranfish/pt-forward/internal/model"
)

// §59.469+§59.476: 音乐种分流判定锚——S1 零误判/S2+ 官组词（映射表
// 词表形态+组位/否决闸）/过滤结构性排除（天空案）/位置3 素材量序。
// 词表以 migration 151 种子行形态固定传入（musicTitleMatch 纯函数——
// DB 推导链在集成层）。
var testMusicWords = []string{"KHQ", "LLM", "Musopia", "OpenCD", "TSxD"}

func TestMusicSeedRouting(t *testing.T) {
	ctx := context.Background()
	h := &PublishTorrentsHandler{}
	// S1：coverage 含音乐源站（任意状态行——tracker 信号）
	musicCov := []model.SiteCoverageCache{
		{SiteName: "海豚", Status: model.CoverageConfirmedHas, Source: model.CoverageSourceTracker, TorrentID: ""},
	}
	if !h.isMusicSeedForRouting(ctx, "Enrique Ugarte - Valse Musette", musicCov) {
		t.Error("S1：海豚 tracker 在场应判音乐种")
	}
	// 影视 coverage（天空+非音乐源站）不命中
	videoCov := []model.SiteCoverageCache{
		{SiteName: "天空", Status: model.CoverageConfirmedHas, TorrentID: "536401"},
	}
	if h.isMusicSeedForRouting(ctx, "Enrique Ugarte - Valse Musette (1993)", videoCov) {
		t.Error("纯影视 coverage 不应判音乐种")
	}
	if h.isMusicSeedForRouting(ctx, "随便什么标题", videoCov) {
		t.Error("无官组词无音乐 coverage 不应判音乐种")
	}
	// 过滤：coverage/簇 双滤（天空出局·海豚保留）
	filtered := filterToMusicSources(append(videoCov, musicCov...))
	if len(filtered) != 1 || filtered[0].SiteName != "海豚" {
		t.Errorf("过滤应仅留海豚: %+v", filtered)
	}
	targets := []comment.DirectTarget{
		{SiteName: "天空", TorrentID: "536401"},
		{SiteName: "皇后", TorrentID: "123"},
	}
	ft := filterClusterToMusicSources(targets)
	if len(ft) != 1 || ft[0].SiteName != "皇后" {
		t.Errorf("簇过滤应仅留皇后: %+v", ft)
	}
}

// §59.476 S2+ 标题官组词判定锚（组位两形态/见到即判/否决闸/词边界）。
func TestMusicTitleMatch(t *testing.T) {
	// 组位权威形态（用户定义·括号外）——恒过（含影视技术词也过：
	// 组位=组名声明·影视圈无此组名）
	groupPos := []string{
		"Various Artist-NAXOS 25周年纪念 9Box 90CDs-wav-OpenCD",            // 尾段（12 环境实证下载名）
		"Gwyneth Herbert - Bittersweet And Blue 2005-FLAC Multi File-OpenCD 16bit 44.1kHz", // 尾段+spec 后缀
		"Thelonious Monk - Misterioso 1965-FLAC 分轨-Rosebudx@OpenCD🌱 保种区 ×2（已保护 88 小时）", // @锚+装饰（乌托邦实证）
		"陈慧娴 - 归来吧 1992 FLAC 分轨 - ZHENGLIFAN@Musopia", // @锚 Musopia
		"群星 - XXX 2019 FLAC - OpenCD",                     // 尾段
	}
	for _, good := range groupPos {
		if !musicTitleMatch(good, testMusicWords) {
			t.Errorf("组位形态应判音乐种: %q", good)
		}
	}
	// 非组位见到即判（无影视技术词 → 音乐）
	for _, good := range []string{
		"OpenCD 官组作品集",
		"OpenCD 精选合集 2024",
		"Musopia 原抓分享",
	} {
		if !musicTitleMatch(good, testMusicWords) {
			t.Errorf("非组位无技术词应判音乐种: %q", good)
		}
	}
	// 否决闸：非组位 ∧ 影视技术词 → 影视（AI 纪录片类内容词 LLM 假阳性拦截）
	for _, bad := range []string{
		"The Rise of LLM 2024 1080p WEB-DL H.264-CHDWEB",
		"LLM 革命纪录片 2160p x265",
		"OpenCD 电影档案 1080p BluRay",
	} {
		if musicTitleMatch(bad, testMusicWords) {
			t.Errorf("否决闸应拦影视形态: %q", bad)
		}
	}
	// 词边界负例：OpenCDX 子串不裂词
	for _, bad := range []string{
		"Movie 2020 1080p BluRay x264-OpenCDX",
		"OpenCDK Team Release 2020",
	} {
		if musicTitleMatch(bad, testMusicWords) {
			t.Errorf("不应判音乐种: %q", bad)
		}
	}
	// 括号内不算组位/不算见到（组名"必定在括号外"）——
	// "[OpenCD] 流水账 1080p" 括号剥除后无词·且技术词在 → 影视
	if musicTitleMatch("[OpenCD] 流水账 1080p", testMusicWords) {
		t.Error("括号内官组词不应命中（且技术词在场）")
	}
	// 空词表/空标题
	if musicTitleMatch("OpenCD 什么", nil) || musicTitleMatch("", testMusicWords) {
		t.Error("空输入防御")
	}
}

// §59.476 辅助函数锚：段首词提取/括号剥除。
func TestMusicTitleHelpers(t *testing.T) {
	if got := firstWordToken("  OpenCD 16bit 44.1kHz"); got != "OpenCD" {
		t.Errorf("firstWordToken spec 后缀: %q", got)
	}
	if got := firstWordToken("OpenCD🌱 保种区"); got != "OpenCD" {
		t.Errorf("firstWordToken emoji: %q", got)
	}
	if got := firstWordToken("全集 2012"); got != "" {
		t.Errorf("中文段首无词: %q", got)
	}
	if got := stripBracketSegments("A [B {C} (D)] E （中文）【中】"); got != "A  E " {
		t.Errorf("stripBracketSegments: %q", got)
	}
}
