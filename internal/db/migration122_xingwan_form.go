package db

import (
	"gorm.io/gorm"
)

// migration122 §59.439: 星湾（xingwan.cc）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（47KB 代理）+ docs/32/xingwan.md
// 336 行读全（促销规则+发布规则完整采集）。
// **资产两槽缺口实时补全**（team_sel 27 项+tags 31 项 4 月完全未
// 捕获——青蛙/学校/海的宝同款）；label 全面漂移（emoji+中文说明长
// 形态——实时为准）；**文档 §1.1 称有 uplver——实时 grep 零次=文档
// 漂移实时为准（无匿名字段）**。
// 槽位映射：
//   - type 14（label=emoji 实时形态）：410 短剧第十五站+416 漫画
//     label-only（dict 无 comics 键）+406=[mv,music_video]+408 双键+
//     404 双键。
//   - **medium 12 全配键（值 10-21 全乱序）**：Blu-ray/BD(10)=
//     [bluray,uhd_bluray] 双键归并（无 UHD 分立项）+Remux(11)=
//     [remux,uhd_remux] 双键+Encode(12)/WEB-DL(13)/WEBRip(14) 三分立
//     各单键+DVD/DVDR(16)=[dvd,dvdr] 站方合并项双键+**MiniBD(17)=
//     [minibd]+HD DVD(18)=[hddvd]（大青虫精确键复利）**+CD/Track/
//     Other。
//   - **codec 11 全配键罕见格式大满贯**：AV1/VP9/Xvid/**DivX=ext
//     活键**/**ProRes=ext 活键（专业中间格式首配）**+H.264/H.265/
//     VC-1/MPEG-2/Other；DNxHD/DNxHR(14) label-only（ext 无键）——
//     无 X26x 零联动。
//   - **standard 16 项全库最多级别**（3D 三形态+IMAX+VR 双形态罕见
//     细分）：SD/720p/1080p/i/1440p/QHD(10)/2160p/8K 全配+**2K(9)
//     label-only（1440p 独立在表防一键多值——13城分辨率类先例）**+
//     **4K Remux(12) label-only（媒介维度混入分辨率槽语义存疑——
//     柠檬 4K/8K-UltraHD 同款保守不猜）**+3D SBS/HSBS/MVC+IMAX+
//     VR 180/360 label-only（无 canonical 键）+Other=[resolution.
//     other]；16 项 label 站方 HTML 原文自带截断（"3D MVC / Blu-ray
//     3D （原生 3D 蓝光画"——§59.384 照抄原则）。
//   - **team 27 外站官组大表（OKPT 28 组后第二例）**：CHD/WiKi/HDS/
//     PTerWEB/FraMeSToR/TrollUHD/CtrlHD/FLUX/JoyHD/HQC/QHstudIo/DOA/
//     EBP/HDT/EMP/NTb/TayTO/NGB/QxR/YIFY/CMCT/MWeb/MTeam/HHClub/
//     HHWeb/**FRDS(30) 在表直中**/Audiences 全 label-only+Other(24)=
//     [team.other] 兜底。
//   - **tags 31 双区（分类镜像区 19+内容区 12）**：分类镜像区
//     label-only（type 域镜像零产出——22 TG吃瓜/23 9KG🈲/25 已刮削
//     同）+**26 英字=[english_subtitle]（藏宝阁首配键——在分类区
//     但语义内容标签）**；内容区：**1 禁转/2 首发 二绝不配**（官方
//     专属财神雷区——本站无原创/官方标签）+泛 HDR 键分配精细化
//     （**7 HDR=[hdr10,hlg] 泛家族剩余两键——hdr10_plus 给 27
//     HDR10+ 独立项/hdr_vivid 给 33 HDRVi 独立项，三站首发防一键
//     多值**）+4 DIY/5 国语/6 中字/31 粤语/35 英语/29 特效=[special_
//     effects_subs] 十二键配+32 4K/34 HDR10bit label-only（分辨率
//     类/站方自定义语义无键）。
//   - 字段：type/medium_sel[4]/codec_sel[4]/standard_sel[4]/
//     team_sel[4]/tags[4][]/small_descr/descr/technical_info；nfo 不配
//     （13城先例——域表无 nfo 域配了必 formdiff 假差异；GTK 澄清实未配）；
//     **无 uplver（文档漂移）+无 url/pt_gen/dburl/douban_id+无
//     audiocodec 槽（第五站）**——质量信息仅靠选择器+标题传达。
// 站规人工把关：随机促销+自动免费（>20GB/原盘/每季首集）+YIFY/
//   YTS 低码率"不建议收藏"+上传者资格。9KG🈲 标签=成人区镜像禁发。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(122, "xingwan_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'xingwan.cc' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'xingwan.cc'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "technical_info": "technical_info"}, "framework": "np", "value_mappings": {"type": [{"label": "Movies（🎬 电影）", "value": "401", "standard_keys": ["category.movie"]}, {"label": "TV Series（📺 电视剧）", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "ShortDrama（📱 短剧）", "value": "410", "standard_keys": ["category.short_drama"]}, {"label": "Animations（🌸 动漫）", "value": "405", "standard_keys": ["category.animation"]}, {"label": "TV Shows（🎤 综艺）", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "Documentaries（🌍 纪录片）", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "Comics（📖 漫画）", "value": "416"}, {"label": "Music Videos（🎵 音乐视频）", "value": "406", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "HQ Audio（🎧 无损音乐）", "value": "408", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "Sports（⚽ 体育）", "value": "407", "standard_keys": ["category.sports"]}, {"label": "Games（🎮 游戏）", "value": "411", "standard_keys": ["category.game"]}, {"label": "Software（💻 软件）", "value": "412", "standard_keys": ["category.software"]}, {"label": "Ebooks（📚 电子书）", "value": "413", "standard_keys": ["category.ebook"]}, {"label": "Misc（📦 其他杂项）", "value": "409", "standard_keys": ["category.other"]}], "medium": [{"label": "Blu-ray / BD（蓝光原盘 “保留完整内容” ）", "value": "10", "standard_keys": ["medium.bluray", "medium.uhd_bluray"]}, {"label": "Remux（无损封装，画质与音频无转码）", "value": "11", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "Encode（压制版，空间与画质折中）", "value": "12", "standard_keys": ["medium.encode"]}, {"label": "WEB-DL（官网源，无台标，质量较好）", "value": "13", "standard_keys": ["medium.webdl"]}, {"label": "WEBRip（流媒体录制源，质量一般）", "value": "14", "standard_keys": ["medium.webrip"]}, {"label": "HDTV（电视台来源，可能带台标/压制）", "value": "15", "standard_keys": ["medium.hdtv"]}, {"label": "DVD / DVDR（DVD 原盘或镜像）", "value": "16", "standard_keys": ["medium.dvd", "medium.dvdr"]}, {"label": "MiniBD（迷你蓝光压制）", "value": "17", "standard_keys": ["medium.minibd"]}, {"label": "HD DVD（老式高清碟片 ”罕见“）", "value": "18", "standard_keys": ["medium.hddvd"]}, {"label": "CD（音乐 CD）", "value": "19", "standard_keys": ["medium.cd"]}, {"label": "Track / Soundtrack（原声音乐 / OST）", "value": "20", "standard_keys": ["medium.track"]}, {"label": "Other（其他或未知媒介）", "value": "21", "standard_keys": ["medium.other"]}], "codec": [{"label": "H.264 / AVC（主流高清编码）", "value": "1", "standard_keys": ["video.h264"]}, {"label": "H.265 / HEVC（4K/HDR 主流高效编码）", "value": "6", "standard_keys": ["video.h265"]}, {"label": "AV1（新一代开源高压缩编码）", "value": "7", "standard_keys": ["video.av1"]}, {"label": "VC-1（老蓝光片常见编码）", "value": "8", "standard_keys": ["video.vc1"]}, {"label": "MPEG-2（DVD/电视台旧格式）", "value": "9", "standard_keys": ["video.mpeg2"]}, {"label": "VP9（YouTube/WEBRip 常见）", "value": "10", "standard_keys": ["video.vp9"]}, {"label": "Xvid（旧影片低码率编码）", "value": "11", "standard_keys": ["video.xvid"]}, {"label": "DivX（早期压制格式 ”旧“ ）", "value": "12", "standard_keys": ["video.divx"]}, {"label": "ProRes（专业后期中间格式）", "value": "13", "standard_keys": ["video.prores"]}, {"label": "DNxHD / DNxHR（专业剪辑中间格式）", "value": "14"}, {"label": "Other（其他或无法识别）", "value": "15", "standard_keys": ["video.other"]}], "standard": [{"label": "SD（标清资源 ”DVD/TV 老片“ ）", "value": "5", "standard_keys": ["resolution.sd"]}, {"label": "720p （高清 “常见旧剧” ）", "value": "6", "standard_keys": ["resolution.r720p"]}, {"label": "1080p （全高清 ”主流下载“ ）", "value": "7", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i （交错扫描 ”早期 HDTV“ ）", "value": "8", "standard_keys": ["resolution.r1080i"]}, {"label": "2K （数字电影分辨率）", "value": "9"}, {"label": "1440p / QHD （介于 1080 与 4K 之间）", "value": "10", "standard_keys": ["resolution.r1440p"]}, {"label": "2160p / 4K UHD （超高清 ”主流 4K“ ）", "value": "11", "standard_keys": ["resolution.r2160p"]}, {"label": "4K Remux （原盘级无损封装 4K）", "value": "12"}, {"label": "8K （超高分辨率）", "value": "13", "standard_keys": ["resolution.r4320p"]}, {"label": "3D SBS （3D 左右格式）", "value": "14"}, {"label": "3D HSBS （半宽左右 3D 格式）", "value": "15"}, {"label": "3D MVC / Blu-ray 3D （原生 3D 蓝光画", "value": "16"}, {"label": "IMAX （大画幅版本）", "value": "17"}, {"label": "VR 180° （半景 VR 格式）", "value": "18"}, {"label": "VR 360° （全景 VR 格式）", "value": "19"}, {"label": "Other （其他或无法识别）", "value": "20", "standard_keys": ["resolution.other"]}], "team": [{"label": "CHD （华语顶级压制）", "value": "6"}, {"label": "WiKi （高质量蓝光压制）", "value": "7"}, {"label": "HDS（高清压制）", "value": "8"}, {"label": "PTerWEB（PTerClub 猫站官方 WEB 压制）", "value": "9"}, {"label": "FraMeSToR （顶级 Remux 制作组）", "value": "10"}, {"label": "TrollUHD （4K/HDR/IMAX 高品质压制）", "value": "11"}, {"label": "CtrlHD （高清 WEB-DL / Remux 来源组）", "value": "12"}, {"label": "FLUX （欧美高质量蓝光压制）", "value": "28"}, {"label": "JoyHD （高码率高画质压制）", "value": "13"}, {"label": "HQC （高保真压制）", "value": "14"}, {"label": "QHstudIo （高质量压制）", "value": "27"}, {"label": "DOA （欧美蓝光压制）", "value": "15"}, {"label": "EBP （动漫/剧集字幕友好）", "value": "16"}, {"label": "HDT （欧洲字幕/修复压制组）", "value": "17"}, {"label": "EMP （高码率忠实原画风格）", "value": "18"}, {"label": "NTb （Netflix / Disney+ 官方源）", "value": "19"}, {"label": "TayTO （欧美剧集压制）", "value": "20"}, {"label": "NGB （亚洲影视压制）", "value": "21"}, {"label": "QxR（小体积优化压制）", "value": "22"}, {"label": "YIFY / YTS （低码率 ”不建议收藏“）", "value": "23"}, {"label": "CMCT（小体积高清压制）", "value": "25"}, {"label": "MWeb / MTeam（馒头站内制作组）", "value": "26"}, {"label": "HHClub / HHWeb（憨憨站内制作组）", "value": "29"}, {"label": "FRDS（UHD 蓝光 mUHD 压制）", "value": "30"}, {"label": "Audiences", "value": "31"}, {"label": "Other （其他未归类）", "value": "24", "standard_keys": ["team.other"]}], "tags": [{"label": "电影", "value": "10"}, {"label": "电视剧", "value": "11"}, {"label": "短剧", "value": "12"}, {"label": "综艺", "value": "13"}, {"label": "纪录片", "value": "14"}, {"label": "动漫", "value": "15"}, {"label": "音乐视频", "value": "16"}, {"label": "无损音乐", "value": "17"}, {"label": "体育", "value": "18"}, {"label": "游戏", "value": "19"}, {"label": "软件", "value": "20"}, {"label": "电子书", "value": "21"}, {"label": "TG吃瓜热搜", "value": "22"}, {"label": "9KG🈲", "value": "23"}, {"label": "其他杂项", "value": "24"}, {"label": "已刮削", "value": "25"}, {"label": "英字", "value": "26", "standard_keys": ["tag.english_subtitle"]}, {"label": "漫画", "value": "30"}, {"label": "禁转", "value": "1"}, {"label": "首发", "value": "2"}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "英语", "value": "35", "standard_keys": ["tag.english_audio"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "4K", "value": "32"}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hlg"]}, {"label": "HDR10bit", "value": "34"}, {"label": "HDRVi", "value": "33", "standard_keys": ["tag.hdr_vivid"]}, {"label": "粤语", "value": "31", "standard_keys": ["tag.cantonese_audio"]}, {"label": "特效", "value": "29", "standard_keys": ["tag.special_effects_subs"]}, {"label": "HDR10+", "value": "27", "standard_keys": ["tag.hdr10_plus"]}]}}`).Error
	})
}
