package db

import (
	"github.com/ranfish/pt-forward/internal/util"
	"gorm.io/gorm"
)

// migration62 §59.375: 副标题尾部空壳括号存量清洗。
// 跨站传播退化残留：keepfrds FRDS 官种 "[中性种子(NL)]" 标记转载丢失
// → 壹吧(51)/包子(39)/阿玲(31) 共 121 行尾部空壳 "[]"（rss_detail 链
// 采集原样入库——族一词表不命中空壳）。StripSiteOperationMarkers 新增
// 空壳族（与采集层同一实现——migration25/§59.136 同款模式）。
// 扫四种空壳形态（[]/【】/()/（）——实证仅 []，其余防御性覆盖）；
// title 列零案例不动（保守）。
func init() {
	RegisterMigration(62, "subtitle_empty_shell_cleanup", func(gormDB *gorm.DB) error {
		type subRow struct {
			InfoHash string
			Subtitle string
		}
		var rows []subRow
		if err := gormDB.Table("torrent_metadata").
			Select("info_hash, subtitle").
			Where("subtitle LIKE ? OR subtitle LIKE ? OR subtitle LIKE ? OR subtitle LIKE ?",
				"%[]", "%【】", "%()", "%（）").Find(&rows).Error; err != nil {
			return err
		}
		for _, r := range rows {
			cleaned := util.StripSiteOperationMarkers(r.Subtitle)
			if cleaned == r.Subtitle {
				continue
			}
			if err := gormDB.Table("torrent_metadata").
				Where("info_hash = ?", r.InfoHash).
				Update("subtitle", cleaned).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
