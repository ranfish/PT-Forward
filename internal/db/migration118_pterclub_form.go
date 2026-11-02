package db

import (
	"gorm.io/gorm"
)

// migration118 §59.435: 猫（pterclub.net）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（89.7KB 直连）+ docs/32/pterclub.md
// 1488 行读全（Playwright 实采+种子检查脚本逆向分析）。实时 vs 资产
// vs 文档全槽一致（零漂移站）。
// 槽位映射（**极简三下拉站**——全站仅 type/source_sel/team_sel）：
//   - type 13 非标值集（**404=电视剧/403=动画/405=综艺**——与惯例
//     402/403/405 错位+402=纪录片）：401=[movie]/404=[tv_series]/403=
//     [animation]/405=[tv_shows]/402 双键/413/406 双键/**418 舞台演出=
//     [concert]**（演唱会同族单键——我堡 419/熊猫 413 同款）+408
//     电子书/410 软件/411 学习=[education]。
//   - **medium→source_sel 槽位特化第四站**（财神/壹吧/青蛙后——
//     source_sel 15=质量槽：媒介+文档格式混合）：UHD Discs=[uhd_bluray]/
//     BD Discs=[bluray]/Remux=[remux,uhd_remux] 双键归并/WEB-DL=
//     [webdl,webrip]/Encode/DCTV/DVD Discs=[dvd,dvdr]+**FLAC/WAV/ISO/
//     PDF/AZW/MOBI extended 活键全配**（文档格式键六项首配齐——
//     PUB 无键 label-only 唯一）+Other。
//   - **team_sel=字段名陷阱（name=team_sel 实为地区槽**：大陆/香港/
//     台湾/欧美/韩/日/印度/其它 8 区——产区语义历史定案不投递；但
//     字段名被 ParsePublishFormHTML 域启发命中 team 域——formdiff 抓
//     出整域假差异 8 条 added——**修正=team 域映射+8 项 label-only
//     占位**（无 standard_keys 永不命中→零投递行为不变+diff 归零——
//     时光 424 特化独占同款；官组 PTer 系不在表预览人工选区）。
//   - **tags 不配（独立 checkbox 形态首见——非 tags[] 数组**：11 标签
//     各自独立 name（jinzhuan/guoyu/zhongzi 等），发布链 tags 域数组
//     投递机制不适用——全标签预览人工；其中 guanfang 官方 disabled
//     仅管理员（绝不配双保险）+pr 原创/bim 自购/jinzhuan 禁转官方
//     专属本就不配。
//   - 字段：url（IMDb 真字段——text input 无 data-pt-gen）+**douban
//     独立豆瓣字段（dburl 域第五站——南洋/葡萄/好大/海胆/我堡后）**+
//     uplver+anonymous:true；referid（转载引用）/offer/
//     has_allowed_offer（候选 hidden）不配；无 pt_gen/nfo/
//     technical_info（简介粘贴 [hide=MediaInfo] 包裹）。
// 站方检查脚本交叉校验对齐（脚本逆向 §266）：分类判定顺序（纪录片>
//     舞台演出>动画>综艺>电视剧>电影默认）与 canonical 映射同向；
//     质量判定（标题 REMUX→Remux/Blu-ray+x264→Encode/WEB-DL+FRDS→
//     Encode/WEBRip→Encode）与 §59.380 压制联动语义一致（IsEncode
//     铁证联动/原生不联动）——**WEB-DL+FRDS→Encode 特殊规则与联动
//     同构零冲突**。
// 站规人工把关：0DAY 英文命名规范+副标题中文名+Dupe 同组同分辨率禁/
//     死种重发 45 天+6 月+禁 SD 拉伸/CAM/RMVB/无 CUE 整盘/成人。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(118, "pterclub_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'pterclub.net' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'pterclub.net'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "source_sel", "team": "team_sel", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "dburl": "douban", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影 (Movie)", "value": "401", "standard_keys": ["category.movie"]}, {"label": "电视剧 (TV Series)", "value": "404", "standard_keys": ["category.tv_series"]}, {"label": "动画 (Animation)", "value": "403", "standard_keys": ["category.animation"]}, {"label": "综艺 (TV Show)", "value": "405", "standard_keys": ["category.tv_shows"]}, {"label": "音乐短片 (MV)", "value": "413", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "音乐 (Music)", "value": "406", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "舞台演出(Stage Performance)", "value": "418", "standard_keys": ["category.concert"]}, {"label": "纪录片 (Documentary)", "value": "402", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "体育 (Sport)", "value": "407", "standard_keys": ["category.sports"]}, {"label": "电子书 (Ebook)", "value": "408", "standard_keys": ["category.ebook"]}, {"label": "软件 (Software)", "value": "410", "standard_keys": ["category.software"]}, {"label": "学习 (Study)", "value": "411", "standard_keys": ["category.education"]}, {"label": "其它 (Other)", "value": "412", "standard_keys": ["category.other"]}], "medium": [{"label": "UHD Discs", "value": "1", "standard_keys": ["medium.uhd_bluray"]}, {"label": "BD Discs", "value": "2", "standard_keys": ["medium.bluray"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "HDTV", "value": "4", "standard_keys": ["medium.hdtv"]}, {"label": "WEB-DL", "value": "5", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "Encode", "value": "6", "standard_keys": ["medium.encode"]}, {"label": "DVD Discs", "value": "7", "standard_keys": ["medium.dvd", "medium.dvdr"]}, {"label": "FLAC", "value": "8", "standard_keys": ["medium.flac"]}, {"label": "WAV", "value": "9", "standard_keys": ["medium.wav"]}, {"label": "ISO", "value": "10", "standard_keys": ["medium.iso"]}, {"label": "PDF", "value": "11", "standard_keys": ["medium.pdf"]}, {"label": "PUB", "value": "12"}, {"label": "AZW", "value": "13", "standard_keys": ["medium.azw"]}, {"label": "MOBI", "value": "14", "standard_keys": ["medium.mobi"]}, {"label": "Other", "value": "15", "standard_keys": ["medium.other"]}], "team": [{"label": "大陆 (Mainland,CHN)", "value": "1"}, {"label": "香港 (HKG,CHN)", "value": "2"}, {"label": "台湾 (TWN,CHN)", "value": "3"}, {"label": "欧美 (Western)", "value": "4"}, {"label": "韩国 (KOR)", "value": "5"}, {"label": "日本 (JPN)", "value": "6"}, {"label": "印度 (IND)", "value": "7"}, {"label": "其它 (Other)", "value": "8"}]}}`).Error
	})
}
