package db

import (
	"gorm.io/gorm"
)

// migration81 §59.395: 织梦（zmpt.cc）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（57.7KB 代理抓取——29 凭证走
// use_global_proxy）+ docs/32/zmpt.md 1447 行读全（含审核脚本完整
// 逆向分析——站方审种脚本 2608 行 specialTags/五层递进 HDR 检测等）。
// 实时 HTML 与 sites.json 4 月资产全槽零漂移（第六个零漂移站）。
// **文档漂移一处**：文档 §246 称有 url（IMDb）字段——实时控件
// **无 url**（IMDb 走 pt_gen/简介——§59.384 附双源教训）。
// 槽位特化：
//   - type 11 项：402 电视剧需完结/分集标签（站方结构校验——审核
//     脚本检测）；422 纪录双键+427 短剧=short_drama（第四站）；
//     无 MV 分类
//   - medium 10 项乱序+含"其他资料"(12)（≈medium.other 配键）；
//     Remux 归并 uhd_remux+WEB-DL 承接 webrip+MiniBD 精确键
//   - standard 6 项**乱序**（480p=7/720p=8/1080p/i=1/2K=6/
//     4K=5/8K=9——非标准排序文档权威警告）；1080p/i 合并双键；
//     480p 无 canonical 键 miss 落空（SD 语义站方无选项）
//   - audiocodec 10 项**音乐向**（无 DDP/TrueHD/DTS-HD 细分——
//     AC3=8 承接 [ac3,dd,ddp] 三键（大青虫/南洋同款）；ALAC 配键
//   - team 8 项全 label-only（ZmPT/ZmWeb/ZmMusic/ZmAudio 官组
//     +DYZ-Movie/GodDramas/RL/RL4B 保护——FRDS 不在表 Other 兜底）
//   - tags 16 项：首发 label-only 铁律+禁转/国语/粤语/中字/DIY/
//     完结配键；电影禁含完结/分集（站方规则——发布链 tag 推断
//     完结是 tag.complete 全类型注入？查 tv/电影分类语义——保守
//     交预览人工）；短剧/汉化/原创/分集/无损/杜比/HDR/驻站/
//     纯享版 label-only
//   - custom_fields[4][1..3]=歌手/专辑/年份（音乐类专用——发布链
//     无此域不配；文档只载 [1] 歌手——[2][3] 实时新增）
//   - 有 pt_gen（站方自建 PTGen 端点）+uplver；无 url/technical_info
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(81, "zmpt_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'zmpt.cc' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'zmpt.cc'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "standard": "standard_sel[4]", "audiocodec": "audiocodec_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "pt_gen": "pt_gen", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "其他 / Misc", "value": "409", "standard_keys": ["category.other"]}, {"label": "纪录片 / documentary", "value": "422", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "动漫 / Anime", "value": "417", "standard_keys": ["category.animation"]}, {"label": "短剧 / Short Play", "value": "427", "standard_keys": ["category.short_drama"]}, {"label": "电影 / Movies", "value": "401", "standard_keys": ["category.movie"]}, {"label": "电视剧 / TV Series", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "综艺 / TV Shows", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "音乐 / Music", "value": "423", "standard_keys": ["category.music"]}, {"label": "有声书 / Audiobook", "value": "424", "standard_keys": ["category.audiobook"]}, {"label": "软件 / Software", "value": "425", "standard_keys": ["category.software"]}, {"label": "游戏 / Game", "value": "426", "standard_keys": ["category.game"]}], "medium": [{"label": "其他资料", "value": "12", "standard_keys": ["medium.other"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "DVDR", "value": "6", "standard_keys": ["medium.dvdr", "medium.dvd"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "MiniBD", "value": "4", "standard_keys": ["medium.minibd"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "HD DVD", "value": "2", "standard_keys": ["medium.hddvd"]}, {"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray", "medium.uhd_bluray"]}, {"label": "WEB-DL", "value": "10", "standard_keys": ["medium.webdl", "medium.webrip"]}], "audiocodec": [{"label": "Other", "value": "7", "standard_keys": ["audio.other"]}, {"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "DTS", "value": "3", "standard_keys": ["audio.dts"]}, {"label": "OGG", "value": "5", "standard_keys": ["audio.ogg"]}, {"label": "AC3", "value": "8", "standard_keys": ["audio.ac3", "audio.dd", "audio.ddp"]}, {"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "MP3", "value": "4", "standard_keys": ["audio.mp3"]}, {"label": "ALAC", "value": "9", "standard_keys": ["audio.alac"]}, {"label": "WAV", "value": "10", "standard_keys": ["audio.wav"]}], "standard": [{"label": "480p", "value": "7"}, {"label": "720p", "value": "8", "standard_keys": ["resolution.r720p"]}, {"label": "1080p/1080i", "value": "1", "standard_keys": ["resolution.r1080p", "resolution.r1080i"]}, {"label": "2K/1440p", "value": "6", "standard_keys": ["resolution.r1440p"]}, {"label": "4K/2160p", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "8K/4320p", "value": "9", "standard_keys": ["resolution.r4320p"]}], "team": [{"label": "Other", "value": "5"}, {"label": "ZmWeb", "value": "7"}, {"label": "ZmPT", "value": "6"}, {"label": "ZmMusic", "value": "8"}, {"label": "ZmAudio", "value": "11"}, {"label": "DYZ-Movie", "value": "9"}, {"label": "GodDramas", "value": "10"}, {"label": "RL/RL4B", "value": "12"}], "tags": [{"label": "驻站", "value": "17"}, {"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "粤语", "value": "15", "standard_keys": ["tag.cantonese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "杜比", "value": "14"}, {"label": "HDR", "value": "7"}, {"label": "无损", "value": "16"}, {"label": "首发", "value": "2"}, {"label": "短剧", "value": "13"}, {"label": "汉化", "value": "11"}, {"label": "原创", "value": "9"}, {"label": "完结", "value": "12", "standard_keys": ["tag.complete"]}, {"label": "分集", "value": "8"}, {"label": "纯享版", "value": "18"}]}}`).Error
	})
}
