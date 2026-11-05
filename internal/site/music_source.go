package site

// §59.469 音乐域角色体系单点（用户定案·实现路径第一步：硬编码）：
//
// MusicSourceSites = 音乐源站集合（音乐适配能力·可转出）。
// **三处消费**（同一权威定义）：
//   ① 音乐种判定 S1：tracker ∈ 音乐源站集合（零误判——纯音乐站无影视内容）
//   ② 音乐链选站：音乐源站 ∩ coverage（搜索空间限定——天空案结构性消失）
//   ③ 发布目标限定：音乐目标站集合（另见·防误发影视站）
//
// 名单（2026-11-05 定案+乌托邦纳入）：
//   皇后（open.cd·NP 定制·素材 12742）·海豚（dicmusic.com·Gazelle·素材 8264）
//   ·乌托邦（musopia.vip·NP 音乐专站·素材 131·三源最强 LogScore/转载链）
//
// 实现路径：第一步硬编码本单点 → 第二步迁 sites.music_source 列
// （migration+配置 UI·新站接入评估清单：数据完整性/字段可满足/禁转必检）
// → 届时本函数改为读 DB（消费方零改动）。

// musicSourceSites 音乐源站名单（站名——与 sites.name 一致）。
var musicSourceSites = map[string]bool{
	"皇后":  true,
	"海豚":  true,
	"乌托邦": true,
}

// MusicSourceSites 返回音乐源站名单。
func MusicSourceSites() []string {
	out := make([]string, 0, len(musicSourceSites))
	for name := range musicSourceSites {
		out = append(out, name)
	}
	return out
}

// IsMusicSourceSite 判定站点是否音乐源站（S1 判定/音乐链选站消费口）。
func IsMusicSourceSite(siteName string) bool {
	return musicSourceSites[siteName]
}
