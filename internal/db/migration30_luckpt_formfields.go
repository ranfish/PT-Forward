package db

import (
	"gorm.io/gorm"

	"github.com/ranfish/pt-forward/internal/model"
)

// §59.159 · migration 30: 幸运 form_config FormFields 补 pt_gen/dburl/uplver 三域。
//
// 用户实战指认+CHS HTML 权威清单：幸运表单含 pt_gen（text，必填嫌疑——"请填写必填项目"
// 根因；PTNexus 竞品传 pt_gen=豆瓣链接）+ dburl 豆瓣 + uplver 匿名（value=yes）。
// executor 按 FormFields 白名单投递——域不注册不传。
func luckptFormFieldsExtra(gormDB *gorm.DB) error {
	var site model.Site
	if err := gormDB.Where("name = ?", "幸运").First(&site).Error; err != nil {
		return nil
	}
	cfg := model.ParseFormConfig(site.PublishFormConfig)
	if cfg == nil {
		return nil
	}
	if cfg.FormFields == nil {
		cfg.FormFields = map[string]string{}
	}
	if _, ok := cfg.FormFields[model.FieldDomainPTGen]; ok {
		return nil // 幂等
	}
	cfg.FormFields[model.FieldDomainPTGen] = "pt_gen"
	cfg.FormFields[model.FieldDomainDoubanURL] = "dburl"
	return gormDB.Model(&model.Site{}).Where("id = ?", site.ID).
		Update("publish_form_config", cfg.Serialize()).Error
}

func init() {
	RegisterMigration(30, "luckpt_formfields_extra", luckptFormFieldsExtra)
}
