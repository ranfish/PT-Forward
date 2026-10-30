package db

import (
	"gorm.io/gorm"
)

// migration64 §59.377: 五二（52pt.site）发布表单配置分发。
// 依据 docs/32/52pt.md（4 月批量分析期 540 行表单分析）+ sites.json form
// 资产（六槽映射）生成；29/243 凭证当前失效——实时比对延后（DryRun 纯
// 本地组装先行验证，§59.370"组装层终点"）。
// 槽位语义（NP 标准裸字段名——无 [4] 后缀无 data-mode）：
//   type/medium_sel/codec_sel/audiocodec_sel/standard_sel/team_sel/tags[]
//   +small_descr/descr/url(imdb)/uplver
// 关键映射决策：
//   - type 404 纪录片：§59.376 双键教训（category.documentary+
//     category.documentaries 并存——dict 单数/extended 复数分叉）
//   - medium：站方带语言维度（无中文/中字）——标准键映射主形态
//     （11 Blu-ray 原盘→bluray / 1 4K UHD→uhd_bluray），DIY/中字特化
//     选项 label-only（默认无中文系，发布预览可手调）
//   - codec：video.h264→13(H.264/AVC 标准形态)——X264/X265(11/12) 是
//     CLI 压制词形态 label-only；H.265 双值(1/14 站方原始重复)配 1
//   - audiocodec 5 APE 重复值 label-only（2 已配）；DTS.HDMA→4
//   - standard 6 others 承接杂项（sd/r1440p/other）；4 1080P-3D
//     label-only（3D 特化）
//   - team 全 label-only（组名走 release_group_mappings 组映射表）
//   - tags 中文值：国语/粤语→audio 键、简中→chinese_subtitle（繁中/
//     简繁/双语字幕同族 label-only 单键不重复）、DIY/官组；首发不存在
//     ✓（转存禁勾首发铁律无风险）
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(64, "pt52_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = '52pt.site' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = '52pt.site'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel", "codec": "codec_sel", "audiocodec": "audiocodec_sel", "standard": "standard_sel", "team": "team_sel", "tags": "tags[]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Movies/电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "TV Series/剧集", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "TV Shows/综艺", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "Documentaries/纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "Animations/动漫", "value": "405", "standard_keys": ["category.animation"]}, {"label": "Music Videos/音乐MV", "value": "406", "standard_keys": ["category.mv"]}, {"label": "Sports/运体", "value": "407", "standard_keys": ["category.sports"]}, {"label": "HQ Audio/无损音乐", "value": "408", "standard_keys": ["category.music"]}, {"label": "Misc/其他", "value": "409", "standard_keys": ["category.other"]}, {"label": "Opera/戏曲", "value": "410"}, {"label": "Erotic/18+", "value": "411"}], "medium": [{"label": "4K UHD无中文", "value": "1", "standard_keys": ["medium.uhd_bluray"]}, {"label": "Blu-ray DIY", "value": "2"}, {"label": "HDTV", "value": "3", "standard_keys": ["medium.hdtv"]}, {"label": "Blu-ray Remux", "value": "4", "standard_keys": ["medium.remux"]}, {"label": "4K UHD Remux", "value": "5", "standard_keys": ["medium.uhd_remux"]}, {"label": "DVD/DVDR", "value": "6", "standard_keys": ["medium.dvdr", "medium.dvd"]}, {"label": "Encode(MKV/MP4)", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "CD/HDCD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "Others", "value": "9", "standard_keys": ["medium.other"]}, {"label": "WEB-DL", "value": "10", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "Blu-ray原盘无中文", "value": "11", "standard_keys": ["medium.bluray"]}, {"label": "4K UHD DIY", "value": "12"}, {"label": "8K UHD", "value": "13"}, {"label": "2K原盘中字", "value": "14"}, {"label": "4K原盘中字", "value": "15"}], "codec": [{"label": "H.265(HEVC)", "value": "1", "standard_keys": ["video.h265"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "Xvid", "value": "3", "standard_keys": ["video.xvid"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}, {"label": "X264", "value": "11"}, {"label": "X265", "value": "12"}, {"label": "H.264/AVC", "value": "13", "standard_keys": ["video.h264"]}, {"label": "H.265", "value": "14"}], "audiocodec": [{"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "DTS:X", "value": "3", "standard_keys": ["audio.dtsx"]}, {"label": "DTS.HDMA", "value": "4", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "AC3/DD", "value": "6", "standard_keys": ["audio.ac3", "audio.dd"]}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}, {"label": "TRUE.HD Atmos", "value": "10", "standard_keys": ["audio.truehd_atmos"]}, {"label": "WAV", "value": "11", "standard_keys": ["audio.wav"]}, {"label": "True.HD", "value": "12", "standard_keys": ["audio.truehd"]}, {"label": "AAC", "value": "13", "standard_keys": ["audio.aac"]}, {"label": "LPCM", "value": "14", "standard_keys": ["audio.lpcm"]}, {"label": "DTS", "value": "15", "standard_keys": ["audio.dts"]}], "standard": [{"label": "2K/1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "1080P-3D", "value": "4"}, {"label": "4K/2160P", "value": "5", "standard_keys": ["resolution.r2160p"]}, {"label": "others", "value": "6", "standard_keys": ["resolution.sd", "resolution.r1440p", "resolution.other"]}, {"label": "8K/4320", "value": "7", "standard_keys": ["resolution.r4320p"]}], "team": [{"label": "BeyondHD", "value": "1"}, {"label": "HDSKY", "value": "2"}, {"label": "TTG", "value": "3"}, {"label": "COASTER", "value": "4"}, {"label": "Other/其他", "value": "5"}, {"label": "52PT DIY", "value": "6"}, {"label": "52PT REMUX", "value": "7"}, {"label": "MTeam", "value": "8"}, {"label": "CHDBits", "value": "9"}, {"label": "OurBits", "value": "10"}, {"label": "HDHome", "value": "11"}, {"label": "CMCT", "value": "12"}, {"label": "HDChina", "value": "14"}, {"label": "PTHOME", "value": "15"}, {"label": "Audies", "value": "16"}], "tags": [{"label": "官组", "value": "官组", "standard_keys": ["tag.official"]}, {"label": "原盘", "value": "原盘"}, {"label": "DIY", "value": "DIY", "standard_keys": ["tag.diy"]}, {"label": "Remux", "value": "Remux"}, {"label": "国语", "value": "国语", "standard_keys": ["tag.chinese_audio"]}, {"label": "粤语", "value": "粤语", "standard_keys": ["tag.cantonese_audio"]}, {"label": "简中", "value": "简中", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "繁中", "value": "繁中"}, {"label": "简繁", "value": "简繁"}, {"label": "双语字幕", "value": "双语字幕"}, {"label": "杜比视界", "value": "杜比视界"}, {"label": "HDR", "value": "HDR"}, {"label": "3D", "value": "3D"}]}}`).Error
	})
}
