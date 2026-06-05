package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ranfish/pt-forward/internal/metrics"
)

type metricsRecorder struct {
	http.ResponseWriter
	status  int
	written bool
}

func (r *metricsRecorder) WriteHeader(code int) {
	if !r.written {
		r.status = code
		r.written = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *metricsRecorder) Write(b []byte) (int, error) {
	if !r.written {
		r.status = http.StatusOK
		r.written = true
	}
	return r.ResponseWriter.Write(b)
}

func (r *metricsRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func Metrics() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := w.(http.Hijacker); ok {
				next.ServeHTTP(w, r)
				return
			}

			path := normalizePath(r.URL.Path)
			start := time.Now()
			rec := &metricsRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rec, r)

			duration := time.Since(start).Seconds()
			status := strconv.Itoa(rec.status)

			metrics.HTTPRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
			metrics.HTTPRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
		})
	}
}

func normalizePath(path string) string {
	if len(path) > 64 {
		return path[:64]
	}
	return path
}
