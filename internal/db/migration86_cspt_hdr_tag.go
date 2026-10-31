package db

import (
	"encoding/json"

	"gorm.io/gorm"
)

// migration86 §59.400: 财神（cspt.top）HDR 标签配键刷正。
//
// 缩小人生案（243 财神种审报"MediaInfo 检测到 HDR信息(HDR10)，请
// 选择HDR标签"）：tags 列已产出 hdr10（fetch 推断链完整）+
// syncHDRTagFromProfile §59.322 补位机制在——唯一断点=财神 HDR(7)
// label-only 无 standard_keys → applier 丢弃不勾。
//
// 配键 [tag.hdr10, tag.hdr10_plus, tag.hdr_vivid, tag.hlg]——站方
// 审核"检测到 HDR 信息请选 HDR 标签"=泛 HDR 语义（HDR 家族承接，
// 车站 DTS-HD/DTS 家族承接同款）；DV 不并入（财神有独立"杜比(11)"
// 标签——DV 种站方要求未知，零案例不预改）。
//
// 实现：JSON 读改写（migration85 同款）；幂等：键已含四族跳过。
func init() {
	RegisterMigration(86, "cspt_hdr_tag_keys", func(gormDB *gorm.DB) error {
		var raw string
		if err := gormDB.Raw("SELECT publish_form_config FROM sites WHERE domain = 'cspt.top'").Scan(&raw).Error; err != nil {
			return err
		}
		if raw == "" {
			return nil
		}
		var cfg map[string]any
		if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
			return nil
		}
		vm, _ := cfg["value_mappings"].(map[string]any)
		if vm == nil {
			return nil
		}
		tags, _ := vm["tags"].([]any)
		for _, it := range tags {
			m, ok := it.(map[string]any)
			if !ok || m["label"] != "HDR" {
				continue
			}
			if ks, _ := m["standard_keys"].([]any); len(ks) > 0 {
				return nil // 已配键（幂等）
			}
			m["standard_keys"] = []string{"tag.hdr10", "tag.hdr10_plus", "tag.hdr_vivid", "tag.hlg"}
			break
		}
		out, err := json.Marshal(cfg)
		if err != nil {
			return err
		}
		return gormDB.Exec("UPDATE sites SET publish_form_config = ?, updated_at = datetime('now') WHERE domain = 'cspt.top'", string(out)).Error
	})
}
