package auth

import (
	"context"

	"github.com/ranfish/pt-forward/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func EnsureAdminUser(ctx context.Context, repo model.AuthRepository, logger *zap.Logger) error {
	_, err := repo.GetByUsername(ctx, "admin")
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return authError(ErrAuthInit, "check admin user", err)
	}

	password, err := GenerateRandomPassword(8)
	if err != nil {
		return authError(ErrAuthInit, "generate random password", err)
	}

	hash, err := HashPassword(password)
	if err != nil {
		return authError(ErrAuthInit, "hash password", err)
	}

	user := &model.User{
		Username:     "admin",
		DisplayName:  "admin",
		PasswordHash: hash,
	}
	if err := repo.Create(ctx, user); err != nil {
		return authError(ErrAuthInit, "create admin user", err)
	}

	logger.Info("admin user created with random password", zap.String("hint", "use -reset-password flag to set a known password"))
	return nil
}
