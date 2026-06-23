package auth

import (
	"context"
	"fmt"

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

	fmt.Printf("======================================================\n")
	fmt.Printf("  Admin user created\n")
	fmt.Printf("  Username: admin\n")
	fmt.Printf("  Password: %s\n", password)
	fmt.Printf("  Please login and change password immediately.\n")
	fmt.Printf("======================================================\n")

	logger.Info("admin user created", zap.String("hint", "check stdout for credentials"))
	return nil
}
