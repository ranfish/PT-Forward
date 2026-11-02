package db

import (
	"gorm.io/gorm"
)

// migration112 §59.428: 分享站（pt.itzmx.com）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（34.9KB 直连）+ docs/32/itzmx.md
// 388 行读全（rules.php 完整站规+促销+账号保留）。
// **实时 vs 4 月资产 type 值集漂移**（站方改版：资产 403 综艺/409 其他
// 已删+实时新增 410 软件/411 游戏——实时=文档一致为准，资产过时）。
// 槽位映射：
//   - type 8：**414 蓝光独立分类**（站内实态仲裁——414 列表页 123 种全
//     原盘标题 Blu-ray AVC/UHD Blu-ray DoVi——时光 424 同构形态）——
//     itzmxCategoryValue typeMappingOf 第八站（**discCategoryRoute 公共
//     路由首复用**——§59.426 时光实现收敛公共+两站 target 参数化）：
//     影视类∧原盘族→414+414 label-only 特化独占；401/402/405/408 双键/
//     410=[software]/411=[game] 标准键直配；404 漫画 label-only（dict
//     无 comic 键）。
//   - standard 3 项极简（乱序 2160p=3）：1080p/720p/2160p 全配键。
//   - team 1 项 Other=[team.other] 配键（全库最少——无官组转载站）。
//   - **无 medium/codec/audiocodec/tags/url/pt_gen/nfo/technical_info/
//     uplver**（极简站历史最少之一——南洋式扩展；**无匿名字段**——
//     anonymous 无承载点不消费）。
// 站规人工把关：候选制+游戏上传员等级+<100MB 禁（软件/文档/单曲豁免）+
// upscale/RMVB/硬盘版游戏禁+dupe 媒介优先级（断种 45 日·18 个月豁免）+
// 促销 90% 50%+体育不得泄露比赛结果+电影/剧集简介必须含 IMDb 链接。
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(112, "itzmx_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'pt.itzmx.com' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'pt.itzmx.com'`, `{"enabled": true, "form_fields": {"type": "type", "standard": "standard_sel", "team": "team_sel", "small_descr": "small_descr", "description": "descr"}, "framework": "np", "value_mappings": {"type": [{"label": "动画", "value": "405", "standard_keys": ["category.animation"]}, {"label": "漫画", "value": "404"}, {"label": "音乐", "value": "408", "standard_keys": ["category.music", "category.lossless_music"]}, {"label": "电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "电视剧", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "蓝光", "value": "414"}, {"label": "软件", "value": "410", "standard_keys": ["category.software"]}, {"label": "游戏", "value": "411", "standard_keys": ["category.game"]}], "standard": [{"label": "1080p", "value": "1", "standard_keys": ["resolution.r1080p"]}, {"label": "720p", "value": "2", "standard_keys": ["resolution.r720p"]}, {"label": "2160p", "value": "3", "standard_keys": ["resolution.r2160p"]}], "team": [{"label": "Other", "value": "1", "standard_keys": ["team.other"]}]}}`).Error
	})
}
