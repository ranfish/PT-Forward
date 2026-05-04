package setting

import (
	"context"

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
		q = q.Where("key LIKE ?", prefix+"%")
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
