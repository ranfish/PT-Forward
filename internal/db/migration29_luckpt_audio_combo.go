package db

import (
	"gorm.io/gorm"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.156 切片 2 回归补丁 · migration 29: 幸运 form_config 补音频组合键 standard_keys。
//
// migration 27 的 audiocodec 域漏配组合键词条（"TrueHD Atmos" label 无 standard_keys），
// 执行器 audioMapping 组合优先链（§59.150 判据六：TrueHD+Atmos→11/DDP+Atmos 组合值）
// 落空后 fallback 单键 14——官方预检实测组合键才是正解。27 已执行不可重跑 → 29 增量补。
func luckptAudioComboKeys(gormDB *gorm.DB) error {
	var site model.Site
	if err := gormDB.Where("name = ?", "幸运").First(&site).Error; err != nil {
		return nil
	}
	cfg := model.ParseFormConfig(site.PublishFormConfig)
	if cfg == nil {
		return nil
	}
	combo := map[string][]string{
		"TrueHD Atmos": {"audio.truehd_atmos"},
		"DDP Atmos":    {"audio.ddp_atmos"},
	}
	changed := false
	domain := cfg.ValueMappings[model.FieldDomainAudiocodec]
	for i := range domain {
		if sk, ok := combo[domain[i].Label]; ok {
			if len(domain[i].StandardKeys) == 0 {
				domain[i].StandardKeys = sk
				changed = true
			}
		}
	}
	if !changed {
		return nil
	}
	cfg.ValueMappings[model.FieldDomainAudiocodec] = domain
	return gormDB.Model(&model.Site{}).Where("id = ?", site.ID).
		Update("publish_form_config", cfg.Serialize()).Error
}

func init() {
	RegisterMigration(29, "luckpt_audio_combo_keys", luckptAudioComboKeys)
}
