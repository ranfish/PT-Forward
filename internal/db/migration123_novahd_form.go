package db

import (
	"gorm.io/gorm"
)

// migration123 §59.440: Nova高清（pt.novahd.top）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（62.3KB 直连）+ docs/32/novahd.md
// 567 行读全（rules.php 完整采集+§4 流水线注意事项）。
// **实时=资产=文档三源全槽一致（零漂移站）**——文档 4-22 Playwright
// 验证与实时完全吻合。
// 槽位映射：
//   - type 13（414 置首乱序）：411 短剧第十六站+406=[mv,music_video,
//     concert] 三键承接（克隆/优堡合并项同款）+404 双键+**412 动漫/
//     413 番剧 label-only**（站方细分语义：412 单季/单集 vs 413 多季
//     合集（§4.3）——发布链无该判定维度，405=[animation] 承接动画
//     电影标准语义，412/413 预览人工）+414=[game]。
//   - medium 12（值 1-12 连续排列乱序）：**Blu-ray(1)/UHD Blu-ray(10)
//     分立各单键**（奥申/躺平分立先例）+Remux(3)=[remux,uhd_remux]
//     双键归并+**WEB-DL(11)=[webdl,webrip] 双键承接**（站方无 WEBRip
//     项）+MiniBD(4)/HD DVD(2)/DVDR(6)/Encode(7)/CD/Track/DVD 全配。
//   - codec 6：合并命名零联动（H264/x264/AVC 三命名一项+H265/HEVC/
//     x265 同——站方合并项语义第六站反证）+VC-1/Xvid/MPEG-2/Other。
//   - **audiocodec 15 全配键**：**TrueHD Atmos(8)=[atmos,
//     truehd_atmos] 双键（躺平模式）+TrueHD(12) 分立**+**DDP/E-AC3(9)=
//     [ddp] 单键（eac3 无键 ddp 承接）+DD/AC3(10)=[ac3,dd] 双键分立**
//     +DTS:X(14)=[dtsx]+DTS-HD MA(13)+**ALAC(7) ext 活键**+FLAC/APE/
//     DTS/MP3/OGG/AAC/LPCM/Other。
//   - standard 8：主链全配（1080p/i/720p/SD/2160p/4320p）+**7 60Fps/
//     8 120Fps label-only**（帧率维度混入分辨率槽——站方 §4.1 规则
//     FrameRate>=60/120 分发；发布链 standard 槽静态映射无帧率判定
//     维度，零案例不预实施（按需生长），5=[r2160p] 承接全部 4K，
//     60/120Fps 种预览人工调）。
//   - team 16 外站官组大表：HDSky/CHD/MySiLU/WiKi/beAst/CMCT/TLF/
//     M-Team/BeiTai/AGSV/HDHome/TTG 全 label-only+**FRDS(6) 在表
//     直中**+**NHDWeb(15)/NDJWEB(16) 站方官组 label-only**+Other(5)=
//     [team.other] 兜底。
//   - **tags 18 罕见标签丰收**：**60FPS(21)=[high_frame_rate]（星陨阁
//     首配键复利——60FPS=高帧率语义）+高码(19)=[high_bitrate]（星陨阁/
//     劳改所复利）+大包(13)=[big_pack]（躺平首配键复利）+杜比(14)=
//     [dolby_vision]（垃圾堆先例）**四长尾键同站配；首发(2)/驻站(8)
//     绝不配（官方专属财神雷区）；DIY/国语/中字/HDR 四键泛家族/英字/
//     特效/完结=[complete]/分集=[ongoing] 十一键配；**10Bit(20)
//     label-only（bit depth 无键）+连载(18) label-only（与分集(9)
//     分立语义模糊——分集=ongoing 历史先例，连载保守不猜）+番组(17)
//     label-only（动画番组语义无键）+应求(12) label-only**。
//   - 字段：type/medium_sel[4]/codec_sel[4]/audiocodec_sel[4]/
//     standard_sel[4]/team_sel[4]/tags[4][]/small_descr/descr/
//     technical_info+url+pt_gen+uplver+anonymous:true；nfo 不配
//     （13城先例域表无此域）。
// 站规人工把关：§6.4 上传规则（H&R+上传者资格）+§6.5 标题命名规范+
//   §6.6 无盒子规则；60/120Fps 高帧率种+10Bit 种预览人工核对
//   （§4.1/§4.2 站方规则）。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(123, "novahd_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'pt.novahd.top' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'pt.novahd.top'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "technical_info": "technical_info", "imdb_url": "url", "pt_gen": "pt_gen", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Game/游戏", "value": "414", "standard_keys": ["category.game"]}, {"label": "Anime Series/番剧", "value": "413"}, {"label": "Anime/动漫", "value": "412"}, {"label": "Short Play/短剧", "value": "411", "standard_keys": ["category.short_drama"]}, {"label": "Movies/电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "TV Series/电视剧", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "TV Shows/综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "Documentaries/记录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "Animations/动画", "value": "405", "standard_keys": ["category.animation"]}, {"label": "MV/演唱会", "value": "406", "standard_keys": ["category.mv", "category.music_video", "category.concert"]}, {"label": "Sports/体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "Music/音乐", "value": "409", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "Othes/其他", "value": "410", "standard_keys": ["category.other"]}], "medium": [{"label": "DVD", "value": "12", "standard_keys": ["medium.dvd"]}, {"label": "WEB-DL", "value": "11", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "UHD Blu-ray", "value": "10", "standard_keys": ["medium.uhd_bluray"]}, {"label": "Track", "value": "9", "standard_keys": ["medium.track"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "DVDR", "value": "6", "standard_keys": ["medium.dvdr"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "MiniBD", "value": "4", "standard_keys": ["medium.minibd"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "HD DVD", "value": "2", "standard_keys": ["medium.hddvd"]}, {"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray"]}], "codec": [{"label": "H264/x264/AVC", "value": "1", "standard_keys": ["video.h264"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "Xvid", "value": "3", "standard_keys": ["video.xvid"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}, {"label": "H265/HEVC/x265", "value": "6", "standard_keys": ["video.h265"]}], "audiocodec": [{"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "DTS", "value": "3", "standard_keys": ["audio.dts"]}, {"label": "MP3", "value": "4", "standard_keys": ["audio.mp3"]}, {"label": "OGG", "value": "5", "standard_keys": ["audio.ogg"]}, {"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "ALAC", "value": "7", "standard_keys": ["audio.alac"]}, {"label": "TrueHD Atmos", "value": "8", "standard_keys": ["audio.atmos", "audio.truehd_atmos"]}, {"label": "DDP/E-AC3", "value": "9", "standard_keys": ["audio.ddp"]}, {"label": "DD/AC3", "value": "10", "standard_keys": ["audio.ac3", "audio.dd"]}, {"label": "LPCM", "value": "11", "standard_keys": ["audio.lpcm"]}, {"label": "TrueHD", "value": "12", "standard_keys": ["audio.truehd"]}, {"label": "DTS-HD MA", "value": "13", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "DTS:X", "value": "14", "standard_keys": ["audio.dtsx"]}, {"label": "Other", "value": "15", "standard_keys": ["audio.other"]}], "standard": [{"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "2160p/4K", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "4320p/8K", "value": "6", "standard_keys": ["resolution.r4320p"]}, {"label": "2160p/4K 60Fps", "value": "7"}, {"label": "2160p/4K 120Fps", "value": "8"}], "team": [{"label": "HDSky", "value": "1"}, {"label": "CHD", "value": "2"}, {"label": "MySiLU", "value": "3"}, {"label": "WiKi", "value": "4"}, {"label": "Other", "value": "5", "standard_keys": ["team.other"]}, {"label": "FRDS", "value": "6"}, {"label": "beAst", "value": "7"}, {"label": "CMCT", "value": "8"}, {"label": "TLF", "value": "9"}, {"label": "M-Team", "value": "10"}, {"label": "BeiTai", "value": "11"}, {"label": "AGSV", "value": "12"}, {"label": "HDHome", "value": "13"}, {"label": "TTG", "value": "14"}, {"label": "NHDWeb", "value": "15"}, {"label": "NDJWEB", "value": "16"}], "tags": [{"label": "首发", "value": "2"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}, {"label": "60FPS", "value": "21", "standard_keys": ["tag.high_frame_rate"]}, {"label": "10Bit", "value": "20"}, {"label": "高码", "value": "19", "standard_keys": ["tag.high_bitrate"]}, {"label": "连载", "value": "18"}, {"label": "番组", "value": "17"}, {"label": "特效", "value": "15", "standard_keys": ["tag.special_effects_subs"]}, {"label": "杜比", "value": "14", "standard_keys": ["tag.dolby_vision"]}, {"label": "大包", "value": "13", "standard_keys": ["tag.big_pack"]}, {"label": "应求", "value": "12"}, {"label": "英字", "value": "11", "standard_keys": ["tag.english_subtitle"]}, {"label": "完结", "value": "10", "standard_keys": ["tag.complete"]}, {"label": "分集", "value": "9", "standard_keys": ["tag.ongoing"]}, {"label": "驻站", "value": "8"}]}}`).Error
	})
}
