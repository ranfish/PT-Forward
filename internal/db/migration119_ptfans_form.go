package db

import (
	"gorm.io/gorm"
)

// migration119 §59.436: PTFans（ptfans.cc）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（48.3KB 直连）+ docs/32/ptfans.md
// 482 行读全（Playwright 3 页采集：rules.php+details+发布页验证）。
// 实时 vs 资产 vs 文档全槽一致（零漂移站）。
// 槽位映射（双模式站 mode=4 综合区/5 9KG 区——§59.372+§59.388 双防线
// 自动护；发布链走综合区；**9KG 成人区 12 分类全局禁止**——成人拦截
// 在前发布天然不可达+文档 §4.4 明文）：
//   - type 17 **教育/知识向大分类站**（Science/School/Book/Code/ACGN/
//     Baby/Resource 罕见细分）+非标值集（404 电视剧/405 综艺/406 纪录
//     片错位）：**407 Music=[music,lossless_music,mv,music_video,
//     concert] 五键合并承接**（音乐、专辑、MV、演唱会一大类——历史
//     三键承接放大）+412 Book=[ebook,audiobook] 双键+414 Animate=
//     [animation]+411 School=[education]+418 Software；408 Art 曲艺/
//     410 Science/413 Code/415 ACGN/416 Baby/417 Resource label-only
//     （dict 无键）。
//   - medium 9：**Encode 7/8 重复值站方配置错误（克隆式去重第五例
//     ——配置只留 HTML 首值 8——§59.394 蟹黄堡口径，formdiff 抓出即修）**+Other(9) 首乱序+Blu-ray=[bluray,uhd_bluray]
//     /Remux=[remux,uhd_remux] 双键归并。
//   - codec 9 **原盘/压制编码双维站（blurayCodecOption 特化挂
//     codecMappingOf）**：Bluray(AVC/HEVC/VC-1) 三项原盘编码 label-only
//     （特化独占——防 h264 键一键多值）+H.264(x264/AVC)/H.265(x265/
//     HEVC) 合并命名零联动+MPEG-2/Xvid/AV1/Other 配键；**动态检测
//     "Bluray(" 前缀非站名硬编码**（原盘族媒介∧有原盘项才分发——
//     §59.407 diyMediumOption 同款通用投资，未来同形态站自动生效）。
//   - standard 6：4K(5)/8K(6) 置尾全配键。
//   - **无 audiocodec 槽第五站**（GTK/克隆/三月/奥申后）。
//   - team 5：HDS/CHD/MySiLU/WiKi 老牌四组 label-only（文档 §1.9
//     "转种基本都选 Other"）+Other=[team.other] 兜底。
//   - tags 7：首发/禁转绝不配（财神雷区）+DoVi=[dolby_vision]+DIY/
//     国语/中字+HDR 四键泛家族（17 站）。
//   - 字段：url+**pt_gen 真字段**+technical_info+**无 uplver**（实时
//     实证——控件全列无匿名字段，anonymous 无承载点）；nfo 不配；
//     qch_bt_rainbow_enable（站点插件）不配。
// 站规人工把关：标题命名规范+鼓励保种+吸血处罚+转载注明来源+9KG 区
// 禁止（全局）+教育类内容健康要求。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(119, "ptfans_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'ptfans.cc' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'ptfans.cc'`, `{"enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "technical_info": "technical_info"}, "framework": "np", "value_mappings": {"type": [{"label": "Movies(电影)", "value": "401", "standard_keys": ["category.movie"]}, {"label": "TV Series(电视剧)", "value": "404", "standard_keys": ["category.tv_series"]}, {"label": "TV Shows(综艺)", "value": "405", "standard_keys": ["category.tv_shows"]}, {"label": "Documentaries(纪录片)", "value": "406", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "Sport(体育、竞技、武术及相关)", "value": "403", "standard_keys": ["category.sports"]}, {"label": "Games(游戏及相关)", "value": "409", "standard_keys": ["category.game"]}, {"label": "Music(音乐、专辑、MV、演唱会)", "value": "407", "standard_keys": ["category.music", "category.lossless_music", "category.mv", "category.music_video", "category.concert"]}, {"label": "Art(曲艺、相声、小品、戏曲、舞蹈、歌剧、评书等)", "value": "408"}, {"label": "Science(科学、知识、技能)", "value": "410"}, {"label": "School(应试、考级、职称、初中以上教育)", "value": "411", "standard_keys": ["category.education"]}, {"label": "Book(书籍、杂志、报刊、有声书)", "value": "412", "standard_keys": ["category.ebook", "category.audiobook"]}, {"label": "Code(IT技术、建模、编程、信息技术、大数据、人工智能）", "value": "413"}, {"label": "Animate(3D动画、2.5次元)", "value": "414", "standard_keys": ["category.animation"]}, {"label": "ACGN(二次元、漫画)", "value": "415"}, {"label": "Baby(婴幼、儿童、早教、小学及相关)", "value": "416"}, {"label": "Resource(素材、数据、图片、文档、模板)", "value": "417"}, {"label": "Software(软件、系统、 程序、APP等)", "value": "418", "standard_keys": ["category.software"]}], "medium": [{"label": "Other", "value": "9", "standard_keys": ["medium.other"]}, {"label": "Encode", "value": "8", "standard_keys": ["medium.encode"]}, {"label": "Blu-ray", "value": "6", "standard_keys": ["medium.bluray", "medium.uhd_bluray"]}, {"label": "Web-DL", "value": "5", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "MiniBD", "value": "4", "standard_keys": ["medium.minibd"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "DVD", "value": "2", "standard_keys": ["medium.dvd", "medium.dvdr"]}, {"label": "CD", "value": "1", "standard_keys": ["medium.cd"]}], "codec": [{"label": "H.264(x264/AVC)", "value": "1", "standard_keys": ["video.h264"]}, {"label": "H.265(x265/HEVC)", "value": "2", "standard_keys": ["video.h265"]}, {"label": "Bluray(VC-1)", "value": "3"}, {"label": "Bluray(AVC)", "value": "4"}, {"label": "Bluray(HEVC)", "value": "5"}, {"label": "MPEG-2", "value": "6", "standard_keys": ["video.mpeg2"]}, {"label": "Xvid", "value": "7", "standard_keys": ["video.xvid"]}, {"label": "AV1", "value": "8", "standard_keys": ["video.av1"]}, {"label": "Other", "value": "9", "standard_keys": ["video.other"]}], "standard": [{"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "4K", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "8K", "value": "6", "standard_keys": ["resolution.r4320p"]}], "team": [{"label": "HDS", "value": "1"}, {"label": "CHD", "value": "2"}, {"label": "MySiLU", "value": "3"}, {"label": "WiKi", "value": "4"}, {"label": "Other", "value": "5", "standard_keys": ["team.other"]}], "tags": [{"label": "DoVi", "value": "8", "standard_keys": ["tag.dolby_vision"]}, {"label": "禁转", "value": "1"}, {"label": "首发", "value": "2"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}]}}`).Error
	})
}
