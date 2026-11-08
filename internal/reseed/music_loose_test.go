package reseed

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.477: 音乐宽松轮锚——毒案拒绝（Prince 跨专辑/Vol.41 分卷）+
// 正当变体放行（同专辑格式变体/中文专辑名）+无锚词不进。
// 案源：12 环境七簇失败深查·29 离线驱动实证（§59.477 案情）。
func TestMusicLoosePick(t *testing.T) {
	gb := int64(1024 * 1024 * 1024)
	srcTitle100 := "[The 100 Greatest Recordings of all time]"
	srcTitleGP := "Great Pianists of the 20th Century"

	// 毒案1：Prince 跨专辑（29 驱动实测 tid=157591 曾被视频宽松轮放行）
	prince := []*model.SeedingSearchResult{
		{TorrentID: "157591", Title: "Prince - Sign O' The Times 2020 - FLAC 分轨", Size: int64(5.52 * float64(gb))},
	}
	if m := musicLoosePick(prince, srcTitle100, 79*gb); m != nil {
		t.Errorf("Prince 跨专辑应拒（标题强匹配不过）: %+v", m)
	}
	// 毒案2：Vol.41 分卷（size 带 4x 拒——61GB 源 × 0.36GB 候选）
	vols := []*model.SeedingSearchResult{
		{TorrentID: "120985", Title: "Friedrich Gulda - Great Pianists of The 20th Century Vol.41 1999 - FLAC", Size: int64(0.36 * float64(gb))},
	}
	if m := musicLoosePick(vols, srcTitleGP, 61*gb); m != nil {
		t.Errorf("Vol.41 分卷应拒（size 带 4x）: %+v", m)
	}
	// 正案1：同专辑格式变体（FLAC→MP3 双版·2.4x 差在带内·标题全词命中）
	variant := []*model.SeedingSearchResult{
		{TorrentID: "9", Title: "Carol Albert - Stronger Now 2020 - MP3", Size: int64(0.35 * float64(gb))},
	}
	if m := musicLoosePick(variant, "Carol Albert - Stronger Now (2020)", int64(0.84*float64(gb))); m == nil {
		t.Error("同专辑变体应放行（size 带内+标题 4/4 命中）")
	}
	// 正案2：中文专辑名（CJK 段命中=强匹配·size 精确）
	cn := []*model.SeedingSearchResult{
		{TorrentID: "8", Title: "陈慧娴 - 归来吧 1992 FLAC 分轨", Size: gb},
	}
	if m := musicLoosePick(cn, "陈慧娴 - 归来吧 (1992)", gb); m == nil {
		t.Error("中文专辑 CJK 段命中应放行")
	}
	// 无锚词：无有效词且无 CJK → 不进宽松（宁漏勿误）
	if m := musicLoosePick(cn, "2020", gb); m != nil {
		t.Error("无锚词不应进宽松轮")
	}
	// 词边界防御：候选词形含源词子串但非独立词（"time"⊄"Times" 独立词形）
	sub := []*model.SeedingSearchResult{
		{TorrentID: "7", Title: "Best Times Collection 1999", Size: gb},
	}
	if m := musicLoosePick(sub, "Time Recordings 1999", gb); m != nil {
		t.Errorf("子串伪命中应拒（词边界）: %+v", m)
	}
}

// §59.477: 音乐标题强匹配单元锚（词边界/阈值/CJK）。
func TestMusicTitleStrongMatch(t *testing.T) {
	words := []string{"greatest", "recordings", "time"} // extractMeaningfulTitleWords 对案1源的实际产出形态
	cjk := []string{}
	if musicTitleStrongMatch(words, cjk, "Prince - Sign O' The Times 2020 - FLAC 分轨") {
		t.Error("Prince 案：仅 'time'⊆'Times' 子串伪命中应拒")
	}
	if !musicTitleStrongMatch(words, cjk, "The 100 Greatest Recordings Of All Time (1999)") {
		t.Error("目标标题 3/3 词边界命中应过")
	}
	// 过半阈值：4 词源需 ≥max(2,2)=2
	w4 := []string{"great", "pianists", "20th", "century"}
	if musicTitleStrongMatch(w4, nil, "Great Something Else Entirely") {
		t.Error("1/4 命中低于阈值应拒")
	}
	if !musicTitleStrongMatch(w4, nil, "Great Pianists of the 20th Century Vol.41") {
		t.Error("4/4 命中应过（分卷由 size 带拒·标题层放行正确）")
	}
	// CJK 强匹配
	if !musicTitleStrongMatch(nil, []string{"归来吧"}, "陈慧娴 - 归来吧 1992") {
		t.Error("CJK 段命中应过")
	}
}

// §59.486: 单词关键词门挂钩——厂牌/艺术家名单词（Monstercat 83.5GB 案：
// max(2,..) 恒 2 → 单词 1 命中永拒 → 全候选灭 → iyuu fallback exhausted）。
func TestMusicTitleStrongMatch_SingleWord(t *testing.T) {
	// 单词整词命中（多词候选）→ 过（修复点）
	if !musicTitleStrongMatch([]string{"monstercat"}, nil, "Monstercat - Best of 2023 2023 - FLAC 分軌") {
		t.Errorf("单词关键词整词命中应过（Monstercat 案）")
	}
	if !musicTitleStrongMatch([]string{"monstercat"}, nil, "Various Artists - Monstercat - Best of 2025 2025 - FLAC 分軌 - Qobuz") {
		t.Errorf("单词关键词长候选命中应过")
	}
	// 单词不命中 → 拒
	if musicTitleStrongMatch([]string{"monstercat"}, nil, "Naxos 25 Years Collection FLAC") {
		t.Errorf("单词不命中应拒")
	}
	// 单词子串不命中（词边界防线保留——§59.477 time⊆Times 防线）
	if musicTitleStrongMatch([]string{"monster"}, nil, "Monstercat Best of 2023") {
		t.Errorf("子串形态应拒（词边界防线）")
	}
	// 多词行为不变：4 词 2 命中过 / 1 命中拒
	if !musicTitleStrongMatch([]string{"koda", "kumi", "feel", "mind"}, nil, "KODA KUMI - feel my mind 2004 - FLAC") {
		t.Errorf("多词过半命中应过（不变）")
	}
	if musicTitleStrongMatch([]string{"koda", "kumi", "feel", "mind"}, nil, "koda something else FLAC") {
		t.Errorf("多词 1 命中<2 应拒（不变）")
	}
}
