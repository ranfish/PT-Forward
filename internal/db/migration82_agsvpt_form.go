package db

import (
	"gorm.io/gorm"
)

// migration82 §59.396: 末日（www.agsvpt.com）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（61.4KB 代理抓取）+ docs/32/agsv.md
// 1253 行读全（含种审脚本黑名单 27 组逆向）。
// **sites.json 4 月资产严重过期**（全槽值集全变——type 值集
// 410 Cartoon/411 Ebook/412 Study 旧→405 Anime/406 MV/419 Playlet 新、
// medium 1-10 旧→11/1/3/7/10/13 新、codec AV1=5→12、audio/standard/
// team/tags 值集全变——文档（后期抓取）与实时一致——**双源教训
// 最重案例：4 月资产完全不可用，文档+实时为源**）。
// **双区域互斥站**（browsecat mode=4 综合区 12 项+specialcat mode=5
// 学习区 4 项——[5] 槽 2 个由 §59.388 模式互斥过滤自动跳过 ✓）。
// 槽位特化：
//   - type 综合区 12 项：419 短剧=short_drama（第五站）+406 MV(演唱)
//     =双键+415 E-Book=ebook+408 Audio=category.audio；**无 Misc**
//     （category.other miss 落空——学习区 4 项不配）；422 纪录双键
//   - medium 10 项**全 canonical**（第二个零 label-only 媒介站——
//     大青虫后）：UHD(11)/Blu-ray(1)/Remux(3) 归并/Encode(7)/
//     WEB-DL(10) 承接 webrip/HDTV(5)/DVD(2) 归并/CD(8)/Track(12)/
//     Other(13)
//   - codec 6 项无 X26x 联动零触发；AV1=12（值与 4 月资产不同）
//   - audiocodec 16 项：DD/AC3(11) 与 DDP(19) 分立+DTS-HD MA(8)/
//     DTS:X(18)/TrueHD Atmos(17) 独立+M4A/ALAC 配键
//   - standard 6 项乱序（4=480p/3=720p/1=1080p——p/i 合并后缀形态
//     齐全：480p/480i、720p/720i、1080p/1080i、4K/2160p/2160i、
//     8K/4320p/4320i）；480p 配 resolution.sd（SD 级语义——
//     无 480p 独立键）
//   - team 11 项全 label-only（AGSVPT/AGSVWEB/AGSVMUS 官组+
//     GodDramas/RL/BeiTai/DYZ/Hares/CatEDU/Pack——FRDS 不在表
//     Other 兜底）；审种脚本黑名单 27 组发布预览人工规避
//   - tags 20 项：首发 label-only 铁律+禁转/国语/粤语/中字/DIY/
//     完结(19) 配键；美剧(31)/韩剧(26)/英字(20)/分集(48)/应求(18)/
//     大包(22)/特效(47)/超分(49)/补帧(50)/HDR(7)/杜比(13)/合并(51)/
//     ⛵方舟(52) label-only
//   - 字段：url（IMDb 真字段——url×2 第二个是编辑器按钮）+pt_gen+
//     technical_info+uplver
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(82, "agsvpt_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'www.agsvpt.com' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'www.agsvpt.com'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Movie(电影)", "value": "401", "standard_keys": ["category.movie"]}, {"label": "TV Series(电视剧)", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "TV Shows(综艺)", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "Anime(动漫)", "value": "405", "standard_keys": ["category.animation"]}, {"label": "Documentaries(纪录片)", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "Playlet（短剧）", "value": "419", "standard_keys": ["category.short_drama"]}, {"label": "Sports(体育)", "value": "407", "standard_keys": ["category.sports"]}, {"label": "MV(演唱)", "value": "406", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "Music(音乐)", "value": "411", "standard_keys": ["category.music"]}, {"label": "Audio(音频)", "value": "408", "standard_keys": ["category.audio"]}, {"label": "Game(游戏)", "value": "413", "standard_keys": ["category.game"]}, {"label": "E-Book(电子书/有声书)", "value": "415", "standard_keys": ["category.ebook"]}], "medium": [{"label": "UHD Blu-ray", "value": "11", "standard_keys": ["medium.uhd_bluray"]}, {"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "WEB-DL", "value": "10", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "DVD", "value": "2", "standard_keys": ["medium.dvdr", "medium.dvd"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "Track", "value": "12", "standard_keys": ["medium.track"]}, {"label": "Other", "value": "13", "standard_keys": ["medium.other"]}], "codec": [{"label": "H.264/AVC", "value": "1", "standard_keys": ["video.h264"]}, {"label": "H.265/HEVC", "value": "6", "standard_keys": ["video.h265"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "AV1", "value": "12", "standard_keys": ["video.av1"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "MP3", "value": "4", "standard_keys": ["audio.mp3"]}, {"label": "WAV", "value": "15", "standard_keys": ["audio.wav"]}, {"label": "M4A", "value": "16", "standard_keys": ["audio.m4a"]}, {"label": "DTS", "value": "3", "standard_keys": ["audio.dts"]}, {"label": "DTS-HD MA", "value": "8", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "DTS:X", "value": "18", "standard_keys": ["audio.dtsx"]}, {"label": "TrueHD", "value": "9", "standard_keys": ["audio.truehd"]}, {"label": "LPCM", "value": "10", "standard_keys": ["audio.lpcm"]}, {"label": "DD/AC3", "value": "11", "standard_keys": ["audio.ac3", "audio.dd"]}, {"label": "DDP/E-AC3", "value": "19", "standard_keys": ["audio.ddp"]}, {"label": "TrueHD Atmos", "value": "17", "standard_keys": ["audio.atmos", "audio.truehd_atmos"]}, {"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "ALAC", "value": "20", "standard_keys": ["audio.alac"]}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}], "standard": [{"label": "480p/480i", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "720p/720i", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "1080p/1080i", "value": "1", "standard_keys": ["resolution.r1080p", "resolution.r1080i"]}, {"label": "4K/2160p/2160i", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "8K/4320p/4320i", "value": "6", "standard_keys": ["resolution.r4320p"]}, {"label": "Other", "value": "8", "standard_keys": ["resolution.other"]}], "team": [{"label": "AGSVPT", "value": "6"}, {"label": "AGSVWEB", "value": "21"}, {"label": "AGSVMUS", "value": "20"}, {"label": "GodDramas", "value": "23"}, {"label": "RL", "value": "31"}, {"label": "BeiTai", "value": "30"}, {"label": "DYZ", "value": "28"}, {"label": "Hares", "value": "29"}, {"label": "CatEDU", "value": "24"}, {"label": "Pack", "value": "16"}, {"label": "Other", "value": "22"}], "tags": [{"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "首发", "value": "2"}, {"label": "粤语", "value": "21", "standard_keys": ["tag.cantonese_audio"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "美剧", "value": "31"}, {"label": "韩剧", "value": "26"}, {"label": "英字", "value": "20"}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "分集", "value": "48"}, {"label": "完结", "value": "19", "standard_keys": ["tag.complete"]}, {"label": "应求", "value": "18"}, {"label": "大包", "value": "22"}, {"label": "特效", "value": "47"}, {"label": "超分", "value": "49"}, {"label": "补帧", "value": "50"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "HDR", "value": "7"}, {"label": "杜比", "value": "13"}, {"label": "合并", "value": "51"}, {"label": "⛵方舟", "value": "52"}]}}`).Error
	})
}
