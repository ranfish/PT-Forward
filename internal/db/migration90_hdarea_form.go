package db

import (
	"gorm.io/gorm"
)

// migration90 §59.405: 好大（hdarea.club）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（75.4KB 直连抓取）+ docs/32/hdarea.md
// 531 行读全（含完整站点规则）。实时 HTML 与 sites.json 4 月资产全槽
// 一致；tags/processing/source 三资产 None=站方真无（文档+控件双证）。
// 无模式裸名字段（五二同款）。
// **type 18 项=电影按媒介/分辨率细分 10 项（三维融合——form_config
// 单键单值不可表达）**：hdareaCategoryValue 特化函数路由（用户五定案
// 2026-11-01——§59.391 葡萄模式第二站，typeMappingOf 统一入口挂载）；
// **电影 10 项 label-only**（防 lookupByStdKey 用 category.movie 撞
// 首项）+非电影 8 项配键走通用链（B 组不分媒介——用户定案5）。
// 槽位特化：
//   - **dburl 字段收豆瓣 ID 数字**（字段名 dburl 语义=编号形态——
//     FieldDomainDoubanID 第二站复用 §59.399；id 属性叫 douban_url
//     但 name=dburl——以 name 为准）
//   - 无 tags/pt_gen/technical_info/nfo/source_sel（站方真无）
//   - medium 9 项乱序：无独立 UHD（Blu-ray(1) 归并 uhd_bluray）+
//     REMUX 归并 uhd_remux+WEB-DL 承接 webrip+DVDR 归并 dvd
//   - codec 10 项合并命名（H.264(x264/AVC) 一项——无独立 X26x 联动
//     零触发）+AVS 配键（全库罕见）
//   - audiocodec 28 项（全库最多级别）：DD5.1(5) 与 DD2.0(11) 通道
//     分立（5 承接 [ac3,dd]）+DDP(16)/DDP Atmos(15) 独立+DTS-HD MA/
//     DTS XLL(4)=dts_hd_ma（XLL 别名）+DTS:X(12)/HR(13) 独立+
//     TrueHD Atmos(10) 双键+DSD/TTA/MPEG/AV3A 配键；Vorbis/WMA/
//     AC-4/MPEG-H/MQA 无 canonical label-only
//   - standard 5 项 p/i 分立（第四站）
//   - team 13 项 label-only：HDArea 官组+外站组十一组（EPiC/HDWING/
//     WiKi/TTG/MTeam/HDApad/CHD/HDAccess/HDATV/cXcY/CMCT——车站/
//     学校/GTK 外站组复利链）；cXcY@FRDS 种子提取 FRDS→miss→other(6)
//   - url IMDb 真字段（url×2 按钮同款）
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(90, "hdarea_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'hdarea.club' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'hdarea.club'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel", "codec": "codec_sel", "audiocodec": "audiocodec_sel", "standard": "standard_sel", "team": "team_sel", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "douban_id": "dburl", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "Movie UHD-4K", "value": "300"}, {"label": "Movies Blu-ray", "value": "401"}, {"label": "Movies REMUX", "value": "415"}, {"label": "Movies 3D", "value": "416"}, {"label": "Movies 1080p", "value": "410"}, {"label": "Movies 720p", "value": "411"}, {"label": "Movies DVD", "value": "414"}, {"label": "Movies WEB-DL", "value": "412"}, {"label": "Movies HDTV", "value": "413"}, {"label": "Movies iPad", "value": "417"}, {"label": "Documentaries", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "Animations", "value": "405", "standard_keys": ["category.animation"]}, {"label": "TV Series", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "TV Shows", "value": "403", "standard_keys": ["category.tv_shows"]}, {"label": "Music Videos", "value": "406", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "Sports", "value": "407", "standard_keys": ["category.sports"]}, {"label": "Misc", "value": "409", "standard_keys": ["category.other"]}, {"label": "HQ Audio", "value": "408", "standard_keys": ["category.lossless_music"]}], "medium": [{"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray", "medium.uhd_bluray"]}, {"label": "REMUX", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "WEB-DL", "value": "9", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "MiniBD", "value": "4", "standard_keys": ["medium.minibd"]}, {"label": "HDTV", "value": "5", "standard_keys": ["medium.hdtv"]}, {"label": "HD DVD", "value": "2", "standard_keys": ["medium.hddvd"]}, {"label": "DVDR", "value": "6", "standard_keys": ["medium.dvdr", "medium.dvd"]}, {"label": "CD", "value": "8", "standard_keys": ["medium.cd"]}], "codec": [{"label": "H.264(x264/AVC)", "value": "7", "standard_keys": ["video.h264"]}, {"label": "MPEG-4", "value": "1", "standard_keys": ["video.mpeg4"]}, {"label": "H.265(x265/HEVC)", "value": "6", "standard_keys": ["video.h265"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "Xvid", "value": "3", "standard_keys": ["video.xvid"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "Other", "value": "5", "standard_keys": ["video.other"]}, {"label": "AV1", "value": "8", "standard_keys": ["video.av1"]}, {"label": "VP8/9", "value": "9", "standard_keys": ["video.vp9"]}, {"label": "AVS", "value": "10", "standard_keys": ["video.avs"]}], "audiocodec": [{"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]}, {"label": "DD5.1/AC-3", "value": "5", "standard_keys": ["audio.ac3", "audio.dd"]}, {"label": "TrueHD", "value": "7", "standard_keys": ["audio.truehd"]}, {"label": "DTS", "value": "3", "standard_keys": ["audio.dts"]}, {"label": "DTS-HD MA/DTS XLL", "value": "4", "standard_keys": ["audio.dts_hd_ma"]}, {"label": "LPCM", "value": "8", "standard_keys": ["audio.lpcm"]}, {"label": "WAV", "value": "9", "standard_keys": ["audio.wav"]}, {"label": "APE", "value": "2", "standard_keys": ["audio.ape"]}, {"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]}, {"label": "TrueHD Atmos", "value": "10", "standard_keys": ["audio.atmos", "audio.truehd_atmos"]}, {"label": "DD2.0/AC-3", "value": "11"}, {"label": "DTS:X", "value": "12", "standard_keys": ["audio.dtsx"]}, {"label": "DTS-HD HR/HRA", "value": "13", "standard_keys": ["audio.dts_hd_hr"]}, {"label": "DSD", "value": "14", "standard_keys": ["audio.dsd"]}, {"label": "DDP Atmos", "value": "15", "standard_keys": ["audio.ddp_atmos"]}, {"label": "DDP/E-AC-3", "value": "16", "standard_keys": ["audio.ddp"]}, {"label": "MPEG", "value": "17", "standard_keys": ["audio.mpeg"]}, {"label": "Vorbis", "value": "18"}, {"label": "TTA", "value": "19", "standard_keys": ["audio.tta"]}, {"label": "AV3A", "value": "20", "standard_keys": ["audio.av3a"]}, {"label": "MP3", "value": "21", "standard_keys": ["audio.mp3"]}, {"label": "ALAC", "value": "22", "standard_keys": ["audio.alac"]}, {"label": "Opus", "value": "25", "standard_keys": ["audio.opus"]}, {"label": "WMA", "value": "26"}, {"label": "AC-4", "value": "27"}, {"label": "MPEG-H", "value": "28"}, {"label": "MQA", "value": "29"}, {"label": "Other", "value": "24", "standard_keys": ["audio.other"]}], "standard": [{"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "4K", "value": "5", "standard_keys": ["resolution.r2160p"]}], "team": [{"label": "EPiC", "value": "1"}, {"label": "HDArea", "value": "2"}, {"label": "HDWING", "value": "3"}, {"label": "WiKi", "value": "4"}, {"label": "TTG", "value": "5"}, {"label": "other", "value": "6"}, {"label": "MTeam", "value": "7"}, {"label": "HDApad", "value": "8"}, {"label": "CHD", "value": "9"}, {"label": "HDAccess", "value": "10"}, {"label": "HDATV", "value": "11"}, {"label": "cXcY", "value": "12"}, {"label": "CMCT", "value": "13"}]}}`).Error
	})
}
