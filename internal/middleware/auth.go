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
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeAuthError(w, http.StatusUnauthorized, 40100, "未提供认证 Token")
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenStr == authHeader {
				writeAuthError(w, http.StatusUnauthorized, 40100, "认证格式错误，需要 Bearer Token")
				return
			}

			claims, err := authManager.ValidateAccessToken(tokenStr)
			if err != nil {
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
