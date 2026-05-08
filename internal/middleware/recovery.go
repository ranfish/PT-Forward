package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

func timeNowUnix() int64 {
	return time.Now().Unix()
}

func Recovery(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					buf := make([]byte, 4096)
					n := runtime.Stack(buf, false)
					stack := string(buf[:n])

					logger.Error("panic recovered",
						zap.Any("error", err),
						zap.String("method", r.Method),
						zap.String("path", r.URL.Path),
						zap.String("stack", stack),
					)

					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"code":    50000,
						"message": "服务器内部错误",
					})
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

type visitor struct {
	count    int
	lastSeen int64
}

type RateLimiter struct {
	mu       sync.RWMutex
	visitors map[string]*visitor
	limit    int
	window   int64
}

func NewRateLimiter(limit int, windowSeconds int64) *RateLimiter {
	return &RateLimiter{
		visitors: make(map[string]*visitor),
		limit:    limit,
		window:   windowSeconds,
	}
}

func (rl *RateLimiter) Allow(ip string) bool {
	now := timeNowUnix()
	rl.mu.Lock()
	defer rl.mu.Unlock()
	v, exists := rl.visitors[ip]
	if !exists || now-v.lastSeen >= rl.window {
		rl.visitors[ip] = &visitor{count: 1, lastSeen: now}
		return true
	}
	v.count++
	v.lastSeen = now
	return v.count <= rl.limit
}

func (rl *RateLimiter) Cleanup() {
	now := timeNowUnix()
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for ip, v := range rl.visitors {
		if now-v.lastSeen > rl.window*2 {
			delete(rl.visitors, ip)
		}
	}
}

func RateLimit(limit int, windowSeconds int64) func(http.Handler) http.Handler {
	limiter := NewRateLimiter(limit, windowSeconds)
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Duration(windowSeconds) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				limiter.Cleanup()
			}
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/healthz" || r.URL.Path == "/api/v1/system/ping" {
				next.ServeHTTP(w, r)
				return
			}

			ip := ExtractIP(r)
			if !limiter.Allow(ip) {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.Header().Set("Retry-After", fmt.Sprintf("%d", windowSeconds))
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"code":    42900,
					"message": "请求过于频繁，请稍后再试",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func ExtractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	idx := strings.LastIndex(r.RemoteAddr, ":")
	if idx != -1 {
		return r.RemoteAddr[:idx]
	}
	return r.RemoteAddr
}
