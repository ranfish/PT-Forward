package api

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/comment"
	"github.com/ranfish/pt-forward/internal/model"
)

// §59.469: 音乐种分流判定锚——S1 零误判/S2 官组词/过滤结构性排除（天空案）。
func TestMusicSeedRouting(t *testing.T) {
	// S1：coverage 含音乐源站（任意状态行——tracker 信号）
	musicCov := []model.SiteCoverageCache{
		{SiteName: "海豚", Status: model.CoverageConfirmedHas, Source: model.CoverageSourceTracker, TorrentID: ""},
	}
	if !isMusicSeedForRouting("Enrique Ugarte - Valse Musette", musicCov) {
		t.Error("S1：海豚 tracker 在场应判音乐种")
	}
	// 影视 coverage（天空+非音乐源站）不命中
	videoCov := []model.SiteCoverageCache{
		{SiteName: "天空", Status: model.CoverageConfirmedHas, TorrentID: "536401"},
	}
	if isMusicSeedForRouting("Enrique Ugarte - Valse Musette (1993)", videoCov) {
		t.Error("纯影视 coverage 不应判音乐种")
	}
	// S2：官组词（集散地救回——tracker 已非 open.cd 但种名带 OpenCD）
	if !isMusicSeedForRouting("群星 - XXX 2019 FLAC - OpenCD", nil) {
		t.Error("S2：标题含 OpenCD 应判音乐种")
	}
	if !isMusicSeedForRouting("陈慧娴 - 归来吧 1992 FLAC 分轨 - ZHENGLIFAN@Musopia", nil) {
		t.Error("S2：标题含 @Musopia 应判音乐种")
	}
	// S2 零误命中：普通标题/影视标题/OpenCDX 子串负例（词边界防御）
	for _, bad := range []string{
		"Movie 2020 1080p BluRay x264-OpenCDX", // OpenCDX 影视组——子串不含完整词 OpenCD
		"随便什么标题",
		"OpenCDK Team Release 2020",
	} {
		if isMusicSeedForRouting(bad, nil) {
			t.Errorf("不应判音乐种: %q", bad)
		}
	}
	// 词边界正例：dash/空格/首位的完整 OpenCD
	for _, good := range []string{
		"群星 - XXX 2019 FLAC - OpenCD",
		"OpenCD 官组作品集",
	} {
		if !isMusicSeedForRouting(good, nil) {
			t.Errorf("应判音乐种: %q", good)
		}
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
