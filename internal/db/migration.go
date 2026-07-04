package db

import (
	"fmt"
	"sort"
	"time"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Migration struct {
	Version int
	Name    string
	Up      func(*gorm.DB) error
}

var registeredMigrations []Migration

func RegisterMigration(version int, name string, up func(*gorm.DB) error) {
	registeredMigrations = append(registeredMigrations, Migration{Version: version, Name: name, Up: up})
}

func RunMigrations(gormDB *gorm.DB, log *zap.Logger) error {
	migrations := make([]Migration, len(registeredMigrations))
	copy(migrations, registeredMigrations)
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	for _, m := range migrations {
		var count int64
		if err := gormDB.Model(&model.SchemaMigration{}).Where("version = ?", m.Version).Count(&count).Error; err != nil {
			return fmt.Errorf("check migration %d: %w", m.Version, err)
		}
		if count > 0 {
			continue
		}
		log.Info("running data migration", zap.Int("version", m.Version), zap.String("name", m.Name))
		if err := m.Up(gormDB); err != nil {
			return fmt.Errorf("migration %d (%s) failed: %w", m.Version, m.Name, err)
		}
		if err := gormDB.Create(&model.SchemaMigration{Version: m.Version, AppliedAt: time.Now()}).Error; err != nil {
			return fmt.Errorf("record migration %d: %w", m.Version, err)
		}
	}
	return nil
}

func init() {
	RegisterMigration(1, "ema_alpha_default_0.1_to_0.3", func(gormDB *gorm.DB) error {
		return gormDB.Model(&model.SeedingClientConfig{}).Where("ema_alpha = ?", 0.1).Update("ema_alpha", 0.3).Error
	})
}
