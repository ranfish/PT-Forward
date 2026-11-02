package db

import (
	"gorm.io/gorm"
)

// migration135 §59.453: 好学（www.hxpt.org）发布表单配置分发——
// **410 纪录片特别通道站**（用户逐点讨论定案）。
// 依据：实时 HTML（82KB 直连）零漂移+tags 古籍(25) 实时新增（23→24）+
// 410 details tid=23917 实态验证（视频/其他/1080p/其他/MKV/Other 七槽
// 实填与用户描述完全一致）。
// 站点形态：**NP 同名槽语义全面错位站**（全库最极端）——
//   audiocodec_sel=年级（幼儿园~高三）/processing_sel=教材出版社版本/
//   source_sel=发布年份/codec_sel=文件格式/medium_sel=内容形态。
// 核心机制（用户定案，讨论终案）：
//   - **站点级拦截**：hxptOnlyDocumentary（碟粉模式——非纪录片
//     ineligible"仅接受纪录片特别通道"+DryRun 同拦）
//   - **hxptDocumentaryPrefill 四槽注入**：年级恒"其他"（audiocodec
//     域键）+版本恒"其他"（直写 processing_sel[4]）+年份三级链
//     （tp.Year 精确 label→早于 2010→"2010年前"→空不填——label 动态
//     匹配站方加新年份自动适配）+格式（MI General 容器 §59.453 透出
//     Matroska→MKV/MPEG-4→MP4+标题词形兜底→一律"其他"）
//   - **type：410=[documentary,documentaries] 双键唯一配键**（其余教育
//     段 401-406 label-only——非纪录片上游已拦）
//   - **medium：全媒介键归一 1 视频**（纪录片天然是视频——多键归一
//     合法；音频/书籍/文档等 9 项 label-only）
//   - **codec/audiocodec/processing/source 四槽零 standard_keys**
//     （jobs 通用链 miss 不填，特化覆盖——label 全录保 formdiff 对齐；
//     processing/source 不进 form_fields：无域常量+htmlDomainRules
//     不含此二域 draft 不收录零差异）
//   - standard：r1080p+i 双键归 3（p/i 合并项——咖啡同款）+sd→6
//     （Other 承接——站方无 SD 以下独立项）
//   - team：[team.other]→10（lookupOtherOption 兜底"选不到就 Other"
//     用户定案）+HX/HXPT/HXWEB 官组 label-only（非组员禁选——官种
//     保护，外站源种组名几乎不可能是 HX 系）
//   - tags 24（实时值集）：**禁转/首发绝不配**（财神雷区）+DIY/国语/
//     中字+**HDR(7) 泛财神四键**+分集=[ongoing]/完结=[complete]+
//     学科系（语文/数学/外语/...信息技术/兴趣爱好）+古籍(25) 实时新增
//     label-only（发布链无学科数据——用户定案按正常 NP 站逻辑消费
//     种子详情标签）
//   - 字段：type/medium_sel[4]/codec_sel[4]/audiocodec_sel[4]/
//     standard_sel[4]/team_sel[4]/tags[4][]/small_descr/descr/
//     technical_info+url（IMDb）+pt_gen+uplver；price（价格——站方
//     付费资源机制）不配+nfo 不配（13城先例）
// 站规人工把关：纪录片特别通道规则（年级/版本恒其它）+官组非组员禁选。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(135, "hxpt_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'www.hxpt.org' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'www.hxpt.org'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "technical_info": "technical_info", "imdb_url": "url", "pt_gen": "pt_gen", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "学前教育", "value": "401"}, {"label": "小学部", "value": "402"}, {"label": "初中部", "value": "403"}, {"label": "高职部", "value": "404"}, {"label": "高中部", "value": "405"}, {"label": "教育", "value": "406"}, {"label": "纪录片", "value": "410", "standard_keys": ["category.documentary", "category.documentaries"]}], "medium": [{"label": "视频", "value": "1", "standard_keys": ["medium.bluray", "medium.uhd_bluray", "medium.hddvd", "medium.remux", "medium.uhd_remux", "medium.encode", "medium.webdl", "medium.webrip", "medium.hdtv", "medium.uhdtv", "medium.dvd", "medium.dvdr", "medium.cd", "medium.track", "medium.minibd", "medium.other"]}, {"label": "音频", "value": "2"}, {"label": "书籍", "value": "3"}, {"label": "文档", "value": "4"}, {"label": "笔记", "value": "5"}, {"label": "课件", "value": "6"}, {"label": "软件", "value": "7"}, {"label": "会议", "value": "8"}, {"label": "图片", "value": "9"}, {"label": "其他", "value": "10"}], "codec": [{"label": "RMVB", "value": "1"}, {"label": "MP4", "value": "2"}, {"label": "JPG", "value": "3"}, {"label": "PDF", "value": "4"}, {"label": "TXT", "value": "5"}, {"label": "DOC", "value": "6"}, {"label": "XLS", "value": "7"}, {"label": "PPT", "value": "8"}, {"label": "WMA", "value": "9"}, {"label": "MP3", "value": "10"}, {"label": "RAR", "value": "11"}, {"label": "EXE", "value": "12"}, {"label": "MKV", "value": "13"}, {"label": "EPUB", "value": "14"}, {"label": "MA4", "value": "15"}, {"label": "AZW3", "value": "16"}, {"label": "DJVU", "value": "17"}, {"label": "CAJ", "value": "18"}, {"label": "MOBI", "value": "19"}, {"label": "TIFF", "value": "20"}, {"label": "PSD", "value": "21"}, {"label": "APE", "value": "22"}, {"label": "FLAC", "value": "23"}, {"label": "ZIP", "value": "24"}, {"label": "UVZ", "value": "25"}, {"label": "其他", "value": "26"}], "audiocodec": [{"label": "幼儿园", "value": "1"}, {"label": "一年级", "value": "2"}, {"label": "二年级", "value": "3"}, {"label": "三年级", "value": "4"}, {"label": "四年级", "value": "5"}, {"label": "五年级", "value": "6"}, {"label": "六年级", "value": "7"}, {"label": "七年级", "value": "8"}, {"label": "八年级", "value": "9"}, {"label": "九年级", "value": "10"}, {"label": "高一", "value": "11"}, {"label": "高二", "value": "12"}, {"label": "高三", "value": "13"}, {"label": "其他", "value": "14"}], "standard": [{"label": "720p/720i", "value": "2", "standard_keys": ["resolution.r720p"]}, {"label": "1080p/1080i", "value": "3", "standard_keys": ["resolution.r1080p", "resolution.r1080i"]}, {"label": "4K/2160p/2160i", "value": "4", "standard_keys": ["resolution.r2160p"]}, {"label": "8K/4320p/4320i", "value": "5", "standard_keys": ["resolution.r4320p"]}, {"label": "Other", "value": "6", "standard_keys": ["resolution.sd"]}], "team": [{"label": "Other", "value": "10", "standard_keys": ["team.other"]}, {"label": "HX", "value": "11"}, {"label": "HXPT", "value": "9"}, {"label": "HXWEB", "value": "8"}], "processing": [{"label": "人教版", "value": "1"}, {"label": "新人教", "value": "26"}, {"label": "人民版", "value": "19"}, {"label": "部编版", "value": "2"}, {"label": "统编版", "value": "28"}, {"label": "苏教版", "value": "3"}, {"label": "鄂教版", "value": "4"}, {"label": "鲁教版", "value": "5"}, {"label": "北师大版", "value": "6"}, {"label": "沪教版", "value": "7"}, {"label": "冀教版", "value": "8"}, {"label": "浙教版", "value": "9"}, {"label": "河大版", "value": "10"}, {"label": "湘教版", "value": "11"}, {"label": "京教版", "value": "12"}, {"label": "外研版", "value": "13"}, {"label": "牛津版", "value": "15"}, {"label": "中图版", "value": "25"}, {"label": "粤沪版", "value": "16"}, {"label": "北师版", "value": "17"}, {"label": "陕教版", "value": "18"}, {"label": "川教版", "value": "21"}, {"label": "上海教育版", "value": "22"}, {"label": "济南版", "value": "23"}, {"label": "北京版", "value": "24"}, {"label": "其他", "value": "27"}], "source": [{"label": "2026", "value": "18"}, {"label": "2025", "value": "17"}, {"label": "2024", "value": "16"}, {"label": "2023", "value": "15"}, {"label": "2022", "value": "14"}, {"label": "2021", "value": "13"}, {"label": "2020", "value": "12"}, {"label": "2019", "value": "11"}, {"label": "2018", "value": "10"}, {"label": "2017", "value": "9"}, {"label": "2016", "value": "8"}, {"label": "2015", "value": "7"}, {"label": "2014", "value": "6"}, {"label": "2013", "value": "5"}, {"label": "2012", "value": "4"}, {"label": "2011", "value": "3"}, {"label": "2010年前", "value": "2"}, {"label": "Other", "value": "1"}], "tags": [{"label": "禁转", "value": "1"}, {"label": "首发", "value": "2"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}, {"label": "语文", "value": "8"}, {"label": "数学", "value": "9"}, {"label": "外语", "value": "10"}, {"label": "历史", "value": "11"}, {"label": "地理", "value": "12"}, {"label": "物理", "value": "22"}, {"label": "化学", "value": "13"}, {"label": "生物", "value": "14"}, {"label": "道德与法治(道法)", "value": "15"}, {"label": "音乐", "value": "16"}, {"label": "美术", "value": "17"}, {"label": "体育", "value": "18"}, {"label": "自然科学", "value": "19"}, {"label": "信息技术", "value": "20"}, {"label": "兴趣爱好", "value": "21"}, {"label": "分集", "value": "23", "standard_keys": ["tag.ongoing"]}, {"label": "完结", "value": "24", "standard_keys": ["tag.complete"]}, {"label": "古籍", "value": "25"}]}}`).Error
	})
}
