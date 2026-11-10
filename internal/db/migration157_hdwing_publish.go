package db

import "gorm.io/gorm"

// migration157 §59.511: 瓷器（hdwing.org）发布适配开闸。
// 依据：docs/32-站点适配器设计/hdwing.md §5（2026-11-10 摸底+四点定案）
//  - 官组修正：删 KiSHD（站规未点名=非官组·首版误配）+补 OpenMV/HDC
//    （用户确认 6+站规点名 LU9998/iHD=8 官组）
//  - form_config：type/medium/codec/audiocodec/standard/team 六域映射 +
//    固定字段（share_rule 恒3欢迎转载·board 恒1普通区·is3d/cover/dburl）
//  - team 值集排除官组三槽（HDC=22/LU9998=21/iHD=12——规则⑥选站层
//    拦截+表单防御双保险）
//  - type 直映射类配 standard_keys（animation14/纪录5/体育15/综艺401/
//    演唱402/MV406/音乐408）；路由类（4K大桶/电影分桶/剧集产区轴）由
//    executor hdwingCategoryValue label 查（值集仍需在场）
func init() {
	RegisterMigration(157, "hdwing_publish", func(gormDB *gorm.DB) error {
		// ① 官组修正（mappings：删 KiSHD·补 OpenMV/HDC）
		if err := gormDB.Exec(`DELETE FROM release_group_mappings WHERE domain = 'hdwing.org' AND group_name = 'KiSHD'`).Error; err != nil {
			return err
		}
		if err := gormDB.Exec(`INSERT OR IGNORE INTO release_group_mappings
			(group_name, domain, site_name, is_official, is_builtin, created_at, updated_at)
			VALUES
			('OpenMV', 'hdwing.org', '瓷器', 1, 1, datetime('now'), datetime('now')),
			('HDC',    'hdwing.org', '瓷器', 1, 1, datetime('now'), datetime('now'))`).Error; err != nil {
			return err
		}

		// ② form_config + is_target 开闸
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'hdwing.org'`, `{
			"enabled": true,
			"anonymous": true,
			"form_fields": {
				"type": "type",
				"medium": "medium_sel",
				"codec": "codec_sel",
				"audiocodec": "audiocodec_sel",
				"standard": "standard_sel",
				"team": "team_sel",
				"small_descr": "small_descr",
				"description": "descr",
				"imdb_url": "url",
				"douban_url": "dburl",
				"cover": "cover",
				"share_rule": "share_rule",
				"board": "board",
				"is3d": "is3d",
				"uplver": "uplver"
			},
			"value_mappings": {
				"type": [
					{"label": "原盘(Full BD)", "value": "20"},
					{"label": "电影Movie(1080p)", "value": "17"},
					{"label": "电影Movie(1080i)", "value": "16"},
					{"label": "电影Movie(720p)", "value": "9"},
					{"label": "欧美剧", "value": "13"},
					{"label": "欧美剧集包", "value": "21"},
					{"label": "中港台剧集", "value": "25"},
					{"label": "中港台剧集包", "value": "22"},
					{"label": "日剧", "value": "24"},
					{"label": "日韩剧集包", "value": "23"},
					{"label": "韩剧", "value": "26"},
					{"label": "纪录片", "value": "5", "standard_keys": ["category.documentary", "category.documentaries"]},
					{"label": "体育节目", "value": "15", "standard_keys": ["category.sports"]},
					{"label": "动画片", "value": "14", "standard_keys": ["category.animation"]},
					{"label": "综艺", "value": "401", "standard_keys": ["category.tv_shows"]},
					{"label": "演唱会", "value": "402", "standard_keys": ["category.concert"]},
					{"label": "MV", "value": "406", "standard_keys": ["category.mv", "category.music_video"]},
					{"label": "音乐", "value": "408", "standard_keys": ["category.music"]},
					{"label": "4K UltraHD", "value": "410"},
					{"label": "其他", "value": "409", "standard_keys": ["category.other"]}
				],
				"medium": [
					{"label": "Blu-ray", "value": "11", "standard_keys": ["medium.bluray"]},
					{"label": "HD DVD", "value": "12", "standard_keys": ["medium.hddvd"]},
					{"label": "Remux", "value": "6", "standard_keys": ["medium.remux", "medium.uhd_remux"]},
					{"label": "MiniBD", "value": "2", "standard_keys": ["medium.minibd"]},
					{"label": "Encode", "value": "5", "standard_keys": ["medium.encode"]},
					{"label": "HDTV", "value": "13", "standard_keys": ["medium.hdtv"]},
					{"label": "WEB-DL", "value": "21", "standard_keys": ["medium.webdl", "medium.webrip"]},
					{"label": "DVD", "value": "14", "standard_keys": ["medium.dvd"]},
					{"label": "DVDR", "value": "4", "standard_keys": ["medium.dvdr"]},
					{"label": "CD", "value": "1", "standard_keys": ["medium.cd"]},
					{"label": "Other", "value": "15", "standard_keys": ["medium.other"]}
				],
				"codec": [
					{"label": "H.264/AVC", "value": "1", "standard_keys": ["video.h264"]},
					{"label": "x264", "value": "6", "standard_keys": ["video.h264"]},
					{"label": "x265/HEVC", "value": "10", "standard_keys": ["video.h265"]},
					{"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]},
					{"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]},
					{"label": "Xvid", "value": "3", "standard_keys": ["video.xvid"]},
					{"label": "Other", "value": "5", "standard_keys": ["video.other"]}
				],
				"audiocodec": [
					{"label": "DTS X", "value": "14", "standard_keys": ["audio.dtsx"]},
					{"label": "Atmos", "value": "15", "standard_keys": ["audio.atmos", "audio.truehd_atmos", "audio.ddp_atmos"]},
					{"label": "DTS-HDMA", "value": "12", "standard_keys": ["audio.dts_hd_ma", "audio.dts_hd_hr"]},
					{"label": "True-HD", "value": "13", "standard_keys": ["audio.truehd"]},
					{"label": "LPCM", "value": "11", "standard_keys": ["audio.lpcm", "audio.pcm"]},
					{"label": "DTS", "value": "3", "standard_keys": ["audio.dts"]},
					{"label": "AC3", "value": "8", "standard_keys": ["audio.ac3", "audio.dd", "audio.ddp"]},
					{"label": "WAVE", "value": "9", "standard_keys": ["audio.wav"]},
					{"label": "FLAC", "value": "1", "standard_keys": ["audio.flac"]},
					{"label": "APE", "value": "2", "standard_keys": ["audio.ape"]},
					{"label": "AAC", "value": "6", "standard_keys": ["audio.aac"]},
					{"label": "OGG", "value": "5", "standard_keys": ["audio.ogg"]},
					{"label": "MP2/3", "value": "4", "standard_keys": ["audio.mp3", "audio.mpeg"]},
					{"label": "Other", "value": "7", "standard_keys": ["audio.other"]}
				],
				"standard": [
					{"label": "8K", "value": "19", "standard_keys": ["resolution.r4320p"]},
					{"label": "4K", "value": "17", "standard_keys": ["resolution.r2160p"]},
					{"label": "2K", "value": "18", "standard_keys": ["resolution.r1440p"]},
					{"label": "1080P", "value": "11", "standard_keys": ["resolution.r1080p"]},
					{"label": "1080I", "value": "12", "standard_keys": ["resolution.r1080i"]},
					{"label": "720P", "value": "13", "standard_keys": ["resolution.r720p"]},
					{"label": "SD", "value": "15", "standard_keys": ["resolution.sd"]},
					{"label": "其它", "value": "10", "standard_keys": ["resolution.other"]}
				],
				"team": [
					{"label": "KiSHD", "value": "17"},
					{"label": "OpenMV", "value": "7"},
					{"label": "DIY", "value": "23"},
					{"label": "KHQ", "value": "6"},
					{"label": "ExREN", "value": "30"},
					{"label": "JOMA", "value": "26"},
					{"label": "Anonymous", "value": "25"},
					{"label": "CrsS", "value": "24"},
					{"label": "Ebp", "value": "18"},
					{"label": "DON", "value": "19"},
					{"label": "ESiR", "value": "20"},
					{"label": "TrollHD", "value": "29"},
					{"label": "WiKi", "value": "9"},
					{"label": "beAst", "value": "4"},
					{"label": "CMCT", "value": "2"},
					{"label": "NGB", "value": "8"},
					{"label": "TAiCHi", "value": "28"},
					{"label": "U2", "value": "27"},
					{"label": "其他(Other)", "value": "5", "standard_keys": ["team.other"]}
				]
			}
		}`).Error
	})
}
