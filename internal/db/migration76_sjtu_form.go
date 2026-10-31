package db

import (
	"gorm.io/gorm"
)

// migration76 §59.391: 葡萄（pt.sjtu.edu.cn）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（34.8KB 直连）+ docs/32/sjtu.md
// 563 行读全。
// **产区×类型双维分类站**（type 27 项全库最多——电影按产区三分/
// 剧集四分/综艺四分，用户定案粒度）：产区类 11 项（401/402/403/
// 407-410/411-414）**label-only**——映射由 executor sjtuRegionCategoryValue
// 特化函数现算（§59.391：PTGen Region 首值归类——陆/港/澳/台→华语
// 且大陆 vs 港台细分；日韩独立（综艺）；其余亚洲→亚洲；再其余欧美
// 兜底；无产区证据→欧美 fallback 预览可手改）；无产区类配键
// （406 纪录双键/431 动画/427 MV 双键/432 体育/429 游戏/434 软件/
// 435 学习/450 其他）。
// 槽位：type/codec_sel/standard_sel 三 select（无 medium/audiocodec/
// team/tags）；字段含 url（IMDb）+**douban_url**（字段名非 dburl——
// 站方 HTML 无引号属性形态 name=douban_url）+uplver+isoday（0DAY
// 标记——转发不勾不配）。
// codec 10 项音视频混合（影视类 H.264/h.265/VC-1 配键+音频项
// FLAC/APE/DTS/AC-3 label-only——不发布音乐种）；h.265(10) 站方
// 小写点形态 label 原文。
// 站规注记（发布预览人工规避）：**禁 HEVC 1080p/720p（仅 4K 允许）
// +禁 10bit H.264**；黑名单组 PRLxXunlei/ZJM/ZYM/Mp4Ba/DNL/WOFEI/
// EVO；候选制（Elite User 以下需候选区）。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(76, "sjtu_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'pt.sjtu.edu.cn' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'pt.sjtu.edu.cn'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "codec": "codec_sel", "standard": "standard_sel", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "dburl": "douban_url", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "华语电影", "value": "401"}, {"label": "欧美电影", "value": "402"}, {"label": "亚洲电影", "value": "403"}, {"label": "纪录片", "value": "406", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "港台电视剧", "value": "407"}, {"label": "亚洲电视剧", "value": "408"}, {"label": "大陆电视剧", "value": "409"}, {"label": "欧美电视剧", "value": "410"}, {"label": "大陆综艺节目", "value": "411"}, {"label": "港台综艺节目", "value": "412"}, {"label": "欧美综艺节目", "value": "413"}, {"label": "日韩综艺节目", "value": "414"}, {"label": "华语音乐", "value": "420"}, {"label": "日韩音乐", "value": "421"}, {"label": "欧美音乐", "value": "422"}, {"label": "原声音乐", "value": "423"}, {"label": "古典音乐", "value": "425"}, {"label": "mp3合辑", "value": "426"}, {"label": "Music Videos", "value": "427", "standard_keys": ["category.mv", "category.music_video"]}, {"label": "游戏", "value": "429", "standard_keys": ["category.game"]}, {"label": "动漫", "value": "431", "standard_keys": ["category.animation"]}, {"label": "体育", "value": "432", "standard_keys": ["category.sports"]}, {"label": "软件", "value": "434", "standard_keys": ["category.software"]}, {"label": "学习", "value": "435", "standard_keys": ["category.education"]}, {"label": "mac", "value": "440"}, {"label": "校园原创", "value": "451"}, {"label": "其他", "value": "450", "standard_keys": ["category.other"]}], "codec": [{"label": "H.264", "value": "1", "standard_keys": ["video.h264"]}, {"label": "VC-1", "value": "2", "standard_keys": ["video.vc1"]}, {"label": "Xvid", "value": "3", "standard_keys": ["video.xvid"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "FLAC", "value": "5"}, {"label": "APE", "value": "6"}, {"label": "DTS", "value": "7"}, {"label": "AC-3", "value": "8"}, {"label": "其他", "value": "9", "standard_keys": ["video.other"]}, {"label": "h.265", "value": "10", "standard_keys": ["video.h265"]}], "standard": [{"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "1080i", "value": "2", "standard_keys": ["resolution.r1080i"]}, {"label": "720p", "value": "3", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "4", "standard_keys": ["resolution.sd"]}, {"label": "其他", "value": "5", "standard_keys": ["resolution.other"]}, {"label": "4k", "value": "6", "standard_keys": ["resolution.r2160p"]}]}}`).Error
	})
}
