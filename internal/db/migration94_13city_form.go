package db

import (
	"gorm.io/gorm"
)

// migration94 §59.410: 13城（13city.org）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（47.3KB 代理抓取）+ docs/32/13city.md
// 685 行读全（2026-04-22 Playwright 实采+rules.php 2025 修订版发种规范）。
// 实时=4 月资产全槽一致；[4] 后缀标准形态；单模式站；无 Cloudflare。
// 槽位特化：
//   - **type 非标值集**（修道院/织梦系）：413 纪录片（非 404！）+
//     406 演唱会、MV=[concert,mv,music_video] 三键承接（克隆/优堡同款
//     合并项）+408 Music=[music,lossless_music] 双键+409 有声读物
//     =audiobook（优堡 415 同键）。
//   - medium 13 项：**BluRay(11)=[bdrip] 独立项**（压制种 BluRay 来源
//     ——与原盘 Blu-ray(1) 分立+§59.349 ST 仲裁驼峰语义对位）+
//     **WEBRip(12) 独立精确**（WEB-DL(10) 不承接 webrip——站方有独立项）
//     +Remux 归并+DVDR 归并+Other=medium.other；全 13 项配键。
//   - codec 7 项合并命名（AVC/H.264/x264——无独立 X26x 联动零触发）
//     +**VPB/VP9=[vp9]**（站方拼写 VPB 原样保留 label）+Xvid；**无 AV1**。
//   - standard 5 项极简：8K/4K/1080p/1080i/Other——**r720p/sd 不配**
//     （站规≥1080p 硬要求——1988 前远古作品例外走预览人工选 Other；
//     与南洋 720p miss 落空同款保守）。
//   - audiocodec 12 项：**DTS-HD/DTS(3) 合并项承接家族四键**
//     [dts,dts_hd_ma,dts_hd_hr,dtsx]（车站同款）+DDP/E-AC3 独立+
//     TrueHD/TrueHD Atmos 分立（atmos 泛键归 9 好大先例）+DD/AC3 三键
//     （ac3+dd）+LPCM。
//   - processing_sel=**产区槽不配**（§59.385 咖啡定案同名异义——中国
//     含港澳台/日/泰/印/韩/欧美——非处理方式语义）。
//   - team 19→18 项 label-only：**站方重复值条目 QHstudIo 8/19 去重留
//     首值 19**（克隆 §59.394 先例——label 源=站方 HTML 原文首值原则）
//     +**FRDS(6) 在表直中**+外站官组十五个（13City/AilMWeb/52pt/HHWEB/
//     CHDWEB/MTeam/QHstudIo/PTerWEB/rainweb/SewageWeb/UBWEB/ZmWeb/
//     ADWeb/UBits/WiKi/HDSWEB）+Other 兜底。
//   - tags 15 项：8 项配键（禁转/完结=complete/分集=ongoing/粤语/国语/
//     中字/**HDR 四键泛家族第六站**/DIY）；首发不配（铁律）+7 项
//     label-only（多语——bilingual_audio 是双语非多语防误配/中英双字
//     ——chinese_subtitle 已配"中字"防一键多值冲突/4K+1080p——分辨率
//     类不进 tags 列零产出/红叶转载——站际标记/有声图书）。
//   - 字段全齐：url（IMDb）+pt_gen（四源）+nfo+technical_info+uplver；
//     url×2 第二个是编辑器按钮；无 source_sel。
//   - 站规注记：≥1080p（1988 前例外）+电影≥1GB+完结必合集+禁转内容
//     概不接收+短剧不收+H&R 手动勾选（考察期 240h）——发布预览人工
//     把关。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(94, "city13_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = '13city.org' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = '13city.org'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "audiocodec": "audiocodec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "pt_gen": "pt_gen", "technical_info": "technical_info", "nfo": "nfo", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影/Movies", "value": "401", "standard_keys": ["category.movie"]}, {"label": "剧集/TV Series", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "综艺/TV Shows", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "动漫/Animations", "value": "405", "standard_keys": ["category.animation"]}, {"label": "演唱会、MV/Concert、Music Videos", "value": "406", "standard_keys": ["category.concert", "category.mv", "category.music_video"]}, {"label": "音乐/Music", "value": "408", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "纪录片/Docmentaries", "value": "413", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "有声读物/Audiobook", "value": "409", "standard_keys": ["category.audiobook"]}], "medium": [{"label": "Track", "value": "9", "standard_keys": ["medium.track"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}, {"label": "DVDR", "value": "6", "standard_keys": ["medium.dvdr", "medium.dvd"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "MiniBD", "value": "4", "standard_keys": ["medium.minibd"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "HD DVD", "value": "2", "standard_keys": ["medium.hddvd"]}, {"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray"]}, {"label": "WEB-DL", "value": "10", "standard_keys": ["medium.webdl"]}, {"label": "BluRay", "value": "11", "standard_keys": ["medium.bdrip"]}, {"label": "WEBRip", "value": "12", "standard_keys": ["medium.webrip"]}, {"label": "Other", "value": "13", "standard_keys": ["medium.other"]}], "codec": [{"label": "AVC/H.264/x264", "value": "1", "standard_keys": ["video.h264"]}, {"label": "HEVC/H.265/x265", "value": "2", "standard_keys": ["video.h265"]}, {"label": "MPEG-2", "value": "3", "standard_keys": ["video.mpeg2"]}, {"label": "VC-1", "value": "4", "standard_keys": ["video.vc1"]}, {"label": "VPB/VP9", "value": "5", "standard_keys": ["video.vp9"]}, {"label": "Xvid", "value": "6", "standard_keys": ["video.xvid"]}, {"label": "Other", "value": "7", "standard_keys": ["video.other"]}], "audiocodec": [{"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "DTS-HD/DTS", "value": "3", "standard_keys": ["audio.dts", "audio.dts_hd_ma", "audio.dts_hd_hr", "audio.dtsx"]}, {"label": "MP3", "value": "4", "standard_keys": ["audio.mp3"]}, {"label": "OGG", "value": "5", "standard_keys": ["audio.ogg"]}, {"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "DDP/E-AC3", "value": "7", "standard_keys": ["audio.ddp"]}, {"label": "TrueHD", "value": "8", "standard_keys": ["audio.truehd"]}, {"label": "TrueHD Atmos", "value": "9", "standard_keys": ["audio.atmos", "audio.truehd_atmos"]}, {"label": "LPCM", "value": "10", "standard_keys": ["audio.lpcm"]}, {"label": "Other", "value": "11", "standard_keys": ["audio.other"]}, {"label": "DD/AC3", "value": "12", "standard_keys": ["audio.ac3", "audio.dd"]}], "standard": [{"label": "8K", "value": "1", "standard_keys": ["resolution.r4320p"]}, {"label": "4K", "value": "2", "standard_keys": ["resolution.r2160p"]}, {"label": "1080p", "value": "3", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "4", "standard_keys": ["resolution.r1080i"]}, {"label": "Other", "value": "5", "standard_keys": ["resolution.other"]}], "team": [{"label": "13City", "value": "1"}, {"label": "AilMWeb", "value": "2"}, {"label": "52pt", "value": "3"}, {"label": "HHWEB", "value": "4"}, {"label": "CHDWEB", "value": "5"}, {"label": "FRDS", "value": "6"}, {"label": "MTeam", "value": "7"}, {"label": "PTerWEB", "value": "9"}, {"label": "rainweb", "value": "10"}, {"label": "Other", "value": "11"}, {"label": "SewageWeb", "value": "12"}, {"label": "UBWEB", "value": "13"}, {"label": "ZmWeb", "value": "14"}, {"label": "ADWeb ", "value": "15"}, {"label": "UBits", "value": "16"}, {"label": "WiKi", "value": "17"}, {"label": "HDSWEB", "value": "18"}, {"label": "QHstudIo", "value": "19"}], "tags": [{"label": "禁转", "value": "1", "standard_keys": ["tag.restricted"]}, {"label": "完结", "value": "9", "standard_keys": ["tag.complete"]}, {"label": "分集", "value": "8", "standard_keys": ["tag.ongoing"]}, {"label": "粤语", "value": "13", "standard_keys": ["tag.cantonese_audio"]}, {"label": "国语", "value": "5", "standard_keys": ["tag.chinese_audio"]}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "首发", "value": "2"}, {"label": "多语", "value": "10"}, {"label": "中英双字", "value": "14"}, {"label": "4K", "value": "12"}, {"label": "1080p", "value": "11"}, {"label": "红叶转载", "value": "19"}, {"label": "有声图书", "value": "15"}]}}`).Error
	})
}
