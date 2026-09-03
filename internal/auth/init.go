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

	// §59.167 直 stdout（不走 zap）——首次随机密码必须在任何日志级别下可见
	//（Docker 默认 PTF_LOG_LEVEL=error 时 Info 被滤——用户唯一必看信息）
	fmt.Println("============================")
	fmt.Println("初始管理员账号已创建")
	fmt.Println("username: admin")
	fmt.Println("password: " + password)
	fmt.Println("Please login and change password immediately.")
	fmt.Println("============================")
	return nil
}
