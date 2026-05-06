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

	fmt.Println("========================================")
	fmt.Println("PT-Forward 初始账号信息（仅显示一次）")
	fmt.Println("用户名: admin")
	fmt.Printf("密码: %s\n", password)
	fmt.Println("请登录后尽快修改密码")
	fmt.Println("========================================")

	logger.Info("admin user created with random password")
	return nil
}
