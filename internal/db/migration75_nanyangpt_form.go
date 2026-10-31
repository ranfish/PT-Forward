package db

import (
	"gorm.io/gorm"
)

// migration75 §59.390: 南洋（nanyangpt.com）发布表单配置分发。
// 依据：站方 upload.php 实时 HTML（42.3KB 直连）+ docs/32/nanyangpt.md
// 377 行读全。
// **全库最极简表单**：唯一 select 槽 type 11 项（401-411）——无
// medium/codec/standard/audiocodec/team/source/processing/tags/
// pt_gen 槽（文档 §1.2+实时双重实证）。质量信息仅通过分类和标题传达。
// 实时 HTML 两处文档漂移（§59.384 附 label+value 双源=实时 HTML）：
//   - 文档 §1.2 称"无 uplver"——**实时有 uplver**（4 月分析后站方加
//     或文档错录）→ 配置含 uplver+anonymous:true
//   - **prohibit_transfer 新字段**（禁转声明 checkbox——发布方主动
//     勾选声明本种禁转；转发种不勾）→ 不配（发布链无此域，语义
//     属于"声明禁转"非我们的转播场景）
// 有 url（IMDb）+**dburl（豆瓣独立字段——FieldDomainDoubanURL
// §59.159 PTNexus/修道院同款）**+cite_site 引用功能。
// 站规要点（适配记录注明——影响发布策略非表单）：
//   - **禁止搬运蓝光原盘（含 DIY）及 REMUX**（未经允许需论坛报备）
//     ——发布预览时人工规避，发布链无媒介槽无法自动过滤
//   - WEB-DL 限制（不接受 WEB-DL 压制/DIY；欧美仅国外流媒体源；
//     有 1080p 不收 720p）
//   - 电影/纪录不执行 dupe（对多家重编码友好）
//   - 码率下限：720p≥4000kbps/1080p≥8000kbps
// 幂等：仅空值行写入。
func init() {
	RegisterMigration(75, "nanyangpt_publish_form_config", func(gormDB *gorm.DB) error {
		var count int64
		gormDB.Raw("SELECT COUNT(*) FROM sites WHERE domain = 'nanyangpt.com' AND (publish_form_config IS NULL OR publish_form_config = '')").Scan(&count)
		if count == 0 {
			return nil
		}
		return gormDB.Exec(`UPDATE sites SET publish_form_config = ?, is_target = 1, updated_at = datetime('now') WHERE domain = 'nanyangpt.com'`, `{"anonymous": true, "enabled": true, "form_fields": {"type": "type", "small_descr": "small_descr", "description": "descr", "imdb_url": "url", "dburl": "dburl", "uplver": "uplver"}, "framework": "np", "value_mappings": {"type": [{"label": "电影", "value": "401", "standard_keys": ["category.movie"]}, {"label": "剧集", "value": "402", "standard_keys": ["category.tv_series"]}, {"label": "动漫", "value": "403", "standard_keys": ["category.animation"]}, {"label": "综艺", "value": "404", "standard_keys": ["category.tv_shows"]}, {"label": "体育", "value": "405", "standard_keys": ["category.sports"]}, {"label": "纪录", "value": "406", "standard_keys": ["category.documentary", "category.documentaries"]}, {"label": "音乐", "value": "407", "standard_keys": ["category.music"]}, {"label": "学习", "value": "408", "standard_keys": ["category.education"]}, {"label": "软件", "value": "409", "standard_keys": ["category.software"]}, {"label": "游戏", "value": "410", "standard_keys": ["category.game"]}, {"label": "其它", "value": "411", "standard_keys": ["category.other"]}]}}`).Error
	})
}
