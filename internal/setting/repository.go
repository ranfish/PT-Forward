package setting

import (
	"context"

	"github.com/ranfish/pt-forward/internal/util"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

type Setting struct {
	Key   string `json:"key" gorm:"primaryKey;size:255"`
	Value string `json:"value" gorm:"type:text"`
}

func (Setting) TableName() string { return "system_settings" }

func (r *Repository) Get(ctx context.Context, key string) (string, error) {
	var s Setting
	err := r.db.WithContext(ctx).Where("key = ?", key).First(&s).Error
	if err != nil {
		return "", err
	}
	return s.Value, nil
}

func (r *Repository) Set(ctx context.Context, key, value string) error {
	return r.db.WithContext(ctx).Save(&Setting{Key: key, Value: value}).Error
}

func (r *Repository) Delete(ctx context.Context, key string) error {
	return r.db.WithContext(ctx).Delete(&Setting{}, "key = ?", key).Error
}

func (r *Repository) ListByPrefix(ctx context.Context, prefix string) (map[string]string, error) {
	var settings []Setting
	q := r.db.WithContext(ctx)
	if prefix != "" {
		// 13 补刀（v0.0.1657·用户复审）：prefix 取自 URL 查询参数
		// （settings_handler:95 r.URL.Query().Get("prefix")）——用户
		// 输入裸拼 prefix+"%" 无转义无 ESCAPE·输入 % 时命中全表
		// （探针实证 3 行 vs 前缀语义应 0 行）。改走 util.LikeEscape
		// 前缀锚定形态（LikeEscape(prefix)+"%"——保留「前缀匹配」
		// 原语义·非 LikeContains 的包含语义）+ESCAPE 成对。
		// 原白名单条目「调用方传常量 key 前缀」登记理由失实撤除
		// ——教训：白名单登记必须 grep 全部调用方核验输入源。
		q = q.Where("key LIKE ?"+util.LikeEscapeSuffix, util.LikeEscape(prefix)+"%")
	}
	if err := q.Find(&settings).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string, len(settings))
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result, nil
}

func (r *Repository) ListAll(ctx context.Context) (map[string]string, error) {
	return r.ListByPrefix(ctx, "")
}

func (r *Repository) RestoreAll(ctx context.Context, settings map[string]string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM system_settings").Error; err != nil {
			return err
		}
		for k, v := range settings {
			if err := tx.Save(&Setting{Key: k, Value: v}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
