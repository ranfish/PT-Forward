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

// musicSourceSitesOrdered §59.476: 音乐源站素材量序（皇后 12742 →
// 海豚 8264 → 乌托邦 131·§59.466）——确定性迭代（map 无序）：位置3
// 换 hash 候选序 + 兜底站首选共用。与 musicSourceSites 键集必须同步
// （init 校验防漂移）。
var musicSourceSitesOrdered = []string{"皇后", "海豚", "乌托邦"}

func init() {
	if len(musicSourceSites) != len(musicSourceSitesOrdered) {
		panic("music source sites: map/ordered list out of sync")
	}
	for _, s := range musicSourceSitesOrdered {
		if !musicSourceSites[s] {
			panic("music source sites: ordered list entry not in map: " + s)
		}
	}
}

// MusicSourceSites 返回音乐源站名单（素材量序·§59.476 确定性）。
func MusicSourceSites() []string {
	return append([]string(nil), musicSourceSitesOrdered...)
}

// IsMusicSourceSite 判定站点是否音乐源站（S1 判定/音乐链选站消费口）。
func IsMusicSourceSite(siteName string) bool {
	return musicSourceSites[siteName]
}
