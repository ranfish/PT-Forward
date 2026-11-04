package db

import (
	"gorm.io/gorm"
)

// migration145 §59.462 附五: 海豹目标站适配开闸——publish_form_config+
// is_target+fetch_rps。
//
// 架构（GazellePW · 全库首个 gazelle framework 站 · §59.462 五轮验证矩阵）：
//   - 发布走 api.php?action=upload（API Key 认证·黄金规则合规）
//   - form_fields 六裸名（source/codec/resolution/container/processing/
//     processing_other）——字符串值直配（站方 $Codecs/$Sources 等数组权威）
//   - 值集：GPW const.php 权威（4 月资产零漂移实测）
//
// value_mappings 要点：
//   - type=Movies 恒定（纯电影站无分类下拉）——不配 type 域
//   - source 八项字符串值（VHS/DVD/HD-DVD/TV/HDTV/WEB/Blu-ray/Other——
//     语义对齐 source_sel 标准键；TV/HDTV 双键承接）
//   - codec 七项（DivX/XviD/x264/H.264/x265/H.265/Other——H/X 四分立
//     §59.380 联动站：h264→H.264/x264→x264 直配+压制联动自动生效）
//   - resolution 九档字符串（NTSC/PAL/480p~2160p/Other——全分辨率覆盖）
//   - container 八项（AVI/MPG/MP4/MKV/VOB IFO/ISO/m2ts/Other——MI 容器
//     透出 §59.453 基建+原盘 ISO/m2ts 形态判）
//   - processing 五项（---/Encode/Remux/DIY/Untouched——encode→Encode/
//     remux→Remux 双键/diy→DIY）
//   - 原盘碟规格（processing_other：BD25/50/66/100+DVD5/9——体积判定
//     executor 特化注入）
//
// fetch_rps=0.4（24/min）：黄金规则 10s/5req=30/min 严格——默认桶 0.5
// RPS=30/min 恰好卡线无余量，收紧留 20% 余量（§59.462 限流实战实证）。
//
// 幂等：条件写（is_target=0 或配置空才 UPDATE）。
func init() {
	RegisterMigration(145, "gpw_publish_config", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites SET is_target = 1,
			fetch_rps = 0.4,
			publish_form_config = ?,
			updated_at = datetime('now')
			WHERE domain = 'greatposterwall.com'
			AND (is_target = 0 OR publish_form_config IS NULL OR publish_form_config = '')`,
			`{"enabled":true,
			"form_fields":{
				"type":"","medium":"","codec":"codec","standard":"resolution","source":"source","container":"container","processing":"processing","processing_other":"processing_other"
			},
			"value_mappings":{
				"source":[
					{"label":"VHS","value":"VHS","standard_keys":["medium.vhs"]},
					{"label":"DVD","value":"DVD","standard_keys":["medium.dvd","medium.dvdr"]},
					{"label":"HD-DVD","value":"HD-DVD","standard_keys":["medium.hddvd"]},
					{"label":"TV","value":"TV","standard_keys":["medium.tvrip"]},
					{"label":"HDTV","value":"HDTV","standard_keys":["medium.hdtv","medium.uhdtv"]},
					{"label":"WEB","value":"WEB","standard_keys":["medium.webdl","medium.webrip"]},
					{"label":"Blu-ray","value":"Blu-ray","standard_keys":["medium.bluray","medium.uhd_bluray","medium.remux","medium.uhd_remux","medium.encode","medium.bdrip","medium.minibd"]},
					{"label":"Other","value":"Other","standard_keys":["medium.other"]}
				],
				"codec":[
					{"label":"DivX","value":"DivX"},
					{"label":"XviD","value":"XviD","standard_keys":["video.xvid"]},
					{"label":"x264","value":"x264","standard_keys":["video.x264"]},
					{"label":"H.264","value":"H.264","standard_keys":["video.h264"]},
					{"label":"x265","value":"x265","standard_keys":["video.x265"]},
					{"label":"H.265","value":"H.265","standard_keys":["video.h265"]},
					{"label":"Other","value":"Other"}
				],
				"standard":[
					{"label":"NTSC","value":"NTSC","standard_keys":["resolution.other"]},
					{"label":"PAL","value":"PAL","standard_keys":["resolution.other"]},
					{"label":"480p","value":"480p","standard_keys":["resolution.r480p"]},
					{"label":"576p","value":"576p","standard_keys":["resolution.other"]},
					{"label":"720p","value":"720p","standard_keys":["resolution.r720p"]},
					{"label":"1080i","value":"1080i","standard_keys":["resolution.r1080i"]},
					{"label":"1080p","value":"1080p","standard_keys":["resolution.r1080p"]},
					{"label":"2160p","value":"2160p","standard_keys":["resolution.r2160p"]},
					{"label":"Other","value":"Other","standard_keys":["resolution.other"]}
				],
				"container":[
					{"label":"MKV","value":"MKV","standard_keys":["container.mkv","container.matroska"]},
					{"label":"MP4","value":"MP4","standard_keys":["container.mp4"]},
					{"label":"AVI","value":"AVI"},
					{"label":"MPG","value":"MPG"},
					{"label":"VOB IFO","value":"VOB IFO","standard_keys":["container.vob"]},
					{"label":"ISO","value":"ISO","standard_keys":["container.iso"]},
					{"label":"m2ts","value":"m2ts","standard_keys":["container.m2ts","container.bdav"]},
					{"label":"Other","value":"Other"}
				],
				"processing":[
					{"label":"---","value":"---"},
					{"label":"Encode","value":"Encode","standard_keys":["medium.encode","medium.bdrip"]},
					{"label":"Remux","value":"Remux","standard_keys":["medium.remux","medium.uhd_remux"]},
					{"label":"DIY","value":"DIY","standard_keys":["tag.diy"]},
					{"label":"Untouched","value":"Untouched","standard_keys":["medium.bluray","medium.uhd_bluray"]}
				],
				"processing_other":[
					{"label":"BD25","value":"BD25"},
					{"label":"BD50","value":"BD50"},
					{"label":"BD66","value":"BD66"},
					{"label":"BD100","value":"BD100"},
					{"label":"DVD5","value":"DVD5"},
					{"label":"DVD9","value":"DVD9"}
				]
			}}`).Error
	})
}
