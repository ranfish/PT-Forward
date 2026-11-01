package db

import (
	"gorm.io/gorm"
)

// migration108 §59.423: 天空（hdsky.me）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（58.1KB 代理抓取）+ docs/32/hdsky.md
// 509 行读全（站规/官组体系/Dupe 规则/上传者资格完整）。
// 实时 vs 文档 vs 4 月资产全槽一致（文档两处计数错勘误：type"17 个"
// 实 16 项、标签"31 个"实 26 项=最大 ID 非 count）；实时实证：url 字段
// 存在（IMDb text）+url_douban 独立豆瓣字段+uplver；**实时无 nfo**
// （文档漂移）+无 pt_gen/technical_info；**标签字段名 option_sel[]**
// （全库第三个非标准名——tags[4][]/span[]/tag_list[] 后）。
// 槽位映射：
//   - type 16 项：**分集/合集×国产/海外双维路由**（typeMappingOf 第五
//     站 hdskyCategoryValue——用户定案恒合集+产区路由）：402/412/414
//     分集项+411/413 剧集合集 label-only（必须经产区路由——陆港澳台
//     →411 国产/其余→413 海外/无产区默认海外）；403 综艺=[tv_shows]
//     （通用链兜底+特化海外改 415）；404 纪录双键+406 MV 双键+408
//     双键+416 短剧第十三站+401/405/407/409 全配；410 iPad label-only
//     （HDSPad 特供形态不匹配）。
//   - medium 16 项：**UHD/BD 各自原盘/DIY 四分**（13/14/1/12——
//     §59.407 DIY 联动自动生效：DIY 项 label-only 动态选）+Remux
//     双键归并+MiniBD 精确键+SACD=[sacd]/HD DVD=[hddvd]/3D Blu-ray=
//     [3d_bluray]（农场先例 extended 键形）+WEB-DL 承接 webrip。
//   - codec 12 项 H/X 分立：**§59.380 压制联动自动生效**（x264/x265
//     label-only 联动通道——压制种 X264/X265/原盘种 H.264/HEVC 完美
//     分发）+MVC/ProRes label-only+AV1/VP9。
//   - standard 6 项 p/i 分立：4K/2K(1080p)/1080i/720p/SD/8K。
//   - audiocodec 21 项：TrueHD Atmos=[atmos,truehd_atmos] 双键+
//     **DDP with Dolby Atmos=[ddp_atmos]（红豆饭先例）**+HDMA:X 7.1=
//     [dtsx]（冬樱先例）+DTS 四级细分+PCM=[pcm]/LPCM=[lpcm] 共存
//     （冬樱先例无冲突）+OGG/DSD/ALAC/Opus+DDP 分立+AC3/DD 双键。
//   - team 17 项 label-only：HDS 系官组（HDSky/HDS/HDS3D/HDSTV/
//     HDSWEB×3/HDSPad/HDSCD/HDSpecial/HDSAB）+Original/Autoseed/
//     Request/BMDru/AREA11+Other(27) 兜底（FRDS 不在表）。
//   - **标签 option_sel[] 26 项配键 10**：禁转=restricted+国语+
//     粤语=cantonese_audio+中字+**HDR 细分站（优堡式五族——HDR10/
//     HDR10+/DV/HLG 独立配键+泛家族归一不适用）**+DIY+原生原盘=
//     original_disc+**特效=[special_effects_subs]（用户定案——PT 圈
//     语义惯例=特效字幕简写）**；首发/官组（雷区）/自制/应求/限转/
//     SL-HDR1/DoVi+HDR/Atmos（tag 域死键——红豆饭 extended 零产出
//     验证）/DTS-X/次世代国语/全景声国语/临境声国语/DIY纯净版/无广告/
//     去头尾广告纯净版 label-only。
//   - 字段：url（IMDb）+url_douban（FieldDomainDoubanURL——完整链接
//     形态）+uplver（anonymous:true §59.422 附二全系统默认开启定案）；
//     nfo 不配（实时无）；无 pt_gen/technical_info。
// 站规发布预览人工把关：**禁转发 REMUX 电影/剧集**（站方明文——
// REMUX 种发天空必被拒）+官组 Dupe 特权（已有官组 HDS/HDSPad 资源
// 不允许再上传其他小组重编码/iPad 资源）+剧集分集仅官方发布员+
// <100MB 禁+RAR 禁+RMVB 禁+upscale 禁+游戏仅上传员+上传者资格
// （终身影帝/65 周 VIP 直发否则候选区——29 账号资格挂账首颗验证）。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(108, "hdsky_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'hdsky.me' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'hdsky.me'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel", "codec": "codec_sel", "audiocodec": "audiocodec_sel", "standard": "standard_sel", "team": "team_sel", "tags": "option_sel[]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "douban_url": "url_douban", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Movies/电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "Documentaries/纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "iPad/iPad影视", "value": "410"}, {"label": "Animations/动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "TV Series/剧集(分集）", "value": "402"}, {"label": "TV Series/剧集(合集）", "value": "411"}, {"label": "TV Shows/综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "Music Videos/音乐MV", "value": "406", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "Sports/体育", "value": "407", "standard_keys": ["category.sports"]}, {"label": "HQ Audio/无损音乐", "value": "408", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "Misc/其他", "value": "409", "standard_keys": ["category.other"]}, {"label": "TV Series/海外剧集(分集）", "value": "412"}, {"label": "TV Series/海外剧集(合集）", "value": "413"}, {"label": "TV Shows/海外综艺(分集）", "value": "414"}, {"label": "TV Shows/海外综艺(合集）", "value": "415"}, {"label": "Shortplay/短剧", "value": "416", "standard_keys": ["category.short_drama"]}], "medium": [{"label": "UHD Blu-ray", "value": "13", "standard_keys": ["medium.uhd_bluray"]}, {"label": "UHD Blu-ray/DIY", "value": "14"}, {"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "Blu-ray/DIY", "value": "12"}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "DVDR", "value": "6", "standard_keys": ["medium.dvdr", "medium.dvd"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "MiniBD", "value": "4", "standard_keys": ["medium.minibd"]}, {"label": "Track", "value": "9", "standard_keys": ["medium.track"]}, {"label": "WEB-DL", "value": "11", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "SACD", "value": "15", "standard_keys": ["medium.sacd"]}, {"label": "HD DVD", "value": "2", "standard_keys": ["medium.hddvd"]}, {"label": "3D Blu-ray", "value": "16", "standard_keys": ["medium.3d_bluray"]}], "codec": [{"label": "H.264/AVC", "value": "1", "standard_keys": ["video.h264"]}, {"label": "x265", "value": "13"}, {"label": "x264", "value": "10"}, {"label": "HEVC", "value": "12", "standard_keys": ["video.h265"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "Xvid", "value": "3", "standard_keys": ["video.xvid"]}, {"label": "Other", "value": "11", "standard_keys": ["video.other"]}, {"label": "MVC", "value": "14"}, {"label": "ProRes", "value": "15"}, {"label": "VP9", "value": "17", "standard_keys": ["video.vp9"]}, {"label": "AV1", "value": "16", "standard_keys": ["video.av1"]}], "audiocodec": [{"label": "DTS-HDMA", "value": "10", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "DTS-HDMA:X 7.1", "value": "16", "standard_keys": ["audio.dtsx"]}, {"label": "TrueHD Atmos", "value": "17", "standard_keys": ["audio.atmos", "audio.truehd_atmos"]}, {"label": "PCM", "value": "19", "standard_keys": ["audio.pcm"]}, {"label": "TrueHD", "value": "11", "standard_keys": ["audio.truehd"]}, {"label": "DTS", "value": "3", "standard_keys": ["audio.dts"]}, {"label": "LPCM", "value": "13", "standard_keys": ["audio.lpcm"]}, {"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "MP3", "value": "4", "standard_keys": ["audio.mp3"]}, {"label": "OGG", "value": "5", "standard_keys": ["audio.ogg"]}, {"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "AC3/DD", "value": "12", "standard_keys": ["audio.ac3", "audio.dd"]}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}, {"label": "DTS-HD HR", "value": "14", "standard_keys": ["audio.dts_hd_hr"]}, {"label": "WAV", "value": "15", "standard_keys": ["audio.wav"]}, {"label": "DSD", "value": "18", "standard_keys": ["audio.dsd"]}, {"label": "Opus", "value": "22", "standard_keys": ["audio.opus"]}, {"label": "E-AC3", "value": "20", "standard_keys": ["audio.ddp"]}, {"label": "DDP with Dolby Atmos", "value": "21", "standard_keys": ["audio.ddp_atmos"]}, {"label": "ALAC", "value": "23", "standard_keys": ["audio.alac"]}], "standard": [{"label": "4K/2160p", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "2K/1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "8K/4320P", "value": "6", "standard_keys": ["resolution.r4320p"]}], "team": [{"label": "HDSky/原盘DIY小组", "value": "6"}, {"label": "HDS/重编码及remux小组", "value": "1"}, {"label": "HDS3D/3D重编码小组", "value": "28"}, {"label": "HDSTV/电视录制小组", "value": "9"}, {"label": "HDSWEB/网络视频小组", "value": "31"}, {"label": "HDSPad/移动视频小组", "value": "18"}, {"label": "HDSCD/无损音乐小组", "value": "22"}, {"label": "HDSpecial|稀缺资源", "value": "34"}, {"label": "Original/自制原创资源", "value": "24"}, {"label": "Other/其他制作组或转发资源", "value": "27"}, {"label": "Autoseed/自动发布机器人", "value": "26"}, {"label": "BMDru小组", "value": "30"}, {"label": "AREA11/韩剧合作小组", "value": "25"}, {"label": "Request/应求发布资源", "value": "33"}, {"label": "HDSWEB/(网络视频小组合集专用)", "value": "35"}, {"label": "HDSAB/有声书小组", "value": "36"}, {"label": "HDSWEB/(补档专用)", "value": "37"}], "tags": [{"label": "官组", "value": "12"}, {"label": "首发", "value": "1"}, {"label": "禁转", "value": "2", "standard_keys": ["tag.restricted"]}, {"label": "DIY", "value": "13", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "粤语", "value": "11", "standard_keys": ["tag.cantonese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "特效", "value": "20", "standard_keys": ["tag.special_effects_subs"]}, {"label": "Dolby Vision", "value": "15", "standard_keys": ["tag.dolby_vision"]}, {"label": "HLG", "value": "16", "standard_keys": ["tag.hlg"]}, {"label": "HDR10+", "value": "17", "standard_keys": ["tag.hdr10_plus"]}, {"label": "SL-HDR1", "value": "18"}, {"label": "应求", "value": "19"}, {"label": "自制", "value": "14"}, {"label": "HDR10", "value": "9", "standard_keys": ["tag.hdr10"]}, {"label": "Atmos", "value": "21"}, {"label": "次世代国语", "value": "22"}, {"label": "DTS-X", "value": "23"}, {"label": "DoVi+HDR", "value": "24"}, {"label": "限转", "value": "25"}, {"label": "DIY纯净版", "value": "26"}, {"label": "全景声国语", "value": "27"}, {"label": "原生原盘", "value": "28", "standard_keys": ["tag.original_disc"]}, {"label": "临境声国语", "value": "29"}, {"label": "无广告", "value": "30"}, {"label": "去头尾广告纯净版", "value": "31"}]}}`).Error
	})
}
