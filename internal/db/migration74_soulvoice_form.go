package db

import (
	"gorm.io/gorm"
)

// migration74 §59.388: 聆音（pt.soulvoice.club）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（45.6KB 代理抓取）+ 站内种子实态
// 三证（details 页媒介实态：WEB-DL 种=Other / UHD REMUX 种=Other /
// BluRay x264 压制种=Encode）+ docs/32/soulvoice.md 535 行读全。
// 站点形态：**双模式站**（影视 mode=4 browsecat + 阅听 mode=8
// specialcat 互斥——壹吧 §59.371 同款；sites.json 4 月资产把 mode=8
// 槽混录（category 2 项=阅听 type/audio_codec=电子书格式/
// processing_sel=书籍体裁/source_sel=语言——"后覆盖"解析陷阱 §59.372）。
// 槽位特化：
//   - **媒介粗粒度站方实态对齐**（三证）：压制→Encode(7)；WEB-DL/
//     Remux→Other(12)（站内用户就选 Other——站方无碟系/WEB 项）；
//     webdl/webrip/bluray/uhd_bluray/remux/uhd_remux 七键归 12
//     （多键一值无冲突）；MiniBD(4) 精确键 §59.325 先例；
//     APE/FLAC(10)/DSD(11) 音乐格式 label-only（不发布音乐种）；
//     sites.json 的 Blu-ray(1) 实时 HTML 无此值（站方已删或 4 月
//     错录——实时为准不配）
//   - type 9 项（影视模式）：407 教学→category.education（库非
//     413 同键先例）；404 纪录双键+406 MV 双键即防
//   - codec 仅 3 项（H.264/H.265/Other）无 X26x——§59.380 压制
//     联动自然零触发
//   - standard 仅 4 项：r720p/r1080p 无 720 项 miss 落空可接受
//     （库非 r4320p 同款）；1080I(2) 大写 I label 站方原文
//   - team 5 项极简：**FRDS(3) 在表**——大青虫/库非外站组直中先例
//     可期；全 label-only
//   - tags[4][] 12 项：首发(2) label-only 铁律+禁转/国语/中字/DIY/
//     完结(8)→tag.complete 配键+HDR/杜比/HLG/特效/原创/应求
//     label-only
//   - 有 url/pt_gen/uplver（表单实证齐全——比库非全）
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(74, "soulvoice_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'pt.soulvoice.club' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'pt.soulvoice.club'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "电视剧", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "音乐视频", "value": "406", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "教学", "value": "407", "standard_keys": ["category.education"]}, {"label": "音乐", "value": "408", "standard_keys": ["category.music"]}, {"label": "其它", "value": "409", "standard_keys": ["category.other"]}], "medium": [{"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "MiniBD", "value": "4", "standard_keys": ["medium.minibd"]}, {"label": "APE/FLAC", "value": "10"}, {"label": "DSD", "value": "11"}, {"label": "Other", "value": "12", "standard_keys": ["medium.webdl", "medium.webrip", "medium.bluray", "medium.uhd_bluray", "medium.remux", "medium.uhd_remux", "medium.other"]}], "codec": [{"label": "H.264", "value": "1", "standard_keys": ["video.h264"]}, {"label": "H.265", "value": "2", "standard_keys": ["video.h265"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}], "standard": [{"label": "1080P", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080I", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "2160P", "value": "3", "standard_keys": ["resolution.r2160p"]}, {"label": "Other", "value": "4", "standard_keys": ["resolution.other"]}], "team": [{"label": "HDS", "value": "1"}, {"label": "CHD", "value": "2"}, {"label": "FRDS", "value": "3"}, {"label": "CMCT", "value": "4"}, {"label": "Other", "value": "5"}], "tags": [{"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "首发", "value": "2"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR", "value": "7"}, {"label": "完结", "value": "8", "standard_keys": ["tag.complete"]}, {"label": "特效", "value": "9"}, {"label": "应求", "value": "10"}, {"label": "HLG", "value": "11"}, {"label": "杜比", "value": "12"}, {"label": "原创", "value": "13"}]}}`).Error
	})
}
