package db

import (
	"gorm.io/gorm"
)

// migration120 §59.437: 铂金家（pthome.net）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（60.9KB 直连）+ docs/32/pthome.md
// 646 行读全（rules.php+黑名单+H&R+促销+盒子规则完整采集）。
// 实时 vs 资产全槽一致（零漂移站）；文档 §1.1 称无标签——实时有
// **tags[] 23 项字符串值**（文档漂移实时为准）。
// 槽位映射（**双 type 互斥**（browsecat 综合区/specialcat 资料区——
// 同 name 裸名双 select disableother 联动：§59.372 首见+special 跳过
// 双防线自动护；发布链走综合区，507 有声书/508 电子书 miss 落空预览
// 人工——OKPT 同款定案）：
//   - type 11：401/402/403/405/407/410/411/412=[education]（学习）+
//     404/408 双键；**值域仅综合区**（裸名双 select 同名——资产 11 项
//     即综合区全量）。
//   - **medium 12 原盘×DIY 双分（§59.407 diyMediumOption 自动生效
//     第七站复利）**：UHD Blu-ray(12)/UHD Blu-ray/DIY(13)/Blu-ray(1)/
//     Blu-ray/DIY(14) 四分（DIY 2 项 label-only 特化独占——动态检测
//     标题 DIY 词形分发）+REMUX=[remux,uhd_remux] 双键归并+encode(15)
//     **小写 label 站方特有形态**+WEB-DL=[webdl,webrip]+DVD(原盘)=
//     [dvd,dvdr]。
//   - codec 5：H.265(6)/H.264(1)/VC-1/MPEG-2/Other 全配键——无 X26x
//     （压制联动零触发=站方语义）。
//   - audiocodec 12：DTS-HD MA/TrueHD/LPCM/DTS/DD·AC3=[ac3,dd] 双键
//     承接（无 DDP 分立——车站家族式）+**M4A/MP3 extended 活键配**
//     （躺平 m4a 先例+好大 mp3）+FLAC/APE/WAV/AAC/Other；无 Atmos/
//     DTS:X/DTS-HD HR。
//   - standard 7：8K(10)/4K(5) 置首乱序全配+None(11) label-only。
//   - team 8：**PTHome 官组系 7 组 label-only**（PTHome/PTH/PTHweb/
//     PTHtv/PTHAudio/PTHeBook/PTHmusic——官组绝不配键）+Other(5)=
//     [team.other] 兜底（文档："外部组统一选 Other"）。
//   - **tags[] 18 主区字符串值（家园/我堡后第三例——yc/gy/db 等拼音
//     键）**：yc 原创/gz 官字组/jz 禁转/xz 限转/sf 首发**五绝不配**
//     （官方专属财神雷区）+gy=[chinese_audio]/yy=[cantonese_audio]/
//     zz=[chinese_subtitle]/db=[dolby_vision]/hdr10=[hdr10]/hdrm=
//     [hdr10_plus]+diy=[diy] 七键配+yq 应求 label-only+mv/卡拉OK/
//     live/concert/yzj 音乐标签 label-only（dict 无键——第二区 5 项
//     重复同名控件不重复配置）。
//   - 字段：url（IMDb）+**douban_id="豆瓣ID/链接"双形态**（站方接受
//     纯 ID 或链接——配 FieldDomainDoubanID 编号域第三站：家面后，
//     URL 尾部编号提取+纯编号直出 §59.399）+uplver+anonymous:true；
//     no_promotion（不勾促销——站点插件字段）不配；nfo 不配；**无
//     pt_gen/technical_info**（文档 §1.1 实证——综合页）。
//   - **音乐发布页 upload_music.php 独立表单**（music_format_sel 四/
//     music_medium_sel 七/music_publish_sel 八——完全独立结构非标准
//     NP；发布链走综合页，音乐类 type 408 命中综合区——独立页字段
//     不配预览人工）。
// 站规人工把关：**黑名单制作组**（文档 §3.2——FGT/RARBG 系公网组）
//   +H&R 360 天/144h+上传者资格（Power User 及以上+考核）+Dupe 组内
//   同分辨率禁+禁转种转载前获许可+盒子备案+游戏区规则。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(120, "pthome_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'pthome.net' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'pthome.net'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel", "codec": "codec_sel", "audiocodec": "audiocodec_sel", "standard": "standard_sel", "team": "team_sel", "tags": "tags[]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "douban_id": "douban_id", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "电视剧", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "音乐", "value": "408", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "游戏", "value": "410", "standard_keys": ["category.game"]}, {"label": "软件", "value": "411", "standard_keys": ["category.software"]}, {"label": "学习", "value": "412", "standard_keys": ["category.education"]}, {"label": "其他", "value": "409", "standard_keys": ["category.other"]}], "medium": [{"label": "UHD Blu-ray", "value": "12", "standard_keys": ["medium.uhd_bluray"]}, {"label": "UHD Blu-ray/DIY", "value": "13"}, {"label": "Blu-ray(原盘)", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "Blu-ray/DIY", "value": "14"}, {"label": "REMUX", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "encode", "value": "15", "standard_keys": ["medium.encode"]}, {"label": "WEB-DL", "value": "10", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "DVD(原盘)", "value": "2", "standard_keys": ["medium.dvd", "medium.dvdr"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "Track", "value": "9", "standard_keys": ["medium.track"]}, {"label": "Other", "value": "11", "standard_keys": ["medium.other"]}], "codec": [{"label": "H.265(HEVC)", "value": "6", "standard_keys": ["video.h265"]}, {"label": "H.264(AVC)", "value": "1", "standard_keys": ["video.h264"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "DTS-HD MA", "value": "19", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "TrueHD", "value": "20", "standard_keys": ["audio.truehd"]}, {"label": "LPCM", "value": "21", "standard_keys": ["audio.lpcm"]}, {"label": "DTS", "value": "3", "standard_keys": ["audio.dts"]}, {"label": "DD/AC3", "value": "18", "standard_keys": ["audio.ac3", "audio.dd"]}, {"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "WAV", "value": "22", "standard_keys": ["audio.wav"]}, {"label": "MP3", "value": "23", "standard_keys": ["audio.mp3"]}, {"label": "M4A", "value": "24", "standard_keys": ["audio.m4a"]}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}], "standard": [{"label": "8K", "value": "10", "standard_keys": ["resolution.r4320p"]}, {"label": "4K", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "None", "value": "11"}], "team": [{"label": "PTHome", "value": "19"}, {"label": "PTH", "value": "21"}, {"label": "PTHweb", "value": "20"}, {"label": "PTHtv", "value": "22"}, {"label": "PTHAudio", "value": "23"}, {"label": "PTHeBook", "value": "24"}, {"label": "PTHmusic", "value": "25"}, {"label": "Other", "value": "5", "standard_keys": ["team.other"]}], "tags": [{"label": "原创", "value": "yc"}, {"label": "国语", "value": "gy", "standard_keys": ["tag.chinese_audio"]}, {"label": "粤语", "value": "yy", "standard_keys": ["tag.cantonese_audio"]}, {"label": "官字组", "value": "gz"}, {"label": "中字", "value": "zz", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "DOLBY VISION", "value": "db", "standard_keys": ["tag.dolby_vision"]}, {"label": "HDR10", "value": "hdr10", "standard_keys": ["tag.hdr10"]}, {"label": "HDR10+", "value": "hdrm", "standard_keys": ["tag.hdr10_plus"]}, {"label": "禁转", "value": "jz"}, {"label": "限转", "value": "xz"}, {"label": "DIY", "value": "diy", "standard_keys": ["tag.diy"]}, {"label": "首发", "value": "sf"}, {"label": "应求", "value": "yq"}, {"label": "MV", "value": "mv"}, {"label": "卡拉OK", "value": "ok"}, {"label": "现场live", "value": "live"}, {"label": "演唱会", "value": "concert"}, {"label": "音乐专辑", "value": "yzj"}]}}`).Error
	})
}
