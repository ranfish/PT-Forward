package db

import (
	"gorm.io/gorm"
)

// migration66 §59.380: 百川（www.hitpt.com）发布表单配置分发。
// 依据 docs/32/hitpt.md（4 月批量分析期 517 行）+ sites.json form 七槽资产。
// 29 实时抓取受阻（CF 严拒：直连/代理/azuretls chrome 指纹全 403——比
// keepfrds 更严，cookie cf_clearance 疑绑 IP）——按 §59.377 五二降级先例：
// 双源资产生成+DryRun 本地组装先行，实时比对留待用户凭证/HTML。
// 槽位语义（§59.371 壹吧同款特化 + 本站特化点）：
//   - 双区互斥（browsecat 种子区/specialcat 特别区 mode=4/7）——我们发布的
//     影视资源全部落种子区；§59.372 双防（special 跳过+seenDomains）已护
//     Diff/解析工具
//   - medium 域 → source_sel[4]（字段名 source 语义媒介——壹吧先例）
//   - 百川无 Encode 项：BDrip(2) 承接 [bdrip, encode]（压制种主归 BD 压制）
//   - codec 区分原盘/压制形态：H.264(1)/H.265(10) 配键，X264/X265 CLI 词
//     label-only（五二同款决策）
//   - audiocodec DTS 家族六级细分全配（dts/dts_hd/dts_hd_hr/dts_hd_ma/dtsx）
//   - tags 首发(2) 存在——label-only 不配键（转存发布禁勾首发铁律）
//   - 分叉族双键即防：413 纪录片 [documentary, documentaries]（§59.376）+
//     415 MV [mv, music_video]（§59.378 第二对）
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(66, "hitpt_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'www.hitpt.com' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'www.hitpt.com'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "source_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "高清电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "高清剧集", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "抢鲜或标清", "value": "403"}, {"label": "动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "纪录片", "value": "413", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "综艺", "value": "416", "standard_keys": ["category.tv_shows"]}, {"label": "Music Video", "value": "415", "standard_keys": ["category.mv", "category.music_video"]}], "medium": [{"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "BDrip", "value": "2", "standard_keys": ["medium.bdrip", "medium.encode"]}, {"label": "DVD", "value": "3", "standard_keys": ["medium.dvdr", "medium.dvd"]}, {"label": "HDTV", "value": "4", "standard_keys": ["medium.hdtv"]}, {"label": "TV", "value": "5"}, {"label": "CD", "value": "7", "standard_keys": ["medium.cd"]}, {"label": "Other", "value": "8", "standard_keys": ["medium.other"]}, {"label": "Web", "value": "9", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "保种资源", "value": "10"}, {"label": "UHD", "value": "11", "standard_keys": ["medium.uhd_bluray"]}, {"label": "Remux", "value": "12", "standard_keys": ["medium.remux", "medium.uhd_remux"]}], "codec": [{"label": "H.264", "value": "1", "standard_keys": ["video.h264"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "Xvid", "value": "3", "standard_keys": ["video.xvid"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}, {"label": "H.265", "value": "10", "standard_keys": ["video.h265"]}, {"label": "VP9", "value": "11", "standard_keys": ["video.vp9"]}, {"label": "MPEG-4", "value": "12", "standard_keys": ["video.mpeg4"]}, {"label": "X264", "value": "13"}, {"label": "X265", "value": "14"}], "audiocodec": [{"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "DTS", "value": "3", "standard_keys": ["audio.dts"]}, {"label": "MP3", "value": "4", "standard_keys": ["audio.mp3"]}, {"label": "OGG", "value": "5", "standard_keys": ["audio.ogg"]}, {"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}, {"label": "AC3", "value": "8", "standard_keys": ["audio.ac3", "audio.dd"]}, {"label": "WAV", "value": "11", "standard_keys": ["audio.wav"]}, {"label": "TrueHD", "value": "12", "standard_keys": ["audio.truehd"]}, {"label": "Atmos", "value": "13", "standard_keys": ["audio.atmos", "audio.truehd_atmos", "audio.ddp_atmos"]}, {"label": "LPCM", "value": "14", "standard_keys": ["audio.lpcm"]}, {"label": "DTS-X", "value": "15", "standard_keys": ["audio.dtsx"]}, {"label": "DTS-HD", "value": "16", "standard_keys": ["audio.dts_hd"]}, {"label": "DTS-HDMR", "value": "17", "standard_keys": ["audio.dts_hd_hr"]}, {"label": "DTS-HDMA", "value": "18", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "DTS-HDMA:X 7.1", "value": "19", "standard_keys": ["audio.dts_hd_ma", "audio.dtsx"]}], "standard": [{"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "4K", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "1440p", "value": "6", "standard_keys": ["resolution.r1440p"]}, {"label": "8K", "value": "7", "standard_keys": ["resolution.r4320p"]}, {"label": "Other", "value": "8", "standard_keys": ["resolution.other"]}], "team": [{"label": "HIT内部资料", "value": "3"}, {"label": "其他", "value": "4"}, {"label": "CMCT", "value": "5"}, {"label": "HDWinG", "value": "6"}, {"label": "CHDBits", "value": "8"}, {"label": "WiKi", "value": "13"}, {"label": "beAst", "value": "14"}, {"label": "MTeam", "value": "16"}, {"label": "百川自制", "value": "17"}, {"label": "HDDolby", "value": "18"}, {"label": "OurBits", "value": "19"}, {"label": "FRDS", "value": "20"}, {"label": "HSPT", "value": "21"}, {"label": "PTer", "value": "22"}], "tags": [{"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "首发", "value": "2"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR", "value": "7"}, {"label": "粤语", "value": "8", "standard_keys": ["tag.cantonese_audio"]}, {"label": "杜比", "value": "9"}]}}`).Error
	})
}
