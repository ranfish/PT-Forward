package reseed

import (
	"testing"

	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

func mkResults(id, title string, size int64) []*model.SeedingSearchResult {
	return []*model.SeedingSearchResult{{TorrentID: id, Title: title, Size: size}}
}

func parseTech(s string) titleparser.TechProfile {
	return titleparser.ParseTitleTech(s)
}

// §59.36 元数据获取宽松轮音频豁免（FRDS 挽救计划案例）。
//
// 站点数据实证（keepfrds id=2782301，2026-08-17）：
//   下载器种子名/文件名: ...DV.HDR10plus.DTS-HD.MA.5.1.mUHD-FRDS（发布者命名错误）
//   站内列表标题:       ...DV.HDR10plus.TrueHD7.1.Atmos.mUHD-FRDS（与内容物一致）
//   .torrent info.name 与下载器逐字一致；MediaInfo 实际音轨 TrueHD Atmos + 国配 AC-3
// → 音频冲突是站点文件名/标题标注不一致的虚假信号，非不同资源。
func TestLoosePickAudioExemption(t *testing.T) {
	source := "挽救计划.Project.Hail.Mary.2026.UHD.BluRay.2160p.x265.DV.HDR10plus.DTS-HD.MA.5.1.mUHD-FRDS"
	siteTitle := "Project Hail Mary 2026 UHD BluRay 2160p x265 DV HDR10plus TrueHD7.1 Atmos mUHD-FRDS"

	// 案例：loose 轮应命中（音频豁免后无其他冲突）
	results := mkResults("2782301", siteTitle, 35057723600)
	if m := loosePick(results, "FRDS", source); m == nil {
		t.Error("Hail Mary 案例：宽松轮应命中（音频标注差异豁免）")
	}

	// 真不同资源：视频编码不同（x264 vs x265 不在等价组）→ 宽松轮仍拒
	diffVideo := mkResults("9", "Project Hail Mary 2026 UHD BluRay 2160p x264 DV HDR10plus TrueHD7.1 Atmos mUHD-FRDS", 35057723600)
	if m := loosePick(diffVideo, "FRDS", source); m != nil {
		t.Error("视频编码冲突（x264 vs x265）：宽松轮不应放行")
	}

	// 真不同资源：HDR 不同（DoVi HDR10 vs SDR）→ 宽松轮仍拒
	diffHDR := mkResults("9", "Project Hail Mary 2026 UHD BluRay 2160p x265 SDR TrueHD7.1 Atmos mUHD-FRDS", 35057723600)
	if m := loosePick(diffHDR, "FRDS", source); m != nil {
		t.Error("HDR 冲突（DV.HDR10+ vs SDR）：宽松轮不应放行")
	}

	// 注：分辨率（2160p vs 1080p）不在 techProfileConflict 字段表（预存在设计）——
	// 由主轮 size 硬校验 + 标题词验证兜底，宽松轮对本维度本就不设防（非本变更引入）

	// 主轮严格性不变：音频冲突在主轮仍拒绝（注入/辅种路径保护）
	if !techProfileConflict(parseTech(source), siteTitle) {
		t.Error("主轮 techProfileConflict：音频冲突（DTS-HD MA vs TrueHD）仍应判冲突")
	}
	// 主轮字段级 skipAudio=false 行为 = 原行为
	if techProfileConflictFields(parseTech(source), siteTitle, true) {
		t.Error("skipAudio=true：音频冲突应豁免")
	}
	// 真不同资源的视频冲突在 skipAudio 下仍拒
	diffVideoTitle := "Project Hail Mary 2026 UHD BluRay 2160p x264 DV HDR10plus TrueHD7.1 Atmos mUHD-FRDS"
	if !techProfileConflictFields(parseTech(source), diffVideoTitle, true) {
		t.Error("skipAudio=true：视频编码冲突仍应判冲突")
	}
}
