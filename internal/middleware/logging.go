package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const RequestIDKey contextKey = "request_id"

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func RequestLogger(logger *zap.Logger) func(http.Handler) http.Handler {
	logger = logger.With(zap.String("component", "http"))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/healthz" || r.URL.Path == "/api/v1/system/ping" {
				next.ServeHTTP(w, r)
				return
			}

			if _, ok := w.(http.Hijacker); ok {
				next.ServeHTTP(w, r)
				return
			}

			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.NewString()[:8]
			}
			w.Header().Set("X-Request-ID", requestID)

			ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
			r = r.WithContext(ctx)

			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rec, r)

			duration := time.Since(start)

			fields := []zap.Field{
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", rec.status),
				zap.Duration("duration", duration),
				zap.String("remote", r.RemoteAddr),
				zap.String("request_id", requestID),
			}

			switch {
			case rec.status >= 500:
				logger.Error("request", fields...)
			case rec.status >= 400:
				logger.Warn("request", fields...)
			default:
				logger.Info("request", fields...)
			}
		})
	}
}

func GetRequestID(ctx context.Context) string {
	if v, ok := ctx.Value(RequestIDKey).(string); ok {
		return v
	}
	return ""
}
