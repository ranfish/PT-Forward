package publish

import (
	"encoding/json"
	"strings"

	"github.com/ranfish/pt-forward/internal/metadata"
	"github.com/ranfish/pt-forward/internal/metadata/extract"
	"github.com/ranfish/pt-forward/internal/model"
	"github.com/ranfish/pt-forward/internal/titleparser"
)

// piggo_inject.go §59.541: 二师兄（piggo）发布特化注入——分区判据覆写 +
// 体裁分类数组生成（form_config 值映射覆盖不了的两组装逻辑：组合判据与
// 多值数组）。
//
// 数据源：PTGen genre/Region（meta.PTGenSourceJSON——rousi §59.458 同款）+
// tp.SeasonEpisode（电影/剧集形态——动画电影 19 分类判据）。

// 猪猪分区常量（catalog 2026-11-16 快照权威——piggo.md §59.541）。
const (
	piggoSecMovie = 1 // 电影
	piggoSecTV    = 2 // 电视剧
	piggoSecShow  = 3 // 综艺
	piggoSecKids  = 5 // 少儿
	piggoSecDoc   = 6 // 纪录片
	piggoSecAnime = 7 // 动漫
)

// piggoGenreCommon 电影/电视剧区共用体裁表（11 项——catalog 同款复用）。
var piggoGenreCommon = map[string]int{
	"剧情": 1, "喜剧": 2, "动作": 3, "爱情": 4,
	"奇幻": 5, "科幻": 5, // 奇幻科幻同分类 id=5
	"惊悚": 7, "悬疑": 8, "战争": 9, "历史": 10,
}

// piggoCategoryTable 分区→PTGen genre 词→分类 id（81 分类有效子集——
// 站方不纠正体裁（MI 纠正只盖 taxonomy），映射质量=审核质量）。
var piggoCategoryTable = map[int]map[string]int{
	piggoSecMovie: piggoGenreCommon,
	piggoSecTV:    piggoGenreCommon,
	piggoSecShow: { // 综艺 9 项：真人秀28 脱口秀29 访谈30 晚会31 选秀偶像32 喜剧2 生活美食33 文化34 其他11
		"真人秀": 28, "脱口秀": 29, "喜剧": 2, "生活美食": 33, "美食": 33, "文化": 34,
	},
	piggoSecKids: { // 少儿 10 项：动画6 动画电影19 儿歌12 真人13 益智启蒙14 冒险15 奇幻科幻5 科普16 玩具交通17 其他11
		"冒险": 15, "奇幻": 5, "科幻": 5, "科普": 16, "益智启蒙": 14, "儿童": 14,
		// 「动画」「动画电影」不在词表——按 S/E 形态特判注入（piggoCategoryIds）
	},
	piggoSecDoc: { // 纪录 9 项：自然41 历史10 社会42 人物43 科学44 战争9 生活美食33 旅行45 其他11
		"自然": 41, "历史": 10, "社会": 42, "人物": 43, "传记": 43, "科学": 44,
		"战争": 9, "生活美食": 33, "美食": 33, "旅行": 45,
	},
	piggoSecAnime: { // 动漫 9 项：热血35 日常36 恋爱37 奇幻科幻5 搞笑38 治愈39 悬疑8 竞技40 其他11
		"热血": 35, "日常": 36, "恋爱": 37, "奇幻": 5, "科幻": 5,
		"搞笑": 38, "治愈": 39, "悬疑": 8, "竞技": 40,
	},
}

// piggoFallbackCategory 分区兜底分类（「其他」——多区通用 id=11）。
const piggoFallbackCategory = 11

// piggoPublishInject §59.541: 猪猪特化注入——
//   - section_id 覆写：animation 组合判据（产地日本→7 动漫 / genre 含
//     家庭·儿童→5 少儿 / 其余→7 默认——站方两区分界=受众年龄感人工判断，
//     摸站 8 颗实证样本 4/4：千与千寻·你的名字·蜘蛛侠平行宇宙→7 /
//     玩具总动员·小黄人→5）
//   - category_ids 生成：genre 词表分区感知映射（命中几个挂几个——Ronin
//     [3,7] 动作+惊悚站方多体裁惯例）+零命中兜 [11]（Rocky III 单项实证
//     合法）
//   - source/processing 域特化组装（jobs 通用段不消费这两域——海豹同款
//     特化先例：来源域键=medium canonical（mediumStdKeyOf 单源）/处理域
//     键=hdr canonical；miss 由 adapter 空维兜 Other/SDR）
func piggoPublishInject(form map[string]string, cfg *model.PublishFormConfig, meta *model.TorrentMetadata, tp titleparser.TechProfile) {
	if form == nil || meta == nil {
		return
	}
	genres, regionIsJapan := piggoPTGenFacts(meta)

	// section_id 组合判据（仅 animation——其余分区 type 值映射已定）
	if needsPiggoSectionOverride(meta) {
		switch {
		case regionIsJapan:
			form["section_id"] = "7"
		case piggoGenreHas(genres, "家庭", "儿童"):
			form["section_id"] = "5"
		default:
			form["section_id"] = "7"
		}
	}

	// category_ids 体裁数组
	var sectionID int
	if v := form["section_id"]; v != "" {
		if n, ok := atoiSafe(v); ok {
			sectionID = n
		}
	}
	ids := piggoCategoryIds(sectionID, genres, tp.SeasonEpisode)
	if b, err := json.Marshal(ids); err == nil {
		form["category_ids"] = string(b)
	}

	// source 域（来源 taxonomy——jobs 段无此域通用消费·特化；cfg nil=
	// 测试形态/未配置站跳过）
	if cfg != nil {
		if fn, ok := cfg.FormFields["source"]; ok && fn != "" {
			if _, filled := form[fn]; !filled {
				if key := mediumStdKeyOf(tp, ""); key != "" {
					for _, m := range cfg.ValueMappings["source"] {
						for _, k := range m.StandardKeys {
							if k == key {
								form[fn] = m.Value
								break
							}
						}
					}
				}
			}
		}
		// processing 域（HDR 处理 taxonomy——同款特化）
		if fn, ok := cfg.FormFields["processing"]; ok && fn != "" {
			if _, filled := form[fn]; !filled {
				if key := extract.LookupStandardKey("hdr", tp.HDR); key != "" {
					for _, m := range cfg.ValueMappings["processing"] {
						for _, k := range m.StandardKeys {
							if k == key {
								form[fn] = m.Value
								break
							}
						}
					}
				}
			}
		}
	}
}

// needsPiggoSectionOverride animation 类型才需要判据覆写（meta.Category
// 形态 category.animation——§59.424 门同源；type 值映射已给 7 默认兜底）。
func needsPiggoSectionOverride(meta *model.TorrentMetadata) bool {
	return meta.Category == "category.animation"
}

// piggoPTGenFacts PTGen genre 词表 + 产区是否日本（Region 合拍串首值
// 优先——rousi region 同款消费形态）。
func piggoPTGenFacts(meta *model.TorrentMetadata) ([]string, bool) {
	src, err := metadata.UnmarshalPTGenSource(meta.PTGenSourceJSON)
	if err != nil || src == nil {
		return nil, false
	}
	japan := false
outer:
	for _, raw := range src.Region {
		for _, part := range strings.Split(raw, "/") {
			w := strings.TrimSpace(part)
			if w == "" {
				continue
			}
			if strings.Contains(w, "日本") {
				japan = true
				break outer
			}
			break outer // 首值优先（合拍首值——§59.391 葡萄先例）
		}
	}
	return src.Genre, japan
}

// piggoGenreHas 词表包含任一目标词。
func piggoGenreHas(genres []string, want ...string) bool {
	for _, g := range genres {
		g = strings.TrimSpace(g)
		for _, w := range want {
			if g == w {
				return true
			}
		}
	}
	return false
}

// piggoCategoryIds 体裁分类数组：分区感知词表映射（保序去重）+少儿区
// 动画形态特判（无 S/E=电影→19 动画电影 / 有=剧集→6 动画——小黄人 [19,..]
// 实测形态）+零命中兜 [11]。
func piggoCategoryIds(sectionID int, genres []string, seasonEpisode string) []int {
	table := piggoCategoryTable[sectionID]
	var ids []int
	seen := map[int]bool{}
	add := func(id int) {
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	// 少儿区动画形态特判（在词表前——体裁主分类）
	if sectionID == piggoSecKids {
		if strings.TrimSpace(seasonEpisode) == "" {
			add(19) // 动画电影（无 S/E 信号=电影形态）
		} else {
			add(6) // 动画（剧集形态）
		}
	}
	for _, g := range genres {
		g = strings.TrimSpace(g)
		if id, ok := table[g]; ok {
			add(id)
		}
	}
	if len(ids) == 0 {
		ids = []int{piggoFallbackCategory}
	}
	return ids
}

// atoiSafe 小工具（section_id form 字符串→int）。
func atoiSafe(s string) (int, bool) {
	n := 0
	if s == "" {
		return 0, false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
		if n > 1000 {
			return 0, false
		}
	}
	return n, true
}
