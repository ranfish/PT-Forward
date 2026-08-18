package reseed

import (
	"strconv"
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.36 修订: MI 仲裁三级链锚定。
func TestResolveAudioConflict(t *testing.T) {
	trueHDTitle := "Project Hail Mary 2026 UHD BluRay 2160p x265 DV HDR10plus TrueHD7.1 Atmos mUHD-FRDS"
	dtsTitle := "Project Hail Mary 2026 UHD BluRay 2160p x265 DV HDR10plus DTS-HD MA 5.1 mUHD-FRDS"

	// 真实 MI 片段（Hail Mary 详情页 Audio #1: TrueHD Atmos；DTS 版模拟）
	miTrueHD := "Audio #1\nFormat : MLP FBA 16-ch\nCommercial Name : Dolby TrueHD with Dolby Atmos\nLanguage : English"
	miDTS := "Audio #1\nFormat : DTS XLL\nCommercial Name : DTS-HD Master Audio\nLanguage : English"

	candTrueHD := &L2MatchResult{TorrentID: "1", Title: trueHDTitle, Size: 35057723600}
	candDTS := &L2MatchResult{TorrentID: "2", Title: dtsTitle, Size: 35057723600}

	t.Run("HailMary_源MI为TrueHD_选对候选", func(t *testing.T) {
		// 源 local MI（真测量）= TrueHD（文件名标错 DTS-HD MA）→ 应选站内 TrueHD 版
		got := ResolveAudioConflict([]*L2MatchResult{candTrueHD, candDTS}, miTrueHD, map[string]string{"1": miTrueHD, "2": miDTS})
		if got == nil || got.TorrentID != "1" {
			t.Errorf("应仲裁选 TrueHD 版（id=1），got %+v", got)
		}
	})
	t.Run("源MI为DTS_站内两版本_选对DTS版", func(t *testing.T) {
		got := ResolveAudioConflict([]*L2MatchResult{candTrueHD, candDTS}, miDTS, map[string]string{"1": miTrueHD, "2": miDTS})
		if got == nil || got.TorrentID != "2" {
			t.Errorf("应仲裁选 DTS 版（id=2），got %+v", got)
		}
	})
	t.Run("MI不一致_单候选出局", func(t *testing.T) {
		got := ResolveAudioConflict([]*L2MatchResult{candTrueHD}, miDTS, map[string]string{"1": miTrueHD})
		if got != nil {
			t.Errorf("唯一候选 MI 不一致应出局（真不同资源），got %+v", got)
		}
	})
	t.Run("源MI不可得_降级盲放行", func(t *testing.T) {
		got := ResolveAudioConflict([]*L2MatchResult{candTrueHD}, "", nil)
		if got == nil || got.TorrentID != "1" {
			t.Errorf("源 MI 缺失应降级②放行首候选，got %+v", got)
		}
	})
	t.Run("候选MI缺失_降级放行", func(t *testing.T) {
		got := ResolveAudioConflict([]*L2MatchResult{candTrueHD}, miTrueHD, nil)
		if got == nil || got.TorrentID != "1" {
			t.Errorf("候选 MI 缺失应降级②放行，got %+v", got)
		}
	})
	t.Run("MI解析空值_证据不足_降级放行", func(t *testing.T) {
		weirdMI := "Audio #1\nFormat : SOMETHING_UNKNOWN\n"
		got := ResolveAudioConflict([]*L2MatchResult{candTrueHD}, weirdMI, map[string]string{"1": miTrueHD})
		if got == nil || got.TorrentID != "1" {
			t.Errorf("解析空值不应判拒（证据不足走②），got %+v", got)
		}
	})
	t.Run("MI一致优先_降级候选兜底", func(t *testing.T) {
		// 候选1 MI 缺失（降级），候选2 MI 一致 → 2 胜出
		got := ResolveAudioConflict([]*L2MatchResult{candTrueHD, candDTS}, miDTS, map[string]string{"2": miDTS})
		if got == nil || got.TorrentID != "2" {
			t.Errorf("MI 一致候选应优先于降级候选，got %+v", got)
		}
	})
	t.Run("空候选列表", func(t *testing.T) {
		if got := ResolveAudioConflict(nil, miTrueHD, nil); got != nil {
			t.Errorf("空列表应返回 nil")
		}
	})
}

// AudioConflictCandidates: 提取器分层判定（仅音频冲突才进仲裁池）
func TestAudioConflictCandidatesExtractor(t *testing.T) {
	source := "挽救计划.Project.Hail.Mary.2026.UHD.BluRay.2160p.x265.DV.HDR10plus.DTS-HD.MA.5.1.mUHD-FRDS"
	audioConflict := "Project Hail Mary 2026 UHD BluRay 2160p x265 DV HDR10plus TrueHD7.1 Atmos mUHD-FRDS"
	videoConflict := "Project Hail Mary 2026 UHD BluRay 2160p x264 DV HDR10plus TrueHD7.1 Atmos mUHD-FRDS"
	groupMiss := "Project Hail Mary 2026 UHD BluRay 2160p x265 DV HDR10plus TrueHD7.1 Atmos-OTHER"

	results := mkResults2([][3]string{
		{"1", audioConflict, "35057723600"},
		{"2", videoConflict, "35057723600"},
		{"3", groupMiss, "35057723600"},
	})
	got := AudioConflictCandidates(results, "FRDS", 35057723600, source)
	if len(got) != 1 || got[0].TorrentID != "1" {
		t.Errorf("应只提取音频冲突候选 id=1（视频冲突/组名 miss 出局），got %+v", got)
	}
}

func mkResults2(rows [][3]string) []*model.SeedingSearchResult {
	var out []*model.SeedingSearchResult
	for _, r := range rows {
		size, _ := strconv.ParseInt(r[2], 10, 64)
		out = append(out, &model.SeedingSearchResult{TorrentID: r[0], Title: r[1], Size: size})
	}
	return out
}
