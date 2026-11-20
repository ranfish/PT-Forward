package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ranfish/pt-forward/internal/auth"
)

type contextKey string

const UserIDKey contextKey = "user_id"

func JWTAuth(authManager *auth.AuthManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := ""

			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
				if tokenStr == authHeader {
					writeAuthError(w, http.StatusUnauthorized, 40100, "认证格式错误，需要 Bearer Token")
					return
				}
			} else if q := r.URL.Query().Get("token"); q != "" {
				tokenStr = q
			} else {
				writeAuthError(w, http.StatusUnauthorized, 40100, "未提供认证 Token")
				return
			}

			claims, err := authManager.ValidateAccessToken(tokenStr)
			if err != nil {
				// §59.542 追记八: ops token 通道（JWT 失败后尝试）——运维 API
				// 认证与用户登录密码分离（Agent 用 .env 的 token·用户改密码
				// 互不干扰）。未配置（PTF_OPS_TOKEN 空）时恒不命中零影响。
				if authManager.MatchOpsToken(tokenStr) {
					ctx := context.WithValue(r.Context(), auth.CtxKeyUserID, "admin")
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				writeAuthError(w, http.StatusUnauthorized, 40101, "Token 无效或已过期")
				return
			}

			ctx := context.WithValue(r.Context(), auth.CtxKeyUserID, claims.Sub)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeAuthError(w http.ResponseWriter, status int, code int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    code,
		"message": message,
	})
}
