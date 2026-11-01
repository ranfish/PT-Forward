package db

import (
	"gorm.io/gorm"
)

// migration98 §59.414: 三月（duckboobee.org）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（80.8KB 直连抓取）+ docs/32/
// duckboobee.md 344 行读全（rules.php 规则/dupe 优先级/打包规范）。
// **实时 vs 文档+4 月资产大改版**（实时为准）：
//   - type 三处 label 漂移：405 Cartoon→**Animations**/406 MTV→
//     **Music Videos**/408 Music→**HQ Audio**。
//   - medium 8→10 项：**新增 Track(9)**+**DVDR 重复双值 6/2**（同
//     label 双 value——克隆式站方重复值条目去重留 HTML 首值 6）；
//     资产漏 6/9 两项。
//   - codec：XVID(9)→**VP9(9)** 漂移；**H.264(1)/X.264(6)+HEVC(2)/
//     H.265(3)/X.265(7) 三对分立**——压制联动第七站（lookupCLIVariant
//     增强带点形态 "X.264"/"X.265"——三月 label 带点不命中无点匹配，
//     通用增强未来站自动生效）；HEVC(2)=[h265] 首现配键+H.265(3)
//     label-only（同键第二现一键多值防线）+X.264/X.265 label-only
//     （联动通道 label 精确匹配）。
//   - **tags 大砍 22→13 项**：站方删除 禁转(1)/首发(9)/粤语(10)/
//     限转(16)/应求(17)/MV(18)/卡拉OK(19)/LVE现场(20)/演唱会(21)/
//     音乐专辑(22) 十项（音乐系标签整族下线+禁转族下线）——新增
//     **AV1(20)**（编码类标签——codec 域零产出 label-only）+**Ai
//     双语字幕(18)**（无键 label-only）；保留 HDR 四细分（泛 HDR/
//     HDR10/HDR10+/Dolby Vision）。
//   - 无 IMG 真字段（文档有——实时仅编辑器按钮）。
// 槽位映射：
//   - type：404 纪录双键+405 Animations=[animation]+406 Music Videos
//     三键 [mv,music_video,concert]+408 HQ Audio=[music,lossless_
//     music] 双键+409 Misc=[other]。
//   - medium：无 UHD 独立 Blu-Ray=[bluray,uhd_bluray] 双键归并
//     第四例+WEB-DL 承接 webrip+Remux 双键+DVDR/DVD 分立配键
//     （6=DVDR 精确/8=DVD 泛义）。
//   - standard 7 项大写 P：全配键+**480P=[sd] 单配**（576P 无键
//     label-only 防一键多值——PAL 标清无 r576p 键）；值 5 缺失。
//   - **无 audiocodec 槽第三站**（GTK/克隆后）。
//   - team 12 项 label-only：3MWEB(1) 官组+外站组（CHD/TTG/CMCT/
//     HDSKY/OurBits/M-Team/WIKI/Beast/HHWEB）+Other 兜底（FRDS
//     不在表）。
//   - tags 13 项配键 9：分集=[ongoing]/DIY/国语/中字/**HDR(7)=
//     [hdr_vivid,hlg]**（泛 HDR 归 Vivid+HLG——站方无独立项精确
//     归属）/HDR10+=[hdr10_plus]/HDR10=[hdr10]/**Dolby Vision=
//     [dolby_vision] 第五站**/完结=[complete]；AV1/Ai双语字幕/
//     动画/官字组 label-only；**无禁转/首发（站方已删）**。
//   - 字段：url+pt_gen+technical_info+uplver；nfo/IMG 不配。
// 站规同垃圾堆系（<100MB 禁/upscale 禁/RMVB 禁/游戏候选制/Dupe
// 媒介优先级+断种豁免）——发布预览人工把关。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(98, "sanyue_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'duckboobee.org' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'duckboobee.org'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Movies", "value": "401", "standard_keys": ["category.movie"]}, {"label": "TV Series", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "TV Shows", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "Documentaries", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "Animations", "value": "405", "standard_keys": ["category.animation"]}, {"label": "Music Videos", "value": "406", "standard_keys": ["category.mv", "category.music_video", "category.concert"]}, {"label": "Sports", "value": "407", "standard_keys": ["category.sports"]}, {"label": "HQ Audio", "value": "408", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "Misc", "value": "409", "standard_keys": ["category.other"]}], "medium": [{"label": "WEB-DL", "value": "13", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "Track", "value": "9", "standard_keys": ["medium.track"]}, {"label": "DVD", "value": "8", "standard_keys": ["medium.dvd"]}, {"label": "DVDR", "value": "6", "standard_keys": ["medium.dvdr"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "Other", "value": "4", "standard_keys": ["medium.other"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "Blu-Ray", "value": "1", "standard_keys": ["medium.bluray", "medium.uhd_bluray"]}], "codec": [{"label": "H.264", "value": "1", "standard_keys": ["video.h264"]}, {"label": "HEVC", "value": "2", "standard_keys": ["video.h265"]}, {"label": "H.265", "value": "3"}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "MPEG-4", "value": "5", "standard_keys": ["video.mpeg4"]}, {"label": "X.264", "value": "6"}, {"label": "X.265", "value": "7"}, {"label": "VP9", "value": "9", "standard_keys": ["video.vp9"]}, {"label": "AV1", "value": "10", "standard_keys": ["video.av1"]}, {"label": "Other", "value": "11", "standard_keys": ["video.other"]}], "standard": [{"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "576P", "value": "4"}, {"label": "2160P", "value": "6", "standard_keys": ["resolution.r2160p"]}, {"label": "480P", "value": "7", "standard_keys": ["resolution.sd"]}, {"label": "4320P", "value": "8", "standard_keys": ["resolution.r4320p"]}], "team": [{"label": "3MWEB", "value": "1"}, {"label": "CHD", "value": "2"}, {"label": "TTG", "value": "3"}, {"label": "CMCT", "value": "4"}, {"label": "HDSKY", "value": "5"}, {"label": "OurBits", "value": "6"}, {"label": "M-Team", "value": "7"}, {"label": "WIKI", "value": "8"}, {"label": "Beast", "value": "9"}, {"label": "HHWEB", "value": "10"}, {"label": "Other", "value": "11"}], "tags": [{"label": "分集", "value": "2", "standard_keys": ["tag.ongoing"]}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr_vivid", "tag.hlg"]}, {"label": "AV1", "value": "20"}, {"label": "Ai双语字幕", "value": "18"}, {"label": "HDR10+", "value": "15", "standard_keys": ["tag.hdr10_plus"]}, {"label": "HDR10", "value": "14", "standard_keys": ["tag.hdr10"]}, {"label": "Dolby Vision", "value": "13", "standard_keys": ["tag.dolby_vision"]}, {"label": "动画", "value": "12"}, {"label": "官字组", "value": "11"}, {"label": "完结", "value": "8", "standard_keys": ["tag.complete"]}]}}`).Error
	})
}
