package db

import (
	"gorm.io/gorm"
)

// migration125 §59.442: 阿玲（pt.aling.de）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（41.5KB 直连）+ docs/32/aling.md
// 385 行读全（rules.php+Dupe 规则+站点特殊政策完整采集）。
// **实时=资产=文档三源全槽一致（零漂移站）**。
// 槽位映射：
//   - **type 4 全库最少（电影/剧集/纪录/动画）**：综艺/体育/音乐/
//     游戏等 miss 落空预览人工（文档 §5.4 分类限制——站方仅收影视
//     动画四类）+404 双键。
//   - medium 8：**Blu-ray DIY(2) label-only（§59.407 diyMediumOption
//     自动生效第八站——DIY 种标题词形动态分发）**+Blu-ray(1)=
//     [bluray,uhd_bluray] 双键归并（无 UHD 项）+Remux(3) 双键+
//     **TV(4)=[hdtv]（TV 代替 HDTV——hdtv 键承接）**+WEB-DL(5)=
//     [webdl,webrip] 双键承接+**DVD 原盘(6)=[dvd,dvdr] 双键宽语义**
//     +Encode/Other。
//   - codec 6 全配键（简短括号命名）：H.264/H.265/AV1/MPEG-2/
//     VC-1/Other——无 X26x 零联动（站方合并项语义）。
//   - standard 5 简短命名（8K 置首乱序）：8K=[r4320p]/4K=[r2160p]/
//     1080p/720p/SD 全配；无 Other/1080i——i 形态 miss 落空预览人工。
//   - **无 audiocodec 槽（第七站）**。
//   - team 3 极简：**aling/alingWEB 站方官组 label-only**+Other(3)=
//     [team.other] 兜底。
//   - **source_sel 11=产区槽不配（咖啡/青蛙/红豆饭定案）**——产区
//     最细分站（内地/港/台/日/朝/印度/印尼/泰国/苏联/欧米/其他）
//     发布链产区数据无静态投递能力预览人工（文档 §5.3；零案例不
//     预实施）。
//   - tags 8：禁转(1) 绝不配+7 键配——DIY/中字/HDR 四键泛家族/
//     国语/粤语/**Dolby Vision(14)=[dolby_vision]**+其他中国方言(10)
//     label-only（无键）。
//   - 字段：**pt_gen 四来源最广（IMDb/豆瓣/Bangumi/indienova）**+
//     technical_info+**uplver 默认勾选（checked——anonymous:true
//     对齐站方默认）**+small_descr/descr；无 url/nfo。
// 站规人工把关：Dupe 规则（媒介优先级+同媒介分辨率组优先级+动漫
//   HDTV/DVD 同级特例 §5.1）+资源打包规则+促销规则+认领规则+
//   站点特殊政策（topicid=1）。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(125, "aling_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'pt.aling.de' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'pt.aling.de'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "medium": "medium_sel[4]", "codec": "codec_sel[4]", "standard": "standard_sel[4]", "team": "team_sel[4]", "tags": "tags[4][]", "small_descr": "small_descr", "description": "descr", "pt_gen": "pt_gen", "technical_info": "technical_info", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "电视剧", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "动画", "value": "405", "standard_keys": ["category.animation"]}, {"label": "纪录片", "value": "404", "standard_keys": ["category.documentary", "category.documentaries"]}], "medium": [{"label": "Blu-ray", "value": "1", "standard_keys": ["medium.bluray", "medium.uhd_bluray"]}, {"label": "Blu-ray DIY", "value": "2"}, {"label": "Remux", "value": "3", "standard_keys": ["medium.remux", "medium.uhd_remux"]}, {"label": "Encode", "value": "7", "standard_keys": ["medium.encode"]}, {"label": "TV", "value": "4", "standard_keys": ["medium.hdtv"]}, {"label": "WEB-DL", "value": "5", "standard_keys": ["medium.webdl", "medium.webrip"]}, {"label": "DVD 原盘", "value": "6", "standard_keys": ["medium.dvd", "medium.dvdr"]}, {"label": "Other", "value": "8", "standard_keys": ["medium.other"]}], "codec": [{"label": "H.264(AVC)", "value": "1", "standard_keys": ["video.h264"]}, {"label": "H.265(HEVC)", "value": "2", "standard_keys": ["video.h265"]}, {"label": "AV1", "value": "3", "standard_keys": ["video.av1"]}, {"label": "MPEG-2", "value": "4", "standard_keys": ["video.mpeg2"]}, {"label": "VC-1", "value": "5", "standard_keys": ["video.vc1"]}, {"label": "Other", "value": "6", "standard_keys": ["video.other"]}], "standard": [{"label": "8K", "value": "1", "standard_keys": ["resolution.r4320p"]}, {"label": "4K", "value": "2", "standard_keys": ["resolution.r2160p"]}, {"label": "1080p", "value": "3", "standard_keys": ["resolution.r1080p"]}, {"label": "720p", "value": "4", "standard_keys": ["resolution.r720p"]}, {"label": "SD", "value": "5", "standard_keys": ["resolution.sd"]}], "team": [{"label": "aling", "value": "1"}, {"label": "alingWEB", "value": "2"}, {"label": "Other", "value": "3", "standard_keys": ["team.other"]}], "tags": [{"label": "国语", "value": "12", "standard_keys": ["tag.chinese_audio"]}, {"label": "粤语", "value": "13", "standard_keys": ["tag.cantonese_audio"]}, {"label": "其他中国方言", "value": "10"}, {"label": "中字", "value": "6", "standard_keys": ["tag.chinese_subtitle"]}, {"label": "禁转", "value": "1"}, {"label": "Dolby Vision", "value": "14", "standard_keys": ["tag.dolby_vision"]}, {"label": "DIY", "value": "4", "standard_keys": ["tag.diy"]}, {"label": "HDR", "value": "7", "standard_keys": ["tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"]}]}}`).Error
	})
}
