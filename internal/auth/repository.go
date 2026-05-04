package auth

import (
	"context"

	"github.com/ranfish/pt-forward/internal/model"
	"gorm.io/gorm"
)

type gormAuthRepository struct {
	db *gorm.DB
}

func NewGormAuthRepository(db *gorm.DB) model.AuthRepository {
	return &gormAuthRepository{db: db}
}

func (r *gormAuthRepository) GetByUsername(_ context.Context, username string) (*model.User, error) {
	var user model.User
	err := r.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *gormAuthRepository) Create(_ context.Context, user *model.User) error {
	return r.db.Create(user).Error
}

func (r *gormAuthRepository) Update(_ context.Context, user *model.User) error {
	return r.db.Save(user).Error
}

func (r *gormAuthRepository) UpdatePassword(_ context.Context, userID uint, hash string) error {
	return r.db.Model(&model.User{}).Where("id = ?", userID).Update("password_hash", hash).Error
}
