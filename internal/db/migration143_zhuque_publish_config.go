package db

import (
	"gorm.io/gorm"
)

// migration143 §59.461: 朱雀目标站适配开闸——publish_form_config+is_target。
//
// 架构（TNode REST API 第四形态 · Vue SPA+CSRF · §59.461 摸底）：
//   - 发布链 TNodeAdapter.UploadTorrent 已在（CSRF/multipart/字段映射
//     /响应解析全套——本 migration 仅配置资产开闸）
//   - form_fields 域→API 字段名（type→category/medium→medium/
//     codec→videoCoding/standard→resolution/tags→tags）
//   - 文本字段（subtitle/note/mediainfo/screenshot）走 PublishRequest
//     顶层直接消费（adapter :344-355），不进 form_fields（避免与顶层
//     双写同名 multipart 字段）
//
// value_mappings 要点（实时值集 2026-11-04 实测=文档零漂移 34 项）：
//   - type 五项：504 节目=[tv_shows(综艺·肉丝权威先例)+documentary]双键；
//     502 剧集=[tv_series]（tv_shows 归 504）
//   - medium 十项：DIY 302/304 label-only（§59.407 diyMediumOption 联动
//     通道自动选中——站方 UHD/BD DIY 分立形态=海的宝同族）；
//     Remux=[remux,uhd_remux] 双键/Encode=[encode,bdrip] 承接/
//     HDTV=[hdtv,tvrip] 承接/WEB-DL=[webdl,webrip] 承接
//   - codec 五项：H/X 四分立完美匹配 dict 键（101 h264/102 h265/
//     103 x264/104 x265——§59.380 压制联动站：压制种 x265→104 直中/
//     原盘 h265→102）；199 Other label-only
//   - standard 五项：无 SD/8K（401-404+499 other——28 站惯例键）
//   - tags 九项：官方(601)/禁转(602)/分集(622) 三绝不配（官种站方专属+
//     站规非官组禁分集+禁转站方标记——财神雷区全集）；国语/中字/杜比视界/
//     HDR10/特效字幕/完结 六键配（Taglist 模式逗号串——野马 tagList 同构）
//
// tmdbid 条件必填：先不填实测（§59.461 用户定案方案 a——meta 全库
// TMDbURL 填充率 0/315 产出通道缺，外部反查违白名单铁律）。
//
// 幂等：条件写（is_target=0 或配置空才 UPDATE）。
func init() {
	RegisterMigration(143, "zhuque_publish_config", func(gormDB *gorm.DB) error {
		return gormDB.Exec(`UPDATE sites SET is_target = 1, publish_form_config = ?,
			updated_at = datetime('now')
			WHERE domain = 'zhuque.in'
			AND (is_target = 0 OR publish_form_config IS NULL OR publish_form_config = '')`,
			`{"form_fields":{"type":"category","medium":"medium","codec":"videoCoding","standard":"resolution","tags":"tags"},
			"value_mappings":{
				"type":[
					{"label":"电影","value":"501","standard_keys":["category.movie"]},
					{"label":"剧集","value":"502","standard_keys":["category.tv_series"]},
					{"label":"动画","value":"503","standard_keys":["category.animation"]},
					{"label":"节目","value":"504","standard_keys":["category.tv_shows","category.documentary"]},
					{"label":"其它","value":"599","standard_keys":["category.other"]}
				],
				"medium":[
					{"label":"UHD Blu-ray","value":"301","standard_keys":["medium.uhd_bluray"]},
					{"label":"UHD Blu-ray DIY","value":"302"},
					{"label":"Blu-ray","value":"303","standard_keys":["medium.bluray"]},
					{"label":"Blu-ray DIY","value":"304"},
					{"label":"Remux","value":"305","standard_keys":["medium.remux","medium.uhd_remux"]},
					{"label":"Encode","value":"306","standard_keys":["medium.encode","medium.bdrip"]},
					{"label":"UHDTV","value":"307","standard_keys":["medium.uhdtv"]},
					{"label":"HDTV","value":"308","standard_keys":["medium.hdtv","medium.tvrip"]},
					{"label":"WEB-DL","value":"309","standard_keys":["medium.webdl","medium.webrip"]},
					{"label":"Other","value":"399","standard_keys":["medium.other"]}
				],
				"codec":[
					{"label":"H264","value":"101","standard_keys":["video.h264"]},
					{"label":"H265","value":"102","standard_keys":["video.h265"]},
					{"label":"x264","value":"103","standard_keys":["video.x264"]},
					{"label":"x265","value":"104","standard_keys":["video.x265"]},
					{"label":"Other","value":"199"}
				],
				"standard":[
					{"label":"720p","value":"401","standard_keys":["resolution.r720p"]},
					{"label":"1080i","value":"402","standard_keys":["resolution.r1080i"]},
					{"label":"1080p","value":"403","standard_keys":["resolution.r1080p"]},
					{"label":"2160p","value":"404","standard_keys":["resolution.r2160p"]},
					{"label":"Other","value":"499","standard_keys":["resolution.other"]}
				],
				"tags":[
					{"label":"官方","value":"601"},
					{"label":"禁转","value":"602"},
					{"label":"国语","value":"603","standard_keys":["tag.chinese_audio"]},
					{"label":"中字","value":"604","standard_keys":["tag.chinese_subtitle"]},
					{"label":"杜比视界","value":"611","standard_keys":["tag.dolby_vision"]},
					{"label":"HDR10","value":"613","standard_keys":["tag.hdr10"]},
					{"label":"特效字幕","value":"614","standard_keys":["tag.special_effects_subs"]},
					{"label":"完结","value":"621","standard_keys":["tag.complete"]},
					{"label":"分集","value":"622"}
				]
			}}`).Error
	})
}
