package reseed

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.59 时空奇旅错配案 seam 回归：分辨率冲突必须全路径拦截。
// 事实来源（243 审计实测，非代码推导）：
//   本地 0c4b57645d = 时空奇旅 Arco 2025 UHD 2160p DTS-HD MA 22.8GB
//   错配结果 = 朋友站 2782081 Arco 1080p DDP7.1 REPACK2 10.6GB
//   同版本存在 = 2782033（22.8GB 完全同规格）
func searchResults(withExact bool) []*model.SeedingSearchResult {
	base := []*model.SeedingSearchResult{
		{TorrentID: "2782031", Title: "时空奇旅.Arco.2025.BluRay.1080p.x265.10bit.DDP7.1.MNHD-FRDS", Size: 10600000000},
		{TorrentID: "2782048", Title: "时空奇旅.Arco.2025.UHD.BluRay.2160p.x265.10bit.DV.HDR.DTS-HD.MA.7.1.REPACK.mUHD-FRDS", Size: 25000000000},
		{TorrentID: "2782081", Title: "时空奇旅.Arco.2025.BluRay.1080p.x265.10bit.DDP7.1.REPACK2.MNHD-FRDS", Size: 10600000000},
	}
	if withExact {
		base = append(base, &model.SeedingSearchResult{
			TorrentID: "2782033",
			Title:     "时空奇旅.Arco.2025.UHD.BluRay.2160p.x265.10bit.DV.HDR.DTS-HD.MA.7.1.mUHD-FRDS",
			Size:      22800000000,
		})
	}
	return base
}

const arcoSource = "时空奇旅.Arco.2025.UHD.BluRay.2160p.x265.10bit.DV.HDR.DTS-HD.MA.7.1.mUHD-FRDS"

// 精确版本在场：主轮必须命中 2782033（字节级 size 相同优先返回）。
func TestVerifyMatch_ArcoExactVersionWins(t *testing.T) {
	m, _ := VerifyMatchWithTruncationCheckAndSource(searchResults(true), "FRDS", 22800000000, arcoSource)
	if m == nil {
		t.Fatal("同版本候选在场却无匹配")
	}
	if m.TorrentID != "2782033" {
		t.Fatalf("应精确命中 2782033（22.8GB 同规格），实得 %s（%s）", m.TorrentID, m.Title)
	}
}

// 同版本不在场（搜索页截断/翻页遗漏）：绝不能放行 1080p 版本，
// 也不得将其纳入音频仲裁候选——分辨率冲突在 skipAudio 视角下仍然致命。
func TestVerifyMatch_ArcoResolutionConflictBlocksAllPaths(t *testing.T) {
	if m, _ := VerifyMatchWithTruncationCheckAndSource(searchResults(false), "FRDS", 22800000000, arcoSource); m != nil {
		t.Fatalf("无同版本候选时不应匹配，实得 %s（%s）", m.TorrentID, m.Title)
	}
	cands := AudioConflictCandidates(searchResults(false), "FRDS", 22800000000, arcoSource)
	for _, c := range cands {
		if c.TorrentID == "2782081" || c.TorrentID == "2782031" {
			t.Fatalf("1080p 候选 %s 不得进入音频仲裁（分辨率冲突非音频问题）", c.TorrentID)
		}
	}
}
