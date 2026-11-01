package db

import (
	"gorm.io/gorm"
)

// migration89 §59.404: 海胆（www.haidan.cc）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（51.3KB 直连抓取）+ docs/32/haidan.md
// 446 行读全。实时 HTML 与 sites.json 4 月资产全槽一致；**四资产缺口
// 实时补全**（tags/team/processing/source 全 None→tag_list[] 6 项+
// team_suffix 形态实证——全库最大资产缺口站）。
// 无模式系统（字段名无 [] 后缀——五二同款裸名）；单 select 站。
// 槽位特化：
//   - **无 team_sel——team_suffix 自由文本（必填！）**：team 域映射
//     此文本框，配置 label=value 同值条目（FRDS="FRDS"——文本框填
//     组名本身非 select value；teamMapping EqualFold label 匹配→
//     Value=组名直填）；其它组 miss→无 Other 项 nil 不填（发布
//     预览人工填——站方自由文本无枚举可配）
//   - **season/episode 独立文本字段**（剧集必填）：发布链无此域
//     （挂账零案例不预实施——电影种不适用；剧集种预览人工填）
//   - **tags 字段名 tag_list[]（第二个非标准名——span[] 后）**：
//     §59.398 current 提示已备（isTagName 精确匹配）；6 项无首发/
//     禁转（站方无此标签✓）；原盘→tag.original_disc 配键；外语→
//     english_audio 单键（英语音轨=外语标记）
//   - **durl 豆瓣链接独有字段**（FieldDomainDoubanURL §59.159 第四
//     站复用——南洋/葡萄后）；无 pt_gen（durl 代替）/无 technical_info
//   - type 9 项标准值集：404 纪录双键+406 MV 双键+408 HQ Audio=
//     lossless_music+409 Misc=other
//   - medium 8 项乱序（UHD=9/WEB-DL=11/DVD=6）：UHD 分立两键+
//     Remux 归并+WEB-DL 承接 webrip+DVD 归并 [dvdr,dvd]
//   - codec 6 项**三重合并命名**（H.264/AVC/X264 一项+MPEG-4/XviD/
//     DivX 三项合一）：无独立 X26x 联动零触发（站方合并语义）；
//     13 承接 [mpeg4,xvid]；无 AV1
//   - audiocodec 10 项：AC3 承接 [ac3,dd,ddp] 三键+DTS-HDMA=
//     dts_hd_ma+True-HD 承接 [truehd,atmos,truehd_atmos]（无独立
//     Atmos）；无 DDP/DTS:X/DTS-HD HR 独立项
//   - standard 5 项 p/i 分立（第三站）+540P→resolution.sd（SD 级
//     语义——末日 480p→sd 先例；无 SD 独立项）
//   - url IMDb 真字段（url×2 按钮同款）；preview/bonus_sticky/
//     bonus_free 付费_boost 字段不配
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(89, "haidan_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'www.haidan.cc' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'www.haidan.cc'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel", "codec": "codec_sel", "audiocodec": "audiocodec_sel", "standard": "standard_sel", "team": "team_suffix", "tags": "tag_list[]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "dburl": "durl", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Movies(电影)", "value": "401", "standard_keys": ["category.movie"]}, {"label": "TV Series(电视剧)", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "TV Shows(综艺)", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "Documentaries(纪录片)", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "Animations(动画片)", "value": "405", "standard_keys": ["category.animation"]}, {"label": "Music Videos(MV)", "value": "406", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "Sports(体育)", "value": "407", "standard_keys": ["category.sports"]}, {"label": "HQ Audio(音乐)", "value": "408", "standard_keys": ["category.lossless_music"]}, {"label": "Misc(其他)", "value": "409", "standard_keys": ["category.other"]}], "medium": [{"label": "UHD Blu-ray", "value": "9", "standard_keys": ["medium.uhd_bluray"]}, {"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "WEB-DL", "value": "11", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "DVD", "value": "6", "standard_keys": ["medium.dvdr", "medium.dvd"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}], "codec": [{"label": "H.264/AVC/X264", "value": "1", "standard_keys": ["video.h264"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}, {"label": "H.265/HEVC/X265", "value": "11", "standard_keys": ["video.h265"]}, {"label": "MPEG-4/XviD/DivX", "value": "13", "standard_keys": ["video.mpeg4", "video.xvid"]}], "audiocodec": [{"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "DTS", "value": "3", "standard_keys": ["audio.dts"]}, {"label": "MP3", "value": "4", "standard_keys": ["audio.mp3"]}, {"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "Other", "value": "7", "standard_keys": ["audio.other"]}, {"label": "AC3", "value": "10", "standard_keys": ["audio.ac3", "audio.dd", "audio.ddp"]}, {"label": "LPCM", "value": "11", "standard_keys": ["audio.lpcm"]}, {"label": "DTS-HDMA", "value": "12", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "True-HD", "value": "13", "standard_keys": ["audio.truehd", "audio.atmos", "audio.truehd_atmos"]}], "standard": [{"label": "2160p/4K", "value": "1", "standard_keys": ["resolution.r2160p"]}, {"label": "1080p", "value": "2", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "3", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "4", "standard_keys": ["resolution.r720p"]}, {"label": "540P", "value": "5", "standard_keys": ["resolution.sd"]}], "team": [{"label": "FRDS", "value": "FRDS"}], "tags": [{"label": "中字", "value": "3", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "原盘", "value": "7", "standard_keys": ["tag.original_disc"]}, {"label": "粤语", "value": "10", "standard_keys": ["tag.cantonese_audio"]}, {"label": "外语", "value": "11", "standard_keys": ["tag.english_audio"]}]}}`).Error
	})
}
