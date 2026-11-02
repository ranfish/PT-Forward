package db

import (
	"gorm.io/gorm"
)

// migration109 §59.425: 碟粉（discfan.net）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（43.8KB）+ docs/32/discfan.md 581 行
// 读全 + **wiki 发布规则实时重读**（用户供 cookie 抓取——实时 vs 4 月
// 抄录六节逐节一致；wiki 独立认证体系=主站 cookie 不通）。
// 实时 vs 文档一致（文档 tags"10 个"计数错勘误——实列 9 项=实时 9）。
// 槽位映射：
//   - type 13：**电影按产区 7 细分**（大陆 401/港 404/台 405/泰 402/
//     日 403/韩 406/世界 410——discfanCategoryValue typeMappingOf 第六站
//     用户定案产区路由：首值优先+列举地区 miss→410 世界兜底；欧美已被
//     discfanReleaseBlocked 禁欧美上游拦——410 承接=亚洲小国等）；电影
//     8 项 label-only（必须经产区路由）；非电影通用链（411 剧集/
//     413 纪录双键/414 音乐双键/416 综艺/417 体育/419 动漫标准键直配）。
//   - **source_sel[4] 12 项=唯一质量槽**（极简站——无 medium/codec/
//     audiocodec/standard/team）：Web-DL 承接 webrip+4K UltraHD=
//     [uhd_bluray]+Blu-ray Disc=[bluray]+**Remux=131 异常值**双键归并+
//     Rip=[encode,bdrip]（重编码泛指）+HDTV+DVD=[dvd,dvdr]；复古媒介
//     SDTV/VCD/LD/VHS+Book label-only（无 standard 键——五年规则下
//     复古旧资源主战场，预览人工）。
//   - tags 9 配键 7：禁转/国语/中字/粤语/HDR 四键泛家族（站方单 HDR
//     标签——泛家族归一）/DoVi=[dolby_vision]/DIY；首发铁律+自购
//     label-only；值 3 站方跳号。
//   - 字段：url+pt_gen+technical_info+uplver；nfo 不配（13城定案）。
//   - anonymous:true（§59.422 附二全系统默认开启定案）。
// **站点级双检查（discfanReleaseBlocked 用户定案——挂 PushOnly 分支后）**：
//   ①五年规则（站规硬规则）：tp.Year（标题+PTGen 校正终态=Tab1 同源）
//     ≤当前年-5 放/年内拦/无年份放（29 库 0 例实证+审核门九字段无 year）。
//   ②禁欧美（用户策略性定案——欧美一颗不发连候选不走；站规"欧美需
//     候选"升级我方直接禁）：classifyRegionWords 亚洲四类放/有产区
//     但不中亚洲词表（regionNone）=欧美拦/无产区防御放（315/315
//     Region 全覆盖实证不触发）。
// 站规人工把关：音乐类须无损（<5.1 有损音频禁令=音乐专属——用户澄清
// 影视有损音轨不受限）/dupe（媒介优先级+断种 45 日·18 个月豁免）/
// upscale/RMVB/标清差质/样片/他站禁转/游戏 AV 政治禁。
// 站点定位：华语为主+日韩为辅（亚洲直发）/经典旧资源（五年规则）/
// 复古媒介（LD/VHS/VCD 全库唯一）。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(109, "discfan_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'discfan.net' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'discfan.net'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "source_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影 - 中国大陆", "value": "401"}, {"label": "电影 - 中国香港", "value": "404"}, {"label": "电影 - 中国台湾", "value": "405"}, {"label": "电影 - 泰国", "value": "402"}, {"label": "电影 - 日本", "value": "403"}, {"label": "电影 - 韩国", "value": "406"}, {"label": "电影 - 世界", "value": "410"}, {"label": "剧集", "value": "411", "standard_keys": ["category.tv_series"]}, {"label": "音乐", "value": "414", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "纪录", "value": "413", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "综艺", "value": "416", "standard_keys": ["category.tv_shows"]}, {"label": "体育", "value": "417", "standard_keys": ["category.sports"]}, {"label": "动漫", "value": "419", "standard_keys": ["category.animation"]}], "medium": [{"label": "HDTV", "value": "1", "standard_keys": ["medium.hdtv"]}, {"label": "4K UltraHD", "value": "2", "standard_keys": ["medium.uhd_bluray"]}, {"label": "Blu-ray Disc", "value": "3", "standard_keys": ["medium.bluray"]}, {"label": "DVD", "value": "4", "standard_keys": ["medium.dvd", "medium.dvdr"]}, {"label": "SDTV", "value": "5"}, {"label": "VCD", "value": "6"}, {"label": "LD", "value": "7"}, {"label": "VHS", "value": "8"}, {"label": "Web-DL", "value": "9", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "Rip", "value": "10", "standard_keys": ["medium.encode", "medium.bdrip"]}, {"label": "Book", "value": "11"}, {"label": "Remux", "value": "131", "standard_keys": ["medium.remux", "medium.uhd_remux"]}], "tags": [{"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "首发", "value": "2"}, {"label": "自购", "value": "9"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "粤语", "value": "8", "standard_keys": ["tag.cantonese_audio"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "DoVi", "value": "10", "standard_keys": ["tag.dolby_vision"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}]}}`).Error
	})
}
