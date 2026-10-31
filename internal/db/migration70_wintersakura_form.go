package db

import (
	"gorm.io/gorm"
)

// migration70 §59.384: 冬樱（wintersakura.net）发布表单配置分发。
// 依据 docs/32/wintersakura.md（699 行读全——最复杂站点之一）+ sites.json form。
// 槽位特化：
//   - **分集/合集双分类**（§9.1 全库唯一）：剧集 402 分集/414 合集、综艺
//     403 合集/418 分集、动漫剧集 413 分集/423 合集+422 动漫电影独立。
//     form_config 单键单值限制——主配**合集/电影形态**（我们发布的剧集
//     基本整季打包 v1.05 S01 语义+站方"单季打包后禁发单集"）：tv_series
//     双条目 414 在前/tv_shows 403 在前/animation 三条目 422 电影→423
//     剧集合集→413。分集场景预览手调；SeasonEpisode 联动挂账（零案例
//     不预实施 §59.208 纪律）
//   - **编码标准名 vs 编码器名**（§9.2=百川 §5.2 同款）：H264/AVC(6) vs
//     x264(8)/H265/HEVC(9) vs x265(7)——§59.380 压制联动**第三站受益**
//     （lookupCLIVariant 小写匹配已就绪——压制种自动 x265(7) 站方语义）
//   - medium 14 项值从 10 起：DIY 特化项（11/13）label-only（不含中字
//     的 DIY 禁发布 §9.3）；WEB-DL(21) 与 Webrip(25) **独立选项精确**
//     （webdl/webrip 各自单配无需并键）；SACD(22)/3D Blu-ray(24) 独有
//   - audiocodec 21 项（全库最多）精确细分：DDP(25)/DDP Atmos(26)/
//     TrueHD Atmos(10) 三态独立；audio.atmos→10 优先
//   - **无 uplver**（§9.9 不配匿名字段）
//   - team 19 项：5 官组（WS/Sakura SUB/WScode/Sakura WEB/Sakura
//     Academic）与外站组全 label-only——**非本站组禁用 WS 标识**（§9.5
//     官组保护——label-only 防自动勾触雷同财神官种案）
//   - tags 22 项：首发+官组标签（SakuraSUB/SakuraWEB/SakuraAcademic/
//     WScode）+HDR 三级+排行榜 label-only；禁转/国语/中字/粤语/DIY 配键
//   - 分叉族双键即防：410 纪录片+406 MV
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(70, "wintersakura_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'wintersakura.net' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'wintersakura.net'`, `{"enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "technical_info": "technical_info"}, "framework": "np", "value_mappings": {"type": [{"label": "Movies/电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "TV Series/剧集(合集)", "value": "414", "standard_keys": ["category.tv_series"]}, {"label": "TV Series/剧集(分集)", "value": "402"}, {"label": "TV Shows/综艺(合集)", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "TV Shows/综艺(分集)", "value": "418"}, {"label": "Music Videos/音乐MV", "value": "406", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "Sports/体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "HQ Audio/无损音乐", "value": "408", "standard_keys": ["category.music"]}, {"label": "Misc/其他", "value": "409", "standard_keys": ["category.other"]}, {"label": "Documentaries/纪录片", "value": "410", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "Animation films/动漫电影", "value": "422", "standard_keys": ["category.animation"]}, {"label": "Animation series/动漫剧集(合集)", "value": "423"}, {"label": "Animation series/动漫剧集(分集)", "value": "413"}], "medium": [{"label": "UHD Blu-ray", "value": "10", "standard_keys": ["medium.uhd_bluray"]}, {"label": "UHD Blu-ray/DIY", "value": "11"}, {"label": "Blu-ray", "value": "12", "standard_keys": ["medium.bluray"]}, {"label": "Blu-ray/DIY", "value": "13"}, {"label": "Remux", "value": "14", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "Encode", "value": "15", "standard_keys": ["medium.encode"]}, {"label": "HDTV", "value": "16", "standard_keys": ["medium.hdtv"]}, {"label": "DVDR", "value": "17", "standard_keys": ["medium.dvdr", "medium.dvd"]}, {"label": "CD", "value": "18", "standard_keys": ["medium.cd"]}, {"label": "WEB-DL", "value": "21", "standard_keys": ["medium.webdl"]}, {"label": "SACD", "value": "22", "standard_keys": ["medium.sacd"]}, {"label": "HD DVD", "value": "23", "standard_keys": ["medium.hddvd"]}, {"label": "3D Blu-ray", "value": "24", "standard_keys": ["medium.bluray_3d"]}, {"label": "Webrip", "value": "25", "standard_keys": ["medium.webrip"]}], "codec": [{"label": "H264/AVC", "value": "6", "standard_keys": ["video.h264"]}, {"label": "x265", "value": "7"}, {"label": "x264", "value": "8"}, {"label": "H265/HEVC", "value": "9", "standard_keys": ["video.h265"]}, {"label": "VC-1", "value": "10", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2", "value": "11", "standard_keys": ["video.mpeg2"]}, {"label": "Other", "value": "13", "standard_keys": ["video.other"]}, {"label": "ProRes", "value": "15"}, {"label": "AV1", "value": "16", "standard_keys": ["video.av1"]}], "audiocodec": [{"label": "DTS-HDMA", "value": "8", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "DTS-HDMA:X 7.1", "value": "9", "standard_keys": ["audio.dtsx"]}, {"label": "TrueHD Atmos", "value": "10", "standard_keys": ["audio.atmos", "audio.truehd_atmos"]}, {"label": "PCM", "value": "11", "standard_keys": ["audio.pcm"]}, {"label": "TrueHD", "value": "12", "standard_keys": ["audio.truehd"]}, {"label": "DTS", "value": "13", "standard_keys": ["audio.dts"]}, {"label": "LPCM", "value": "14", "standard_keys": ["audio.lpcm"]}, {"label": "FLAC", "value": "15", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "16", "standard_keys": ["audio.ape"]}, {"label": "MP3", "value": "17", "standard_keys": ["audio.mp3"]}, {"label": "OGG", "value": "18", "standard_keys": ["audio.ogg"]}, {"label": "AAC", "value": "19", "standard_keys": ["audio.aac"]}, {"label": "AC3/DD", "value": "20", "standard_keys": ["audio.ac3", "audio.dd"]}, {"label": "Other", "value": "21", "standard_keys": ["audio.other"]}, {"label": "DTS-HD HR", "value": "22", "standard_keys": ["audio.dts_hd_hr"]}, {"label": "WAV", "value": "23", "standard_keys": ["audio.wav"]}, {"label": "DSD", "value": "24", "standard_keys": ["audio.dsd"]}, {"label": "DDP", "value": "25", "standard_keys": ["audio.ddp"]}, {"label": "DDP Atmos", "value": "26", "standard_keys": ["audio.ddp_atmos"]}, {"label": "Opus", "value": "27", "standard_keys": ["audio.opus"]}], "standard": [{"label": "2K/1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "4K/2160p", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "8K/4320p", "value": "6", "standard_keys": ["resolution.r4320p"]}], "team": [{"label": "HDS", "value": "1"}, {"label": "CHD", "value": "2"}, {"label": "Other/其他制作组或转发资源", "value": "5"}, {"label": "CMCT", "value": "6"}, {"label": "FRDS", "value": "7"}, {"label": "PTer", "value": "8"}, {"label": "BHD", "value": "9"}, {"label": "WS/冬樱原盘DIY小组", "value": "11"}, {"label": "Sakura SUB/冬樱字幕组", "value": "12"}, {"label": "Mteam", "value": "13"}, {"label": "HDC", "value": "14"}, {"label": "ttg", "value": "15"}, {"label": "WScode/冬樱重编码及Remux组", "value": "16"}, {"label": "tjupt", "value": "17"}, {"label": "Sakura WEB/冬樱web制作组", "value": "18"}, {"label": "Sakura Academic", "value": "19"}], "tags": [{"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "首发", "value": "2"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR10", "value": "7"}, {"label": "原创字幕", "value": "8"}, {"label": "特效字幕", "value": "9"}, {"label": "HDR10+", "value": "10"}, {"label": "HDR+杜比视界", "value": "11"}, {"label": "粤语", "value": "12", "standard_keys": ["tag.cantonese_audio"]}, {"label": "SakuraSUB", "value": "13"}, {"label": "SakuraWEB", "value": "14"}, {"label": "SakuraAcademic", "value": "15"}, {"label": "独占", "value": "16"}, {"label": "豆瓣电影 Top 250", "value": "17"}, {"label": "IMDb Top 250", "value": "18"}, {"label": "标准收藏版", "value": "19"}, {"label": "WScode", "value": "20"}, {"label": "子供向", "value": "21"}, {"label": "原生原盘", "value": "22"}]}}`).Error
	})
}
